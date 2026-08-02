// Package detector implements table detection algorithms.
package tabledetect

import (
	"fmt"

	"github.com/coregx/gxpdf/internal/extractor"
)

// ExtractionMethod represents the method used to extract a table.
type ExtractionMethod int

const (
	// MethodLattice indicates table was detected using ruling lines (grid).
	MethodLattice ExtractionMethod = iota
	// MethodStream indicates table was detected using whitespace analysis.
	MethodStream
	// MethodAuto indicates automatic detection mode.
	MethodAuto
)

const (
	methodNameLattice = "Lattice" //nolint:goconst // enum definition, not duplication
	methodNameStream  = "Stream"  //nolint:goconst // enum definition, not duplication
)

// String returns a string representation of the extraction method.
func (em ExtractionMethod) String() string {
	switch em {
	case MethodLattice:
		return methodNameLattice
	case MethodStream:
		return methodNameStream
	case MethodAuto:
		return "Auto"
	default:
		return "Unknown"
	}
}

// TableRegion represents a detected table region on a PDF page.
//
// A table region includes:
//   - Bounding rectangle
//   - Detection method used
//   - Grid structure (for lattice mode)
//   - Row/column coordinates (for stream mode)
//   - Ruling lines (for merged cell analysis in lattice mode)
//
// This represents the output of Phase 2.6 (Table Detection).
// Phase 2.7 will use this to extract actual cell content.
type TableRegion struct {
	Bounds         extractor.Rectangle // Bounding rectangle of table
	HasRulingLines bool                // True if detected via ruling lines
	Method         ExtractionMethod    // Detection method used
	Grid           *Grid               // Grid structure (lattice mode)
	Rows           []float64           // Row coordinates (stream mode)
	Columns        []float64           // Column coordinates (stream mode)
	RulingLines    []*RulingLine       // Ruling lines used to build the grid (lattice mode, for merged cell detection)
}

// NewTableRegion creates a new TableRegion.
func NewTableRegion(bounds extractor.Rectangle, method ExtractionMethod) *TableRegion {
	return &TableRegion{
		Bounds:         bounds,
		HasRulingLines: method == MethodLattice,
		Method:         method,
	}
}

// RowCount returns the number of rows in the table.
func (tr *TableRegion) RowCount() int {
	if tr.HasRulingLines && tr.Grid != nil {
		return tr.Grid.RowCount()
	}
	if len(tr.Rows) > 1 {
		return len(tr.Rows) - 1
	}
	return 0
}

// ColumnCount returns the number of columns in the table.
func (tr *TableRegion) ColumnCount() int {
	if tr.HasRulingLines && tr.Grid != nil {
		return tr.Grid.ColumnCount()
	}
	if len(tr.Columns) > 1 {
		return len(tr.Columns) - 1
	}
	return 0
}

// String returns a string representation of the table region.
func (tr *TableRegion) String() string {
	return fmt.Sprintf("TableRegion{method=%s, bounds=%s, rows=%d, cols=%d}",
		tr.Method.String(), tr.Bounds.String(), tr.RowCount(), tr.ColumnCount())
}

// DefaultTableDetector detects table regions on PDF pages.
//
// This is the default implementation of the TableDetector interface.
// The detector supports two modes:
//   - Lattice mode: Detect tables with visible ruling lines
//   - Stream mode: Detect tables using whitespace analysis
//   - Auto mode: Automatically choose the best mode
//
// Algorithm inspired by tabula-java's detection and extraction algorithms.
// References:
//   - tabula-java/technology/tabula/detectors/NurminenDetectionAlgorithm.java
//   - tabula-java/technology/tabula/extractors/SpreadsheetExtractionAlgorithm.java
//   - tabula-java/technology/tabula/extractors/BasicExtractionAlgorithm.java
type DefaultTableDetector struct {
	rulingDetector     RulingLineDetector
	whitespaceAnalyzer WhitespaceAnalyzer
	gridBuilder        GridBuilder
}

// NewDefaultTableDetector creates a new DefaultTableDetector with default implementations.
func NewDefaultTableDetector() *DefaultTableDetector {
	return &DefaultTableDetector{
		rulingDetector:     NewDefaultRulingLineDetector(),
		whitespaceAnalyzer: NewDefaultWhitespaceAnalyzer(),
		gridBuilder:        NewDefaultGridBuilder(),
	}
}

