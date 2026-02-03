package creator

import (
	"math"
	"testing"
)

func TestIdentity(t *testing.T) {
	identity := Identity()

	expected := Transform{A: 1, B: 0, C: 0, D: 1, E: 0, F: 0}
	if identity != expected {
		t.Errorf("Identity() = %+v, want %+v", identity, expected)
	}
}

func TestTranslate(t *testing.T) {
	tests := []struct {
		name string
		tx   float64
		ty   float64
		want Transform
	}{
		{"Zero", 0, 0, Transform{A: 1, B: 0, C: 0, D: 1, E: 0, F: 0}},
		{"Right", 100, 0, Transform{A: 1, B: 0, C: 0, D: 1, E: 100, F: 0}},
		{"Up", 0, 200, Transform{A: 1, B: 0, C: 0, D: 1, E: 0, F: 200}},
		{"Both", 100, 200, Transform{A: 1, B: 0, C: 0, D: 1, E: 100, F: 200}},
		{"Negative", -50, -75, Transform{A: 1, B: 0, C: 0, D: 1, E: -50, F: -75}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Translate(tt.tx, tt.ty)
			if got != tt.want {
				t.Errorf("Translate(%f, %f) = %+v, want %+v", tt.tx, tt.ty, got, tt.want)
			}
		})
	}
}

func TestScale(t *testing.T) {
	tests := []struct {
		name string
		sx   float64
		sy   float64
		want Transform
	}{
		{"Identity", 1, 1, Transform{A: 1, B: 0, C: 0, D: 1, E: 0, F: 0}},
		{"Double", 2, 2, Transform{A: 2, B: 0, C: 0, D: 2, E: 0, F: 0}},
		{"Half", 0.5, 0.5, Transform{A: 0.5, B: 0, C: 0, D: 0.5, E: 0, F: 0}},
		{"Flip X", -1, 1, Transform{A: -1, B: 0, C: 0, D: 1, E: 0, F: 0}},
		{"Flip Y", 1, -1, Transform{A: 1, B: 0, C: 0, D: -1, E: 0, F: 0}},
		{"Non-uniform", 2, 3, Transform{A: 2, B: 0, C: 0, D: 3, E: 0, F: 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Scale(tt.sx, tt.sy)
			if got != tt.want {
				t.Errorf("Scale(%f, %f) = %+v, want %+v", tt.sx, tt.sy, got, tt.want)
			}
		})
	}
}

func TestRotate(t *testing.T) {
	tests := []struct {
		name    string
		degrees float64
	}{
		{"0 degrees", 0},
		{"90 degrees", 90},
		{"180 degrees", 180},
		{"270 degrees", 270},
		{"-90 degrees", -90},
		{"45 degrees", 45},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Rotate(tt.degrees)

			// Verify it's a rotation matrix
			// For rotation: a² + b² = 1 and c² + d² = 1
			sumAB := got.A*got.A + got.B*got.B
			sumCD := got.C*got.C + got.D*got.D

			if !approxEqual(sumAB, 1.0) {
				t.Errorf("Rotate(%f): a² + b² = %f, want 1.0", tt.degrees, sumAB)
			}
			if !approxEqual(sumCD, 1.0) {
				t.Errorf("Rotate(%f): c² + d² = %f, want 1.0", tt.degrees, sumCD)
			}

			// Verify no translation
			if got.E != 0 || got.F != 0 {
				t.Errorf("Rotate(%f) has translation: E=%f, F=%f", tt.degrees, got.E, got.F)
			}
		})
	}
}

func TestRotate90Degrees(t *testing.T) {
	rot := Rotate(90)

	// 90° rotation should be approximately:
	// [ 0  1 ]
	// [-1  0 ]
	if !approxEqual(rot.A, 0) || !approxEqual(rot.B, 1) ||
		!approxEqual(rot.C, -1) || !approxEqual(rot.D, 0) {
		t.Errorf("Rotate(90) = %+v, want A≈0, B≈1, C≈-1, D≈0", rot)
	}
}

func TestRotateAround(t *testing.T) {
	// Rotate 90° around point (100, 100)
	rot := RotateAround(90, 100, 100)

	// Transform the center point - should stay at (100, 100)
	x, y := rot.TransformPoint(100, 100)
	if !approxEqual(x, 100) || !approxEqual(y, 100) {
		t.Errorf("RotateAround(90, 100, 100) center = (%f, %f), want (100, 100)", x, y)
	}

	// Transform a point to the right of center
	// (200, 100) should rotate to (100, 200)
	x, y = rot.TransformPoint(200, 100)
	if !approxEqual(x, 100) || !approxEqual(y, 200) {
		t.Errorf("RotateAround(90, 100, 100) point = (%f, %f), want (100, 200)", x, y)
	}
}

