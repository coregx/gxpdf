package creator

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // Import JPEG decoder
	_ "image/png"  // Import PNG decoder
	"io"
	"os"

	"github.com/coregx/gxpdf/internal/encoding"
)

// Image represents an image that can be embedded in a PDF document.
//
// Currently supports:
//   - JPEG images: grayscale, RGB (YCbCr) and CMYK (Adobe CMYK/YCCK included)
//   - PNG images: RGB, RGBA, grayscale, paletted, 8 and 16 bit
//
// The image data is stored as:
//   - JPEG: the original bytes (DCTDecode), color space from the component count
//   - PNG: 8-bit samples compressed with FlateDecode (DeviceGray or DeviceRGB)
//
// Any PNG transparency (alpha channel, or tRNS in a paletted, grey or RGB PNG)
// is stored separately as an 8-bit SMask (soft mask).
//
// Example:
//
//	img, err := creator.LoadImage("photo.jpg")  // or "photo.png"
//	if err != nil {
//	    return err
//	}
//	page.DrawImage(img, 100, 500, 200, 150)
type Image struct {
	// Image format (jpeg or png).
	format string

	// Raw image data (JPEG bytes or compressed PNG pixels).
	data []byte

	// Alpha mask data for RGBA PNG (compressed with FlateDecode).
	alphaMask []byte

	// Image dimensions.
	width  int
	height int

	// Color space information.
	colorSpace ColorSpace

	// Components per pixel (1=Gray, 3=RGB, 4=CMYK).
	components int

	// Bits per component (8 for most images).
	bitsPerComponent int

	// decode is the image's /Decode array, nil for the default mapping.
	// Adobe CMYK JPEGs store inverted samples and need [1 0 1 0 1 0 1 0].
	decode []float64
}

// ColorSpace represents the image color space.
type ColorSpace string

const (
	// ColorSpaceRGB is RGB color space (3 components).
	ColorSpaceRGB ColorSpace = "DeviceRGB"

	// ColorSpaceCMYK is CMYK color space (4 components).
	ColorSpaceCMYK ColorSpace = "DeviceCMYK"

	// ColorSpaceGray is grayscale (1 component).
	ColorSpaceGray ColorSpace = "DeviceGray"
)

// LoadImage loads an image from a file.
//
// Supported formats: JPEG, PNG.
// For JPEG: grayscale, RGB and CMYK (1, 3 or 4 components).
// For PNG: RGB, RGBA, grayscale, paletted; transparency becomes an SMask.
//
// Example:
//
//	img, err := creator.LoadImage("photo.jpg")  // or "photo.png"
//	if err != nil {
//	    return err
//	}
func LoadImage(path string) (*Image, error) {
	//nolint:gosec // File path is provided by user, G304 false positive.
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open image file: %w", err)
	}
	defer func() {
		_ = file.Close() // Best effort cleanup.
	}()

	return LoadImageFromReader(file)
}

// LoadImageFromReader loads an image from an io.Reader.
//
// Supported formats: JPEG, PNG.
// This allows loading images from various sources (files, HTTP responses, etc.).
//
// Example:
//
//	resp, _ := http.Get("https://example.com/image.png")
//	defer resp.Body.Close()
//	img, _ := creator.LoadImageFromReader(resp.Body)
func LoadImageFromReader(r io.Reader) (*Image, error) {
	// Read all data first.
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	// Detect format.
	format := detectImageFormat(data)
	if format == "" {
		return nil, ErrUnsupportedImageFormat
	}

	// Load based on format.
	var img *Image
	switch format {
	case "jpeg":
		img, err = loadJPEG(data)
	case "png":
		img, err = loadPNG(data)
	default:
		return nil, fmt.Errorf("unsupported image format: %s", format)
	}
	if err != nil {
		return nil, err
	}
	// A JPEG frame header may declare a zero width or height (the decoder
	// accepts it); an image XObject needs at least one pixel.
	if img.width <= 0 || img.height <= 0 {
		return nil, fmt.Errorf("%w: %s is %dx%d pixels", ErrInvalidImageDimensions, format, img.width, img.height)
	}
	return img, nil
}

// detectImageFormat detects the image format by checking file header.
func detectImageFormat(data []byte) string {
	if len(data) < 8 {
		return ""
	}

	// Check JPEG signature (0xFF 0xD8 0xFF).
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "jpeg"
	}

	// Check PNG signature (0x89 PNG \r\n 0x1A \n).
	if data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' {
		return "png"
	}

	return ""
}

