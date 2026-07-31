// Package table implements table extraction use cases.
package tabledetect

import (
	"fmt"

	"github.com/coregx/gxpdf/internal/extractor"
	domaintable "github.com/coregx/gxpdf/internal/models/table"
)

// TableExtractor extracts table content from detected table regions.
//
// This is the main orchestrator for Phase 2.7 (Table Extraction & Export).
//
// The extractor:
//  1. Takes a detected TableRegion (from Phase 2.6)
//  2. Extracts text content from each cell
//  3. Builds a complete Table with cell content
//  4. Handles both lattice mode (grid) and stream mode (row/column coordinates)
//
// Architecture note:
// This is in the Application layer (use case orchestration).
// It uses:
//   - TableRegion (from Phase 2.6 detection)
//   - domaintable.Table (domain entity from Phase 2.7)
//   - extractor.CellExtractor (application service)
type TableExtractor struct {
	cellExtractor *extractor.CellExtractor
}

// NewTableExtractor creates a new TableExtractor with the given text elements.
func NewTableExtractor(textElements []*extractor.TextElement) *TableExtractor {
	return &TableExtractor{
		cellExtractor: extractor.NewCellExtractor(textElements),
	}
}

// ExtractTable extracts cell content from a detected table region.
//
// Parameters:
//   - region: The detected table region (from Phase 2.6)
//
// Returns a complete Table with extracted cell content, or error.
func (te *TableExtractor) ExtractTable(region *TableRegion) (*domaintable.Table, error) {
	if region == nil {
		return nil, fmt.Errorf("table region is nil")
	}

	// Extract based on detection method
	switch region.Method {
	case MethodLattice:
		return te.extractLatticeTable(region)
	case MethodStream:
		return te.extractStreamTable(region)
	default:
		return nil, fmt.Errorf("unknown extraction method: %s", region.Method)
	}
}

// extractLatticeTable extracts a table using lattice mode (grid structure).
//
// In lattice mode, the table has a well-defined grid from ruling lines.
// We extract text from each cell in the grid.
//
// When ruling lines are available in the region, merged cell detection is
// performed via DetectMergedCells. Owner cells receive RowSpan/ColSpan values
// greater than 1, while covered cells are skipped (left as empty placeholders
// initialized by NewTable).
func (te *TableExtractor) extractLatticeTable(region *TableRegion) (*domaintable.Table, error) {
	if region.Grid == nil {
		return nil, fmt.Errorf("lattice mode requires grid structure")
	}

	grid := region.Grid
	rowCount := grid.RowCount()
	colCount := grid.ColumnCount()

	// Create table (all cells initialized to empty by NewTable).
	tbl, err := domaintable.NewTable(rowCount, colCount)
	if err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	// Set metadata
	tbl.Method = MethodLattice.String()
	// Convert extractor.Rectangle to domaintable.Rectangle
	tbl.Bounds = domaintable.NewRectangle(region.Bounds.X, region.Bounds.Y, region.Bounds.Width, region.Bounds.Height)

	// Detect merged cells when ruling lines are available.
	//
	// DetectMergedCells works in GRID space where rows are sorted ascending
	// (bottom-to-top PDF space, i.e. grid row 0 = visual bottom row).
	// extractLatticeTable iterates with gridRow = rowCount-1-r so that output
	// row 0 corresponds to the visual top row. We must convert grid-space merge
	// coordinates to output-space coordinates before building the lookup maps.
	var mergedOwners map[mergedKey]MergedCellInfo
	var coveredCells map[mergedKey]bool

	if len(region.RulingLines) > 0 {
		gridMerges := DetectMergedCells(grid, region.RulingLines, defaultMergeTolerance)
		if len(gridMerges) > 0 {
			// Convert grid-space row indices to output-space row indices.
			// output row = rowCount - 1 - gridRow
			outputMerges := make([]MergedCellInfo, len(gridMerges))
			for i, m := range gridMerges {
				outputRow := rowCount - 1 - m.Row
				// When a cell spans multiple grid rows (ascending), it occupies
				// grid rows [m.Row, m.Row+m.RowSpan). In output space (descending)
				// those rows become [outputRow-m.RowSpan+1, outputRow].
				// The owner in output space is the smallest output row index,
				// which is outputRow - (m.RowSpan - 1).
				outputMerges[i] = MergedCellInfo{
					Row:     outputRow - (m.RowSpan - 1),
					Col:     m.Col,
					RowSpan: m.RowSpan,
					ColSpan: m.ColSpan,
				}
			}
			mergedOwners = buildMergedMap(outputMerges)
			coveredCells = buildCoveredMap(outputMerges)
		}
	}

	// Extract content from each cell.
	// Grid rows are sorted ascending by Y (bottom-to-top in PDF space).
	// Reverse the iteration so output rows follow natural reading order
	// (top-to-bottom: header first, data rows next, footer last).
	for r := 0; r < rowCount; r++ {
		gridRow := rowCount - 1 - r
		for c := 0; c < colCount; c++ {
			// Skip cells that are covered by a merge from an earlier owner cell.
			// These positions are left as empty placeholder cells (already
			// initialized by NewTable), which is the Variant A approach from ADR-004.
			if coveredCells[mergedKey{r, c}] {
				continue
			}

			gridCell := grid.GetCell(gridRow, c)
			if gridCell == nil {
				continue
			}

			content := te.cellExtractor.ExtractCellContent(gridCell.Bounds)

			domainBounds := domaintable.NewRectangle(gridCell.Bounds.X, gridCell.Bounds.Y, gridCell.Bounds.Width, gridCell.Bounds.Height)

			cell := domaintable.NewCellWithBounds(content, r, c, domainBounds)
			cell = cell.WithAlignment(te.detectAlignment(content, gridCell.Bounds))

			// Apply span information if this cell is a merge owner.
			if info, ok := mergedOwners[mergedKey{r, c}]; ok {
				cell = cell.WithRowSpan(info.RowSpan).WithColSpan(info.ColSpan)
			}

			if err := tbl.SetCell(r, c, cell); err != nil {
				return nil, fmt.Errorf("failed to set cell (%d,%d): %w", r, c, err)
			}
		}
	}

	return tbl, nil
}

