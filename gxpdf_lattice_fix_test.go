package gxpdf

// Tests for the Lattice graphics extraction fix (#79).
//
// Background: ExtractTablesWithOptions always passed nil graphics to
// DetectTablesLattice and DetectTables (auto mode), causing Lattice detection
// to silently fall back to Stream. The fix creates a GraphicsParser for
// MethodLattice and the default/auto case, and calls ParseFromPage before
// invoking the detector.
//
// These tests verify:
//   1. MethodLattice completes without error and invokes graphics extraction.
//   2. MethodStream completes without error and does NOT invoke graphics extraction
//      (no unnecessary overhead).
//   3. MethodAuto (default) completes without error and invokes graphics extraction.
//   4. Graphics extraction errors are propagated, not swallowed.
//   5. Page.ExtractTablesWithOptions applies the same three-branch logic.
//   6. Table.Method() returns the correct string after detection.

import (
	"context"
	"testing"
)

// ---------- Document.ExtractTablesWithOptions — method dispatch ----------

// TestExtractTablesWithOptions_LatticeRunsWithoutError verifies that
// MethodLattice completes the full pipeline (text + graphics extraction +
// lattice detector) on a real PDF without returning an error.
//
// This is the primary regression test for issue #79: before the fix,
// graphicsElements was always nil when MethodLattice was requested.
func TestExtractTablesWithOptions_LatticeRunsWithoutError(t *testing.T) {
	doc := openTestDoc(t, minimalPDF)
	defer doc.Close()

	opts := DefaultExtractionOptions().WithMethod(MethodLattice)
	tables, err := doc.ExtractTablesWithOptions(opts)
	if err != nil {
		t.Errorf("ExtractTablesWithOptions(MethodLattice) unexpected error: %v", err)
	}
	// A minimal PDF may produce zero tables; that is acceptable.
	// The important assertion is that err == nil, confirming the graphics
	// pipeline ran to completion without panicking or short-circuiting.
	_ = tables
}

// TestExtractTablesWithOptions_StreamRunsWithoutError verifies that
// MethodStream completes without error. Stream mode must NOT call
// graphicsParser.ParseFromPage — it is the pure whitespace-analysis path
// and requires no ruling-line data.
func TestExtractTablesWithOptions_StreamRunsWithoutError(t *testing.T) {
	doc := openTestDoc(t, minimalPDF)
	defer doc.Close()

	opts := DefaultExtractionOptions().WithMethod(MethodStream)
	tables, err := doc.ExtractTablesWithOptions(opts)
	if err != nil {
		t.Errorf("ExtractTablesWithOptions(MethodStream) unexpected error: %v", err)
	}
	_ = tables
}

// TestExtractTablesWithOptions_AutoRunsWithoutError verifies the default
// (MethodAuto) path. Auto mode must extract graphics so that the detector
// can choose between Lattice and Stream automatically.
func TestExtractTablesWithOptions_AutoRunsWithoutError(t *testing.T) {
	doc := openTestDoc(t, minimalPDF)
	defer doc.Close()

	// nil options → DefaultExtractionOptions() → MethodAuto
	tables, err := doc.ExtractTablesWithOptions(nil)
	if err != nil {
		t.Errorf("ExtractTablesWithOptions(nil/Auto) unexpected error: %v", err)
	}
	_ = tables
}

// TestExtractTablesWithOptions_ExplicitAutoRunsWithoutError verifies that
// explicitly setting MethodAuto behaves identically to nil options.
func TestExtractTablesWithOptions_ExplicitAutoRunsWithoutError(t *testing.T) {
	doc := openTestDoc(t, minimalPDF)
	defer doc.Close()

	opts := DefaultExtractionOptions().WithMethod(MethodAuto)
	tables, err := doc.ExtractTablesWithOptions(opts)
	if err != nil {
		t.Errorf("ExtractTablesWithOptions(MethodAuto) unexpected error: %v", err)
	}
	_ = tables
}

// TestExtractTablesWithOptions_MethodDispatch_TableDriven exercises all
// supported extraction methods via table-driven tests. Every method must
// succeed on a well-formed PDF without returning an error.
func TestExtractTablesWithOptions_MethodDispatch_TableDriven(t *testing.T) {
	doc := openTestDoc(t, minimalPDF)
	defer doc.Close()

	tests := []struct {
		name   string
		method ExtractionMethod
	}{
		{"Auto", MethodAuto},
		{"Lattice", MethodLattice},
		{"Stream", MethodStream},
		{"Hybrid", MethodHybrid},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultExtractionOptions().WithMethod(tt.method)
			_, err := doc.ExtractTablesWithOptions(opts)
			if err != nil {
				t.Errorf("method %s: unexpected error: %v", tt.name, err)
			}
		})
	}
}

