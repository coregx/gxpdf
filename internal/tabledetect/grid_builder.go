// Package detector implements table detection algorithms.
package tabledetect

import (
	"fmt"
	"math"
	"sort"

	"github.com/coregx/gxpdf/internal/extractor"
)

// Cell represents a single cell in a table grid.
//
// A cell is defined by its position in the grid (row, column)
// and its bounding rectangle.
type Cell struct {
	Row    int                 // Row index (0-based)
	Column int                 // Column index (0-based)
	Bounds extractor.Rectangle // Bounding rectangle
}

// NewCell creates a new Cell.
func NewCell(row, col int, bounds extractor.Rectangle) *Cell {
	return &Cell{
		Row:    row,
		Column: col,
		Bounds: bounds,
	}
}

// String returns a string representation of the cell.
func (c *Cell) String() string {
	return fmt.Sprintf("Cell{row=%d, col=%d, bounds=%s}", c.Row, c.Column, c.Bounds.String())
}

// Grid represents a table grid structure.
//
// A grid is composed of rows and columns defined by ruling lines,
// and cells at each intersection.
//
// This is used for lattice mode table extraction.
//
// Inspired by tabula-java's grid-based extraction.
type Grid struct {
	Rows    []float64 // Y coordinates of horizontal lines (sorted top to bottom)
	Columns []float64 // X coordinates of vertical lines (sorted left to right)
	Cells   [][]*Cell // 2D array of cells [row][column]
}

// NewGrid creates a new Grid.
func NewGrid(rows, columns []float64) *Grid {
	// Sort rows ascending by Y coordinate (bottom to top in PDF space).
	// The final reading-order reversal (top-to-bottom) happens in
	// TableExtractor.ExtractTable when building the output rows.
	sortedRows := make([]float64, len(rows))
	copy(sortedRows, rows)
	sort.Float64s(sortedRows)

	// Sort columns (left to right means ascending X)
	sortedColumns := make([]float64, len(columns))
	copy(sortedColumns, columns)
	sort.Float64s(sortedColumns)

	return &Grid{
		Rows:    sortedRows,
		Columns: sortedColumns,
		Cells:   nil, // Created separately
	}
}

// RowCount returns the number of rows in the grid.
func (g *Grid) RowCount() int {
	if len(g.Rows) <= 1 {
		return 0
	}
	return len(g.Rows) - 1
}

// ColumnCount returns the number of columns in the grid.
func (g *Grid) ColumnCount() int {
	if len(g.Columns) <= 1 {
		return 0
	}
	return len(g.Columns) - 1
}

// GetCell returns the cell at the specified row and column.
func (g *Grid) GetCell(row, col int) *Cell {
	if row < 0 || row >= len(g.Cells) {
		return nil
	}
	if col < 0 || col >= len(g.Cells[row]) {
		return nil
	}
	return g.Cells[row][col]
}

// Bounds returns the bounding rectangle of the entire grid.
func (g *Grid) Bounds() extractor.Rectangle {
	if len(g.Rows) < 2 || len(g.Columns) < 2 {
		return extractor.NewRectangle(0, 0, 0, 0)
	}

	minX := g.Columns[0]
	maxX := g.Columns[len(g.Columns)-1]
	minY := g.Rows[0]
	maxY := g.Rows[len(g.Rows)-1]

	return extractor.NewRectangle(minX, minY, maxX-minX, maxY-minY)
}

// String returns a string representation of the grid.
func (g *Grid) String() string {
	return fmt.Sprintf("Grid{rows=%d, cols=%d, bounds=%s}",
		g.RowCount(), g.ColumnCount(), g.Bounds().String())
}

// DefaultGridBuilder builds a grid structure from ruling lines.
//
// This is the default implementation of the GridBuilder interface.
// The grid is used for lattice mode table extraction, where tables
// have visible borders and grid lines.
//
// Algorithm inspired by tabula-java's SpreadsheetExtractionAlgorithm.
// Reference: tabula-java/technology/tabula/extractors/SpreadsheetExtractionAlgorithm.java
type DefaultGridBuilder struct {
	tolerance float64 // Tolerance for snapping points to grid
}