// extractStreamTable extracts a table using stream mode (row/column coordinates).
//
// In stream mode, we have row and column boundaries detected from whitespace.
// We build cells from these boundaries.
func (te *TableExtractor) extractStreamTable(region *TableRegion) (*domaintable.Table, error) {
	if len(region.Rows) < 2 || len(region.Columns) < 2 {
		return nil, fmt.Errorf("stream mode requires at least 2 rows and 2 columns")
	}

	rowCount := len(region.Rows) - 1
	colCount := len(region.Columns) - 1

	// Create table
	tbl, err := domaintable.NewTable(rowCount, colCount)
	if err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	// Set metadata
	tbl.Method = MethodStream.String()
	tbl.Bounds = domaintable.NewRectangle(region.Bounds.X, region.Bounds.Y, region.Bounds.Width, region.Bounds.Height)

	// Extract content from each cell
	for r := 0; r < rowCount; r++ {
		for c := 0; c < colCount; c++ {
			// Calculate cell bounds from row/column coordinates
			// PDF coordinates: Y increases upward, so higher Y is higher on page
			//
			// Row coordinate sorting (from WhitespaceAnalyzer.DetectRows):
			// - Rows are sorted ascending (bottom to top): [low_Y, ..., high_Y]
			// - region.Rows[0] = bottom edge of table (lowest Y)
			// - region.Rows[n] = top edge of table (highest Y)
			//
			// Table row indexing convention:
			// - Row 0 = first row = top row (highest Y in PDF coordinates)
			// - Row n = last row = bottom row (lowest Y in PDF coordinates)
			//
			// Therefore: we need to reverse the indexing to map table rows to Y coordinates

			// For row r (0-based from top):
			//   Top edge Y = region.Rows[rowCount - r]     (higher Y)
			//   Bottom edge Y = region.Rows[rowCount - r - 1]  (lower Y)
			y2 := region.Rows[rowCount-r]   // Top of cell (higher Y)
			y1 := region.Rows[rowCount-r-1] // Bottom of cell (lower Y)
			x1 := region.Columns[c]         // Left
			x2 := region.Columns[c+1]       // Right

			// Create cell bounds with positive width and height
			// Rectangle(x, y, width, height) where y is bottom-left corner
			cellBounds := extractor.NewRectangle(x1, y1, x2-x1, y2-y1)

			// Extract text content
			content := te.cellExtractor.ExtractCellContent(cellBounds)

			// Convert to domain rectangle
			domainBounds := domaintable.NewRectangle(x1, y1, x2-x1, y2-y1)

			// Create cell
			cell := domaintable.NewCellWithBounds(content, r, c, domainBounds)

			// Detect alignment
			cell = cell.WithAlignment(te.detectAlignment(content, cellBounds))

			// Set cell in table
			if err := tbl.SetCell(r, c, cell); err != nil {
				return nil, fmt.Errorf("failed to set cell (%d,%d): %w", r, c, err)
			}
		}
	}

	return tbl, nil
}

// detectAlignment detects text alignment within a cell.
//
// This is a simple heuristic based on text position within cell bounds.
// For production, this could be enhanced with more sophisticated algorithms.
func (te *TableExtractor) detectAlignment(content string, bounds extractor.Rectangle) domaintable.TextAlign {
	if len(content) == 0 {
		return domaintable.AlignLeft
	}

	// Find text elements in cell
	elements := te.cellExtractor.FindElementsInBounds(bounds)
	if len(elements) == 0 {
		return domaintable.AlignLeft
	}

	// Calculate average X position of text
	avgX := 0.0
	for _, elem := range elements {
		avgX += elem.X
	}
	avgX /= float64(len(elements))

	// Calculate cell center X
	cellCenterX := bounds.X + bounds.Width/2

	// Determine alignment based on position
	distFromLeft := avgX - bounds.X
	distFromCenter := abs(avgX - cellCenterX)
	distFromRight := bounds.Right() - avgX

	// Threshold: 10% of cell width
	threshold := bounds.Width * 0.1

	if distFromCenter < threshold {
		return domaintable.AlignCenter
	} else if distFromRight < distFromLeft {
		return domaintable.AlignRight
	}

	return domaintable.AlignLeft
}

// ExtractTables extracts multiple tables from detected regions.
//
// This is a convenience method for extracting all tables at once.
func (te *TableExtractor) ExtractTables(regions []*TableRegion) ([]*domaintable.Table, error) {
	tables := make([]*domaintable.Table, 0, len(regions))

	for i, region := range regions {
		tbl, err := te.ExtractTable(region)
		if err != nil {
			return nil, fmt.Errorf("failed to extract table %d: %w", i, err)
		}
		tables = append(tables, tbl)
	}

	return tables, nil
}
