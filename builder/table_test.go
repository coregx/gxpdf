package builder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coregx/gxpdf/layout"
)

// --- helpers ---

// newTestBuilder creates a minimal Builder for use in table tests.
func newTestBuilder() *Builder {
	b := NewBuilder(
		WithPageSize(A4),
		WithMargins(Mm(10), Mm(10), Mm(10), Mm(10)),
	)
	return b
}

// --- TableBuilder unit tests ---

// TestTableBuilder_Columns verifies that column definitions are stored correctly.
func TestTableBuilder_Columns(t *testing.T) {
	b := newTestBuilder()
	tb := &TableBuilder{b: b}
	tb.Columns(Fr(1), Fr(2), Pt(80))

	require.Len(t, tb.columns, 3)
	assert.Equal(t, layout.UnitFr, tb.columns[0].Width.Unit)
	assert.InDelta(t, 1.0, tb.columns[0].Width.Amount, 0.001)
	assert.Equal(t, layout.UnitFr, tb.columns[1].Width.Unit)
	assert.InDelta(t, 2.0, tb.columns[1].Width.Amount, 0.001)
	assert.Equal(t, layout.UnitPt, tb.columns[2].Width.Unit)
	assert.InDelta(t, 80.0, tb.columns[2].Width.Amount, 0.001)
}

// TestTableBuilder_HeaderBodyFooter verifies sections are accumulated separately.
func TestTableBuilder_HeaderBodyFooter(t *testing.T) {
	b := newTestBuilder()
	tb := &TableBuilder{b: b}
	tb.Columns(Fr(1), Fr(1))

	tb.Header(func(h *TableRowBuilder) {
		h.Cell(func(c *CellBuilder) { c.Text("H1") })
		h.Cell(func(c *CellBuilder) { c.Text("H2") })
	})

	tb.Row(func(r *TableRowBuilder) {
		r.Cell(func(c *CellBuilder) { c.Text("B1") })
		r.Cell(func(c *CellBuilder) { c.Text("B2") })
	})
	tb.Row(func(r *TableRowBuilder) {
		r.Cell(func(c *CellBuilder) { c.Text("B3") })
		r.Cell(func(c *CellBuilder) { c.Text("B4") })
	})

	tb.Footer(func(f *TableRowBuilder) {
		f.Cell(func(c *CellBuilder) { c.Text("F1") })
		f.Cell(func(c *CellBuilder) { c.Text("F2") })
	})

	tbl := tb.build()
	assert.Len(t, tbl.Header, 1)
	assert.Len(t, tbl.Body, 2)
	assert.Len(t, tbl.Footer, 1)
}

// TestTableBuilder_ColSpanOption verifies ColSpan is propagated to the layout cell.
func TestTableBuilder_ColSpanOption(t *testing.T) {
	b := newTestBuilder()
	tb := &TableBuilder{b: b}
	tb.Columns(Fr(1), Fr(1), Fr(1))

	tb.Row(func(r *TableRowBuilder) {
		r.Cell(func(c *CellBuilder) { c.Text("Span3") }, ColSpan(3))
	})

	tbl := tb.build()
	require.Len(t, tbl.Body, 1)
	require.Len(t, tbl.Body[0].Cells, 1)
	assert.Equal(t, 3, tbl.Body[0].Cells[0].ColSpan)
}

// TestTableBuilder_RowSpanOption verifies RowSpan is propagated to the layout cell.
func TestTableBuilder_RowSpanOption(t *testing.T) {
	b := newTestBuilder()
	tb := &TableBuilder{b: b}
	tb.Columns(Fr(1), Fr(1))

	tb.Row(func(r *TableRowBuilder) {
		r.Cell(func(c *CellBuilder) { c.Text("Span2Rows") }, RowSpan(2))
		r.Cell(func(c *CellBuilder) { c.Text("Normal") })
	})
	tb.Row(func(r *TableRowBuilder) {
		r.Cell(func(c *CellBuilder) { c.Text("Row2Col2") })
	})

	tbl := tb.build()
	require.Len(t, tbl.Body[0].Cells, 2)
	assert.Equal(t, 2, tbl.Body[0].Cells[0].RowSpan)
}

