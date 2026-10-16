package layout

import "testing"

func TestFits(t *testing.T) {
	tests := []struct {
		name        string
		need, avail float64
		want        bool
	}{
		{"exact", 100, 100, true},
		{"smaller", 99, 100, true},
		{"rounding_excess", 233.33333333333334, 233.3333333333333, true},
		{"within_tolerance", 100 + FitTolerance, 100, true},
		{"beyond_tolerance", 100 + 2*FitTolerance, 100, false},
		{"zero_area", 1, 0, false},
		{"empty_content_zero_area", 0, 0, true},
		{"unlimited", 5e9, 1e9, true},
		{"unlimited_threshold", 5e9, unlimitedHeight, true},
		{"just_below_unlimited", 1e8, 1e8 - 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Fits(tt.need, tt.avail); got != tt.want {
				t.Errorf("Fits(%v, %v) = %v, want %v", tt.need, tt.avail, got, tt.want)
			}
		})
	}
}
