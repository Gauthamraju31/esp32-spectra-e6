package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// NanoBananaProvider generates images using the NanoBanana API.
type NanoBananaProvider struct {
	apiKey  string
	baseURL string
}

// NewNanoBananaProvider creates a new NanoBanana image generation provider.
func NewNanoBananaProvider(apiKey, baseURL string) *NanoBananaProvider {
	return &NanoBananaProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
	}
}

func (n *NanoBananaProvider) Name() string {
	return "NanoBanana"
}

type nanoBananaResponse struct {
	ImageURL string `json:"image_url"`
	Error    string `json:"error,omitempty"`
}

func (n *NanoBananaProvider) Generate(ctx context.Context, prompt string) ([]byte, string, error) {
	// Build multipart form request
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if err := writer.WriteField("prompt", prompt); err != nil {
		return nil, "", fmt.Errorf("failed to write prompt field: %w", err)
	}
	if err := writer.WriteField("width", "1024"); err != nil {
		return nil, "", fmt.Errorf("failed to write width field: %w", err)
	}
	if err := writer.WriteField("height", "1024"); err != nil {
		return nil, "", fmt.Errorf("failed to write height field: %w", err)
	}
	writer.Close()

	url := fmt.Sprintf("%s/api/v1/generate", n.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, &buf)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+n.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("NanoBanana API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Try to parse as JSON with image_url
	var apiResp nanoBananaResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		// If not JSON, assume the response body is the image itself
		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "image/png"
		}
		return body, contentType, nil
	}

	if apiResp.Error != "" {
		return nil, "", fmt.Errorf("NanoBanana API error: %s", apiResp.Error)
	}

	if apiResp.ImageURL == "" {
		return nil, "", fmt.Errorf("no image URL in NanoBanana response")
	}

	// Download the image
	imgResp, err := http.Get(apiResp.ImageURL)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download generated image: %w", err)
	}
	defer imgResp.Body.Close()

	imgData, err := io.ReadAll(imgResp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read image data: %w", err)
	}

	contentType := imgResp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/png"
	}

	return imgData, contentType, nil
}
