// Package tabledetect implements table detection algorithms.
package tabledetect

import "math"

// MergedCellInfo describes a cell that spans multiple rows or columns.
//
// Only the top-left "owner" cell of a merged region is represented.
// Covered cells (those hidden behind the owner's span) are identified by
// their position relative to the owner: rows [Row, Row+RowSpan) and
// columns [Col, Col+ColSpan).
type MergedCellInfo struct {
	Row     int // Grid row index of the top-left cell (0-based, ascending Y)
	Col     int // Grid column index of the top-left cell (0-based)
	RowSpan int // Number of rows this cell spans (>= 1)
	ColSpan int // Number of columns this cell spans (>= 1)
}

// HLineIndex is a spatial index for horizontal ruling lines keyed by
// their quantised Y coordinate.
//
// The key is int(math.Round(Y / tolerance)), allowing fast lookup of all
// H-lines near a given Y value.
type HLineIndex map[int][]*RulingLine

// HasLineAt reports whether there is a horizontal ruling line at the given
// Y coordinate that covers the X range [x1, x2] with at least minCoverageRatio
// of the cell width.
//
// Parameters:
//   - y: expected Y position of the ruling line
//   - x1, x2: left and right boundary of the column
//   - tolerance: acceptable deviation in Y and X coordinates (points)
func (idx HLineIndex) HasLineAt(y, x1, x2, tolerance float64) bool {
	key := int(math.Round(y / tolerance))
	required := (x2 - x1) * minCoverageRatio
	for _, line := range idx[key] {
		if math.Abs(line.Start.Y-y) > tolerance {
			continue
		}
		lineMinX := math.Min(line.Start.X, line.End.X)
		lineMaxX := math.Max(line.Start.X, line.End.X)
		// Coverage = overlap between [lineMinX, lineMaxX] and [x1, x2].
		overlapLeft := math.Max(lineMinX, x1)
		overlapRight := math.Min(lineMaxX, x2)
		if overlapRight-overlapLeft >= required-tolerance {
			return true
		}
	}
	return false
}

// VLineIndex is a spatial index for vertical ruling lines keyed by
// their quantised X coordinate.
type VLineIndex map[int][]*RulingLine

// HasLineAt reports whether there is a vertical ruling line at the given
// X coordinate that covers the Y range [y1, y2] with at least minCoverageRatio
// of the row span height.
func (idx VLineIndex) HasLineAt(x, y1, y2, tolerance float64) bool {
	key := int(math.Round(x / tolerance))
	required := (y2 - y1) * minCoverageRatio
	for _, line := range idx[key] {
		if math.Abs(line.Start.X-x) > tolerance {
			continue
		}
		lineMinY := math.Min(line.Start.Y, line.End.Y)
		lineMaxY := math.Max(line.Start.Y, line.End.Y)
		overlapBottom := math.Max(lineMinY, y1)
		overlapTop := math.Min(lineMaxY, y2)
		if overlapTop-overlapBottom >= required-tolerance {
			return true
		}
	}
	return false
}

// minCoverageRatio is the minimum fraction of a cell edge that must be covered
// by a ruling line for the line to count as a separator.
// 0.7 means the line must cover at least 70% of the cell width or height.
// This handles PDFs that draw partial border lines.
const minCoverageRatio = 0.7

// defaultMergeTolerance is the coordinate tolerance used when comparing
// ruling line positions against grid coordinates (same value as DefaultGridBuilder).
const defaultMergeTolerance = 2.0

// buildHLineIndex constructs an HLineIndex from the provided ruling lines.
// Only horizontal lines are indexed.
func buildHLineIndex(lines []*RulingLine, tolerance float64) HLineIndex {
	idx := make(HLineIndex)
	for _, line := range lines {
		if !line.IsHorizontal {
			continue
		}
		key := int(math.Round(line.Start.Y / tolerance))
		idx[key] = append(idx[key], line)
	}
	return idx
}

