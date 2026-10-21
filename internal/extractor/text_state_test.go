package extractor

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMatrix(t *testing.T) {
	m := NewMatrix(1, 2, 3, 4, 5, 6)

	assert.Equal(t, 1.0, m.A)
	assert.Equal(t, 2.0, m.B)
	assert.Equal(t, 3.0, m.C)
	assert.Equal(t, 4.0, m.D)
	assert.Equal(t, 5.0, m.E)
	assert.Equal(t, 6.0, m.F)
}

func TestIdentity(t *testing.T) {
	m := Identity()

	assert.Equal(t, 1.0, m.A)
	assert.Equal(t, 0.0, m.B)
	assert.Equal(t, 0.0, m.C)
	assert.Equal(t, 1.0, m.D)
	assert.Equal(t, 0.0, m.E)
	assert.Equal(t, 0.0, m.F)
	assert.True(t, m.IsIdentity())
}

func TestTranslation(t *testing.T) {
	m := Translation(100, 200)

	assert.Equal(t, 1.0, m.A)
	assert.Equal(t, 0.0, m.B)
	assert.Equal(t, 0.0, m.C)
	assert.Equal(t, 1.0, m.D)
	assert.Equal(t, 100.0, m.E)
	assert.Equal(t, 200.0, m.F)

	// Test transformation
	x, y := m.Transform(0, 0)
	assert.Equal(t, 100.0, x)
	assert.Equal(t, 200.0, y)

	x, y = m.Transform(10, 20)
	assert.Equal(t, 110.0, x)
	assert.Equal(t, 220.0, y)
}

func TestScaling(t *testing.T) {
	m := Scaling(2, 3)

	assert.Equal(t, 2.0, m.A)
	assert.Equal(t, 0.0, m.B)
	assert.Equal(t, 0.0, m.C)
	assert.Equal(t, 3.0, m.D)
	assert.Equal(t, 0.0, m.E)
	assert.Equal(t, 0.0, m.F)

	// Test transformation
	x, y := m.Transform(10, 20)
	assert.Equal(t, 20.0, x)
	assert.Equal(t, 60.0, y)
}

func TestRotation(t *testing.T) {
	// Rotate 90 degrees (π/2 radians)
	m := Rotation(math.Pi / 2)

	// cos(90°) ≈ 0, sin(90°) ≈ 1
	assert.InDelta(t, 0.0, m.A, 0.0001)
	assert.InDelta(t, 1.0, m.B, 0.0001)
	assert.InDelta(t, -1.0, m.C, 0.0001)
	assert.InDelta(t, 0.0, m.D, 0.0001)

	// Test transformation: (1, 0) should become (0, 1)
	x, y := m.Transform(1, 0)
	assert.InDelta(t, 0.0, x, 0.0001)
	assert.InDelta(t, 1.0, y, 0.0001)
}

func TestMatrix_Transform(t *testing.T) {
	tests := []struct {
		name     string
		matrix   Matrix
		inX, inY float64
		outX     float64
		outY     float64
	}{
		{
			name:   "identity",
			matrix: Identity(),
			inX:    10, inY: 20,
			outX: 10, outY: 20,
		},
		{
			name:   "translation",
			matrix: Translation(100, 200),
			inX:    10, inY: 20,
			outX: 110, outY: 220,
		},
		{
			name:   "scaling",
			matrix: Scaling(2, 3),
			inX:    10, inY: 20,
			outX: 20, outY: 60,
		},
		{
			name:   "custom matrix",
			matrix: NewMatrix(1, 0, 0, 1, 50, 100),
			inX:    10, inY: 20,
			outX: 60, outY: 120,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, y := tt.matrix.Transform(tt.inX, tt.inY)
			assert.InDelta(t, tt.outX, x, 0.0001)
			assert.InDelta(t, tt.outY, y, 0.0001)
		})
	}
}

func TestMatrix_Multiply(t *testing.T) {
	// Column-vector order: Translation(100, 200) * Scaling(2, 2)
	// applies the scale first, then the translation.
	translate := Translation(100, 200)
	scale := Scaling(2, 2)
	result := translate.Multiply(scale)

	// Transform (10, 10):
	// 1. Scale: (10, 10) -> (20, 20)
	// 2. Translate: (20, 20) -> (120, 220)
	x, y := result.Transform(10, 10)
	assert.Equal(t, 120.0, x)
	assert.Equal(t, 220.0, y)
}

func TestMatrix_MultiplyMatchesSequentialNonCommutingTransforms(t *testing.T) {
	first := NewMatrix(2, 0, 0, 3, 10, 20)
	second := NewMatrix(0, 1, -1, 0, 5, 7)
	combined := first.Multiply(second)

	secondX, secondY := second.Transform(4, 5)
	wantX, wantY := first.Transform(secondX, secondY)
	gotX, gotY := combined.Transform(4, 5)

	assert.InDelta(t, wantX, gotX, 1e-9)
	assert.InDelta(t, wantY, gotY, 1e-9)
	assert.Equal(t, NewMatrix(0, 3, -2, 0, 20, 41), combined)
}

