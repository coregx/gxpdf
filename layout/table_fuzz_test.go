package layout

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// fuzzBytes turns fuzz input into bounded choices; exhausted input yields 0.
type fuzzBytes struct {
	b []byte
	i int
}

func (f *fuzzBytes) n(limit int) int {
	if limit <= 0 || f.i >= len(f.b) {
		return 0
	}
	v := int(f.b[f.i])
	f.i++
	return v % limit
}

// fuzzTable builds a table from fuzz input. Every cell's text consists of
// tokens "c<id>w<k>" so drawn words can be traced back to their cell.
func fuzzTable(in *fuzzBytes) (tbl *Table, width, pageH float64) {
	fonts := &MockFontResolver{}
	nCols := 1 + in.n(4)
	id := 0
	cell := func(prefix string) TableCell {
		id++
		words := make([]string, in.n(25))
		for k := range words {
			words[k] = fmt.Sprintf("%s%dw%d", prefix, id, k)
		}
		c := TableCell{
			Content:       []Element{&Text{Content: strings.Join(words, " "), Fonts: fonts}},
			ColSpan:       1 + in.n(2),
			RowSpan:       1,
			AllowBreak:    in.n(4) == 0,
			RepeatOnBreak: in.n(4) == 0,
		}
		if in.n(3) == 0 {
			c.RowSpan = 2 + in.n(5)
		}
		c.Style.VerticalAlign = VAlign(in.n(3))
		if in.n(5) == 0 {
			bg := RGB(0.9, 0.9, 0.9)
			c.Style.Background = &bg
		}
		return c
	}
	rows := func(prefix string, n int) []TableRow {
		out := make([]TableRow, n)
		for r := range out {
			cells := make([]TableCell, in.n(nCols+1))
			for c := range cells {
				cells[c] = cell(prefix)
			}
			out[r].Cells = cells
		}
		return out
	}

	cols := make([]ColumnDef, nCols)
	for i := range cols {
		cols[i] = ColumnDef{Width: Fr(1)}
	}
	tbl = &Table{Columns: cols, Fonts: fonts}
	tbl.Header = rows("h", in.n(2))
	tbl.Body = rows("c", 1+in.n(16))
	width = float64(100 + in.n(300))
	pageH = float64(20 + in.n(180))
	return tbl, width, pageH
}

// cellWords returns the words of every cell placed in a section grid, by
// cell token prefix ("c7"), and whether that cell repeats on breaks.
func cellWords(rows []TableRow, colWidths []float64) (map[string][]string, map[string]bool) {
	words := map[string][]string{}
	repeat := map[string]bool{}
	for _, gc := range buildSectionGrid(rows, colWidths, nil).cells {
		text := gc.cell.Content[0].(*Text).Content
		ws := strings.Fields(text)
		if len(ws) == 0 {
			continue
		}
		key := ws[0][:strings.IndexByte(ws[0], 'w')]
		words[key] = ws
		repeat[key] = gc.cell.RepeatOnBreak && gc.rowSpan > 1
	}
	return words, repeat
}

// tokenCell returns the cell prefix of a token ("c7w3" → "c7").
func tokenCell(tok string) string {
	if i := strings.IndexByte(tok, 'w'); i > 0 {
		if _, err := strconv.Atoi(tok[1:i]); err == nil {
			return tok[:i]
		}
	}
	return ""
}

// snapshot copies the rows deeply enough to detect mutation by PlanLayout.
func snapshot(rows []TableRow) []TableRow {
	out := make([]TableRow, len(rows))
	for i, r := range rows {
		out[i] = TableRow{Style: r.Style}
		if r.Cells != nil {
			out[i].Cells = append(make([]TableCell, 0, len(r.Cells)), r.Cells...)
		}
	}
	return out
}

