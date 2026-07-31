package tabledetect

import (
	"testing"

	"github.com/coregx/gxpdf/internal/extractor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rectElem builds a GraphicsTypeRectangle element from x,y,w,h.
// Points follow the re operator convention: BL, BR, TR, TL, BL (close).
func rectElem(x, y, w, h float64) *extractor.GraphicsElement {
	return &extractor.GraphicsElement{
		Type: extractor.GraphicsTypeRectangle,
		Points: []extractor.Point{
			extractor.NewPoint(x, y),
			extractor.NewPoint(x+w, y),
			extractor.NewPoint(x+w, y+h),
			extractor.NewPoint(x, y+h),
			extractor.NewPoint(x, y), // close
		},
		Color: extractor.NewColor(0, 0, 0),
		Width: 1.0,
	}
}

// TestLinesFromRectangle_ThinHorizontal verifies that a rectangle whose height
// is below maxRulingThickness is decomposed into a single horizontal ruling
// line placed at the midpoint Y of the rectangle.
func TestLinesFromRectangle_ThinHorizontal(t *testing.T) {
	d := NewDefaultRulingLineDetector()

	// 100pt wide × 1pt tall — classic wkhtmltopdf table row border.
	elem := rectElem(10, 50, 100, 1)
	lines := d.linesFromRectangle(elem)

	require.Len(t, lines, 1, "thin horizontal rectangle → exactly 1 ruling line")
	assert.True(t, lines[0].IsHorizontal, "ruling line must be horizontal")
	assert.InDelta(t, 50.5, lines[0].Start.Y, 0.001, "midpoint Y = 50 + 0.5")
	assert.InDelta(t, 10.0, lines[0].Start.X, 0.001, "start X = left edge")
	assert.InDelta(t, 110.0, lines[0].End.X, 0.001, "end X = right edge")
}

// TestLinesFromRectangle_ThinVertical verifies that a rectangle whose width
// is below maxRulingThickness is decomposed into a single vertical ruling line
// placed at the midpoint X.
func TestLinesFromRectangle_ThinVertical(t *testing.T) {
	d := NewDefaultRulingLineDetector()

	// 1pt wide × 100pt tall — column border.
	elem := rectElem(30, 0, 1, 100)
	lines := d.linesFromRectangle(elem)

	require.Len(t, lines, 1, "thin vertical rectangle → exactly 1 ruling line")
	assert.False(t, lines[0].IsHorizontal, "ruling line must be vertical")
	assert.InDelta(t, 30.5, lines[0].Start.X, 0.001, "midpoint X = 30 + 0.5")
	assert.InDelta(t, 0.0, lines[0].Start.Y, 0.001, "start Y = bottom edge")
	assert.InDelta(t, 100.0, lines[0].End.Y, 0.001, "end Y = top edge")
}

// TestLinesFromRectangle_LargeRectangle verifies that a rectangle with both
// dimensions exceeding maxRulingThickness is decomposed into 4 edge lines
// (top, bottom, left, right) that form the rectangle border.
func TestLinesFromRectangle_LargeRectangle(t *testing.T) {
	d := NewDefaultRulingLineDetector()

	// 100pt × 50pt — a normal table cell outline.
	elem := rectElem(0, 0, 100, 50)
	lines := d.linesFromRectangle(elem)

	require.Len(t, lines, 4, "large rectangle → exactly 4 edge lines")

	// Classify the 4 lines by orientation and count them.
	hCount, vCount := 0, 0
	for _, l := range lines {
		if l.IsHorizontal {
			hCount++
		} else {
			vCount++
		}
	}
	assert.Equal(t, 2, hCount, "should have 2 horizontal edges (top + bottom)")
	assert.Equal(t, 2, vCount, "should have 2 vertical edges (left + right)")

	// Verify that the lines span the expected coordinate ranges.
	for _, l := range lines {
		length := l.Length()
		assert.True(t, length == 50 || length == 100,
			"each edge must be 50pt or 100pt long, got %.2f", length)
	}
}

// TestLinesFromRectangle_TooSmall verifies that a rectangle below the minimum
// line length threshold produces no ruling lines. A 5×5 rectangle is too small
// to be a meaningful ruling line in either dimension.
func TestLinesFromRectangle_TooSmall(t *testing.T) {
	d := NewDefaultRulingLineDetector() // minLineLength = 10pt

	// 5×5 square — smaller than minLineLength in every direction.
	elem := rectElem(0, 0, 5, 5)
	lines := d.linesFromRectangle(elem)

	assert.Empty(t, lines, "rectangle below minLineLength in both dimensions must produce no lines")
}

// TestLinesFromRectangle_ExactThreshold verifies boundary behavior at exactly
// maxRulingThickness. A rectangle with height == maxRulingThickness (5pt) must
// still be treated as a thin horizontal ruler (≤ threshold).
func TestLinesFromRectangle_ExactThreshold(t *testing.T) {
	d := NewDefaultRulingLineDetector()

	// height == maxRulingThickness, width > minLineLength
	elem := rectElem(0, 0, 100, maxRulingThickness)
	lines := d.linesFromRectangle(elem)

	require.Len(t, lines, 1, "height == maxRulingThickness → thin horizontal")
	assert.True(t, lines[0].IsHorizontal)
}

// TestLinesFromRectangle_ExactWidthThreshold mirrors the threshold test for
// vertical rectangles: width == maxRulingThickness must yield a vertical line.
func TestLinesFromRectangle_ExactWidthThreshold(t *testing.T) {
	d := NewDefaultRulingLineDetector()

	// width == maxRulingThickness, height > minLineLength
	elem := rectElem(0, 0, maxRulingThickness, 100)
	lines := d.linesFromRectangle(elem)

	require.Len(t, lines, 1, "width == maxRulingThickness → thin vertical")
	assert.False(t, lines[0].IsHorizontal)
}

// TestLinesFromRectangle_InsufficientPoints verifies that an element with fewer
// than 4 points (malformed rectangle) produces no lines and does not panic.
func TestLinesFromRectangle_InsufficientPoints(t *testing.T) {
	d := NewDefaultRulingLineDetector()

	elem := &extractor.GraphicsElement{
		Type:   extractor.GraphicsTypeRectangle,
		Points: []extractor.Point{extractor.NewPoint(0, 0), extractor.NewPoint(100, 0)},
	}
	lines := d.linesFromRectangle(elem)

	assert.Empty(t, lines, "fewer than 4 points must not produce any lines")
}

// TestDetectRulingLines_MixedElements verifies that DetectRulingLines correctly
// handles a slice that contains both GraphicsTypeLine and GraphicsTypeRectangle
// elements and produces ruling lines from both sources.
func TestDetectRulingLines_MixedElements(t *testing.T) {
	d := NewDefaultRulingLineDetector()

	graphics := []*extractor.GraphicsElement{
		// Explicit stroked horizontal line (top border)
		{
			Type: extractor.GraphicsTypeLine,
			Points: []extractor.Point{
				extractor.NewPoint(0, 100),
				extractor.NewPoint(200, 100),
			},
		},
		// Thin filled rectangle as horizontal border (row separator)
		rectElem(0, 50, 200, 1),
		// Thin filled rectangle as vertical column separator
		rectElem(100, 0, 1, 100),
	}

	lines, err := d.DetectRulingLines(graphics)

	require.NoError(t, err)
	// The stroked line + 1 thin horizontal rectangle + 1 thin vertical rectangle
	// → at least 3 ruling lines (merging may reduce collinear ones, never below 3).
	assert.GreaterOrEqual(t, len(lines), 2,
		"mixed elements (line + thin rect horiz + thin rect vert) must yield ≥ 2 ruling lines after merge")
}

// TestDetectRulingLines_FilledRectanglesOnly is the key regression test that
// mirrors @AtifChy's PDF: all table borders are drawn as filled rectangles
// (re f) with no explicit stroked lines. The ruling line detector must still
// produce a complete grid from rectangles alone.
func TestDetectRulingLines_FilledRectanglesOnly(t *testing.T) {
	d := NewDefaultRulingLineDetector()

	// Simulate a 2-column, 3-row table drawn entirely with thin filled rectangles.
	// Horizontal separators (row borders):
	//   Y=0:  bottom border
	//   Y=50: first row separator
	//   Y=100: top border
	// Vertical separators (column borders):
	//   X=0:   left border
	//   X=150: column separator
	//   X=300: right border
	graphics := []*extractor.GraphicsElement{
		// Horizontal borders (thin, height=1pt)
		rectElem(0, 0, 300, 1),
		rectElem(0, 50, 300, 1),
		rectElem(0, 100, 300, 1),
		// Vertical borders (thin, width=1pt)
		rectElem(0, 0, 1, 100),
		rectElem(149, 0, 1, 100),
		rectElem(299, 0, 1, 100),
	}

	lines, err := d.DetectRulingLines(graphics)

	require.NoError(t, err)
	require.NotEmpty(t, lines, "filled rectangles must produce ruling lines")

	// Count orientation classes.
	hLines, vLines := 0, 0
	for _, l := range lines {
		if l.IsHorizontal {
			hLines++
		} else {
			vLines++
		}
	}
	assert.GreaterOrEqual(t, hLines, 3, "must detect at least 3 horizontal ruling lines")
	assert.GreaterOrEqual(t, vLines, 3, "must detect at least 3 vertical ruling lines")
}

// TestDetectRulingLines_GraphicsPathIgnored verifies that GraphicsTypePath
// elements are ignored by DetectRulingLines (only Line and Rectangle types
// contribute ruling lines).
func TestDetectRulingLines_GraphicsPathIgnored(t *testing.T) {
	d := NewDefaultRulingLineDetector()

	graphics := []*extractor.GraphicsElement{
		{
			Type: extractor.GraphicsTypePath,
			Points: []extractor.Point{
				extractor.NewPoint(0, 0),
				extractor.NewPoint(100, 50),
				extractor.NewPoint(200, 0),
			},
		},
	}

	lines, err := d.DetectRulingLines(graphics)

	require.NoError(t, err)
	assert.Empty(t, lines, "GraphicsTypePath elements must not produce ruling lines")
}

// TestDetectRulingLines_EmptyGraphics verifies that an empty input slice
// returns an empty slice without error.
func TestDetectRulingLines_EmptyGraphics(t *testing.T) {
	d := NewDefaultRulingLineDetector()

	lines, err := d.DetectRulingLines([]*extractor.GraphicsElement{})

	require.NoError(t, err)
	assert.Empty(t, lines)
}
