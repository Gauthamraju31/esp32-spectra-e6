package dither

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
)

// Palette defines the 6 colors used by the Spectra E6 e-paper display.
var Palette = []color.RGBA{
	{0x00, 0x00, 0x00, 0xFF}, // 0: Black
	{0xFF, 0xFF, 0xFF, 0xFF}, // 1: White
	{0xE6, 0xE6, 0x00, 0xFF}, // 2: Yellow
	{0xCC, 0x00, 0x00, 0xFF}, // 3: Red
	{0x00, 0x33, 0xCC, 0xFF}, // 4: Blue
	{0x00, 0xCC, 0x00, 0xFF}, // 5: Green
}

// Ditherer handles image dithering for e-paper displays.
type Ditherer struct {
	remoteURL string
	mode      string // "local", "remote", "local_with_fallback"
	width     int
	height    int
}

// NewDitherer creates a new Ditherer.
func NewDitherer(remoteURL, mode string, width, height int) *Ditherer {
	return &Ditherer{
		remoteURL: remoteURL,
		mode:      mode,
		width:     width,
		height:    height,
	}
}

// Process dithers the given image data using the default display dimensions.
// Returns an 8-bit indexed BMP suitable for the ESP32.
func (d *Ditherer) Process(imgData []byte) ([]byte, error) {
	return d.ProcessWithSize(imgData, d.width, d.height)
}

// ProcessWithSize dithers the given image data at the specified dimensions.
// Use this to switch between landscape (600×400) and portrait (400×600).
func (d *Ditherer) ProcessWithSize(imgData []byte, width, height int) ([]byte, error) {
	switch d.mode {
	case "remote":
		return d.processRemoteWithSize(imgData, width, height)
	case "local":
		return d.processLocalWithSize(imgData, width, height)
	case "local_with_fallback":
		result, err := d.processLocalWithSize(imgData, width, height)
		if err != nil {
			fmt.Printf("Local dithering failed, falling back to remote: %v\n", err)
			return d.processRemoteWithSize(imgData, width, height)
		}
		return result, nil
	default:
		return d.processLocalWithSize(imgData, width, height)
	}
}

// processLocalWithSize performs Floyd-Steinberg dithering locally in Go.
func (d *Ditherer) processLocalWithSize(imgData []byte, width, height int) ([]byte, error) {
	// Decode the input image
	src, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Scale the image to display dimensions
	scaled := scaleImage(src, width, height)

	// Apply Floyd-Steinberg dithering with our 6-color palette
	dithered := floydSteinberg(scaled, width, height)

	// Encode as 8-bit indexed BMP
	return encodeBMP(dithered, width, height)
}

// processRemoteWithSize sends the image to the external dithering service.
// It uploads the raw image bytes and gets back a dithered BMP.
func (d *Ditherer) processRemoteWithSize(imgData []byte, width, height int) ([]byte, error) {
	params := url.Values{}
	params.Set("width", strconv.Itoa(width))
	params.Set("height", strconv.Itoa(height))
	params.Set("dither", "true")
	params.Set("normalize", "false")
	params.Set("colors", "000000,ffffff,e6e600,cc0000,0033cc,00cc00")

	reqURL := fmt.Sprintf("%s/process?%s", d.remoteURL, params.Encode())

	req, err := http.NewRequest("POST", reqURL, bytes.NewReader(imgData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remote dithering request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("remote dithering failed (status %d): %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// scaleImage resizes an image to fit within the target dimensions using
// bilinear interpolation while maintaining aspect ratio and centering.
func scaleImage(src image.Image, targetW, targetH int) *image.RGBA {
	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	// Calculate scale to fill the target (cover mode)
	scaleX := float64(targetW) / float64(srcW)
	scaleY := float64(targetH) / float64(srcH)
	scale := math.Max(scaleX, scaleY)

	newW := int(float64(srcW) * scale)
	newH := int(float64(srcH) * scale)

	// Center crop offsets
	offsetX := (newW - targetW) / 2
	offsetY := (newH - targetH) / 2

	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))

	for y := 0; y < targetH; y++ {
		for x := 0; x < targetW; x++ {
			// Map back to source coordinates
			srcX := float64(x+offsetX) / scale
			srcY := float64(y+offsetY) / scale

			// Bilinear interpolation
			c := bilinearSample(src, srcX, srcY)
			dst.SetRGBA(x, y, c)
		}
	}

	return dst
}

// bilinearSample performs bilinear interpolation at the given source coordinates.
func bilinearSample(src image.Image, x, y float64) color.RGBA {
	bounds := src.Bounds()
	maxX := bounds.Max.X - 1
	maxY := bounds.Max.Y - 1

	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))
	x1 := x0 + 1
	y1 := y0 + 1

	// Clamp to bounds
	x0 = clamp(x0, bounds.Min.X, maxX)
	y0 = clamp(y0, bounds.Min.Y, maxY)
	x1 = clamp(x1, bounds.Min.X, maxX)
	y1 = clamp(y1, bounds.Min.Y, maxY)

	fx := x - math.Floor(x)
	fy := y - math.Floor(y)

	r00, g00, b00, _ := src.At(x0, y0).RGBA()
	r01, g01, b01, _ := src.At(x0, y1).RGBA()
	r10, g10, b10, _ := src.At(x1, y0).RGBA()
	r11, g11, b11, _ := src.At(x1, y1).RGBA()

	r := bilerp(float64(r00), float64(r10), float64(r01), float64(r11), fx, fy)
	g := bilerp(float64(g00), float64(g10), float64(g01), float64(g11), fx, fy)
	b := bilerp(float64(b00), float64(b10), float64(b01), float64(b11), fx, fy)

	return color.RGBA{
		R: uint8(r / 256),
		G: uint8(g / 256),
		B: uint8(b / 256),
		A: 0xFF,
	}
}

