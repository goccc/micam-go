package bridge

import (
	"log"
	"sync"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	gortspliburl "github.com/bluenviron/gortsplib/v4/pkg/url" // Renamed to avoid conflict
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs" // Added missing import
)

const (
	// Default buffer size for the write queue channel
	defaultWriteBufferSize = 100
)

type RTSPPublisher struct {
	client     *gortsplib.Client
	u          *gortspliburl.URL
	media      *description.Media
	packetizer rtp.Packetizer

	// Async write support
	writeQueue chan []byte
	done       chan struct{}
	wg         sync.WaitGroup
}

func NewRTSPPublisher(rtspURL, codec string) (*RTSPPublisher, error) {
	u, err := gortspliburl.Parse(rtspURL)
	if err != nil {
		return nil, err
	}

	// Configure client with larger write queue as a safety net
	// The main buffering is now done via the channel
	c := &gortsplib.Client{
		WriteQueueSize: 1024, // Increased from default 256
	}

	// Define the media format
	var forma format.Format
	var payloadType uint8

	if codec == "h264" {
		forma = &format.H264{
			PayloadTyp: 96,
		}
		payloadType = 96
	} else if codec == "hevc" {
		forma = &format.H265{
			PayloadTyp: 96,
		}
		payloadType = 96
	} else {
		log.Printf("Unknown codec: %s, defaulting to H264", codec)
		forma = &format.H264{
			PayloadTyp: 96,
		}
		payloadType = 96
	}

	media := &description.Media{
		Type:    description.MediaTypeVideo,
		Formats: []format.Format{forma},
	}

	desc := &description.Session{
		Medias: []*description.Media{media},
	}

	// Connect to the server
	if err := c.Start(u.Scheme, u.Host); err != nil {
		return nil, err
	}

	// Announce the stream
	_, err = c.Announce(u, desc)
	if err != nil {
		c.Close()
		return nil, err
	}

	// Setup the media
	_, err = c.Setup(u, media, 0, 0)
	if err != nil {
		c.Close()
		return nil, err
	}

	// Start recording
	_, err = c.Record()
	if err != nil {
		c.Close()
		return nil, err
	}

	// Create packetizer
	var payloader rtp.Payloader
	if codec == "h264" {
		payloader = &codecs.H264Payloader{}
	} else {
		payloader = &codecs.H265Payloader{}
	}

	packetizer := rtp.NewPacketizer(
		1200, // MTU
		payloadType,
		0, // Initial SSRC
		payloader,
		rtp.NewRandomSequencer(),
		90000, // Clock rate
	)

	p := &RTSPPublisher{
		client:     c,
		u:          u,
		media:      media,
		packetizer: packetizer,
		writeQueue: make(chan []byte, defaultWriteBufferSize),
		done:       make(chan struct{}),
	}

	// Start the async write loop
	p.wg.Add(1)
	go p.writeLoop()

	log.Printf("Created RTSP publisher with async writes (buffer: %d, WriteQueueSize: %d)",
		defaultWriteBufferSize, c.WriteQueueSize)

	return p, nil
}

// Write queues data for async writing. This is non-blocking.
func (p *RTSPPublisher) Write(data []byte) error {
	// Make a copy of the data since the caller may reuse the buffer
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	select {
	case p.writeQueue <- dataCopy:
		return nil
	case <-p.done:
		return nil // Publisher is closing
	default:
		// Queue is full - log and drop the frame
		// In production, you might want to drop only non-keyframes
		log.Printf("Warning: Write queue full, dropping frame (%d bytes)", len(data))
		return nil
	}
}

// writeLoop runs in a goroutine and processes the write queue
func (p *RTSPPublisher) writeLoop() {
	defer p.wg.Done()

	for {
		select {
		case data := <-p.writeQueue:
			// Process the data: split NALUs and write RTP packets
			p.processData(data)
		case <-p.done:
			// Drain remaining items in the queue before exiting
			for {
				select {
				case data := <-p.writeQueue:
					p.processData(data)
				default:
					return
				}
			}
		}
	}
}

// processData splits NALUs and writes RTP packets (blocking operations done here)
func (p *RTSPPublisher) processData(data []byte) {
	nalus := splitNALUs(data)

	for _, nalu := range nalus {
		// Packetize
		packets := p.packetizer.Packetize(nalu, 90000) // 90000 samples per second for video

		for _, packet := range packets {
			if err := p.client.WritePacketRTP(p.media, packet); err != nil {
				log.Printf("Error writing RTP packet: %v", err)
				return
			}
		}
	}
}

func (p *RTSPPublisher) Close() {
	// Signal the write loop to stop
	close(p.done)

	// Wait for the write loop to finish
	p.wg.Wait()

	// Close the RTSP client
	p.client.Close()

	log.Println("RTSP publisher closed gracefully")
}

// splitNALUs splits the byte slice into NAL units based on start codes.
// This is a simplified implementation.
func splitNALUs(data []byte) [][]byte {
	var nalus [][]byte
	start := 0
	for i := 0; i < len(data)-3; i++ {
		if data[i] == 0x00 && data[i+1] == 0x00 {
			if data[i+2] == 0x01 {
				if i > start {
					nalus = append(nalus, data[start:i])
				}
				start = i + 3
				i += 2
			} else if data[i+2] == 0x00 && data[i+3] == 0x01 {
				if i > start {
					nalus = append(nalus, data[start:i])
				}
				start = i + 4
				i += 3
			}
		}
	}
	if start < len(data) {
		nalus = append(nalus, data[start:])
	}
	return nalus
}
