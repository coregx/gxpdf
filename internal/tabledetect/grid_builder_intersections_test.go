package tabledetect

import (
	"math"
	"sort"
	"testing"

	"github.com/coregx/gxpdf/internal/extractor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newH creates a horizontal ruling line at constant Y spanning [x1, x2].
func newH(x1, x2, y float64) *RulingLine {
	return NewRulingLine(
		extractor.NewPoint(x1, y),
		extractor.NewPoint(x2, y),
	)
}

// newV creates a vertical ruling line at constant X spanning [y1, y2].
func newV(x, y1, y2 float64) *RulingLine {
	return NewRulingLine(
		extractor.NewPoint(x, y1),
		extractor.NewPoint(x, y2),
	)
}

// sortIntersections sorts a slice of Intersection by (Y desc, X asc) for
// deterministic comparison in tests.
func sortIntersections(is []Intersection) {
	sort.Slice(is, func(i, j int) bool {
		if math.Abs(is[i].Y-is[j].Y) > 0.01 {
			return is[i].Y > is[j].Y
		}
		return is[i].X < is[j].X
	})
}

// ---------------------------------------------------------------------------
// FindIntersections tests
// ---------------------------------------------------------------------------

// TestFindIntersections_SingleCross verifies a single H/V crossing.
func TestFindIntersections_SingleCross(t *testing.T) {
	h := newH(0, 100, 50)
	v := newV(50, 0, 100)

	got := FindIntersections([]*RulingLine{h}, []*RulingLine{v}, 2.0)

	require.Len(t, got, 1)
	assert.InDelta(t, 50.0, got[0].X, 0.01)
	assert.InDelta(t, 50.0, got[0].Y, 0.01)
	assert.Equal(t, h, got[0].HLine, "should record the H-line")
	assert.Equal(t, v, got[0].VLine, "should record the V-line")
}

// TestFindIntersections_NoOverlap verifies that non-crossing lines produce
// zero intersections.
func TestFindIntersections_NoOverlap(t *testing.T) {
	// H-line at Y=50 from X=0 to X=40.
	// V-line at X=60 from Y=0 to Y=100.
	// They do NOT cross because vX=60 > hMaxX=40.
	h := newH(0, 40, 50)
	v := newV(60, 0, 100)

	got := FindIntersections([]*RulingLine{h}, []*RulingLine{v}, 2.0)
	assert.Empty(t, got, "non-overlapping lines must not produce intersections")
}

// TestFindIntersections_TolerantEndpoint verifies that an endpoint that is
// within tolerance is accepted as an intersection.
func TestFindIntersections_TolerantEndpoint(t *testing.T) {
	// H-line ends at X=50; V-line is at X=51.5 (within 2pt tolerance).
	h := newH(0, 50, 50)
	v := newV(51.5, 0, 100)

	got := FindIntersections([]*RulingLine{h}, []*RulingLine{v}, 2.0)
	require.Len(t, got, 1, "endpoint within tolerance must yield an intersection")
}

// TestFindIntersections_2x2Grid verifies a simple 2×2 grid (4 intersections).
//
// Grid layout (PDF Y-up coords):
//
//	H1 at Y=100, X=[0,200]
//	H2 at Y=0,   X=[0,200]
//	V1 at X=0,   Y=[0,100]
//	V2 at X=200, Y=[0,100]
//
// Expected intersections: (0,100), (200,100), (0,0), (200,0).
func TestFindIntersections_2x2Grid(t *testing.T) {
	h1 := newH(0, 200, 100)
	h2 := newH(0, 200, 0)
	v1 := newV(0, 0, 100)
	v2 := newV(200, 0, 100)

	got := FindIntersections([]*RulingLine{h1, h2}, []*RulingLine{v1, v2}, 2.0)
	sortIntersections(got)

	require.Len(t, got, 4)

	// (0,100)
	assert.InDelta(t, 0.0, got[0].X, 0.01)
	assert.InDelta(t, 100.0, got[0].Y, 0.01)
	// (200,100)
	assert.InDelta(t, 200.0, got[1].X, 0.01)
	assert.InDelta(t, 100.0, got[1].Y, 0.01)
	// (0,0)
	assert.InDelta(t, 0.0, got[2].X, 0.01)
	assert.InDelta(t, 0.0, got[2].Y, 0.01)
	// (200,0)
	assert.InDelta(t, 200.0, got[3].X, 0.01)
	assert.InDelta(t, 0.0, got[3].Y, 0.01)
}

// TestFindIntersections_PartialHLines verifies that a short H-line that does
// not span all V-lines produces fewer intersections than a full-width H-line.
//
// Layout (Y-up):
//
//	H1 at Y=100, spans X=[0,200]     → intersects V1 and V2
//	H2 at Y=50,  spans X=[0,100]     → intersects V1 only (V2 at X=200)
//	H3 at Y=0,   spans X=[0,200]     → intersects V1 and V2
//	V1 at X=0,   Y=[0,100]
//	V2 at X=200, Y=[0,100]
func TestFindIntersections_PartialHLines(t *testing.T) {
	h1 := newH(0, 200, 100) // full width
	h2 := newH(0, 100, 50)  // partial: only covers left half
	h3 := newH(0, 200, 0)   // full width
	v1 := newV(0, 0, 100)
	v2 := newV(200, 0, 100)

	got := FindIntersections([]*RulingLine{h1, h2, h3}, []*RulingLine{v1, v2}, 2.0)

	// Expected: h1∩v1, h1∩v2, h2∩v1 (h2 does not reach v2), h3∩v1, h3∩v2
	assert.Len(t, got, 5, "partial H-line must not intersect out-of-range V-line")

	// Verify h2 does NOT intersect v2.
	for _, p := range got {
		if p.HLine == h2 {
			assert.NotEqual(t, v2, p.VLine, "h2 must not intersect v2")
		}
	}
}

// TestFindIntersections_NoDuplicates verifies that duplicate intersection
// points (same X,Y within tolerance) are deduplicated.
func TestFindIntersections_NoDuplicates(t *testing.T) {
	// Two H-lines at nearly the same Y (within tolerance 2pt).
	h1 := newH(0, 100, 50.0)
	h2 := newH(0, 100, 50.9) // within 2pt of h1
	v := newV(50, 0, 100)

	got := FindIntersections([]*RulingLine{h1, h2}, []*RulingLine{v}, 2.0)
	// Both h1 and h2 cross v at approximately (50, 50), which after quantisation
	// with tolerance=2 map to the same key. Only one should survive.
	assert.Len(t, got, 1, "deduplicated intersections within tolerance")
}

// TestFindIntersections_EmptyInputs verifies that empty H or V slices return
// zero intersections without panicking.
func TestFindIntersections_EmptyInputs(t *testing.T) {
	h := newH(0, 100, 50)
	v := newV(50, 0, 100)

	assert.Empty(t, FindIntersections(nil, []*RulingLine{v}, 2.0))
	assert.Empty(t, FindIntersections([]*RulingLine{h}, nil, 2.0))
	assert.Empty(t, FindIntersections(nil, nil, 2.0))
}

// ---------------------------------------------------------------------------
// BuildGridFromIntersections tests
// ---------------------------------------------------------------------------

// TestBuildGridFromIntersections_SimpleUniform verifies a uniform 3×4 grid
// (3 rows, 4 columns = 12 cells).
//
// H-lines at Y = 300, 200, 100, 0 (4 lines = 3 row intervals)
// V-lines at X = 0, 90, 335, 508, 556 (5 lines = 4 column intervals)
// All H-lines span the full X range, all V-lines span the full Y range.
func TestBuildGridFromIntersections_SimpleUniform(t *testing.T) {
	ys := []float64{0, 100, 200, 300}
	xs := []float64{0, 90, 335, 508, 556}

	var hLines []*RulingLine
	for _, y := range ys {
		hLines = append(hLines, newH(xs[0], xs[len(xs)-1], y))
	}
	var vLines []*RulingLine
	for _, x := range xs {
		vLines = append(vLines, newV(x, ys[0], ys[len(ys)-1]))
	}

	gb := NewDefaultGridBuilder()
	grid, err := gb.BuildGridFromIntersections(hLines, vLines)

	require.NoError(t, err)
	assert.Equal(t, 3, grid.RowCount(), "3 row intervals")
	assert.Equal(t, 4, grid.ColumnCount(), "4 column intervals")

	// Every cell in the uniform grid must be non-nil.
	for r := 0; r < grid.RowCount(); r++ {
		for c := 0; c < grid.ColumnCount(); c++ {
			cell := grid.GetCell(r, c)
			require.NotNil(t, cell, "uniform grid cell [%d,%d] must not be nil", r, c)
			assert.Greater(t, cell.Bounds.Width, 0.0, "cell [%d,%d] must have positive width", r, c)
			assert.Greater(t, cell.Bounds.Height, 0.0, "cell [%d,%d] must have positive height", r, c)
		}
	}
}

// TestBuildGridFromIntersections_NonUniformColumns verifies that a non-uniform
// grid produces the correct cells. The key test case from ADR-005:
//
// TIME column (X=[0,90]) has only one tall cell from Y=0 to Y=300.
// COURSE TITLE / SECTIONS columns (X=[90,556]) have 3 short rows.
//
// H-lines:
//   - H_full: Y=300, spans X=[0,556] (top border)
//   - H_mid1: Y=200, spans X=[90,556] (only right part)
//   - H_mid2: Y=100, spans X=[90,556] (only right part)
//   - H_bot:  Y=0,   spans X=[0,556] (bottom border)
//
// V-lines:
//   - V0: X=0,   Y=[0,300]
//   - V1: X=90,  Y=[0,300]
//   - V2: X=556, Y=[0,300]
//
// Expected cells:
//   - TIME column (c=0): one cell rows [0..300] (Y=0→300, height=300)
//   - Right columns (c=1): three cells each 100pt tall
func TestBuildGridFromIntersections_NonUniformColumns(t *testing.T) {
	hFull1 := newH(0, 556, 300) // top border
	hMid1 := newH(90, 556, 200) // interior: right part only
	hMid2 := newH(90, 556, 100) // interior: right part only
	hBot := newH(0, 556, 0)     // bottom border

	v0 := newV(0, 0, 300)
	v1 := newV(90, 0, 300)
	v2 := newV(556, 0, 300)

	hLines := []*RulingLine{hFull1, hMid1, hMid2, hBot}
	vLines := []*RulingLine{v0, v1, v2}

	gb := NewDefaultGridBuilder()
	grid, err := gb.BuildGridFromIntersections(hLines, vLines)

	require.NoError(t, err)

	// The grid must have 4 distinct Y values: 0, 100, 200, 300.
	assert.Equal(t, 4, len(grid.Rows), "4 unique Y values: 0, 100, 200, 300")
	// 3 distinct X values: 0, 90, 556.
	assert.Equal(t, 3, len(grid.Columns), "3 unique X values: 0, 90, 556")

	// Grid has 3 row intervals × 2 column intervals = 6 possible cells.
	assert.Equal(t, 3, grid.RowCount())
	assert.Equal(t, 2, grid.ColumnCount())

	// Left column (c=0, X=[0,90]): only one actual cell at the bottom row
	// (Y=0→300). The interior H-lines don't span X=0, so no interior cells
	// in column 0. Only the full-height cell at row 0 (Y=0→100) may or may
	// not exist depending on algorithm; what we CAN assert is that the total
	// cell count is 4 (1 left + 3 right).
	totalCells := 0
	for r := 0; r < grid.RowCount(); r++ {
		for c := 0; c < grid.ColumnCount(); c++ {
			if grid.GetCell(r, c) != nil {
				totalCells++
			}
		}
	}
	// 1 TIME cell + 3 right-column cells = 4.
	assert.Equal(t, 4, totalCells, "1 tall TIME cell + 3 right-column cells")

	// The TIME cell (column 0) must span the full height 300.
	var timeCell *Cell
	for r := 0; r < grid.RowCount(); r++ {
		if c := grid.GetCell(r, 0); c != nil {
			timeCell = c
		}
	}
	require.NotNil(t, timeCell, "TIME column must have at least one cell")
	assert.InDelta(t, 300.0, timeCell.Bounds.Height, 1.0,
		"TIME cell must span full height 300pt")

	// Right-column cells (column 1) each have height 100.
	for r := 0; r < grid.RowCount(); r++ {
		c := grid.GetCell(r, 1)
		require.NotNil(t, c, "right column cell [%d,1] must exist", r)
		assert.InDelta(t, 100.0, c.Bounds.Height, 1.0,
			"right column cell [%d,1] must be 100pt tall", r)
	}
}

// TestBuildGridFromIntersections_InsufficientLines verifies that too few lines
// return an error rather than panicking.
func TestBuildGridFromIntersections_InsufficientLines(t *testing.T) {
	gb := NewDefaultGridBuilder()

	// Only one H-line.
	_, err := gb.BuildGridFromIntersections(
		[]*RulingLine{newH(0, 100, 50)},
		[]*RulingLine{newV(0, 0, 100), newV(100, 0, 100)},
	)
	assert.Error(t, err, "single H-line must fail")

	// Only one V-line.
	_, err = gb.BuildGridFromIntersections(
		[]*RulingLine{newH(0, 100, 50), newH(0, 100, 0)},
		[]*RulingLine{newV(50, 0, 100)},
	)
	assert.Error(t, err, "single V-line must fail")
}

// TestBuildGridFromIntersections_CellBoundsPositive verifies that all cells
// produced by BuildGridFromIntersections have positive Width and Height.
func TestBuildGridFromIntersections_CellBoundsPositive(t *testing.T) {
	hLines := []*RulingLine{
		newH(0, 200, 100),
		newH(0, 200, 50),
		newH(0, 200, 0),
	}
	vLines := []*RulingLine{
		newV(0, 0, 100),
		newV(100, 0, 100),
		newV(200, 0, 100),
	}

	gb := NewDefaultGridBuilder()
	grid, err := gb.BuildGridFromIntersections(hLines, vLines)
	require.NoError(t, err)

	for r := 0; r < grid.RowCount(); r++ {
		for c := 0; c < grid.ColumnCount(); c++ {
			cell := grid.GetCell(r, c)
			if cell == nil {
				continue
			}
			assert.Greater(t, cell.Bounds.Width, 0.0,
				"cell [%d,%d] width must be positive", r, c)
			assert.Greater(t, cell.Bounds.Height, 0.0,
				"cell [%d,%d] height must be positive", r, c)
		}
	}
}

// TestBuildGridFromIntersections_CellCountMatchesGeometry verifies that the
// number of detected cells matches the geometrically expected count for a
// uniform grid.
func TestBuildGridFromIntersections_CellCountMatchesGeometry(t *testing.T) {
	tests := []struct {
		name         string
		hCount       int // number of H-lines (n H-lines → n-1 rows)
		vCount       int // number of V-lines (m V-lines → m-1 cols)
		expectedRows int
		expectedCols int
	}{
		{"2x2", 3, 3, 2, 2},
		{"3x4", 4, 5, 3, 4},
		{"1x1", 2, 2, 1, 1},
		{"5x2", 6, 3, 5, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hLines []*RulingLine
			for i := 0; i < tt.hCount; i++ {
				y := float64(i * 100)
				hLines = append(hLines, newH(0, float64((tt.vCount-1)*100), y))
			}
			var vLines []*RulingLine
			for j := 0; j < tt.vCount; j++ {
				x := float64(j * 100)
				vLines = append(vLines, newV(x, 0, float64((tt.hCount-1)*100)))
			}

			gb := NewDefaultGridBuilder()
			grid, err := gb.BuildGridFromIntersections(hLines, vLines)
			require.NoError(t, err, tt.name)

			assert.Equal(t, tt.expectedRows, grid.RowCount(), "%s row count", tt.name)
			assert.Equal(t, tt.expectedCols, grid.ColumnCount(), "%s column count", tt.name)

			// All cells must be non-nil in a uniform grid.
			for r := 0; r < grid.RowCount(); r++ {
				for c := 0; c < grid.ColumnCount(); c++ {
					assert.NotNil(t, grid.GetCell(r, c), "%s cell [%d,%d]", tt.name, r, c)
				}
			}
		})
	}
}

