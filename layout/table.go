package layout

import "sort"

// Table is a layout element that renders a structured grid of cells with
// optional repeating header and footer rows and page-split support.
//
// Column widths are resolved using a four-pass algorithm:
//   - Fixed (UnitPt/Mm/Cm/In): resolved to points directly
//   - Pct (UnitPct): resolved as a percentage of the available table width
//   - Fr (UnitFr): fractional share of remaining space after fixed/pct columns
//   - Auto (UnitAuto): intrinsic content width via the Measurable interface
//
// Table implements Element and Measurable.
type Table struct {
	// Columns defines the width specification for each column.
	// The number of entries here determines the column count.
	Columns []ColumnDef
	// Header rows are placed at the top of every page (repeated on overflow).
	Header []TableRow
	// Body rows form the main content; they split across pages.
	Body []TableRow
	// Footer rows are placed at the bottom of every page.
	Footer []TableRow
	// Style is applied to the whole table (background, border).
	Style Style
	// Fonts is the font resolver used by cell content measurement.
	// If nil, the MockFontResolver approximation is used.
	Fonts FontResolver

	// cont is set on the overflow table that continues a split table on the
	// next page; it is nil for tables built by callers.
	cont *tableContinuation
}

// tableContinuation is the state a split table hands to its continuation.
type tableContinuation struct {
	// width is the area width colWidths were resolved for.
	width float64
	// colWidths keeps the column grid of the whole table: a continuation
	// must not re-resolve Auto columns or the column count from the rows it
	// has left, or its columns would differ from the previous page.
	colWidths []float64
	// carry holds the remainders of row-spanning cells cut by the page
	// break; they occupy their columns from the first body row on.
	carry []carriedCell
}

// carriedCell is the continuation of a row-spanning cell on the next page.
type carriedCell struct {
	cell    TableCell // remaining content and span; style of the original
	col     int
	colSpan int
}

// ColumnDef describes how a single column's width is determined.
type ColumnDef struct {
	// Width specifies the column width using Value units.
	// UnitAuto means the column sizes itself to its content.
	// UnitFr distributes the remaining space fractionally.
	Width Value
}

// TableRow is a single row in a table section (header, body, or footer).
type TableRow struct {
	// Cells is the ordered list of cells in this row.
	Cells []TableCell
	// Style applies row-level styling (background color).
	Style Style
}

// TableCell is a single cell in a TableRow.
type TableCell struct {
	// Content is the list of elements rendered inside the cell.
	// Cells support any Element (Text, nested Table, Image, etc.).
	Content []Element
	// ColSpan is the number of columns this cell spans. 1 is normal.
	ColSpan int
	// RowSpan is the number of rows this cell spans. 1 is normal.
	RowSpan int
	// Style provides cell-level styling: padding, border, background, vertical align.
	Style Style
}

// effectiveColSpan returns ColSpan clamped to at least 1.
func (c *TableCell) effectiveColSpan() int {
	if c.ColSpan < 1 {
		return 1
	}
	return c.ColSpan
}

// effectiveRowSpan returns RowSpan clamped to at least 1.
func (c *TableCell) effectiveRowSpan() int {
	if c.RowSpan < 1 {
		return 1
	}
	return c.RowSpan
}

