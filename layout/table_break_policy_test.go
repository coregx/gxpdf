package layout

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rosterTable returns a duty roster: teams of the given sizes, each with a
// TEAM cell spanning its member rows.
func rosterTable(sizes ...int) *Table {
	var body []TableRow
	for ti, n := range sizes {
		for i := 0; i < n; i++ {
			var cells []TableCell
			if i == 0 {
				cells = append(cells, textCell(fmt.Sprintf("team %d", ti+1), withRowSpan(n)))
			}
			cells = append(cells, textCell(fmt.Sprintf("t%d m%d", ti+1, i+1)))
			body = append(body, TableRow{Cells: cells})
		}
	}
	return simpleTable(2, body, &MockFontResolver{})
}

// pageTexts paginates elements on pages of the given body height and returns
// the texts drawn on each page.
func pageTexts(t *testing.T, height float64, elems ...Element) [][]string {
	t.Helper()
	pages := (&Paginator{}).Paginate([]*PageDef{{
		Size:    Size{Width: 200, Height: height},
		Content: elems,
	}})
	out := make([][]string, len(pages))
	for i, pg := range pages {
		for _, d := range capture(pg.Blocks) {
			if d.kind == "text" {
				out[i] = append(out[i], d.text)
			}
		}
	}
	return out
}

// assertOnPage checks that every page's blocks end within the page body,
// except blocks taller than an empty page (ADR-007 I3).
func assertWithinPages(t *testing.T, height float64, elems ...Element) {
	t.Helper()
	pages := (&Paginator{}).Paginate([]*PageDef{{
		Size:    Size{Width: 200, Height: height},
		Content: elems,
	}})
	for i, pg := range pages {
		for _, b := range pg.Blocks {
			if b.Height <= height {
				assert.LessOrEqual(t, b.Y+b.Height, height+FitTolerance, "page %d: block runs off the page", i+1)
			}
		}
	}
}

// TestTable_SpanGroupMovesWhole verifies the default policy: a span group
// that does not fit in the rest of the page moves to the next page whole,
// instead of being torn (ADR-007 §5 rule 1).
func TestTable_SpanGroupMovesWhole(t *testing.T) {
	// Page body 60pt holds 4 rows of 14.4pt. Team 1 has 2 rows, team 2 has 3:
	// after team 1 (28.8pt) only 2 of team 2's 3 rows would fit.
	pages := pageTexts(t, 60, rosterTable(2, 3))
	require.Len(t, pages, 2)
	assert.Equal(t, []string{"team 1", "t1 m1", "t1 m2"}, pages[0])
	assert.Equal(t, []string{"team 2", "t2 m1", "t2 m2", "t2 m3"}, pages[1])
}

// TestTable_SpanGroupMovesWhole_AfterOtherContent verifies a table whose first
// group does not fit below other content moves to the next page whole.
func TestTable_SpanGroupMovesWhole_AfterOtherContent(t *testing.T) {
	intro := &Text{Content: "intro", Fonts: &MockFontResolver{}}
	pages := pageTexts(t, 60, intro, rosterTable(4)) // 14.4 + 4*14.4 > 60
	require.Len(t, pages, 2)
	assert.Equal(t, []string{"intro"}, pages[0])
	assert.Equal(t, "team 1", pages[1][0])
	assert.Len(t, pages[1], 5)
}

// TestTable_SpanGroupTallerThanPage verifies a group that does not fit even
// on an empty page is broken (the keep-together rule is relaxed), with the
// span continuing, and nothing runs off a page.
func TestTable_SpanGroupTallerThanPage(t *testing.T) {
	tbl := rosterTable(1, 7) // team 2: 7 rows = 100.8pt > 60pt
	pages := pageTexts(t, 60, tbl)
	require.Len(t, pages, 3)
	assert.Equal(t, []string{"team 1", "t1 m1"}, pages[0], "team 2 does not fit after team 1 and moves")
	assert.Equal(t, []string{"team 2", "t2 m1", "t2 m2", "t2 m3", "t2 m4"}, pages[1])
	assert.Equal(t, []string{"t2 m5", "t2 m6", "t2 m7"}, pages[2])
	assertWithinPages(t, 60, rosterTable(1, 7))
}

// TestTable_OversizedRowDoesNotDragTable verifies that a body row taller than
// an empty page is placed alone and the rest of the table continues on the
// next pages. Before, the paginator forced the table with unlimited height
// and the whole table was drawn on one page, below its bottom edge.
func TestTable_OversizedRowDoesNotDragTable(t *testing.T) {
	fonts := &MockFontResolver{}
	body := []TableRow{{Cells: []TableCell{textCell(strings.Repeat("big ", 40))}}}
	for i := 0; i < 5; i++ {
		body = append(body, TableRow{Cells: []TableCell{textCell(fmt.Sprintf("r%d", i))}})
	}
	tbl := simpleTable(1, body, fonts)

	pages := pageTexts(t, 30, tbl)
	require.Greater(t, len(pages), 2)
	for i := 1; i < len(pages); i++ {
		for _, s := range pages[i] {
			assert.NotContains(t, s, "big", "page %d", i+1)
		}
	}
	var rows []string
	for _, p := range pages[1:] {
		rows = append(rows, p...)
	}
	assert.Equal(t, []string{"r0", "r1", "r2", "r3", "r4"}, rows)
}

