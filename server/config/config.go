package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Port             int
	Password         string
	ImageProvider    string
	OpenAIAPIKey     string
	NanoBananaAPIKey string
	NanoBananaURL    string
	DitherServiceURL string
	DitherMode       string // "local", "remote", or "local_with_fallback"
	DailyRateLimit   int
	DisplayWidth     int
	DisplayHeight    int
	DataDir          string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Port:             getEnvInt("PORT", 8080),
		Password:         getEnv("PASSWORD", ""),
		ImageProvider:    getEnv("IMAGE_PROVIDER", "stub"),
		OpenAIAPIKey:     getEnv("OPENAI_API_KEY", ""),
		NanoBananaAPIKey: getEnv("NANOBANANA_API_KEY", ""),
		NanoBananaURL:    getEnv("NANOBANANA_URL", "https://api.nanobanana.com"),
		DitherServiceURL: getEnv("DITHER_SERVICE_URL", "https://dither.shvn.dev"),
		DitherMode:       getEnv("DITHER_MODE", "local_with_fallback"),
		DailyRateLimit:   getEnvInt("DAILY_RATE_LIMIT", 10),
		DisplayWidth:     getEnvInt("DISPLAY_WIDTH", 600),
		DisplayHeight:    getEnvInt("DISPLAY_HEIGHT", 400),
		DataDir:          getEnv("DATA_DIR", "./data"),
	}

	if cfg.Password == "" {
		return nil, fmt.Errorf("PASSWORD environment variable is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return fallback
}
