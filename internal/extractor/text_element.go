// Package extractor implements PDF content extraction use cases.
//
// This is the Application layer in DDD/Clean Architecture.
// It orchestrates domain logic and infrastructure for extracting content from PDFs.
package extractor

import (
	"fmt"
	"math"
)

// TextElement represents a single piece of text extracted from a PDF page.
//
// Each TextElement has position information (X, Y coordinates) which is critical
// for table extraction and layout analysis. The coordinates represent the
// bottom-left corner of the text element in PDF coordinate space.
//
// PDF Coordinate System (Section 8.3.2):
//   - Origin (0,0) is at bottom-left of page
//   - X increases to the right
//   - Y increases upward
//   - Coordinates are in points (1 point = 1/72 inch)
//
// Reference: PDF 1.7 specification, Section 9.4 (Text Objects).
type TextElement struct {
	Text     string  // The actual text content
	X        float64 // X coordinate (bottom-left, in points)
	Y        float64 // Y coordinate (bottom-left, in points)
	Width    float64 // Width of text (in points)
	Height   float64 // Height of text (in points)
	FontName string  // Font name (e.g., "/F1", "/Helvetica")
	FontSize float64 // Font size in points

	// preciseWidth records whether Width came from the PDF font metrics rather
	// than the legacy 0.6-em estimate. It intentionally remains internal: it is
	// extraction metadata used to distinguish glyph kerning from word gaps.
	preciseWidth bool
}

// NewTextElement creates a new TextElement with the given properties.
func NewTextElement(text string, x, y, width, height float64, fontName string, fontSize float64) *TextElement {
	return &TextElement{
		Text:     text,
		X:        x,
		Y:        y,
		Width:    width,
		Height:   height,
		FontName: fontName,
		FontSize: fontSize,
	}
}

// Right returns the X coordinate of the right edge of the text.
func (te *TextElement) Right() float64 {
	return te.X + te.Width
}

// Top returns the Y coordinate of the top edge of the text.
func (te *TextElement) Top() float64 {
	return te.Y + te.Height
}

// Bottom returns the Y coordinate of the bottom edge (same as Y).
func (te *TextElement) Bottom() float64 {
	return te.Y
}

// Left returns the X coordinate of the left edge (same as X).
func (te *TextElement) Left() float64 {
	return te.X
}

// CenterX returns the X coordinate of the center of the text.
func (te *TextElement) CenterX() float64 {
	return te.X + te.Width/2
}

// CenterY returns the Y coordinate of the center of the text.
func (te *TextElement) CenterY() float64 {
	return te.Y + te.Height/2
}

// String returns a string representation of the text element.
func (te *TextElement) String() string {
	return fmt.Sprintf("TextElement{text=%q, x=%.2f, y=%.2f, w=%.2f, h=%.2f, font=%s, size=%.1f}",
		te.Text, te.X, te.Y, te.Width, te.Height, te.FontName, te.FontSize)
}

// Rectangle represents a rectangular bounding box.
//
// This is a simplified version for text extraction.
// The full Rectangle value object is in domain/types.
type Rectangle struct {
	X      float64 // Bottom-left X coordinate
	Y      float64 // Bottom-left Y coordinate
	Width  float64 // Width
	Height float64 // Height
}

// NewRectangle creates a new Rectangle.
func NewRectangle(x, y, width, height float64) Rectangle {
	return Rectangle{X: x, Y: y, Width: width, Height: height}
}

// Right returns the X coordinate of the right edge.
func (r Rectangle) Right() float64 {
	return r.X + r.Width
}

// Top returns the Y coordinate of the top edge.
func (r Rectangle) Top() float64 {
	return r.Y + r.Height
}

// Bottom returns the Y coordinate of the bottom edge.
func (r Rectangle) Bottom() float64 {
	return r.Y
}

// Left returns the X coordinate of the left edge.
func (r Rectangle) Left() float64 {
	return r.X
}

// Contains checks if a point (x, y) is inside the rectangle.
func (r Rectangle) Contains(x, y float64) bool {
	return x >= r.X && x <= r.Right() && y >= r.Y && y <= r.Top()
}

