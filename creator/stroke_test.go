package creator

import (
	"testing"
)

func TestNewStroke(t *testing.T) {
	tests := []struct {
		name  string
		paint Paint
	}{
		{
			name:  "Solid color",
			paint: Black,
		},
		{
			name:  "Transparent color",
			paint: ColorRGBA{0, 0, 0, 0.5},
		},
		{
			name:  "CMYK color",
			paint: ColorCMYK{0, 0, 0, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stroke := NewStroke(tt.paint)
			if stroke == nil {
				t.Fatal("NewStroke returned nil")
			}
			if stroke.Paint != tt.paint {
				t.Errorf("Paint = %v, expected %v", stroke.Paint, tt.paint)
			}
			if stroke.Width != 1.0 {
				t.Errorf("Width = %f, expected 1.0", stroke.Width)
			}
			if stroke.LineCap != LineCapButt {
				t.Errorf("LineCap = %v, expected LineCapButt", stroke.LineCap)
			}
			if stroke.LineJoin != LineJoinMiter {
				t.Errorf("LineJoin = %v, expected LineJoinMiter", stroke.LineJoin)
			}
			if stroke.MiterLimit != 10.0 {
				t.Errorf("MiterLimit = %f, expected 10.0", stroke.MiterLimit)
			}
			if stroke.DashArray != nil {
				t.Errorf("DashArray = %v, expected nil", stroke.DashArray)
			}
			if stroke.DashPhase != 0 {
				t.Errorf("DashPhase = %f, expected 0", stroke.DashPhase)
			}
		})
	}
}

func TestStrokeWithWidth(t *testing.T) {
	stroke := NewStroke(Black).WithWidth(2.0)
	if stroke.Width != 2.0 {
		t.Errorf("Width = %f, expected 2.0", stroke.Width)
	}
}

func TestStrokeWithLineCap(t *testing.T) {
	tests := []struct {
		name string
		cap  LineCap
	}{
		{"Butt", LineCapButt},
		{"Round", LineCapRound},
		{"Square", LineCapSquare},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stroke := NewStroke(Black).WithLineCap(tt.cap)
			if stroke.LineCap != tt.cap {
				t.Errorf("LineCap = %v, expected %v", stroke.LineCap, tt.cap)
			}
		})
	}
}

func TestStrokeWithLineJoin(t *testing.T) {
	tests := []struct {
		name string
		join LineJoin
	}{
		{"Miter", LineJoinMiter},
		{"Round", LineJoinRound},
		{"Bevel", LineJoinBevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stroke := NewStroke(Black).WithLineJoin(tt.join)
			if stroke.LineJoin != tt.join {
				t.Errorf("LineJoin = %v, expected %v", stroke.LineJoin, tt.join)
			}
		})
	}
}

func TestStrokeWithMiterLimit(t *testing.T) {
	stroke := NewStroke(Black).WithMiterLimit(5.0)
	if stroke.MiterLimit != 5.0 {
		t.Errorf("MiterLimit = %f, expected 5.0", stroke.MiterLimit)
	}
}

func TestStrokeWithDash(t *testing.T) {
	dashArray := []float64{5, 3, 2, 3}
	dashPhase := 2.0
	stroke := NewStroke(Black).WithDash(dashArray, dashPhase)

	if len(stroke.DashArray) != len(dashArray) {
		t.Errorf("DashArray length = %d, expected %d", len(stroke.DashArray), len(dashArray))
	}
	for i := range dashArray {
		if stroke.DashArray[i] != dashArray[i] {
			t.Errorf("DashArray[%d] = %f, expected %f", i, stroke.DashArray[i], dashArray[i])
		}
	}
	if stroke.DashPhase != dashPhase {
		t.Errorf("DashPhase = %f, expected %f", stroke.DashPhase, dashPhase)
	}
}

func TestStrokeChaining(t *testing.T) {
	stroke := NewStroke(Black).
		WithWidth(2.0).
		WithLineCap(LineCapRound).
		WithLineJoin(LineJoinRound).
		WithMiterLimit(5.0).
		WithDash([]float64{5, 3}, 0)

	if stroke.Width != 2.0 {
		t.Errorf("Width = %f, expected 2.0", stroke.Width)
	}
	if stroke.LineCap != LineCapRound {
		t.Errorf("LineCap = %v, expected LineCapRound", stroke.LineCap)
	}
	if stroke.LineJoin != LineJoinRound {
		t.Errorf("LineJoin = %v, expected LineJoinRound", stroke.LineJoin)
	}
	if stroke.MiterLimit != 5.0 {
		t.Errorf("MiterLimit = %f, expected 5.0", stroke.MiterLimit)
	}
	if len(stroke.DashArray) != 2 {
		t.Errorf("DashArray length = %d, expected 2", len(stroke.DashArray))
	}
}

