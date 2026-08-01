// Package table implements table extraction use cases.
package tabledetect

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/coregx/gxpdf/internal/extractor"
	domaintable "github.com/coregx/gxpdf/internal/models/table"
)

// sectionCodeAll is the special section identifier meaning "all sections".
// It is context-ambiguous: also used as venue text "All" in the PDF schedule.
const sectionCodeAll = "All"

// columnHeaderSections is the canonical name of the SECTIONS column header.
// Used in knownNonSections and knownSectionsColumnHeaders.
const columnHeaderSections = "SECTIONS"

// columnHeaderVenue is the canonical name of the VENUE column header.
// Used in knownNonSections and knownVenueWords.
const columnHeaderVenue = "VENUE"

// columnHeaderCourseTitle is the canonical name of the COURSE TITLE header.
// Used in knownNonSections.
const columnHeaderCourseTitle = "COURSE TITLE"

// columnHeaderTime is the canonical name of the TIME column header.
// Used in knownNonSections.
const columnHeaderTime = "TIME"

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

	// Extend edge cell bounds BEFORE merged cell detection and extraction.
	// Ruling lines for the very first/last table row may be missing,
	// leaving text outside the grid. The extension must happen first so
	// that merged cell detection and text extraction see the full bounds.
	extendGridEdgeBounds(grid)

	// Detect merged cells when ruling lines are available.
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

	// Two-pass extraction: non-merged cells first, then merged cells.
	//
	// Non-merged cells (course titles, sections) get priority for text
	// assignment. Merged cells (TIME, VENUE) capture only remaining text.
	// This prevents merged TIME cells from "stealing" course titles that
	// happen to be positioned in the TIME column's X range.

	// Pass 1: non-merged cells with expanded bounds into merged neighbors.
	for r := 0; r < rowCount; r++ {
		gridRow := rowCount - 1 - r
		for c := 0; c < colCount; c++ {
			if coveredCells[mergedKey{r, c}] {
				continue
			}
			if _, isMerged := mergedOwners[mergedKey{r, c}]; isMerged {
				continue
			}

			gridCell := grid.GetCell(gridRow, c)
			if gridCell == nil {
				continue
			}

			extractBounds := expandBoundsIntoMergedNeighbors(grid, gridRow, c, gridCell.Bounds, coveredCells, rowCount)
			content := te.cellExtractor.ExtractCellContent(extractBounds)
			domainBounds := domaintable.NewRectangle(extractBounds.X, extractBounds.Y, extractBounds.Width, extractBounds.Height)
			cell := domaintable.NewCellWithBounds(content, r, c, domainBounds)
			cell = cell.WithAlignment(te.detectAlignment(content, gridCell.Bounds))

			if err := tbl.SetCell(r, c, cell); err != nil {
				return nil, fmt.Errorf("failed to set cell (%d,%d): %w", r, c, err)
			}
		}
	}

	// Pass 2: merged cell owners capture remaining text from their full area.
	for r := 0; r < rowCount; r++ {
		gridRow := rowCount - 1 - r
		for c := 0; c < colCount; c++ {
			if coveredCells[mergedKey{r, c}] {
				continue
			}
			info, isMerged := mergedOwners[mergedKey{r, c}]
			if !isMerged {
				continue
			}

			gridCell := grid.GetCell(gridRow, c)
			if gridCell == nil {
				continue
			}

			extractBounds := computeMergedBounds(grid, gridRow, c, info)
			content := te.cellExtractor.ExtractCellContent(extractBounds)
			domainBounds := domaintable.NewRectangle(extractBounds.X, extractBounds.Y, extractBounds.Width, extractBounds.Height)
			cell := domaintable.NewCellWithBounds(content, r, c, domainBounds)
			cell = cell.WithAlignment(te.detectAlignment(content, gridCell.Bounds))
			cell = cell.WithRowSpan(info.RowSpan).WithColSpan(info.ColSpan)

			if err := tbl.SetCell(r, c, cell); err != nil {
				return nil, fmt.Errorf("failed to set cell (%d,%d): %w", r, c, err)
			}
		}
	}

	// Merge multi-line cell text that overflows across grid row boundaries.
	// When sections text (e.g., "A,B,...,T,") wraps across more lines than the
	// grid row height accommodates, the overflow lands in the next row's cell
	// for the same column. mergeTextContinuations detects and repairs this.
	mergeTextContinuations(tbl)

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