// TestTableBuilder_BreakOptions verifies CellAllowBreak and CellRepeatOnBreak
// reach the layout cell, and are off by default.
func TestTableBuilder_BreakOptions(t *testing.T) {
	b := newTestBuilder()
	tb := &TableBuilder{b: b}
	tb.Columns(Fr(1), Fr(1), Fr(1))
	tb.Row(func(r *TableRowBuilder) {
		r.Cell(func(c *CellBuilder) { c.Text("default") }, RowSpan(2))
		r.Cell(func(c *CellBuilder) { c.Text("allow") }, RowSpan(2), CellAllowBreak())
		r.Cell(func(c *CellBuilder) { c.Text("repeat") }, RowSpan(2), CellRepeatOnBreak())
	})

	cells := tb.build().Body[0].Cells
	require.Len(t, cells, 3)
	assert.False(t, cells[0].AllowBreak)
	assert.False(t, cells[0].RepeatOnBreak)
	assert.True(t, cells[1].AllowBreak)
	assert.False(t, cells[1].RepeatOnBreak)
	assert.False(t, cells[2].AllowBreak)
	assert.True(t, cells[2].RepeatOnBreak)
}

// TestTableBuilder_TableRowBg verifies row background color is applied.
func TestTableBuilder_TableRowBg(t *testing.T) {
	b := newTestBuilder()
	tb := &TableBuilder{b: b}
	tb.Columns(Fr(1))

	tb.Row(func(r *TableRowBuilder) {
		r.Cell(func(c *CellBuilder) { c.Text("Zebra") })
	}, TableRowBg(LightGray))

	tbl := tb.build()
	require.Len(t, tbl.Body, 1)
	assert.NotNil(t, tbl.Body[0].Style.Background)
}

// TestTableBuilder_CellBg verifies cell background is applied.
func TestTableBuilder_CellBg(t *testing.T) {
	b := newTestBuilder()
	tb := &TableBuilder{b: b}
	tb.Columns(Fr(1))

	tb.Row(func(r *TableRowBuilder) {
		r.Cell(func(c *CellBuilder) { c.Text("hi") }, CellBg(LightGray))
	})

	tbl := tb.build()
	require.Len(t, tbl.Body[0].Cells, 1)
	assert.NotNil(t, tbl.Body[0].Cells[0].Style.Background)
}

// TestTableBuilder_CellPadding verifies cell padding is applied.
func TestTableBuilder_CellPadding(t *testing.T) {
	b := newTestBuilder()
	tb := &TableBuilder{b: b}
	tb.Columns(Fr(1))

	tb.Row(func(r *TableRowBuilder) {
		r.Cell(func(c *CellBuilder) { c.Text("padded") }, CellPadding(Pt(8)))
	})

	tbl := tb.build()
	cell := tbl.Body[0].Cells[0]
	// Uniform padding: all 4 sides should be 8pt.
	resolved := cell.Style.Padding.Resolve(100, 100, 12)
	assert.InDelta(t, 8.0, resolved.Top, 0.001)
	assert.InDelta(t, 8.0, resolved.Right, 0.001)
	assert.InDelta(t, 8.0, resolved.Bottom, 0.001)
	assert.InDelta(t, 8.0, resolved.Left, 0.001)
}

// TestTableBuilder_CellTextColorInheritance verifies CellTextColor is
// propagated to Text elements within cells.
func TestTableBuilder_CellTextColorInheritance(t *testing.T) {
	b := newTestBuilder()
	tb := &TableBuilder{b: b}
	tb.Columns(Fr(1))

	tb.Header(func(h *TableRowBuilder) {
		h.Cell(func(c *CellBuilder) {
			// No explicit TextColor — should inherit from row.
			c.Text("Header")
		})
	}, TableRowBg(Navy), CellTextColor(White))

	tbl := tb.build()
	require.Len(t, tbl.Header[0].Cells, 1)
	content := tbl.Header[0].Cells[0].Content
	require.NotEmpty(t, content)
	txt, ok := content[0].(*layout.Text)
	require.True(t, ok)
	// The white color should have been applied.
	assert.InDelta(t, 1.0, txt.Style.Color.R, 0.001)
	assert.InDelta(t, 1.0, txt.Style.Color.G, 0.001)
	assert.InDelta(t, 1.0, txt.Style.Color.B, 0.001)
}

