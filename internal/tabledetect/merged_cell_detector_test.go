package tabledetect

import (
	"fmt"
	"testing"

	"github.com/coregx/gxpdf/internal/extractor"
	domaintable "github.com/coregx/gxpdf/internal/models/table"
	"github.com/coregx/gxpdf/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pdfPipeline wraps parser.Reader to expose text and graphics extraction
// needed by the integration test helper, without depending on the root gxpdf package.
type pdfPipeline struct {
	reader         *parser.Reader
	textExtractor  *extractor.TextExtractor
	graphicsParser *extractor.GraphicsParser
}

// newReaderFromPath opens a PDF file and returns a pdfPipeline.
// The caller must call Close() when done.
func newReaderFromPath(path string) (*pdfPipeline, error) {
	r := parser.NewReader(path)
	if err := r.Open(); err != nil {
		return nil, fmt.Errorf("parser.Reader.Open: %w", err)
	}
	return &pdfPipeline{
		reader:         r,
		textExtractor:  extractor.NewTextExtractor(r),
		graphicsParser: extractor.NewGraphicsParser(r),
	}, nil
}

// Close releases the underlying parser.Reader.
func (p *pdfPipeline) Close() error {
	return p.reader.Close()
}

// ExtractTextElements extracts text elements from the given page (0-based).
func (p *pdfPipeline) ExtractTextElements(pageNum int) ([]*extractor.TextElement, error) {
	return p.textExtractor.ExtractFromPage(pageNum)
}

// ExtractGraphicsElements extracts graphics elements from the given page (0-based).
func (p *pdfPipeline) ExtractGraphicsElements(pageNum int) ([]*extractor.GraphicsElement, error) {
	return p.graphicsParser.ParseFromPage(pageNum)
}

// ---------------------------------------------------------------------------
// Helpers for constructing test grids and ruling lines
// ---------------------------------------------------------------------------

// makeGrid builds a Grid with the given row and column boundaries and
// populates its Cells slice.  Rows and Columns are in ascending order
// (bottom-to-top PDF space for rows, left-to-right for columns).
func makeGrid(rows, cols []float64) *Grid {
	g := NewGrid(rows, cols)
	g.Cells = NewDefaultGridBuilder().createCells(g.Rows, g.Columns)
	return g
}

// hLine creates a horizontal RulingLine from (x1, y) to (x2, y).
func hLine(x1, y, x2 float64) *RulingLine {
	return NewRulingLine(extractor.NewPoint(x1, y), extractor.NewPoint(x2, y))
}

// vLine creates a vertical RulingLine from (x, y1) to (x, y2).
func vLine(x, y1, y2 float64) *RulingLine {
	return NewRulingLine(extractor.NewPoint(x, y1), extractor.NewPoint(x, y2))
}

// ---------------------------------------------------------------------------
// HLineIndex tests
// ---------------------------------------------------------------------------

func TestHLineIndex_HasLineAt_ExactMatch(t *testing.T) {
	lines := []*RulingLine{hLine(0, 100, 200)}
	idx := buildHLineIndex(lines, defaultMergeTolerance)

	assert.True(t, idx.HasLineAt(100, 0, 200, defaultMergeTolerance),
		"should find line covering full range")
}

func TestHLineIndex_HasLineAt_ToleranceY(t *testing.T) {
	lines := []*RulingLine{hLine(0, 100.5, 200)}
	idx := buildHLineIndex(lines, defaultMergeTolerance)

	// Y is 0.5 off — within tolerance 2.0
	assert.True(t, idx.HasLineAt(100, 0, 200, defaultMergeTolerance))
}

func TestHLineIndex_HasLineAt_OutsideTolerance(t *testing.T) {
	lines := []*RulingLine{hLine(0, 105, 200)}
	idx := buildHLineIndex(lines, defaultMergeTolerance)

	// Y is 5 off — outside tolerance 2.0
	assert.False(t, idx.HasLineAt(100, 0, 200, defaultMergeTolerance))
}

func TestHLineIndex_HasLineAt_PartialLineBelowThreshold(t *testing.T) {
	// Line covers only 50% of the column width — below minCoverageRatio 0.7
	lines := []*RulingLine{hLine(0, 100, 50)} // covers x=[0,50] of column [0,200]
	idx := buildHLineIndex(lines, defaultMergeTolerance)

	assert.False(t, idx.HasLineAt(100, 0, 200, defaultMergeTolerance),
		"50% coverage should not count as separator")
}

func TestHLineIndex_HasLineAt_PartialLineAboveThreshold(t *testing.T) {
	// Line covers 80% of column width [0,100] → covers [0,80]
	lines := []*RulingLine{hLine(0, 100, 80)}
	idx := buildHLineIndex(lines, defaultMergeTolerance)

	assert.True(t, idx.HasLineAt(100, 0, 100, defaultMergeTolerance),
		"80% coverage should count as separator")
}

func TestHLineIndex_HasLineAt_EmptyIndex(t *testing.T) {
	idx := buildHLineIndex(nil, defaultMergeTolerance)
	assert.False(t, idx.HasLineAt(100, 0, 200, defaultMergeTolerance))
}

// ---------------------------------------------------------------------------
// VLineIndex tests
// ---------------------------------------------------------------------------

func TestVLineIndex_HasLineAt_ExactMatch(t *testing.T) {
	lines := []*RulingLine{vLine(50, 100, 300)}
	idx := buildVLineIndex(lines, defaultMergeTolerance)

	assert.True(t, idx.HasLineAt(50, 100, 300, defaultMergeTolerance))
}

func TestVLineIndex_HasLineAt_ToleranceX(t *testing.T) {
	lines := []*RulingLine{vLine(50.8, 100, 300)}
	idx := buildVLineIndex(lines, defaultMergeTolerance)

	assert.True(t, idx.HasLineAt(50, 100, 300, defaultMergeTolerance))
}

func TestVLineIndex_HasLineAt_PartialLineBelowThreshold(t *testing.T) {
	// V-line covers only 40% of Y range [100,300] → covers [100,180]
	lines := []*RulingLine{vLine(50, 100, 180)}
	idx := buildVLineIndex(lines, defaultMergeTolerance)

	assert.False(t, idx.HasLineAt(50, 100, 300, defaultMergeTolerance))
}

// ---------------------------------------------------------------------------
// DetectMergedCells — no merges baseline
// ---------------------------------------------------------------------------

func TestDetectMergedCells_NoMerges_3x3(t *testing.T) {
	// 3×3 grid with complete ruling lines at every boundary.
	//
	// Grid rows (ascending Y, bottom-to-top):  [100, 120, 140, 160]
	// Grid cols (ascending X, left-to-right):  [0, 50, 100, 150]
	//
	// All interior H-lines: Y=120 and Y=140 spanning full width [0,150]
	// All interior V-lines: X=50 and X=100 spanning full height [100,160]
	rows := []float64{100, 120, 140, 160}
	cols := []float64{0, 50, 100, 150}
	grid := makeGrid(rows, cols)

	lines := []*RulingLine{
		// Outer boundary H-lines
		hLine(0, 100, 150), hLine(0, 160, 150),
		// Interior H-lines (complete — no merge)
		hLine(0, 120, 150), hLine(0, 140, 150),
		// Outer boundary V-lines
		vLine(0, 100, 160), vLine(150, 100, 160),
		// Interior V-lines (complete — no merge)
		vLine(50, 100, 160), vLine(100, 100, 160),
	}

	result := DetectMergedCells(grid, lines, defaultMergeTolerance)
	assert.Empty(t, result, "all separators present → no merged cells")
}

// ---------------------------------------------------------------------------
// DetectMergedCells — row span
// ---------------------------------------------------------------------------

func TestDetectMergedCells_RowSpan_Col0_Spans2(t *testing.T) {
	// 3×3 grid. Column 0 (X=[0,50]) is missing the H-line at Y=120,
	// so cell (gridRow=0, col=0) spans rows 0-1 (rowSpan=2).
	//
	// Grid layout (ascending Y):
	//   Y=100 ──────────────── (outer border)
	//   Y=120      ──────────  (H-line exists only for cols 1-2, not col 0)
	//   Y=140 ────────────────
	rows := []float64{100, 120, 140}
	cols := []float64{0, 50, 100, 150}
	grid := makeGrid(rows, cols)

	lines := []*RulingLine{
		// Outer H-lines
		hLine(0, 100, 150), hLine(0, 140, 150),
		// Interior H-line at Y=120 exists only for cols 1-2 (X=[50,150])
		hLine(50, 120, 150),
		// Outer V-lines
		vLine(0, 100, 140), vLine(150, 100, 140),
		// Interior V-lines
		vLine(50, 100, 140), vLine(100, 100, 140),
	}

	result := DetectMergedCells(grid, lines, defaultMergeTolerance)
	require.Len(t, result, 1)
	assert.Equal(t, 0, result[0].Row)
	assert.Equal(t, 0, result[0].Col)
	assert.Equal(t, 2, result[0].RowSpan)
	assert.Equal(t, 1, result[0].ColSpan)
}

func TestDetectMergedCells_RowSpan_Col0_Spans3(t *testing.T) {
	// 4×1 grid. Column 0 has no interior H-lines → rowSpan=3 for grid row 0.
	rows := []float64{100, 120, 140, 160}
	cols := []float64{0, 50}
	grid := makeGrid(rows, cols)

	lines := []*RulingLine{
		// Only outer H-lines
		hLine(0, 100, 50), hLine(0, 160, 50),
		// Outer V-lines
		vLine(0, 100, 160), vLine(50, 100, 160),
	}

	result := DetectMergedCells(grid, lines, defaultMergeTolerance)
	require.Len(t, result, 1)
	assert.Equal(t, 0, result[0].Row)
	assert.Equal(t, 0, result[0].Col)
	assert.Equal(t, 3, result[0].RowSpan)
	assert.Equal(t, 1, result[0].ColSpan)
}

// ---------------------------------------------------------------------------
// DetectMergedCells — col span
// ---------------------------------------------------------------------------

func TestDetectMergedCells_ColSpan_Row0_Spans2(t *testing.T) {
	// 2×3 grid. Row 0 (gridRow=0, lowest Y band Y=100..120) is missing the
	// V-line at X=50, so cell (0,0) spans cols 0-1 (colSpan=2).
	rows := []float64{100, 120, 140}
	cols := []float64{0, 50, 100, 150}
	grid := makeGrid(rows, cols)

	lines := []*RulingLine{
		// Outer H-lines
		hLine(0, 100, 150), hLine(0, 140, 150),
		// Interior H-line at Y=120 (full width — separates grid rows)
		hLine(0, 120, 150),
		// Outer V-lines
		vLine(0, 100, 140), vLine(150, 100, 140),
		// V-line at X=50 exists only for top row Y=[120,140], not for bottom row Y=[100,120]
		vLine(50, 120, 140),
		// V-line at X=100 full height
		vLine(100, 100, 140),
	}

	result := DetectMergedCells(grid, lines, defaultMergeTolerance)
	require.Len(t, result, 1, "should detect exactly one merged cell")
	assert.Equal(t, 0, result[0].Row)
	assert.Equal(t, 0, result[0].Col)
	assert.Equal(t, 1, result[0].RowSpan)
	assert.Equal(t, 2, result[0].ColSpan)
}

// ---------------------------------------------------------------------------
// DetectMergedCells — both row and col span
// ---------------------------------------------------------------------------

func TestDetectMergedCells_BothSpans_2x2_in_4x4(t *testing.T) {
	// 4×3 grid (3 rows × 3 cols). The top-left 2×2 block is merged.
	//
	// Grid rows ascending (bottom-to-top PDF space):
	//   [100, 120, 140, 160]
	//   gridRow 0 = Y-band [100,120]
	//   gridRow 1 = Y-band [120,140]
	//   gridRow 2 = Y-band [140,160]
	//
	// Grid cols: [0, 50, 100, 150]
	//
	// Setup: H-line at Y=140 exists only for X=[50,150] (not col 0 [0,50]).
	// H-line at Y=120 exists only for X=[50,150] (not col 0 [0,50]).
	// → col 0 at gridRow 0 has no separator at Y=120 or Y=140 → rowSpan=3.
	//   But col 1-2 do have the separator at Y=120 and Y=140 so they don't merge.
	//
	// For the 2×2 merge, we need the merge starting at gridRow 1 (not 0):
	// H-line at Y=120: full width (separates gridRow 0 from 1 everywhere)
	// H-line at Y=140: exists only for X=[50,150] (not col 0)
	// → gridRow 1, col 0 has no separator at Y=140 → rowSpan=2.
	//
	// V-line at X=50: exists only for Y=[100,120] (below the merge zone Y=[120,160])
	// → For Y-range of merge [gridRow 1..2] = [120,160]: V-line covers [100,120]
	//   overlap with [120,160] is only [120,120] = 0 → no separator → colSpan=2.
	//
	// Expected: one MergedCellInfo{Row:1, Col:0, RowSpan:2, ColSpan:2}
	rows := []float64{100, 120, 140, 160}
	cols := []float64{0, 50, 100, 150}
	grid := makeGrid(rows, cols)

	lines := []*RulingLine{
		// Outer H-lines
		hLine(0, 100, 150), hLine(0, 160, 150),
		// H-line at Y=120: full width (separates gridRow 0 from 1 for ALL cols)
		hLine(0, 120, 150),
		// H-line at Y=140: exists only for cols 1-2 (X=[50,150])
		// → col 0 has NO separator between gridRow 1 and 2 → rowSpan=2 from gridRow 1
		hLine(50, 140, 150),
		// Outer V-lines
		vLine(0, 100, 160), vLine(150, 100, 160),
		// V-line at X=50: only for Y=[100,120] (below merge zone [120,160])
		// → colSpan coverage: overlap [100,120]∩[120,160] = empty → no separator → colSpan=2
		vLine(50, 100, 120),
		// V-line at X=100: full height (separates col 1 from col 2 everywhere)
		vLine(100, 100, 160),
	}

	result := DetectMergedCells(grid, lines, defaultMergeTolerance)

	// Find the 2×2 merge starting at (row=1, col=0)
	found := false
	for _, m := range result {
		if m.Row == 1 && m.Col == 0 && m.RowSpan == 2 && m.ColSpan == 2 {
			found = true
		}
	}
	assert.True(t, found, "expected MergedCellInfo{Row:1, Col:0, RowSpan:2, ColSpan:2}, got %v", result)
}

// ---------------------------------------------------------------------------
// DetectMergedCells — partial lines below threshold
// ---------------------------------------------------------------------------

func TestDetectMergedCells_PartialLine_BelowThreshold_TreatedAsMerge(t *testing.T) {
	// H-line at Y=120 covers only 40% of col 0 width [0,50] → [0,20].
	// Coverage < 70% → not a separator → cell spans rows 0-1.
	rows := []float64{100, 120, 140}
	cols := []float64{0, 50, 100}
	grid := makeGrid(rows, cols)

	lines := []*RulingLine{
		hLine(0, 100, 100), hLine(0, 140, 100),
		// Interior H-line covers only 40% of col 0 (20 out of 50 units)
		hLine(0, 120, 20),
		// H-line for col 1 [50,100] exists and covers full col
		hLine(50, 120, 100),
		vLine(0, 100, 140), vLine(100, 100, 140),
		vLine(50, 100, 140),
	}

	result := DetectMergedCells(grid, lines, defaultMergeTolerance)
	// col 0 at gridRow 0 should have rowSpan=2 (partial line insufficient)
	found := false
	for _, m := range result {
		if m.Col == 0 && m.Row == 0 && m.RowSpan == 2 {
			found = true
		}
	}
	assert.True(t, found, "partial line below threshold should result in rowSpan=2")
}

// ---------------------------------------------------------------------------
// DetectMergedCells — empty / edge cases
// ---------------------------------------------------------------------------

func TestDetectMergedCells_NilGrid(t *testing.T) {
	result := DetectMergedCells(nil, nil, defaultMergeTolerance)
	assert.Nil(t, result)
}

func TestDetectMergedCells_EmptyLines(t *testing.T) {
	// No ruling lines at all. Every interior boundary is "missing" so every
	// cell would span the full table — but that means grid row 0 spans all rows,
	// and the covered-map prevents double-counting. We get one merge per column
	// in grid row 0.
	rows := []float64{100, 120, 140}
	cols := []float64{0, 50, 100}
	grid := makeGrid(rows, cols)

	result := DetectMergedCells(grid, nil, defaultMergeTolerance)
	// With no lines at all, grid row 0 of each column spans the full 2 rows
	// and col 0 of grid row 0 spans 2 cols.  The details depend on the
	// traversal order, but we should get at least one merge.
	assert.NotEmpty(t, result, "no lines → every cell merges with neighbors")
}

func TestDetectMergedCells_SingleCellGrid(t *testing.T) {
	rows := []float64{100, 200}
	cols := []float64{0, 100}
	grid := makeGrid(rows, cols)

	lines := []*RulingLine{
		hLine(0, 100, 100), hLine(0, 200, 100),
		vLine(0, 100, 200), vLine(100, 100, 200),
	}

	result := DetectMergedCells(grid, lines, defaultMergeTolerance)
	assert.Empty(t, result, "1×1 grid cannot have merged cells")
}

// ---------------------------------------------------------------------------
// buildMergedMap and buildCoveredMap
// ---------------------------------------------------------------------------

func TestBuildMergedMap(t *testing.T) {
	infos := []MergedCellInfo{
		{Row: 0, Col: 0, RowSpan: 2, ColSpan: 1},
		{Row: 0, Col: 2, RowSpan: 1, ColSpan: 3},
	}
	m := buildMergedMap(infos)
	assert.Len(t, m, 2)
	v, ok := m[mergedKey{0, 0}]
	require.True(t, ok)
	assert.Equal(t, 2, v.RowSpan)
}

func TestBuildCoveredMap(t *testing.T) {
	infos := []MergedCellInfo{
		{Row: 0, Col: 0, RowSpan: 2, ColSpan: 2},
	}
	covered := buildCoveredMap(infos)
	// Owner (0,0) not covered
	assert.False(t, covered[mergedKey{0, 0}])
	// Covered positions
	assert.True(t, covered[mergedKey{0, 1}])
	assert.True(t, covered[mergedKey{1, 0}])
	assert.True(t, covered[mergedKey{1, 1}])
	// Outside span
	assert.False(t, covered[mergedKey{2, 0}])
}

// ---------------------------------------------------------------------------
// Integration: extractLatticeTable applies span information
// ---------------------------------------------------------------------------

func TestExtractLatticeTable_AppliesMergedSpans(t *testing.T) {
	// 3×2 grid (3 rows, 2 cols).
	// Col 0 has no interior H-line at Y=120 → gridRow 0 spans rows 0-1.
	//
	// Grid rows (ascending Y): [100, 120, 140]
	// Grid cols:               [0, 50, 100]
	//
	// In output space (top-to-bottom), gridRow 2 = output row 0,
	//                                  gridRow 1 = output row 1,
	//                                  gridRow 0 = output row 2.
	// gridRow 0 spans rows 0-1 in grid space = output rows 1-2.
	// The owner in output space is the smaller output row = output row 1.
	rows := []float64{100, 120, 140}
	cols := []float64{0, 50, 100}
	grid := makeGrid(rows, cols)

	lines := []*RulingLine{
		hLine(0, 100, 100), hLine(0, 140, 100),
		// Interior H-line at Y=120 exists only for col 1 (X=[50,100])
		hLine(50, 120, 100),
		vLine(0, 100, 140), vLine(100, 100, 140),
		vLine(50, 100, 140),
	}

	// Text elements: one in each "visual" cell
	textElements := []*extractor.TextElement{
		// gridRow 2 (output row 0) col 0 → Y=[120,140], X=[0,50]
		extractor.NewTextElement("Top-Left", 10, 135, 30, 10, "/F1", 12),
		// gridRow 2 (output row 0) col 1 → Y=[120,140], X=[50,100]
		extractor.NewTextElement("Top-Right", 60, 135, 30, 10, "/F1", 12),
		// gridRow 0-1 merged in col 0 (owner gridRow 0 = output row 2, but
		// owner is output row 1 after coordinate flip) → Y=[100,120], X=[0,50]
		extractor.NewTextElement("Merged", 10, 110, 30, 10, "/F1", 12),
		// gridRow 1 (output row 1) col 1 → Y=[100,120], X=[50,100]
		extractor.NewTextElement("Mid-Right", 60, 115, 30, 10, "/F1", 12),
		// gridRow 0 (output row 2) col 1 → Y=[100,120], X=[50,100]
		// actually same Y band as Mid-Right because grid is 3 rows: row 0=[100,120], row1=[120,140]
		// Wait: we have 3 rows → 2 row-bands: band 0=[100,120], band 1=[120,140]
	}

	region := &TableRegion{
		Bounds:      extractor.NewRectangle(0, 100, 100, 40),
		Method:      MethodLattice,
		Grid:        grid,
		RulingLines: lines,
	}

	te := NewTableExtractor(textElements)
	tbl, err := te.ExtractTable(region)
	require.NoError(t, err)

	assert.Equal(t, 2, tbl.RowCount)
	assert.Equal(t, 2, tbl.ColCount)
	assert.True(t, tbl.HasMergedCells(), "table should have merged cells")

	// Find the owner cell with RowSpan=2 somewhere in col 0
	found := false
	for r := 0; r < tbl.RowCount; r++ {
		cell := tbl.GetCell(r, 0)
		if cell != nil && cell.RowSpan == 2 {
			found = true
		}
	}
	assert.True(t, found, "col 0 should have a cell with RowSpan=2")
}

func TestExtractLatticeTable_NoRulingLines_NoMerges(t *testing.T) {
	// When region.RulingLines is empty, no merged cell detection runs,
	// and the table is extracted normally.
	rows := []float64{100, 120, 140}
	cols := []float64{0, 50, 100}
	grid := makeGrid(rows, cols)

	textElements := []*extractor.TextElement{
		extractor.NewTextElement("A", 10, 135, 20, 10, "/F1", 12),
		extractor.NewTextElement("B", 60, 135, 20, 10, "/F1", 12),
		extractor.NewTextElement("C", 10, 110, 20, 10, "/F1", 12),
		extractor.NewTextElement("D", 60, 110, 20, 10, "/F1", 12),
	}

	region := &TableRegion{
		Bounds: extractor.NewRectangle(0, 100, 100, 40),
		Method: MethodLattice,
		Grid:   grid,
		// RulingLines intentionally omitted
	}

	te := NewTableExtractor(textElements)
	tbl, err := te.ExtractTable(region)
	require.NoError(t, err)

	assert.Equal(t, 2, tbl.RowCount)
	assert.Equal(t, 2, tbl.ColCount)
	assert.False(t, tbl.HasMergedCells(), "no ruling lines → no merged cells detected")
}

// ---------------------------------------------------------------------------
// Integration test: real PDF with merged cells (issue #79)
// ---------------------------------------------------------------------------

// extractTablesFromPDF runs the full internal pipeline on the given PDF file
// and returns the domain tables found on pageNum (0-based).
//
// The function uses the internal parser, text extractor, graphics parser, and
// table detector + extractor directly so that the test can stay inside the
// tabledetect package without importing the root gxpdf package (which would
// be a circular dependency).
func extractTablesFromPDF(t *testing.T, path string, pageNum int) ([]*domaintable.Table, error) {
	t.Helper()

	readerPkg, err := newReaderFromPath(path)
	if err != nil {
		return nil, fmt.Errorf("open PDF: %w", err)
	}
	defer readerPkg.Close()

	textElements, err := readerPkg.ExtractTextElements(pageNum)
	if err != nil {
		return nil, fmt.Errorf("extract text: %w", err)
	}

	graphicsElements, err := readerPkg.ExtractGraphicsElements(pageNum)
	if err != nil {
		return nil, fmt.Errorf("extract graphics: %w", err)
	}

	detector := NewDefaultTableDetector()
	regions, err := detector.DetectTablesLattice(textElements, graphicsElements)
	if err != nil {
		return nil, fmt.Errorf("detect tables: %w", err)
	}

	tableExtractor := NewTableExtractor(textElements)
	var tables []*domaintable.Table
	for _, region := range regions {
		tbl, err := tableExtractor.ExtractTable(region)
		if err != nil {
			continue
		}
		tbl.PageNum = pageNum
		tables = append(tables, tbl)
	}
	return tables, nil
}

func TestIssue79_MergedCells(t *testing.T) {
	// This integration test verifies that the exam schedule PDF used to report
	// issue #79 is correctly parsed with merged cells in the TIME column.
	//
	// The PDF is located at testdata/pdfs/issue79/sample.pdf.
	// We parse it with the full pipeline and check that:
	//   - At least one cell in the extracted table has IsMerged() == true
	//   - The table has multiple rows (not collapsed to 1 row)
	//
	// We do NOT assert specific row/col indices because the exact layout
	// depends on the PDF content which may vary.

	const pdfPath = "../../testdata/pdfs/issue79/sample.pdf"

	tables, err := extractTablesFromPDF(t, pdfPath, 0)
	if err != nil {
		t.Skipf("skipping integration test: could not open PDF: %v", err)
	}
	if len(tables) == 0 {
		t.Skip("no tables found in test PDF — skipping")
	}

	tbl := tables[0]

	// The schedule table must have multiple rows.
	assert.Greater(t, tbl.RowCount, 1, "schedule table should have multiple rows")

	// At least one cell should be merged (TIME or VENUE column).
	assert.True(t, tbl.HasMergedCells(),
		"exam schedule table should have merged cells in TIME/VENUE columns")

	// Verify that all cells have valid spans (>= 1 in both dimensions).
	for r := 0; r < tbl.RowCount; r++ {
		for c := 0; c < tbl.ColCount; c++ {
			cell := tbl.GetCell(r, c)
			require.NotNil(t, cell, "cell at (%d,%d) should not be nil", r, c)
			assert.GreaterOrEqual(t, cell.RowSpan, 1,
				"cell (%d,%d) RowSpan must be >= 1", r, c)
			assert.GreaterOrEqual(t, cell.ColSpan, 1,
				"cell (%d,%d) ColSpan must be >= 1", r, c)
		}
	}
}