// PlanLayout implements Element. It resolves column widths, builds the cell
// grid, measures row heights, places header and footer rows on every page,
// and splits body rows across pages when needed.
func (t *Table) PlanLayout(area Area) Plan {
	colWidths := t.columnWidths(area.Width)
	if len(colWidths) == 0 {
		return Plan{Status: Full}
	}

	// Resolve each section into a grid with final row heights.
	header := buildSectionGrid(t.Header, colWidths, nil)
	body := buildSectionGrid(t.Body, colWidths, t.carry())
	footer := buildSectionGrid(t.Footer, colWidths, nil)

	headerHeight := header.height(0, header.numRows())
	footerHeight := footer.height(0, footer.numRows())

	// Check whether all content fits on one page.
	bodyHeight := body.height(0, body.numRows())
	totalHeight := headerHeight + bodyHeight + footerHeight

	if Fits(totalHeight, area.Height) {
		// Everything fits — produce a single plan.
		blocks, _ := buildAllBlocks(header, body, body.numRows(), footer, colWidths, area.Width)
		return Plan{
			Status:   Full,
			Consumed: totalHeight,
			Blocks:   blocks,
		}
	}

	// Determine how many body rows fit on this page.
	available := area.Height - headerHeight - footerHeight
	if available <= 0 {
		// Not even a header+footer fits; force-place header only.
		if headerHeight == 0 {
			return Plan{Status: Nothing}
		}
		blocks, _ := buildAllBlocks(header, body, 0, footer, colWidths, area.Width)
		overflow := t.overflowTable(0, t.carry(), colWidths, area.Width)
		status := Partial
		if overflow == nil {
			status = Full // no body rows remain
		}
		return Plan{
			Status:   status,
			Consumed: headerHeight + footerHeight,
			Blocks:   blocks,
			Overflow: overflow,
		}
	}

	// Walk body rows until we exceed available space.
	splitAt := 0
	usedHeight := 0.0
	for i, h := range body.heights {
		if !Fits(usedHeight+h, available) {
			// This row doesn't fit; split before it.
			break
		}
		usedHeight += h
		splitAt = i + 1
	}

	if splitAt == 0 {
		// Nothing from the body fits on this page.
		if body.numRows() == 0 {
			// No body rows — full fit with header/footer only.
			blocks, _ := buildAllBlocks(header, body, 0, footer, colWidths, area.Width)
			return Plan{
				Status:   Full,
				Consumed: headerHeight + footerHeight,
				Blocks:   blocks,
			}
		}
		// No body row fits on this page.
		return Plan{Status: Nothing}
	}

	// Build plan for what fits.
	consumed := headerHeight + body.height(0, splitAt) + footerHeight
	blocks, carry := buildAllBlocks(header, body, splitAt, footer, colWidths, area.Width)

	status := Partial
	var overflow Element
	if splitAt >= body.numRows() {
		status = Full
	} else {
		overflow = t.overflowTable(splitAt, carry, colWidths, area.Width)
	}

	return Plan{
		Status:   status,
		Consumed: consumed,
		Blocks:   blocks,
		Overflow: overflow,
	}
}

// MinWidth implements Measurable. It returns the sum of each column's minimum
// content width (longest unbreakable word) across all cells.
func (t *Table) MinWidth() float64 {
	if len(t.Columns) == 0 {
		return 0
	}
	numCols := t.numCols()
	colMin := make([]float64, numCols)
	allRows := combineRows(t.Header, t.Body, t.Footer)
	measureIntrinsicWidths(allRows, numCols, colMin, nil)
	total := 0.0
	for _, w := range colMin {
		total += w
	}
	return total
}

// MaxWidth implements Measurable. It returns the sum of each column's maximum
// content width (full content without wrapping) across all cells.
func (t *Table) MaxWidth() float64 {
	if len(t.Columns) == 0 {
		return 0
	}
	numCols := t.numCols()
	colMax := make([]float64, numCols)
	allRows := combineRows(t.Header, t.Body, t.Footer)
	measureIntrinsicWidths(allRows, numCols, nil, colMax)
	total := 0.0
	for _, w := range colMax {
		total += w
	}
	return total
}

// --- numCols ---

// numCols returns the number of columns determined by t.Columns.
// If t.Columns is empty, it scans all rows to find the maximum column count.
func (t *Table) numCols() int {
	if len(t.Columns) > 0 {
		return len(t.Columns)
	}
	max := 0
	all := combineRows(t.Header, t.Body, t.Footer)
	for ri := range all {
		n := 0
		for ci := range all[ri].Cells {
			n += all[ri].Cells[ci].effectiveColSpan()
		}
		if n > max {
			max = n
		}
	}
	return max
}

// combineRows concatenates three row slices without allocating a large backing
// array eagerly — it returns a pre-allocated slice large enough for all rows.
func combineRows(a, b, c []TableRow) []TableRow {
	result := make([]TableRow, 0, len(a)+len(b)+len(c))
	result = append(result, a...)
	result = append(result, b...)
	result = append(result, c...)
	return result
}

// --- Column width resolution ---

