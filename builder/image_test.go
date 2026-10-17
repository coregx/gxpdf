package builder

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"strings"
	"testing"

	"github.com/coregx/gxpdf/layout"
)

// testPNG returns a solid red w x h PNG.
func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+3] = 255, 255
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestImageElement_DisplaySize(t *testing.T) {
	pt := func(v float64) *layout.Value { x := layout.Pt(v); return &x }
	pct := func(v float64) *layout.Value { x := layout.Pct(v); return &x }
	tests := []struct {
		name         string
		w, h         *layout.Value
		areaW, areaH float64
		wantW, wantH float64
	}{
		{"intrinsic size, 1 px = 1 pt", nil, nil, 500, 800, 200, 100},
		{"FitWidth keeps aspect", pt(100), nil, 500, 800, 100, 50},
		{"FitHeight keeps aspect (#101: 20 mm tall, 2:1)", nil, pt(56.693), 500, 800, 113.386, 56.693},
		{"both, width limits", pt(100), pt(100), 500, 800, 100, 50},
		{"both, height limits", pt(300), pt(40), 500, 800, 80, 40},
		{"wider than the area is scaled down", nil, nil, 150, 800, 150, 75},
		{"FitWidth beyond the area is capped", pt(400), nil, 300, 800, 300, 150},
		{"percentage width of the area", pct(50), nil, 300, 800, 150, 75},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newImageElement(nil, 200, 100, imageConfig{width: tt.w, height: tt.h})
			w, h := e.displaySize(tt.areaW, tt.areaH)
			if math.Abs(w-tt.wantW) > 1e-3 || math.Abs(h-tt.wantH) > 1e-3 {
				t.Errorf("displaySize = %.3f x %.3f, want %.3f x %.3f", w, h, tt.wantW, tt.wantH)
			}
		})
	}
}

func TestImageElement_Measurable(t *testing.T) {
	pt := func(v float64) *layout.Value { x := layout.Pt(v); return &x }
	pct := func(v float64) *layout.Value { x := layout.Pct(v); return &x }
	tests := []struct {
		name     string
		w, h     *layout.Value
		min, max float64
	}{
		{"intrinsic: shrinks to nothing, natural max", nil, nil, 0, 200},
		{"FitWidth: fixed", pt(80), nil, 80, 80},
		{"FitHeight: fixed width from aspect", nil, pt(30), 60, 60},
		{"both: the contained width", pt(300), pt(40), 80, 80},
		{"percentage width: shrinkable", pct(50), nil, 0, 200},
		{"fixed width, percentage height: shrinkable", pt(80), pct(10), 0, 80},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newImageElement(nil, 200, 100, imageConfig{width: tt.w, height: tt.h})
			if got := e.MinWidth(); !near(got, tt.min) {
				t.Errorf("MinWidth = %v, want %v", got, tt.min)
			}
			if got := e.MaxWidth(); !near(got, tt.max) {
				t.Errorf("MaxWidth = %v, want %v", got, tt.max)
			}
		})
	}
	var _ layout.Measurable = (*imageElement)(nil)
}

func TestImageElement_PlanLayout(t *testing.T) {
	e := newImageElement([]byte("x"), 200, 400, imageConfig{})

	t.Run("fits", func(t *testing.T) {
		plan := e.PlanLayout(layout.Area{Width: 500, Height: 800})
		if plan.Status != layout.Full || len(plan.Blocks) != 1 || plan.Consumed != 400 {
			t.Fatalf("plan = %+v", plan)
		}
		if b := plan.Blocks[0]; b.Width != 200 || b.Height != 400 {
			t.Errorf("block %vx%v, want 200x400", b.Width, b.Height)
		}
	})
	t.Run("does not fit mid-page: next page", func(t *testing.T) {
		plan := e.PlanLayout(layout.Area{Width: 500, Height: 300})
		if plan.Status != layout.Nothing {
			t.Fatalf("status = %v, want Nothing", plan.Status)
		}
	})
	t.Run("taller than an empty page: scaled to the page", func(t *testing.T) {
		plan := e.PlanLayout(layout.Area{Width: 500, Height: 300, AtTop: true})
		if plan.Status != layout.Full || len(plan.Blocks) != 1 {
			t.Fatalf("plan = %+v", plan)
		}
		if b := plan.Blocks[0]; !near(b.Width, 150) || !near(b.Height, 300) || plan.Consumed != 300 {
			t.Errorf("block %vx%v consumed %v, want 150x300 consumed 300", b.Width, b.Height, plan.Consumed)
		}
	})
	t.Run("zero-width area: nothing drawn", func(t *testing.T) {
		plan := e.PlanLayout(layout.Area{Width: 0, Height: 800})
		if plan.Status != layout.Full || len(plan.Blocks) != 0 || plan.Consumed != 0 {
			t.Fatalf("plan = %+v", plan)
		}
	})
}

func TestContainer_Image_BadData(t *testing.T) {
	for name, data := range map[string][]byte{
		"empty":     nil,
		"not image": []byte("FAKE_PNG_DATA"),
		"truncated": testPNG(t, 4, 4)[:40],
	} {
		t.Run(name, func(t *testing.T) {
			b := testBuilder()
			c := newContainer(b)
			c.Image(data)
			if len(c.elements) != 0 {
				t.Errorf("bad data added %d elements", len(c.elements))
			}
			if len(b.errs) != 1 || !strings.Contains(b.errs[0].Error(), "image (") {
				t.Errorf("errs = %v, want one image error", b.errs)
			}
		})
	}
}

func TestContainer_Image_SameBytesDecodedOnce(t *testing.T) {
	b := testBuilder()
	data := testPNG(t, 3, 2)
	first, err := b.images.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	c := newContainer(b)
	c.Image(append([]byte(nil), data...)) // equal content, different slice
	again, _ := b.images.Load(data)
	if first != again {
		t.Error("same content decoded twice")
	}
}