// loadJPEG loads a JPEG image from raw data. The bytes are embedded as they
// are (DCTDecode); the color space follows the number of components in the
// frame header: 1 = DeviceGray, 3 = DeviceRGB (YCbCr or RGB), 4 = DeviceCMYK.
func loadJPEG(data []byte) (*Image, error) {
	// Decode config to get dimensions (and to validate the stream).
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode JPEG: %w", err)
	}
	components, adobe, err := jpegComponents(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JPEG: %w", err)
	}

	img := &Image{
		format:           "jpeg",
		data:             data,
		width:            cfg.Width,
		height:           cfg.Height,
		components:       components,
		bitsPerComponent: 8,
	}
	switch components {
	case 1:
		img.colorSpace = ColorSpaceGray
	case 3:
		img.colorSpace = ColorSpaceRGB
	case 4:
		img.colorSpace = ColorSpaceCMYK
		if adobe {
			// Adobe applications write CMYK (and YCCK) JPEGs with inverted
			// samples; the APP14 marker identifies them. Same rule as
			// libtexpdf; krilla inverts every CMYK JPEG.
			img.decode = []float64{1, 0, 1, 0, 1, 0, 1, 0}
		}
	default:
		return nil, fmt.Errorf("unsupported JPEG with %d color components", components)
	}
	return img, nil
}

// jpegComponents scans the JPEG markers up to the frame header and returns its
// number of color components and whether an Adobe APP14 marker precedes it.
func jpegComponents(data []byte) (components int, adobe bool, err error) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 0, false, errors.New("missing SOI marker")
	}
	for pos := 2; pos+4 <= len(data); {
		if data[pos] != 0xFF {
			return 0, false, fmt.Errorf("expected a marker at offset %d", pos)
		}
		marker := data[pos+1]
		switch {
		case marker == 0xFF: // fill byte
			pos++
			continue
		case marker == 0xD8 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7):
			pos += 2 // markers without a length
			continue
		}
		length := int(data[pos+2])<<8 | int(data[pos+3])
		if length < 2 || pos+2+length > len(data) {
			return 0, false, fmt.Errorf("truncated segment 0x%02X at offset %d", marker, pos)
		}
		seg := data[pos+4 : pos+2+length]
		switch {
		case marker == 0xEE && len(seg) >= 5 && string(seg[:5]) == "Adobe":
			adobe = true
		case marker >= 0xC0 && marker <= 0xCF && marker != 0xC4 && marker != 0xC8 && marker != 0xCC:
			// SOFn: precision, height, width, component count.
			if len(seg) < 6 {
				return 0, false, errors.New("truncated frame header")
			}
			return int(seg[5]), adobe, nil
		}
		pos += 2 + length
	}
	return 0, false, errors.New("no frame header")
}

// loadPNG loads a PNG image from raw data.
func loadPNG(data []byte) (*Image, error) {
	// Decode the full PNG image.
	img, err := decodePNGImage(data)
	if err != nil {
		return nil, err
	}

	// Convert PNG to raw pixel data.
	return convertPNGToImage(img)
}

// decodePNGImage decodes PNG data to an image.Image.
func decodePNGImage(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode PNG: %w", err)
	}
	return img, nil
}

// convertPNGToImage converts a decoded PNG into samples for an image XObject:
// DeviceGray for grayscale PNGs (8 or 16 bit), DeviceRGB otherwise, always
// 8 bits per component. Any transparency — an alpha channel, 16-bit alpha, or
// tRNS in a paletted, grey or RGB PNG — becomes an 8-bit SMask; an image with
// no translucent pixel gets none. Color samples are straight (not multiplied
// by alpha), as an SMask without /Matte requires.
func convertPNGToImage(img image.Image) (*Image, error) {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	m := img.ColorModel()
	gray := m == color.GrayModel || m == color.Gray16Model
	out := &Image{
		format:           "png",
		width:            width,
		height:           height,
		colorSpace:       ColorSpaceRGB,
		components:       3,
		bitsPerComponent: 8,
	}
	if gray {
		out.colorSpace, out.components = ColorSpaceGray, 1
	}

	samples := make([]byte, 0, width*height*out.components)
	var alpha []byte
	if o, ok := img.(interface{ Opaque() bool }); !ok || !o.Opaque() {
		alpha = make([]byte, 0, width*height)
	}
	translucent := false
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := straightNRGBA(img.At(x, y))
			if gray {
				samples = append(samples, c.R)
			} else {
				samples = append(samples, c.R, c.G, c.B)
			}
			if alpha != nil {
				alpha = append(alpha, c.A)
				translucent = translucent || c.A != 0xFF
			}
		}
	}

	var err error
	if out.data, err = compressData(samples); err != nil {
		return nil, fmt.Errorf("failed to compress PNG samples: %w", err)
	}
	if translucent {
		if out.alphaMask, err = compressData(alpha); err != nil {
			return nil, fmt.Errorf("failed to compress PNG alpha: %w", err)
		}
	}
	return out, nil
}

// straightNRGBA returns c with straight (non-premultiplied) 8-bit components.
// NRGBA colors (NRGBA images, tRNS palettes) are taken as they are, so even a
// fully transparent palette entry keeps its color; Color.RGBA() would return
// premultiplied values.
func straightNRGBA(c color.Color) color.NRGBA {
	switch v := c.(type) {
	case color.NRGBA:
		return v
	case color.NRGBA64:
		return color.NRGBA{R: uint8(v.R >> 8), G: uint8(v.G >> 8), B: uint8(v.B >> 8), A: uint8(v.A >> 8)}
	}
	return color.NRGBAModel.Convert(c).(color.NRGBA)
}

