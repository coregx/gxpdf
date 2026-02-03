package creator

import (
	"testing"
)

func TestNewFill(t *testing.T) {
	tests := []struct {
		name  string
		paint Paint
	}{
		{
			name:  "Solid color",
			paint: Red,
		},
		{
			name:  "Transparent color",
			paint: ColorRGBA{1, 0, 0, 0.5},
		},
		{
			name:  "CMYK color",
			paint: ColorCMYK{0, 1, 1, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fill := NewFill(tt.paint)
			if fill == nil {
				t.Fatal("NewFill returned nil")
			}
			if fill.Paint != tt.paint {
				t.Errorf("Paint = %v, expected %v", fill.Paint, tt.paint)
			}
			if fill.Opacity != 1.0 {
				t.Errorf("Opacity = %f, expected 1.0", fill.Opacity)
			}
			if fill.Rule != FillRuleNonZero {
				t.Errorf("Rule = %v, expected FillRuleNonZero", fill.Rule)
			}
		})
	}
}

func TestFillWithOpacity(t *testing.T) {
	fill := NewFill(Red).WithOpacity(0.5)
	if fill.Opacity != 0.5 {
		t.Errorf("Opacity = %f, expected 0.5", fill.Opacity)
	}
}

func TestFillWithRule(t *testing.T) {
	fill := NewFill(Red).WithRule(FillRuleEvenOdd)
	if fill.Rule != FillRuleEvenOdd {
		t.Errorf("Rule = %v, expected FillRuleEvenOdd", fill.Rule)
	}
}

func TestFillChaining(t *testing.T) {
	fill := NewFill(Red).WithOpacity(0.8).WithRule(FillRuleEvenOdd)
	if fill.Paint != Red {
		t.Errorf("Paint = %v, expected Red", fill.Paint)
	}
	if fill.Opacity != 0.8 {
		t.Errorf("Opacity = %f, expected 0.8", fill.Opacity)
	}
	if fill.Rule != FillRuleEvenOdd {
		t.Errorf("Rule = %v, expected FillRuleEvenOdd", fill.Rule)
	}
}

func TestFillValidate(t *testing.T) {
	tests := []struct {
		name    string
		fill    *Fill
		wantErr bool
	}{
		{
			name: "Valid solid color",
			fill: &Fill{
				Paint:   Red,
				Opacity: 1.0,
				Rule:    FillRuleNonZero,
			},
			wantErr: false,
		},
		{
			name: "Valid gradient",
			fill: &Fill{
				Paint: func() *Gradient {
					g := NewLinearGradient(0, 0, 100, 0)
					_ = g.AddColorStop(0, Red)
					_ = g.AddColorStop(1, Blue)
					return g
				}(),
				Opacity: 0.8,
				Rule:    FillRuleNonZero,
			},
			wantErr: false,
		},
		{
			name: "Nil paint",
			fill: &Fill{
				Paint:   nil,
				Opacity: 1.0,
				Rule:    FillRuleNonZero,
			},
			wantErr: true,
		},
		{
			name: "Invalid opacity > 1",
			fill: &Fill{
				Paint:   Red,
				Opacity: 1.5,
				Rule:    FillRuleNonZero,
			},
			wantErr: true,
		},
		{
			name: "Invalid opacity < 0",
			fill: &Fill{
				Paint:   Red,
				Opacity: -0.1,
				Rule:    FillRuleNonZero,
			},
			wantErr: true,
		},
		{
			name: "Invalid color",
			fill: &Fill{
				Paint:   Color{1.5, 0, 0},
				Opacity: 1.0,
				Rule:    FillRuleNonZero,
			},
			wantErr: true,
		},
		{
			name: "Invalid gradient",
			fill: &Fill{
				Paint: func() *Gradient {
					g := NewLinearGradient(0, 0, 0, 0) // Same start/end
					_ = g.AddColorStop(0, Red)
					_ = g.AddColorStop(1, Blue)
					return g
				}(),
				Opacity: 1.0,
				Rule:    FillRuleNonZero,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fill.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFillRuleString(t *testing.T) {
	tests := []struct {
		rule     FillRule
		expected string
	}{
		{FillRuleNonZero, "NonZero"},
		{FillRuleEvenOdd, "EvenOdd"},
		{FillRule(99), "NonZero"}, // Unknown defaults to NonZero
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			str := tt.rule.String()
			if str != tt.expected {
				t.Errorf("String() = %q, expected %q", str, tt.expected)
			}
		})
	}
}