func TestSkew(t *testing.T) {
	skew := Skew(15, 0)

	// Skew should modify B or C but not A or D
	if skew.A != 1 || skew.D != 1 {
		t.Errorf("Skew(15, 0): A=%f, D=%f, want both 1.0", skew.A, skew.D)
	}

	// Should have no translation
	if skew.E != 0 || skew.F != 0 {
		t.Errorf("Skew(15, 0) has translation: E=%f, F=%f", skew.E, skew.F)
	}
}

func TestThen(t *testing.T) {
	// Translate then scale
	translate := Translate(100, 200)
	scale := Scale(2, 2)

	combined := translate.Then(scale)

	// Apply to origin
	x, y := combined.TransformPoint(0, 0)

	// Should translate then scale: (0,0) -> (100,200) -> (200,400)
	if !approxEqual(x, 200) || !approxEqual(y, 400) {
		t.Errorf("Translate.Then(Scale) point = (%f, %f), want (200, 400)", x, y)
	}
}

func TestThenIdentity(t *testing.T) {
	translate := Translate(100, 200)
	identity := Identity()

	// Composing with identity should not change the transform
	result := translate.Then(identity)

	if result != translate {
		t.Errorf("Translate.Then(Identity) = %+v, want %+v", result, translate)
	}
}

func TestTransformPoint(t *testing.T) {
	tests := []struct {
		name      string
		transform Transform
		x         float64
		y         float64
		wantX     float64
		wantY     float64
	}{
		{
			name:      "Identity",
			transform: Identity(),
			x:         10, y: 20,
			wantX: 10, wantY: 20,
		},
		{
			name:      "Translate",
			transform: Translate(100, 200),
			x:         10, y: 20,
			wantX: 110, wantY: 220,
		},
		{
			name:      "Scale",
			transform: Scale(2, 3),
			x:         10, y: 20,
			wantX: 20, wantY: 60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotX, gotY := tt.transform.TransformPoint(tt.x, tt.y)
			if !approxEqual(gotX, tt.wantX) || !approxEqual(gotY, tt.wantY) {
				t.Errorf("TransformPoint(%f, %f) = (%f, %f), want (%f, %f)",
					tt.x, tt.y, gotX, gotY, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestToPDFMatrix(t *testing.T) {
	transform := Transform{
		A: 1, B: 2,
		C: 3, D: 4,
		E: 5, F: 6,
	}

	matrix := transform.ToPDFMatrix()

	expected := [6]float64{1, 2, 3, 4, 5, 6}
	if matrix != expected {
		t.Errorf("ToPDFMatrix() = %v, want %v", matrix, expected)
	}
}

func TestComplexTransformChain(t *testing.T) {
	// Complex chain: translate -> rotate -> scale
	transform := Translate(100, 100).
		Then(Rotate(45)).
		Then(Scale(2, 2))

	// Transform origin
	x, y := transform.TransformPoint(0, 0)

	// Should have moved and scaled
	if approxEqual(x, 0) && approxEqual(y, 0) {
		t.Error("Complex transform did not move origin")
	}

	// Verify transform is not identity
	if transform == Identity() {
		t.Error("Complex transform is identity")
	}
}

func TestRotateInverse(t *testing.T) {
	// Rotate 45° then -45° should be identity
	forward := Rotate(45)
	backward := Rotate(-45)

	combined := forward.Then(backward)

	// Should be close to identity
	identity := Identity()

	if !transformApproxEqual(combined, identity) {
		t.Errorf("Rotate(45).Then(Rotate(-45)) = %+v, want identity", combined)
	}
}

// Helper functions for floating-point comparison

func approxEqual(a, b float64) bool {
	const epsilon = 1e-9
	return math.Abs(a-b) < epsilon
}

func transformApproxEqual(a, b Transform) bool {
	return approxEqual(a.A, b.A) &&
		approxEqual(a.B, b.B) &&
		approxEqual(a.C, b.C) &&
		approxEqual(a.D, b.D) &&
		approxEqual(a.E, b.E) &&
		approxEqual(a.F, b.F)
}