// String returns a string representation of the rectangle.
func (r Rectangle) String() string {
	return fmt.Sprintf("Rectangle{x=%.2f, y=%.2f, w=%.2f, h=%.2f}", r.X, r.Y, r.Width, r.Height)
}

// TextChunk represents a group of text elements.
//
// A chunk is used to group related text elements (e.g., text on the same line,
// text in the same cell, text in the same paragraph).
//
// This is useful for table extraction where we need to group text into cells.
type TextChunk struct {
	Elements []*TextElement // Text elements in this chunk
	Bounds   Rectangle      // Bounding box of all elements
}

// NewTextChunk creates a new TextChunk with the given elements.
//
// The bounding box is calculated from the elements.
func NewTextChunk(elements []*TextElement) *TextChunk {
	chunk := &TextChunk{
		Elements: elements,
	}

	if len(elements) > 0 {
		chunk.calculateBounds()
	}

	return chunk
}

// calculateBounds calculates the bounding box from all text elements.
func (tc *TextChunk) calculateBounds() {
	if len(tc.Elements) == 0 {
		return
	}

	// Initialize with first element
	first := tc.Elements[0]
	minX := first.X
	minY := first.Y
	maxX := first.Right()
	maxY := first.Top()

	// Find min/max coordinates
	for _, elem := range tc.Elements[1:] {
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

	tc.Bounds = NewRectangle(minX, minY, maxX-minX, maxY-minY)
}

// Text returns the concatenated text of all elements.
func (tc *TextChunk) Text() string {
	var result string
	for _, elem := range tc.Elements {
		result += elem.Text
	}
	return result
}

// Add adds a text element to the chunk and updates bounds.
func (tc *TextChunk) Add(elem *TextElement) {
	tc.Elements = append(tc.Elements, elem)
	tc.calculateBounds()
}

// Len returns the number of elements in the chunk.
func (tc *TextChunk) Len() int {
	return len(tc.Elements)
}

// String returns a string representation of the chunk.
func (tc *TextChunk) String() string {
	return fmt.Sprintf("TextChunk{elements=%d, text=%q, bounds=%s}",
		len(tc.Elements), tc.Text(), tc.Bounds.String())
}

// VerticalOverlapRatio calculates the vertical overlap ratio between this element and another.
//
// Returns a value between 0.0 (no overlap) and 1.0 (complete overlap).
// Based on Tabula's algorithm (tabula-java/Rectangle.java:73-90).
//
// This is used for row detection in tables without ruling lines (Stream mode).
// Elements with overlap < threshold (e.g., 0.1) are considered separate rows.
func (te *TextElement) VerticalOverlapRatio(other *TextElement) float64 {
	thisBottom := te.Bottom()
	thisTop := te.Top()
	otherBottom := other.Bottom()
	otherTop := other.Top()

	// Delta = minimum height of the two elements
	delta := math.Min(thisTop-thisBottom, otherTop-otherBottom)

	if delta == 0 {
		return 0.0
	}

	var overlapHeight float64

	// Case 1: other starts before this, ends within this
	// other:  [--------]
	// this:       [--------]
	if otherBottom <= thisBottom && thisBottom <= otherTop && otherTop <= thisTop {
		overlapHeight = otherTop - thisBottom
	} else if thisBottom <= otherBottom && otherBottom <= thisTop && thisTop <= otherTop {
		// Case 2: this starts before other, ends within other
		// this:   [--------]
		// other:      [--------]
		overlapHeight = thisTop - otherBottom
	} else if thisBottom <= otherBottom && otherBottom <= otherTop && otherTop <= thisTop {
		// Case 3: other completely within this
		// this:   [------------]
		// other:     [------]
		overlapHeight = otherTop - otherBottom
	} else if otherBottom <= thisBottom && thisBottom <= thisTop && thisTop <= otherTop {
		// Case 4: this completely within other
		// other:  [------------]
		// this:      [------]
		overlapHeight = thisTop - thisBottom
	} else {
		// No overlap
		return 0.0
	}

	return overlapHeight / delta
}