func bilerp(v00, v10, v01, v11, fx, fy float64) float64 {
	return v00*(1-fx)*(1-fy) + v10*fx*(1-fy) + v01*(1-fx)*fy + v11*fx*fy
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// floydSteinberg applies Floyd-Steinberg dithering using the Spectra E6 palette.
// Returns a 2D slice of palette indices.
func floydSteinberg(img *image.RGBA, width, height int) [][]uint8 {
	// Create error diffusion buffers (using float64 for precision)
	type pixel struct{ r, g, b float64 }

	buf := make([][]pixel, height)
	for y := 0; y < height; y++ {
		buf[y] = make([]pixel, width)
		for x := 0; x < width; x++ {
			c := img.RGBAAt(x, y)
			buf[y][x] = pixel{float64(c.R), float64(c.G), float64(c.B)}
		}
	}

	result := make([][]uint8, height)
	for y := 0; y < height; y++ {
		result[y] = make([]uint8, width)
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			old := buf[y][x]

			// Clamp values
			old.r = math.Max(0, math.Min(255, old.r))
			old.g = math.Max(0, math.Min(255, old.g))
			old.b = math.Max(0, math.Min(255, old.b))

			// Find closest palette color
			bestIdx := 0
			bestDist := math.MaxFloat64
			for i, pc := range Palette {
				dr := old.r - float64(pc.R)
				dg := old.g - float64(pc.G)
				db := old.b - float64(pc.B)
				dist := dr*dr + dg*dg + db*db
				if dist < bestDist {
					bestDist = dist
					bestIdx = i
				}
			}

			result[y][x] = uint8(bestIdx)

			// Calculate error
			chosen := Palette[bestIdx]
			errR := old.r - float64(chosen.R)
			errG := old.g - float64(chosen.G)
			errB := old.b - float64(chosen.B)

			// Distribute error
			diffuse := func(dx, dy int, factor float64) {
				nx, ny := x+dx, y+dy
				if nx >= 0 && nx < width && ny >= 0 && ny < height {
					buf[ny][nx].r += errR * factor
					buf[ny][nx].g += errG * factor
					buf[ny][nx].b += errB * factor
				}
			}

			diffuse(1, 0, 7.0/16.0)
			diffuse(-1, 1, 3.0/16.0)
			diffuse(0, 1, 5.0/16.0)
			diffuse(1, 1, 1.0/16.0)
		}
	}

	return result
}

// encodeBMP creates an 8-bit indexed BMP file from palette indices.
// This matches the format expected by the ESP32 firmware.
func encodeBMP(indices [][]uint8, width, height int) ([]byte, error) {
	rowSize := ((width*8 + 31) / 32) * 4 // Row size padded to 4-byte boundary
	paletteSize := 256 * 4                // 256 colors × 4 bytes (BGRA)
	dataOffset := 54 + paletteSize        // BMP header + palette
	imageSize := rowSize * height
	fileSize := dataOffset + imageSize

	var buf bytes.Buffer
	buf.Grow(fileSize)

	// BMP File Header (14 bytes)
	buf.Write([]byte{'B', 'M'})
	binary.Write(&buf, binary.LittleEndian, uint32(fileSize))
	binary.Write(&buf, binary.LittleEndian, uint16(0)) // Reserved
	binary.Write(&buf, binary.LittleEndian, uint16(0)) // Reserved
	binary.Write(&buf, binary.LittleEndian, uint32(dataOffset))

	// DIB Header (BITMAPINFOHEADER - 40 bytes)
	binary.Write(&buf, binary.LittleEndian, uint32(40))     // Header size
	binary.Write(&buf, binary.LittleEndian, int32(width))    // Width
	binary.Write(&buf, binary.LittleEndian, int32(height))   // Height (positive = bottom-up)
	binary.Write(&buf, binary.LittleEndian, uint16(1))       // Color planes
	binary.Write(&buf, binary.LittleEndian, uint16(8))       // Bits per pixel
	binary.Write(&buf, binary.LittleEndian, uint32(0))       // Compression (none)
	binary.Write(&buf, binary.LittleEndian, uint32(imageSize))
	binary.Write(&buf, binary.LittleEndian, int32(2835))     // H resolution (72 DPI)
	binary.Write(&buf, binary.LittleEndian, int32(2835))     // V resolution
	binary.Write(&buf, binary.LittleEndian, uint32(6))       // Colors used
	binary.Write(&buf, binary.LittleEndian, uint32(6))       // Important colors

	// Color Palette (256 entries × 4 bytes BGRA)
	for i := 0; i < 256; i++ {
		if i < len(Palette) {
			c := Palette[i]
			buf.Write([]byte{c.B, c.G, c.R, 0x00}) // BMP uses BGRA order
		} else {
			buf.Write([]byte{0x00, 0x00, 0x00, 0x00})
		}
	}

	// Pixel Data (bottom-up row order, as expected by BMP format)
	row := make([]byte, rowSize)
	for y := height - 1; y >= 0; y-- {
		// Clear row (includes padding)
		for i := range row {
			row[i] = 0
		}
		for x := 0; x < width; x++ {
			row[x] = indices[y][x]
		}
		buf.Write(row)
	}

	return buf.Bytes(), nil
}
