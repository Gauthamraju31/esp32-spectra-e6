package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Gauthamraju31/esp32-spectra-e6/server/auth"
	"github.com/Gauthamraju31/esp32-spectra-e6/server/config"
	"github.com/Gauthamraju31/esp32-spectra-e6/server/dither"
	"github.com/Gauthamraju31/esp32-spectra-e6/server/handler"
	"github.com/Gauthamraju31/esp32-spectra-e6/server/provider"
	"github.com/Gauthamraju31/esp32-spectra-e6/server/ratelimit"
	"github.com/Gauthamraju31/esp32-spectra-e6/server/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// Initialize image provider
	var prov provider.ImageProvider
	switch cfg.ImageProvider {
	case "openai":
		if cfg.OpenAIAPIKey == "" {
			fmt.Fprintln(os.Stderr, "OPENAI_API_KEY is required when IMAGE_PROVIDER=openai")
			os.Exit(1)
		}
		prov = provider.NewOpenAIProvider(cfg.OpenAIAPIKey)
	case "nanobanana":
		if cfg.NanoBananaAPIKey == "" {
			fmt.Fprintln(os.Stderr, "NANOBANANA_API_KEY is required when IMAGE_PROVIDER=nanobanana")
			os.Exit(1)
		}
		prov = provider.NewNanoBananaProvider(cfg.NanoBananaAPIKey, cfg.NanoBananaURL)
	default:
		prov = provider.NewStubProvider(cfg.DisplayWidth, cfg.DisplayHeight)
	}

	// Initialize components
	authMgr := auth.NewManager(cfg.Password)
	limiter := ratelimit.NewLimiter(cfg.DailyRateLimit)
	ditherer := dither.NewDitherer(cfg.DitherServiceURL, cfg.DitherMode, cfg.DisplayWidth, cfg.DisplayHeight)

	var imgStore store.ImageStore
	if cfg.S3Endpoint != "" && cfg.S3AccessKey != "" && cfg.S3SecretKey != "" && cfg.S3BucketName != "" {
		imgStore, err = store.NewS3Store(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3BucketName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize S3 image store: %v\n", err)
			os.Exit(1)
		}
		log.Printf("   Storage: S3/R2 (%s)", cfg.S3BucketName)
	} else {
		imgStore, err = store.NewDiskStore(cfg.DataDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize local disk image store: %v\n", err)
			os.Exit(1)
		}
		log.Printf("   Storage: Local Disk (%s)", cfg.DataDir)
	}

	// Initialize handlers
	imgHandler := handler.NewImageHandler(imgStore)
	promptHandler := handler.NewPromptHandler(authMgr, limiter, prov, ditherer, imgStore, cfg.DisplayWidth, cfg.DisplayHeight, cfg.CdnDomain)

	// Setup routes
	mux := http.NewServeMux()

	// Public routes
	mux.HandleFunc("/", promptHandler.HandleRoot)
	mux.HandleFunc("/login", promptHandler.HandleLogin)
	mux.HandleFunc("/logout", promptHandler.HandleLogout)
	mux.HandleFunc("/image", imgHandler.ServeImage)
	mux.HandleFunc("/image/original", imgHandler.ServeOriginal)
	mux.HandleFunc("/firmware", imgHandler.ServeFirmware)

	// Protected routes (behind auth middleware)
	mux.Handle("/prompt", authMgr.Middleware(http.HandlerFunc(promptHandler.HandlePrompt)))
	mux.Handle("/firmware/upload", authMgr.Middleware(http.HandlerFunc(promptHandler.HandleFirmwareUpload)))

	// Logging middleware
	loggedMux := loggingMiddleware(mux)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("🖼  E-Paper Studio starting on http://localhost%s", addr)
	log.Printf("   Provider: %s", prov.Name())
	log.Printf("   Dither mode: %s", cfg.DitherMode)
	log.Printf("   Display: %dx%d", cfg.DisplayWidth, cfg.DisplayHeight)
	log.Printf("   Rate limit: %d/day", cfg.DailyRateLimit)
	log.Printf("   Data dir: %s", cfg.DataDir)

	if err := http.ListenAndServe(addr, loggedMux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// loggingMiddleware logs each HTTP request.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}