func TestStrokeValidate(t *testing.T) {
	tests := []struct {
		name    string
		stroke  *Stroke
		wantErr bool
	}{
		{
			name: "Valid solid color",
			stroke: &Stroke{
				Paint:      Black,
				Width:      1.0,
				LineCap:    LineCapButt,
				LineJoin:   LineJoinMiter,
				MiterLimit: 10.0,
			},
			wantErr: false,
		},
		{
			name: "Valid with dash",
			stroke: &Stroke{
				Paint:      Black,
				Width:      2.0,
				LineCap:    LineCapRound,
				LineJoin:   LineJoinRound,
				MiterLimit: 10.0,
				DashArray:  []float64{5, 3},
				DashPhase:  0,
			},
			wantErr: false,
		},
		{
			name: "Nil paint",
			stroke: &Stroke{
				Paint:      nil,
				Width:      1.0,
				LineCap:    LineCapButt,
				LineJoin:   LineJoinMiter,
				MiterLimit: 10.0,
			},
			wantErr: true,
		},
		{
			name: "Invalid width <= 0",
			stroke: &Stroke{
				Paint:      Black,
				Width:      0,
				LineCap:    LineCapButt,
				LineJoin:   LineJoinMiter,
				MiterLimit: 10.0,
			},
			wantErr: true,
		},
		{
			name: "Invalid negative width",
			stroke: &Stroke{
				Paint:      Black,
				Width:      -1.0,
				LineCap:    LineCapButt,
				LineJoin:   LineJoinMiter,
				MiterLimit: 10.0,
			},
			wantErr: true,
		},
		{
			name: "Invalid miter limit < 1",
			stroke: &Stroke{
				Paint:      Black,
				Width:      1.0,
				LineCap:    LineCapButt,
				LineJoin:   LineJoinMiter,
				MiterLimit: 0.5,
			},
			wantErr: true,
		},
		{
			name: "Invalid dash array negative value",
			stroke: &Stroke{
				Paint:      Black,
				Width:      1.0,
				LineCap:    LineCapButt,
				LineJoin:   LineJoinMiter,
				MiterLimit: 10.0,
				DashArray:  []float64{5, -3},
			},
			wantErr: true,
		},
		{
			name: "Invalid color",
			stroke: &Stroke{
				Paint:      Color{1.5, 0, 0},
				Width:      1.0,
				LineCap:    LineCapButt,
				LineJoin:   LineJoinMiter,
				MiterLimit: 10.0,
			},
			wantErr: true,
		},
		{
			name: "Valid gradient",
			stroke: &Stroke{
				Paint: func() *Gradient {
					g := NewLinearGradient(0, 0, 100, 0)
					_ = g.AddColorStop(0, Red)
					_ = g.AddColorStop(1, Blue)
					return g
				}(),
				Width:      1.0,
				LineCap:    LineCapButt,
				LineJoin:   LineJoinMiter,
				MiterLimit: 10.0,
			},
			wantErr: false,
		},
		{
			name: "Invalid gradient",
			stroke: &Stroke{
				Paint: func() *Gradient {
					g := NewLinearGradient(0, 0, 0, 0) // Same start/end
					_ = g.AddColorStop(0, Red)
					_ = g.AddColorStop(1, Blue)
					return g
				}(),
				Width:      1.0,
				LineCap:    LineCapButt,
				LineJoin:   LineJoinMiter,
				MiterLimit: 10.0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.stroke.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLineCapString(t *testing.T) {
	tests := []struct {
		cap      LineCap
		expected string
	}{
		{LineCapButt, "Butt"},
		{LineCapRound, "Round"},
		{LineCapSquare, "Square"},
		{LineCap(99), "Butt"}, // Unknown defaults to Butt
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			str := tt.cap.String()
			if str != tt.expected {
				t.Errorf("String() = %q, expected %q", str, tt.expected)
			}
		})
	}
}

func TestLineJoinString(t *testing.T) {
	tests := []struct {
		join     LineJoin
		expected string
	}{
		{LineJoinMiter, "Miter"},
		{LineJoinRound, "Round"},
		{LineJoinBevel, "Bevel"},
		{LineJoin(99), "Miter"}, // Unknown defaults to Miter
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			str := tt.join.String()
			if str != tt.expected {
				t.Errorf("String() = %q, expected %q", str, tt.expected)
			}
		})
	}
}
