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
	// PDF row-vector order: Translation(100, 200) * Scaling(2, 2)
	// applies the translation first, then the scale.
	translate := Translation(100, 200)
	scale := Scaling(2, 2)
	result := translate.Multiply(scale)

	// Transform (10, 10):
	// 1. Translate: (10, 10) -> (110, 210)
	// 2. Scale: (110, 210) -> (220, 420)
	x, y := result.Transform(10, 10)
	assert.Equal(t, 220.0, x)
	assert.Equal(t, 420.0, y)
}

func TestMatrix_MultiplyNonCommutingTransforms(t *testing.T) {
	first := NewMatrix(2, 0, 0, 3, 10, 20)
	second := NewMatrix(0, 1, -1, 0, 5, 7)
	combined := first.Multiply(second)

	firstX, firstY := first.Transform(4, 5)
	wantX, wantY := second.Transform(firstX, firstY)
	gotX, gotY := combined.Transform(4, 5)

	assert.InDelta(t, wantX, gotX, 1e-9)
	assert.InDelta(t, wantY, gotY, 1e-9)
	assert.Equal(t, NewMatrix(0, 2, -3, 0, -15, 17), combined)
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