// TestTableBuilder_VAlignMiddle verifies VAlignMiddle is propagated.
func TestTableBuilder_VAlignMiddle(t *testing.T) {
	b := newTestBuilder()
	tb := &TableBuilder{b: b}
	tb.Columns(Fr(1))

	tb.Row(func(r *TableRowBuilder) {
		r.Cell(func(c *CellBuilder) { c.Text("middle") }, CellVAlignMiddle())
	})

	tbl := tb.build()
	assert.Equal(t, layout.VAlignMiddle, tbl.Body[0].Cells[0].Style.VerticalAlign)
}

// TestTableBuilder_VAlignBottom verifies VAlignBottom is propagated.
func TestTableBuilder_VAlignBottom(t *testing.T) {
	b := newTestBuilder()
	tb := &TableBuilder{b: b}
	tb.Columns(Fr(1))

	tb.Row(func(r *TableRowBuilder) {
		r.Cell(func(c *CellBuilder) { c.Text("bottom") }, CellVAlignBottom())
	})

	tbl := tb.build()
	assert.Equal(t, layout.VAlignBottom, tbl.Body[0].Cells[0].Style.VerticalAlign)
}

// TestTableBuilder_CellBorder verifies CellBorder propagates to the cell style.
func TestTableBuilder_CellBorder(t *testing.T) {
	b := newTestBuilder()
	tb := &TableBuilder{b: b}
	tb.Columns(Fr(1))

	tb.Row(func(r *TableRowBuilder) {
		r.Cell(func(c *CellBuilder) { c.Text("bordered") }, CellBorder(DarkGray, 1.0))
	})

	tbl := tb.build()
	border := tbl.Body[0].Cells[0].Style.Border
	assert.InDelta(t, 1.0, border.Top.Width, 0.001)
	assert.InDelta(t, 1.0, border.Bottom.Width, 0.001)
}

// TestContainer_Table_IntegratesWithLayout verifies that a table added to a
// Container produces a valid layout plan when processed through the paginator.
func TestContainer_Table_IntegratesWithLayout(t *testing.T) {
	b := newTestBuilder()
	var capturedElements []layout.Element

	b.Page(func(p *PageBuilder) {
		p.Content(func(c *Container) {
			c.Table(func(tb *TableBuilder) {
				tb.Columns(Fr(1), Fr(1), Fr(1))

				tb.Header(func(h *TableRowBuilder) {
					h.Cell(func(c *CellBuilder) { c.Text("Name", Bold()) })
					h.Cell(func(c *CellBuilder) { c.Text("Q1", Bold()) })
					h.Cell(func(c *CellBuilder) { c.Text("Q2", Bold()) })
				}, TableRowBg(Navy), CellTextColor(White))

				tb.Row(func(r *TableRowBuilder) {
					r.Cell(func(c *CellBuilder) { c.Text("Revenue") })
					r.Cell(func(c *CellBuilder) { c.Text("$4.2M") })
					r.Cell(func(c *CellBuilder) { c.Text("$4.8M") })
				})

				tb.Row(func(r *TableRowBuilder) {
					r.Cell(func(c *CellBuilder) { c.Text("Margin") })
					r.Cell(func(c *CellBuilder) { c.Text("62%") })
					r.Cell(func(c *CellBuilder) { c.Text("64%") })
				}, TableRowBg(LightGray))
			})
			capturedElements = c.elements
		})
	})

	require.Len(t, capturedElements, 1)
	tbl, ok := capturedElements[0].(*layout.Table)
	require.True(t, ok, "element should be *layout.Table")
	assert.Len(t, tbl.Columns, 3)
	assert.Len(t, tbl.Header, 1)
	assert.Len(t, tbl.Body, 2)
}