// TestBuildGridFromIntersections_GridAPI verifies that the Grid returned by
// BuildGridFromIntersections satisfies the existing Grid API (RowCount,
// ColumnCount, GetCell, Bounds).
func TestBuildGridFromIntersections_GridAPI(t *testing.T) {
	hLines := []*RulingLine{
		newH(0, 300, 200),
		newH(0, 300, 100),
		newH(0, 300, 0),
	}
	vLines := []*RulingLine{
		newV(0, 0, 200),
		newV(150, 0, 200),
		newV(300, 0, 200),
	}

	gb := NewDefaultGridBuilder()
	grid, err := gb.BuildGridFromIntersections(hLines, vLines)
	require.NoError(t, err)

	assert.Equal(t, 2, grid.RowCount())
	assert.Equal(t, 2, grid.ColumnCount())

	// GetCell out-of-bounds must return nil, not panic.
	assert.Nil(t, grid.GetCell(-1, 0))
	assert.Nil(t, grid.GetCell(0, -1))
	assert.Nil(t, grid.GetCell(grid.RowCount(), 0))
	assert.Nil(t, grid.GetCell(0, grid.ColumnCount()))

	// Bounds must cover the full grid area.
	b := grid.Bounds()
	assert.InDelta(t, 0.0, b.X, 0.01)
	assert.InDelta(t, 0.0, b.Y, 0.01)
	assert.InDelta(t, 300.0, b.Width, 0.01)
	assert.InDelta(t, 200.0, b.Height, 0.01)
}

