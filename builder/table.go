package builder

import (
	"github.com/coregx/gxpdf/layout"
)

// TableBuilder is the fluent builder for a Table element.
// Use Container.Table to create one.
//
// Example:
//
//	c.Table(func(t *builder.TableBuilder) {
//	    t.Columns(builder.Fr(1), builder.Fr(2), builder.Pt(80))
//	    t.Header(func(h *builder.TableRowBuilder) {
//	        h.Cell(func(c *builder.CellBuilder) { c.Text("#", builder.Bold()) })
//	        h.Cell(func(c *builder.CellBuilder) { c.Text("Item", builder.Bold()) })
//	        h.Cell(func(c *builder.CellBuilder) { c.Text("Price", builder.Bold()) })
//	    }, builder.RowBg(builder.Navy), builder.CellTextColor(builder.White))
//	    t.Row(func(r *builder.TableRowBuilder) {
//	        r.Cell(func(c *builder.CellBuilder) { c.Text("1") })
//	        r.Cell(func(c *builder.CellBuilder) { c.Text("Widget") })
//	        r.Cell(func(c *builder.CellBuilder) { c.Text("$9.99") })
//	    })
//	})
type TableBuilder struct {
	b       *Builder
	columns []layout.ColumnDef
	header  []tableRowEntry
	body    []tableRowEntry
	footer  []tableRowEntry
}

// tableRowEntry captures a row builder callback along with its resolved config.
type tableRowEntry struct {
	fn  func(*TableRowBuilder)
	cfg tableRowConfig
}

// tableRowConfig holds per-row options resolved from TableRowOption values.
type tableRowConfig struct {
	bgColor   *layout.Color
	textColor *layout.Color // inherited by cells without explicit color
}

// TableRowOption is a functional option that configures a table row.
type TableRowOption func(*tableRowConfig)

// TableRowBg sets the background color for a table row.
func TableRowBg(c Color) TableRowOption {
	lc := c.toLayout()
	return func(cfg *tableRowConfig) {
		cfg.bgColor = &lc
	}
}

// CellTextColor sets the default text color inherited by all cells in a row.
// Individual cells may override this with their own TextOption.
func CellTextColor(c Color) TableRowOption {
	lc := c.toLayout()
	return func(cfg *tableRowConfig) {
		cfg.textColor = &lc
	}
}

