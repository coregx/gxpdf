package layout

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// drawn is a drawing call captured with absolute coordinates.
type drawn struct {
	kind string // "text", "rect" or "line"
	x, y float64
	w, h float64 // rect size; line end point for lines
	text string
	fill *Color
}

// drawCapture is a Renderer that records drawing calls of one block.
type drawCapture struct {
	baseX, baseY float64
	out          *[]drawn
}

func (d *drawCapture) DrawText(s string, x, y float64, _ FontRef, _ float64, _ Color, _ TextDrawOptions) {
	*d.out = append(*d.out, drawn{kind: "text", x: d.baseX + x, y: d.baseY + y, text: s})
}
func (d *drawCapture) DrawRect(x, y, w, h float64, fill, _ *Color, _ float64) {
	*d.out = append(*d.out, drawn{kind: "rect", x: d.baseX + x, y: d.baseY + y, w: w, h: h, fill: fill})
}
func (d *drawCapture) DrawLine(x1, y1, x2, y2 float64, _ Color, _ float64) {
	*d.out = append(*d.out, drawn{kind: "line", x: d.baseX + x1, y: d.baseY + y1, w: d.baseX + x2, h: d.baseY + y2})
}
func (d *drawCapture) DrawImage([]byte, float64, float64, float64, float64) {}
func (d *drawCapture) PushState()                                           {}
func (d *drawCapture) PopState()                                            {}
func (d *drawCapture) SetClipRect(float64, float64, float64, float64)       {}

// capture draws blocks (recursively, children offset by their parent) and
// returns the drawing calls.
func capture(blocks []Block) []drawn {
	var out []drawn
	var walk func(bs []Block, ox, oy float64)
	walk = func(bs []Block, ox, oy float64) {
		for _, b := range bs {
			x, y := ox+b.X, oy+b.Y
			if b.Draw != nil {
				b.Draw(&drawCapture{baseX: x, baseY: y, out: &out})
			}
			walk(b.Children, x, y)
		}
	}
	walk(blocks, 0, 0)
	return out
}

// paginateTable lays the table out on pages of the given height until it is
// done and returns one plan per page. It fails on a Nothing plan or when the
// table does not finish within maxPages.
func paginateTable(t *testing.T, tbl *Table, width, pageH float64, maxPages int) []Plan {
	t.Helper()
	var plans []Plan
	var elem Element = tbl
	for page := 0; elem != nil; page++ {
		require.Less(t, page, maxPages, "table did not finish within %d pages", maxPages)
		p := elem.PlanLayout(Area{Width: width, Height: pageH, AtTop: true})
		require.NotEqual(t, Nothing, p.Status, "page %d: nothing placed", page+1)
		if p.Status == Full {
			require.Nil(t, p.Overflow, "page %d: Full plan with overflow", page+1)
		} else {
			require.NotNil(t, p.Overflow, "page %d: Partial plan without overflow", page+1)
		}
		plans = append(plans, p)
		elem = p.Overflow
	}
	return plans
}

// textsAt returns the texts drawn in the column starting at x, top to bottom.
func textsAt(ds []drawn, x float64) []string {
	var out []string
	for _, d := range ds {
		if d.kind == "text" && d.x >= x-0.001 && d.x < x+0.001 {
			out = append(out, d.text)
		}
	}
	return out
}

// cellBoxes returns the TD blocks of a plan as "x,y wxh" strings.
func cellBoxes(p Plan) []string {
	var out []string
	for _, b := range tdBlocks(p) {
		out = append(out, fmt.Sprintf("%.1f,%.1f %.1fx%.1f", b.X, b.Y, b.Width, b.Height))
	}
	return out
}

const longText = "Line one of a long merged text that wraps over several lines inside the merged cell"

// TestTable_RowSpanSplit_ColumnsPreserved is the D5 repro (#100 table, split
// after row 1): rows after the break stay in their columns and both spanning
// cells continue on the next page.
func TestTable_RowSpanSplit_ColumnsPreserved(t *testing.T) {
	first := issue100Table(4).PlanLayout(Area{Width: 300, Height: 30, AtTop: true})
	require.Equal(t, Partial, first.Status)
	second := first.Overflow.PlanLayout(Area{Width: 300, Height: 1000, AtTop: true})
	require.Equal(t, Full, second.Status)
	plans := []Plan{first, second}

	// Page 1: only row 1 (21.6pt) fits; both spanning cells are fragments of it.
	assert.Equal(t, []string{
		"0.0,0.0 100.0x21.6",
		"100.0,0.0 100.0x21.6",
		"200.0,0.0 100.0x21.6",
	}, cellBoxes(plans[0]))

	// Page 2: TEAM and the long text continue in columns 0 and 2 over the
	// three remaining rows; members stay in column 1.
	assert.Equal(t, []string{
		"0.0,0.0 100.0x72.0",
		"100.0,0.0 100.0x24.0",
		"200.0,0.0 100.0x72.0",
		"100.0,24.0 100.0x24.0",
		"100.0,48.0 100.0x24.0",
	}, cellBoxes(plans[1]))
	p2 := capture(plans[1].Blocks)
	assert.Equal(t, []string{"member 2", "member 3", "member 4"}, textsAt(p2, 100))
	assert.Empty(t, textsAt(p2, 0), "TEAM was fully shown on page 1 and is not repeated")
}