// resolveColumnWidths computes the width of every column in points.
//
// Resolution order:
//  1. Fixed units (Pt, Mm, Cm, In) — resolved directly.
//  2. Pct — percentage of available table width.
//  3. Auto — content-driven min/max measurement.
//  4. Fr — fractional share of remaining space after the above.
func (t *Table) resolveColumnWidths(numCols int, tableWidth float64) []float64 {
	widths := make([]float64, numCols)

	// Handle case where no ColumnDef are provided: distribute equally.
	if len(t.Columns) == 0 {
		w := tableWidth / float64(numCols)
		for i := range widths {
			widths[i] = w
		}
		return widths
	}

	usedWidth := 0.0
	totalFr := 0.0

	// First pass: resolve fixed and pct columns.
	for i := 0; i < numCols; i++ {
		col := t.columnAt(i)
		switch col.Width.Unit {
		case UnitPt, UnitMm, UnitCm, UnitIn:
			w := col.Width.Resolve(tableWidth, 12)
			widths[i] = w
			usedWidth += w
		case UnitPct:
			w := col.Width.Resolve(tableWidth, 12)
			widths[i] = w
			usedWidth += w
		case UnitFr:
			totalFr += col.Width.Amount
		case UnitAuto:
			// handled in second pass
		}
	}

	// Second pass: auto columns — measure content.
	allRows := combineRows(t.Header, t.Body, t.Footer)
	autoColMax := make([]float64, numCols)
	autoColMin := make([]float64, numCols)
	measureIntrinsicWidths(allRows, numCols, autoColMin, autoColMax)

	for i := 0; i < numCols; i++ {
		col := t.columnAt(i)
		if col.Width.Unit == UnitAuto {
			remaining := tableWidth - usedWidth
			if remaining < 0 {
				remaining = 0
			}
			w := autoColMax[i]
			if w > remaining {
				w = autoColMin[i]
			}
			widths[i] = w
			usedWidth += w
		}
	}

	// Third pass: Fr columns share remaining space.
	if totalFr > 0 {
		remaining := tableWidth - usedWidth
		if remaining < 0 {
			remaining = 0
		}
		for i := 0; i < numCols; i++ {
			col := t.columnAt(i)
			if col.Width.Unit == UnitFr {
				widths[i] = remaining * (col.Width.Amount / totalFr)
			}
		}
	}

	return widths
}

// columnAt returns the ColumnDef at index i, or an Auto column if out of range.
func (t *Table) columnAt(i int) ColumnDef {
	if i < len(t.Columns) {
		return t.Columns[i]
	}
	return ColumnDef{Width: Auto()}
}

// measureIntrinsicWidths measures min/max widths for Auto column sizing.
// For single-column cells it updates the column arrays directly.
// For colspan cells it distributes any deficit equally.
// Passing nil for colMin or colMax skips updating that slice.
func measureIntrinsicWidths(rows []TableRow, numCols int, colMin, colMax []float64) {
	// First pass: single-column cells only.
	for ri := range rows {
		colIdx := 0
		for ci := range rows[ri].Cells {
			cell := &rows[ri].Cells[ci]
			span := cell.effectiveColSpan()
			if colIdx >= numCols {
				break
			}
			if span == 1 {
				mn, mx := cellIntrinsicWidths(cell)
				if colMin != nil && mn > colMin[colIdx] {
					colMin[colIdx] = mn
				}
				if colMax != nil && mx > colMax[colIdx] {
					colMax[colIdx] = mx
				}
			}
			colIdx += span
		}
	}

	// Second pass: colspan cells — distribute deficit across spanned columns.
	for ri := range rows {
		colIdx := 0
		for ci := range rows[ri].Cells {
			cell := &rows[ri].Cells[ci]
			span := cell.effectiveColSpan()
			if colIdx >= numCols {
				break
			}
			if span > 1 {
				distributeColspanWidth(cell, colIdx, span, numCols, colMin, colMax)
			}
			colIdx += span
		}
	}
}