// NewDefaultGridBuilder creates a new DefaultGridBuilder with default settings.
func NewDefaultGridBuilder() *DefaultGridBuilder {
	return &DefaultGridBuilder{
		tolerance: 2.0, // 2 points tolerance
	}
}

// NewGridBuilder creates a new DefaultGridBuilder with default settings.
// Deprecated: Use NewDefaultGridBuilder instead. Kept for backward compatibility.
func NewGridBuilder() *DefaultGridBuilder {
	return NewDefaultGridBuilder()
}

// WithTolerance sets the tolerance for snapping points.
func (gb *DefaultGridBuilder) WithTolerance(tol float64) *DefaultGridBuilder {
	gb.tolerance = tol
	return gb
}

// BuildGrid creates a grid from ruling lines.
//
// The grid is defined by the intersections of horizontal and vertical lines.
func (gb *DefaultGridBuilder) BuildGrid(lines []*RulingLine) (*Grid, error) {
	if len(lines) == 0 {
		return nil, fmt.Errorf("no ruling lines provided")
	}

	// Separate horizontal and vertical lines
	var horizontal, vertical []*RulingLine
	for _, line := range lines {
		if line.IsHorizontal {
			horizontal = append(horizontal, line)
		} else {
			vertical = append(vertical, line)
		}
	}

	// Need at least 2 horizontal and 2 vertical lines to form a grid
	if len(horizontal) < 2 || len(vertical) < 2 {
		return nil, fmt.Errorf("insufficient lines: need at least 2 horizontal and 2 vertical (got %d H, %d V)",
			len(horizontal), len(vertical))
	}

	// Extract unique Y coordinates from horizontal lines
	rows := gb.extractYCoordinates(horizontal)

	// Extract unique X coordinates from vertical lines
	columns := gb.extractXCoordinates(vertical)

	// Create grid
	grid := NewGrid(rows, columns)

	// Create cells
	grid.Cells = gb.createCells(grid.Rows, grid.Columns)

	return grid, nil
}

// extractYCoordinates extracts unique Y coordinates from horizontal lines.
func (gb *DefaultGridBuilder) extractYCoordinates(horizontal []*RulingLine) []float64 {
	var yCoords []float64
	seen := make(map[int]bool)

	for _, line := range horizontal {
		y := line.Start.Y
		// Snap to grid
		key := int(math.Round(y / gb.tolerance))

		if !seen[key] {
			seen[key] = true
			yCoords = append(yCoords, y)
		}
	}

	return yCoords
}

// extractXCoordinates extracts unique X coordinates from vertical lines.
func (gb *DefaultGridBuilder) extractXCoordinates(vertical []*RulingLine) []float64 {
	var xCoords []float64
	seen := make(map[int]bool)

	for _, line := range vertical {
		x := line.Start.X
		// Snap to grid
		key := int(math.Round(x / gb.tolerance))

		if !seen[key] {
			seen[key] = true
			xCoords = append(xCoords, x)
		}
	}

	return xCoords
}

// createCells creates a 2D array of cells from row and column coordinates.
//
// Each cell is defined by the space between adjacent rows and columns.
func (gb *DefaultGridBuilder) createCells(rows, columns []float64) [][]*Cell {
	if len(rows) < 2 || len(columns) < 2 {
		return nil
	}

	rowCount := len(rows) - 1
	colCount := len(columns) - 1

	cells := make([][]*Cell, rowCount)

	for r := 0; r < rowCount; r++ {
		cells[r] = make([]*Cell, colCount)

		for c := 0; c < colCount; c++ {
			// Cell bounds
			x := columns[c]
			y := rows[r]
			width := columns[c+1] - x
			height := rows[r+1] - y

			bounds := extractor.NewRectangle(x, y, width, height)
			cells[r][c] = NewCell(r, c, bounds)
		}
	}

	return cells
}

