// Package extractor implements PDF content extraction use cases.
package extractor

import (
	"fmt"

	"github.com/coregx/gxpdf/internal/parser"
)

// GraphicsElement represents a graphics element extracted from a PDF page.
//
// Graphics elements include lines, rectangles, and paths that can be used
// to detect ruling lines in tables (lattice mode detection).
//
// PDF Graphics Operators (Section 8.5):
//   - Path construction: m (moveto), l (lineto), re (rectangle), c (curve)
//   - Path painting: S (stroke), s (close/stroke), f (fill), F (fill)
//
// Reference: PDF 1.7 specification, Section 8.5 (Graphics Objects).
type GraphicsElement struct {
	Type   GraphicsType // Type of graphics element
	Points []Point      // Points defining the element
	Color  Color        // Stroke/fill color
	Width  float64      // Line width
}

// GraphicsType represents the type of graphics element.
type GraphicsType int

const (
	// GraphicsTypeLine represents a straight line.
	GraphicsTypeLine GraphicsType = iota
	// GraphicsTypeRectangle represents a rectangle.
	GraphicsTypeRectangle
	// GraphicsTypePath represents a generic path.
	GraphicsTypePath
)

// String returns a string representation of the graphics type.
func (gt GraphicsType) String() string {
	switch gt {
	case GraphicsTypeLine:
		return "Line"
	case GraphicsTypeRectangle:
		return "Rectangle"
	case GraphicsTypePath:
		return "Path"
	default:
		return "Unknown"
	}
}

// Point represents a 2D point in PDF coordinate space.
//
// PDF coordinates are in points (1/72 inch), with origin at bottom-left.
type Point struct {
	X, Y float64
}

// NewPoint creates a new Point.
func NewPoint(x, y float64) Point {
	return Point{X: x, Y: y}
}

// String returns a string representation of the point.
func (p Point) String() string {
	return fmt.Sprintf("(%.2f, %.2f)", p.X, p.Y)
}

// Color represents an RGB color.
type Color struct {
	R, G, B float64 // RGB values (0.0 - 1.0)
}

// NewColor creates a new Color.
func NewColor(r, g, b float64) Color {
	return Color{R: r, G: g, B: b}
}

// IsBlack returns true if the color is black (or very dark).
func (c Color) IsBlack() bool {
	// Consider colors with all components < 0.1 as black
	return c.R < 0.1 && c.G < 0.1 && c.B < 0.1
}

// String returns a string representation of the color.
func (c Color) String() string {
	return fmt.Sprintf("RGB(%.2f, %.2f, %.2f)", c.R, c.G, c.B)
}

// GraphicsParser extracts graphics elements from PDF content streams.
//
// The parser processes graphics operators to extract lines, rectangles,
// and other shapes that can be used for table detection (ruling lines).
//
// Graphics State (Section 8.4):
//   - Current transformation matrix (CTM)
//   - Current path
//   - Line width, color, etc.
//
// Reference: PDF 1.7 specification, Section 8 (Graphics).
type GraphicsParser struct {
	reader   *parser.Reader
	elements []*GraphicsElement
	state    *GraphicsState
}

// GraphicsState tracks the current graphics state during parsing.
type GraphicsState struct {
	CurrentPath []Point // Points in current path
	LineWidth   float64 // Current line width
	StrokeColor Color   // Current stroke color
	FillColor   Color   // Current fill color
}

// NewGraphicsState creates a new graphics state with defaults.
func NewGraphicsState() *GraphicsState {
	return &GraphicsState{
		CurrentPath: []Point{},
		LineWidth:   1.0,
		StrokeColor: NewColor(0, 0, 0), // Black
		FillColor:   NewColor(0, 0, 0), // Black
	}
}

// NewGraphicsParser creates a new GraphicsParser for the given PDF reader.
func NewGraphicsParser(reader *parser.Reader) *GraphicsParser {
	return &GraphicsParser{
		reader:   reader,
		elements: []*GraphicsElement{},
		state:    NewGraphicsState(),
	}
}

