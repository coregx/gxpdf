package gxpdf_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/coregx/gxpdf"
	"github.com/coregx/gxpdf/creator"
)

func TestCreatorImagesRoundTripThroughPublicExtraction(t *testing.T) {
	opaque := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	opaque.SetNRGBA(0, 0, color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xff})
	opaque.SetNRGBA(1, 0, color.NRGBA{R: 0x78, G: 0x9a, B: 0xbc, A: 0xff})
	opaque.SetNRGBA(2, 0, color.NRGBA{R: 0xde, G: 0xf0, B: 0x11, A: 0xff})
	opaque.SetNRGBA(0, 1, color.NRGBA{R: 0x22, G: 0x44, B: 0x66, A: 0xff})
	opaque.SetNRGBA(1, 1, color.NRGBA{R: 0x88, G: 0xaa, B: 0xcc, A: 0xff})
	opaque.SetNRGBA(2, 1, color.NRGBA{R: 0xee, G: 0x10, B: 0x32, A: 0xff})

	var pngSource bytes.Buffer
	if err := png.Encode(&pngSource, opaque); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	var jpegSource bytes.Buffer
	if err := jpeg.Encode(&jpegSource, opaque, &jpeg.Options{Quality: 91}); err != nil {
		t.Fatalf("encode JPEG: %v", err)
	}

	tests := []struct {
		name       string
		source     []byte
		filter     string
		verifyData func(*testing.T, *gxpdf.Image)
	}{
		{
			name:   "opaque PNG uses FlateDecode",
			source: pngSource.Bytes(),
			filter: "/FlateDecode",
			verifyData: func(t *testing.T, extracted *gxpdf.Image) {
				t.Helper()
				converted, err := extracted.ToGoImage()
				if err != nil {
					t.Fatalf("ToGoImage() error = %v", err)
				}
				for y := range opaque.Bounds().Dy() {
					for x := range opaque.Bounds().Dx() {
						got := color.NRGBAModel.Convert(converted.At(x, y)).(color.NRGBA)
						want := opaque.NRGBAAt(x, y)
						if got != want {
							t.Fatalf("pixel (%d,%d) = %#v, want %#v", x, y, got, want)
						}
					}
				}
			},
		},
		{
			name:   "JPEG uses DCTDecode",
			source: jpegSource.Bytes(),
			filter: "/DCTDecode",
			verifyData: func(t *testing.T, extracted *gxpdf.Image) {
				t.Helper()
				path := filepath.Join(t.TempDir(), "roundtrip.jpg")
				if err := extracted.SaveToFile(path); err != nil {
					t.Fatalf("SaveToFile() error = %v", err)
				}
				saved, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read saved JPEG: %v", err)
				}
				if !bytes.Equal(saved, jpegSource.Bytes()) {
					t.Fatal("SaveToFile() changed the original JPEG payload")
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := openCreatorImageDocument(t, test.source)
			images, err := document.GetImagesWithError()
			if err != nil {
				t.Fatalf("GetImagesWithError() error = %v", err)
			}
			if len(images) != 1 {
				t.Fatalf("GetImagesWithError() returned %d images, want 1", len(images))
			}

			extracted := images[0]
			if extracted.Width() != 3 || extracted.Height() != 2 {
				t.Fatalf("dimensions = %dx%d, want 3x2", extracted.Width(), extracted.Height())
			}
			if extracted.ColorSpace() != "DeviceRGB" {
				t.Fatalf("ColorSpace() = %q, want DeviceRGB", extracted.ColorSpace())
			}
			if extracted.BitsPerComponent() != 8 {
				t.Fatalf("BitsPerComponent() = %d, want 8", extracted.BitsPerComponent())
			}
			if extracted.Filter() != test.filter {
				t.Fatalf("Filter() = %q, want %q", extracted.Filter(), test.filter)
			}
			if extracted.Name() != "Im1" {
				t.Fatalf("Name() = %q, want Im1", extracted.Name())
			}
			test.verifyData(t, extracted)
		})
	}
}

func openCreatorImageDocument(t *testing.T, source []byte) *gxpdf.Document {
	t.Helper()

	embedded, err := creator.LoadImageFromReader(bytes.NewReader(source))
	if err != nil {
		t.Fatalf("LoadImageFromReader() error = %v", err)
	}
	document := creator.New()
	page, err := document.NewPage()
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	if err := page.DrawImage(embedded, 24, 36, float64(embedded.Width()), float64(embedded.Height())); err != nil {
		t.Fatalf("DrawImage() error = %v", err)
	}
	pdf, err := document.Bytes()
	if err != nil {
		t.Fatalf("Bytes() error = %v", err)
	}

	parsed, err := gxpdf.OpenFromBytes(pdf)
	if err != nil {
		t.Fatalf("OpenFromBytes() error = %v", err)
	}
	t.Cleanup(func() { _ = parsed.Close() })
	return parsed
}
