package builder

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coregx/gxpdf/layout"
)

// updateGolden regenerates golden files instead of comparing:
//
//	go test ./builder -run TestTableGolden -update-golden
var updateGolden = flag.Bool("update-golden", false, "regenerate golden test files instead of comparing")

// opRecorder is a layout.Renderer that logs every drawing call in absolute
// layout coordinates (top-left origin). The log is a deterministic, readable
// snapshot of what the PDF renderer would emit.
type opRecorder struct {
	sb           *strings.Builder
	baseX, baseY float64
}

func (r *opRecorder) DrawText(text string, x, y float64, font layout.FontRef, size float64, color layout.Color, o layout.TextDrawOptions) {
	fmt.Fprintf(r.sb, "text %.3f %.3f %s/%d/%d %.2f %s %q ls=%.3f ws=%.3f u=%t s=%t\n",
		r.baseX+x, r.baseY+y, font.Family, font.Weight, font.Style, size, fmtColor(&color), text,
		o.LetterSpacing, o.WordSpacing, o.Underline, o.Strikethrough)
}

func (r *opRecorder) DrawRect(x, y, w, h float64, fill, stroke *layout.Color, sw float64) {
	fmt.Fprintf(r.sb, "rect %.3f %.3f %.3f %.3f fill=%s stroke=%s %.3f\n",
		r.baseX+x, r.baseY+y, w, h, fmtColor(fill), fmtColor(stroke), sw)
}

func (r *opRecorder) DrawLine(x1, y1, x2, y2 float64, color layout.Color, w float64) {
	fmt.Fprintf(r.sb, "line %.3f %.3f %.3f %.3f %s %.3f\n",
		r.baseX+x1, r.baseY+y1, r.baseX+x2, r.baseY+y2, fmtColor(&color), w)
}

func (r *opRecorder) DrawImage(data []byte, x, y, w, h float64) {
	fmt.Fprintf(r.sb, "image %.3f %.3f %.3f %.3f %dB\n", r.baseX+x, r.baseY+y, w, h, len(data))
}

func (r *opRecorder) PushState() { r.sb.WriteString("push\n") }
func (r *opRecorder) PopState()  { r.sb.WriteString("pop\n") }

func (r *opRecorder) SetClipRect(x, y, w, h float64) {
	fmt.Fprintf(r.sb, "clip %.3f %.3f %.3f %.3f\n", r.baseX+x, r.baseY+y, w, h)
}

func fmtColor(c *layout.Color) string {
	if c == nil {
		return "-"
	}
	return fmt.Sprintf("%.3f,%.3f,%.3f", c.R, c.G, c.B)
}

// recordBlock mirrors builder/internal pageRenderer.renderBlock: Draw with the
// block origin, then children offset by the parent origin.
func recordBlock(sb *strings.Builder, b *layout.Block) {
	if b.Draw != nil {
		b.Draw(&opRecorder{sb: sb, baseX: b.X, baseY: b.Y})
	}
	for i := range b.Children {
		child := b.Children[i]
		child.X += b.X
		child.Y += b.Y
		recordBlock(sb, &child)
	}
}

// recordDrawOps lays out the document and returns the drawing log of all pages.
func recordDrawOps(t *testing.T, b *Builder) string {
	t.Helper()
	if err := b.accumulatedError(); err != nil {
		t.Fatalf("builder errors: %v", err)
	}
	var sb strings.Builder
	for i, pl := range b.layoutPages() {
		fmt.Fprintf(&sb, "page %d %.3fx%.3f\n", i+1, pl.Size.Width, pl.Size.Height)
		for j := range pl.Blocks {
			recordBlock(&sb, &pl.Blocks[j])
		}
	}
	return sb.String()
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name+".golden")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (run with -update-golden to create): %v", path, err)
	}
	if string(want) != got {
		t.Errorf("drawing ops differ from %s (run with -update-golden to accept):\n%s", path, firstDiff(string(want), got))
	}
}

// firstDiff reports the first differing line with a little context.
func firstDiff(want, got string) string {
	w := strings.Split(want, "\n")
	g := strings.Split(got, "\n")
	for i := 0; i < len(w) || i < len(g); i++ {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl {
			return fmt.Sprintf("line %d:\n  want: %s\n  got:  %s", i+1, wl, gl)
		}
	}
	return "(no line difference)"
}