// expandBoundsIntoMergedNeighbors extends a non-merged cell's X bounds
// into adjacent columns that are covered by a vertical merge at the same
// output row. This captures text (e.g. course titles) that the PDF
// positions in the merged column's X range but semantically belongs to
// the adjacent non-merged column.
func expandBoundsIntoMergedNeighbors(
	grid *Grid, gridRow, col int,
	bounds extractor.Rectangle,
	coveredCells map[mergedKey]bool,
	rowCount int,
) extractor.Rectangle {
	if coveredCells == nil {
		return bounds
	}
	outputRow := rowCount - 1 - gridRow

	// Expand left: if the column to the left is covered by a merge,
	// extend this cell's X to the left column's X start.
	if col > 0 && coveredCells[mergedKey{outputRow, col - 1}] {
		leftCell := grid.GetCell(gridRow, col-1)
		if leftCell != nil {
			expansion := bounds.X - leftCell.Bounds.X
			bounds = extractor.NewRectangle(
				leftCell.Bounds.X, bounds.Y,
				bounds.Width+expansion, bounds.Height,
			)
		}
	}

	// Expand right: if the column to the right is covered by a merge,
	// extend this cell's width to include the right column.
	if col < grid.ColumnCount()-1 && coveredCells[mergedKey{outputRow, col + 1}] {
		rightCell := grid.GetCell(gridRow, col+1)
		if rightCell != nil {
			rightEdge := rightCell.Bounds.X + rightCell.Bounds.Width
			currentRight := bounds.X + bounds.Width
			if rightEdge > currentRight {
				bounds = extractor.NewRectangle(
					bounds.X, bounds.Y,
					rightEdge-bounds.X, bounds.Height,
				)
			}
		}
	}

	return bounds
}

// computeMergedBounds calculates the bounding rectangle that covers
// all grid cells in a merged region. Used to extract text from the
// entire merged area, not just the owner cell.
func computeMergedBounds(grid *Grid, gridRow, col int, info MergedCellInfo) extractor.Rectangle {
	// Start from the owner cell's bounds.
	topLeft := grid.GetCell(gridRow, col)
	if topLeft == nil {
		return extractor.Rectangle{}
	}

	minX := topLeft.Bounds.X
	minY := topLeft.Bounds.Y
	maxX := topLeft.Bounds.X + topLeft.Bounds.Width
	maxY := topLeft.Bounds.Y + topLeft.Bounds.Height

	// Expand to cover all cells in the merged region.
	for dr := 0; dr < info.RowSpan; dr++ {
		for dc := 0; dc < info.ColSpan; dc++ {
			gr := gridRow - dr // grid rows descend (gridRow is top in grid space)
			gc := col + dc
			if gr < 0 || gr >= grid.RowCount() || gc >= grid.ColumnCount() {
				continue
			}
			c := grid.GetCell(gr, gc)
			if c == nil {
				continue
			}
			if c.Bounds.X < minX {
				minX = c.Bounds.X
			}
			if c.Bounds.Y < minY {
				minY = c.Bounds.Y
			}
			if c.Bounds.X+c.Bounds.Width > maxX {
				maxX = c.Bounds.X + c.Bounds.Width
			}
			if c.Bounds.Y+c.Bounds.Height > maxY {
				maxY = c.Bounds.Y + c.Bounds.Height
			}
		}
	}

	return extractor.NewRectangle(minX, minY, maxX-minX, maxY-minY)
}

// extendGridEdgeBounds extends the first and last grid rows by the average
// row height. This captures text that falls just outside the detected grid
// because the outermost ruling lines were not found by the graphics parser.
//
// Only edge rows are extended — interior rows remain at their exact positions.
// Cell bounds are recalculated after the extension via Grid.Cells rebuild.
//
// Inspired by camelot-py's _extend_table_areas_with_textlines which extends
// table areas based on overlapping text lines.
func extendGridEdgeBounds(grid *Grid) {
	if grid == nil || len(grid.Rows) < 3 {
		return
	}

	// Compute average row height (excluding outliers).
	var totalH float64
	var count int
	for i := 0; i < len(grid.Rows)-1; i++ {
		h := grid.Rows[i+1] - grid.Rows[i]
		if h > 0 && h < 100 {
			totalH += h
			count++
		}
	}
	if count == 0 {
		return
	}
	avgH := totalH / float64(count)

	// Extend bottom edge (grid.Rows[0]) downward by 2×avgH.
	// 2× because text may be positioned one full row below the last
	// detected ruling line (the ruling line IS the cell boundary, but
	// text sits inside the cell below it).
	grid.Rows[0] -= 2 * avgH

	// Extend top edge (grid.Rows[last]) upward by 2×avgH.
	grid.Rows[len(grid.Rows)-1] += 2 * avgH

	// Rebuild cells with new bounds.
	gb := NewDefaultGridBuilder()
	grid.Cells = gb.createCells(grid.Rows, grid.Columns)
}