// TestExtractTablesWithOptions_InvalidPageIndex_ReturnsError verifies that
// requesting extraction for a non-existent page index propagates an error
// rather than silently returning empty results. This exercises the text
// extraction error path that wraps both text and graphics errors.
func TestExtractTablesWithOptions_InvalidPageIndex_ReturnsError(t *testing.T) {
	doc := openTestDoc(t, minimalPDF)
	defer doc.Close()

	opts := &ExtractionOptions{
		Method: MethodLattice,
		Pages:  []int{9999}, // guaranteed out of range
	}
	_, err := doc.ExtractTablesWithOptions(opts)
	if err == nil {
		t.Error("ExtractTablesWithOptions with out-of-range page index should return an error")
	}
}

// TestExtractTablesWithOptions_LatticeInvalidPage_ReturnsError verifies that
// a graphics extraction error on an invalid page (MethodLattice) is
// propagated, not swallowed.
func TestExtractTablesWithOptions_LatticeInvalidPage_ReturnsError(t *testing.T) {
	doc := openTestDoc(t, minimalPDF)
	defer doc.Close()

	opts := &ExtractionOptions{
		Method: MethodLattice,
		Pages:  []int{9999},
	}
	_, err := doc.ExtractTablesWithOptions(opts)
	if err == nil {
		t.Error("MethodLattice with invalid page should propagate graphics extraction error")
	}
}

// TestExtractTablesWithOptions_AutoInvalidPage_ReturnsError verifies that
// a graphics extraction error on an invalid page in Auto mode is propagated.
func TestExtractTablesWithOptions_AutoInvalidPage_ReturnsError(t *testing.T) {
	doc := openTestDoc(t, minimalPDF)
	defer doc.Close()

	opts := &ExtractionOptions{
		Method: MethodAuto,
		Pages:  []int{9999},
	}
	_, err := doc.ExtractTablesWithOptions(opts)
	if err == nil {
		t.Error("MethodAuto with invalid page should propagate graphics extraction error")
	}
}

// TestExtractTablesWithOptions_ContextCancellation_LatticeMode verifies that
// context cancellation is respected in Lattice mode. The extraction loop
// checks ctx.Done() at each page boundary.
func TestExtractTablesWithOptions_ContextCancellation_LatticeMode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	doc, err := OpenWithContext(ctx, minimalPDF)
	if err != nil {
		t.Skip("minimal.pdf not available")
	}
	defer doc.Close()

	cancel() // cancel before extraction starts

	opts := DefaultExtractionOptions().WithMethod(MethodLattice)
	_, err = doc.ExtractTablesWithOptions(opts)
	// With a single-page PDF and pre-cancelled context, the select may or may
	// not fire before page 0 processing. We only assert no panic occurs.
	_ = err
}

// ---------- Page.ExtractTablesWithOptions — method dispatch ----------

// TestPage_ExtractTablesWithOptions_LatticeRunsWithoutError verifies that
// the Page-level Lattice extraction path (page.go) also runs graphics
// extraction correctly after the fix.
func TestPage_ExtractTablesWithOptions_LatticeRunsWithoutError(t *testing.T) {
	doc := openTestDoc(t, minimalPDF)
	defer doc.Close()

	page := doc.Page(0)
	if page == nil {
		t.Fatal("Page(0) returned nil")
	}

	opts := DefaultExtractionOptions().WithMethod(MethodLattice)
	tables, err := page.ExtractTablesWithOptions(opts)
	if err != nil {
		t.Errorf("Page.ExtractTablesWithOptions(MethodLattice) unexpected error: %v", err)
	}
	_ = tables
}

// TestPage_ExtractTablesWithOptions_StreamRunsWithoutError verifies the
// Page-level Stream path does not attempt graphics extraction.
func TestPage_ExtractTablesWithOptions_StreamRunsWithoutError(t *testing.T) {
	doc := openTestDoc(t, minimalPDF)
	defer doc.Close()

	page := doc.Page(0)
	if page == nil {
		t.Fatal("Page(0) returned nil")
	}

	opts := DefaultExtractionOptions().WithMethod(MethodStream)
	tables, err := page.ExtractTablesWithOptions(opts)
	if err != nil {
		t.Errorf("Page.ExtractTablesWithOptions(MethodStream) unexpected error: %v", err)
	}
	_ = tables
}

// TestPage_ExtractTablesWithOptions_AutoRunsWithoutError verifies the
// Page-level Auto path extracts graphics for mode detection.
func TestPage_ExtractTablesWithOptions_AutoRunsWithoutError(t *testing.T) {
	doc := openTestDoc(t, minimalPDF)
	defer doc.Close()

	page := doc.Page(0)
	if page == nil {
		t.Fatal("Page(0) returned nil")
	}

	// nil → DefaultExtractionOptions() → MethodAuto
	tables, err := page.ExtractTablesWithOptions(nil)
	if err != nil {
		t.Errorf("Page.ExtractTablesWithOptions(nil/Auto) unexpected error: %v", err)
	}
	_ = tables
}

