package tabledetect

import (
	"errors"

	"github.com/coregx/gxpdf/internal/extractor"
)

var errMockNotImplemented = errors.New("not implemented in mock")

// MockRulingLineDetector is a mock implementation of RulingLineDetector for testing.
type MockRulingLineDetector struct {
	DetectRulingLinesFunc   func(graphics []*extractor.GraphicsElement) ([]*RulingLine, error)
	FindIntersectionsFunc   func(lines []*RulingLine) []extractor.Point
	DetectRulingLinesResult []*RulingLine
	DetectRulingLinesError  error
	FindIntersectionsResult []extractor.Point
}

// NewMockRulingLineDetector creates a new MockRulingLineDetector.
func NewMockRulingLineDetector() *MockRulingLineDetector {
	return &MockRulingLineDetector{}
}

// DetectRulingLines implements RulingLineDetector interface.
func (m *MockRulingLineDetector) DetectRulingLines(graphics []*extractor.GraphicsElement) ([]*RulingLine, error) {
	if m.DetectRulingLinesFunc != nil {
		return m.DetectRulingLinesFunc(graphics)
	}
	return m.DetectRulingLinesResult, m.DetectRulingLinesError
}

// FindIntersections implements RulingLineDetector interface.
func (m *MockRulingLineDetector) FindIntersections(lines []*RulingLine) []extractor.Point {
	if m.FindIntersectionsFunc != nil {
		return m.FindIntersectionsFunc(lines)
	}
	return m.FindIntersectionsResult
}

// MockWhitespaceAnalyzer is a mock implementation of WhitespaceAnalyzer for testing.
type MockWhitespaceAnalyzer struct {
	DetectColumnsFunc       func(elements []*extractor.TextElement) []float64
	DetectRowsFunc          func(elements []*extractor.TextElement) []float64
	DetectTableRegionFunc   func(elements []*extractor.TextElement) *extractor.Rectangle
	DetectColumnsResult     []float64
	DetectRowsResult        []float64
	DetectTableRegionResult *extractor.Rectangle
}

// NewMockWhitespaceAnalyzer creates a new MockWhitespaceAnalyzer.
func NewMockWhitespaceAnalyzer() *MockWhitespaceAnalyzer {
	return &MockWhitespaceAnalyzer{}
}

// DetectColumns implements WhitespaceAnalyzer interface.
func (m *MockWhitespaceAnalyzer) DetectColumns(elements []*extractor.TextElement) []float64 {
	if m.DetectColumnsFunc != nil {
		return m.DetectColumnsFunc(elements)
	}
	return m.DetectColumnsResult
}

// DetectRows implements WhitespaceAnalyzer interface.
func (m *MockWhitespaceAnalyzer) DetectRows(elements []*extractor.TextElement) []float64 {
	if m.DetectRowsFunc != nil {
		return m.DetectRowsFunc(elements)
	}
	return m.DetectRowsResult
}

// DetectTableRegion implements WhitespaceAnalyzer interface.
func (m *MockWhitespaceAnalyzer) DetectTableRegion(elements []*extractor.TextElement) *extractor.Rectangle {
	if m.DetectTableRegionFunc != nil {
		return m.DetectTableRegionFunc(elements)
	}
	return m.DetectTableRegionResult
}

// MockProjectionAnalyzer is a mock implementation of ProjectionAnalyzer for testing.
type MockProjectionAnalyzer struct {
	AnalyzeHorizontalFunc     func(elements []*extractor.TextElement) *ProjectionProfile
	AnalyzeVerticalFunc       func(elements []*extractor.TextElement) *ProjectionProfile
	FindGapsFunc              func(profile *ProjectionProfile) []Gap
	FindSignificantGapsFunc   func(profile *ProjectionProfile, minWidth float64) []Gap
	AnalyzeHorizontalResult   *ProjectionProfile
	AnalyzeVerticalResult     *ProjectionProfile
	FindGapsResult            []Gap
	FindSignificantGapsResult []Gap
}

// NewMockProjectionAnalyzer creates a new MockProjectionAnalyzer.
func NewMockProjectionAnalyzer() *MockProjectionAnalyzer {
	return &MockProjectionAnalyzer{}
}

// AnalyzeHorizontal implements ProjectionAnalyzer interface.
func (m *MockProjectionAnalyzer) AnalyzeHorizontal(elements []*extractor.TextElement) *ProjectionProfile {
	if m.AnalyzeHorizontalFunc != nil {
		return m.AnalyzeHorizontalFunc(elements)
	}
	return m.AnalyzeHorizontalResult
}

// AnalyzeVertical implements ProjectionAnalyzer interface.
func (m *MockProjectionAnalyzer) AnalyzeVertical(elements []*extractor.TextElement) *ProjectionProfile {
	if m.AnalyzeVerticalFunc != nil {
		return m.AnalyzeVerticalFunc(elements)
	}
	return m.AnalyzeVerticalResult
}

// FindGaps implements ProjectionAnalyzer interface.
func (m *MockProjectionAnalyzer) FindGaps(profile *ProjectionProfile) []Gap {
	if m.FindGapsFunc != nil {
		return m.FindGapsFunc(profile)
	}
	return m.FindGapsResult
}