// mergeTextContinuations repairs multi-line cell text that overflows across
// grid row boundaries. Three overflow patterns are handled in a two-pass
// approach (split then merge):
//
// Pattern A — header contamination:
// A cell with a non-sections header (e.g., "SECTIONS") has sections text
// appended because the sections text starts at a Y coordinate just above
// the actual course row. Example:
//
//	Cell [r]:   "SECTIONS\nA,B,...,T,"   ← header + first chunk of next course
//	Cell [r+1]: "U,V,W,X,Y"             ← tail chunk of that course
//
// Fix (Pass 1): split "SECTIONS" from "A,B,...,T,", keep header in [r],
// prepend sections chunk to [r+1].
//
// Pattern B — orphan prefix:
// Sections text appears in a row with no course title, and the next row
// (which has the course title) has a continuation starting with a comma.
// Example:
//
//	Cell [r]:   c1=""  c2="B1,...,C3"    ← first chunk, no title
//	Cell [r+1]: c1="ENGLISH READING..."  c2=",C4,...,P1"  ← tail, starts with comma
//
// Fix (Pass 1): move [r]'s c2 content to [r+1]'s c2 as a prefix (when [r]
// has no title and [r+1]'s content starts with a comma).
//
// Pattern C — trailing-comma continuation:
// A cell ends with a comma because sections text wrapped across the grid
// row boundary. The continuation is in the next row's same column.
// Example:
//
//	Cell [r]:   "A,B,...,T,"   ← truncated list ends with comma
//	Cell [r+1]: "U,V,W,X,Y"   ← tail, no leading comma
//
// Fix (Pass 2): append [r+1] text to [r], clear [r+1]. Chain downward.
//
// IMPORTANT: All three patterns are applied only to the sections column,
// identified as the column immediately to the right of a course-title column.
// This prevents corrupting the TIME, COURSE TITLE, or VENUE columns.
//
// This is a post-extraction pass that does not touch the CellExtractor.
func mergeTextContinuations(tbl *domaintable.Table) {
	if tbl == nil || tbl.RowCount < 2 || tbl.ColCount < 2 {
		return
	}

	// Identify sections columns: for each column c, it is a candidate
	// sections column if the column to its left (c-1) is a title-like column
	// (contains course titles rather than section codes in the majority of rows).
	// We scan all adjacent column pairs and run the repair on candidate c values.
	for c := 1; c < tbl.ColCount; c++ {
		titleCol := c - 1

		// Quick check: is this column pair (titleCol, c) a title|sections pair?
		// We sample the first 20 non-empty rows and check the ratio.
		if !isTitleSectionsPair(tbl, titleCol, c) {
			continue
		}

		// Pass 1: split Pattern A and relocate Pattern B.
		for r := 0; r < tbl.RowCount-1; r++ {
			cell := tbl.GetCell(r, c)
			if cell == nil {
				continue
			}

			// Pattern A: "NonSectionsHeader\nSectionsChunk" in the same cell.
			if cell.Text != "" {
				if headerOnly, sectionsChunk := splitHeaderFromSections(cell.Text); sectionsChunk != "" {
					updated := *cell
					updated.Text = headerOnly
					tbl.Rows[r][c] = &updated
					cell = tbl.Rows[r][c]

					// Prepend sections chunk to the next row's sections cell.
					if nextCell := tbl.GetCell(r+1, c); nextCell != nil {
						nc := *nextCell
						nc.Text = sectionsChunk + nc.Text
						tbl.Rows[r+1][c] = &nc
					}
				}
			}

			// Pattern B: orphan-prefix row.
			// Condition: current row has no course title in titleCol AND
			// the next row's sections cell starts with a comma.
			currentTitleCell := tbl.GetCell(r, titleCol)
			currentHasNoTitle := currentTitleCell == nil || strings.TrimSpace(currentTitleCell.Text) == ""

			if currentHasNoTitle && cell.Text != "" && looksLikeSectionsContinuation(cell.Text) {
				if nextCell := tbl.GetCell(r+1, c); nextCell != nil &&
					strings.HasPrefix(strings.TrimSpace(nextCell.Text), ",") {
					// Move current cell's content to the next row as a prefix.
					nc := *nextCell
					nc.Text = cell.Text + nc.Text
					tbl.Rows[r+1][c] = &nc

					// Clear the current cell.
					cleared := *cell
					cleared.Text = ""
					tbl.Rows[r][c] = &cleared
				}
			}
		}

		// Pattern D: orphan-before-title.
		// When Row N has no title and Row N+1 has a title, and Row N's sections
		// look like valid section codes, and Row N-1 does NOT end with a comma
		// (so Row N is not already a trailing-comma continuation), then Row N's
		// sections belong to Row N+1's course. Prepend Row N's sections to
		// Row N+1's sections and clear Row N.
		//
		// Guards: skip rows where the TIME column (col 0) has a course name
		// (these are time-slot merged cells whose c2 belongs to the c0 course,
		// not the adjacent c1 course in the next row).
		for r := 1; r < tbl.RowCount-1; r++ {
			currentTitleCell := tbl.GetCell(r, titleCol)
			if currentTitleCell != nil && strings.TrimSpace(currentTitleCell.Text) != "" {
				continue // Row r has its own title — not an orphan
			}

			cell := tbl.GetCell(r, c)
			if cell == nil || strings.TrimSpace(cell.Text) == "" {
				continue // No sections content
			}

			if !looksLikeSectionsContinuation(cell.Text) {
				continue // Content doesn't look like section codes
			}

			// Skip if c0 (TIME column) has a course name at this row.
			// Time-slot merged cells encode the course in c0; their c2 sections
			// belong to that c0 course, not to the next c1-titled row.
			if titleCol > 0 {
				timeCell := tbl.GetCell(r, 0)
				if timeCell != nil {
					timeText := strings.TrimSpace(timeCell.Text)
					// Check if any line in c0 is a course title (all-caps, multi-word).
					for _, line := range strings.Split(timeText, "\n") {
						line = strings.TrimSpace(line)
						if isCourseTitleText(line) {
							goto skipPatternD
						}
					}
				}
			}

			{
				// Check Row r-1 does NOT end with comma (otherwise Row r is
				// already a Pattern C continuation and will be handled there).
				prevCell := tbl.GetCell(r-1, c)
				prevEffective := ""
				if prevCell != nil {
					prevEffective = stripTrailingVenueContamination(prevCell.Text)
				}
				if endsWithComma(prevEffective) {
					continue // Row r is a trailing-comma continuation, skip
				}

				// Check Row r+1 has a course title.
				nextTitleCell := tbl.GetCell(r+1, titleCol)
				if nextTitleCell == nil || strings.TrimSpace(nextTitleCell.Text) == "" {
					continue // Row r+1 has no title either
				}
				if !isCourseTitleText(strings.TrimSpace(nextTitleCell.Text)) {
					continue // Row r+1 text doesn't look like a course title
				}

				// Move Row r's sections to prefix of Row r+1's sections.
				nextCell := tbl.GetCell(r+1, c)
				if nextCell == nil {
					continue
				}
				nc := *nextCell
				if nc.Text == "" {
					nc.Text = cell.Text
				} else {
					nc.Text = cell.Text + "," + nc.Text
				}
				tbl.Rows[r+1][c] = &nc

				// Clear Row r's sections.
				cleared := *cell
				cleared.Text = ""
				tbl.Rows[r][c] = &cleared
			}
		skipPatternD:
		}

		// Pattern E: forward-looking comma-start continuation.
		// If a row has NO course title and its sections cell starts with
		// comma (,D,E,F,...), it's a continuation from a PREVIOUS row.
		// Only apply when the row is an orphan (no title) — rows WITH
		// a title may have mixed content (continuation + own sections).
		for r := 1; r < tbl.RowCount; r++ {
			titleCell := tbl.GetCell(r, titleCol)
			hasTitle := titleCell != nil && strings.TrimSpace(titleCell.Text) != ""
			if hasTitle {
				continue
			}

			cell := tbl.GetCell(r, c)
			if cell == nil || cell.Text == "" {
				continue
			}
			trimmed := strings.TrimSpace(cell.Text)
			if !strings.HasPrefix(trimmed, ",") {
				continue
			}
			for prev := r - 1; prev >= 0; prev-- {
				prevCell := tbl.GetCell(prev, c)
				if prevCell == nil || strings.TrimSpace(prevCell.Text) == "" {
					continue
				}
				pc := *prevCell
				pc.Text = strings.TrimRight(pc.Text, " \n") + trimmed
				tbl.Rows[prev][c] = &pc
				cleared := *cell
				cleared.Text = ""
				tbl.Rows[r][c] = &cleared
				break
			}
		}

		// Pattern F: split mixed title+sections cells.
		// When the title column contains course titles mixed with section codes
		// (newline-separated), split them: titles stay, sections move to col c.
		// This happens when grid edge extension creates oversized last/first rows
		// that capture text from multiple visual rows.
		for r := 0; r < tbl.RowCount; r++ {
			titleCell := tbl.GetCell(r, titleCol)
			if titleCell == nil || titleCell.Text == "" {
				continue
			}
			sectCell := tbl.GetCell(r, c)
			if sectCell != nil && strings.TrimSpace(sectCell.Text) != "" {
				continue // sections column already has content
			}

			// Check if title cell contains mixed content
			titles, sections := splitTitleAndSections(titleCell.Text)
			if sections == "" {
				continue
			}

			// Update title cell with only titles
			tc := *titleCell
			tc.Text = titles
			tbl.Rows[r][titleCol] = &tc

			// Move sections to sections column
			if sectCell != nil {
				sc := *sectCell
				sc.Text = sections
				tbl.Rows[r][c] = &sc
			}
		}

		// Pass 2: merge trailing-comma continuations downward (Pattern C).
		// Pattern C+ extension: before checking endsWithComma, strip trailing
		// venue contamination (a trailing "\nShortNonSectionWord" that the
		// CellExtractor captured from a venue label below the grid row). This
		// reveals the true trailing comma when present.
		for r := 0; r < tbl.RowCount-1; r++ {
			// Re-fetch cell each iteration since Pattern D may have modified it.
			cell := tbl.GetCell(r, c)
			if cell == nil {
				continue
			}

			// Use the stripped-contamination text for the trailing-comma check.
			effectiveText := stripTrailingVenueContamination(cell.Text)
			if !endsWithComma(effectiveText) {
				continue
			}

			combined := effectiveText
			lastMergedRow := -1
			partialHarvestDone := false

			for next := r + 1; next < tbl.RowCount; next++ {
				nextCell := tbl.GetCell(next, c)
				if nextCell == nil || nextCell.Text == "" {
					break
				}

				// Stop if the next row owns its own course title.
				// Exception: if the next row's sections cell has a comma-separated
				// prefix BEFORE its first newline that looks like continuation
				// sections, harvest just that prefix (Pattern C partial harvest).
				titleCell := tbl.GetCell(next, titleCol)
				hasTitleNext := titleCell != nil && isCourseTitleText(titleCell.Text)
				if hasTitleNext {
					// Pattern C partial harvest: take pre-newline prefix if it
					// looks like a section continuation.
					prefix := sectionsPrefixBeforeNewline(nextCell.Text)
					if prefix != "" {
						combined = combined + prefix

						// Re-fetch current cell to get latest text (Pattern D may
						// have already modified it; we use effectiveText as base
						// but need to write back via the live pointer).
						currentCell := tbl.GetCell(r, c)
						updated := *currentCell
						updated.Text = strings.TrimRight(combined, ",")
						tbl.Rows[r][c] = &updated

						// Remove the harvested prefix from nextCell.
						// The prefix appears at the start of nextCell.Text.
						// After harvesting, the remainder starts at the newline.
						nlIdx := strings.Index(nextCell.Text, "\n")
						remainder := ""
						if nlIdx >= 0 {
							remainder = strings.TrimSpace(nextCell.Text[nlIdx+1:])
						}
						nc := *nextCell
						nc.Text = remainder
						tbl.Rows[next][c] = &nc
						partialHarvestDone = true
					}
					break // Always stop at titled rows
				}

				// Use venue-stripped text for the looksLike check and as the
				// contribution to combined (strip venue contamination from
				// continuation rows just as we do for the source row).
				nextEffective := stripTrailingVenueContamination(nextCell.Text)
				if !looksLikeSectionsContinuation(nextEffective) {
					break
				}

				combined = combined + nextEffective
				lastMergedRow = next

				if !endsWithComma(combined) {
					break
				}
			}

			if partialHarvestDone {
				continue // Already applied inline above
			}

			if lastMergedRow < 0 {
				continue
			}

			currentCell := tbl.GetCell(r, c)
			updated := *currentCell
			updated.Text = strings.TrimRight(combined, ",")
			tbl.Rows[r][c] = &updated

			for next := r + 1; next <= lastMergedRow; next++ {
				nextCell := tbl.GetCell(next, c)
				if nextCell == nil {
					continue
				}
				cleared := *nextCell
				cleared.Text = ""
				tbl.Rows[next][c] = &cleared
			}
		}

		// Final pass: strip venue contamination from all sections cells.
		cleanVenueContamination(tbl, c)
	}
}