// FindCellsFromIntersections builds cells from intersection points.
//
// This is an alternative approach that finds cells by looking at
// intersection points rather than extracting coordinates first.
//
// Inspired by tabula-java's SpreadsheetExtractionAlgorithm.findCells().
func (gb *DefaultGridBuilder) FindCellsFromIntersections(
	horizontal, vertical []*RulingLine,
) ([]*Cell, error) {
	if len(horizontal) < 2 || len(vertical) < 2 {
		return nil, fmt.Errorf("insufficient lines for cells")
	}

	var cells []*Cell

	// For each pair of adjacent horizontal lines
	for i := 0; i < len(horizontal)-1; i++ {
		topLine := horizontal[i]
		bottomLine := horizontal[i+1]

		// For each pair of adjacent vertical lines
		for j := 0; j < len(vertical)-1; j++ {
			leftLine := vertical[j]
			rightLine := vertical[j+1]

			// Find intersection points
			topLeft := topLine.Intersects(leftLine)
			topRight := topLine.Intersects(rightLine)
			bottomLeft := bottomLine.Intersects(leftLine)
			bottomRight := bottomLine.Intersects(rightLine)

			// If all four corners exist, we have a cell
			if topLeft != nil && topRight != nil && bottomLeft != nil && bottomRight != nil {
				// Calculate cell bounds
				x := math.Min(topLeft.X, bottomLeft.X)
				y := math.Min(topLeft.Y, topRight.Y)
				width := math.Max(topRight.X, bottomRight.X) - x
				height := math.Max(bottomLeft.Y, bottomRight.Y) - y

				// Create cell
				bounds := extractor.NewRectangle(x, y, width, height)
				cell := NewCell(i, j, bounds)
				cells = append(cells, cell)
			}
		}
	}

	return cells, nil
}

// BuildGridFromCells creates a grid structure from detected cells.
//
// This is useful when cells are found through intersection detection.
func (gb *DefaultGridBuilder) BuildGridFromCells(cells []*Cell) (*Grid, error) {
	if len(cells) == 0 {
		return nil, fmt.Errorf("no cells provided")
	}

	// Extract unique Y coordinates (row boundaries)
	ySet := make(map[int]float64)
	for _, cell := range cells {
		key1 := int(math.Round(cell.Bounds.Y / gb.tolerance))
		key2 := int(math.Round(cell.Bounds.Top() / gb.tolerance))
		ySet[key1] = cell.Bounds.Y
		ySet[key2] = cell.Bounds.Top()
	}

	rows := make([]float64, 0, len(ySet))
	for _, y := range ySet {
		rows = append(rows, y)
	}

	// Extract unique X coordinates (column boundaries)
	xSet := make(map[int]float64)
	for _, cell := range cells {
		key1 := int(math.Round(cell.Bounds.X / gb.tolerance))
		key2 := int(math.Round(cell.Bounds.Right() / gb.tolerance))
		xSet[key1] = cell.Bounds.X
		xSet[key2] = cell.Bounds.Right()
	}

	columns := make([]float64, 0, len(xSet))
	for _, x := range xSet {
		columns = append(columns, x)
	}

	// Create grid
	grid := NewGrid(rows, columns)

	// Map cells to grid
	grid.Cells = gb.mapCellsToGrid(cells, grid.Rows, grid.Columns)

	return grid, nil
}

// mapCellsToGrid maps detected cells to a grid structure.
func (gb *DefaultGridBuilder) mapCellsToGrid(cells []*Cell, rows, columns []float64) [][]*Cell {
	if len(rows) < 2 || len(columns) < 2 {
		return nil
	}

	rowCount := len(rows) - 1
	colCount := len(columns) - 1

	// Initialize grid
	grid := make([][]*Cell, rowCount)
	for r := 0; r < rowCount; r++ {
		grid[r] = make([]*Cell, colCount)
	}

	// Place each cell in the grid
	for _, cell := range cells {
		// Find row index
		rowIdx := gb.findRowIndex(cell.Bounds.Y, rows)
		if rowIdx < 0 || rowIdx >= rowCount {
			continue
		}

		// Find column index
		colIdx := gb.findColumnIndex(cell.Bounds.X, columns)
		if colIdx < 0 || colIdx >= colCount {
			continue
		}

		// Update cell indices
		cell.Row = rowIdx
		cell.Column = colIdx

		// Place in grid
		grid[rowIdx][colIdx] = cell
	}

	return grid
}