// distributeColspanWidth distributes any width deficit from a colspan cell
// equally across the spanned columns.
func distributeColspanWidth(cell *TableCell, colIdx, span, numCols int, colMin, colMax []float64) {
	end := colIdx + span
	if end > numCols {
		end = numCols
		span = end - colIdx
	}
	if span <= 0 {
		return
	}
	mn, mx := cellIntrinsicWidths(cell)

	if colMin != nil {
		spanMin := 0.0
		for c := colIdx; c < end; c++ {
			spanMin += colMin[c]
		}
		if mn > spanMin {
			per := (mn - spanMin) / float64(span)
			for c := colIdx; c < end; c++ {
				colMin[c] += per
			}
		}
	}
	if colMax != nil {
		spanMax := 0.0
		for c := colIdx; c < end; c++ {
			spanMax += colMax[c]
		}
		if mx > spanMax {
			per := (mx - spanMax) / float64(span)
			for c := colIdx; c < end; c++ {
				colMax[c] += per
			}
		}
	}
}

// cellIntrinsicWidths returns the (minWidth, maxWidth) for a cell, including
// its padding. For cells with Measurable content, delegates to MinWidth/MaxWidth.
// The font resolver is embedded in each Element at construction time, so it is
// not needed here directly.
func cellIntrinsicWidths(cell *TableCell) (minW, maxW float64) {
	s := cell.Style.effective()
	fontSize := s.FontSize
	pad := s.Padding.Resolve(0, 0, fontSize)
	padH := pad.Left + pad.Right

	for _, elem := range cell.Content {
		if m, ok := elem.(Measurable); ok {
			mn := m.MinWidth()
			mx := m.MaxWidth()
			if mn+padH > minW {
				minW = mn + padH
			}
			if mx+padH > maxW {
				maxW = mx + padH
			}
		}
	}
	if minW == 0 && maxW == 0 {
		minW = padH
		maxW = padH
	}
	return minW, maxW
}

// --- Section grid ---

// sectionGrid is one table section (header, body or footer) resolved into a
// grid: every cell at its origin row and column with clamped spans, and the
// final row heights. Building the whole grid before measuring is what lets a
// row-spanning cell take part in the heights of all rows it covers
// (ADR-007 §1–§2).
type sectionGrid struct {
	rows    []TableRow
	heights []float64
	// cells holds every originating cell, ordered by origin row, then column.
	cells []gridCell
	// covered[r][c] reports that column c of row r belongs to a cell that
	// originates in an earlier row. covered[r] is nil when no column of row r
	// is covered, which is the case for every row of a table without row spans.
	covered [][]bool
}

// gridCell is a cell placed in a sectionGrid.
type gridCell struct {
	cell    TableCell
	row     int     // origin row
	col     int     // first column
	colSpan int     // clamped to the table width and to columns already occupied
	rowSpan int     // clamped to the end of the section
	width   float64 // total width of the spanned columns
	// carried marks the continuation of a cell that originates on a previous
	// page; the background of the row it starts in here does not apply to it.
	carried bool
}

// buildSectionGrid places the cells of rows into a grid and computes the row
// heights. Carried cells (continuations of row spans cut by a page break)
// are placed first, at their own columns of the first row. Cells then fill
// the free columns of their row from left to right; a column stays occupied
// for the rows a cell spans. RowSpan is clamped to the end of the section
// (the HTML row-group rule) and ColSpan to the table width and to the next
// occupied column, so cell boxes never overlap.
func buildSectionGrid(rows []TableRow, colWidths []float64, carry []carriedCell) *sectionGrid {
	g := &sectionGrid{
		rows:    rows,
		heights: make([]float64, len(rows)),
		covered: make([][]bool, len(rows)),
	}
	p := gridPlacer{g: g, colWidths: colWidths, occupiedUntil: make([]int, len(colWidths))}

	if len(rows) > 0 {
		for i := range carry {
			p.placeCarried(&carry[i])
		}
	}
	for r := range rows {
		p.markCovered(r)
		p.placeRow(r)
		if r == 0 && len(carry) > 0 {
			// Only first-row cells exist so far: order them left to right
			// together with the carried cells.
			sort.SliceStable(g.cells, func(a, b int) bool { return g.cells[a].col < g.cells[b].col })
		}
	}

	g.computeHeights()
	return g
}

// gridPlacer assigns cells to grid positions for buildSectionGrid.
type gridPlacer struct {
	g         *sectionGrid
	colWidths []float64
	// occupiedUntil[c] is the first row at which column c is free again.
	occupiedUntil []int
}