// FindSignificantGaps implements ProjectionAnalyzer interface.
func (m *MockProjectionAnalyzer) FindSignificantGaps(profile *ProjectionProfile, minWidth float64) []Gap {
	if m.FindSignificantGapsFunc != nil {
		return m.FindSignificantGapsFunc(profile, minWidth)
	}
	return m.FindSignificantGapsResult
}

// MockGridBuilder is a mock implementation of GridBuilder for testing.
type MockGridBuilder struct {
	BuildGridFunc                  func(lines []*RulingLine) (*Grid, error)
	FindCellsFromIntersectionsFunc func(horizontal, vertical []*RulingLine) ([]*Cell, error)
	BuildGridFromCellsFunc         func(cells []*Cell) (*Grid, error)
	BuildGridResult                *Grid
	BuildGridError                 error
	FindCellsResult                []*Cell
	FindCellsError                 error
	BuildGridFromCellsResult       *Grid
	BuildGridFromCellsError        error
}

// NewMockGridBuilder creates a new MockGridBuilder.
func NewMockGridBuilder() *MockGridBuilder {
	return &MockGridBuilder{}
}

// BuildGrid implements GridBuilder interface.
func (m *MockGridBuilder) BuildGrid(lines []*RulingLine) (*Grid, error) {
	if m.BuildGridFunc != nil {
		return m.BuildGridFunc(lines)
	}
	return m.BuildGridResult, m.BuildGridError
}

// FindCellsFromIntersections implements GridBuilder interface.
func (m *MockGridBuilder) FindCellsFromIntersections(horizontal, vertical []*RulingLine) ([]*Cell, error) {
	if m.FindCellsFromIntersectionsFunc != nil {
		return m.FindCellsFromIntersectionsFunc(horizontal, vertical)
	}
	return m.FindCellsResult, m.FindCellsError
}

// BuildGridFromIntersections implements GridBuilder interface.
func (m *MockGridBuilder) BuildGridFromIntersections(horizontal, vertical []*RulingLine) (*Grid, error) {
	return nil, errMockNotImplemented
}

// BuildGridFromCells implements GridBuilder interface.
func (m *MockGridBuilder) BuildGridFromCells(cells []*Cell) (*Grid, error) {
	if m.BuildGridFromCellsFunc != nil {
		return m.BuildGridFromCellsFunc(cells)
	}
	return m.BuildGridFromCellsResult, m.BuildGridFromCellsError
}

// MockTableDetector is a mock implementation of TableDetector for testing.
type MockTableDetector struct {
	DetectTablesFunc          func(textElements []*extractor.TextElement, graphics []*extractor.GraphicsElement) ([]*TableRegion, error)
	DetectModeFunc            func(textElements []*extractor.TextElement, graphics []*extractor.GraphicsElement) ExtractionMethod
	DetectTablesLatticeFunc   func(textElements []*extractor.TextElement, graphics []*extractor.GraphicsElement) ([]*TableRegion, error)
	DetectTablesStreamFunc    func(textElements []*extractor.TextElement) ([]*TableRegion, error)
	DetectTablesResult        []*TableRegion
	DetectTablesError         error
	DetectModeResult          ExtractionMethod
	DetectTablesLatticeResult []*TableRegion
	DetectTablesLatticeError  error
	DetectTablesStreamResult  []*TableRegion
	DetectTablesStreamError   error
}

// NewMockTableDetector creates a new MockTableDetector.
func NewMockTableDetector() *MockTableDetector {
	return &MockTableDetector{}
}

// DetectTables implements TableDetector interface.
func (m *MockTableDetector) DetectTables(textElements []*extractor.TextElement, graphics []*extractor.GraphicsElement) ([]*TableRegion, error) {
	if m.DetectTablesFunc != nil {
		return m.DetectTablesFunc(textElements, graphics)
	}
	return m.DetectTablesResult, m.DetectTablesError
}

// DetectMode implements TableDetector interface.
func (m *MockTableDetector) DetectMode(textElements []*extractor.TextElement, graphics []*extractor.GraphicsElement) ExtractionMethod {
	if m.DetectModeFunc != nil {
		return m.DetectModeFunc(textElements, graphics)
	}
	return m.DetectModeResult
}

// DetectTablesLattice implements TableDetector interface.
func (m *MockTableDetector) DetectTablesLattice(textElements []*extractor.TextElement, graphics []*extractor.GraphicsElement) ([]*TableRegion, error) {
	if m.DetectTablesLatticeFunc != nil {
		return m.DetectTablesLatticeFunc(textElements, graphics)
	}
	return m.DetectTablesLatticeResult, m.DetectTablesLatticeError
}

// DetectTablesStream implements TableDetector interface.
func (m *MockTableDetector) DetectTablesStream(textElements []*extractor.TextElement) ([]*TableRegion, error) {
	if m.DetectTablesStreamFunc != nil {
		return m.DetectTablesStreamFunc(textElements)
	}
	return m.DetectTablesStreamResult, m.DetectTablesStreamError
}
