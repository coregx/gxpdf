package creator

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func dedupPNG(t *testing.T, w, h int, alpha uint8) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+3] = 200, alpha
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func loadDedupImage(t *testing.T, data []byte) *Image {
	t.Helper()
	img, err := LoadImageFromReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// countImageXObjects draws each image of every page and counts the image
// XObjects (SMasks included) in the output.
func countImageXObjects(t *testing.T, pages [][]*Image) int {
	t.Helper()
	c := New()
	for _, imgs := range pages {
		p, err := c.NewPage()
		if err != nil {
			t.Fatal(err)
		}
		for i, img := range imgs {
			if err := p.DrawImage(img, 50, 700-float64(i)*120, 100, 50); err != nil {
				t.Fatal(err)
			}
		}
	}
	out, err := c.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Count(out, []byte("/Subtype /Image"))
}

func TestImageXObjectsShared(t *testing.T) {
	logo := loadDedupImage(t, dedupPNG(t, 8, 4, 255))
	logoAgain := loadDedupImage(t, dedupPNG(t, 8, 4, 255)) // same bytes, separate load
	other := loadDedupImage(t, dedupPNG(t, 4, 8, 255))
	withAlpha := loadDedupImage(t, dedupPNG(t, 8, 4, 128)) // image + SMask

	tests := []struct {
		name  string
		pages [][]*Image
		want  int
	}{
		{"same image on three pages", [][]*Image{{logo}, {logo}, {logo}}, 1},
		{"same image twice on one page", [][]*Image{{logo, logo}}, 1},
		{"equal content from two loads", [][]*Image{{logo}, {logoAgain}}, 1},
		{"different images", [][]*Image{{logo, other}, {other}}, 2},
		{"transparent image on two pages: one image, one SMask", [][]*Image{{withAlpha}, {withAlpha}}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countImageXObjects(t, tt.pages); got != tt.want {
				t.Errorf("%d image XObjects, want %d", got, tt.want)
			}
		})
	}
}
