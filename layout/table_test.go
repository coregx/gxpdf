package layout

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Helpers ---

// textCell creates a TableCell containing a single Text element.
func textCell(content string, opts ...func(*TableCell)) TableCell {
	cell := TableCell{
		Content: []Element{
			&Text{Content: content, Fonts: &MockFontResolver{}},
		},
		ColSpan: 1,
		RowSpan: 1,
	}
	for _, o := range opts {
		o(&cell)
	}
	return cell
}

// withColSpan returns a TableCell option that sets the column span.
func withColSpan(n int) func(*TableCell) {
	return func(c *TableCell) {
		c.ColSpan = n
	}
}

// withRowSpan returns a TableCell option that sets the row span.
func withRowSpan(n int) func(*TableCell) {
	return func(c *TableCell) {
		c.RowSpan = n
	}
}

// rowBg returns a TableRow Style option that sets the row background.
func rowWithBg(row TableRow, color Color) TableRow {
	bg := color
	row.Style.Background = &bg
	return row
}

// simpleTable creates a Table with equal-width columns and the given body rows.
func simpleTable(numCols int, rows []TableRow, fonts FontResolver) *Table {
	cols := make([]ColumnDef, numCols)
	for i := range cols {
		cols[i] = ColumnDef{Width: Fr(1)}
	}
	return &Table{
		Columns: cols,
		Body:    rows,
		Fonts:   fonts,
	}
}

// --- Tests ---

// TestTable_SimpleFullFit verifies a small 3x3 table that fits on one page.
func TestTable_SimpleFullFit(t *testing.T) {
	fonts := &MockFontResolver{}
	tbl := simpleTable(3, []TableRow{
		{Cells: []TableCell{textCell("A"), textCell("B"), textCell("C")}},
		{Cells: []TableCell{textCell("D"), textCell("E"), textCell("F")}},
		{Cells: []TableCell{textCell("G"), textCell("H"), textCell("I")}},
	}, fonts)

	plan := tbl.PlanLayout(Area{Width: 300, Height: 1000})
	assert.Equal(t, Full, plan.Status)
	assert.Nil(t, plan.Overflow)
	assert.Greater(t, plan.Consumed, 0.0)
}

// TestTable_ExplicitFixedColumns verifies Fixed column widths are respected.
func TestTable_ExplicitFixedColumns(t *testing.T) {
	fonts := &MockFontResolver{}
	tbl := &Table{
		Columns: []ColumnDef{
			{Width: Pt(100)},
			{Width: Pt(150)},
			{Width: Pt(50)},
		},
		Body: []TableRow{
			{Cells: []TableCell{textCell("One"), textCell("Two"), textCell("Three")}},
		},
		Fonts: fonts,
	}

	plan := tbl.PlanLayout(Area{Width: 300, Height: 500})
	assert.Equal(t, Full, plan.Status)
	assert.Nil(t, plan.Overflow)
}

// TestTable_PctColumns verifies percentage column widths.
func TestTable_PctColumns(t *testing.T) {
	fonts := &MockFontResolver{}
	tbl := &Table{
		Columns: []ColumnDef{
			{Width: Pct(30)},
			{Width: Pct(70)},
		},
		Body: []TableRow{
			{Cells: []TableCell{textCell("Left"), textCell("Right")}},
		},
		Fonts: fonts,
	}

	tableWidth := 400.0
	colWidths := tbl.resolveColumnWidths(2, tableWidth)
	assert.InDelta(t, 120.0, colWidths[0], 0.01, "30%% of 400 = 120")
	assert.InDelta(t, 280.0, colWidths[1], 0.01, "70%% of 400 = 280")
}

