package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/miiot/micam-go/bridge"
	"github.com/miiot/micam-go/config"
)

func main() {
	// Configure logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	appCfg := config.Load()

	if len(appCfg.Cameras) == 0 {
		log.Fatal("No cameras configured")
	}

	var bridges []*bridge.RTSPBridge

	for i := range appCfg.Cameras {
		camCfg := &appCfg.Cameras[i]
		if camCfg.Password == "" {
			log.Printf("Warning: Password is required for camera %s, skipping", camCfg.CameraID)
			continue
		}
		if camCfg.CameraID == "" {
			log.Printf("Warning: Camera ID is required, skipping")
			continue
		}

		b, err := bridge.NewRTSPBridge(camCfg)
		if err != nil {
			log.Printf("Failed to create bridge for camera %s: %v", camCfg.CameraID, err)
			continue
		}
		bridges = append(bridges, b)
	}

	if len(bridges) == 0 {
		log.Fatal("No valid bridges created")
	}

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start all bridges
	for _, b := range bridges {
		go func(br *bridge.RTSPBridge) {
			br.Run()
		}(b)
	}

	log.Printf("Started %d bridges", len(bridges))

	<-sigChan
	log.Println("Received signal, shutting down...")

	for _, b := range bridges {
		b.Close()
	}

	// Give some time for cleanup if needed, or just exit
	os.Exit(0)
}