// ParseFromPage extracts all graphics elements from the specified page.
//
// Page numbers are 0-based (first page is 0).
//
// Returns a slice of GraphicsElements, or error if extraction fails.
func (gp *GraphicsParser) ParseFromPage(pageNum int) ([]*GraphicsElement, error) {
	// Reset state
	gp.elements = []*GraphicsElement{}
	gp.state = NewGraphicsState()

	// Get page
	page, err := gp.reader.GetPage(pageNum)
	if err != nil {
		return nil, fmt.Errorf("failed to get page %d: %w", pageNum, err)
	}

	// Get content stream(s)
	contentData, err := gp.getPageContent(page)
	if err != nil {
		return nil, fmt.Errorf("failed to get page content: %w", err)
	}

	// If no content, return empty list
	if len(contentData) == 0 {
		return []*GraphicsElement{}, nil
	}

	// Parse content stream operators
	contentParser := NewContentParser(contentData)
	operators, err := contentParser.ParseOperators()
	if err != nil {
		return nil, fmt.Errorf("failed to parse content stream: %w", err)
	}

	// Process operators to extract graphics
	for _, op := range operators {
		gp.processOperator(op)
	}

	return gp.elements, nil
}

// getPageContent retrieves and decodes the content stream(s) for a page.
//
// This is the same logic as text extraction.
//
//nolint:cyclop,dupl // Similar to TextExtractor.getPageContent, refactoring later
func (gp *GraphicsParser) getPageContent(page *parser.Dictionary) ([]byte, error) {
	contentsObj := page.Get("Contents")
	if contentsObj == nil {
		// No content stream - empty page
		return []byte{}, nil
	}

	// Resolve if it's an indirect reference
	if ref, ok := contentsObj.(*parser.IndirectReference); ok {
		resolved, err := gp.reader.GetObject(ref.Number)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve contents reference: %w", err)
		}
		contentsObj = resolved
	}

	var allContent []byte

	// Check if it's a single stream or an array of streams
	switch obj := contentsObj.(type) {
	case *parser.Stream:
		// Single stream
		content, err := gp.decodeStream(obj)
		if err != nil {
			return nil, fmt.Errorf("failed to decode content stream: %w", err)
		}
		allContent = content

	case *parser.Array:
		// Array of streams - concatenate them
		for i := 0; i < obj.Len(); i++ {
			streamRef := obj.Get(i)
			if streamRef == nil {
				continue
			}

			// Resolve indirect reference
			if ref, ok := streamRef.(*parser.IndirectReference); ok {
				resolved, err := gp.reader.GetObject(ref.Number)
				if err != nil {
					continue
				}
				streamRef = resolved
			}

			// Decode stream
			if stream, ok := streamRef.(*parser.Stream); ok {
				content, err := gp.decodeStream(stream)
				if err != nil {
					continue
				}
				allContent = append(allContent, content...)
				// Add space between streams for safety
				allContent = append(allContent, ' ')
			}
		}

	default:
		return nil, fmt.Errorf("unexpected Contents type: %T", obj)
	}

	return allContent, nil
}

// decodeStream decodes a PDF stream based on its filters.
//
// For now, we handle FlateDecode (most common).
//
//nolint:dupl // Similar to TextExtractor.decodeStream, refactoring later
func (gp *GraphicsParser) decodeStream(stream *parser.Stream) ([]byte, error) {
	// Get filter
	filterObj := stream.Dictionary().Get("Filter")
	if filterObj == nil {
		// No filter - return raw content
		return stream.Content(), nil
	}

	// Get filter name
	var filterName string
	if name, ok := filterObj.(*parser.Name); ok {
		filterName = name.Value()
	} else if arr, ok := filterObj.(*parser.Array); ok {
		// Array of filters - for now, just handle first one
		if arr.Len() > 0 {
			if name, ok := arr.Get(0).(*parser.Name); ok {
				filterName = name.Value()
			}
		}
	}

	// Apply filter
	switch filterName {
	case filterFlateDecode:
		// Use shared decoding logic from text extractor
		te := &TextExtractor{reader: gp.reader}
		return te.decodeFlateDecode(stream.Content())

	case "":
		// No filter
		return stream.Content(), nil

	default:
		// Unsupported filter - return raw content
		return stream.Content(), nil
	}
}