func TestMatrix_MultiplyAffineParityAcrossRotationReflectionAndShear(t *testing.T) {
	tests := []struct {
		name  string
		outer Matrix
		inner Matrix
		point [2]float64
	}{
		{
			name:  "rotation after nonuniform scale",
			outer: Rotation(math.Pi / 2),
			inner: NewMatrix(2, 0, 0, 3, 11, -7),
			point: [2]float64{4, 5},
		},
		{
			name:  "reflection after shear",
			outer: NewMatrix(-1, 0, 0, 1, 20, 30),
			inner: NewMatrix(1, 0.75, -0.25, 1, 3, 4),
			point: [2]float64{-6, 9},
		},
		{
			name:  "two noncommuting general affines",
			outer: NewMatrix(0.5, -1.25, 2, 0.75, -13, 17),
			inner: NewMatrix(1.5, 0.25, -0.5, 3, 19, -23),
			point: [2]float64{8, -2},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			innerX, innerY := test.inner.Transform(test.point[0], test.point[1])
			wantX, wantY := test.outer.Transform(innerX, innerY)
			gotX, gotY := test.outer.Multiply(test.inner).Transform(test.point[0], test.point[1])
			assert.InDelta(t, wantX, gotX, 1e-12)
			assert.InDelta(t, wantY, gotY, 1e-12)
		})
	}
}

func TestMatrix_MultiplyMatchesSequentialShearRotationTranslation(t *testing.T) {
	outer := NewMatrix(1, 0.25, -0.5, 2, 17, -9)
	inner := NewMatrix(0, -1, 1, 0, 4, 11)
	combined := outer.Multiply(inner)
	for _, point := range [][2]float64{{0, 0}, {3, -7}, {-2.5, 8.25}} {
		innerX, innerY := inner.Transform(point[0], point[1])
		wantX, wantY := outer.Transform(innerX, innerY)
		gotX, gotY := combined.Transform(point[0], point[1])
		assert.InDelta(t, wantX, gotX, 1e-12)
		assert.InDelta(t, wantY, gotY, 1e-12)
	}
}

func TestMatrix_MultiplyIdentity(t *testing.T) {
	m := NewMatrix(1, 2, 3, 4, 5, 6)
	identity := Identity()

	// m * identity = m
	result1 := m.Multiply(identity)
	assert.Equal(t, m.A, result1.A)
	assert.Equal(t, m.B, result1.B)
	assert.Equal(t, m.C, result1.C)
	assert.Equal(t, m.D, result1.D)
	assert.Equal(t, m.E, result1.E)
	assert.Equal(t, m.F, result1.F)

	// identity * m = m
	result2 := identity.Multiply(m)
	assert.Equal(t, m.A, result2.A)
	assert.Equal(t, m.B, result2.B)
	assert.Equal(t, m.C, result2.C)
	assert.Equal(t, m.D, result2.D)
	assert.Equal(t, m.E, result2.E)
	assert.Equal(t, m.F, result2.F)
}

func TestMatrix_IsIdentity(t *testing.T) {
	tests := []struct {
		name     string
		matrix   Matrix
		expected bool
	}{
		{"identity", Identity(), true},
		{"translation", Translation(1, 2), false},
		{"scaling", Scaling(2, 2), false},
		{"almost identity", NewMatrix(1.0000001, 0, 0, 1, 0, 0), true}, // within epsilon
		{"not identity", NewMatrix(1, 1, 0, 1, 0, 0), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.matrix.IsIdentity())
		})
	}
}

func TestMatrix_String(t *testing.T) {
	m := NewMatrix(1, 2, 3, 4, 5, 6)
	str := m.String()

	assert.Contains(t, str, "1.000")
	assert.Contains(t, str, "2.000")
	assert.Contains(t, str, "3.000")
	assert.Contains(t, str, "4.000")
	assert.Contains(t, str, "5.000")
	assert.Contains(t, str, "6.000")
}

func TestNewTextState(t *testing.T) {
	ts := NewTextState()

	assert.True(t, ts.Tm.IsIdentity())
	assert.True(t, ts.Tlm.IsIdentity())
	assert.Equal(t, "", ts.FontName)
	assert.Equal(t, 0.0, ts.FontSize)
	assert.Equal(t, 0.0, ts.CharSpace)
	assert.Equal(t, 0.0, ts.WordSpace)
	assert.Equal(t, 100.0, ts.HorizScale)
	assert.Equal(t, 0.0, ts.Leading)
	assert.Equal(t, 0.0, ts.Rise)
	assert.Equal(t, 0.0, ts.CurrentX)
	assert.Equal(t, 0.0, ts.CurrentY)
}