// TestBuildGridFromIntersections_AdjacentCellEdges verifies that adjacent
// cells share boundary coordinates (no gaps or overlaps).
func TestBuildGridFromIntersections_AdjacentCellEdges(t *testing.T) {
	hLines := []*RulingLine{
		newH(0, 200, 200),
		newH(0, 200, 100),
		newH(0, 200, 0),
	}
	vLines := []*RulingLine{
		newV(0, 0, 200),
		newV(100, 0, 200),
		newV(200, 0, 200),
	}

	gb := NewDefaultGridBuilder()
	grid, err := gb.BuildGridFromIntersections(hLines, vLines)
	require.NoError(t, err)

	// Row 0 and Row 1 must share the boundary at Y=100.
	// In grid coordinates rows are sorted ascending, so:
	//   row 0 = bottom row (Y=0 to Y=100)
	//   row 1 = top row    (Y=100 to Y=200)
	cell00 := grid.GetCell(0, 0)
	cell10 := grid.GetCell(1, 0)
	require.NotNil(t, cell00)
	require.NotNil(t, cell10)

	// cell00 top edge must equal cell10 bottom edge.
	assert.InDelta(t, cell00.Bounds.Top(), cell10.Bounds.Y, 0.01,
		"adjacent cells must share boundary Y")

	// col 0 and col 1 in same row must share boundary at X=100.
	cell01 := grid.GetCell(0, 1)
	require.NotNil(t, cell01)
	assert.InDelta(t, cell00.Bounds.Right(), cell01.Bounds.X, 0.01,
		"adjacent cells must share boundary X")
}

// ---------------------------------------------------------------------------
// Regression: BuildGrid (existing method) must still work unchanged
// ---------------------------------------------------------------------------

// TestBuildGrid_ExistingAPIUnchanged verifies that the original BuildGrid
// method still produces a valid grid, ensuring backward compatibility.
func TestBuildGrid_ExistingAPIUnchanged(t *testing.T) {
	lines := []*RulingLine{
		newH(0, 200, 100),
		newH(0, 200, 0),
		newV(0, 0, 100),
		newV(100, 0, 100),
		newV(200, 0, 100),
	}

	gb := NewDefaultGridBuilder()
	grid, err := gb.BuildGrid(lines)

	require.NoError(t, err)
	assert.Equal(t, 1, grid.RowCount())
	assert.Equal(t, 2, grid.ColumnCount())
	assert.NotNil(t, grid.GetCell(0, 0))
	assert.NotNil(t, grid.GetCell(0, 1))
}