// place adds a cell at (row, col) and occupies its columns for its rows.
func (p *gridPlacer) place(gc *gridCell) {
	gc.width = sumColWidths(p.colWidths, gc.col, gc.col+gc.colSpan)
	p.g.cells = append(p.g.cells, *gc)
	for c := gc.col; c < gc.col+gc.colSpan; c++ {
		p.occupiedUntil[c] = gc.row + gc.rowSpan
	}
}

// placeCarried places a carried cell at its own columns of the first row.
func (p *gridPlacer) placeCarried(cc *carriedCell) {
	nCols := len(p.colWidths)
	if cc.col >= nCols || p.occupiedUntil[cc.col] > 0 {
		return // cannot happen for a carry built from a valid grid
	}
	p.place(&gridCell{
		cell:    cc.cell,
		row:     0,
		col:     cc.col,
		colSpan: min(max(cc.colSpan, 1), nCols-cc.col),
		rowSpan: min(cc.cell.effectiveRowSpan(), p.g.numRows()),
		carried: true,
	})
}

// markCovered records the columns of row r occupied by earlier cells.
func (p *gridPlacer) markCovered(r int) {
	for c, until := range p.occupiedUntil {
		if until > r {
			if p.g.covered[r] == nil {
				p.g.covered[r] = make([]bool, len(p.occupiedUntil))
			}
			p.g.covered[r][c] = true
		}
	}
}

// placeRow places the cells of row r into its free columns, left to right.
// Cells beyond the last free column are dropped.
func (p *gridPlacer) placeRow(r int) {
	nCols := len(p.colWidths)
	cells := p.g.rows[r].Cells
	col := 0
	for ci := range cells {
		for col < nCols && p.occupiedUntil[col] > r {
			col++
		}
		if col >= nCols {
			return
		}
		cell := &cells[ci]
		colSpan := min(cell.effectiveColSpan(), nCols-col)
		for k := 1; k < colSpan; k++ {
			if p.occupiedUntil[col+k] > r {
				colSpan = k
				break
			}
		}
		p.place(&gridCell{
			cell:    *cell,
			row:     r,
			col:     col,
			colSpan: colSpan,
			rowSpan: min(cell.effectiveRowSpan(), p.g.numRows()-r),
		})
		col += colSpan
	}
}

// computeHeights sets the row heights in two passes (ADR-007 §2):
//
//  1. every row is as tall as its tallest single-row cell;
//  2. each row-spanning cell, shortest span first, grows the rows it covers
//     until their sum fits the cell (see growRows).
//
// Settling shorter spans first means a longer span only adds what is still
// missing after the spans nested inside it, instead of growing rows that a
// nested span grows again (rows 10/10/10, span 3 needing 90, span 2 needing
// 80: 90 in total this way, 110 the other way). CSS leaves the order
// unspecified (csswg-drafts#4418).
func (g *sectionGrid) computeHeights() {
	need := make([]float64, len(g.cells))
	var spanning []int
	for i := range g.cells {
		gc := &g.cells[i]
		need[i] = measureCellHeight(&gc.cell, gc.width)
		if gc.rowSpan == 1 {
			if need[i] > g.heights[gc.row] {
				g.heights[gc.row] = need[i]
			}
			continue
		}
		spanning = append(spanning, i)
	}

	// cells are in origin-row order, so the stable sort keeps spans of equal
	// length in top-to-bottom order.
	sort.SliceStable(spanning, func(a, b int) bool {
		return g.cells[spanning[a]].rowSpan < g.cells[spanning[b]].rowSpan
	})
	for _, i := range spanning {
		gc := &g.cells[i]
		spanned := g.heights[gc.row : gc.row+gc.rowSpan]
		growRows(spanned, need[i]-sumFloats(spanned))
	}
}

// growRows adds deficit to rows in proportion to their current heights, so
// empty rows do not grow; when every row is empty the last one takes it all.
// This is the rule proposed for CSS Tables 3 (csswg-drafts#4418) and encoded
// in the WPT test css-tables/tentative/rowspan-height-redistribution.html.
// The last row receives the rounding remainder of the shares; the rows then
// sum to the previous total plus deficit up to floating-point rounding, which
// content layout absorbs with its fit tolerance.
func growRows(rows []float64, deficit float64) {
	if deficit <= 0 || len(rows) == 0 {
		return
	}
	last := len(rows) - 1
	total := sumFloats(rows)
	if total <= 0 {
		rows[last] += deficit
		return
	}
	given := 0.0
	for i := 0; i < last; i++ {
		share := deficit * rows[i] / total
		rows[i] += share
		given += share
	}
	rows[last] += deficit - given
}