// findRowIndex finds the row index for a given Y coordinate.
func (gb *DefaultGridBuilder) findRowIndex(y float64, rows []float64) int {
	for i := 0; i < len(rows)-1; i++ {
		if math.Abs(y-rows[i]) <= gb.tolerance {
			return i
		}
	}
	return -1
}

// findColumnIndex finds the column index for a given X coordinate.
func (gb *DefaultGridBuilder) findColumnIndex(x float64, columns []float64) int {
	for i := 0; i < len(columns)-1; i++ {
		if math.Abs(x-columns[i]) <= gb.tolerance {
			return i
		}
	}
	return -1
}

// Intersection represents a point where a horizontal and a vertical ruling
// line cross. Each intersection is keyed by its (X, Y) position and records
// which H-line and V-line produced it.
//
// This mirrors tabula-java's Ruling.findIntersections() return type, which
// maps Point2D → Ruling[2] where [0] is the H-line and [1] is the V-line.
type Intersection struct {
	X     float64
	Y     float64
	HLine *RulingLine // horizontal ruling line passing through this point
	VLine *RulingLine // vertical ruling line passing through this point
}

// intersectionKey is a discretised (X, Y) pair used as a map key.
// Coordinates are rounded to the nearest tolerance unit so that floating-point
// jitter does not create spurious distinct points.
type intersectionKey struct {
	xi, yi int
}

// FindIntersections finds all points where a horizontal and a vertical ruling
// line cross, within the given tolerance.
//
// A crossing exists at (vLine.X, hLine.Y) when:
//   - hLine.StartX ≤ vLine.X ≤ hLine.EndX  (H-line spans the V-line's X)
//   - vLine.StartY ≤ hLine.Y ≤ vLine.EndY  (V-line spans the H-line's Y)
//
// Both inequalities are checked with ±tolerance to handle floating-point jitter.
//
// The function is a package-level helper (not a method) so it can be used
// independently of DefaultGridBuilder — for example, in unit tests.
func FindIntersections(hLines, vLines []*RulingLine, tolerance float64) []Intersection {
	if tolerance <= 0 {
		tolerance = 2.0
	}

	invTol := 1.0 / tolerance
	seen := make(map[intersectionKey]bool)
	var result []Intersection

	for _, h := range hLines {
		if !h.IsHorizontal {
			continue
		}
		hY := h.Start.Y
		hMinX := math.Min(h.Start.X, h.End.X)
		hMaxX := math.Max(h.Start.X, h.End.X)

		for _, v := range vLines {
			if v.IsHorizontal {
				continue
			}
			vX := v.Start.X
			vMinY := math.Min(v.Start.Y, v.End.Y)
			vMaxY := math.Max(v.Start.Y, v.End.Y)

			// H-line must span the V-line's X position.
			if vX < hMinX-tolerance || vX > hMaxX+tolerance {
				continue
			}
			// V-line must span the H-line's Y position.
			if hY < vMinY-tolerance || hY > vMaxY+tolerance {
				continue
			}

			xi := int(math.Round(vX * invTol))
			yi := int(math.Round(hY * invTol))
			key := intersectionKey{xi, yi}
			if seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, Intersection{X: vX, Y: hY, HLine: h, VLine: v})
		}
	}

	return result
}

// intersectionPointKey is used to look up intersections by discretised (X, Y).
type intersectionPointKey = intersectionKey

// buildIntersectionMap builds a map from discretised (X, Y) → Intersection
// for O(1) corner-existence checks in BuildGridFromIntersections.
func buildIntersectionMap(intersections []Intersection, tolerance float64) map[intersectionPointKey]*Intersection {
	invTol := 1.0 / tolerance
	m := make(map[intersectionPointKey]*Intersection, len(intersections))
	for i := range intersections {
		ip := &intersections[i]
		key := intersectionPointKey{
			xi: int(math.Round(ip.X * invTol)),
			yi: int(math.Round(ip.Y * invTol)),
		}
		m[key] = ip
	}
	return m
}

// lookupIntersection returns the Intersection at (x, y) ±tolerance, or nil.
func lookupIntersection(m map[intersectionPointKey]*Intersection, x, y, tolerance float64) *Intersection {
	invTol := 1.0 / tolerance
	key := intersectionPointKey{
		xi: int(math.Round(x * invTol)),
		yi: int(math.Round(y * invTol)),
	}
	return m[key]
}