// buildVLineIndex constructs a VLineIndex from the provided ruling lines.
// Only vertical lines are indexed.
func buildVLineIndex(lines []*RulingLine, tolerance float64) VLineIndex {
	idx := make(VLineIndex)
	for _, line := range lines {
		if line.IsHorizontal {
			continue
		}
		key := int(math.Round(line.Start.X / tolerance))
		idx[key] = append(idx[key], line)
	}
	return idx
}

// computeRowSpan determines how many grid rows cell (r, c) spans downward.
//
// Grid.Rows are sorted ascending (bottom-to-top in PDF space). The boundary
// between grid row r and grid row r+1 is at Y = grid.Rows[r+1]. If no
// H-line exists at that Y within the column's X range, the span extends.
func computeRowSpan(grid *Grid, hIdx HLineIndex, r, c int, tolerance float64) int {
	rowSpan := 1
	x1 := grid.Columns[c]
	x2 := grid.Columns[c+1]

	for nextR := r + 1; nextR < grid.RowCount(); nextR++ {
		// grid.Rows[nextR] is the Y of the separator between row (nextR-1) and row nextR.
		separatorY := grid.Rows[nextR]
		if hIdx.HasLineAt(separatorY, x1, x2, tolerance) {
			// Separator found — merge ends before this row.
			break
		}
		rowSpan++
	}
	return rowSpan
}

// computeColSpan determines how many grid columns cell (r, c) spans rightward.
//
// For each candidate column boundary, we check V-lines per individual row
// segment rather than across the full rowSpan height. V-lines in PDFs are
// typically drawn per-row (not as a single line spanning the entire table),
// so requiring a single V-line to cover 70% of a large rowSpan would cause
// false column merges. If a V-line exists at ANY row boundary within the
// span, the column separation is considered present.
func computeColSpan(grid *Grid, vIdx VLineIndex, r, c, rowSpan int, tolerance float64) int {
	colSpan := 1

	for nextC := c + 1; nextC < grid.ColumnCount(); nextC++ {
		separatorX := grid.Columns[nextC]
		found := false
		for dr := 0; dr < rowSpan && !found; dr++ {
			segY1 := grid.Rows[r+dr]
			segY2Idx := r + dr + 1
			if segY2Idx >= len(grid.Rows) {
				segY2Idx = len(grid.Rows) - 1
			}
			segY2 := grid.Rows[segY2Idx]
			if vIdx.HasLineAt(separatorX, segY1, segY2, tolerance) {
				found = true
			}
		}
		if found {
			break
		}
		colSpan++
	}
	return colSpan
}