// numRows returns the number of rows in the section.
func (g *sectionGrid) numRows() int {
	return len(g.heights)
}

// height returns the total height of rows [from, to).
func (g *sectionGrid) height(from, to int) float64 {
	return sumFloats(g.heights[from:to])
}

// sumFloats returns the sum of values, added in order.
func sumFloats(values []float64) float64 {
	total := 0.0
	for _, v := range values {
		total += v
	}
	return total
}

// sumColWidths returns the total width of columns [start, end).
func sumColWidths(colWidths []float64, start, end int) float64 {
	total := 0.0
	for c := start; c < end; c++ {
		total += colWidths[c]
	}
	return total
}

// measureCellHeight returns the total height a cell needs including padding.
// Font resolvers are embedded within each Element (e.g. Text.Fonts) at
// construction time, so no FontResolver argument is required here.
func measureCellHeight(cell *TableCell, cellWidth float64) float64 {
	s := cell.Style.effective()
	fontSize := s.FontSize
	pad := s.Padding.Resolve(cellWidth, 0, fontSize)
	padV := pad.Top + pad.Bottom
	innerWidth := cellWidth - pad.Left - pad.Right
	if innerWidth < 0 {
		innerWidth = 0
	}

	if len(cell.Content) == 0 {
		// Empty cell: at least one line height.
		return fontSize*s.LineHeight + padV
	}

	// Measure by running layout on each content element with unlimited height.
	contentH := 0.0
	for _, elem := range cell.Content {
		plan := elem.PlanLayout(Area{Width: innerWidth, Height: 1e9})
		contentH += plan.Consumed
	}
	return contentH + padV
}

// --- Block building ---

// buildAllBlocks converts the sections into positioned Blocks: the whole
// header, body rows [0, bodyEnd), then the whole footer, starting at Y=0.
// It also returns the continuations of body cells whose row span is cut at
// bodyEnd, for the table that continues on the next page.
func buildAllBlocks(header, body *sectionGrid, bodyEnd int, footer *sectionGrid, colWidths []float64, tableWidth float64) ([]Block, []carriedCell) {
	nCols := len(colWidths)
	colOffsets := make([]float64, nCols)
	for i := 1; i < nCols; i++ {
		colOffsets[i] = colOffsets[i-1] + colWidths[i-1]
	}

	totalRows := header.numRows() + bodyEnd + footer.numRows()
	blocks := make([]Block, 0, totalRows*4) // rows + bg + cells
	cursorY := 0.0
	var carry []carriedCell
	blocks, cursorY, _ = header.appendBlocks(blocks, header.numRows(), colOffsets, tableWidth, cursorY)
	blocks, cursorY, carry = body.appendBlocks(blocks, bodyEnd, colOffsets, tableWidth, cursorY)
	blocks, _, _ = footer.appendBlocks(blocks, footer.numRows(), colOffsets, tableWidth, cursorY)
	return blocks, carry
}

// appendBlocks appends the blocks of rows [0, end) to blocks, starting at
// rowY, and returns the extended slice, the Y below the last row, and the
// continuations of cells cut at end. For each row it emits the row
// background, then the cells that originate in the row. A spanning cell is
// as tall as the rows it covers; one that reaches past end is emitted as a
// fragment of the rows before end (ADR-007 §4).
func (g *sectionGrid) appendBlocks(blocks []Block, end int, colOffsets []float64, tableWidth, rowY float64) ([]Block, float64, []carriedCell) {
	var carry []carriedCell
	next := 0 // index of the first cell not yet emitted
	for r := 0; r < end; r++ {
		rowH := g.heights[r]
		rowStyle := g.rows[r].Style.effective()
		if rowStyle.Background != nil {
			blocks = g.appendRowBackground(blocks, r, *rowStyle.Background, colOffsets, tableWidth, rowY)
		}

		for ; next < len(g.cells) && g.cells[next].row == r; next++ {
			gc := &g.cells[next]

			// A row's background covers the cells that originate in it,
			// including the rows they span (CSS 2.2 table layers). A carried
			// cell originates on a previous page and has that row's
			// background in its own style already.
			var rowBg *Color
			if gc.rowSpan > 1 && !gc.carried {
				rowBg = rowStyle.Background
			}

			if r+gc.rowSpan <= end {
				blocks = append(blocks, buildCellBlock(gc, colOffsets, g.height(r, r+gc.rowSpan), rowY, rowBg))
				continue
			}
			block, rest := layoutCell(gc, colOffsets, g.height(r, end), rowY, rowBg, true)
			blocks = append(blocks, block)
			carry = append(carry, continuation(gc, rest, end, rowBg))
		}

		rowY += rowH
	}
	return blocks, rowY, carry
}

