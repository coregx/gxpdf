package builder

import (
	"fmt"
	"strings"
	"testing"
)

// TestTable_RowSpanContentNotDroppedByRounding guards the layout fit
// tolerance: the rows a spanning cell grows can sum to a few ulps less than
// the height its content measured (7 rows grown to 700/3pt sum to
// 233.3333333333333, the image needs 233.33333333333334). Content compared
// without tolerance was dropped.
func TestTable_RowSpanContentNotDroppedByRounding(t *testing.T) {
	const rows = 7
	imgH := 700.0 / 3

	doc := NewBuilder(WithPageSize(A3))
	doc.Page(func(p *PageBuilder) {
		p.Content(func(c *Container) {
			c.Table(func(tb *TableBuilder) {
				tb.Columns(Fr(1), Fr(1))
				for i := 0; i < rows; i++ {
					tb.Row(func(r *TableRowBuilder) {
						if i == 0 {
							r.Cell(func(cb *CellBuilder) {
								cb.Image([]byte{1}, FitHeight(Pt(imgH)), FitWidth(Pt(50)))
							}, RowSpan(rows), CellPadding(Pt(0)))
						}
						r.Cell(func(cb *CellBuilder) { cb.Text(fmt.Sprintf("row %d", i)) })
					})
				}
			})
		})
	})

	ops := recordDrawOps(t, doc)
	if !strings.Contains(ops, "image ") {
		t.Fatalf("image inside the spanning cell was dropped:\n%s", ops)
	}
}