// TestTable_RowSpanSplit_ContentConserved splits a long spanning cell over
// several pages: every word appears exactly once, in order (ADR-007 I1), and
// the column grid stays the same on every page (I2).
func TestTable_RowSpanSplit_ContentConserved(t *testing.T) {
	text := strings.Repeat(longText+" ", 3)
	var body []TableRow
	const rows = 12
	for i := 0; i < rows; i++ {
		var cells []TableCell
		cells = append(cells, textCell(fmt.Sprintf("r%d", i)))
		if i == 0 {
			cells = append(cells, textCell(text, withRowSpan(rows)))
		}
		body = append(body, TableRow{Cells: cells})
	}
	tbl := simpleTable(2, body, &MockFontResolver{})

	plans := paginateTable(t, tbl, 200, 50, 20)
	require.Greater(t, len(plans), 2, "the span must break more than once")

	var words, rowLabels []string
	for i, p := range plans {
		ds := capture(p.Blocks)
		for _, s := range textsAt(ds, 100) {
			words = append(words, strings.Fields(s)...)
		}
		rowLabels = append(rowLabels, textsAt(ds, 0)...)
		for _, b := range tdBlocks(p) {
			assert.Contains(t, []float64{0, 100}, b.X, "page %d: cell outside the column grid", i+1)
			assert.InDelta(t, 100.0, b.Width, 1e-9, "page %d", i+1)
			assert.LessOrEqual(t, b.Y+b.Height, 50+FitTolerance, "page %d: cell below the page", i+1)
		}
	}
	assert.Equal(t, strings.Fields(text), words)
	for i := range rows {
		assert.Equal(t, fmt.Sprintf("r%d", i), rowLabels[i])
	}
}

// TestTable_RowSpanSplit_FragmentAlignment verifies the alignment rule of
// ADR-007 §4: a fragment whose content continues is top-aligned; a fragment
// holding the whole content keeps the cell's alignment.
func TestTable_RowSpanSplit_FragmentAlignment(t *testing.T) {
	plans := paginateTable(t, issue100Table(4), 300, 30, 5)
	p1 := capture(plans[0].Blocks)

	var team, long drawn
	for _, d := range p1 {
		switch {
		case d.kind == "text" && d.text == "TEAM":
			team = d
		case d.kind == "text" && d.x == 200:
			long = d
		}
	}
	// Text is drawn half a leading below its line top: (14.4-12)/2.
	const halfLeading = 1.2
	// TEAM (whole content in the 21.6pt fragment) is centered: (21.6-14.4)/2.
	assert.InDelta(t, 3.6+halfLeading, team.y, 1e-9)
	// The long text continues on page 2, so its fragment starts at the top.
	assert.InDelta(t, halfLeading, long.y, 1e-9)

	p2 := capture(plans[1].Blocks)
	cont := textsAt(p2, 200)
	require.NotEmpty(t, cont)
	for _, d := range p2 {
		if d.kind == "text" && d.text == cont[0] {
			assert.InDelta(t, halfLeading, d.y, 1e-9, "continuation starts at the top")
		}
	}
}

// TestTable_RowSpanSplit_EmptyContinuationFramed verifies a continuation with
// no content left still draws its border, so the column stays framed.
func TestTable_RowSpanSplit_EmptyContinuationFramed(t *testing.T) {
	tbl := issue100Table(4)
	border := BorderSide{Width: 1, Color: Black}
	tbl.Body[0].Cells[0].Style.Border = BorderEdges{Top: border, Right: border, Bottom: border, Left: border}

	plans := paginateTable(t, tbl, 300, 30, 5)
	var lines int
	for _, d := range capture(plans[1].Blocks) {
		if d.kind == "line" && d.x <= 100 && d.w <= 100 { // both ends inside the TEAM column
			lines++
		}
	}
	assert.Equal(t, 4, lines, "empty TEAM continuation draws all four borders")
}

