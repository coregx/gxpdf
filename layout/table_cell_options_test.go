package layout

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withTeamOptions applies f to every spanning (team) cell of a roster table.
func withTeamOptions(tbl *Table, f func(*TableCell)) *Table {
	for r := range tbl.Body {
		for c := range tbl.Body[r].Cells {
			if tbl.Body[r].Cells[c].RowSpan > 1 {
				f(&tbl.Body[r].Cells[c])
			}
		}
	}
	return tbl
}

// TestTable_AllowBreak_DenseFill verifies AllowBreak lets rows fill the page
// instead of moving the group to the next page (ADR-007 §5 rule 2).
func TestTable_AllowBreak_DenseFill(t *testing.T) {
	tbl := withTeamOptions(rosterTable(2, 3), func(c *TableCell) { c.AllowBreak = true })
	pages := pageTexts(t, 60, tbl) // 4 rows per page
	require.Len(t, pages, 2)
	assert.Equal(t, []string{"team 1", "t1 m1", "t1 m2", "team 2", "t2 m1", "t2 m2"}, pages[0])
	assert.Equal(t, []string{"t2 m3"}, pages[1], "team 2 continues without its label")
}

// TestTable_AllowBreak_EveryCrossingCellMustAllow verifies a boundary is a
// break opportunity only if every cell spanning across it allows breaks.
func TestTable_AllowBreak_EveryCrossingCellMustAllow(t *testing.T) {
	build := func(allowB bool) *Table {
		a := textCell("A", withRowSpan(3))
		a.AllowBreak = true
		b := textCell("B", withRowSpan(3))
		b.AllowBreak = allowB
		return simpleTable(3, []TableRow{
			{Cells: []TableCell{a, textCell("m0"), b}},
			{Cells: []TableCell{textCell("m1")}},
			{Cells: []TableCell{textCell("m2")}},
		}, &MockFontResolver{})
	}
	intro := &Text{Content: "intro", Fonts: &MockFontResolver{}}

	// Page 50: after the intro (14.4) two of the group's three rows fit;
	// the group (43.2) fits on an empty page.
	both := pageTexts(t, 50, intro, build(true))
	require.Len(t, both, 2)
	assert.Equal(t, []string{"intro", "A", "m0", "B", "m1"}, both[0], "both cells allow: break inside the group")

	one := pageTexts(t, 50, intro, build(false))
	require.Len(t, one, 2)
	assert.Equal(t, []string{"intro"}, one[0], "B keeps its rows together: the group moves")
	assert.Equal(t, []string{"A", "m0", "B", "m1", "m2"}, one[1])
}

// TestTable_RepeatOnBreak_Label verifies a label that was shown completely is
// shown again, with its alignment, on every page the span continues on
// (ADR-007 §5 rule 3).
func TestTable_RepeatOnBreak_Label(t *testing.T) {
	tbl := withTeamOptions(rosterTable(10), func(c *TableCell) {
		c.RepeatOnBreak = true
		c.Style.VerticalAlign = VAlignMiddle
	})
	plans := paginateTable(t, tbl, 200, 60, 5) // 4 + 4 + 2 rows

	require.Len(t, plans, 3)
	for i, p := range plans {
		rows := len(textsAt(capture(p.Blocks), 100))
		var label []drawn
		for _, d := range capture(p.Blocks) {
			if d.kind == "text" && d.x == 0 {
				label = append(label, d)
			}
		}
		require.Len(t, label, 1, "page %d shows the label once", i+1)
		assert.Equal(t, "team 1", label[0].text)
		// Centered in the fragment: (rows*14.4 - 14.4)/2 + half leading 1.2.
		assert.InDelta(t, float64(rows-1)*14.4/2+1.2, label[0].y, 1e-9, "page %d", i+1)
	}
}

// TestTable_RepeatOnBreak_LongTextNotDuplicated verifies content that is still
// continuing is continued, not repeated, and is not repeated after it ends
// (ADR-007 I1).
func TestTable_RepeatOnBreak_LongTextNotDuplicated(t *testing.T) {
	text := strings.Repeat("word ", 30)
	lines := len((&Text{Content: text, Fonts: &MockFontResolver{}}).PlanLayout(Area{Width: 100, Height: 1e9}).Blocks)
	rows := lines + 6 // the span outlasts the text by two pages of 3 rows
	var body []TableRow
	for i := 0; i < rows; i++ {
		cells := []TableCell{textCell(fmt.Sprintf("r%d", i))}
		if i == 0 {
			c := textCell(text, withRowSpan(rows))
			c.RepeatOnBreak = true
			cells = append(cells, c)
		}
		body = append(body, TableRow{Cells: cells})
	}
	plans := paginateTable(t, simpleTable(2, body, &MockFontResolver{}), 200, 50, 50)

	var words []string
	lastWithText := -1
	for i, p := range plans {
		for _, s := range textsAt(capture(p.Blocks), 100) {
			words = append(words, strings.Fields(s)...)
			lastWithText = i
		}
	}
	assert.Equal(t, strings.Fields(text), words, "every word exactly once")
	require.Less(t, lastWithText, len(plans)-2, "the span continues for pages after the text ends")
	for i := lastWithText + 1; i < len(plans); i++ {
		assert.Empty(t, textsAt(capture(plans[i].Blocks), 100), "page %d: not repeated after the text ended", i+1)
	}
}

// TestTable_RepeatOnBreak_ZeroValueDoesNotRepeat verifies the default.
func TestTable_RepeatOnBreak_ZeroValueDoesNotRepeat(t *testing.T) {
	plans := paginateTable(t, rosterTable(10), 200, 60, 5)
	require.Len(t, plans, 3)
	for i, p := range plans[1:] {
		assert.Empty(t, textsAt(capture(p.Blocks), 0), "page %d", i+2)
	}
}