// NewTableDetector creates a new DefaultTableDetector with default implementations.
// Deprecated: Use NewDefaultTableDetector or NewTableDetectorWithDeps instead. Kept for backward compatibility.
func NewTableDetector() *DefaultTableDetector {
	return NewDefaultTableDetector()
}

// NewTableDetectorWithDeps creates a new DefaultTableDetector with custom dependencies.
//
// This is the recommended constructor for dependency injection.
// Use this when you want to provide custom implementations of components.
//
// Example:
//
//	detector := NewTableDetectorWithDeps(
//	    myCustomRulingDetector,
//	    NewDefaultWhitespaceAnalyzer(),
//	    NewDefaultGridBuilder(),
//	)
func NewTableDetectorWithDeps(
	rulingDetector RulingLineDetector,
	whitespaceAnalyzer WhitespaceAnalyzer,
	gridBuilder GridBuilder,
) *DefaultTableDetector {
	return &DefaultTableDetector{
		rulingDetector:     rulingDetector,
		whitespaceAnalyzer: whitespaceAnalyzer,
		gridBuilder:        gridBuilder,
	}
}

// WithRulingDetector sets the ruling line detector.
func (td *DefaultTableDetector) WithRulingDetector(detector RulingLineDetector) *DefaultTableDetector {
	td.rulingDetector = detector
	return td
}

// WithWhitespaceAnalyzer sets the whitespace analyzer.
func (td *DefaultTableDetector) WithWhitespaceAnalyzer(analyzer WhitespaceAnalyzer) *DefaultTableDetector {
	td.whitespaceAnalyzer = analyzer
	return td
}

// WithGridBuilder sets the grid builder.
func (td *DefaultTableDetector) WithGridBuilder(builder GridBuilder) *DefaultTableDetector {
	td.gridBuilder = builder
	return td
}

// DetectTables finds all table regions on a page.
//
// This is the main entry point for table detection.
//
// Parameters:
//   - textElements: Text elements extracted from the page
//   - graphics: Graphics elements extracted from the page
//
// Returns a slice of TableRegions, or error if detection fails.
func (td *DefaultTableDetector) DetectTables(
	textElements []*extractor.TextElement,
	graphics []*extractor.GraphicsElement,
) ([]*TableRegion, error) {
	// Auto-detect best mode
	mode := td.DetectMode(textElements, graphics)

	switch mode {
	case MethodLattice:
		return td.detectLattice(textElements, graphics)
	case MethodStream:
		return td.detectStream(textElements)
	default:
		return nil, fmt.Errorf("unknown detection mode: %v", mode)
	}
}

// DetectMode auto-detects the best extraction mode.
//
// Returns MethodLattice if ruling lines are detected,
// otherwise returns MethodStream.
func (td *DefaultTableDetector) DetectMode(
	textElements []*extractor.TextElement,
	graphics []*extractor.GraphicsElement,
) ExtractionMethod {
	// Try to detect ruling lines
	rulingLines, err := td.rulingDetector.DetectRulingLines(graphics)
	if err == nil && len(rulingLines) >= 4 {
		// Found ruling lines - use lattice mode
		// Need at least 2 horizontal and 2 vertical lines
		horizontal := 0
		vertical := 0
		for _, line := range rulingLines {
			if line.IsHorizontal {
				horizontal++
			} else {
				vertical++
			}
		}

		if horizontal >= 2 && vertical >= 2 {
			return MethodLattice
		}
	}

	// No ruling lines - use stream mode
	return MethodStream
}