// TestContainer_Table_BuildProducesValidPDF verifies that Build() succeeds
// when the document contains a table (no panics, no errors, non-empty output).
func TestContainer_Table_BuildProducesValidPDF(t *testing.T) {
	b := newTestBuilder()

	b.Page(func(p *PageBuilder) {
		p.Content(func(c *Container) {
			c.Table(func(tb *TableBuilder) {
				tb.Columns(Fr(1), Fr(1))

				tb.Header(func(h *TableRowBuilder) {
					h.Cell(func(c *CellBuilder) { c.Text("Item", Bold()) })
					h.Cell(func(c *CellBuilder) { c.Text("Value", Bold()) })
				}, TableRowBg(Navy), CellTextColor(White))

				data := []struct{ k, v string }{
					{"Revenue", "$4.2M"},
					{"Margin", "62%"},
					{"Customers", "1,247"},
				}
				for i, row := range data {
					bg := White
					if i%2 == 1 {
						bg = LightGray
					}
					r := row
					c.Table(func(inner *TableBuilder) {
						// purposely test nested; but at container level first.
						_ = inner
					})
					_ = bg
					_ = r
				}

				// Simpler: just direct rows.
				tb.Row(func(r *TableRowBuilder) {
					r.Cell(func(c *CellBuilder) { c.Text("Revenue") })
					r.Cell(func(c *CellBuilder) { c.Text("$4.2M") })
				})
				tb.Row(func(r *TableRowBuilder) {
					r.Cell(func(c *CellBuilder) { c.Text("Margin") })
					r.Cell(func(c *CellBuilder) { c.Text("62%") })
				}, TableRowBg(LightGray))

				tb.Footer(func(f *TableRowBuilder) {
					f.Cell(func(c *CellBuilder) {
						c.Text("Total", Bold(), AlignRight())
					}, ColSpan(1))
					f.Cell(func(c *CellBuilder) {
						c.Text("$4.2M", Bold())
					})
				})
			})
		})
	})

	pdfBytes, err := b.Build()
	require.NoError(t, err)
	assert.NotEmpty(t, pdfBytes)
}

// TestColSpan_Clamping verifies ColSpan clamps values < 1 to 1.
func TestColSpan_Clamping(t *testing.T) {
	opt := ColSpan(0)
	cfg := &cellConfig{}
	opt(cfg)
	assert.Equal(t, 1, cfg.colSpan)
}

// TestRowSpan_Clamping verifies RowSpan clamps values < 1 to 1.
func TestRowSpan_Clamping(t *testing.T) {
	opt := RowSpan(-5)
	cfg := &cellConfig{}
	opt(cfg)
	assert.Equal(t, 1, cfg.rowSpan)
}

// TestTableBuilder_EmptyTable verifies building an empty table does not panic.
func TestTableBuilder_EmptyTable(t *testing.T) {
	b := newTestBuilder()
	tb := &TableBuilder{b: b}
	tb.Columns(Fr(1))
	// No rows added.
	tbl := tb.build()
	assert.Empty(t, tbl.Body)
}

// TestTableBuilder_NoColumns verifies a table with no ColumnDef uses equal distribution.
func TestTableBuilder_NoColumns(t *testing.T) {
	b := newTestBuilder()
	tb := &TableBuilder{b: b}
	// Do not call tb.Columns() — should get equal distribution.
	tb.Row(func(r *TableRowBuilder) {
		r.Cell(func(c *CellBuilder) { c.Text("A") })
		r.Cell(func(c *CellBuilder) { c.Text("B") })
	})
	tbl := tb.build()
	assert.Empty(t, tbl.Columns, "no columns defined")
}

// TestApplyTextColorToElements_ExistingColorPreserved verifies that a Text
// element with an explicit non-zero color is not overridden.
func TestApplyTextColorToElements_ExistingColorPreserved(t *testing.T) {
	red := layout.Color{R: 1, G: 0, B: 0}
	white := layout.Color{R: 1, G: 1, B: 1}

	txt := &layout.Text{
		Content: "red text",
		Style:   layout.Style{Color: red},
	}

	result := applyTextColorToElements([]layout.Element{txt}, white)
	require.Len(t, result, 1)
	resultTxt, ok := result[0].(*layout.Text)
	require.True(t, ok)
	// Original red color should be preserved.
	assert.Equal(t, red, resultTxt.Style.Color)
}
