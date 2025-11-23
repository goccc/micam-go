package config

import (
	"flag"
	"log"
	"os"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type CameraConfig struct {
	BaseURL    string `yaml:"base_url"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
	CameraID   string `yaml:"camera_id"`
	Channel    string `yaml:"channel"`
	VideoCodec string `yaml:"video_codec"`
	RTSPURL    string `yaml:"rtsp_url"`
	UseFFmpeg  bool   `yaml:"use_ffmpeg"`
}

type AppConfig struct {
	Cameras []CameraConfig `yaml:"cameras"`
}

// Load returns an AppConfig. It checks for config.yaml first, then falls back to env vars.
func Load() *AppConfig {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Check for config.yaml
	if _, err := os.Stat("config.yaml"); err == nil {
		data, err := os.ReadFile("config.yaml")
		if err != nil {
			log.Fatalf("Failed to read config.yaml: %v", err)
		}
		var appCfg AppConfig
		if err := yaml.Unmarshal(data, &appCfg); err != nil {
			log.Fatalf("Failed to parse config.yaml: %v", err)
		}
		if len(appCfg.Cameras) > 0 {
			log.Printf("Loaded %d cameras from config.yaml", len(appCfg.Cameras))
			return &appCfg
		}
	}

	// Fallback to env vars / flags
	cfg := CameraConfig{}

	flag.StringVar(&cfg.BaseURL, "base-url", getEnv("MILOCO_BASE_URL", "https://miloco:8000"), "Base URL of the Miloco server")
	flag.StringVar(&cfg.Username, "username", getEnv("MILOCO_USERNAME", "admin"), "Login username")
	flag.StringVar(&cfg.Password, "password", getEnv("MILOCO_PASSWORD", ""), "Login password (MD5)")
	flag.StringVar(&cfg.CameraID, "camera-id", getEnv("CAMERA_ID", ""), "Camera ID to stream")
	flag.StringVar(&cfg.Channel, "channel", getEnv("STREAM_CHANNEL", "0"), "Camera channel")
	flag.StringVar(&cfg.VideoCodec, "video-codec", getEnv("VIDEO_CODEC", "hevc"), "Input video codec (hevc or h264)")
	flag.StringVar(&cfg.RTSPURL, "rtsp-url", getEnv("RTSP_URL", "rtsp://0.0.0.0:8554/live"), "Target RTSP URL")
	flag.BoolVar(&cfg.UseFFmpeg, "use-ffmpeg", getEnv("USE_FFMPEG", "false") == "true", "Use FFmpeg for streaming instead of native Go implementation")

	flag.Parse()

	return &AppConfig{
		Cameras: []CameraConfig{cfg},
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