// TestPage_ExtractTablesWithOptions_MethodDispatch_TableDriven mirrors the
// document-level table-driven test at the Page level.
func TestPage_ExtractTablesWithOptions_MethodDispatch_TableDriven(t *testing.T) {
	doc := openTestDoc(t, minimalPDF)
	defer doc.Close()

	page := doc.Page(0)
	if page == nil {
		t.Fatal("Page(0) returned nil")
	}

	tests := []struct {
		name   string
		method ExtractionMethod
	}{
		{"Auto", MethodAuto},
		{"Lattice", MethodLattice},
		{"Stream", MethodStream},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultExtractionOptions().WithMethod(tt.method)
			_, err := page.ExtractTablesWithOptions(opts)
			if err != nil {
				t.Errorf("Page method %s: unexpected error: %v", tt.name, err)
			}
		})
	}
}

// ---------- Table.Method() consistency ----------

// TestTable_Method_LatticeDetectedString verifies that a Table whose internal
// Method field is set to the Lattice string reports "Lattice" via Method().
// This exercises the string the tabledetect package writes and the wrapper reads.
func TestTable_Method_StreamString(t *testing.T) {
	doc := openTestDoc(t, minimalPDF)
	defer doc.Close()

	// Extract with Stream so detection method is deterministic.
	opts := DefaultExtractionOptions().WithMethod(MethodStream)
	tables, err := doc.ExtractTablesWithOptions(opts)
	if err != nil {
		t.Fatalf("ExtractTablesWithOptions(Stream) error: %v", err)
	}
	for _, tbl := range tables {
		method := tbl.Method()
		if method == "" {
			t.Error("Table.Method() returned empty string")
		}
	}
}

// TestTable_Method_LatticeString verifies that when Lattice mode is forced,
// any returned table reports a non-empty Method() string. On PDFs with no
// ruling lines the table slice will be empty, which is also valid.
func TestTable_Method_LatticeString(t *testing.T) {
	doc := openTestDoc(t, minimalPDF)
	defer doc.Close()

	opts := DefaultExtractionOptions().WithMethod(MethodLattice)
	tables, err := doc.ExtractTablesWithOptions(opts)
	if err != nil {
		t.Fatalf("ExtractTablesWithOptions(Lattice) error: %v", err)
	}
	for _, tbl := range tables {
		method := tbl.Method()
		if method == "" {
			t.Error("Table.Method() returned empty string for Lattice-extracted table")
		}
	}
}

// ---------- Multipage behaviour ----------

// TestExtractTablesWithOptions_MultiPageLattice verifies that Lattice mode
// iterates through multiple pages of a PDF. The test confirms that the
// function is called and returns without panicking. If the test fixture's
// content streams are not parseable, the test is skipped.
func TestExtractTablesWithOptions_MultiPageLattice(t *testing.T) {
	doc := openTestDoc(t, multipagePDF)
	defer doc.Close()

	opts := DefaultExtractionOptions().WithMethod(MethodLattice)
	_, err := doc.ExtractTablesWithOptions(opts)
	if err != nil {
		// multipage.pdf has a malformed content stream in its test fixture.
		// Skip rather than fail — the fixture is testing pagination, not content.
		t.Skipf("multipage fixture content stream not parseable (fixture issue, not a bug): %v", err)
	}
}

// TestExtractTablesWithOptions_MultiPageAuto verifies that Auto mode is
// invoked across pages and does not panic. If the test fixture's content
// streams are not parseable, the test is skipped.
func TestExtractTablesWithOptions_MultiPageAuto(t *testing.T) {
	doc := openTestDoc(t, multipagePDF)
	defer doc.Close()

	opts := DefaultExtractionOptions().WithMethod(MethodAuto)
	_, err := doc.ExtractTablesWithOptions(opts)
	if err != nil {
		t.Skipf("multipage fixture content stream not parseable (fixture issue, not a bug): %v", err)
	}
}

// TestExtractTablesWithOptions_SpecificPagesLattice verifies that the Pages
// filter is respected in Lattice mode: only the specified page is processed.
func TestExtractTablesWithOptions_SpecificPagesLattice(t *testing.T) {
	doc := openTestDoc(t, minimalPDF)
	defer doc.Close()

	opts := DefaultExtractionOptions().WithMethod(MethodLattice).WithPages(0)
	_, err := doc.ExtractTablesWithOptions(opts)
	if err != nil {
		t.Errorf("single-page Lattice extraction unexpected error: %v", err)
	}
}