// BuildGridFromIntersections builds a Grid using the tabula-java
// SpreadsheetExtractionAlgorithm.findCells() approach.
//
// Algorithm:
//
//  1. Compute all H×V intersection points and record which H-line and V-line
//     produced each point.
//
//  2. Sort intersection points top-to-bottom, then left-to-right (by Y
//     descending, then X ascending in PDF coordinates where Y increases up).
//
//  3. For each candidate top-left corner P:
//     - Collect xPoints: intersections with same X as P, below P (smaller Y)
//     that share the same V-line as P → these are the left edge of candidate cells
//     - Collect yPoints: intersections with same Y as P, to the right of P
//     that share the same H-line as P → these are the top edge of candidate cells
//     - For each (xPoint, yPoint) pair, check if bottomRight = (yPoint.X, xPoint.Y)
//     exists in the map AND shares the correct H-line (xPoint's H-line) and
//     V-line (yPoint's V-line) → valid cell found
//
//  4. Build a Grid from the collected cells.
//
// This correctly handles non-uniform grids where different columns have
// different row boundaries (e.g., TIME column with one tall cell vs. COURSE
// TITLE column with many short cells). Multi-line text content stays within
// a single cell because cell height matches the actual visual cell height.
//
// Reference: tabula-java SpreadsheetExtractionAlgorithm.findCells().
func (gb *DefaultGridBuilder) BuildGridFromIntersections(
	horizontal, vertical []*RulingLine,
) (*Grid, error) {
	if len(horizontal) < 2 || len(vertical) < 2 {
		return nil, fmt.Errorf("insufficient lines for intersection-based grid: need at least 2 H and 2 V (got %d H, %d V)",
			len(horizontal), len(vertical))
	}

	// Step 1: find all intersection points.
	intersections := FindIntersections(horizontal, vertical, gb.tolerance)
	if len(intersections) < 4 {
		return nil, fmt.Errorf("insufficient intersections for grid: need at least 4 (got %d)", len(intersections))
	}

	// Step 2: sort intersections top-to-bottom then left-to-right.
	// In PDF coordinates Y increases upward, so "top" = highest Y value.
	sort.Slice(intersections, func(i, j int) bool {
		dy := intersections[j].Y - intersections[i].Y // descending Y (higher Y first)
		if math.Abs(dy) > gb.tolerance {
			return dy > 0
		}
		return intersections[i].X < intersections[j].X // ascending X
	})

	// Step 3: build intersection map for O(1) corner lookups.
	imap := buildIntersectionMap(intersections, gb.tolerance)

	// Step 4: find cells using the tabula-java corner-verification algorithm.
	var cells []*Cell

	for i := range intersections {
		topLeft := &intersections[i]

		// Collect xPoints: same X as topLeft, below topLeft (smaller Y in PDF coords),
		// sharing the same V-line.
		var xPoints []*Intersection
		for j := range intersections {
			p := &intersections[j]
			if math.Abs(p.X-topLeft.X) <= gb.tolerance &&
				p.Y < topLeft.Y-gb.tolerance &&
				p.VLine == topLeft.VLine {
				xPoints = append(xPoints, p)
			}
		}

		// Collect yPoints: same Y as topLeft, to the right of topLeft,
		// sharing the same H-line.
		var yPoints []*Intersection
		for j := range intersections {
			p := &intersections[j]
			if math.Abs(p.Y-topLeft.Y) <= gb.tolerance &&
				p.X > topLeft.X+gb.tolerance &&
				p.HLine == topLeft.HLine {
				yPoints = append(yPoints, p)
			}
		}

		// Sort xPoints ascending by Y (closest first = smallest valid cell).
		sort.Slice(xPoints, func(a, b int) bool {
			return xPoints[a].Y > xPoints[b].Y // descending Y → closest to topLeft first
		})
		// Sort yPoints ascending by X (closest first).
		sort.Slice(yPoints, func(a, b int) bool {
			return yPoints[a].X < yPoints[b].X
		})

	outer:
		for _, xPoint := range xPoints {
			// xPoint is a candidate bottom-left (same V-line as topLeft, below).
			for _, yPoint := range yPoints {
				// yPoint is a candidate top-right (same H-line as topLeft, right of it).

				// The bottom-right corner must exist at (yPoint.X, xPoint.Y).
				btmRight := lookupIntersection(imap, yPoint.X, xPoint.Y, gb.tolerance)
				if btmRight == nil {
					continue
				}

				// bottom-right must share:
				//   H-line with xPoint (= the horizontal line at the bottom of the cell)
				//   V-line with yPoint (= the vertical line on the right edge)
				if btmRight.HLine != xPoint.HLine || btmRight.VLine != yPoint.VLine {
					continue
				}

				// Valid cell found.
				x := topLeft.X
				y := xPoint.Y // bottom edge Y (smaller in PDF coords)
				w := yPoint.X - x
				h := topLeft.Y - y // positive: topLeft.Y > xPoint.Y

				bounds := extractor.NewRectangle(x, y, w, h)
				cells = append(cells, &Cell{Bounds: bounds})
				break outer
			}
		}
	}

	if len(cells) == 0 {
		return nil, fmt.Errorf("no cells found from intersections")
	}

	// Step 5: assign row/column indices and build Grid.
	return gb.buildGridFromRawCells(cells)
}

