package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Store manages images in an S3-compatible object store (like Cloudflare R2).
type S3Store struct {
	client            *s3.Client
	bucket            string
	etag              string
	updatedAt         time.Time
	firmwareEtag      string
	firmwareUpdatedAt time.Time
	mu                sync.RWMutex
}

// NewS3Store initializes an S3Store with the provided endpoints and credentials.
// Useful for Cloudflare R2 where the endpoint URL points to your specific account hash.
func NewS3Store(endpoint, accessKey, secretKey, bucket string) (*S3Store, error) {
	ctx := context.TODO()

	// Load default config with static credentials and 'auto' region for R2
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("auto"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load S3 config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})
	s := &S3Store{
		client: client,
		bucket: bucket,
	}

	// Fetch current metadata on startup to populate ETag
	s.refreshMetadata(ctx)

	return s, nil
}

func (s *S3Store) Save(data []byte) error {
	ctx := context.TODO()

	hash := sha256.Sum256(data)
	computedETag := `"` + hex.EncodeToString(hash[:8]) + `"`

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String("image/current.bmp"),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("image/bmp"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload image to S3: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.etag = computedETag
	s.updatedAt = time.Now()

	return nil
}

func (s *S3Store) SaveOriginal(data []byte, contentType string) error {
	ctx := context.TODO()
	ext := ".png"
	if contentType == "image/jpeg" {
		ext = ".jpg"
	}

	key := "image/original" + ext

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("failed to upload original to S3: %w", err)
	}
	return nil
}

func (s *S3Store) Load() ([]byte, error) {
	ctx := context.TODO()
	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String("image/current.bmp"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load image from S3: %w", err)
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (s *S3Store) LoadOriginal() ([]byte, string, error) {
	ctx := context.TODO()

	// Try PNG first, then JPEG
	for _, ext := range []string{".png", ".jpg"} {
		key := "image/original" + ext
		resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		})

		if err == nil {
			defer resp.Body.Close()
			data, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				return nil, "", fmt.Errorf("failed to read S3 object body: %w", readErr)
			}

			contentType := "image/png"
			if ext == ".jpg" {
				contentType = "image/jpeg"
			}
			return data, contentType, nil
		}
	}
	return nil, "", fmt.Errorf("no original image found in S3")
}

func (s *S3Store) ETag() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.etag
}

func (s *S3Store) HasImage() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.etag != ""
}

func (s *S3Store) UpdatedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.updatedAt
}

func (s *S3Store) SaveFirmware(data []byte) error {
	ctx := context.TODO()

	hash := sha256.Sum256(data)
	computedETag := `"` + hex.EncodeToString(hash[:8]) + `"`

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String("firmware/current.bin"),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/octet-stream"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload firmware to S3: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.firmwareEtag = computedETag
	s.firmwareUpdatedAt = time.Now()

	return nil
}

func (s *S3Store) LoadFirmware() ([]byte, error) {
	ctx := context.TODO()
	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String("firmware/current.bin"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load firmware from S3: %w", err)
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (s *S3Store) FirmwareETag() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.firmwareEtag
}

func (s *S3Store) HasFirmware() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.firmwareEtag != ""
}

func (s *S3Store) FirmwareUpdatedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.firmwareUpdatedAt
}

// refreshMetadata tries to fetch the HEAD of current.bmp and current.bin to initialize ETag/UpdatedAt on boot
func (s *S3Store) refreshMetadata(ctx context.Context) {
	resp, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String("image/current.bmp"),
	})
	if err == nil {
		s.mu.Lock()

		// If S3 returns an ETag, we could use it, but since we compute strong pseudo-ETags based on sha256,
		// and S3 ETags are sometimes MD5s or complicated multipart hashes, it's safer to just
		// pull the object and hash it if we really cared. For now, if the object exists,
		// we just mark it as existing with a dummy generated ETag until the next save.
		if resp.LastModified != nil {
			s.updatedAt = *resp.LastModified
		} else {
			s.updatedAt = time.Now()
		}

		// We could use resp.ETag directly if we want to rely on the S3 provider.
		if resp.ETag != nil {
			s.etag = *resp.ETag
		} else {
			s.etag = `"s3-cached"`
		}
		s.mu.Unlock()
	}

	respFirmware, errFirmware := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String("firmware/current.bin"),
	})
	if errFirmware == nil {
		s.mu.Lock()
		if respFirmware.LastModified != nil {
			s.firmwareUpdatedAt = *respFirmware.LastModified
		} else {
			s.firmwareUpdatedAt = time.Now()
		}
		if respFirmware.ETag != nil {
			s.firmwareEtag = *respFirmware.ETag
		} else {
			s.firmwareEtag = `"s3-cached"`
		}
		s.mu.Unlock()
	}
}