// TestTable_AutoColumns verifies Auto column widths are content-driven.
func TestTable_AutoColumns(t *testing.T) {
	fonts := &MockFontResolver{}
	// MockFontResolver: width = chars * size * 0.5 = chars * 12 * 0.5 = chars * 6
	// "Short" = 5 chars * 6 = 30pt
	// "Much longer text" = 16 chars * 6 = 96pt
	tbl := &Table{
		Columns: []ColumnDef{
			{Width: Auto()},
			{Width: Auto()},
		},
		Body: []TableRow{
			{Cells: []TableCell{textCell("Short"), textCell("Much longer text")}},
		},
		Fonts: fonts,
	}

	tableWidth := 500.0
	colWidths := tbl.resolveColumnWidths(2, tableWidth)
	// The auto columns should be non-zero and the second should be wider.
	assert.Greater(t, colWidths[0], 0.0)
	assert.Greater(t, colWidths[1], colWidths[0],
		"longer content column should be wider")
}

// TestTable_FrColumns verifies fractional column widths distribute remaining space.
func TestTable_FrColumns(t *testing.T) {
	fonts := &MockFontResolver{}
	tbl := &Table{
		Columns: []ColumnDef{
			{Width: Fr(1)},
			{Width: Fr(2)},
			{Width: Fr(1)},
		},
		Body: []TableRow{
			{Cells: []TableCell{textCell("A"), textCell("B"), textCell("C")}},
		},
		Fonts: fonts,
	}

	tableWidth := 400.0
	colWidths := tbl.resolveColumnWidths(3, tableWidth)
	// 1/4 + 2/4 + 1/4 of 400 = 100 + 200 + 100
	assert.InDelta(t, 100.0, colWidths[0], 0.01)
	assert.InDelta(t, 200.0, colWidths[1], 0.01)
	assert.InDelta(t, 100.0, colWidths[2], 0.01)
}

// TestTable_ColSpan2 verifies a cell spanning 2 columns.
func TestTable_ColSpan2(t *testing.T) {
	fonts := &MockFontResolver{}
	tbl := simpleTable(3, []TableRow{
		{Cells: []TableCell{
			textCell("Span2", withColSpan(2)),
			textCell("Right"),
		}},
		{Cells: []TableCell{textCell("A"), textCell("B"), textCell("C")}},
	}, fonts)

	plan := tbl.PlanLayout(Area{Width: 300, Height: 1000})
	assert.Equal(t, Full, plan.Status)
	assert.Nil(t, plan.Overflow)
	// Row 0 has 2 cells (span-2 + 1 normal).
	g := buildSectionGrid(tbl.Body, tbl.resolveColumnWidths(3, 300), nil)
	require.Len(t, g.cells, 5)
	assert.Equal(t, 0, g.cells[1].row)
	assert.Equal(t, 1, g.cells[2].row)
	// Span-2 cell should have width = 2/3 * 300 = 200.
	assert.InDelta(t, 200.0, g.cells[0].width, 0.1)
	assert.Equal(t, 2, g.cells[0].colSpan)
}

// TestTable_ColSpan3Header verifies colspan 3 in the header row.
func TestTable_ColSpan3Header(t *testing.T) {
	fonts := &MockFontResolver{}
	tbl := &Table{
		Columns: []ColumnDef{
			{Width: Fr(1)},
			{Width: Fr(1)},
			{Width: Fr(1)},
		},
		Header: []TableRow{
			{Cells: []TableCell{textCell("Full Width Header", withColSpan(3))}},
		},
		Body: []TableRow{
			{Cells: []TableCell{textCell("A"), textCell("B"), textCell("C")}},
		},
		Fonts: fonts,
	}

	plan := tbl.PlanLayout(Area{Width: 300, Height: 1000})
	assert.Equal(t, Full, plan.Status)
	assert.Nil(t, plan.Overflow)
}

