package creator

import (
	"bytes"
	"compress/zlib"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"testing"
)

// headerJPEG returns a JPEG made of markers only (SOI, optional Adobe APP14,
// SOF0 with n components, SOS, EOI): enough for the loader to read its
// geometry and color model. Go's encoder cannot write CMYK JPEGs.
func headerJPEG(n int, adobe bool, transform byte) []byte {
	b := []byte{0xFF, 0xD8}
	if adobe {
		b = append(b, 0xFF, 0xEE, 0x00, 0x0E, 'A', 'd', 'o', 'b', 'e', 0x00, 0x64, 0x00, 0x00, 0x00, 0x00, transform)
	}
	b = append(b, 0xFF, 0xC0, 0x00, byte(8+3*n), 8, 0x00, 0x02, 0x00, 0x03, byte(n))
	for i := 0; i < n; i++ {
		b = append(b, byte(i+1), 0x11, 0x00)
	}
	b = append(b, 0xFF, 0xDA, 0x00, byte(6+2*n), byte(n))
	for i := 0; i < n; i++ {
		b = append(b, byte(i+1), 0x00)
	}
	return append(b, 0x00, 0x3F, 0x00, 0xFF, 0xD9)
}

func loadBytes(t *testing.T, data []byte) *Image {
	t.Helper()
	img, err := LoadImageFromReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return img
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func inflateData(t *testing.T, data []byte) []byte {
	t.Helper()
	zr, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestLoadJPEG_ColorSpaceFromComponents(t *testing.T) {
	var gray bytes.Buffer
	if err := jpeg.Encode(&gray, image.NewGray(image.Rect(0, 0, 4, 2)), nil); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		data  []byte
		cs    ColorSpace
		comps int
	}{
		{"grayscale (Go encoder)", gray.Bytes(), ColorSpaceGray, 1},
		{"YCbCr", headerJPEG(3, false, 0), ColorSpaceRGB, 3},
		{"RGB with Adobe transform 0", headerJPEG(3, true, 0), ColorSpaceRGB, 3},
		{"Adobe CMYK (stored inverted)", headerJPEG(4, true, 0), ColorSpaceCMYK, 4},
		{"Adobe YCCK", headerJPEG(4, true, 2), ColorSpaceCMYK, 4},
		{"CMYK without Adobe marker", headerJPEG(4, false, 0), ColorSpaceCMYK, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := loadBytes(t, tt.data)
			if img.ColorSpace() != tt.cs || img.Components() != tt.comps {
				t.Errorf("got %s/%d, want %s/%d", img.ColorSpace(), img.Components(), tt.cs, tt.comps)
			}
			if !bytes.Equal(img.Data(), tt.data) {
				t.Error("JPEG bytes not kept as they are")
			}
		})
	}
}

func TestLoadPNG_AlphaAndColorSpace(t *testing.T) {
	r := image.Rect(0, 0, 2, 1)

	paletted := image.NewPaletted(r, color.Palette{color.NRGBA{255, 0, 0, 255}, color.NRGBA{0, 0, 255, 0}})
	paletted.SetColorIndex(1, 0, 1) // second pixel fully transparent

	n64 := image.NewNRGBA64(r)
	n64.SetNRGBA64(0, 0, color.NRGBA64{0xFFFF, 0, 0, 0xFFFF})
	n64.SetNRGBA64(1, 0, color.NRGBA64{0xFFFF, 0, 0, 0x8080})

	g16 := image.NewGray16(r)
	g16.SetGray16(0, 0, color.Gray16{0x8000})

	gray := image.NewGray(r)
	gray.SetGray(1, 0, color.Gray{200})

	opaque := image.NewNRGBA(r)
	for i := range opaque.Pix {
		opaque.Pix[i] = 255
	}

	tests := []struct {
		name  string
		img   image.Image
		cs    ColorSpace
		comps int
		alpha []byte // decoded SMask, nil = no SMask
		color []byte // decoded color samples
	}{
		{"paletted with transparent entry (tRNS)", paletted, ColorSpaceRGB, 3, []byte{255, 0}, []byte{255, 0, 0, 0, 0, 255}},
		{"16-bit with alpha", n64, ColorSpaceRGB, 3, []byte{255, 128}, []byte{255, 0, 0, 255, 0, 0}},
		{"16-bit grayscale", g16, ColorSpaceGray, 1, nil, []byte{128, 0}},
		{"8-bit grayscale", gray, ColorSpaceGray, 1, nil, []byte{0, 200}},
		{"opaque RGBA: no SMask", opaque, ColorSpaceRGB, 3, nil, []byte{255, 255, 255, 255, 255, 255}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := loadBytes(t, encodePNG(t, tt.img))
			if img.ColorSpace() != tt.cs || img.Components() != tt.comps {
				t.Errorf("got %s/%d, want %s/%d", img.ColorSpace(), img.Components(), tt.cs, tt.comps)
			}
			if got := inflateData(t, img.Data()); !bytes.Equal(got, tt.color) {
				t.Errorf("color samples %v, want %v", got, tt.color)
			}
			switch {
			case tt.alpha == nil && img.HasAlpha():
				t.Errorf("unexpected SMask %v", inflateData(t, img.AlphaMask()))
			case tt.alpha != nil && !img.HasAlpha():
				t.Error("transparency lost: no SMask")
			case tt.alpha != nil:
				if got := inflateData(t, img.AlphaMask()); !bytes.Equal(got, tt.alpha) {
					t.Errorf("SMask %v, want %v", got, tt.alpha)
				}
			}
		})
	}
}

