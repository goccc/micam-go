package bridge

import (
	"crypto/tls"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/miiot/micam-go/config"
)

type RTSPBridge struct {
	Config    *config.CameraConfig
	Client    *Client
	Publisher VideoPublisher
	mu        sync.Mutex
	closed    bool
}

func NewRTSPBridge(cfg *config.CameraConfig) (*RTSPBridge, error) {
	client, err := NewClient(cfg.BaseURL, cfg.Username, cfg.Password)
	if err != nil {
		return nil, err
	}
	return &RTSPBridge{
		Config: cfg,
		Client: client,
	}, nil
}

func (b *RTSPBridge) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.closed = true
	if b.Publisher != nil {
		b.Publisher.Close()
	}
}

func (b *RTSPBridge) Run() {
	// Reconnection loop
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		// Check if closed
		b.mu.Lock()
		if b.closed {
			b.mu.Unlock()
			return
		}
		b.mu.Unlock()

		func() {
			// Start Publisher
			var err error
			var pub VideoPublisher

			if b.Config.UseFFmpeg {
				pub, err = StartFFmpeg(b.Config.RTSPURL, b.Config.VideoCodec)
			} else {
				pub, err = NewRTSPPublisher(b.Config.RTSPURL, b.Config.VideoCodec)
			}

			if err != nil {
				log.Printf("Failed to start Publisher: %v", err)
				return
			}

			b.mu.Lock()
			if b.closed {
				pub.Close()
				b.mu.Unlock()
				return
			}
			b.Publisher = pub
			b.mu.Unlock()

			defer func() {
				b.mu.Lock()
				if b.Publisher != nil {
					b.Publisher.Close()
					b.Publisher = nil // Clear publisher on close
				}
				b.mu.Unlock()
			}()

			// Login
			if err := b.Client.Login(); err != nil {
				log.Printf("Login failed: %v", err)
				return
			}
			log.Println("Login successful")

			// Connect WebSocket
			protocol := "ws"
			if strings.HasPrefix(b.Config.BaseURL, "https") {
				protocol = "wss"
			}
			host := strings.TrimPrefix(b.Config.BaseURL, "https://")
			host = strings.TrimPrefix(host, "http://")

			wsURL := fmt.Sprintf("%s://%s/api/miot/ws/video_stream?camera_id=%s&channel=%s",
				protocol, host, b.Config.CameraID, b.Config.Channel)

			log.Printf("Connecting to WebSocket: %s", wsURL)

			dialer := websocket.Dialer{
				Jar:             b.Client.HTTPClient.Jar,
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
			conn, _, err := dialer.Dial(wsURL, nil)
			if err != nil {
				log.Printf("WebSocket connection failed: %v", err)
				return
			}
			defer conn.Close()

			log.Println("WebSocket connected. Streaming data...")

			// Reset backoff on successful connection
			backoff = time.Second

			waitingForKeyframe := true

			for {
				messageType, data, err := conn.ReadMessage()
				if err != nil {
					log.Printf("Read error: %v", err)
					break
				}

				if messageType == websocket.BinaryMessage {
					if waitingForKeyframe {
						if IsKeyframe(data, b.Config.VideoCodec) {
							log.Println("Keyframe detected! Starting stream...")
							waitingForKeyframe = false
						} else {
							continue
						}
					}

					b.mu.Lock()
					pub := b.Publisher
					b.mu.Unlock()

					if pub != nil {
						if err := pub.Write(data); err != nil {
							log.Printf("Failed to write to Publisher: %v", err)
							break
						}
					}
				} else if messageType == websocket.CloseMessage {
					log.Println("WebSocket closed")
					break
				}
			}
		}()

		log.Printf("Disconnected. Retrying in %v...", backoff)
		time.Sleep(backoff)
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}
