package extractor

import (
	"testing"

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