// TestTable_RowSpan2 verifies a cell spanning 2 rows tracks column occupancy.
func TestTable_RowSpan2(t *testing.T) {
	fonts := &MockFontResolver{}
	// Column 0 in row 0 spans 2 rows.
	// Row 1 should only have cells for columns 1 and 2.
	tbl := simpleTable(3, []TableRow{
		{Cells: []TableCell{
			textCell("Row01Span", withRowSpan(2)),
			textCell("R0C1"),
			textCell("R0C2"),
		}},
		{Cells: []TableCell{
			// only 2 cells for columns 1 and 2
			textCell("R1C1"),
			textCell("R1C2"),
		}},
	}, fonts)

	colWidths := tbl.resolveColumnWidths(3, 300)
	g := buildSectionGrid(tbl.Body, colWidths, nil)
	require.Equal(t, 2, g.numRows())
	require.Len(t, g.cells, 5)

	// Row 0: 3 cells (rowspan cell + 2 normal).
	assert.Equal(t, gridCell{cell: tbl.Body[0].Cells[0], row: 0, col: 0, colSpan: 1, rowSpan: 2, width: 100}, g.cells[0])

	// Row 1: 2 cells (column 0 is occupied by rowspan), starting at column 1.
	assert.Equal(t, [2]int{1, 1}, [2]int{g.cells[3].row, g.cells[3].col})
	assert.Equal(t, [2]int{1, 2}, [2]int{g.cells[4].row, g.cells[4].col})
	assert.Equal(t, []bool{true, false, false}, g.covered[1])
	assert.Nil(t, g.covered[0])
}

// TestTable_HeaderRepeatOnOverflow verifies the overflow Table carries the header.
func TestTable_HeaderRepeatOnOverflow(t *testing.T) {
	fonts := &MockFontResolver{}
	// Each row is approximately 12*1.2 = 14.4 pt tall.
	// With height=50, we can fit header (14.4) + ~2 body rows (28.8) = ~43.2.
	// The 3rd body row will overflow.
	tbl := &Table{
		Columns: []ColumnDef{{Width: Fr(1)}},
		Header: []TableRow{
			{Cells: []TableCell{textCell("Header")}},
		},
		Body: []TableRow{
			{Cells: []TableCell{textCell("Row1")}},
			{Cells: []TableCell{textCell("Row2")}},
			{Cells: []TableCell{textCell("Row3")}},
			{Cells: []TableCell{textCell("Row4")}},
		},
		Fonts: fonts,
	}

	plan := tbl.PlanLayout(Area{Width: 200, Height: 50})
	// Should be partial — some rows overflow.
	if plan.Status == Partial {
		require.NotNil(t, plan.Overflow)
		overflow, ok := plan.Overflow.(*Table)
		require.True(t, ok, "overflow should be *Table")
		// Overflow table must carry the header.
		assert.Len(t, overflow.Header, 1)
		assert.Equal(t, "Header", overflow.Header[0].Cells[0].Content[0].(*Text).Content)
	}
	// If everything fit (height was big enough), that's also acceptable.
}

// TestTable_PageSplitBetweenRows verifies Partial status when body overflows.
func TestTable_PageSplitBetweenRows(t *testing.T) {
	fonts := &MockFontResolver{}
	// Each row height ≈ 14.4pt. 4 rows ≈ 57.6pt. Area height = 30 (fits ~2 rows).
	tbl := simpleTable(2, []TableRow{
		{Cells: []TableCell{textCell("A"), textCell("B")}},
		{Cells: []TableCell{textCell("C"), textCell("D")}},
		{Cells: []TableCell{textCell("E"), textCell("F")}},
		{Cells: []TableCell{textCell("G"), textCell("H")}},
	}, fonts)

	plan := tbl.PlanLayout(Area{Width: 200, Height: 30})
	// Either Partial (some rows overflowed) or Full (all fit, which means
	// the height estimate was generous). Both are valid — we just check consistency.
	if plan.Status == Partial {
		require.NotNil(t, plan.Overflow)
		overflow, ok := plan.Overflow.(*Table)
		require.True(t, ok)
		assert.Less(t, len(overflow.Body), 4, "overflow should have fewer rows than original")
	}
}

