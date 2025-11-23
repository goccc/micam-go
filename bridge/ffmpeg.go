package bridge

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

type FFmpeg struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stderr io.ReadCloser
}

func StartFFmpeg(rtspURL, codec string) (*FFmpeg, error) {
	args := []string{
		"-y",
		"-v", "error",
		"-hide_banner",
		"-use_wallclock_as_timestamps", "1",
		"-analyzeduration", "20000000", // 20 seconds
		"-probesize", "20000000", // 20 MB
		"-f", codec,
		"-i", "pipe:0",
		"-c:v", "copy",
		"-c:a", "copy",
		"-f", "rtsp",
		"-rtsp_transport", "tcp",
		rtspURL,
	}

	log.Printf("Starting FFmpeg: ffmpeg %s", strings.Join(args, " "))

	cmd := exec.Command("ffmpeg", args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	return &FFmpeg{
		cmd:    cmd,
		stdin:  stdin,
		stderr: stderr,
	}, nil
}

func (f *FFmpeg) Write(data []byte) error {
	_, err := f.stdin.Write(data)
	return err
}

func (f *FFmpeg) Stop() {
	if f.cmd != nil && f.cmd.Process != nil {
		// Try to terminate gracefully
		f.cmd.Process.Signal(os.Interrupt)

		// Wait a bit
		done := make(chan error, 1)
		go func() {
			done <- f.cmd.Wait()
		}()

		select {
		case <-time.After(5 * time.Second):
			f.cmd.Process.Kill()
		case <-done:
			// Exited
		}
	}
}

func (f *FFmpeg) LogStderr() {
	buf := make([]byte, 1024)
	for {
		n, err := f.stderr.Read(buf)
		if n > 0 {
			log.Printf("FFmpeg stderr: %s", string(buf[:n]))
		}
		if err != nil {
			break
		}
	}
}

// Close implements VideoPublisher interface
func (f *FFmpeg) Close() {
	f.Stop()
}
