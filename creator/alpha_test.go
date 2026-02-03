package creator

import (
	"testing"
)

// TestColorRGBA tests the ColorRGBA struct and its methods.
func TestColorRGBA(t *testing.T) {
	tests := []struct {
		name  string
		color ColorRGBA
		wantR float64
		wantG float64
		wantB float64
		wantA float64
	}{
		{
			name:  "fully opaque red",
			color: ColorRGBA{1, 0, 0, 1},
			wantR: 1,
			wantG: 0,
			wantB: 0,
			wantA: 1,
		},
		{
			name:  "semi-transparent blue",
			color: ColorRGBA{0, 0, 1, 0.5},
			wantR: 0,
			wantG: 0,
			wantB: 1,
			wantA: 0.5,
		},
		{
			name:  "fully transparent",
			color: ColorRGBA{0, 0, 0, 0},
			wantR: 0,
			wantG: 0,
			wantB: 0,
			wantA: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.color.R != tt.wantR {
				t.Errorf("R = %v, want %v", tt.color.R, tt.wantR)
			}
			if tt.color.G != tt.wantG {
				t.Errorf("G = %v, want %v", tt.color.G, tt.wantG)
			}
			if tt.color.B != tt.wantB {
				t.Errorf("B = %v, want %v", tt.color.B, tt.wantB)
			}
			if tt.color.A != tt.wantA {
				t.Errorf("A = %v, want %v", tt.color.A, tt.wantA)
			}
		})
	}
}

