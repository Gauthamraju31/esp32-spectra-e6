package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RunwareProvider interfaces with the Runware AI image generation REST API.
type RunwareProvider struct {
	apiKey  string
	modelID string
}

// NewRunwareProvider creates a new Runware provider.
func NewRunwareProvider(apiKey, modelID string) *RunwareProvider {
	return &RunwareProvider{apiKey: apiKey, modelID: modelID}
}

// Name returns the display name of the provider.
func (p *RunwareProvider) Name() string {
	return "Runware AI"
}

// runwareRequest represents the task array sent to Runware's API.
type runwareRequest struct {
	TaskType       string `json:"taskType"`
	TaskUUID       string `json:"taskUUID"`
	Model          string `json:"model"`
	PositivePrompt string `json:"positivePrompt"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	OutputType     string `json:"outputType"`
	OutputFormat   string `json:"outputFormat"`
	NumberResults  int    `json:"numberResults"`
}

// runwareResponse represents a single completed response object from Runware's array payload.
type runwareResponse struct {
	TaskUUID        string `json:"taskUUID"`
	ImageBase64Data string `json:"imageBase64Data"`
	Error           bool   `json:"error"`
	ErrorMessage    string `json:"errorMessage"`
}

// Generate requests an image from Runware using the requested dimensions.
func (p *RunwareProvider) Generate(ctx context.Context, prompt string) ([]byte, string, error) {
	// Respect requested dimensions if injected into context (e.g. for portrait mode)
	width := 600
	height := 400
	if dims, ok := ctx.Value(ImageDimsKey{}).(ImageDims); ok {
		width = dims.Width
		height = dims.Height
	}

	// RUNWARE REQUIREMENT:
	// "Image width must be an integer value between 128 and 2048, in multiples of '16'."
	// 600 is not a multiple of 16. We must round width and height UP to the nearest multiple of 16.
	// We pad to 608x400 (or 400x608). The `dither.scaleImage` algorithm will gracefully downscale
	// and center-crop the generated 608px image back down to perfectly fit the 600px e-paper array.
	width = roundUpToMultipleOf16(width)
	height = roundUpToMultipleOf16(height)

	taskUUID := uuid.New().String()

	// Build the JSON payload array expected by the Runware REST API
	reqPayload := []runwareRequest{
		{
			TaskType:       "imageInference",
			TaskUUID:       taskUUID,
			Model:          p.modelID,
			PositivePrompt: prompt,
			Width:          width,
			Height:         height,
			OutputType:     "base64Data",
			OutputFormat:   "jpg",
			NumberResults:  1,
		},
	}

	reqBody, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encode Runware request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.runware.ai/v1", bytes.NewReader(reqBody))
	if err != nil {
		return nil, "", fmt.Errorf("failed to create Runware request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	// Runware uses standard Bearer token auth via Authorization string
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("runware API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("runware API returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Parse JSON array response
	var runwareResps []runwareResponse
	if err := json.NewDecoder(resp.Body).Decode(&runwareResps); err != nil {
		return nil, "", fmt.Errorf("failed to decode runware response: %w", err)
	}

	if len(runwareResps) == 0 {
		return nil, "", fmt.Errorf("runware API returned an empty response array")
	}

	result := runwareResps[0]
	if result.Error {
		return nil, "", fmt.Errorf("runware API inference error: %s", result.ErrorMessage)
	}

	if result.ImageBase64Data == "" {
		return nil, "", fmt.Errorf("runware returned success but gave an empty base64 string")
	}

	// Extract the Base64 data (it may optionally contain a data URI prefix, we only want the block)
	b64String := result.ImageBase64Data
	if idx := strings.Index(b64String, ","); idx != -1 {
		b64String = b64String[idx+1:]
	}

	// Decode Base64 buffer
	imgData, err := base64.StdEncoding.DecodeString(b64String)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode base64 runware payload: %w", err)
	}

	return imgData, "image/jpeg", nil
}

// roundUpToMultipleOf16 ensures lengths fit Runware generation constraints.
func roundUpToMultipleOf16(v int) int {
	return (v + 15) &^ 15
}
