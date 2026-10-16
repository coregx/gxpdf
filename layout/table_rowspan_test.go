package layout

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixedElement is an Element of a fixed height, for exact row-height tests.
type fixedElement struct{ h float64 }

func (e *fixedElement) PlanLayout(_ Area) Plan {
	return Plan{Status: Full, Consumed: e.h, Blocks: []Block{{Height: e.h}}}
}

// fixedCell returns a cell whose content is exactly h points tall.
func fixedCell(h float64, rowSpan int) TableCell {
	return TableCell{Content: []Element{&fixedElement{h: h}}, RowSpan: rowSpan}
}

// tdBlocks returns the cell blocks of a plan in emission order.
func tdBlocks(p Plan) []Block {
	var out []Block
	for _, b := range p.Blocks {
		if b.Tag == "TD" {
			out = append(out, b)
		}
	}
	return out
}

// issue100Table is the table from GitHub issue #100: TEAM and a long text
// span all member rows, vertically centered.
func issue100Table(members int) *Table {
	fonts := &MockFontResolver{}
	long := "Line one of a long merged text that wraps over several lines inside the merged cell"
	mid := func(s string, span int) TableCell {
		c := textCell(s, withRowSpan(span))
		c.Style.VerticalAlign = VAlignMiddle
		return c
	}
	var body []TableRow
	for i := 1; i <= members; i++ {
		var cells []TableCell
		if i == 1 {
			cells = append(cells, mid("TEAM", members))
		}
		cells = append(cells, textCell(fmt.Sprintf("member %d", i)))
		if i == 1 {
			cells = append(cells, mid(long, members))
		}
		body = append(body, TableRow{Cells: cells})
	}
	return simpleTable(3, body, fonts)
}

// TestTable_RowSpan_Issue100 is the #100 repro: a spanning cell covers all
// spanned rows, shows all its text and is centered within the span.
func TestTable_RowSpan_Issue100(t *testing.T) {
	tbl := issue100Table(4)
	plan := tbl.PlanLayout(Area{Width: 300, Height: 1000})
	require.Equal(t, Full, plan.Status)

	tds := tdBlocks(plan)
	require.Len(t, tds, 6) // TEAM, member 1, long, members 2..4
	team, long := tds[0], tds[2]

	// The long text wraps to 6 mock lines of 14.4pt: the rows must grow to fit it.
	require.Len(t, long.Children, 6, "every line of the merged text is laid out")
	assert.InDelta(t, 6*14.4, plan.Consumed, 1e-9)

	// Both spanning cells are as tall as the whole table body.
	assert.InDelta(t, plan.Consumed, team.Height, 1e-9)
	assert.InDelta(t, plan.Consumed, long.Height, 1e-9)

	// The member rows grew in proportion to their heights (all equal here).
	for i, td := range tds[3:] {
		assert.InDelta(t, plan.Consumed/4, td.Height, 1e-9, "member row %d", i+2)
	}

	// TEAM is centered within the 4-row span, not within the first row.
	require.Len(t, team.Children, 1)
	assert.InDelta(t, (plan.Consumed-14.4)/2, team.Children[0].Y, 1e-9)
}

// TestGrowRows checks the height distribution rule (ADR-007 §2).
func TestGrowRows(t *testing.T) {
	tests := []struct {
		name    string
		rows    []float64
		deficit float64
		want    []float64
	}{
		{"proportional", []float64{10, 20, 30}, 60, []float64{20, 40, 60}},
		// WPT css-tables/tentative/rowspan-height-redistribution.html:
		// rows 45/15 under a 100px span end at 75/25 (equal split: 65/35).
		{"wpt_45_15", []float64{45, 15}, 40, []float64{75, 25}},
		{"empty_rows_do_not_grow", []float64{0, 10, 0}, 10, []float64{0, 20, 0}},
		{"all_empty_last_row_takes_all", []float64{0, 0, 0}, 30, []float64{0, 0, 30}},
		{"no_deficit", []float64{10, 10}, 0, []float64{10, 10}},
		{"negative_deficit", []float64{10, 10}, -5, []float64{10, 10}},
		{"single_row", []float64{7}, 3, []float64{10}},
		{"empty", nil, 5, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := append([]float64(nil), tt.rows...)
			before := sumFloats(rows)
			growRows(rows, tt.deficit)
			require.Len(t, rows, len(tt.want))
			for i := range rows {
				assert.InDelta(t, tt.want[i], rows[i], 1e-9, "row %d", i)
			}
			if tt.deficit > 0 && len(rows) > 0 {
				assert.InDelta(t, before+tt.deficit, sumFloats(rows), 1e-9, "rows sum to the target")
			}
		})
	}
}

// TestGrowRows_RemainderGoesToLastRow verifies the shares do not drift for
// awkward ratios: the error stays at floating-point rounding level, far below
// the 0.01pt fit tolerance of text layout.
func TestGrowRows_RemainderGoesToLastRow(t *testing.T) {
	rows := []float64{14.4, 14.4, 14.4, 14.4, 14.4, 14.4, 14.4}
	target := 100.0 / 3 * 7
	growRows(rows, target-sumFloats(rows))
	assert.InDelta(t, target, sumFloats(rows), 1e-12)
}

// TestTable_RowSpan_NestedShortestFirst pins the processing order: shorter
// spans settle first (ADR-007 §2 example).
func TestTable_RowSpan_NestedShortestFirst(t *testing.T) {
	g := buildSectionGrid([]TableRow{
		{Cells: []TableCell{fixedCell(90, 3), fixedCell(80, 2), fixedCell(10, 1)}},
		{Cells: []TableCell{fixedCell(10, 1)}},
		{Cells: []TableCell{fixedCell(10, 1), fixedCell(10, 1)}},
	}, []float64{100, 100, 100}, nil)

	assert.InDeltaSlice(t, []float64{40, 40, 10}, g.heights, 1e-9)
}

