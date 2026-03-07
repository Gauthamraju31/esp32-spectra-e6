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

// ImageStore defines the interface for storing and retrieving images.
type ImageStore interface {
	Save(data []byte) error
	SaveOriginal(data []byte, contentType string) error
	Load() ([]byte, error)
	LoadOriginal() ([]byte, string, error)
	ETag() string
	HasImage() bool
	UpdatedAt() time.Time

	SaveFirmware(data []byte) error
	LoadFirmware() ([]byte, error)
	FirmwareETag() string
	HasFirmware() bool
	FirmwareUpdatedAt() time.Time
}

// DiskStore manages the current dithered image on disk
// and provides ETag-based caching for the ESP32.
type DiskStore struct {
	dataDir           string
	etag              string
	updatedAt         time.Time
	firmwareEtag      string
	firmwareUpdatedAt time.Time
	mu                sync.RWMutex
}

// NewDiskStore creates a new disk store and ensures the data directory exists.
func NewDiskStore(dataDir string) (*DiskStore, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	s := &DiskStore{dataDir: dataDir}

	// Load existing image and compute ETag if present
	if data, err := os.ReadFile(s.imagePath()); err == nil {
		s.etag = computeETag(data)
		if info, err := os.Stat(s.imagePath()); err == nil {
			s.updatedAt = info.ModTime()
		}
	}

	// Load existing firmware and compute ETag if present
	if data, err := os.ReadFile(s.firmwarePath()); err == nil {
		s.firmwareEtag = computeETag(data)
		if info, err := os.Stat(s.firmwarePath()); err == nil {
			s.firmwareUpdatedAt = info.ModTime()
		}
	}

	return s, nil
}

// Save stores the dithered BMP image and updates the ETag.
func (s *DiskStore) Save(data []byte) error {
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
func (s *DiskStore) SaveOriginal(data []byte, contentType string) error {
	ext := ".png"
	if contentType == "image/jpeg" {
		ext = ".jpg"
	}

	path := filepath.Join(s.dataDir, "original"+ext)
	return os.WriteFile(path, data, 0644)
}

// Load returns the current dithered BMP image data.
func (s *DiskStore) Load() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return os.ReadFile(s.imagePath())
}

// LoadOriginal returns the original image data for preview.
func (s *DiskStore) LoadOriginal() ([]byte, string, error) {
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
func (s *DiskStore) ETag() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.etag
}

// HasImage returns true if an image has been stored.
func (s *DiskStore) HasImage() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, err := os.Stat(s.imagePath())
	return err == nil
}

func (s *DiskStore) UpdatedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.updatedAt
}

func (s *DiskStore) SaveFirmware(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.WriteFile(s.firmwarePath(), data, 0644); err != nil {
		return fmt.Errorf("failed to save firmware: %w", err)
	}

	s.firmwareEtag = computeETag(data)
	s.firmwareUpdatedAt = time.Now()

	return nil
}

func (s *DiskStore) LoadFirmware() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return os.ReadFile(s.firmwarePath())
}

func (s *DiskStore) FirmwareETag() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.firmwareEtag
}

func (s *DiskStore) HasFirmware() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, err := os.Stat(s.firmwarePath())
	return err == nil
}

func (s *DiskStore) FirmwareUpdatedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.firmwareUpdatedAt
}

func (s *DiskStore) imagePath() string {
	return filepath.Join(s.dataDir, "current.bmp")
}

func (s *DiskStore) firmwarePath() string {
	return filepath.Join(s.dataDir, "current.bin")
}

func computeETag(data []byte) string {
	hash := sha256.Sum256(data)
	return `"` + hex.EncodeToString(hash[:8]) + `"`
}