// compressData compresses data using FlateDecode.
func compressData(data []byte) ([]byte, error) {
	encoder := encoding.NewFlateDecoder()
	compressed, err := encoder.Encode(data)
	if err != nil {
		return nil, fmt.Errorf("flate compression failed: %w", err)
	}
	return compressed, nil
}

// Width returns the image width in pixels.
func (img *Image) Width() int {
	return img.width
}

// Height returns the image height in pixels.
func (img *Image) Height() int {
	return img.height
}

// Data returns the raw JPEG data.
//
// This is used internally by the PDF writer to embed the image.
func (img *Image) Data() []byte {
	return img.data
}

// ColorSpace returns the image color space.
func (img *Image) ColorSpace() ColorSpace {
	return img.colorSpace
}

// Format returns the image format (jpeg or png).
func (img *Image) Format() string {
	return img.format
}

// AlphaMask returns the alpha mask data (nil if no transparency).
//
// For RGBA PNG images with transparency, this contains the compressed
// alpha channel data used for the SMask (soft mask) in PDF.
func (img *Image) AlphaMask() []byte {
	return img.alphaMask
}

// HasAlpha returns true if the image has transparency data.
func (img *Image) HasAlpha() bool {
	return img.alphaMask != nil
}

// Components returns the number of color components.
//
// Returns:
//   - 1 for grayscale
//   - 3 for RGB
//   - 4 for CMYK
func (img *Image) Components() int {
	return img.components
}

// BitsPerComponent returns the bits per component (typically 8).
func (img *Image) BitsPerComponent() int {
	return img.bitsPerComponent
}

// DrawImage draws an image at the specified position and size.
//
// The image is scaled to fit the specified width and height.
// No aspect ratio preservation - the image is stretched to fit.
//
// Parameters:
//   - img: The image to draw
//   - x: Horizontal position in points (from left edge)
//   - y: Vertical position in points (from bottom edge)
//   - width: Display width in points
//   - height: Display height in points
//
// Example:
//
//	img, _ := creator.LoadImage("photo.jpg")
//	page.DrawImage(img, 100, 500, 200, 150)
func (p *Page) DrawImage(img *Image, x, y, width, height float64) error {
	// Validate dimensions.
	if width <= 0 || height <= 0 {
		return errors.New("image dimensions must be positive")
	}

	// Store image operation.
	p.graphicsOps = append(p.graphicsOps, GraphicsOperation{
		Type:   GraphicsOpImage,
		X:      x,
		Y:      y,
		Width:  width,
		Height: height,
		Image:  img,
	})

	return nil
}

// DrawImageFit draws an image scaled to fit within the specified dimensions.
//
// The image is scaled to fit within the width/height while maintaining
// its aspect ratio. The image is centered in the available space.
//
// Parameters:
//   - img: The image to draw
//   - x: Horizontal position in points (from left edge)
//   - y: Vertical position in points (from bottom edge)
//   - maxWidth: Maximum width in points
//   - maxHeight: Maximum height in points
//
// Example:
//
//	img, _ := creator.LoadImage("photo.jpg")
//	page.DrawImageFit(img, 100, 500, 200, 200)  // Fit in 200x200 box
func (p *Page) DrawImageFit(img *Image, x, y, maxWidth, maxHeight float64) error {
	// Validate dimensions.
	if maxWidth <= 0 || maxHeight <= 0 {
		return errors.New("image max dimensions must be positive")
	}

	// Calculate scaled dimensions.
	scaledW, scaledH := calculateFitDimensions(
		float64(img.width),
		float64(img.height),
		maxWidth,
		maxHeight,
	)

	// Center the image in available space.
	centerX := x + (maxWidth-scaledW)/2
	centerY := y + (maxHeight-scaledH)/2

	return p.DrawImage(img, centerX, centerY, scaledW, scaledH)
}

// calculateFitDimensions calculates dimensions to fit within max bounds.
//
// Maintains aspect ratio by scaling down the larger dimension.
func calculateFitDimensions(imgW, imgH, maxW, maxH float64) (float64, float64) {
	// Calculate scale factors for width and height.
	scaleW := maxW / imgW
	scaleH := maxH / imgH

	// Use the smaller scale factor (to fit within both constraints).
	scale := scaleW
	if scaleH < scaleW {
		scale = scaleH
	}

	return imgW * scale, imgH * scale
}

// Errors.
var (
	// ErrUnsupportedImageFormat is returned for unsupported image formats.
	ErrUnsupportedImageFormat = errors.New("unsupported image format (supported: JPEG, PNG)")

	// ErrInvalidImageDimensions is returned for zero/negative dimensions.
	ErrInvalidImageDimensions = errors.New("image dimensions must be positive")
)