// TestTable_RowSpan_ClampedToSection verifies RowSpan never reaches past the
// end of its section, and a header span does not occupy body columns.
func TestTable_RowSpan_ClampedToSection(t *testing.T) {
	tbl := &Table{
		Columns: []ColumnDef{{Width: Fr(1)}, {Width: Fr(1)}},
		Header:  []TableRow{{Cells: []TableCell{textCell("H", withRowSpan(5)), textCell("H2")}}},
		Body: []TableRow{
			{Cells: []TableCell{textCell("A", withRowSpan(10)), textCell("B")}},
			{Cells: []TableCell{textCell("C")}},
		},
		Fonts: &MockFontResolver{},
	}
	widths := tbl.resolveColumnWidths(2, 200)

	header := buildSectionGrid(tbl.Header, widths, nil)
	assert.Equal(t, 1, header.cells[0].rowSpan)

	body := buildSectionGrid(tbl.Body, widths, nil)
	require.Len(t, body.cells, 3)
	assert.Equal(t, 2, body.cells[0].rowSpan)
	// Body row 0 starts at column 0: the header span did not leak into it.
	assert.Equal(t, 0, body.cells[0].col)
	assert.Equal(t, 1, body.cells[2].col) // "C" skips the column occupied by "A"

	plan := tbl.PlanLayout(Area{Width: 200, Height: 1000})
	require.Equal(t, Full, plan.Status)
	assert.InDelta(t, 3*14.4, plan.Consumed, 1e-9)
}

// TestTable_ColSpanStopsAtOccupiedColumn verifies a colspan never overlaps a
// column occupied by a row span from an earlier row.
func TestTable_ColSpanStopsAtOccupiedColumn(t *testing.T) {
	g := buildSectionGrid([]TableRow{
		{Cells: []TableCell{textCell("a"), textCell("span", withRowSpan(2)), textCell("c")}},
		{Cells: []TableCell{textCell("wide", withColSpan(3)), textCell("d")}},
	}, []float64{100, 100, 100}, nil)

	require.Len(t, g.cells, 5)
	wide, d := g.cells[3], g.cells[4]
	assert.Equal(t, [3]int{1, 0, 1}, [3]int{wide.row, wide.col, wide.colSpan})
	assert.InDelta(t, 100.0, wide.width, 1e-9)
	assert.Equal(t, [3]int{1, 2, 1}, [3]int{d.row, d.col, d.colSpan})
}

// recordingRenderer logs DrawRect calls with absolute coordinates.
type rectLog struct {
	baseX, baseY float64
	rects        *[]string
}

func (r *rectLog) DrawText(string, float64, float64, FontRef, float64, Color, TextDrawOptions) {}
func (r *rectLog) DrawRect(x, y, w, h float64, fill, _ *Color, _ float64) {
	*r.rects = append(*r.rects, fmt.Sprintf("%.1f,%.1f %.1fx%.1f %.2f", r.baseX+x, r.baseY+y, w, h, fill.R))
}
func (r *rectLog) DrawLine(float64, float64, float64, float64, Color, float64) {}
func (r *rectLog) DrawImage([]byte, float64, float64, float64, float64)        {}
func (r *rectLog) PushState()                                                  {}
func (r *rectLog) PopState()                                                   {}
func (r *rectLog) SetClipRect(float64, float64, float64, float64)              {}

// TestTable_RowSpan_RowBackgrounds verifies CSS 2.2 layering: a row's
// background covers the cells originating in it (including their spanned
// area) and never paints over a span from an earlier row.
func TestTable_RowSpan_RowBackgrounds(t *testing.T) {
	own := RGB(0.5, 0, 0)
	span := textCell("span", withRowSpan(2))
	spanOwnBg := textCell("own", withRowSpan(2))
	spanOwnBg.Style.Background = &own

	tbl := simpleTable(3, []TableRow{
		rowWithBg(TableRow{Cells: []TableCell{span, textCell("b"), spanOwnBg}}, RGB(0.1, 0, 0)),
		rowWithBg(TableRow{Cells: []TableCell{textCell("d")}}, RGB(0.2, 0, 0)),
	}, &MockFontResolver{})

	plan := tbl.PlanLayout(Area{Width: 300, Height: 1000})
	var rects []string
	for _, b := range plan.Blocks {
		if b.Draw != nil {
			b.Draw(&rectLog{baseX: b.X, baseY: b.Y, rects: &rects})
		}
	}

	assert.Equal(t, []string{
		"0.0,0.0 300.0x14.4 0.10",    // row 0 background, full width
		"0.0,0.0 100.0x28.8 0.10",    // span without own bg: row 0 bg over both rows
		"200.0,0.0 100.0x28.8 0.50",  // span with own bg keeps it
		"100.0,14.4 100.0x14.4 0.20", // row 1 background only over its own column
	}, rects)
}

// TestTable_RowSpan_PlanDoesNotMutate verifies PlanLayout leaves the table
// untouched (Element contract, ADR-007 invariant I5).
func TestTable_RowSpan_PlanDoesNotMutate(t *testing.T) {
	tbl := issue100Table(6)
	before := make([]TableRow, len(tbl.Body))
	for i, r := range tbl.Body {
		before[i] = TableRow{Cells: append([]TableCell(nil), r.Cells...), Style: r.Style}
	}

	for _, h := range []float64{1000, 60, 20} {
		tbl.PlanLayout(Area{Width: 300, Height: h})
	}
	assert.Equal(t, before, tbl.Body)
}