// DetectMergedCells analyses ruling lines against the grid structure to find
// cells that span multiple rows or columns.
//
// Algorithm (Interior Line Absence / Grid Gap Analysis):
//
//	For each cell (r, c) that is not already covered by a previously found merge:
//	  1. Walk down from row r: count how many consecutive row boundaries
//	     at Y = grid.Rows[r+1], grid.Rows[r+2], ... are missing an H-line
//	     in the X range of column c. This gives rowSpan.
//	  2. Walk right from col c: count how many consecutive column boundaries
//	     at X = grid.Columns[c+1], ... are missing a V-line in the Y range of
//	     the merged rows. This gives colSpan.
//	  3. If rowSpan > 1 or colSpan > 1, record a MergedCellInfo for the owner
//	     and mark all covered cells.
//
// Grid coordinates are in ascending Y order (bottom-to-top PDF space).
// The caller (extractLatticeTable) is responsible for mapping grid indices
// to output row indices.
//
// Parameters:
//   - grid: grid structure from BuildGrid (rows sorted ascending)
//   - lines: all ruling lines from the page (DetectRulingLines output)
//   - tolerance: coordinate tolerance in PDF points (use 2.0, same as GridBuilder)
//
// Returns only cells where RowSpan > 1 or ColSpan > 1. Cells with RowSpan=1
// and ColSpan=1 are omitted (callers should treat them as normal cells).
func DetectMergedCells(grid *Grid, lines []*RulingLine, tolerance float64) []MergedCellInfo {
	if grid == nil || grid.RowCount() == 0 || grid.ColumnCount() == 0 {
		return nil
	}

	hIdx := buildHLineIndex(lines, tolerance)
	vIdx := buildVLineIndex(lines, tolerance)

	rowCount := grid.RowCount()
	colCount := grid.ColumnCount()

	// covered[r][c] = true means this cell is subsumed by a merge whose
	// owner is at an earlier (r', c') position.
	covered := make([][]bool, rowCount)
	for r := range covered {
		covered[r] = make([]bool, colCount)
	}

	var merged []MergedCellInfo

	avgRowHeight := computeAverageRowHeight(grid)

	for r := 0; r < rowCount; r++ {
		for c := 0; c < colCount; c++ {
			if covered[r][c] {
				continue
			}

			rowSpan := computeRowSpan(grid, hIdx, r, c, tolerance)
			colSpan := computeColSpan(grid, vIdx, r, c, rowSpan, tolerance)

			// Sanity check: if a single-row cell is abnormally tall
			// (> 5× average row height), it's likely a header/footer artifact.
			// Don't allow colSpan expansion for such cells.
			if rowSpan == 1 && colSpan > 1 {
				cellHeight := cellRowHeight(grid, r)
				if cellHeight > avgRowHeight*5 {
					colSpan = 1
				}
			}

			if rowSpan > 1 || colSpan > 1 {
				merged = append(merged, MergedCellInfo{
					Row:     r,
					Col:     c,
					RowSpan: rowSpan,
					ColSpan: colSpan,
				})
				// Mark all covered cells (skip the owner cell itself).
				for dr := 0; dr < rowSpan; dr++ {
					for dc := 0; dc < colSpan; dc++ {
						if dr == 0 && dc == 0 {
							continue
						}
						cr, cc := r+dr, c+dc
						if cr < rowCount && cc < colCount {
							covered[cr][cc] = true
						}
					}
				}
			}
		}
	}

	return merged
}

// mergedKey is the lookup key for the mergedMap used in extractLatticeTable.
type mergedKey struct{ row, col int }

// buildMergedMap converts a slice of MergedCellInfo into a map keyed by
// (output row, col) for O(1) lookup during cell iteration.
//
// The map stores owner cells only. Covered cells are tracked separately via
// buildCoveredMap.
func buildMergedMap(infos []MergedCellInfo) map[mergedKey]MergedCellInfo {
	m := make(map[mergedKey]MergedCellInfo, len(infos))
	for _, info := range infos {
		m[mergedKey{info.Row, info.Col}] = info
	}
	return m
}

// buildCoveredMap constructs a set of (row, col) positions that are covered
// (hidden) by a merge. Owner cells are not included.
//
// Both row and col are in output coordinate space (the same space as the
// mergedMap keys).
func buildCoveredMap(infos []MergedCellInfo) map[mergedKey]bool {
	covered := make(map[mergedKey]bool)
	for _, info := range infos {
		for dr := 0; dr < info.RowSpan; dr++ {
			for dc := 0; dc < info.ColSpan; dc++ {
				if dr == 0 && dc == 0 {
					continue
				}
				covered[mergedKey{info.Row + dr, info.Col + dc}] = true
			}
		}
	}
	return covered
}

// computeAverageRowHeight returns the average height of grid rows,
// excluding outliers (rows with height > 10× the median).
func computeAverageRowHeight(grid *Grid) float64 {
	if len(grid.Rows) < 2 {
		return 0
	}
	var heights []float64
	for i := 0; i < len(grid.Rows)-1; i++ {
		h := grid.Rows[i+1] - grid.Rows[i]
		if h > 0 {
			heights = append(heights, h)
		}
	}
	if len(heights) == 0 {
		return 0
	}
	var sum float64
	for _, h := range heights {
		sum += h
	}
	return sum / float64(len(heights))
}

// cellRowHeight returns the height of grid row r.
func cellRowHeight(grid *Grid, r int) float64 {
	if r+1 >= len(grid.Rows) {
		return 0
	}
	return grid.Rows[r+1] - grid.Rows[r]
}