// continuation returns the cell that continues gc after row end, on the next
// page: same columns, style and the rows it still spans, holding the content
// that did not fit (possibly none, so the column stays framed). It keeps the
// background of its originating row, and its content continues from the top.
func continuation(gc *gridCell, rest []Element, end int, rowBg *Color) carriedCell {
	cell := gc.cell
	cell.Content = rest
	cell.RowSpan = gc.row + gc.rowSpan - end
	cell.ColSpan = gc.colSpan
	if cell.Style.Background == nil {
		cell.Style.Background = rowBg
	}
	cell.Style.VerticalAlign = VAlignTop
	return carriedCell{cell: cell, col: gc.col, colSpan: gc.colSpan}
}

// appendRowBackground appends the background of row r. The background skips
// columns covered by cells from earlier rows, so it never paints over them;
// a row without covered columns gets a single full-width rectangle.
func (g *sectionGrid) appendRowBackground(blocks []Block, r int, bg Color, colOffsets []float64, tableWidth, rowY float64) []Block {
	rowH := g.heights[r]
	covered := g.covered[r]
	if covered == nil {
		return append(blocks, rectBlock(0, rowY, tableWidth, rowH, bg))
	}

	nCols := len(colOffsets)
	for c := 0; c < nCols; {
		if covered[c] {
			c++
			continue
		}
		start := c
		for c < nCols && !covered[c] {
			c++
		}
		x := colOffsets[start]
		right := tableWidth
		if c < nCols {
			right = colOffsets[c]
		}
		blocks = append(blocks, rectBlock(x, rowY, right-x, rowH, bg))
	}
	return blocks
}

// rectBlock returns a block that fills its own area with color.
func rectBlock(x, y, w, h float64, color Color) Block {
	return Block{
		X:      x,
		Y:      y,
		Width:  w,
		Height: h,
		Draw: func(r Renderer) {
			r.DrawRect(0, 0, w, h, &color, nil, 0)
		},
	}
}

// buildCellBlock creates the Block for a single cell of height cellH,
// including background, border, and content layout. rowBg, when non-nil, is
// the background of the cell's originating row; it fills the cell when the
// cell has no background of its own.
func buildCellBlock(gc *gridCell, colOffsets []float64, cellH, rowY float64, rowBg *Color) Block {
	block, _ := layoutCell(gc, colOffsets, cellH, rowY, rowBg, false)
	return block
}