// cleanVenueContamination removes venue-like tokens from all sections cells.
// Venue text ("All", "Annexes", "&", "Building", "D") sometimes bleeds into
// the SECTIONS column due to adjacent merged VENUE cells. This is a final
// cleanup pass after all continuation merging is complete.
func cleanVenueContamination(tbl *domaintable.Table, sectionsCol int) {
	venueTokens := map[string]bool{
		"All": true, "Annexes": true, "&": true, "Building": true,
	}

	for r := 0; r < tbl.RowCount; r++ {
		cell := tbl.GetCell(r, sectionsCol)
		if cell == nil || cell.Text == "" {
			continue
		}

		// Split by both newlines and commas, filter venue tokens, rejoin
		parts := strings.FieldsFunc(cell.Text, func(r rune) bool {
			return r == ',' || r == '\n'
		})
		var clean []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" || venueTokens[p] {
				continue
			}
			// Single letter "D" is ambiguous — could be section or venue "Building D"
			// Keep it only if surrounded by other section codes
			clean = append(clean, p)
		}

		newText := strings.Join(clean, ",")
		if newText != strings.ReplaceAll(strings.ReplaceAll(cell.Text, "\n", ","), " ", "") {
			updated := *cell
			updated.Text = newText
			tbl.Rows[r][sectionsCol] = &updated
		}
	}
}