// FuzzTableRowSpans checks the invariants of ADR-007 §7 on random tables:
//
//	I1 every word of every placed body cell is drawn exactly once, or in
//	   whole copies for cells with RepeatOnBreak; header words appear once
//	   on every page
//	I2 cell boxes stay in the column grid and never overlap
//	I3 nothing is drawn below the page, except a page holding a single body
//	   row that is taller than an empty page
//	I4 every page places at least one body row, so pagination ends
//	I5 PlanLayout does not mutate the table
func FuzzTableRowSpans(f *testing.F) {
	f.Add([]byte{2, 3, 0, 0, 1, 0, 0, 0, 3, 1, 1, 0, 0, 0})
	f.Add([]byte("issue #100: team spans members across a page break"))
	f.Add([]byte{3, 8, 2, 20, 1, 0, 2, 0, 1, 9, 3, 1, 0, 1, 2, 3, 0, 15, 2, 1, 1, 0, 4, 0, 0, 30, 0, 10})
	f.Add([]byte{0, 15, 0, 24, 0, 0, 0, 0, 4, 1, 0, 1, 1, 0, 0, 0, 0, 255, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		tbl, width, pageH := fuzzTable(&fuzzBytes{b: data})
		colWidths := tbl.columnWidths(width)
		colEdges := map[float64]bool{width: true}
		x := 0.0
		for _, w := range colWidths {
			colEdges[x] = true
			x += w
		}
		want, repeat := cellWords(tbl.Body, colWidths)
		headerWords, _ := cellWords(tbl.Header, colWidths)
		before := snapshot(tbl.Body)

		got := map[string][]string{}
		var elem Element = tbl
		for page := 1; elem != nil; page++ {
			cur := elem.(*Table)
			if page > len(tbl.Body)+1 {
				t.Fatalf("I4: not done after %d pages for %d body rows", page-1, len(tbl.Body))
			}
			p := cur.PlanLayout(Area{Width: width, Height: pageH, AtTop: true})
			if p.Status == Nothing {
				t.Fatalf("page %d: Nothing at the page top", page)
			}
			if (p.Status == Full) != (p.Overflow == nil) {
				t.Fatalf("page %d: status %v with overflow %v", page, p.Status, p.Overflow != nil)
			}

			placed := len(cur.Body)
			if p.Overflow != nil {
				placed -= len(p.Overflow.(*Table).Body)
			}
			if placed < 1 {
				t.Fatalf("I4: page %d placed %d body rows", page, placed)
			}
			if p.Consumed > pageH+FitTolerance && placed != 1 {
				t.Fatalf("I3: page %d consumed %.3f > %.3f with %d body rows", page, p.Consumed, pageH, placed)
			}

			tds := tdBlocks(p)
			for i, a := range tds {
				if !nearEdge(colEdges, a.X) || !nearEdge(colEdges, a.X+a.Width) {
					t.Fatalf("I2: page %d cell at x=%.3f w=%.3f off the column grid", page, a.X, a.Width)
				}
				if p.Consumed <= pageH+FitTolerance && a.Y+a.Height > pageH+FitTolerance {
					t.Fatalf("I3: page %d cell bottom %.3f below %.3f", page, a.Y+a.Height, pageH)
				}
				for _, b := range tds[i+1:] {
					if overlaps(a, b) {
						t.Fatalf("I2: page %d cells overlap: %+v and %+v", page, box(a), box(b))
					}
				}
			}

			pageHeader := map[string][]string{}
			for _, d := range capture(p.Blocks) {
				if d.kind != "text" {
					continue
				}
				for _, tok := range strings.Fields(d.text) {
					key := tokenCell(tok)
					switch {
					case strings.HasPrefix(key, "h"):
						pageHeader[key] = append(pageHeader[key], tok)
					case key != "":
						got[key] = append(got[key], tok)
					}
				}
			}
			if !reflect.DeepEqual(normalize(pageHeader), normalize(headerWords)) {
				t.Fatalf("I1: page %d header words %v, want %v", page, pageHeader, headerWords)
			}
			elem = p.Overflow
		}

		for key, ws := range want {
			g := got[key]
			if repeat[key] {
				if len(g) == 0 || len(g)%len(ws) != 0 {
					t.Fatalf("I1: cell %s drew %d words, want whole copies of %d", key, len(g), len(ws))
				}
				for i := 0; i < len(g); i += len(ws) {
					if !reflect.DeepEqual(g[i:i+len(ws)], ws) {
						t.Fatalf("I1: cell %s copy at %d is %v, want %v", key, i, g[i:i+len(ws)], ws)
					}
				}
				continue
			}
			if !reflect.DeepEqual(g, ws) {
				t.Fatalf("I1: cell %s drew %v, want %v", key, g, ws)
			}
		}
		for key := range got {
			if _, ok := want[key]; !ok {
				t.Fatalf("I1: drew words of cell %s that is not in the grid", key)
			}
		}

		if !reflect.DeepEqual(before, tbl.Body) {
			t.Fatal("I5: PlanLayout mutated the table body")
		}
	})
}

// nearEdge reports whether x is within rounding of a column edge (widths
// summed in a different order).
func nearEdge(edges map[float64]bool, x float64) bool {
	for e := range edges {
		if x > e-1e-9 && x < e+1e-9 {
			return true
		}
	}
	return false
}

// overlaps reports whether two blocks overlap with positive area.
func overlaps(a, b Block) bool {
	const eps = 1e-9
	return a.X+a.Width > b.X+eps && b.X+b.Width > a.X+eps &&
		a.Y+a.Height > b.Y+eps && b.Y+b.Height > a.Y+eps
}

func box(b Block) string {
	return fmt.Sprintf("%.3f,%.3f %.3fx%.3f", b.X, b.Y, b.Width, b.Height)
}

// normalize maps an empty map to nil so DeepEqual treats them alike.
func normalize(m map[string][]string) map[string][]string {
	if len(m) == 0 {
		return nil
	}
	return m
}
