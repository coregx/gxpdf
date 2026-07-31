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