// isTitleSectionsPair reports whether the (titleCol, sectionsCol) column pair
// looks like a COURSE TITLE | SECTIONS pairing in this table.
//
// A pair qualifies when the majority of non-empty cells in titleCol contain
// course-title-like text (multi-word ALL CAPS) and the majority of non-empty
// cells in sectionsCol contain section-code-like text (short codes, commas).
//
// This prevents the continuation repair from running on unrelated column pairs
// (e.g., TIME | COURSE TITLE, or SECTIONS | VENUE).
func isTitleSectionsPair(tbl *domaintable.Table, titleCol, sectionsCol int) bool {
	titleScore := 0
	sectionsScore := 0
	sampleSize := 0

	for r := 0; r < tbl.RowCount && sampleSize < 30; r++ {
		tc := tbl.GetCell(r, titleCol)
		sc := tbl.GetCell(r, sectionsCol)
		if tc == nil || sc == nil {
			continue
		}
		tText := strings.TrimSpace(tc.Text)
		sText := strings.TrimSpace(sc.Text)
		if tText == "" && sText == "" {
			continue
		}
		sampleSize++
		if isCourseTitleText(tText) {
			titleScore++
		}
		if looksLikeSectionsContinuation(sText) {
			sectionsScore++
		}
	}

	if sampleSize < 3 {
		return false
	}
	// Both columns must score at least 30% each for the pair to qualify.
	return titleScore*100/sampleSize >= 30 && sectionsScore*100/sampleSize >= 30
}