// layoutCell lays out a cell box of height cellH. A whole cell lays out each
// content element in the full inner area, as it always has. A fragment (the
// part of a row-spanning cell before a page break) stacks its elements in
// the height that remains and returns the content that did not fit: the
// overflow of the element that was split plus every element after it.
// Vertical alignment applies only when the box holds the cell's entire
// content; a fragment whose content continues is top-aligned (ADR-007 §4).
func layoutCell(gc *gridCell, colOffsets []float64, cellH, rowY float64, rowBg *Color, fragment bool) (Block, []Element) {
	s := gc.cell.Style.effective()
	fontSize := s.FontSize

	cellX := colOffsets[gc.col]
	cellW := gc.width

	pad := s.Padding.Resolve(cellW, cellH, fontSize)
	innerW := cellW - pad.Left - pad.Right
	innerH := cellH - pad.Top - pad.Bottom
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}

	// Layout cell content to get actual content height (for vertical alignment).
	var contentBlocks []Block
	var rest []Element
	innerCursorY := 0.0
	for i, elem := range gc.cell.Content {
		avail := innerH
		if fragment {
			avail = innerH - innerCursorY
		}
		plan := elem.PlanLayout(Area{Width: innerW, Height: avail})
		if fragment && plan.Status == Nothing {
			rest = append(rest, gc.cell.Content[i:]...)
			break
		}
		placed := cloneBlocks(plan.Blocks)
		offsetBlocks(placed, 0, innerCursorY)
		contentBlocks = append(contentBlocks, placed...)
		innerCursorY += plan.Consumed
		if fragment && plan.Status == Partial && plan.Overflow != nil {
			rest = append(rest, plan.Overflow)
			rest = append(rest, gc.cell.Content[i+1:]...)
			break
		}
	}
	contentH := innerCursorY

	// Vertical alignment offset.
	var vOffset float64
	align := s.VerticalAlign
	if len(rest) > 0 {
		align = VAlignTop
	}
	switch align {
	case VAlignMiddle:
		vOffset = (innerH - contentH) / 2
	case VAlignBottom:
		vOffset = innerH - contentH
	default: // VAlignTop
		vOffset = 0
	}
	if vOffset < 0 {
		vOffset = 0
	}

	// Offset content blocks by padding + vertical alignment.
	offsetBlocks(contentBlocks, pad.Left, pad.Top+vOffset)

	// Capture values for the Draw closure.
	// Draw at (0,0) relative to block origin — renderer adds block.X/Y.
	capW := cellW
	capH := cellH
	capBg := s.Background
	if capBg == nil {
		capBg = rowBg
	}
	capBorder := s.Border

	outerBlock := Block{
		X:        cellX,
		Y:        rowY,
		Width:    cellW,
		Height:   cellH,
		Tag:      "TD",
		Children: contentBlocks,
		Draw: func(r Renderer) {
			if capBg != nil {
				bg := *capBg
				r.DrawRect(0, 0, capW, capH, &bg, nil, 0)
			}
			if capBorder.Top.Width > 0 {
				r.DrawLine(0, 0, capW, 0, capBorder.Top.Color, capBorder.Top.Width)
			}
			if capBorder.Right.Width > 0 {
				r.DrawLine(capW, 0, capW, capH, capBorder.Right.Color, capBorder.Right.Width)
			}
			if capBorder.Bottom.Width > 0 {
				r.DrawLine(0, capH, capW, capH, capBorder.Bottom.Color, capBorder.Bottom.Width)
			}
			if capBorder.Left.Width > 0 {
				r.DrawLine(0, 0, 0, capH, capBorder.Left.Color, capBorder.Left.Width)
			}
		},
	}

	return outerBlock, rest
}

// --- Continuation ---

// columnWidths returns the column grid for an area of the given width. A
// continuation reuses the grid of the table it continues, so columns stay
// identical across pages; it only re-resolves (keeping the column count) if
// the area width changed.
func (t *Table) columnWidths(width float64) []float64 {
	if t.cont != nil {
		if t.cont.width == width {
			return t.cont.colWidths
		}
		return t.resolveColumnWidths(len(t.cont.colWidths), width)
	}
	numCols := t.numCols()
	if numCols == 0 {
		return nil
	}
	return t.resolveColumnWidths(numCols, width)
}

// carry returns the row-spanning cells continued from the previous page.
func (t *Table) carry() []carriedCell {
	if t.cont == nil {
		return nil
	}
	return t.cont.carry
}

// overflowTable returns the Table that continues this one on the next page:
// the body rows from bodyStart, the same Header, Footer, Columns, Style and
// Fonts, the column grid, and the carried row-spanning cells. It returns nil
// when no body rows remain.
func (t *Table) overflowTable(bodyStart int, carry []carriedCell, colWidths []float64, width float64) Element {
	if bodyStart >= len(t.Body) {
		return nil
	}
	return &Table{
		Columns: t.Columns,
		Header:  t.Header,
		Body:    t.Body[bodyStart:],
		Footer:  t.Footer,
		Style:   t.Style,
		Fonts:   t.Fonts,
		cont: &tableContinuation{
			width:     width,
			colWidths: colWidths,
			carry:     carry,
		},
	}
}
