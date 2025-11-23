package bridge

import (
	"log"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	gortspliburl "github.com/bluenviron/gortsplib/v4/pkg/url" // Renamed to avoid conflict
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs" // Added missing import
)

type RTSPPublisher struct {
	client     *gortsplib.Client
	u          *gortspliburl.URL
	media      *description.Media
	packetizer rtp.Packetizer
}

func NewRTSPPublisher(rtspURL, codec string) (*RTSPPublisher, error) {
	u, err := gortspliburl.Parse(rtspURL)
	if err != nil {
		return nil, err
	}

	c := &gortsplib.Client{}

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

	return &RTSPPublisher{
		client:     c,
		u:          u,
		media:      media,
		packetizer: packetizer,
	}, nil
}

func (p *RTSPPublisher) Write(data []byte) error {
	// The data received from WebSocket is likely a raw stream (Annex B).
	// We need to split it into NALUs.
	// A simple way is to look for start codes 00 00 00 01 or 00 00 01.

	nalus := splitNALUs(data)

	for _, nalu := range nalus {
		// Packetize
		packets := p.packetizer.Packetize(nalu, 90000) // 90000 samples per second for video

		for _, packet := range packets {
			if err := p.client.WritePacketRTP(p.media, packet); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *RTSPPublisher) Close() {
	p.client.Close()
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