// TestNewColorRGBA tests the NewColorRGBA constructor.
func TestNewColorRGBA(t *testing.T) {
	tests := []struct {
		name       string
		r, g, b, a float64
		want       ColorRGBA
	}{
		{
			name: "red with 50% opacity",
			r:    1, g: 0, b: 0, a: 0.5,
			want: ColorRGBA{1, 0, 0, 0.5},
		},
		{
			name: "green with 30% opacity",
			r:    0, g: 1, b: 0, a: 0.3,
			want: ColorRGBA{0, 1, 0, 0.3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewColorRGBA(tt.r, tt.g, tt.b, tt.a)
			if got != tt.want {
				t.Errorf("NewColorRGBA() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestColorRGBA_ToColor tests converting RGBA to RGB.
func TestColorRGBA_ToColor(t *testing.T) {
	tests := []struct {
		name string
		rgba ColorRGBA
		want Color
	}{
		{
			name: "semi-transparent red to opaque red",
			rgba: ColorRGBA{1, 0, 0, 0.5},
			want: Color{1, 0, 0},
		},
		{
			name: "fully transparent blue to opaque blue",
			rgba: ColorRGBA{0, 0, 1, 0},
			want: Color{0, 0, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.rgba.ToColor()
			if got != tt.want {
				t.Errorf("ToColor() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestColorRGBA_WithAlpha tests changing alpha value.
func TestColorRGBA_WithAlpha(t *testing.T) {
	tests := []struct {
		name     string
		original ColorRGBA
		newAlpha float64
		want     ColorRGBA
	}{
		{
			name:     "change red from opaque to 50% transparent",
			original: ColorRGBA{1, 0, 0, 1},
			newAlpha: 0.5,
			want:     ColorRGBA{1, 0, 0, 0.5},
		},
		{
			name:     "change blue from 30% to fully transparent",
			original: ColorRGBA{0, 0, 1, 0.3},
			newAlpha: 0,
			want:     ColorRGBA{0, 0, 1, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.original.WithAlpha(tt.newAlpha)
			if got != tt.want {
				t.Errorf("WithAlpha() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPredefinedTransparentColors tests predefined transparent color constants.
func TestPredefinedTransparentColors(t *testing.T) {
	tests := []struct {
		name  string
		color ColorRGBA
		wantA float64
	}{
		{"TransparentBlack", TransparentBlack, 0},
		{"SemiTransparentBlack", SemiTransparentBlack, 0.5},
		{"TransparentWhite", TransparentWhite, 0},
		{"SemiTransparentWhite", SemiTransparentWhite, 0.5},
		{"TransparentRed", TransparentRed, 0},
		{"SemiTransparentRed", SemiTransparentRed, 0.5},
		{"TransparentGreen", TransparentGreen, 0},
		{"SemiTransparentGreen", SemiTransparentGreen, 0.5},
		{"TransparentBlue", TransparentBlue, 0},
		{"SemiTransparentBlue", SemiTransparentBlue, 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.color.A != tt.wantA {
				t.Errorf("%s alpha = %v, want %v", tt.name, tt.color.A, tt.wantA)
			}
		})
	}
}

// TestLineOpacity tests drawing line with opacity.
func TestLineOpacity(t *testing.T) {
	c := New()
	page, err := c.NewPage()
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	opacity := 0.7
	err = page.DrawLine(100, 100, 300, 200, &LineOptions{
		Color:   Blue,
		Width:   2,
		Opacity: &opacity,
	})
	if err != nil {
		t.Fatalf("DrawLine with opacity failed: %v", err)
	}

	if len(page.graphicsOps) != 1 {
		t.Fatalf("Expected 1 graphics operation, got %d", len(page.graphicsOps))
	}

	if page.graphicsOps[0].LineOpts.Opacity == nil {
		t.Error("Opacity not set on LineOptions")
	} else if *page.graphicsOps[0].LineOpts.Opacity != opacity {
		t.Errorf("Opacity = %v, want %v", *page.graphicsOps[0].LineOpts.Opacity, opacity)
	}
}

// TestRectOpacity tests drawing rectangle with opacity.
func TestRectOpacity(t *testing.T) {
	c := New()
	page, err := c.NewPage()
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	opacity := 0.3
	err = page.DrawRect(100, 100, 200, 100, &RectOptions{
		FillColor: &Red,
		Opacity:   &opacity,
	})
	if err != nil {
		t.Fatalf("DrawRect with opacity failed: %v", err)
	}

	if len(page.graphicsOps) != 1 {
		t.Fatalf("Expected 1 graphics operation, got %d", len(page.graphicsOps))
	}

	if page.graphicsOps[0].RectOpts.Opacity == nil {
		t.Error("Opacity not set on RectOptions")
	} else if *page.graphicsOps[0].RectOpts.Opacity != opacity {
		t.Errorf("Opacity = %v, want %v", *page.graphicsOps[0].RectOpts.Opacity, opacity)
	}
}

// TestCircleOpacity tests drawing circle with opacity.
func TestCircleOpacity(t *testing.T) {
	c := New()
	page, err := c.NewPage()
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	opacity := 0.4
	err = page.DrawCircle(300, 400, 50, &CircleOptions{
		FillColor: &Green,
		Opacity:   &opacity,
	})
	if err != nil {
		t.Fatalf("DrawCircle with opacity failed: %v", err)
	}

	if len(page.graphicsOps) != 1 {
		t.Fatalf("Expected 1 graphics operation, got %d", len(page.graphicsOps))
	}

	if page.graphicsOps[0].CircleOpts.Opacity == nil {
		t.Error("Opacity not set on CircleOptions")
	} else if *page.graphicsOps[0].CircleOpts.Opacity != opacity {
		t.Errorf("Opacity = %v, want %v", *page.graphicsOps[0].CircleOpts.Opacity, opacity)
	}
}

// TestEllipseOpacity tests drawing ellipse with opacity.
func TestEllipseOpacity(t *testing.T) {
	c := New()
	page, err := c.NewPage()
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	opacity := 0.6
	err = page.DrawEllipse(300, 400, 100, 50, &EllipseOptions{
		FillColor: &Yellow,
		Opacity:   &opacity,
	})
	if err != nil {
		t.Fatalf("DrawEllipse with opacity failed: %v", err)
	}

	if len(page.graphicsOps) != 1 {
		t.Fatalf("Expected 1 graphics operation, got %d", len(page.graphicsOps))
	}

	if page.graphicsOps[0].EllipseOpts.Opacity == nil {
		t.Error("Opacity not set on EllipseOptions")
	} else if *page.graphicsOps[0].EllipseOpts.Opacity != opacity {
		t.Errorf("Opacity = %v, want %v", *page.graphicsOps[0].EllipseOpts.Opacity, opacity)
	}
}

// TestPolygonOpacity tests drawing polygon with opacity.
func TestPolygonOpacity(t *testing.T) {
	c := New()
	page, err := c.NewPage()
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	opacity := 0.5
	vertices := []Point{
		{X: 100, Y: 100},
		{X: 150, Y: 50},
		{X: 200, Y: 100},
	}

	err = page.DrawPolygon(vertices, &PolygonOptions{
		FillColor: &Cyan,
		Opacity:   &opacity,
	})
	if err != nil {
		t.Fatalf("DrawPolygon with opacity failed: %v", err)
	}

	if len(page.graphicsOps) != 1 {
		t.Fatalf("Expected 1 graphics operation, got %d", len(page.graphicsOps))
	}

	if page.graphicsOps[0].PolygonOpts.Opacity == nil {
		t.Error("Opacity not set on PolygonOptions")
	} else if *page.graphicsOps[0].PolygonOpts.Opacity != opacity {
		t.Errorf("Opacity = %v, want %v", *page.graphicsOps[0].PolygonOpts.Opacity, opacity)
	}
}

// TestPolylineOpacity tests drawing polyline with opacity.
func TestPolylineOpacity(t *testing.T) {
	c := New()
	page, err := c.NewPage()
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	opacity := 0.8
	vertices := []Point{
		{X: 100, Y: 100},
		{X: 150, Y: 150},
		{X: 200, Y: 100},
	}

	err = page.DrawPolyline(vertices, &PolylineOptions{
		Color:   Magenta,
		Width:   2,
		Opacity: &opacity,
	})
	if err != nil {
		t.Fatalf("DrawPolyline with opacity failed: %v", err)
	}

	if len(page.graphicsOps) != 1 {
		t.Fatalf("Expected 1 graphics operation, got %d", len(page.graphicsOps))
	}

	if page.graphicsOps[0].PolylineOpts.Opacity == nil {
		t.Error("Opacity not set on PolylineOptions")
	} else if *page.graphicsOps[0].PolylineOpts.Opacity != opacity {
		t.Errorf("Opacity = %v, want %v", *page.graphicsOps[0].PolylineOpts.Opacity, opacity)
	}
}

// TestBezierOpacity tests drawing bezier curve with opacity.
func TestBezierOpacity(t *testing.T) {
	c := New()
	page, err := c.NewPage()
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	opacity := 0.4
	segments := []BezierSegment{
		{
			Start: Point{X: 100, Y: 100},
			C1:    Point{X: 150, Y: 200},
			C2:    Point{X: 200, Y: 200},
			End:   Point{X: 250, Y: 100},
		},
	}

	err = page.DrawBezierCurve(segments, &BezierOptions{
		Color:   Blue,
		Width:   2,
		Opacity: &opacity,
	})
	if err != nil {
		t.Fatalf("DrawBezierCurve with opacity failed: %v", err)
	}

	if len(page.graphicsOps) != 1 {
		t.Fatalf("Expected 1 graphics operation, got %d", len(page.graphicsOps))
	}

	if page.graphicsOps[0].BezierOpts.Opacity == nil {
		t.Error("Opacity not set on BezierOptions")
	} else if *page.graphicsOps[0].BezierOpts.Opacity != opacity {
		t.Errorf("Opacity = %v, want %v", *page.graphicsOps[0].BezierOpts.Opacity, opacity)
	}
}

// TestMultipleOpacityValues tests that different opacity values are stored correctly.
func TestMultipleOpacityValues(t *testing.T) {
	c := New()
	page, err := c.NewPage()
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	opacityValues := []float64{0.1, 0.3, 0.5, 0.7, 0.9, 1.0}

	for i, opacity := range opacityValues {
		op := opacity // Capture for pointer
		x1 := 100.0
		y := 700.0 - float64(i)*50
		x2 := 400.0

		err := page.DrawLine(x1, y, x2, y, &LineOptions{
			Color:   Black,
			Width:   2,
			Opacity: &op,
		})
		if err != nil {
			t.Fatalf("DrawLine with opacity %v failed: %v", opacity, err)
		}
	}

	if len(page.graphicsOps) != len(opacityValues) {
		t.Fatalf("Expected %d graphics operations, got %d", len(opacityValues), len(page.graphicsOps))
	}

	for i, expectedOpacity := range opacityValues {
		gotOpacity := page.graphicsOps[i].LineOpts.Opacity
		if gotOpacity == nil {
			t.Errorf("Operation %d: Opacity is nil", i)
		} else if *gotOpacity != expectedOpacity {
			t.Errorf("Operation %d: Opacity = %v, want %v", i, *gotOpacity, expectedOpacity)
		}
	}
}
