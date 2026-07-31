package gxpdf

// Golden / snapshot tests for table extraction.
//
// Each test extracts a table from a real PDF fixture, converts it to a
// GoldenTable value, and compares it cell-by-cell against a committed
// JSON file in testdata/golden/.
//
// Rationale:
//   - Table extraction involves many interdependent subsystems (graphics
//     parser, ruling-line detector, merged-cell logic). A subtle regression
//     in any layer can silently shift cell boundaries or span counts.
//   - Golden tests catch those regressions immediately without requiring
//     manual visual inspection of every cell.
//
// Updating golden files:
//
//	go test -run TestGolden -update-golden
//
// When to update:
//   - An intentional algorithm change alters extraction output.
//   - A new PDF fixture is added.
//   - Never update silently — always review the diff before committing.
//
// Golden files are committed test fixtures (NOT generated output).
// They live in testdata/golden/ and must be kept in sync with the PDFs.

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// updateGolden is set with -update-golden to regenerate golden files.
var updateGolden = flag.Bool("update-golden", false, "regenerate golden test files instead of comparing")

// GoldenTable is the serialisable snapshot of a single extracted table.
// Only cells that carry information (non-empty text, or a span > 1) are
// included to keep the JSON file compact and human-readable.
type GoldenTable struct {
	PDF    string       `json:"pdf"`
	Page   int          `json:"page"`
	Method string       `json:"method"`
	Rows   int          `json:"rows"`
	Cols   int          `json:"cols"`
	Cells  []GoldenCell `json:"cells"`
}

// GoldenCell captures the text content and merge metadata of one cell.
type GoldenCell struct {
	Row     int    `json:"row"`
	Col     int    `json:"col"`
	Text    string `json:"text"`
	RowSpan int    `json:"rowSpan"`
	ColSpan int    `json:"colSpan"`
}

// goldenPath returns the canonical path for a golden file given a
// human-friendly test name.
func goldenPath(name string) string {
	safe := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ' ' || r == ':' {
			return '_'
		}
		return r
	}, name)
	return filepath.Join("testdata", "golden", safe+".json")
}

// tableToGolden converts a *Table to a GoldenTable, skipping cells that
// contain no text AND have no merge spans (rowSpan == 1 && colSpan == 1).
// Those trivially-empty cells add noise without aiding regression detection.
func tableToGolden(pdfPath string, pageNum int, tbl *Table) GoldenTable {
	g := GoldenTable{
		PDF:    pdfPath,
		Page:   pageNum,
		Method: tbl.Method(),
		Rows:   tbl.RowCount(),
		Cols:   tbl.ColumnCount(),
	}

	for r := 0; r < tbl.RowCount(); r++ {
		for c := 0; c < tbl.ColumnCount(); c++ {
			info := tbl.CellAt(r, c)
			if info == nil {
				continue
			}
			// Skip trivially empty, unmerged cells.
			if info.Text == "" && info.RowSpan == 1 && info.ColSpan == 1 {
				continue
			}
			g.Cells = append(g.Cells, GoldenCell{
				Row:     info.Row,
				Col:     info.Col,
				Text:    info.Text,
				RowSpan: info.RowSpan,
				ColSpan: info.ColSpan,
			})
		}
	}

	return g
}

// writeGolden serializes gt to path using indented JSON.
func writeGolden(path string, gt GoldenTable) error {
	data, err := json.MarshalIndent(gt, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal golden: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write golden %s: %w", path, err)
	}
	return nil
}

// readGolden deserializes the golden file at path.
func readGolden(path string) (GoldenTable, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return GoldenTable{}, fmt.Errorf("read golden %s: %w", path, err)
	}
	var gt GoldenTable
	if err := json.Unmarshal(data, &gt); err != nil {
		return GoldenTable{}, fmt.Errorf("unmarshal golden %s: %w", path, err)
	}
	return gt, nil
}

// compareGolden returns a human-readable diff between want and got.
// It reports structural mismatches (rows/cols/method) and then lists
// every cell that differs, both cells that changed and cells that
// appeared or disappeared.
func compareGolden(t *testing.T, want, got GoldenTable) {
	t.Helper()

	failed := false

	if want.Method != got.Method {
		t.Errorf("Method: want %q, got %q", want.Method, got.Method)
		failed = true
	}
	if want.Rows != got.Rows {
		t.Errorf("Rows: want %d, got %d", want.Rows, got.Rows)
		failed = true
	}
	if want.Cols != got.Cols {
		t.Errorf("Cols: want %d, got %d", want.Cols, got.Cols)
		failed = true
	}

	// Build maps keyed by (row, col) for O(1) lookup.
	type key struct{ row, col int }
	wantMap := make(map[key]GoldenCell, len(want.Cells))
	for _, c := range want.Cells {
		wantMap[key{c.Row, c.Col}] = c
	}
	gotMap := make(map[key]GoldenCell, len(got.Cells))
	for _, c := range got.Cells {
		gotMap[key{c.Row, c.Col}] = c
	}

	// Cells present in want — check if they match got.
	for k, wc := range wantMap {
		gc, ok := gotMap[k]
		if !ok {
			t.Errorf("  MISSING  cell [%d,%d]: want text=%q rowSpan=%d colSpan=%d",
				k.row, k.col, wc.Text, wc.RowSpan, wc.ColSpan)
			failed = true
			continue
		}
		if wc.Text != gc.Text || wc.RowSpan != gc.RowSpan || wc.ColSpan != gc.ColSpan {
			t.Errorf("  CHANGED  cell [%d,%d]:\n    want text=%-30q rowSpan=%d colSpan=%d\n    got  text=%-30q rowSpan=%d colSpan=%d",
				k.row, k.col,
				wc.Text, wc.RowSpan, wc.ColSpan,
				gc.Text, gc.RowSpan, gc.ColSpan)
			failed = true
		}
	}

	// Cells present in got but absent in want — unexpected additions.
	for k, gc := range gotMap {
		if _, ok := wantMap[k]; !ok {
			t.Errorf("  ADDED    cell [%d,%d]: got  text=%q rowSpan=%d colSpan=%d",
				k.row, k.col, gc.Text, gc.RowSpan, gc.ColSpan)
			failed = true
		}
	}

	if failed {
		t.Log("Tip: run `go test -run TestGolden -update-golden` to regenerate golden files after an intentional change")
	}
}