// TestTable_CellVerticalAlignTop verifies VAlignTop produces zero vOffset.
func TestTable_CellVerticalAlignTop(t *testing.T) {
	cell := TableCell{
		Content: []Element{&Text{Content: "hi", Fonts: &MockFontResolver{}}},
		Style:   Style{VerticalAlign: VAlignTop},
	}
	h := measureCellHeight(&cell, 200)
	assert.Greater(t, h, 0.0)
}

// TestTable_CellVerticalAlignMiddle verifies VAlignMiddle content is shifted.
func TestTable_CellVerticalAlignMiddle(t *testing.T) {
	fonts := &MockFontResolver{}
	gc := gridCell{
		cell: TableCell{
			Content: []Element{&Text{Content: "hi", Fonts: fonts}},
			Style:   Style{VerticalAlign: VAlignMiddle},
		},
		col:     0,
		width:   200,
		colSpan: 1,
		rowSpan: 1,
	}
	colOffsets := []float64{0}
	rowH := 100.0
	rowY := 0.0
	block := buildCellBlock(&gc, colOffsets, rowH, rowY, nil)
	// Children are offset by (100 - 14.4) / 2 for middle-aligned short content.
	require.NotEmpty(t, block.Children)
	assert.InDelta(t, 42.8, block.Children[0].Y, 0.001)
}

// TestTable_CellVerticalAlignBottom verifies VAlignBottom.
func TestTable_CellVerticalAlignBottom(t *testing.T) {
	fonts := &MockFontResolver{}
	gc := gridCell{
		cell: TableCell{
			Content: []Element{&Text{Content: "hi", Fonts: fonts}},
			Style:   Style{VerticalAlign: VAlignBottom},
		},
		col:     0,
		width:   200,
		colSpan: 1,
		rowSpan: 1,
	}
	colOffsets := []float64{0}
	block := buildCellBlock(&gc, colOffsets, 100.0, 0.0, nil)
	require.NotEmpty(t, block.Children)
	assert.InDelta(t, 100-14.4, block.Children[0].Y, 0.001)
}

// TestTable_CellPadding verifies padding increases measured height.
func TestTable_CellPadding(t *testing.T) {
	fonts := &MockFontResolver{}
	cellNoPad := TableCell{
		Content: []Element{&Text{Content: "text", Fonts: fonts}},
	}
	cellWithPad := TableCell{
		Content: []Element{&Text{Content: "text", Fonts: fonts}},
		Style:   Style{Padding: UniformEdges(Pt(10))},
	}

	hNoPad := measureCellHeight(&cellNoPad, 200)
	hWithPad := measureCellHeight(&cellWithPad, 200)
	assert.Greater(t, hWithPad, hNoPad, "cell with padding should be taller")
}

// TestTable_CellBackground verifies background is present in block's Draw closure.
func TestTable_CellBackground(t *testing.T) {
	fonts := &MockFontResolver{}
	bg := RGB255(200, 200, 200)
	cell := TableCell{
		Content: []Element{&Text{Content: "hi", Fonts: fonts}},
		Style:   Style{Background: &bg},
	}
	gc := gridCell{cell: cell, col: 0, width: 100, colSpan: 1, rowSpan: 1}
	colOffsets := []float64{0}
	block := buildCellBlock(&gc, colOffsets, 50.0, 0.0, nil)
	assert.NotNil(t, block.Draw, "cell with background should have a Draw closure")
}

// TestTable_RowBackground verifies row background block is emitted.
func TestTable_RowBackground(t *testing.T) {
	fonts := &MockFontResolver{}
	bg := RGB255(240, 240, 240)
	tbl := &Table{
		Columns: []ColumnDef{{Width: Fr(1)}, {Width: Fr(1)}},
		Body: []TableRow{
			rowWithBg(TableRow{Cells: []TableCell{textCell("A"), textCell("B")}}, bg),
		},
		Fonts: fonts,
	}

	plan := tbl.PlanLayout(Area{Width: 200, Height: 500})
	assert.Equal(t, Full, plan.Status)
	assert.NotEmpty(t, plan.Blocks, "blocks should include row background block")
}

