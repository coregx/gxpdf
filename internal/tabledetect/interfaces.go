// Package detector implements table detection algorithms.
//
// This file defines interfaces for all major components to enable:
//   - Easy implementation replacement (different algorithms)
//   - Better testability (mocks)
//   - SOLID principles (Dependency Inversion)
package tabledetect

import (
	"github.com/coregx/gxpdf/internal/extractor"
)

// RulingLineDetector detects ruling lines from graphics elements.
//
// This interface enables:
//   - Swapping detection algorithms (e.g., Hough transform, machine learning)
//   - Mocking in tests
//   - Custom tolerance and filtering strategies
//
// Default implementation: DefaultRulingLineDetector
type RulingLineDetector interface {
	// DetectRulingLines extracts horizontal and vertical lines from graphics.
	// Returns a slice of RulingLines sorted by position.
	DetectRulingLines(graphics []*extractor.GraphicsElement) ([]*RulingLine, error)

	// FindIntersections finds intersection points between ruling lines.
	// Returns a slice of unique intersection points.
	FindIntersections(lines []*RulingLine) []extractor.Point
}

// WhitespaceAnalyzer analyzes whitespace distribution to find table structure.
//
// This interface enables:
//   - Alternative gap detection algorithms
//   - Custom alignment strategies
//   - Machine learning-based structure detection
//
// Default implementation: DefaultWhitespaceAnalyzer
type WhitespaceAnalyzer interface {
	// DetectColumns finds vertical alignment patterns (column boundaries).
	// Returns a slice of X coordinates representing column boundaries, sorted left to right.
	DetectColumns(elements []*extractor.TextElement) []float64

	// DetectRows finds horizontal alignment patterns (row boundaries).
	// Returns a slice of Y coordinates representing row boundaries, sorted bottom to top.
	DetectRows(elements []*extractor.TextElement) []float64

	// DetectTableRegion detects a table region based on whitespace analysis.
	// Returns the bounding rectangle of the detected table, or nil if no table.
	DetectTableRegion(elements []*extractor.TextElement) *extractor.Rectangle
}

// ProjectionAnalyzer analyzes text distribution to find whitespace gaps.
//
// This interface enables:
//   - Alternative projection algorithms (e.g., wavelet transforms)
//   - Custom density calculation methods
//   - Advanced gap detection strategies
//
// Default implementation: DefaultProjectionAnalyzer
type ProjectionAnalyzer interface {
	// AnalyzeHorizontal creates a horizontal projection profile from text elements.
	// The profile shows text density by Y coordinate (vertical distribution).
	AnalyzeHorizontal(elements []*extractor.TextElement) *ProjectionProfile

	// AnalyzeVertical creates a vertical projection profile from text elements.
	// The profile shows text density by X coordinate (horizontal distribution).
	AnalyzeVertical(elements []*extractor.TextElement) *ProjectionProfile

	// FindGaps finds whitespace gaps in a projection profile.
	// Gaps are regions where text density is below the threshold.
	FindGaps(profile *ProjectionProfile) []Gap

	// FindSignificantGaps finds gaps that are wide enough to be meaningful.
	// Returns gaps with width >= minWidth.
	FindSignificantGaps(profile *ProjectionProfile, minWidth float64) []Gap
}

// GridBuilder builds a grid structure from ruling lines.
//
// This interface enables:
//   - Alternative grid construction algorithms
//   - Custom cell detection strategies
//   - Handling of irregular or broken grids
//
// Default implementation: DefaultGridBuilder
type GridBuilder interface {
	// BuildGrid creates a grid from ruling lines.
	// The grid is defined by the intersections of horizontal and vertical lines.
	BuildGrid(lines []*RulingLine) (*Grid, error)

	// FindCellsFromIntersections builds cells from intersection points.
	FindCellsFromIntersections(horizontal, vertical []*RulingLine) ([]*Cell, error)

	// BuildGridFromIntersections creates a grid using ruling line intersection
	// points (ADR-005). Produces cells matching visual boundaries rather than
	// minimum ruling line spacing.
	BuildGridFromIntersections(horizontal, vertical []*RulingLine) (*Grid, error)

	// BuildGridFromCells creates a grid structure from detected cells.
	// Useful when cells are found through intersection detection.
	BuildGridFromCells(cells []*Cell) (*Grid, error)
}

// TableDetector detects table regions on PDF pages.
//
// This is the main interface for table detection. It supports multiple modes:
//   - Lattice mode: Detect tables with visible ruling lines
//   - Stream mode: Detect tables using whitespace analysis
//   - Auto mode: Automatically choose the best mode
//
// This interface enables:
//   - Custom detection strategies (e.g., ML-based, hybrid)
//   - A/B testing different algorithms
//   - Easy integration of third-party detectors
//
// Default implementation: DefaultTableDetector
type TableDetector interface {
	// DetectTables finds all table regions on a page.
	// This is the main entry point for table detection.
	// Uses auto-detection to choose the best mode.
	DetectTables(textElements []*extractor.TextElement, graphics []*extractor.GraphicsElement) ([]*TableRegion, error)

	// DetectMode auto-detects the best extraction mode.
	// Returns MethodLattice if ruling lines are detected, otherwise returns MethodStream.
	DetectMode(textElements []*extractor.TextElement, graphics []*extractor.GraphicsElement) ExtractionMethod

	// DetectTablesLattice explicitly uses lattice mode detection.
	// Use this when you know the table has ruling lines.
	DetectTablesLattice(textElements []*extractor.TextElement, graphics []*extractor.GraphicsElement) ([]*TableRegion, error)

	// DetectTablesStream explicitly uses stream mode detection.
	// Use this when you know the table doesn't have ruling lines.
	DetectTablesStream(textElements []*extractor.TextElement) ([]*TableRegion, error)
}