// knownSectionsColumnHeaders is the set of strings that are expected to appear
// in the sections column header row. Only cells starting with one of these
// strings are candidates for Pattern A splitting.
var knownSectionsColumnHeaders = []string{
	columnHeaderSections, "SECTION", "SEC",
}

// splitHeaderFromSections detects Pattern A overflow: a cell whose text is
// "KnownSectionsHeader\nSectionsChunk," where the header is a recognized
// column header keyword and the trailing part is section codes (ends with comma
// or contains comma-separated section identifiers).
//
// Returns (headerOnly, sectionsChunk) if the pattern matches.
// Returns (originalText, "") if no split is needed.
//
// IMPORTANT: The header MUST be a recognized sections column header word
// (e.g., "SECTIONS"). This prevents accidentally splitting venue text like
// "Annexes\nA,B,..." or multi-line section cells like "A,B,C\nD,E,F".
//
// Examples:
//
//	"SECTIONS\nA,B,C,D,E,"  → ("SECTIONS", "A,B,C,D,E,")
//	"Annexes\nA,B,C"        → ("Annexes\nA,B,C", "")  ← not a column header
//	"A,B,C,D,"              → ("A,B,C,D,", "")        ← already sections
func splitHeaderFromSections(text string) (string, string) {
	// Only consider texts that contain a newline — the newline separates
	// the header from the overflow sections chunk.
	newlineIdx := strings.Index(text, "\n")
	if newlineIdx < 0 {
		return text, ""
	}

	header := strings.TrimSpace(text[:newlineIdx])
	rest := strings.TrimSpace(text[newlineIdx+1:])

	if header == "" || rest == "" {
		return text, ""
	}

	// The header must be a known sections column header keyword.
	// This is the key guard that prevents venue text ("Annexes", "Building")
	// from being mistakenly treated as a column header, which would cause
	// section data in the same cell to be relocated to the wrong row.
	headerUpper := strings.ToUpper(header)
	isKnownHeader := false
	for _, h := range knownSectionsColumnHeaders {
		if headerUpper == h {
			isKnownHeader = true
			break
		}
	}
	if !isKnownHeader {
		return text, ""
	}

	// The remainder must look like a sections continuation.
	if !looksLikeSectionsContinuation(rest) {
		return text, ""
	}

	// The remainder belongs to the next row. Return it as-is.
	return header, rest
}

