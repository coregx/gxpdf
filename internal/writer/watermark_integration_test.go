package writer

import (
	"testing"
)

// TestWatermarkRendering verifies that watermark operations render correctly.
//
// This is a regression test for bug-005 where watermarks were silently failing
// due to missing case 4 handler in renderGraphicsOp().
func TestWatermarkRendering(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		x         float64
		y         float64
		fontSize  float64
		colorR    float64
		colorG    float64
		colorB    float64
		font      string
		opacity   float64
		rotation  float64
		wantEmpty bool // If true, expect empty content stream (error case)
	}{
		{
			name:     "basic watermark",
			text:     "DRAFT",
			x:        300,
			y:        400,
			fontSize: 48,
			colorR:   0.5,
			colorG:   0.5,
			colorB:   0.5,
			font:     "Helvetica-Bold",
			opacity:  0.5,
			rotation: 45,
		},
		{
			name:     "no rotation",
			text:     "CONFIDENTIAL",
			x:        200,
			y:        500,
			fontSize: 36,
			colorR:   1.0,
			colorG:   0.0,
			colorB:   0.0,
			font:     "Helvetica",
			opacity:  0.3,
			rotation: 0,
		},
		{
			name:     "fully opaque",
			text:     "TOP SECRET",
			x:        250,
			y:        300,
			fontSize: 60,
			colorR:   0.0,
			colorG:   0.0,
			colorB:   0.0,
			font:     "Times-Bold",
			opacity:  1.0,
			rotation: 45,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create watermark operation
			gop := GraphicsOp{
				Type:              4, // Watermark
				X:                 tt.x,
				Y:                 tt.y,
				Text:              tt.text,
				TextSize:          tt.fontSize,
				TextColorR:        tt.colorR,
				TextColorG:        tt.colorG,
				TextColorB:        tt.colorB,
				WatermarkFont:     tt.font,
				WatermarkOpacity:  tt.opacity,
				WatermarkRotation: tt.rotation,
			}

			// Render the watermark
			csw := NewContentStreamWriter()
			resources := NewResourceDictionary()

			err := renderGraphicsOp(csw, gop, resources)

			if tt.wantEmpty {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("renderGraphicsOp() error = %v", err)
				return
			}

			// Verify content stream is not empty
			content := csw.String()
			if len(content) == 0 {
				t.Errorf("content stream is empty (watermark silently failed)")
				return
			}

			// Verify basic content stream structure
			// Should contain: q (save state), BT (begin text), text operators, ET (end text), Q (restore state)
			if !containsSubstring(content, "q") {
				t.Errorf("content stream missing 'q' (save state)")
			}
			if !containsSubstring(content, "BT") {
				t.Errorf("content stream missing 'BT' (begin text)")
			}
			if !containsSubstring(content, "ET") {
				t.Errorf("content stream missing 'ET' (end text)")
			}
			if !containsSubstring(content, "Q") {
				t.Errorf("content stream missing 'Q' (restore state)")
			}

			// Verify text is present (in some form)
			// Note: Text might be escaped/encoded, so just check it's not blank
			if !containsSubstring(content, "Tj") && !containsSubstring(content, "TJ") {
				t.Errorf("content stream missing text show operator (Tj or TJ)")
			}

			// Verify opacity handling (if opacity < 1.0, should have gs operator)
			if tt.opacity < 1.0 && !containsSubstring(content, "gs") {
				t.Errorf("content stream missing 'gs' (set graphics state) for opacity %.2f", tt.opacity)
			}

			// Verify rotation handling (if rotation != 0, should have cm operator)
			if tt.rotation != 0 && !containsSubstring(content, "cm") {
				t.Errorf("content stream missing 'cm' (concat matrix) for rotation %.2f", tt.rotation)
			}

			// Verify font resource was registered
			if len(resources.fonts) == 0 {
				t.Errorf("no fonts registered in resource dictionary")
			}
		})
	}
}

// TestWatermarkEdgeCases tests error conditions for watermark rendering.
func TestWatermarkEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		gop     GraphicsOp
		wantErr bool
		errMsg  string
	}{
		{
			name: "empty text",
			gop: GraphicsOp{
				Type:              4,
				Text:              "",
				TextSize:          48,
				WatermarkFont:     "Helvetica",
				WatermarkOpacity:  0.5,
				WatermarkRotation: 45,
			},
			wantErr: true,
			errMsg:  "watermark text is empty",
		},
		{
			name: "no font",
			gop: GraphicsOp{
				Type:              4,
				Text:              "TEST",
				TextSize:          48,
				WatermarkFont:     "",
				WatermarkOpacity:  0.5,
				WatermarkRotation: 45,
			},
			wantErr: true,
			errMsg:  "watermark font is not set",
		},
		{
			name: "zero font size",
			gop: GraphicsOp{
				Type:              4,
				Text:              "TEST",
				TextSize:          0,
				WatermarkFont:     "Helvetica",
				WatermarkOpacity:  0.5,
				WatermarkRotation: 45,
			},
			wantErr: true,
			errMsg:  "watermark font size must be positive: 0.00",
		},
		{
			name: "negative font size",
			gop: GraphicsOp{
				Type:              4,
				Text:              "TEST",
				TextSize:          -10,
				WatermarkFont:     "Helvetica",
				WatermarkOpacity:  0.5,
				WatermarkRotation: 45,
			},
			wantErr: true,
			errMsg:  "watermark font size must be positive: -10.00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csw := NewContentStreamWriter()
			resources := NewResourceDictionary()

			err := renderGraphicsOp(csw, tt.gop, resources)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("error message = %q, want %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// containsSubstring checks if a string contains a substring.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && indexOfSubstring(s, substr) >= 0
}

// indexOfSubstring returns the index of the first occurrence of substr in s, or -1 if not found.
func indexOfSubstring(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(substr) > len(s) {
		return -1
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