// processOperator processes a single graphics operator.
//
// This extracts lines, rectangles, and paths from graphics operators.
//
// Reference: PDF 1.7 specification, Section 8.5 (Graphics Objects).
//
//nolint:cyclop,funlen // Graphics operator processing requires many cases
func (gp *GraphicsParser) processOperator(op *Operator) {
	switch op.Name {
	// Path construction operators (Section 8.5.2)
	case "m": // moveto - start new subpath
		if len(op.Operands) >= 2 {
			x := getNumber(op.Operands[0])
			y := getNumber(op.Operands[1])
			if x != nil && y != nil {
				// Start new path
				gp.state.CurrentPath = []Point{NewPoint(*x, *y)}
			}
		}

	case "l": // lineto - add line segment to path
		if len(op.Operands) >= 2 {
			x := getNumber(op.Operands[0])
			y := getNumber(op.Operands[1])
			if x != nil && y != nil {
				gp.state.CurrentPath = append(gp.state.CurrentPath, NewPoint(*x, *y))
			}
		}

	case "re": // rectangle - add rectangle to path
		if len(op.Operands) >= 4 {
			x := getNumber(op.Operands[0])
			y := getNumber(op.Operands[1])
			w := getNumber(op.Operands[2])
			h := getNumber(op.Operands[3])
			if x != nil && y != nil && w != nil && h != nil {
				// Rectangle as path: bottom-left, bottom-right, top-right, top-left, close
				gp.state.CurrentPath = []Point{
					NewPoint(*x, *y),
					NewPoint(*x+*w, *y),
					NewPoint(*x+*w, *y+*h),
					NewPoint(*x, *y+*h),
					NewPoint(*x, *y), // Close path
				}
			}
		}

	// Path painting operators (Section 8.5.3)
	case "S": // Stroke path
		gp.strokePath()

	case "s": // Close and stroke path
		gp.closePath()
		gp.strokePath()

	case "f", "F", "f*": // Fill path
		gp.fillPath()

	case "B", "B*": // Fill and stroke path
		gp.fillPath()

	case "b", "b*": // Close, fill, and stroke path
		gp.closePath()
		gp.fillPath()

	case "h": // Close subpath
		gp.closePath()

	// Graphics state operators (Section 8.4)
	case "w": // Set line width
		if len(op.Operands) >= 1 {
			if width := getNumber(op.Operands[0]); width != nil {
				gp.state.LineWidth = *width
			}
		}

	case "RG": // Set RGB stroke color
		if len(op.Operands) >= 3 {
			r := getNumber(op.Operands[0])
			g := getNumber(op.Operands[1])
			b := getNumber(op.Operands[2])
			if r != nil && g != nil && b != nil {
				gp.state.StrokeColor = NewColor(*r, *g, *b)
			}
		}

	case "rg": // Set RGB fill color
		if len(op.Operands) >= 3 {
			r := getNumber(op.Operands[0])
			g := getNumber(op.Operands[1])
			b := getNumber(op.Operands[2])
			if r != nil && g != nil && b != nil {
				gp.state.FillColor = NewColor(*r, *g, *b)
			}
		}

	case "G": // Set grayscale stroke color
		if len(op.Operands) >= 1 {
			if gray := getNumber(op.Operands[0]); gray != nil {
				gp.state.StrokeColor = NewColor(*gray, *gray, *gray)
			}
		}

	case "g": // Set grayscale fill color
		if len(op.Operands) >= 1 {
			if gray := getNumber(op.Operands[0]); gray != nil {
				gp.state.FillColor = NewColor(*gray, *gray, *gray)
			}
		}
	}
}

