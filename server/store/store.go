package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ImageStore manages the current dithered image on disk
// and provides ETag-based caching for the ESP32.
type ImageStore struct {
	dataDir   string
	etag      string
	updatedAt time.Time
	mu        sync.RWMutex
}

// NewImageStore creates a new image store and ensures the data directory exists.
func NewImageStore(dataDir string) (*ImageStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	s := &ImageStore{dataDir: dataDir}

	// Load existing image and compute ETag if present
	if data, err := os.ReadFile(s.imagePath()); err == nil {
		s.etag = computeETag(data)
		if info, err := os.Stat(s.imagePath()); err == nil {
			s.updatedAt = info.ModTime()
		}
	}

	return s, nil
}

// Save stores the dithered BMP image and updates the ETag.
func (s *ImageStore) Save(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.WriteFile(s.imagePath(), data, 0644); err != nil {
		return fmt.Errorf("failed to save image: %w", err)
	}

	s.etag = computeETag(data)
	s.updatedAt = time.Now()

	return nil
}

// SaveOriginal stores the original (pre-dithered) image for preview purposes.
func (s *ImageStore) SaveOriginal(data []byte, contentType string) error {
	ext := ".png"
	if contentType == "image/jpeg" {
		ext = ".jpg"
	}

	path := filepath.Join(s.dataDir, "original"+ext)
	return os.WriteFile(path, data, 0644)
}

// Load returns the current dithered BMP image data.
func (s *ImageStore) Load() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return os.ReadFile(s.imagePath())
}

// LoadOriginal returns the original image data for preview.
func (s *ImageStore) LoadOriginal() ([]byte, string, error) {
	// Try PNG first, then JPEG
	for _, ext := range []string{".png", ".jpg"} {
		path := filepath.Join(s.dataDir, "original"+ext)
		if data, err := os.ReadFile(path); err == nil {
			contentType := "image/png"
			if ext == ".jpg" {
				contentType = "image/jpeg"
			}
			return data, contentType, nil
		}
	}
	return nil, "", fmt.Errorf("no original image found")
}

// ETag returns the current ETag for cache validation.
func (s *ImageStore) ETag() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.etag
}

// HasImage returns true if an image has been stored.
func (s *ImageStore) HasImage() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, err := os.Stat(s.imagePath())
	return err == nil
}

// UpdatedAt returns when the image was last updated.
func (s *ImageStore) UpdatedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.updatedAt
}

func (s *ImageStore) imagePath() string {
	return filepath.Join(s.dataDir, "current.bmp")
}

func computeETag(data []byte) string {
	hash := sha256.Sum256(data)
	return `"` + hex.EncodeToString(hash[:8]) + `"`
}