func TestTextState_Reset(t *testing.T) {
	ts := NewTextState()
	ts.Tm = Translation(100, 200)
	ts.Tlm = Translation(50, 100)
	ts.FontName = "Helvetica"
	ts.FontSize = 12

	ts.Reset()

	assert.True(t, ts.Tm.IsIdentity())
	assert.True(t, ts.Tlm.IsIdentity())
	assert.Equal(t, 0.0, ts.CurrentX)
	assert.Equal(t, 0.0, ts.CurrentY)

	// Font properties should NOT be reset
	assert.Equal(t, "Helvetica", ts.FontName)
	assert.Equal(t, 12.0, ts.FontSize)
}

func TestTextState_SetTextMatrix(t *testing.T) {
	ts := NewTextState()
	ts.SetTextMatrix(1, 0, 0, 1, 100, 200)

	assert.Equal(t, 1.0, ts.Tm.A)
	assert.Equal(t, 0.0, ts.Tm.B)
	assert.Equal(t, 0.0, ts.Tm.C)
	assert.Equal(t, 1.0, ts.Tm.D)
	assert.Equal(t, 100.0, ts.Tm.E)
	assert.Equal(t, 200.0, ts.Tm.F)

	// Tlm should be same as Tm
	assert.Equal(t, ts.Tm, ts.Tlm)

	// Current position should be updated
	assert.Equal(t, 100.0, ts.CurrentX)
	assert.Equal(t, 200.0, ts.CurrentY)
}

func TestTextState_Translate(t *testing.T) {
	ts := NewTextState()
	ts.SetTextMatrix(1, 0, 0, 1, 100, 200)

	ts.Translate(50, 30)

	// Position should be moved by (50, 30)
	assert.Equal(t, 150.0, ts.CurrentX)
	assert.Equal(t, 230.0, ts.CurrentY)
}

func TestTextState_TranslateUsesExistingRotatedBasis(t *testing.T) {
	ts := NewTextState()
	ts.SetTextMatrix(0, 1, -1, 0, 100, 200)

	ts.Translate(50, 30)

	assert.InDelta(t, 70, ts.CurrentX, 1e-9)
	assert.InDelta(t, 250, ts.CurrentY, 1e-9)
}

func TestTextState_TranslateSetLeading(t *testing.T) {
	ts := NewTextState()
	ts.SetTextMatrix(1, 0, 0, 1, 100, 200)

	ts.TranslateSetLeading(50, 30)

	// Position should be moved by (50, 30)
	assert.Equal(t, 150.0, ts.CurrentX)
	assert.Equal(t, 230.0, ts.CurrentY)

	// Leading should be set to -ty
	assert.Equal(t, -30.0, ts.Leading)
}

func TestTextState_MoveToNextLine(t *testing.T) {
	ts := NewTextState()
	ts.SetTextMatrix(1, 0, 0, 1, 100, 200)
	ts.Leading = 14 // Typical line spacing

	ts.MoveToNextLine()

	// Y should decrease by leading
	assert.Equal(t, 100.0, ts.CurrentX)
	assert.Equal(t, 186.0, ts.CurrentY) // 200 - 14
}

func TestTextState_SetFont(t *testing.T) {
	ts := NewTextState()
	ts.SetFont("Helvetica", 12)

	assert.Equal(t, "Helvetica", ts.FontName)
	assert.Equal(t, 12.0, ts.FontSize)
}

func TestTextState_AdvanceX(t *testing.T) {
	ts := NewTextState()
	ts.SetTextMatrix(1, 0, 0, 1, 100, 200)

	ts.AdvanceX(50)

	// X should be advanced
	assert.Equal(t, 150.0, ts.CurrentX)
	assert.Equal(t, 200.0, ts.CurrentY) // Y unchanged
}

func TestTextState_AdvanceXUsesExistingRotatedBasis(t *testing.T) {
	ts := NewTextState()
	ts.SetTextMatrix(0, 1, -1, 0, 100, 200)

	ts.AdvanceX(50)

	assert.InDelta(t, 100, ts.CurrentX, 1e-9)
	assert.InDelta(t, 250, ts.CurrentY, 1e-9)
}

func TestTextState_String(t *testing.T) {
	ts := NewTextState()
	ts.SetTextMatrix(1, 0, 0, 1, 100, 200)
	ts.SetFont("Helvetica", 12)

	str := ts.String()

	assert.Contains(t, str, "Helvetica")
	assert.Contains(t, str, "12.0")
	assert.Contains(t, str, "100.00")
	assert.Contains(t, str, "200.00")
}