// applyTableRowOptions applies a slice of TableRowOption values.
func applyTableRowOptions(opts []TableRowOption) tableRowConfig {
	cfg := tableRowConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// Columns sets the column width definitions for the table.
// Each Value specifies how one column's width is computed:
//   - builder.Fr(1) — fractional share of available space
//   - builder.Pct(30) — 30% of available table width
//   - builder.Pt(80) — exactly 80 PDF points
//   - builder.Mm(25) — 25 millimeters
//   - builder.Auto() — content-driven (measured from cell text)
//
// Example:
//
//	t.Columns(builder.Fr(1), builder.Fr(2), builder.Pt(80))
func (t *TableBuilder) Columns(defs ...Value) {
	t.columns = make([]layout.ColumnDef, len(defs))
	for i, d := range defs {
		t.columns[i] = layout.ColumnDef{Width: d.toLayout()}
	}
}

// Header adds a repeating header row to the table.
// Header rows are rendered at the top of every page when the table overflows.
// Row-level styling (background, text color) is applied via TableRowOption values.
//
// Example:
//
//	t.Header(func(h *builder.TableRowBuilder) {
//	    h.Cell(func(c *builder.CellBuilder) { c.Text("Name", builder.Bold()) })
//	    h.Cell(func(c *builder.CellBuilder) { c.Text("Amount", builder.Bold()) })
//	}, builder.TableRowBg(builder.Navy), builder.CellTextColor(builder.White))
func (t *TableBuilder) Header(fn func(*TableRowBuilder), opts ...TableRowOption) {
	t.header = append(t.header, tableRowEntry{
		fn:  fn,
		cfg: applyTableRowOptions(opts),
	})
}

// Row adds a body row to the table.
// Body rows form the main content and are split across pages on overflow.
//
// Example:
//
//	t.Row(func(r *builder.TableRowBuilder) {
//	    r.Cell(func(c *builder.CellBuilder) { c.Text("Alice") })
//	    r.Cell(func(c *builder.CellBuilder) { c.Text("$42.00") })
//	}, builder.TableRowBg(builder.LightGray))
func (t *TableBuilder) Row(fn func(*TableRowBuilder), opts ...TableRowOption) {
	t.body = append(t.body, tableRowEntry{
		fn:  fn,
		cfg: applyTableRowOptions(opts),
	})
}

// Footer adds a repeating footer row to the table.
// Footer rows are placed at the bottom of every page.
//
// Example:
//
//	t.Footer(func(f *builder.TableRowBuilder) {
//	    f.Cell(func(c *builder.CellBuilder) {
//	        c.Text("Total", builder.Bold(), builder.AlignRight())
//	    }, builder.ColSpan(2))
//	    f.Cell(func(c *builder.CellBuilder) { c.Text("$99.00", builder.Bold()) })
//	})
func (t *TableBuilder) Footer(fn func(*TableRowBuilder), opts ...TableRowOption) {
	t.footer = append(t.footer, tableRowEntry{
		fn:  fn,
		cfg: applyTableRowOptions(opts),
	})
}

// build converts the TableBuilder into a layout.Table element.
func (t *TableBuilder) build() *layout.Table {
	return &layout.Table{
		Columns: t.columns,
		Header:  t.buildRows(t.header),
		Body:    t.buildRows(t.body),
		Footer:  t.buildRows(t.footer),
		Fonts:   t.b.fontResolver(),
	}
}

// buildRows converts a slice of tableRowEntry values into layout.TableRow values.
func (t *TableBuilder) buildRows(entries []tableRowEntry) []layout.TableRow {
	rows := make([]layout.TableRow, 0, len(entries))
	for _, entry := range entries {
		rb := &TableRowBuilder{b: t.b, cfg: entry.cfg}
		entry.fn(rb)
		rows = append(rows, rb.build())
	}
	return rows
}

// --- TableRowBuilder ---

// TableRowBuilder collects cell definitions for a single table row.
type TableRowBuilder struct {
	b     *Builder
	cfg   tableRowConfig
	cells []tableCellEntry
}

// tableCellEntry captures a cell builder callback along with its options.
type tableCellEntry struct {
	fn   func(*CellBuilder)
	opts cellConfig
}

// Cell adds a cell to the row.
// The fn callback receives a CellBuilder (which embeds Container) for adding content.
// Cell-level options (ColSpan, RowSpan, CellAllowBreak, CellRepeatOnBreak,
// CellPadding, CellBg, CellVAlign*, CellBorder) are applied via CellOption values.
//
// Example:
//
//	r.Cell(func(c *builder.CellBuilder) {
//	    c.Text("Total", builder.Bold(), builder.AlignRight())
//	}, builder.ColSpan(3), builder.CellBg(builder.LightGray))
func (r *TableRowBuilder) Cell(fn func(*CellBuilder), opts ...CellOption) {
	cfg := applyCellOptions(opts)
	r.cells = append(r.cells, tableCellEntry{fn: fn, opts: cfg})
}

// build converts the TableRowBuilder into a layout.TableRow.
func (r *TableRowBuilder) build() layout.TableRow {
	cells := make([]layout.TableCell, 0, len(r.cells))
	for _, entry := range r.cells {
		cb := &CellBuilder{Container: Container{b: r.b}}
		entry.fn(cb)

		cellStyle := layout.Style{}
		if entry.opts.padding != nil {
			p := *entry.opts.padding
			cellStyle.Padding = layout.UniformEdges(p)
		}
		if entry.opts.bgColor != nil {
			bg := *entry.opts.bgColor
			cellStyle.Background = &bg
		}
		cellStyle.VerticalAlign = entry.opts.vAlign
		if entry.opts.borderColor != nil {
			bc := *entry.opts.borderColor
			bw := entry.opts.borderWidth
			if bw <= 0 {
				bw = 0.5
			}
			side := layout.BorderSide{Width: bw, Color: bc}
			cellStyle.Border = layout.BorderEdges{
				Top: side, Right: side, Bottom: side, Left: side,
			}
		}

		// Apply row-level text color to cell content elements if the row
		// has a default text color set (e.g. white text on navy header).
		elements := cb.elements
		if r.cfg.textColor != nil {
			elements = applyTextColorToElements(elements, *r.cfg.textColor)
		}

		cell := layout.TableCell{
			Content:       elements,
			ColSpan:       entry.opts.colSpan,
			RowSpan:       entry.opts.rowSpan,
			Style:         cellStyle,
			AllowBreak:    entry.opts.allowBreak,
			RepeatOnBreak: entry.opts.repeatOnBreak,
		}
		cells = append(cells, cell)
	}

	rowStyle := layout.Style{}
	if r.cfg.bgColor != nil {
		bg := *r.cfg.bgColor
		rowStyle.Background = &bg
	}

	return layout.TableRow{
		Cells: cells,
		Style: rowStyle,
	}
}

// applyTextColorToElements returns a copy of elements slice where any
// layout.Text elements that have zero-value (black) Color receive the
// provided default color. Non-text elements are passed through unchanged.
// This implements row-level text color inheritance.
func applyTextColorToElements(elements []layout.Element, color layout.Color) []layout.Element {
	result := make([]layout.Element, len(elements))
	for i, elem := range elements {
		if txt, ok := elem.(*layout.Text); ok {
			if txt.Style.Color == (layout.Color{}) {
				// Clone the text with the inherited color.
				clone := *txt
				clone.Style.Color = color
				result[i] = &clone
				continue
			}
		}
		result[i] = elem
	}
	return result
}

// --- CellBuilder ---

// CellBuilder is a Container that forms the content of a table cell.
// It embeds Container so all content methods (Text, Row, Image, Spacer, etc.)
// are available inside a cell.
type CellBuilder struct {
	Container
}

// --- Cell options ---

// CellOption is a functional option that configures a single table cell.
type CellOption func(*cellConfig)

// cellConfig holds per-cell layout configuration derived from CellOption values.
type cellConfig struct {
	colSpan     int
	rowSpan     int
	padding     *layout.Value
	bgColor     *layout.Color
	vAlign      layout.VAlign
	borderColor *layout.Color
	borderWidth float64

	allowBreak    bool
	repeatOnBreak bool
}

// applyCellOptions applies a slice of CellOption values and returns the resulting cellConfig.
func applyCellOptions(opts []CellOption) cellConfig {
	cfg := cellConfig{colSpan: 1, rowSpan: 1}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// ColSpan makes a cell span the given number of columns. Must be >= 1.
//
// Example:
//
//	r.Cell(func(c *builder.CellBuilder) { c.Text("Total") }, builder.ColSpan(3))
func ColSpan(n int) CellOption {
	return func(cfg *cellConfig) {
		if n < 1 {
			n = 1
		}
		cfg.colSpan = n
	}
}

// RowSpan makes a cell span the given number of rows. Must be >= 1.
// A span never reaches past the end of its section (header, body, footer).
//
// The cell covers the full height of the rows it spans; if its content needs
// more, those rows grow. Rows linked by a span stay on one page: a group that
// does not fit in the rest of the page moves to the next page whole. A group
// taller than a page is broken, and the cell continues in its columns on the
// next page. See CellAllowBreak and CellRepeatOnBreak.
//
// Example:
//
//	r.Cell(func(c *builder.CellBuilder) { c.Text("Merged") }, builder.RowSpan(2))
func RowSpan(n int) CellOption {
	return func(cfg *cellConfig) {
		if n < 1 {
			n = 1
		}
		cfg.rowSpan = n
	}
}

// CellAllowBreak lets a page break fall between the rows a RowSpan cell
// spans, so the rows fill each page instead of moving to the next page
// together. A break inside a group happens only where every cell spanning
// across it allows it. Use it for long data tables where density matters
// more than keeping a group on one page.
//
// Example:
//
//	r.Cell(fn, builder.RowSpan(12), builder.CellAllowBreak())
func CellAllowBreak() CellOption {
	return func(cfg *cellConfig) {
		cfg.allowBreak = true
	}
}

// CellRepeatOnBreak shows a RowSpan cell's content again at the top of every
// page its span continues on, like a repeated group header in a report: a
// team name next to its members, a category next to its items. It applies
// when the content was shown completely before the break; text still
// continuing on the next page is continued, not repeated.
//
// Example:
//
//	r.Cell(func(c *builder.CellBuilder) { c.Text("Team A") },
//	    builder.RowSpan(40), builder.CellRepeatOnBreak())
func CellRepeatOnBreak() CellOption {
	return func(cfg *cellConfig) {
		cfg.repeatOnBreak = true
	}
}

// CellPadding sets uniform padding (all 4 sides) for a cell.
//
// Example:
//
//	r.Cell(fn, builder.CellPadding(builder.Pt(6)))
func CellPadding(v Value) CellOption {
	lv := v.toLayout()
	return func(cfg *cellConfig) {
		cfg.padding = &lv
	}
}

// CellBg sets the background fill color for a single cell.
//
// Example:
//
//	r.Cell(fn, builder.CellBg(builder.LightGray))
func CellBg(c Color) CellOption {
	lc := c.toLayout()
	return func(cfg *cellConfig) {
		cfg.bgColor = &lc
	}
}

// CellVAlign sets the vertical alignment for a cell's content.
// Valid values: layout.VAlignTop (default), layout.VAlignMiddle, layout.VAlignBottom.
//
// Example:
//
//	r.Cell(fn, builder.CellVAlignMiddle())
func CellVAlignMiddle() CellOption {
	return func(cfg *cellConfig) {
		cfg.vAlign = layout.VAlignMiddle
	}
}

// CellVAlignBottom sets bottom vertical alignment for a cell's content.
func CellVAlignBottom() CellOption {
	return func(cfg *cellConfig) {
		cfg.vAlign = layout.VAlignBottom
	}
}

// CellVAlignTop sets top vertical alignment for a cell's content (the default).
func CellVAlignTop() CellOption {
	return func(cfg *cellConfig) {
		cfg.vAlign = layout.VAlignTop
	}
}

// CellBorder sets a uniform border on all four sides of the cell.
//
// Example:
//
//	r.Cell(fn, builder.CellBorder(builder.Gray, 0.5))
func CellBorder(c Color, width float64) CellOption {
	lc := c.toLayout()
	return func(cfg *cellConfig) {
		cfg.borderColor = &lc
		cfg.borderWidth = width
	}
}

// --- Table method on Container ---

// Table adds a table to the container.
// The fn callback receives a TableBuilder for defining columns, header, body and footer rows.
//
// Example:
//
//	c.Table(func(t *builder.TableBuilder) {
//	    t.Columns(builder.Fr(1), builder.Fr(1), builder.Fr(1))
//	    t.Header(func(h *builder.TableRowBuilder) {
//	        h.Cell(func(c *builder.CellBuilder) { c.Text("Name", builder.Bold()) })
//	        h.Cell(func(c *builder.CellBuilder) { c.Text("Value", builder.Bold()) })
//	        h.Cell(func(c *builder.CellBuilder) { c.Text("Notes", builder.Bold()) })
//	    }, builder.TableRowBg(builder.Navy), builder.CellTextColor(builder.White))
//	    t.Row(func(r *builder.TableRowBuilder) {
//	        r.Cell(func(c *builder.CellBuilder) { c.Text("Revenue") })
//	        r.Cell(func(c *builder.CellBuilder) { c.Text("$4.2M") })
//	        r.Cell(func(c *builder.CellBuilder) { c.Text("Q1 2026") })
//	    })
//	})
func (c *Container) Table(fn func(*TableBuilder)) {
	tb := &TableBuilder{b: c.b}
	fn(tb)
	elem := tb.build()
	c.elements = append(c.elements, elem)
}
