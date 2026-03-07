package handler

import (
	"fmt"
	"net/http"

	"github.com/Gauthamraju31/esp32-spectra-e6/server/store"
)

// ImageHandler serves the dithered BMP image for the ESP32 to download.
type ImageHandler struct {
	store store.ImageStore
}

// NewImageHandler creates a new image handler.
func NewImageHandler(s store.ImageStore) *ImageHandler {
	return &ImageHandler{store: s}
}

// ServeImage handles GET /image — serves the current dithered BMP
// with ETag support for efficient caching by the ESP32.
func (h *ImageHandler) ServeImage(w http.ResponseWriter, r *http.Request) {
	if !h.store.HasImage() {
		http.Error(w, "No image available yet", http.StatusNotFound)
		return
	}

	// ETag-based caching
	etag := h.store.ETag()
	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	data, err := h.store.Load()
	if err != nil {
		http.Error(w, "Failed to load image", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/bmp")
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.Write(data)
}

// ServeOriginal handles GET /image/original — serves the original image for preview.
func (h *ImageHandler) ServeOriginal(w http.ResponseWriter, r *http.Request) {
	data, contentType, err := h.store.LoadOriginal()
	if err != nil {
		http.Error(w, "No original image available", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(data)
}

// ServeFirmware handles GET /firmware — serves the current firmware bin.
func (h *ImageHandler) ServeFirmware(w http.ResponseWriter, r *http.Request) {
	if !h.store.HasFirmware() {
		http.Error(w, "No firmware available yet", http.StatusNotFound)
		return
	}

	etag := h.store.FirmwareETag()
	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	data, err := h.store.LoadFirmware()
	if err != nil {
		http.Error(w, "Failed to load firmware", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.Write(data)
}