// TestTable_HeaderTallerThanPageTerminates verifies pagination always makes
// progress (ADR-007 I4). A header taller than the page used to produce a page
// with only the header and the same body as overflow, forever.
func TestTable_HeaderTallerThanPageTerminates(t *testing.T) {
	fonts := &MockFontResolver{}
	var body []TableRow
	for i := 0; i < 3; i++ {
		body = append(body, TableRow{Cells: []TableCell{textCell(fmt.Sprintf("r%d", i))}})
	}
	tbl := &Table{
		Columns: []ColumnDef{{Width: Fr(1)}},
		Header:  []TableRow{{Cells: []TableCell{textCell(strings.Repeat("h ", 200))}}},
		Body:    body,
		Fonts:   fonts,
	}

	plans := paginateTable(t, tbl, 200, 30, 10)
	assert.Len(t, plans, 3, "one body row per page")
}

// TestTable_NotAtTopNothingFits verifies Nothing is returned mid-page when not
// even a single row fits, so the paginator retries on a fresh page.
func TestTable_NotAtTopNothingFits(t *testing.T) {
	tbl := rosterTable(1, 1)
	assert.Equal(t, Nothing, tbl.PlanLayout(Area{Width: 200, Height: 10}).Status)
	p := tbl.PlanLayout(Area{Width: 200, Height: 10, AtTop: true})
	assert.Equal(t, Partial, p.Status, "at the page top one row is placed")
}

// TestTable_SpanGroupInsideCell verifies cell content is laid out with AtTop:
// a nested table in the fragment of a broken span cannot move to another page
// on its own, so its own span group must break into the fragment instead of
// returning Nothing (which would leave the fragment empty).
func TestTable_SpanGroupInsideCell(t *testing.T) {
	inner := rosterTable(5) // one group, 72pt
	outer := simpleTable(2, []TableRow{
		{Cells: []TableCell{{Content: []Element{inner}, RowSpan: 2}, textCell("a")}},
		{Cells: []TableCell{textCell("b")}},
	}, &MockFontResolver{})

	// Outer rows grow to 36pt each; a 50pt page breaks the outer group after
	// row "a", leaving a 36pt fragment for the nested table.
	plans := paginateTable(t, outer, 200, 50, 5)
	require.Len(t, plans, 2)

	var page1, all []string
	for i, p := range plans {
		for _, d := range capture(p.Blocks) {
			if d.kind == "text" && d.x < 100 {
				all = append(all, d.text)
				if i == 0 {
					page1 = append(page1, d.text)
				}
			}
		}
	}
	assert.Equal(t, []string{"team 1", "t1 m1", "t1 m2"}, page1, "nested group broken into the fragment")
	assert.Equal(t, []string{"team 1", "t1 m1", "t1 m2", "t1 m3", "t1 m4", "t1 m5"}, all)
}

// TestArea_AtTopPropagation verifies how AtTop reaches nested elements: the
// paginator sets it at the page top, a vertical Box passes it to its first
// child only, a horizontal Box to every child.
func TestArea_AtTopPropagation(t *testing.T) {
	var seen []string
	probe := func(name string) Element {
		return elementFunc(func(a Area) Plan {
			seen = append(seen, fmt.Sprintf("%s:%t", name, a.AtTop))
			return Plan{Status: Full, Consumed: 10}
		})
	}
	paginate := func(content ...Element) []string {
		seen = nil
		(&Paginator{}).Paginate([]*PageDef{{Size: Size{Width: 200, Height: 500}, Content: content}})
		return seen
	}

	assert.Equal(t, []string{"v1:true", "v2:false", "h1:false", "h2:false"}, paginate(
		&Box{Children: []Element{probe("v1"), probe("v2")}},
		&Box{Direction: Horizontal, Children: []Element{probe("h1"), probe("h2")}},
	))
	assert.Equal(t, []string{"h1:true", "h2:true", "v1:false"}, paginate(
		&Box{Direction: Horizontal, Children: []Element{probe("h1"), probe("h2")}},
		&Box{Children: []Element{probe("v1")}},
	))
}

// elementFunc adapts a function to Element.
type elementFunc func(Area) Plan

func (f elementFunc) PlanLayout(a Area) Plan { return f(a) }