// buildGridFromRawCells assigns row and column indices to unindexed cells
// and builds a Grid. Cells must already have their Bounds set.
//
// Unique Y coordinates (bottom and top of each cell) become grid.Rows.
// Unique X coordinates (left and right of each cell) become grid.Columns.
// Row and column indices are assigned by sorted position.
func (gb *DefaultGridBuilder) buildGridFromRawCells(cells []*Cell) (*Grid, error) {
	invTol := 1.0 / gb.tolerance

	// Collect unique Y values (bottom and top edges).
	ySet := make(map[int]float64)
	for _, c := range cells {
		k1 := int(math.Round(c.Bounds.Y * invTol))
		k2 := int(math.Round(c.Bounds.Top() * invTol))
		ySet[k1] = c.Bounds.Y
		ySet[k2] = c.Bounds.Top()
	}

	// Collect unique X values (left and right edges).
	xSet := make(map[int]float64)
	for _, c := range cells {
		k1 := int(math.Round(c.Bounds.X * invTol))
		k2 := int(math.Round(c.Bounds.Right() * invTol))
		xSet[k1] = c.Bounds.X
		xSet[k2] = c.Bounds.Right()
	}

	rows := make([]float64, 0, len(ySet))
	for _, y := range ySet {
		rows = append(rows, y)
	}
	sort.Float64s(rows) // ascending Y (bottom to top in PDF coords)

	columns := make([]float64, 0, len(xSet))
	for _, x := range xSet {
		columns = append(columns, x)
	}
	sort.Float64s(columns) // ascending X (left to right)

	grid := &Grid{
		Rows:    rows,
		Columns: columns,
	}

	rowCount := grid.RowCount()
	colCount := grid.ColumnCount()

	// Build the 2D cell array. Not all (row, col) positions are guaranteed to
	// have cells (non-uniform grids), so initialize with nils and fill only
	// where a physical cell was found.
	gridCells := make([][]*Cell, rowCount)
	for r := 0; r < rowCount; r++ {
		gridCells[r] = make([]*Cell, colCount)
	}

	for _, cell := range cells {
		// Find row index: which gap [rows[r], rows[r+1]] does this cell's Y occupy?
		rowIdx := -1
		for r := 0; r < rowCount; r++ {
			if math.Abs(cell.Bounds.Y-rows[r]) <= gb.tolerance {
				rowIdx = r
				break
			}
		}

		// Find column index: which gap [columns[c], columns[c+1]] does this cell's X occupy?
		colIdx := -1
		for c := 0; c < colCount; c++ {
			if math.Abs(cell.Bounds.X-columns[c]) <= gb.tolerance {
				colIdx = c
				break
			}
		}

		if rowIdx < 0 || rowIdx >= rowCount || colIdx < 0 || colIdx >= colCount {
			continue
		}

		cell.Row = rowIdx
		cell.Column = colIdx
		gridCells[rowIdx][colIdx] = cell
	}

	grid.Cells = gridCells
	return grid, nil
}