// runGoldenTest is the shared driver for all golden tests.
//
// It opens pdfPath, extracts tables using opts, picks table at tableIndex,
// and either writes a new golden file (when -update-golden is set) or
// compares the current extraction against the committed golden.
func runGoldenTest(t *testing.T, name, pdfPath string, pageNum, tableIndex int, opts *ExtractionOptions) {
	t.Helper()

	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Skipf("fixture not found: %s", pdfPath)
	}

	doc, err := Open(pdfPath)
	if err != nil {
		t.Skipf("cannot open %s: %v", pdfPath, err)
	}
	defer doc.Close()

	tables, err := doc.ExtractTablesWithOptions(opts)
	if err != nil {
		t.Fatalf("ExtractTablesWithOptions: %v", err)
	}

	if tableIndex >= len(tables) {
		t.Skipf("tableIndex %d out of range: PDF returned %d table(s)", tableIndex, len(tables))
	}

	tbl := tables[tableIndex]
	got := tableToGolden(pdfPath, pageNum, tbl)
	path := goldenPath(name)

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}
		if err := writeGolden(path, got); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("golden updated: %s (%d rows × %d cols, %d significant cells)",
			path, got.Rows, got.Cols, len(got.Cells))
		return
	}

	want, err := readGolden(path)
	if err != nil {
		t.Fatalf("golden file missing — run `go test -run TestGolden -update-golden` to create it: %v", err)
	}

	compareGolden(t, want, got)
}

// ---------- Golden test cases ----------

// TestGolden_Issue79_Page0_Table0 snapshots the first table on page 0 of the
// issue-79 sample PDF. This PDF uses filled rectangles for borders (wkhtmltopdf
// / Chrome-print style) and was the primary fixture for the Lattice detection
// fix. The snapshot captures the 4-column schedule grid with merged day cells.
func TestGolden_Issue79_Page0_Table0(t *testing.T) {
	opts := DefaultExtractionOptions().
		WithMethod(MethodLattice).
		WithPages(0)

	runGoldenTest(t,
		"issue79_page0_table0",
		"testdata/pdfs/issue79/sample.pdf",
		0, // pageNum for metadata
		0, // tableIndex
		opts,
	)
}

// TestGolden_Issue79_Page1_Table0 snapshots the first table on page 1 of the
// issue-79 sample PDF. Page 1 may use a different layout from page 0; snapshotting
// it independently ensures multi-page extraction regressions are caught.
func TestGolden_Issue79_Page1_Table0(t *testing.T) {
	opts := DefaultExtractionOptions().
		WithMethod(MethodLattice).
		WithPages(1)

	runGoldenTest(t,
		"issue79_page1_table0",
		"testdata/pdfs/issue79/sample.pdf",
		1, // pageNum for metadata
		0, // tableIndex
		opts,
	)
}

// TestGolden_Sample_Page0_Stream snapshots the first table on page 0 of the
// general sample.pdf using Stream mode. Stream mode uses whitespace analysis
// rather than ruling lines; snapshotting it separately prevents regressions
// in the whitespace analyser from being masked by Lattice results.
func TestGolden_Sample_Page0_Stream(t *testing.T) {
	opts := DefaultExtractionOptions().
		WithMethod(MethodStream).
		WithPages(0)

	runGoldenTest(t,
		"sample_page0_stream_table0",
		"testdata/pdfs/sample.pdf",
		0, // pageNum for metadata
		0, // tableIndex
		opts,
	)
}

// TestGolden_Issue79_AutoMode snapshots the first table from the issue-79
// PDF in Auto mode. Auto mode should select Lattice for this PDF; the golden
// file therefore double-checks both the method-selection logic and the
// extraction result in one test.
func TestGolden_Issue79_AutoMode(t *testing.T) {
	opts := DefaultExtractionOptions().
		WithMethod(MethodAuto).
		WithPages(0)

	runGoldenTest(t,
		"issue79_page0_auto_table0",
		"testdata/pdfs/issue79/sample.pdf",
		0, // pageNum for metadata
		0, // tableIndex
		opts,
	)
}
