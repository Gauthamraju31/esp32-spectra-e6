package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// StubProvider fetches a random image from picsum.photos for testing.
// Useful for testing the pipeline without an external AI image generation API.
type StubProvider struct {
	width  int
	height int
}

func NewStubProvider(width, height int) *StubProvider {
	return &StubProvider{width: width, height: height}
}

func (p *StubProvider) Name() string {
	return "Stub (Picsum Photos)"
}

func (p *StubProvider) Generate(ctx context.Context, prompt string) ([]byte, string, error) {
	// Use dimensions from context if provided (accounts for orientation),
	// otherwise fall back to the configured display dimensions.
	width, height := p.width, p.height
	if dims, ok := ctx.Value(ImageDimsKey{}).(ImageDims); ok {
		width, height = dims.Width, dims.Height
	}
	url := fmt.Sprintf("https://picsum.photos/%d/%d", width, height)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetching image from picsum.photos: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("picsum.photos returned status %d", resp.StatusCode)
	}

	imgData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("reading image data: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	return imgData, contentType, nil
}