// TestTable_RowSpanSplit_TwoSpansEndingApart crosses one break with two spans
// that end at different rows.
func TestTable_RowSpanSplit_TwoSpansEndingApart(t *testing.T) {
	c := func(s string) TableCell { return textCell(s) }
	tbl := simpleTable(3, []TableRow{
		{Cells: []TableCell{textCell("A", withRowSpan(3)), c("m0"), textCell("B", withRowSpan(5))}},
		{Cells: []TableCell{c("m1")}},
		{Cells: []TableCell{c("m2")}},
		{Cells: []TableCell{c("x3"), c("m3")}},
		{Cells: []TableCell{c("x4"), c("m4")}},
	}, &MockFontResolver{})

	plans := paginateTable(t, tbl, 300, 30, 5) // 2 rows of 14.4 per page
	require.Len(t, plans, 3)

	// Page 2 starts at row 2: A continues for 1 row, B for 3 rows.
	assert.Equal(t, []string{
		"0.0,0.0 100.0x14.4",    // A continuation
		"100.0,0.0 100.0x14.4",  // m2
		"200.0,0.0 100.0x28.8",  // B continuation, cut again after row 3
		"0.0,14.4 100.0x14.4",   // x3
		"100.0,14.4 100.0x14.4", // m3
	}, cellBoxes(plans[1]))

	// Page 3: B continues once more over row 4.
	assert.Equal(t, []string{
		"0.0,0.0 100.0x14.4",
		"100.0,0.0 100.0x14.4",
		"200.0,0.0 100.0x14.4",
	}, cellBoxes(plans[2]))
	p3 := capture(plans[2].Blocks)
	assert.Equal(t, []string{"x4"}, textsAt(p3, 0))
	assert.Equal(t, []string{"m4"}, textsAt(p3, 100))
}

// TestTable_RowSpanSplit_KeepsOriginRowBackground verifies a continuation
// keeps the background of the row it originates in (CSS 2.2 layers), and the
// first row on the next page does not paint over it.
func TestTable_RowSpanSplit_KeepsOriginRowBackground(t *testing.T) {
	origin := RGB(0.1, 0, 0)
	next := RGB(0.2, 0, 0)
	tbl := simpleTable(2, []TableRow{
		rowWithBg(TableRow{Cells: []TableCell{textCell("span", withRowSpan(3)), textCell("a")}}, origin),
		rowWithBg(TableRow{Cells: []TableCell{textCell("b")}}, next),
		rowWithBg(TableRow{Cells: []TableCell{textCell("c")}}, next),
	}, &MockFontResolver{})

	plans := paginateTable(t, tbl, 200, 30, 5)
	require.Len(t, plans, 2)

	var rects []string
	for _, d := range capture(plans[1].Blocks) {
		if d.kind == "rect" {
			rects = append(rects, fmt.Sprintf("%.1f,%.1f %.1fx%.1f %.1f", d.x, d.y, d.w, d.h, d.fill.R))
		}
	}
	assert.Equal(t, []string{
		"100.0,0.0 100.0x14.4 0.2", // row "c" background skips the carried column
		"0.0,0.0 100.0x14.4 0.1",   // continuation keeps the origin row background
	}, rects)
}

// TestTable_SplitKeepsColumnGrid verifies a continuation reuses the column
// grid of the whole table. Before, Auto columns and tables without Columns
// were re-resolved from the remaining rows: a 114pt Auto column became 6pt on
// the next page, and a 3-column table became 2 columns.
func TestTable_SplitKeepsColumnGrid(t *testing.T) {
	fonts := &MockFontResolver{}
	t.Run("auto_column", func(t *testing.T) {
		var body []TableRow
		for i := 0; i < 4; i++ {
			v := "x"
			if i == 0 {
				v = "a much longer value"
			}
			body = append(body, TableRow{Cells: []TableCell{textCell(v), textCell("rest")}})
		}
		tbl := &Table{Columns: []ColumnDef{{Width: Auto()}, {Width: Fr(1)}}, Body: body, Fonts: fonts}
		plans := paginateTable(t, tbl, 400, 30, 5)
		require.Greater(t, len(plans), 1)
		first := tdBlocks(plans[0])[1].X
		for i, p := range plans {
			assert.InDelta(t, first, tdBlocks(p)[1].X, 1e-9, "page %d", i+1)
		}
	})
	t.Run("no_columns", func(t *testing.T) {
		var body []TableRow
		for i := 0; i < 4; i++ {
			cs := []TableCell{textCell("a"), textCell("b")}
			if i < 2 {
				cs = append(cs, textCell("c"))
			}
			body = append(body, TableRow{Cells: cs})
		}
		plans := paginateTable(t, &Table{Body: body, Fonts: fonts}, 300, 30, 5)
		require.Greater(t, len(plans), 1)
		for i, p := range plans {
			for _, b := range tdBlocks(p) {
				assert.InDelta(t, 100.0, b.Width, 1e-9, "page %d", i+1)
			}
		}
	})
}

// TestTable_HeaderTallerThanPageEmptyBody verifies the header-only path does
// not return a typed-nil overflow when there are no body rows.
func TestTable_HeaderTallerThanPageEmptyBody(t *testing.T) {
	tbl := &Table{
		Columns: []ColumnDef{{Width: Fr(1)}},
		Header:  []TableRow{{Cells: []TableCell{textCell("H")}}},
		Fonts:   &MockFontResolver{},
	}
	// Mid-page: move to a fresh page.
	assert.Equal(t, Nothing, tbl.PlanLayout(Area{Width: 100, Height: 5}).Status)

	// Page top: place it (nothing better is possible), with no overflow.
	p := tbl.PlanLayout(Area{Width: 100, Height: 5, AtTop: true})
	assert.Equal(t, Full, p.Status)
	assert.Nil(t, p.Overflow)
}