// TestTableGolden_NoRowSpan pins the output of tables without row spans.
// ADR-007 invariant I6: the row-span rework must not change these bytes.
// The golden files were captured on main (v0.9.5 + #105) before the rework.
func TestTableGolden_NoRowSpan(t *testing.T) {
	navy := RGB(0.1, 0.15, 0.35)
	zebra := RGB(0.94, 0.94, 0.94)

	t.Run("multipage_header_footer", func(t *testing.T) {
		doc := NewBuilder(WithPageSize(A4))
		doc.Page(func(p *PageBuilder) {
			p.Content(func(c *Container) {
				c.Text("Inventory")
				c.Table(func(tb *TableBuilder) {
					tb.Columns(Pt(40), Fr(2), Auto(), Pct(20))
					tb.Header(func(r *TableRowBuilder) {
						for _, h := range []string{"#", "Item", "Qty", "Price"} {
							r.Cell(func(cb *CellBuilder) { cb.Text(h, Bold()) }, CellBorder(Black, 0.5))
						}
					}, TableRowBg(navy), CellTextColor(White))
					for i := 1; i <= 70; i++ {
						opts := []TableRowOption{}
						if i%2 == 0 {
							opts = append(opts, TableRowBg(zebra))
						}
						tb.Row(func(r *TableRowBuilder) {
							r.Cell(func(cb *CellBuilder) { cb.Text(fmt.Sprintf("%d", i)) }, CellBorder(Black, 0.5))
							item := fmt.Sprintf("Item %d", i)
							if i%7 == 0 {
								item += " with a longer description that wraps onto a second line in this column"
							}
							r.Cell(func(cb *CellBuilder) { cb.Text(item) }, CellBorder(Black, 0.5))
							r.Cell(func(cb *CellBuilder) { cb.Text(fmt.Sprintf("%d", i*3), AlignRight()) },
								CellBorder(Black, 0.5), CellVAlignBottom())
							r.Cell(func(cb *CellBuilder) { cb.Text(fmt.Sprintf("$%d.99", i)) },
								CellBorder(Black, 0.5), CellVAlignMiddle())
						}, opts...)
					}
					tb.Footer(func(r *TableRowBuilder) {
						r.Cell(func(cb *CellBuilder) { cb.Text("Total", Bold(), AlignRight()) },
							ColSpan(3), CellBorder(Black, 1), CellBg(zebra))
						r.Cell(func(cb *CellBuilder) { cb.Text("$2555.30", Bold()) }, CellBorder(Black, 1))
					})
				})
				c.Text("After the table")
			})
		})
		assertGolden(t, "table_norowspan_multipage", recordDrawOps(t, doc))
	})

	t.Run("cell_styles_and_nesting", func(t *testing.T) {
		doc := NewBuilder(WithPageSize(A4))
		doc.Page(func(p *PageBuilder) {
			p.Content(func(c *Container) {
				c.Table(func(tb *TableBuilder) {
					tb.Columns(Fr(1), Fr(1), Fr(1))
					tb.Row(func(r *TableRowBuilder) {
						r.Cell(func(cb *CellBuilder) {
							cb.Text("Padded, colored, top-aligned cell with enough text to wrap twice in a third of A4")
						}, CellPadding(Pt(8)), CellBg(zebra), CellBorder(navy, 2), CellVAlignTop())
						r.Cell(func(cb *CellBuilder) { cb.Text("middle") }, CellVAlignMiddle(), CellBorder(Black, 0.5))
						r.Cell(func(cb *CellBuilder) {
							cb.Text("nested:")
							cb.Table(func(inner *TableBuilder) {
								inner.Columns(Fr(1), Fr(1))
								inner.Row(func(ir *TableRowBuilder) {
									ir.Cell(func(icb *CellBuilder) { icb.Text("a") }, CellBorder(Black, 0.25))
									ir.Cell(func(icb *CellBuilder) { icb.Text("b") }, CellBorder(Black, 0.25))
								})
								inner.Row(func(ir *TableRowBuilder) {
									ir.Cell(func(icb *CellBuilder) { icb.Text("wide") }, ColSpan(2), CellBorder(Black, 0.25))
								})
							})
						}, CellBorder(Black, 0.5), CellVAlignBottom())
					})
					tb.Row(func(r *TableRowBuilder) {
						r.Cell(func(cb *CellBuilder) { cb.Text("colspan 2") }, ColSpan(2), CellBorder(Black, 0.5))
						r.Cell(func(cb *CellBuilder) {}, CellBorder(Black, 0.5))
					}, TableRowBg(zebra))
					tb.Row(func(r *TableRowBuilder) {
						r.Cell(func(cb *CellBuilder) { cb.Text("short row") }, CellBorder(Black, 0.5))
					})
				})
			})
		})
		assertGolden(t, "table_norowspan_styles", recordDrawOps(t, doc))
	})
}
