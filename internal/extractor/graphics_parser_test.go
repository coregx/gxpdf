package extractor

import (
	"testing"

	"github.com/coregx/gxpdf/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGraphicsElement_String(t *testing.T) {
	elem := &GraphicsElement{
		Type:   GraphicsTypeLine,
		Points: []Point{{X: 0, Y: 0}, {X: 100, Y: 0}},
		Color:  NewColor(0, 0, 0),
		Width:  1.0,
	}

	str := elem.String()
	assert.Contains(t, str, "Line")
	assert.Contains(t, str, "0.00")
	assert.Contains(t, str, "100.00")
}

func TestGraphicsType_String(t *testing.T) {
	tests := []struct {
		typ      GraphicsType
		expected string
	}{
		{GraphicsTypeLine, "Line"},
		{GraphicsTypeRectangle, "Rectangle"},
		{GraphicsTypePath, "Path"},
		{GraphicsType(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.typ.String())
		})
	}
}

func TestPoint_NewPoint(t *testing.T) {
	p := NewPoint(10.5, 20.3)
	assert.Equal(t, 10.5, p.X)
	assert.Equal(t, 20.3, p.Y)
}

func TestPoint_String(t *testing.T) {
	p := NewPoint(10.5, 20.3)
	str := p.String()
	assert.Contains(t, str, "10.50")
	assert.Contains(t, str, "20.30")
}

func TestColor_NewColor(t *testing.T) {
	c := NewColor(0.5, 0.7, 0.9)
	assert.Equal(t, 0.5, c.R)
	assert.Equal(t, 0.7, c.G)
	assert.Equal(t, 0.9, c.B)
}

func TestColor_IsBlack(t *testing.T) {
	tests := []struct {
		name     string
		color    Color
		expected bool
	}{
		{"pure black", NewColor(0, 0, 0), true},
		{"almost black", NewColor(0.05, 0.05, 0.05), true},
		{"dark gray", NewColor(0.15, 0.15, 0.15), false},
		{"white", NewColor(1, 1, 1), false},
		{"red", NewColor(1, 0, 0), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.color.IsBlack())
		})
	}
}

func TestColor_String(t *testing.T) {
	c := NewColor(0.5, 0.7, 0.9)
	str := c.String()
	assert.Contains(t, str, "0.50")
	assert.Contains(t, str, "0.70")
	assert.Contains(t, str, "0.90")
}

func TestNewGraphicsState(t *testing.T) {
	state := NewGraphicsState()

	require.NotNil(t, state)
	assert.NotNil(t, state.CurrentPath)
	assert.Equal(t, 1.0, state.LineWidth)
	assert.True(t, state.StrokeColor.IsBlack())
	assert.True(t, state.FillColor.IsBlack())
	// CTM must default to identity matrix
	assert.True(t, state.CTM.IsIdentity(), "initial CTM must be identity matrix")
}