// TestLoadPNG_ColorNotPremultiplied: a PDF image with an SMask takes straight
// (non-premultiplied) color; writing Go's premultiplied RGBA() values made
// translucent pixels darker.
func TestLoadPNG_ColorNotPremultiplied(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{0, 128, 0, 100})
	got := loadBytes(t, encodePNG(t, img))
	if c := inflateData(t, got.Data()); !bytes.Equal(c, []byte{0, 128, 0}) {
		t.Errorf("color %v, want [0 128 0] (straight, not premultiplied)", c)
	}
	if a := inflateData(t, got.AlphaMask()); !bytes.Equal(a, []byte{100}) {
		t.Errorf("alpha %v, want [100]", a)
	}
}

func TestLoadJPEG_UnsupportedComponents(t *testing.T) {
	if _, err := LoadImageFromReader(bytes.NewReader(headerJPEG(2, false, 0))); err == nil {
		t.Error("2-component JPEG accepted")
	}
}

// TestDrawImage_XObjectColorSpaceAndDecode checks what reaches the PDF: the
// color space of each JPEG kind and the Decode array of Adobe CMYK.
func TestDrawImage_XObjectColorSpaceAndDecode(t *testing.T) {
	var gray bytes.Buffer
	if err := jpeg.Encode(&gray, image.NewGray(image.Rect(0, 0, 4, 2)), nil); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"gray", gray.Bytes(), "/ColorSpace /DeviceGray /BitsPerComponent 8 /Filter /DCTDecode"},
		{"rgb", headerJPEG(3, false, 0), "/ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode"},
		{"adobe cmyk", headerJPEG(4, true, 0), "/ColorSpace /DeviceCMYK /BitsPerComponent 8 /Decode [1 0 1 0 1 0 1 0] /Filter /DCTDecode"},
		{"cmyk", headerJPEG(4, false, 0), "/ColorSpace /DeviceCMYK /BitsPerComponent 8 /Filter /DCTDecode"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New()
			p, err := c.NewPage()
			if err != nil {
				t.Fatal(err)
			}
			if err := p.DrawImage(loadBytes(t, tt.data), 50, 500, 40, 20); err != nil {
				t.Fatal(err)
			}
			out, err := c.Bytes()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(out, []byte(tt.want)) {
				i := bytes.Index(out, []byte("/Subtype /Image"))
				t.Errorf("XObject lacks %q; got %q", tt.want, out[i:min(len(out), i+160)])
			}
		})
	}
}

// TestLoadJPEG_ZeroDimensions: image.DecodeConfig accepts a frame header with
// a zero width (found by FuzzLoadImageFromReader); an image XObject cannot.
func TestLoadJPEG_ZeroDimensions(t *testing.T) {
	for name, at := range map[string]int{"height": 7, "width": 9} {
		data := headerJPEG(3, false, 0)
		data[at], data[at+1] = 0, 0 // SOF0: height at 7-8, width at 9-10
		if img, err := LoadImageFromReader(bytes.NewReader(data)); err == nil {
			t.Errorf("zero %s accepted: %dx%d", name, img.Width(), img.Height())
		}
	}
}
