package gxpdf

import (
	"path/filepath"
	"reflect"
	"testing"
)

// TestFormPositionedGeometryReachesTableDetection verifies the complete public
// handoff from corrected Form glyph geometry through table reconstruction.
func TestFormPositionedGeometryReachesTableDetection(t *testing.T) {
	want := [][]string{
		{"Income Statement", "", ""},
		{"Line Item", "2023", "2024"},
		{"Revenue", "1200", "1450"},
		{"Cost of Sales", "(400)", "(570)"},
	}
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
			if got := tables[0].Rows(); !reflect.DeepEqual(got, want) {
				t.Errorf("table rows = %#v, want %#v", got, want)
			}
		})
	}
}

func TestFormPositionedTableIgnoresFarRightPageText(t *testing.T) {
	document, err := Open(filepath.Join("testdata", "pdfs", "form_positioned_table_outlier.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer document.Close()

	tables, err := document.ExtractTablesWithOptions(DefaultExtractionOptions().WithMethod(MethodStream))
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(tables))
	}
	if got := tables[0].ColumnCount(); got != 3 {
		t.Fatalf("columns = %d, want 3", got)
	}
	rows := tables[0].Rows()
	want := [][]string{
		{"Income Statement", "", ""},
		{"Line Item", "2023", "2024"},
		{"Revenue", "1200", "1450"},
		{"Cost of Sales", "(400)", "(570)"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("table rows = %#v, want %#v", rows, want)
	}
}

func TestFormPositionedTableBoundaryCases(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    [][]string
	}{
		{
			name:    "nearby footer row",
			fixture: "form_positioned_table_near_footer.pdf",
			want: [][]string{
				{"Income Statement", "", ""},
				{"Line Item", "2023", "2024"},
				{"Revenue", "1200", "1450"},
				{"Cost of Sales", "(400)", "(570)"},
			},
		},
		{
			name:    "sparse value after section gap",
			fixture: "form_positioned_table_sparse_section.pdf",
			want: [][]string{
				{"Income Statement", "", ""},
				{"Line Item", "2023", ""},
				{"Revenue", "1200", ""},
				{"Cost of Sales", "(400)", ""},
				{"Other", "", "900"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertFormPositionedTableRows(t, test.fixture, test.want)
		})
	}
}

func assertFormPositionedTableRows(t *testing.T, fixture string, want [][]string) {
	t.Helper()
	document, err := Open(filepath.Join("testdata", "pdfs", fixture))
	if err != nil {
		t.Fatal(err)
	}
	defer document.Close()

	tables, err := document.ExtractTablesWithOptions(DefaultExtractionOptions().WithMethod(MethodStream))
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(tables))
	}
	if got := tables[0].Rows(); !reflect.DeepEqual(got, want) {
		t.Fatalf("table rows = %#v, want %#v", got, want)
	}
}