// strokePath creates graphics elements from the current path.
func (gp *GraphicsParser) strokePath() {
	if len(gp.state.CurrentPath) < 2 {
		gp.clearPath()
		return
	}

	// If it's a simple 2-point path, it's a line
	if len(gp.state.CurrentPath) == 2 {
		elem := &GraphicsElement{
			Type:   GraphicsTypeLine,
			Points: gp.state.CurrentPath,
			Color:  gp.state.StrokeColor,
			Width:  gp.state.LineWidth,
		}
		gp.elements = append(gp.elements, elem)
	} else if gp.isRectangle(gp.state.CurrentPath) {
		// If it's a closed rectangle (5 points, last == first)
		elem := &GraphicsElement{
			Type:   GraphicsTypeRectangle,
			Points: gp.state.CurrentPath,
			Color:  gp.state.StrokeColor,
			Width:  gp.state.LineWidth,
		}
		gp.elements = append(gp.elements, elem)
	} else {
		// Generic path - we can extract individual line segments
		for i := 0; i < len(gp.state.CurrentPath)-1; i++ {
			elem := &GraphicsElement{
				Type:   GraphicsTypeLine,
				Points: []Point{gp.state.CurrentPath[i], gp.state.CurrentPath[i+1]},
				Color:  gp.state.StrokeColor,
				Width:  gp.state.LineWidth,
			}
			gp.elements = append(gp.elements, elem)
		}
	}

	gp.clearPath()
}

// fillPath creates graphics elements from filled paths.
// Filled rectangles are common table borders in PDFs generated by
// wkhtmltopdf, Chrome print, LibreOffice, and similar tools.
func (gp *GraphicsParser) fillPath() {
	if len(gp.state.CurrentPath) < 3 {
		gp.clearPath()
		return
	}

	if gp.isRectangle(gp.state.CurrentPath) {
		elem := &GraphicsElement{
			Type:   GraphicsTypeRectangle,
			Points: gp.state.CurrentPath,
			Color:  gp.state.FillColor,
			Width:  gp.state.LineWidth,
		}
		gp.elements = append(gp.elements, elem)
	}

	gp.clearPath()
}

// closePath closes the current path by adding a line to the start point.
func (gp *GraphicsParser) closePath() {
	if len(gp.state.CurrentPath) > 0 {
		// Add line back to first point
		first := gp.state.CurrentPath[0]
		gp.state.CurrentPath = append(gp.state.CurrentPath, first)
	}
}

// clearPath clears the current path.
func (gp *GraphicsParser) clearPath() {
	gp.state.CurrentPath = []Point{}
}

// isRectangle checks if a path represents a rectangle.
func (gp *GraphicsParser) isRectangle(points []Point) bool {
	// Rectangle should have 5 points (4 corners + close to first)
	if len(points) != 5 {
		return false
	}

	// Last point should be same as first (closed path)
	if points[0] != points[4] {
		return false
	}

	// Check if points form a rectangle (axis-aligned)
	// Point 0 and 1 should share either X or Y
	// Point 1 and 2 should share the other coordinate
	p0, p1, p2, p3 := points[0], points[1], points[2], points[3]

	// Horizontal then vertical, or vertical then horizontal
	horizontalFirst := (p0.Y == p1.Y) && (p1.X == p2.X) && (p2.Y == p3.Y) && (p3.X == p0.X)
	verticalFirst := (p0.X == p1.X) && (p1.Y == p2.Y) && (p2.X == p3.X) && (p3.Y == p0.Y)

	return horizontalFirst || verticalFirst
}

// String returns a string representation of the graphics element.
func (ge *GraphicsElement) String() string {
	return fmt.Sprintf("GraphicsElement{type=%s, points=%v, color=%s, width=%.2f}",
		ge.Type.String(), ge.Points, ge.Color.String(), ge.Width)
}
