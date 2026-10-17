package internal

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/coregx/gxpdf/creator"
	"github.com/coregx/gxpdf/layout"
)

// pngBytes returns a w x h opaque PNG.
func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestImageSet_DecodesEachContentOnce(t *testing.T) {
	s := NewImageSet()
	data := pngBytes(t, 3, 2)

	first, err := s.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if first.Width() != 3 || first.Height() != 2 {
		t.Errorf("size %dx%d, want 3x2", first.Width(), first.Height())
	}
	again, err := s.Load(append([]byte(nil), data...))
	if err != nil || again != first {
		t.Errorf("equal content: got %p (err %v), want the first decode %p", again, err, first)
	}

	if _, err := s.Load([]byte("FAKE")); err == nil {
		t.Fatal("bad data decoded")
	}
	if _, err := s.Load([]byte("FAKE")); err == nil {
		t.Fatal("cached bad data decoded")
	}
}

// imageBlockPage draws data at 2 positions on one page.
func imageBlockPage(data []byte) layout.PageLayout {
	draw := func(r layout.Renderer) { r.DrawImage(data, 0, 0, 200, 100) }
	return layout.PageLayout{
		Size: layout.Size{Width: 595, Height: 842},
		Blocks: []layout.Block{
			{X: 50, Y: 100, Width: 200, Height: 100, Draw: draw},
			{X: 50, Y: 300, Width: 200, Height: 100, Draw: draw},
		},
	}
}

func TestRenderDocument_ImagesEmbeddedOnce(t *testing.T) {
	cr := creator.New()
	r := NewPDFRenderer(cr, nil, nil)
	page := imageBlockPage(pngBytes(t, 20, 10))

	if err := r.RenderDocument([]layout.PageLayout{page, page}, ExportedMetadata("", "")); err != nil {
		t.Fatal(err)
	}
	out, err := cr.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	// Four placements on two pages, one image XObject (the PNG is opaque:
	// no SMask).
	if n := bytes.Count(out, []byte("/Subtype /Image")); n != 1 {
		t.Errorf("%d image XObjects, want 1", n)
	}
	if bytes.Contains(out, []byte(" 0.90 0.90 0.90 rg")) {
		t.Error("placeholder rectangle drawn")
	}
}

func TestRenderDocument_ImageBadDataFails(t *testing.T) {
	cr := creator.New()
	r := NewPDFRenderer(cr, nil, nil)
	err := r.RenderDocument([]layout.PageLayout{imageBlockPage([]byte("FAKE"))}, ExportedMetadata("", ""))
	if err == nil || !strings.Contains(err.Error(), "page 1: image:") {
		t.Fatalf("err = %v, want a page 1 image error", err)
	}
}