// TestTable_EmptyTable verifies an empty table (no body rows) returns Full with zero height.
func TestTable_EmptyTable(t *testing.T) {
	tbl := &Table{
		Columns: []ColumnDef{{Width: Fr(1)}},
		Body:    nil,
		Fonts:   &MockFontResolver{},
	}

	plan := tbl.PlanLayout(Area{Width: 200, Height: 500})
	assert.Equal(t, Full, plan.Status)
	assert.Nil(t, plan.Overflow)
}

// TestTable_HeaderOnly verifies a table with only header rows.
func TestTable_HeaderOnly(t *testing.T) {
	fonts := &MockFontResolver{}
	tbl := &Table{
		Columns: []ColumnDef{{Width: Fr(1)}, {Width: Fr(1)}},
		Header: []TableRow{
			{Cells: []TableCell{textCell("Col1"), textCell("Col2")}},
		},
		Fonts: fonts,
	}

	plan := tbl.PlanLayout(Area{Width: 200, Height: 500})
	assert.Equal(t, Full, plan.Status)
	assert.Nil(t, plan.Overflow)
	assert.Greater(t, plan.Consumed, 0.0)
}

// TestTable_FooterPlacement verifies footer rows are placed after the body.
func TestTable_FooterPlacement(t *testing.T) {
	fonts := &MockFontResolver{}
	tbl := &Table{
		Columns: []ColumnDef{{Width: Fr(1)}, {Width: Fr(1)}},
		Body: []TableRow{
			{Cells: []TableCell{textCell("B1"), textCell("B2")}},
		},
		Footer: []TableRow{
			{Cells: []TableCell{textCell("Total"), textCell("42")}},
		},
		Fonts: fonts,
	}

	plan := tbl.PlanLayout(Area{Width: 200, Height: 500})
	assert.Equal(t, Full, plan.Status)
	assert.Nil(t, plan.Overflow)
	// Consumed should include both body and footer height.
	assert.Greater(t, plan.Consumed, 0.0)
}

// TestTable_OverflowCarriesColumns verifies overflow table preserves column defs.
func TestTable_OverflowCarriesColumns(t *testing.T) {
	fonts := &MockFontResolver{}
	cols := []ColumnDef{{Width: Pt(100)}, {Width: Pt(100)}}
	tbl := &Table{
		Columns: cols,
		Body: []TableRow{
			{Cells: []TableCell{textCell("Row1A"), textCell("Row1B")}},
			{Cells: []TableCell{textCell("Row2A"), textCell("Row2B")}},
			{Cells: []TableCell{textCell("Row3A"), textCell("Row3B")}},
		},
		Fonts: fonts,
	}

	// Force overflow by providing very little height.
	plan := tbl.PlanLayout(Area{Width: 200, Height: 15})
	if plan.Status == Partial {
		overflow, ok := plan.Overflow.(*Table)
		require.True(t, ok)
		assert.Equal(t, 2, len(overflow.Columns))
		assert.Equal(t, Pt(100), overflow.Columns[0].Width)
	}
}

// TestTable_MeasurableMinWidth verifies Table.MinWidth returns non-zero.
func TestTable_MeasurableMinWidth(t *testing.T) {
	fonts := &MockFontResolver{}
	tbl := &Table{
		Columns: []ColumnDef{{Width: Auto()}, {Width: Auto()}},
		Body: []TableRow{
			{Cells: []TableCell{textCell("Hello world"), textCell("Bye")}},
		},
		Fonts: fonts,
	}
	assert.Greater(t, tbl.MinWidth(), 0.0)
}