// detectLattice detects tables using lattice mode (ruling lines).
//
// This mode is more accurate when tables have visible borders.
func (td *DefaultTableDetector) detectLattice(
	textElements []*extractor.TextElement,
	graphics []*extractor.GraphicsElement,
) ([]*TableRegion, error) {
	// Detect ruling lines
	rulingLines, err := td.rulingDetector.DetectRulingLines(graphics)
	if err != nil {
		return nil, fmt.Errorf("failed to detect ruling lines: %w", err)
	}

	if len(rulingLines) < 4 {
		// Not enough lines - fall back to stream mode
		return td.detectStream(textElements)
	}

	// Separate horizontal and vertical lines for intersection-based grid.
	var hLines, vLines []*RulingLine
	for _, line := range rulingLines {
		if line.IsHorizontal {
			hLines = append(hLines, line)
		} else {
			vLines = append(vLines, line)
		}
	}

	// Build grid from ruling line intersections (ADR-005).
	// Falls back to sorted-coordinate grid if intersection method fails.
	grid, err := td.gridBuilder.BuildGridFromIntersections(hLines, vLines)
	if err != nil {
		grid, err = td.gridBuilder.BuildGrid(rulingLines)
	}
	if err != nil {
		return td.detectStream(textElements)
	}

	// Validate grid
	if !td.isValidGrid(grid) {
		// Invalid grid - fall back to stream mode
		return td.detectStream(textElements)
	}

	// Create table region
	region := NewTableRegion(grid.Bounds(), MethodLattice)
	region.Grid = grid
	region.HasRulingLines = true
	// Copy grid columns/rows to region for convenience (2025-10-27)
	// This makes region.Columns available for easy access
	region.Columns = grid.Columns
	region.Rows = grid.Rows
	// Store ruling lines for merged cell detection in Phase 2.7
	region.RulingLines = rulingLines

	return []*TableRegion{region}, nil
}

// detectStream detects tables using stream mode (whitespace analysis).
//
// This mode is used when tables don't have visible borders.
func (td *DefaultTableDetector) detectStream(textElements []*extractor.TextElement) ([]*TableRegion, error) {
	if len(textElements) == 0 {
		return []*TableRegion{}, nil
	}

	// Detect columns and rows
	columns := td.whitespaceAnalyzer.DetectColumns(textElements)
	rows := td.whitespaceAnalyzer.DetectRows(textElements)

	// Need at least 2 rows and 2 columns for a table
	if len(columns) < 2 || len(rows) < 2 {
		// No table detected
		return []*TableRegion{}, nil
	}

	// Calculate bounding rectangle
	bounds := td.calculateBoundsFromText(textElements)

	// Create table region
	region := NewTableRegion(bounds, MethodStream)
	region.Rows = rows
	region.Columns = columns
	region.HasRulingLines = false

	return []*TableRegion{region}, nil
}

// isValidGrid checks if a grid is valid for table extraction.
//
// A valid grid should have:
//   - At least 2 rows and 2 columns
//   - Reasonable cell sizes
func (td *DefaultTableDetector) isValidGrid(grid *Grid) bool {
	if grid == nil {
		return false
	}

	// Need at least 2 rows and 2 columns
	if grid.RowCount() < 1 || grid.ColumnCount() < 1 {
		return false
	}

	// Grid should have reasonable bounds
	bounds := grid.Bounds()
	if bounds.Width < 50 || bounds.Height < 50 {
		// Too small to be a table
		return false
	}

	return true
}

// calculateBoundsFromText calculates the bounding rectangle from text elements.
func (td *DefaultTableDetector) calculateBoundsFromText(elements []*extractor.TextElement) extractor.Rectangle {
	if len(elements) == 0 {
		return extractor.NewRectangle(0, 0, 0, 0)
	}

	// Find min/max coordinates
	minX := elements[0].X
	minY := elements[0].Y
	maxX := elements[0].Right()
	maxY := elements[0].Top()

	for _, elem := range elements[1:] {
		if elem.X < minX {
			minX = elem.X
		}
		if elem.Y < minY {
			minY = elem.Y
		}
		if elem.Right() > maxX {
			maxX = elem.Right()
		}
		if elem.Top() > maxY {
			maxY = elem.Top()
		}
	}

	return extractor.NewRectangle(minX, minY, maxX-minX, maxY-minY)
}

// DetectTablesLattice explicitly uses lattice mode detection.
//
// Use this when you know the table has ruling lines.
func (td *DefaultTableDetector) DetectTablesLattice(
	textElements []*extractor.TextElement,
	graphics []*extractor.GraphicsElement,
) ([]*TableRegion, error) {
	return td.detectLattice(textElements, graphics)
}

// DetectTablesStream explicitly uses stream mode detection.
//
// Use this when you know the table doesn't have ruling lines.
func (td *DefaultTableDetector) DetectTablesStream(
	textElements []*extractor.TextElement,
) ([]*TableRegion, error) {
	return td.detectStream(textElements)
}
