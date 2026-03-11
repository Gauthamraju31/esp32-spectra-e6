package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// User represents a user of the system.
type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Port             int
	Password         string // Deprecated: use Users instead
	UsersJSON        string
	Users            []User
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
	S3Endpoint       string
	S3AccessKey      string
	S3SecretKey      string
	S3BucketName     string
	CdnDomain        string
	RunwareAPIKey    string
	RunwareModelID   string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Port:             getEnvInt("PORT", 8080),
		Password:         getEnv("PASSWORD", ""),
		UsersJSON:        getEnv("USERS_JSON", ""),
		ImageProvider:    getEnv("IMAGE_PROVIDER", "stub"),
		OpenAIAPIKey:     getEnv("OPENAI_API_KEY", ""),
		NanoBananaAPIKey: getEnv("NANOBANANA_API_KEY", ""),
		NanoBananaURL:    getEnv("NANOBANANA_URL", "https://api.nanobanana.com"),
		DitherServiceURL: getEnv("DITHER_SERVICE_URL", "https://dither.shvn.dev"),
		DitherMode:       getEnv("DITHER_MODE", "local_with_fallback"),
		DailyRateLimit:   getEnvInt("DAILY_RATE_LIMIT", 10),
		DisplayWidth:     getEnvInt("DISPLAY_WIDTH", 600),
		DisplayHeight:    getEnvInt("DISPLAY_HEIGHT", 400),
		DataDir:          getEnv("DATA_DIR", getDefaultDataDir()),
		S3Endpoint:       getEnv("S3_ENDPOINT", ""),
		S3AccessKey:      getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey:      getEnv("S3_SECRET_KEY", ""),
		S3BucketName:     getEnv("S3_BUCKET_NAME", ""),
		CdnDomain:        getEnv("CDN_DOMAIN", ""),
		RunwareAPIKey:    getEnv("RUNWARE_API_KEY", ""),
		RunwareModelID:   getEnv("RUNWARE_MODEL_ID", "runware:101@1"),
	}

	if cfg.UsersJSON != "" {
		if err := json.Unmarshal([]byte(cfg.UsersJSON), &cfg.Users); err != nil {
			return nil, fmt.Errorf("failed to parse USERS_JSON: %v", err)
		}
	} else if cfg.Password != "" {
		// Fallback for single password mode
		cfg.Users = []User{{
			Username: "admin",
			Password: cfg.Password,
			Role:     "admin",
		}}
	} else {
		return nil, fmt.Errorf("PASSWORD or USERS_JSON environment variable is required")
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

func getDefaultDataDir() string {
	// App Engine Standard and Cloud Run have a read-only filesystem except for /tmp
	if os.Getenv("GAE_ENV") != "" || os.Getenv("GAE_APPLICATION") != "" || os.Getenv("K_SERVICE") != "" {
		return "/tmp/esp32-data"
	}
	return "./data"
}