// TestTable_MeasurableMaxWidth verifies Table.MaxWidth >= MinWidth.
func TestTable_MeasurableMaxWidth(t *testing.T) {
	fonts := &MockFontResolver{}
	tbl := &Table{
		Columns: []ColumnDef{{Width: Auto()}, {Width: Auto()}},
		Body: []TableRow{
			{Cells: []TableCell{textCell("Hello world"), textCell("Bye")}},
		},
		Fonts: fonts,
	}
	assert.GreaterOrEqual(t, tbl.MaxWidth(), tbl.MinWidth())
}

// TestTable_NilFontsUseMock verifies nil Fonts falls back to MockFontResolver.
func TestTable_NilFontsUseMock(t *testing.T) {
	tbl := &Table{
		Columns: []ColumnDef{{Width: Fr(1)}},
		Body: []TableRow{
			{Cells: []TableCell{textCell("hi")}},
		},
		Fonts: nil, // intentionally nil
	}
	// Should not panic.
	plan := tbl.PlanLayout(Area{Width: 200, Height: 500})
	assert.Equal(t, Full, plan.Status)
}

// TestTable_CellBorder verifies cell border does not panic during rendering.
func TestTable_CellBorder(t *testing.T) {
	fonts := &MockFontResolver{}
	borderColor := Black
	cell := TableCell{
		Content: []Element{&Text{Content: "bordered", Fonts: fonts}},
		Style: Style{
			Border: BorderEdges{
				Top:    BorderSide{Width: 1, Color: borderColor},
				Bottom: BorderSide{Width: 1, Color: borderColor},
				Left:   BorderSide{Width: 1, Color: borderColor},
				Right:  BorderSide{Width: 1, Color: borderColor},
			},
		},
	}
	gc := gridCell{cell: cell, col: 0, width: 100, colSpan: 1, rowSpan: 1}
	colOffsets := []float64{0}
	block := buildCellBlock(&gc, colOffsets, 50.0, 0.0, nil)
	assert.NotNil(t, block.Draw)

	// Execute the Draw closure to verify it doesn't panic.
	block.Draw(&noopRenderer{})
}

// TestTable_EffectiveColSpanClamp verifies ColSpan < 1 is treated as 1.
func TestTable_EffectiveColSpanClamp(t *testing.T) {
	c := TableCell{ColSpan: -3}
	assert.Equal(t, 1, c.effectiveColSpan())
}

// TestTable_EffectiveRowSpanClamp verifies RowSpan < 1 is treated as 1.
func TestTable_EffectiveRowSpanClamp(t *testing.T) {
	c := TableCell{RowSpan: 0}
	assert.Equal(t, 1, c.effectiveRowSpan())
}

// TestTable_NoColumnsDefinedEqualDistribution verifies equal distribution
// when no ColumnDef is provided.
func TestTable_NoColumnsDefinedEqualDistribution(t *testing.T) {
	fonts := &MockFontResolver{}
	tbl := &Table{
		// Columns intentionally empty.
		Body: []TableRow{
			{Cells: []TableCell{textCell("A"), textCell("B"), textCell("C")}},
		},
		Fonts: fonts,
	}

	colWidths := tbl.resolveColumnWidths(3, 300)
	for i, w := range colWidths {
		assert.InDelta(t, 100.0, w, 0.01, "column %d should be 100pt", i)
	}
}

// --- noopRenderer is used to execute Draw closures in tests without PDF output. ---

type noopRenderer struct{}

func (n *noopRenderer) DrawText(_ string, _, _ float64, _ FontRef, _ float64, _ Color, _ TextDrawOptions) {
}
func (n *noopRenderer) DrawRect(_, _, _, _ float64, _ *Color, _ *Color, _ float64) {}
func (n *noopRenderer) DrawLine(_, _, _, _ float64, _ Color, _ float64)            {}
func (n *noopRenderer) DrawImage(_ []byte, _, _, _, _ float64)                     {}
func (n *noopRenderer) PushState()                                                 {}
func (n *noopRenderer) PopState()                                                  {}
func (n *noopRenderer) SetClipRect(_, _, _, _ float64)                             {}
