package provider

import "context"

// ImageProvider defines the interface for image generation services.
// Implementations should generate an image from a text prompt and return
// the raw image bytes (PNG or JPEG format).
type ImageProvider interface {
	// Name returns the human-readable name of the provider.
	Name() string

	// Generate creates an image from the given text prompt.
	// Returns the raw image bytes (PNG/JPEG) or an error.
	Generate(ctx context.Context, prompt string) ([]byte, string, error) // bytes, content-type, error
}

// contextKey is an unexported type for context keys in this package.
type contextKey struct{ name string }

// ImageDimsKey is the context key for desired image dimensions.
type ImageDimsKey struct{}

// ImageDims holds desired image dimensions.
type ImageDims struct {
	Width  int
	Height int
}

// WithImageDims returns a context with the desired image dimensions attached.
func WithImageDims(ctx context.Context, width, height int) context.Context {
	return context.WithValue(ctx, ImageDimsKey{}, ImageDims{Width: width, Height: height})
}

// GetImageDims retrieves image dimensions from the context, if present.
func GetImageDims(ctx context.Context) (ImageDims, bool) {
	dims, ok := ctx.Value(ImageDimsKey{}).(ImageDims)
	return dims, ok
}
