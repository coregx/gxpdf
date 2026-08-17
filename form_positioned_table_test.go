package gxpdf

import (
	"path/filepath"
	"testing"
)

// TestFormPositionedGeometryReachesTableDetection verifies the public handoff:
// corrected Form glyph geometry must reach the table detector instead of leaving
// a native-text table undiscoverable. Exact multi-column reconstruction is a
// separate concern and is intentionally not frozen by this regression test.
func TestFormPositionedGeometryReachesTableDetection(t *testing.T) {
	methods := []ExtractionMethod{MethodAuto, MethodStream, MethodLattice, MethodHybrid}
	for _, method := range methods {
		t.Run(method.String(), func(t *testing.T) {
			document, err := Open(filepath.Join("testdata", "pdfs", "form_positioned_table.pdf"))
			if err != nil {
				t.Fatal(err)
			}
			defer document.Close()

			tables, err := document.ExtractTablesWithOptions(DefaultExtractionOptions().WithMethod(method))
			if err != nil {
				t.Fatal(err)
			}
			if len(tables) != 1 {
				t.Fatalf("tables = %d, want 1", len(tables))
			}
			rows := tables[0].Rows()
			for _, label := range []string{"Line Item", "Revenue", "Cost of Sales"} {
				if !tableContainsText(rows, label) {
					t.Errorf("table does not contain %q: %#v", label, rows)
				}
			}
		})
	}
}

func tableContainsText(rows [][]string, want string) bool {
	for _, row := range rows {
		for _, cell := range row {
			if cell == want {
				return true
			}
		}
	}
	return false
}
