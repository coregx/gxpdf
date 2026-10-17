package creator

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// FuzzLoadImageFromReader feeds arbitrary bytes to the loader: it must return
// an image or an error, never panic, and a loaded image must be consistent.
func FuzzLoadImageFromReader(f *testing.F) {
	for _, n := range []int{1, 3, 4} {
		f.Add(headerJPEG(n, false, 0))
		f.Add(headerJPEG(n, true, 2))
	}
	var j bytes.Buffer
	_ = jpeg.Encode(&j, image.NewGray(image.Rect(0, 0, 3, 3)), nil)
	f.Add(j.Bytes())
	pal := image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.NRGBA{1, 2, 3, 0}, color.NRGBA{4, 5, 6, 255}})
	var p bytes.Buffer
	_ = png.Encode(&p, pal)
	f.Add(p.Bytes())

	f.Fuzz(func(t *testing.T, data []byte) {
		img, err := LoadImageFromReader(bytes.NewReader(data))
		if err != nil {
			return
		}
		if img.Width() <= 0 || img.Height() <= 0 {
			t.Fatalf("loaded %dx%d image", img.Width(), img.Height())
		}
		switch img.ColorSpace() {
		case ColorSpaceGray, ColorSpaceRGB, ColorSpaceCMYK:
		default:
			t.Fatalf("color space %q", img.ColorSpace())
		}
		want := map[ColorSpace]int{ColorSpaceGray: 1, ColorSpaceRGB: 3, ColorSpaceCMYK: 4}[img.ColorSpace()]
		if img.Components() != want {
			t.Fatalf("%s with %d components", img.ColorSpace(), img.Components())
		}
	})
}