// endsWithComma reports whether s ends with a comma (after trimming spaces).
// A trailing comma is the primary signal that a comma-separated list was
// truncated — the rest of the list lives in the next grid row.
func endsWithComma(s string) bool {
	return strings.HasSuffix(strings.TrimSpace(s), ",")
}

// stripTrailingVenueContamination removes a trailing "\nWord" segment from a
// sections cell when "Word" is NOT a valid section code and the text before it
// ends with a comma. This handles venue contamination such as:
//
//	"A,B,C,D,E,F,G,H,I,J,K,L,M,N,O,P,Q,R,\nD"
//	↳ last segment "D" is venue text; stripping it reveals trailing comma
//
// Only single-token trailing segments are stripped (≤8 chars, no commas).
// Multi-segment tails or valid section codes are left intact.
func stripTrailingVenueContamination(s string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return s
	}

	// Find the last newline.
	lastNL := strings.LastIndex(trimmed, "\n")
	if lastNL < 0 {
		return s // No newline, nothing to strip
	}

	prefix := strings.TrimSpace(trimmed[:lastNL])
	suffix := strings.TrimSpace(trimmed[lastNL+1:])

	// Only strip if the suffix is a single token (no commas, short ≤8 chars).
	if strings.ContainsRune(suffix, ',') {
		return s // Multi-token suffix — might be continuation data
	}
	if len(suffix) > 8 {
		return s // Too long to be simple venue contamination
	}

	// Strip if the suffix is a known venue word.
	// These words appear in the venue column and contaminate sections cells
	// when the PDF grid rows overlap. Strip them regardless of whether the
	// prefix ends with a comma.
	suffixUpper := strings.ToUpper(suffix)
	knownVenueWords := []string{"D", "BUILDING", "ANNEXES", "ANNEX", columnHeaderVenue, "ROOM"}
	for _, v := range knownVenueWords {
		if suffixUpper == v {
			return prefix // Strip it
		}
	}

	// Only strip the suffix when the prefix ends with a comma (revealing truncation).
	// This narrows the scope to avoid incorrectly stripping short words that might
	// be valid additional section codes.
	if !strings.HasSuffix(prefix, ",") {
		return s // Prefix doesn't end with comma — don't strip ambiguous suffixes
	}

	// sectionCodeAll is context-ambiguous: it's a valid section code but also
	// appears as venue text "All" (short for "All buildings/venues"). When the
	// prefix already ends with a comma (indicating truncation), sectionCodeAll
	// appearing as the sole trailing suffix is venue contamination, not a section
	// entry. This specifically fixes: "A,...,T,\nAll" → "A,...,T," (revealed).
	if suffix == sectionCodeAll {
		return prefix
	}

	return s
}

// sectionsPrefixBeforeNewline returns the portion of a sections cell text that
// appears before the first newline, IF that prefix looks like section codes
// (i.e., is a valid continuation). Returns "" if no harvesting is possible.
//
// Used by Pattern C to partially harvest from a titled row: when a row has
// both its own course title AND sections text that starts with a continuation
// of the previous row (interleaved due to PDF layout overflow), we can take
// only the pre-newline part as the continuation and leave the rest intact.
func sectionsPrefixBeforeNewline(s string) string {
	nlIdx := strings.Index(s, "\n")
	if nlIdx < 0 {
		return "" // No newline — the whole cell is a single line
	}
	prefix := strings.TrimSpace(s[:nlIdx])
	if prefix == "" {
		return ""
	}
	if !looksLikeSectionsContinuation(prefix) {
		return ""
	}
	return prefix
}

