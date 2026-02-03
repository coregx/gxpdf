package creator

import (
	"testing"
)

func TestRGB(t *testing.T) {
	tests := []struct {
		name     string
		r, g, b  uint8
		expected Color
	}{
		{
			name:     "Black",
			r:        0,
			g:        0,
			b:        0,
			expected: Color{0, 0, 0},
		},
		{
			name:     "White",
			r:        255,
			g:        255,
			b:        255,
			expected: Color{1, 1, 1},
		},
		{
			name:     "Red",
			r:        255,
			g:        0,
			b:        0,
			expected: Color{1, 0, 0},
		},
		{
			name:     "Gray 50%",
			r:        128,
			g:        128,
			b:        128,
			expected: Color{128.0 / 255.0, 128.0 / 255.0, 128.0 / 255.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			color := RGB(tt.r, tt.g, tt.b)
			if color.R != tt.expected.R || color.G != tt.expected.G || color.B != tt.expected.B {
				t.Errorf("RGB(%d, %d, %d) = %v, expected %v",
					tt.r, tt.g, tt.b, color, tt.expected)
			}
		})
	}
}

func TestRGBA(t *testing.T) {
	tests := []struct {
		name     string
		r, g, b  uint8
		a        float64
		expected ColorRGBA
	}{
		{
			name:     "Transparent Red",
			r:        255,
			g:        0,
			b:        0,
			a:        0.5,
			expected: ColorRGBA{1, 0, 0, 0.5},
		},
		{
			name:     "Opaque Blue",
			r:        0,
			g:        0,
			b:        255,
			a:        1.0,
			expected: ColorRGBA{0, 0, 1, 1.0},
		},
		{
			name:     "Fully Transparent",
			r:        0,
			g:        0,
			b:        0,
			a:        0.0,
			expected: ColorRGBA{0, 0, 0, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			color := RGBA(tt.r, tt.g, tt.b, tt.a)
			if color.R != tt.expected.R || color.G != tt.expected.G ||
				color.B != tt.expected.B || color.A != tt.expected.A {
				t.Errorf("RGBA(%d, %d, %d, %f) = %v, expected %v",
					tt.r, tt.g, tt.b, tt.a, color, tt.expected)
			}
		})
	}
}

func TestHex(t *testing.T) {
	tests := []struct {
		name     string
		hex      string
		expected Color
		wantErr  bool
	}{
		{
			name:     "Red long form with #",
			hex:      "#FF0000",
			expected: Color{1, 0, 0},
		},
		{
			name:     "Green long form without #",
			hex:      "00FF00",
			expected: Color{0, 1, 0},
		},
		{
			name:     "Blue short form",
			hex:      "#00F",
			expected: Color{0, 0, 1},
		},
		{
			name:     "Red short form without #",
			hex:      "F00",
			expected: Color{1, 0, 0},
		},
		{
			name:     "Gray",
			hex:      "#808080",
			expected: Color{128.0 / 255.0, 128.0 / 255.0, 128.0 / 255.0},
		},
		{
			name:    "Invalid length",
			hex:     "#FFFF",
			wantErr: true,
		},
		{
			name:    "Invalid characters",
			hex:     "#GGGGGG",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			color, err := Hex(tt.hex)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Hex(%q) expected error, got nil", tt.hex)
				}
				return
			}
			if err != nil {
				t.Errorf("Hex(%q) unexpected error: %v", tt.hex, err)
				return
			}

			// Allow small floating point differences
			const epsilon = 0.01
			if abs(color.R-tt.expected.R) > epsilon ||
				abs(color.G-tt.expected.G) > epsilon ||
				abs(color.B-tt.expected.B) > epsilon {
				t.Errorf("Hex(%q) = %v, expected %v", tt.hex, color, tt.expected)
			}
		})
	}
}

func TestGrayN(t *testing.T) {
	tests := []struct {
		name     string
		value    uint8
		expected Color
	}{
		{
			name:     "Black",
			value:    0,
			expected: Color{0, 0, 0},
		},
		{
			name:     "White",
			value:    255,
			expected: Color{1, 1, 1},
		},
		{
			name:     "50% Gray",
			value:    128,
			expected: Color{128.0 / 255.0, 128.0 / 255.0, 128.0 / 255.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			color := GrayN(tt.value)
			if color.R != tt.expected.R || color.G != tt.expected.G || color.B != tt.expected.B {
				t.Errorf("GrayN(%d) = %v, expected %v", tt.value, color, tt.expected)
			}
		})
	}
}

func TestPaintInterface(t *testing.T) {
	// Test that all paint types implement the Paint interface
	var _ Paint = Color{}
	var _ Paint = ColorRGBA{}
	var _ Paint = ColorCMYK{}
	var _ Paint = (*Gradient)(nil)
}

// Note: validateColor is tested through the fill and stroke validation tests

func TestValidateColorRGBA(t *testing.T) {
	tests := []struct {
		name    string
		color   ColorRGBA
		wantErr bool
	}{
		{
			name:    "Valid opaque",
			color:   ColorRGBA{1, 0, 0, 1},
			wantErr: false,
		},
		{
			name:    "Valid transparent",
			color:   ColorRGBA{0, 0, 1, 0.5},
			wantErr: false,
		},
		{
			name:    "Invalid alpha > 1",
			color:   ColorRGBA{1, 0, 0, 1.5},
			wantErr: true,
		},
		{
			name:    "Invalid alpha < 0",
			color:   ColorRGBA{1, 0, 0, -0.1},
			wantErr: true,
		},
		{
			name:    "Invalid color component",
			color:   ColorRGBA{1.5, 0, 0, 1},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateColorRGBA(tt.color)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateColorRGBA(%v) error = %v, wantErr %v", tt.color, err, tt.wantErr)
			}
		})
	}
}

func TestValidateColorCMYK(t *testing.T) {
	tests := []struct {
		name    string
		color   ColorCMYK
		wantErr bool
	}{
		{
			name:    "Valid black",
			color:   ColorCMYK{0, 0, 0, 1},
			wantErr: false,
		},
		{
			name:    "Valid cyan",
			color:   ColorCMYK{1, 0, 0, 0},
			wantErr: false,
		},
		{
			name:    "Invalid cyan > 1",
			color:   ColorCMYK{1.5, 0, 0, 0},
			wantErr: true,
		},
		{
			name:    "Invalid magenta < 0",
			color:   ColorCMYK{0, -0.1, 0, 0},
			wantErr: true,
		},
		{
			name:    "Invalid yellow > 1",
			color:   ColorCMYK{0, 0, 1.5, 0},
			wantErr: true,
		},
		{
			name:    "Invalid black < 0",
			color:   ColorCMYK{0, 0, 0, -0.1},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateColorCMYK(tt.color)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateColorCMYK(%v) error = %v, wantErr %v", tt.color, err, tt.wantErr)
			}
		})
	}
}

// Note: abs function is defined in bezier.go
