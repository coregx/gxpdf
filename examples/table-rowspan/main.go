// Package main demonstrates row-spanning cells in the Builder table API with
// a duty roster: each team's name, vehicle and shift span all of its member
// rows.
//
// It shows:
//   - RowSpan cells covering the full height of the rows they span, with
//     wrapped text and vertical centering
//   - teams kept on one page: a team that does not fit in the rest of a page
//     moves to the next page whole, under the repeated table header
//   - a team taller than a page, broken across pages, with CellRepeatOnBreak
//     repeating its name and vehicle on each page
//   - CellAllowBreak for a dense log table that fills every page
//
// Usage:
//
//	go run ./examples/table-rowspan [output.pdf]
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/coregx/gxpdf/builder"
)

type team struct {
	name, vehicle, shift string
	members              int
}

func main() {
	out := "table-rowspan.pdf"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}

	b := builder.NewBuilder(
		builder.WithPageSize(builder.A4),
		builder.WithMargins(builder.Mm(15), builder.Mm(15), builder.Mm(15), builder.Mm(15)),
		builder.WithDefaultFontSize(10),
		builder.WithTitle("Duty roster"),
	)

	teams := []team{
		{"Alpha", "Engine 1 (HLF 20)", "06:00 - 18:00", 4},
		{"Bravo", "Engine 2 (LF 10)", "06:00 - 18:00", 6},
		{"Charlie", "Ladder 1 (DLK 23/12)", "18:00 - 06:00", 5},
		{"Delta", "Rescue 1 (RW 2)", "18:00 - 06:00", 7},
		{"Echo", "Tanker 1 (TLF 4000)", "On call", 9},
		{"Reserve pool", "Assigned per incident by the shift commander", "On call", 70},
	}

	b.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) {
			c.Text("Duty roster", builder.Bold(), builder.FontSize(16))
			c.Text("Teams stay on one page; the reserve pool is longer than a page and repeats its name on each page.",
				builder.FontSize(9))
			c.Spacer(builder.Mm(4))
			rosterTable(c, teams)

			c.PageBreak()
			c.Text("Radio log", builder.Bold(), builder.FontSize(16))
			c.Text("CellAllowBreak: each day's entries fill the page instead of moving together.", builder.FontSize(9))
			c.Spacer(builder.Mm(4))
			logTable(c)
		})
	})

	if err := b.BuildToFile(out); err != nil {
		log.Fatalf("build: %v", err)
	}
	fmt.Println("written:", out)
}

func rosterTable(c *builder.Container, teams []team) {
	border := builder.CellBorder(builder.Gray, 0.5)
	spanned := func(n int) []builder.CellOption {
		return []builder.CellOption{border, builder.RowSpan(n), builder.CellVAlignMiddle(),
			builder.CellPadding(builder.Pt(4)), builder.CellRepeatOnBreak()}
	}

	c.Table(func(t *builder.TableBuilder) {
		t.Columns(builder.Fr(2), builder.Fr(3), builder.Fr(3), builder.Fr(2))
		t.Header(func(r *builder.TableRowBuilder) {
			for _, h := range []string{"Team", "Member", "Vehicle", "Shift"} {
				r.Cell(func(cb *builder.CellBuilder) { cb.Text(h, builder.Bold()) },
					border, builder.CellPadding(builder.Pt(4)))
			}
		}, builder.TableRowBg(builder.LightGray))

		for _, tm := range teams {
			for i := 1; i <= tm.members; i++ {
				t.Row(func(r *builder.TableRowBuilder) {
					if i == 1 {
						r.Cell(func(cb *builder.CellBuilder) { cb.Text(tm.name, builder.Bold()) }, spanned(tm.members)...)
					}
					r.Cell(func(cb *builder.CellBuilder) { cb.Text(fmt.Sprintf("%s %02d", tm.name, i)) },
						border, builder.CellPadding(builder.Pt(4)))
					if i == 1 {
						r.Cell(func(cb *builder.CellBuilder) { cb.Text(tm.vehicle) }, spanned(tm.members)...)
						r.Cell(func(cb *builder.CellBuilder) { cb.Text(tm.shift) }, spanned(tm.members)...)
					}
				})
			}
		}
	})
}

func logTable(c *builder.Container) {
	border := builder.CellBorder(builder.Gray, 0.5)
	pad := builder.CellPadding(builder.Pt(4))
	c.Table(func(t *builder.TableBuilder) {
		t.Columns(builder.Fr(1), builder.Fr(1), builder.Fr(4))
		t.Header(func(r *builder.TableRowBuilder) {
			for _, h := range []string{"Day", "Time", "Entry"} {
				r.Cell(func(cb *builder.CellBuilder) { cb.Text(h, builder.Bold()) }, border, pad)
			}
		}, builder.TableRowBg(builder.LightGray))

		for day := 1; day <= 3; day++ {
			const entries = 30
			for i := 0; i < entries; i++ {
				t.Row(func(r *builder.TableRowBuilder) {
					if i == 0 {
						r.Cell(func(cb *builder.CellBuilder) { cb.Text(fmt.Sprintf("Day %d", day), builder.Bold()) },
							border, pad, builder.RowSpan(entries), builder.CellAllowBreak(), builder.CellRepeatOnBreak())
					}
					r.Cell(func(cb *builder.CellBuilder) { cb.Text(fmt.Sprintf("%02d:%02d", 6+i/2, i%2*30)) }, border, pad)
					r.Cell(func(cb *builder.CellBuilder) { cb.Text(fmt.Sprintf("Status report %d from unit %d", i+1, i%5+1)) }, border, pad)
				})
			}
		}
	})
}