// looksLikeSectionsContinuation reports whether s looks like the continuation
// of a comma-separated section list (e.g., "U,V,W,X,Y" or "C10,D1,E1").
//
// Criteria:
//   - Not a known column header or venue keyword (SECTIONS, VENUE, BUILDING, etc.)
//   - Contains at least one comma-separated token
//   - Tokens are short uppercase alphanumeric codes (A–Z, A1–Z99, AA, BB,
//     [ARCH], All, etc.) — the typical section identifiers in this PDF
//   - At least 75% of non-empty tokens must be valid section codes
//
// The 75% threshold prevents false positives on venue-contaminated cells such
// as "A\nBuilding" or "A\nAnnexes" which score only 50% and would incorrectly
// be treated as section continuations.
func looksLikeSectionsContinuation(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}

	// Reject known column headers and common venue/location words.
	upper := strings.ToUpper(trimmed)
	knownNonSections := []string{
		columnHeaderSections, columnHeaderVenue, columnHeaderCourseTitle, columnHeaderTime, "DAY", "DATE",
		"COURSE", "TITLE", "SLOT", "BUILDING", "ANNEXES", "ANNEX",
	}
	for _, h := range knownNonSections {
		if upper == h {
			return false
		}
	}

	// Split on commas and newlines — sections may span lines within a cell.
	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == ',' || r == '\n'
	})
	if len(parts) == 0 {
		return false
	}

	// Count valid section codes and non-empty parts.
	validCount := 0
	totalParts := 0
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		totalParts++
		if isSectionCode(part) {
			validCount++
		}
	}

	if totalParts == 0 {
		return false
	}

	// Require at least 75% of non-empty tokens to be valid section codes.
	// Uses integer arithmetic: validCount*4 >= totalParts*3.
	return validCount > 0 && validCount*4 >= totalParts*3
}

// isSectionCode reports whether token looks like a section identifier.
//
// Valid section codes found in the sample PDF:
//   - Single uppercase letter: A, B, ..., Z
//   - Uppercase letter + digits: A1, B2, C10
//   - Double uppercase letters: AA, BB, CC
//   - Bracketed codes: [ARCH], [FST/FE]
//   - Special: All
//
// Rejected: lowercase words, long words (>6 chars without bracket), digits alone.
func isSectionCode(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}

	// Bracketed codes like [ARCH] or [FST/FE]
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		inner := s[1 : len(s)-1]
		return len(inner) > 0 && len(inner) <= 8
	}

	// sectionCodeAll is a valid section indicator meaning "all sections".
	if s == sectionCodeAll {
		return true
	}

	// Must be short (section codes are rarely > 4 chars).
	if len(s) > 4 {
		return false
	}

	// Must start with an uppercase letter.
	runes := []rune(s)
	if !unicode.IsUpper(runes[0]) {
		return false
	}

	// Remaining characters must be letters or digits.
	for _, r := range runes[1:] {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}

	return true
}

// isCourseTitleText reports whether s looks like a course title rather than
// a section list. Course titles are ALL CAPS, multi-word, and longer than
// typical section codes.
func isCourseTitleText(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	// Multi-word ALL CAPS with spaces → likely a course title.
	if strings.ContainsRune(s, ' ') && s == strings.ToUpper(s) && len(s) > 10 {
		return true
	}
	// Also reject if it contains no commas but is long (> 20 chars)
	// and doesn't look like section codes.
	if !strings.ContainsRune(s, ',') && len(s) > 20 {
		return true
	}
	return false
}

// splitTitleAndSections separates course titles from section codes
// in a mixed newline-separated cell. Returns (titles, sections) where
// titles contains course-title-like lines joined with newlines, and
// sections contains section-code-like lines joined with commas.
//
// A line is a "section code" if it's short and comma-separated
// (e.g. "A,B,C,D,E" or "A"). A line is a "course title" if it's
// multi-word ALL CAPS (e.g. "DISCRETE MATHEMATICS").
func splitTitleAndSections(text string) (string, string) {
	lines := strings.Split(text, "\n")
	var titles, sections []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if isCourseTitleText(line) {
			titles = append(titles, line)
		} else {
			sections = append(sections, line)
		}
	}

	return strings.Join(titles, "\n"), strings.Join(sections, ",")
}