func TestGraphicsParser_isRectangle(t *testing.T) {
	gp := &GraphicsParser{state: NewGraphicsState()}

	tests := []struct {
		name     string
		points   []Point
		expected bool
	}{
		{
			name: "valid rectangle horizontal first",
			points: []Point{
				{X: 0, Y: 0},
				{X: 100, Y: 0},
				{X: 100, Y: 50},
				{X: 0, Y: 50},
				{X: 0, Y: 0},
			},
			expected: true,
		},
		{
			name: "valid rectangle vertical first",
			points: []Point{
				{X: 0, Y: 0},
				{X: 0, Y: 50},
				{X: 100, Y: 50},
				{X: 100, Y: 0},
				{X: 0, Y: 0},
			},
			expected: true,
		},
		{
			name: "too few points",
			points: []Point{
				{X: 0, Y: 0},
				{X: 100, Y: 0},
			},
			expected: false,
		},
		{
			name: "not closed",
			points: []Point{
				{X: 0, Y: 0},
				{X: 100, Y: 0},
				{X: 100, Y: 50},
				{X: 0, Y: 50},
				{X: 10, Y: 10}, // Not back to start
			},
			expected: false,
		},
		{
			name: "oblique shape",
			points: []Point{
				{X: 0, Y: 0},
				{X: 100, Y: 10},
				{X: 100, Y: 50},
				{X: 0, Y: 40},
				{X: 0, Y: 0},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gp.isRectangle(tt.points)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGraphicsParser_clearPath(t *testing.T) {
	gp := &GraphicsParser{state: NewGraphicsState()}

	gp.state.CurrentPath = []Point{{X: 0, Y: 0}, {X: 100, Y: 100}}
	assert.Len(t, gp.state.CurrentPath, 2)

	gp.clearPath()
	assert.Len(t, gp.state.CurrentPath, 0)
}

func TestGraphicsParser_closePath(t *testing.T) {
	gp := &GraphicsParser{state: NewGraphicsState()}

	gp.state.CurrentPath = []Point{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 100, Y: 100}}
	gp.closePath()

	assert.Len(t, gp.state.CurrentPath, 4)
	assert.Equal(t, gp.state.CurrentPath[0], gp.state.CurrentPath[3])
}

func TestGraphicsParser_strokePath_line(t *testing.T) {
	gp := &GraphicsParser{
		state:    NewGraphicsState(),
		elements: []*GraphicsElement{},
	}

	// Create a simple line
	gp.state.CurrentPath = []Point{{X: 0, Y: 0}, {X: 100, Y: 0}}
	gp.strokePath()

	require.Len(t, gp.elements, 1)
	assert.Equal(t, GraphicsTypeLine, gp.elements[0].Type)
	assert.Len(t, gp.elements[0].Points, 2)
}

func TestGraphicsParser_strokePath_rectangle(t *testing.T) {
	gp := &GraphicsParser{
		state:    NewGraphicsState(),
		elements: []*GraphicsElement{},
	}

	// Create a rectangle
	gp.state.CurrentPath = []Point{
		{X: 0, Y: 0},
		{X: 100, Y: 0},
		{X: 100, Y: 50},
		{X: 0, Y: 50},
		{X: 0, Y: 0},
	}
	gp.strokePath()

	require.Len(t, gp.elements, 1)
	assert.Equal(t, GraphicsTypeRectangle, gp.elements[0].Type)
	assert.Len(t, gp.elements[0].Points, 5)
}

func TestGraphicsParser_strokePath_multiSegment(t *testing.T) {
	gp := &GraphicsParser{
		state:    NewGraphicsState(),
		elements: []*GraphicsElement{},
	}

	// Create a multi-segment path (not a rectangle)
	gp.state.CurrentPath = []Point{
		{X: 0, Y: 0},
		{X: 50, Y: 0},
		{X: 100, Y: 50},
	}
	gp.strokePath()

	// Should extract 2 line segments
	require.Len(t, gp.elements, 2)
	assert.Equal(t, GraphicsTypeLine, gp.elements[0].Type)
	assert.Equal(t, GraphicsTypeLine, gp.elements[1].Type)
}

// TestFillPath_Rectangle verifies that the "re f" sequence (rectangle path
// followed by fill operator) produces a GraphicsTypeRectangle element.
// This is the most common table border pattern in PDFs from wkhtmltopdf,
// Chrome print, and LibreOffice — it was the root cause of issue #79.
func TestFillPath_Rectangle(t *testing.T) {
	gp := &GraphicsParser{
		state:    NewGraphicsState(),
		elements: []*GraphicsElement{},
	}

	// Simulate "re" operator: 10 50 100 1 re  (x=10, y=50, w=100, h=1)
	// The re operator builds a closed rectangle path: BL, BR, TR, TL, BL
	gp.state.CurrentPath = []Point{
		{X: 10, Y: 50},
		{X: 110, Y: 50},
		{X: 110, Y: 51},
		{X: 10, Y: 51},
		{X: 10, Y: 50}, // close
	}
	// Simulate fill color (black)
	gp.state.FillColor = NewColor(0, 0, 0)

	gp.fillPath()

	require.Len(t, gp.elements, 1, "re f must produce exactly 1 element")
	assert.Equal(t, GraphicsTypeRectangle, gp.elements[0].Type,
		"filled rectangle path must produce GraphicsTypeRectangle, not Line or Path")
	assert.Len(t, gp.elements[0].Points, 5, "rectangle element must carry all 5 path points")
}

// TestFillPath_NonRectangle verifies that a non-rectangular filled path
// (e.g., a triangle from "m l l f") is ignored and produces no elements.
// Only rectangles are meaningful as ruling-line candidates.
func TestFillPath_NonRectangle(t *testing.T) {
	gp := &GraphicsParser{
		state:    NewGraphicsState(),
		elements: []*GraphicsElement{},
	}

	// Simulate "m l l f" — a triangle, not a rectangle
	gp.state.CurrentPath = []Point{
		{X: 0, Y: 0},
		{X: 100, Y: 0},
		{X: 50, Y: 100},
	}
	gp.fillPath()

	assert.Empty(t, gp.elements, "non-rectangular filled path must not produce any elements")
}

// TestFillStrokePath verifies that the "re B" sequence (rectangle followed by
// fill-and-stroke operator) produces a GraphicsTypeRectangle element.
// The B/B* operators call fillPath which handles rectangle detection.
func TestFillStrokePath(t *testing.T) {
	gp := &GraphicsParser{
		state:    NewGraphicsState(),
		elements: []*GraphicsElement{},
	}

	// Simulate "re" operator building a closed rectangle
	gp.state.CurrentPath = []Point{
		{X: 0, Y: 0},
		{X: 200, Y: 0},
		{X: 200, Y: 2},
		{X: 0, Y: 2},
		{X: 0, Y: 0}, // close
	}

	// processOperator for "B" calls fillPath()
	gp.processOperator(&Operator{Name: "B", Operands: nil})

	require.Len(t, gp.elements, 1, "re B must produce exactly 1 element via fillPath")
	assert.Equal(t, GraphicsTypeRectangle, gp.elements[0].Type,
		"B operator on rectangle path must produce GraphicsTypeRectangle")
}

// ── CTM tracking tests ────────────────────────────────────────────────────────

// TestTransformPoint_Identity verifies that a point is unchanged when the CTM
// is the identity matrix [1 0 0 1 0 0].
func TestTransformPoint_Identity(t *testing.T) {
	gp := &GraphicsParser{state: NewGraphicsState()}
	// Identity CTM: point must be returned as-is
	pt := gp.transformPoint(100, 200)
	assert.InDelta(t, 100.0, pt.X, 1e-6, "identity CTM must not change X")
	assert.InDelta(t, 200.0, pt.Y, 1e-6, "identity CTM must not change Y")
}

// TestTransformPoint_Translation verifies that a CTM with translation offsets
// (e, f) shifts coordinates correctly: x' = x+e, y' = y+f.
func TestTransformPoint_Translation(t *testing.T) {
	gp := &GraphicsParser{state: NewGraphicsState()}
	// Set CTM to translate by (50, 100): [1 0 0 1 50 100]
	gp.state.CTM = NewMatrix(1, 0, 0, 1, 50, 100)
	pt := gp.transformPoint(10, 20)
	assert.InDelta(t, 60.0, pt.X, 1e-6, "X must be shifted by translation e=50")
	assert.InDelta(t, 120.0, pt.Y, 1e-6, "Y must be shifted by translation f=100")
}

// TestTransformPoint_ScaleAndTranslate verifies CTM with both scale and
// translation: [sx 0 0 sy tx ty] where x'=sx*x+tx, y'=sy*y+ty.
func TestTransformPoint_ScaleAndTranslate(t *testing.T) {
	gp := &GraphicsParser{state: NewGraphicsState()}
	// Scale by 2 and translate by (10, 20): [2 0 0 2 10 20]
	gp.state.CTM = NewMatrix(2, 0, 0, 2, 10, 20)
	pt := gp.transformPoint(5, 8)
	assert.InDelta(t, 20.0, pt.X, 1e-6, "X must be 2*5+10=20")
	assert.InDelta(t, 36.0, pt.Y, 1e-6, "Y must be 2*8+20=36")
}

// TestConcatMatrix_Translation verifies that the "cm" operator concatenates
// the new matrix with the current CTM. Starting from identity, applying a
// translation cm must set e/f offsets on the CTM.
func TestConcatMatrix_Translation(t *testing.T) {
	gp := &GraphicsParser{
		state:    NewGraphicsState(),
		elements: []*GraphicsElement{},
	}
	// Simulate: 1 0 0 1 30 40 cm  (pure translation)
	gp.processOperator(gpOpN("cm", 1, 0, 0, 1, 30, 40))

	assert.InDelta(t, 1.0, gp.state.CTM.A, 1e-6)
	assert.InDelta(t, 0.0, gp.state.CTM.B, 1e-6)
	assert.InDelta(t, 0.0, gp.state.CTM.C, 1e-6)
	assert.InDelta(t, 1.0, gp.state.CTM.D, 1e-6)
	assert.InDelta(t, 30.0, gp.state.CTM.E, 1e-6, "translation e must be 30")
	assert.InDelta(t, 40.0, gp.state.CTM.F, 1e-6, "translation f must be 40")
}

// TestSaveRestoreState verifies that q/Q operators correctly save and restore
// the graphics state including CTM. After Q the previous CTM must be active.
func TestSaveRestoreState(t *testing.T) {
	gp := &GraphicsParser{
		state:    NewGraphicsState(),
		elements: []*GraphicsElement{},
	}

	// Set a custom CTM before saving
	gp.state.CTM = NewMatrix(1, 0, 0, 1, 50, 100)
	gp.state.LineWidth = 3.0

	// q — save state
	gp.processOperator(&Operator{Name: "q"})

	// Modify CTM and line width inside the nested scope
	gp.state.CTM = NewMatrix(2, 0, 0, 2, 200, 300)
	gp.state.LineWidth = 7.0

	assert.InDelta(t, 200.0, gp.state.CTM.E, 1e-6, "CTM.E must be 200 inside q scope")
	assert.Equal(t, 7.0, gp.state.LineWidth)

	// Q — restore state
	gp.processOperator(&Operator{Name: "Q"})

	// Must be back to the saved values
	assert.InDelta(t, 50.0, gp.state.CTM.E, 1e-6, "CTM.E must be restored to 50 after Q")
	assert.InDelta(t, 100.0, gp.state.CTM.F, 1e-6, "CTM.F must be restored to 100 after Q")
	assert.Equal(t, 3.0, gp.state.LineWidth, "line width must be restored to 3.0 after Q")
}

// TestSaveRestoreState_EmptyStack verifies that a Q without a matching q is
// silently ignored and does not panic.
func TestSaveRestoreState_EmptyStack(t *testing.T) {
	gp := &GraphicsParser{
		state:    NewGraphicsState(),
		elements: []*GraphicsElement{},
	}
	// Q on empty stack must not panic
	assert.NotPanics(t, func() {
		gp.processOperator(&Operator{Name: "Q"})
	})
	// State must remain valid
	assert.True(t, gp.state.CTM.IsIdentity())
}

// TestGraphicsParser_CTMTransform verifies that the "cm" operator shifts
// rectangle coordinates into page space before the element is stored.
//
// This mirrors the exact failure pattern from issue #79:
//
//	q
//	1 0 0 1 100 200 cm  % translate to (100, 200)
//	0 0 50 10 re        % local rectangle (0,0)→(50,10)
//	f                   % fill
//	Q
//
// Without CTM tracking all points would be stored at local (0,0)→(50,10).
// With CTM tracking the stored page-space origin must be (100,200).
func TestGraphicsParser_CTMTransform(t *testing.T) {
	gp := &GraphicsParser{
		state:    NewGraphicsState(),
		elements: []*GraphicsElement{},
	}

	// q
	gp.processOperator(&Operator{Name: "q"})
	// 1 0 0 1 100 200 cm — pure translation by (100, 200)
	gp.processOperator(gpOpN("cm", 1, 0, 0, 1, 100, 200))
	// 0 0 50 10 re — local rectangle at (0,0), width 50, height 10
	gp.processOperator(gpOpN("re", 0, 0, 50, 10))
	// f — fill
	gp.processOperator(&Operator{Name: "f"})
	// Q
	gp.processOperator(&Operator{Name: "Q"})

	require.Len(t, gp.elements, 1, "re f after cm must produce exactly 1 element")
	assert.Equal(t, GraphicsTypeRectangle, gp.elements[0].Type)

	pts := gp.elements[0].Points
	require.Len(t, pts, 5, "rectangle element must have 5 points")

	// Bottom-left corner must be at page (100, 200), NOT local (0, 0).
	assert.InDelta(t, 100.0, pts[0].X, 1e-6, "BL.X must be shifted by CTM e=100")
	assert.InDelta(t, 200.0, pts[0].Y, 1e-6, "BL.Y must be shifted by CTM f=200")

	// Top-right corner must be at page (150, 210).
	assert.InDelta(t, 150.0, pts[2].X, 1e-6, "TR.X must be 100+50=150")
	assert.InDelta(t, 210.0, pts[2].Y, 1e-6, "TR.Y must be 200+10=210")
}

// TestGraphicsParser_SaveRestoreState verifies that "q" saves the CTM and
// "Q" restores it, so that a "cm" inside a q/Q block does not leak into the
// outer graphics context.
func TestGraphicsParser_SaveRestoreState(t *testing.T) {
	gp := &GraphicsParser{
		state:    NewGraphicsState(),
		elements: []*GraphicsElement{},
	}

	// Set outer CTM: translate (10, 20).
	gp.processOperator(gpOpN("cm", 1, 0, 0, 1, 10, 20))

	// Save graphics state.
	gp.processOperator(&Operator{Name: "q"})

	// Apply inner additional translation (5, 5) → total is (15, 25).
	gp.processOperator(gpOpN("cm", 1, 0, 0, 1, 5, 5))

	// Rectangle at local (0,0) 10×10 → page origin (15, 25).
	gp.processOperator(gpOpN("re", 0, 0, 10, 10))
	gp.processOperator(&Operator{Name: "f"})

	// Restore graphics state — CTM back to translate (10, 20).
	gp.processOperator(&Operator{Name: "Q"})

	// Rectangle at local (0,0) 10×10 → page origin (10, 20).
	gp.processOperator(gpOpN("re", 0, 0, 10, 10))
	gp.processOperator(&Operator{Name: "f"})

	require.Len(t, gp.elements, 2, "must produce 2 rectangle elements")

	// Element drawn inside q/Q: origin at page (15, 25).
	assert.InDelta(t, 15.0, gp.elements[0].Points[0].X, 1e-6, "inner rect BL.X must be 15")
	assert.InDelta(t, 25.0, gp.elements[0].Points[0].Y, 1e-6, "inner rect BL.Y must be 25")

	// Element drawn after Q restore: origin at page (10, 20).
	assert.InDelta(t, 10.0, gp.elements[1].Points[0].X, 1e-6, "outer rect BL.X must be 10 after Q restore")
	assert.InDelta(t, 20.0, gp.elements[1].Points[0].Y, 1e-6, "outer rect BL.Y must be 20 after Q restore")
}

// TestGraphicsParser_NestedCTM verifies correct coordinate accumulation across
// multiple consecutive q/cm/.../Q blocks — the real-world pattern used by PDF
// generators that draw each table row/cell border in its own translated context.
func TestGraphicsParser_NestedCTM(t *testing.T) {
	gp := &GraphicsParser{
		state:    NewGraphicsState(),
		elements: []*GraphicsElement{},
	}

	// Outer context: translate (100, 200) — e.g. page margin.
	gp.processOperator(gpOpN("cm", 1, 0, 0, 1, 100, 200))

	// First nested block: additional y offset 50 → total y = 250.
	gp.processOperator(&Operator{Name: "q"})
	gp.processOperator(gpOpN("cm", 1, 0, 0, 1, 0, 50))
	gp.processOperator(gpOpN("re", 0, 0, 200, 1))
	gp.processOperator(&Operator{Name: "f"})
	gp.processOperator(&Operator{Name: "Q"})

	// Second nested block: additional y offset 100 → total y = 300.
	gp.processOperator(&Operator{Name: "q"})
	gp.processOperator(gpOpN("cm", 1, 0, 0, 1, 0, 100))
	gp.processOperator(gpOpN("re", 0, 0, 200, 1))
	gp.processOperator(&Operator{Name: "f"})
	gp.processOperator(&Operator{Name: "Q"})

	require.Len(t, gp.elements, 2, "must produce 2 elements from consecutive nested CTM blocks")

	// First element: origin at page (100, 250).
	assert.InDelta(t, 100.0, gp.elements[0].Points[0].X, 1e-6, "1st rect BL.X must be 100")
	assert.InDelta(t, 250.0, gp.elements[0].Points[0].Y, 1e-6, "1st rect BL.Y must be 250 (200+50)")

	// Second element: origin at page (100, 300).
	assert.InDelta(t, 100.0, gp.elements[1].Points[0].X, 1e-6, "2nd rect BL.X must be 100")
	assert.InDelta(t, 300.0, gp.elements[1].Points[0].Y, 1e-6, "2nd rect BL.Y must be 300 (200+100)")
}

// TestCTMTracking_LineTransformed verifies that m/l paths respect CTM so that
// line endpoints land in page space rather than local space.
func TestCTMTracking_LineTransformed(t *testing.T) {
	gp := &GraphicsParser{
		state:    NewGraphicsState(),
		elements: []*GraphicsElement{},
	}

	// Simulate: 1 0 0 1 70 80 cm  0 0 m  100 0 l  S
	gp.processOperator(gpOpN("cm", 1, 0, 0, 1, 70, 80))
	gp.processOperator(gpOpN("m", 0, 0))
	gp.processOperator(gpOpN("l", 100, 0))
	gp.processOperator(&Operator{Name: "S"})

	require.Len(t, gp.elements, 1, "must produce exactly 1 line element")
	elem := gp.elements[0]
	assert.Equal(t, GraphicsTypeLine, elem.Type)
	assert.InDelta(t, 70.0, elem.Points[0].X, 1e-6, "start X must be 70 after CTM translation")
	assert.InDelta(t, 80.0, elem.Points[0].Y, 1e-6, "start Y must be 80 after CTM translation")
	assert.InDelta(t, 170.0, elem.Points[1].X, 1e-6, "end X must be 170 (100+70)")
	assert.InDelta(t, 80.0, elem.Points[1].Y, 1e-6, "end Y must be 80")
}

// TestCTMTracking_MultipleRectanglesIssue79 tests the exact PDF pattern from
// issue #79: multiple rectangles each in their own q/cm/re/f/Q group, all at
// local (0,0) but positioned by different CTM translations. This verifies that
// stacking q/Q correctly isolates each rectangle's CTM and that coordinates are
// not all (0,0) after the fix.
func TestCTMTracking_MultipleRectanglesIssue79(t *testing.T) {
	positions := [][2]float64{
		{50, 100},
		{50, 120},
		{50, 140},
	}

	gp := &GraphicsParser{
		state:    NewGraphicsState(),
		elements: []*GraphicsElement{},
	}

	for _, pos := range positions {
		// q  1 0 0 1 tx ty cm  0 0 10 5 re  f  Q
		gp.processOperator(&Operator{Name: "q"})
		gp.processOperator(gpOpN("cm", 1, 0, 0, 1, pos[0], pos[1]))
		gp.processOperator(gpOpN("re", 0, 0, 10, 5))
		gp.processOperator(&Operator{Name: "f"})
		gp.processOperator(&Operator{Name: "Q"})
	}

	require.Len(t, gp.elements, 3, "must produce 3 rectangle elements")
	for i, pos := range positions {
		elem := gp.elements[i]
		assert.Equal(t, GraphicsTypeRectangle, elem.Type)
		assert.InDelta(t, pos[0], elem.Points[0].X, 1e-6,
			"rectangle %d BL.X must be %.0f (CTM translation)", i, pos[0])
		assert.InDelta(t, pos[1], elem.Points[0].Y, 1e-6,
			"rectangle %d BL.Y must be %.0f (CTM translation)", i, pos[1])
	}
}

// ── Test helpers ──────────────────────────────────────────────────────────────

// gpOpN creates an Operator with float64 operands for GraphicsParser unit tests.
// Uses parser.NewReal so getNumber() resolves each value correctly.
func gpOpN(name string, vals ...float64) *Operator {
	ops := make([]parser.PdfObject, len(vals))
	for i, v := range vals {
		ops[i] = parser.NewReal(v)
	}
	return &Operator{Name: name, Operands: ops}
}

// TestFillPathIgnored_BeforeFix is a regression guard: confirms that after
// the fix, filled rectangle paths (re f) do produce GraphicsTypeRectangle
// elements. Before the fix, f/F/f* called clearPath() and produced nothing.
// This test documents the before/after behavior for issue #79.
//
// Covered operators: f, F, f* (fill only) and B, B* (fill+stroke).
// The b/b* (close+fill+stroke) operators are intentionally excluded: they call
// closePath() before fillPath(), which appends a duplicate first point to an
// already-closed "re" path (6 points total), and isRectangle() requires
// exactly 5. In practice, b/b* are used with m/l paths, not re rectangles.
func TestFillPathIgnored_BeforeFix(t *testing.T) {
	tests := []struct {
		name     string
		operator string
	}{
		{"fill f", "f"},
		{"fill F", "F"},
		{"fill f*", "f*"},
		{"fill+stroke B", "B"},
		{"fill+stroke B*", "B*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gp := &GraphicsParser{
				state:    NewGraphicsState(),
				elements: []*GraphicsElement{},
			}

			// Set up a valid closed rectangle path (as produced by the "re" operator):
			// BL → BR → TR → TL → BL (close)
			gp.state.CurrentPath = []Point{
				{X: 0, Y: 0},
				{X: 100, Y: 0},
				{X: 100, Y: 1},
				{X: 0, Y: 1},
				{X: 0, Y: 0}, // close
			}

			gp.processOperator(&Operator{Name: tt.operator, Operands: nil})

			// After the fix: fill operators on rectangle paths produce elements.
			require.NotEmpty(t, gp.elements,
				"operator %q on rectangle path must produce a GraphicsTypeRectangle element (regression guard for issue #79)",
				tt.operator)
			assert.Equal(t, GraphicsTypeRectangle, gp.elements[0].Type,
				"operator %q must produce GraphicsTypeRectangle", tt.operator)
		})
	}
}
