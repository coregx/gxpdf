// Package extractor implements PDF content extraction use cases.
package extractor

import (
	"sort"
	"strings"
)

// CellExtractor extracts text content from a rectangular cell region.
//
// The extractor is stateful: it tracks which text elements have been
// assigned to a cell. Each element is assigned to at most one cell
// (first-come-first-served based on extraction order). This prevents
// multi-line text from bleeding into adjacent grid cells.
//
// This is a critical component for table extraction (Phase 2.7).
type CellExtractor struct {
	textElements []*TextElement
	assigned     map[*TextElement]bool
	// gridRows stores Y coordinates of grid row boundaries (sorted ascending).
	// Used for grid-aware visual grouping: elements are only grouped if
	// they don't cross a grid row boundary. This prevents grouping sections
	// from different courses that happen to be at similar X/Y positions.
	// nil = no grid awareness (legacy behavior).
	gridRows []float64
}

// NewCellExtractor creates a new CellExtractor with the given text elements.
func NewCellExtractor(textElements []*TextElement) *CellExtractor {
	return &CellExtractor{
		textElements: textElements,
		assigned:     make(map[*TextElement]bool),
	}
}

// WithGridRows sets grid row boundaries for grid-aware visual grouping.
// When set, FindElementsInBounds will include adjacent-line elements
// that belong to the same visual block, but only if they don't cross
// a grid row boundary.
func (ce *CellExtractor) WithGridRows(rows []float64) *CellExtractor {
	ce.gridRows = rows
	return ce
}

// ExtractCellContent extracts text from a rectangular region (cell bounds).
//
// Algorithm:
//  1. Find all text elements within the cell bounds
//  2. Group text elements by line (based on Y position)
//  3. Sort lines from top to bottom
//  4. Within each line, sort elements left to right
//  5. Join text with appropriate spacing
//
// Parameters:
//   - bounds: The rectangular region to extract text from
//
// Returns the extracted text, or empty string if no text is found.
func (ce *CellExtractor) ExtractCellContent(bounds Rectangle) string {
	elementsInCell := ce.FindElementsInBounds(bounds)
	if len(elementsInCell) == 0 {
		return ""
	}

	// Mark elements as assigned so they won't appear in adjacent cells.
	ce.MarkAssigned(elementsInCell)

	lines := ce.groupByLine(elementsInCell)
	ce.sortLines(lines)
	return ce.buildTextFromLines(lines)
}

// cellBoundsPadding is added to cell bounds when checking text element
// containment. Text positioned near grid lines (within a few points) may
// have its center point outside the strict cell rectangle due to coordinate
// transformation precision, font metrics, or baseline vs bounding-box
// differences. A 2pt padding matches the grid builder tolerance.
const cellBoundsPadding = 2.0

// FindElementsInBounds returns all text elements that are within the bounds.
//
// An element is considered "within" if its center point is inside the bounds
// expanded by cellBoundsPadding on all sides. This handles coordinate
// precision issues at grid boundaries without pulling in text from adjacent cells
// (grid rows are typically 10-15pt apart).
//
// This method is exported for use by other extractors (e.g., table alignment detection).
func (ce *CellExtractor) FindElementsInBounds(bounds Rectangle) []*TextElement {
	var result []*TextElement

	expanded := NewRectangle(
		bounds.X-cellBoundsPadding,
		bounds.Y-cellBoundsPadding,
		bounds.Width+2*cellBoundsPadding,
		bounds.Height+2*cellBoundsPadding,
	)

	for _, elem := range ce.textElements {
		if ce.assigned[elem] {
			continue
		}
		if expanded.Contains(elem.CenterX(), elem.CenterY()) {
			result = append(result, elem)
		}
	}

	// Grid-aware continuation: only expand when initial text looks
	// truncated (ends with comma = incomplete section list).
	// This avoids regressions from pulling adjacent course sections.
	if len(ce.gridRows) > 0 && len(result) > 0 {
		text := ce.buildTextFromFoundElements(result)
		if strings.HasSuffix(strings.TrimSpace(text), ",") {
			result = ce.expandWithContinuations(result, bounds)
		}
	}

	return result
}

// expandWithContinuations adds text elements that are visually connected
// to already-found elements but outside cell bounds. Only expands within
// the same grid row band (between two consecutive grid row Y coordinates).
func (ce *CellExtractor) expandWithContinuations(found []*TextElement, bounds Rectangle) []*TextElement {
	foundSet := make(map[*TextElement]bool, len(found))
	for _, e := range found {
		foundSet[e] = true
	}

	// Determine the grid row band for these elements
	gridBandMin, gridBandMax := ce.findGridBand(bounds.Y, bounds.Y+bounds.Height)

	var added []*TextElement
	for _, elem := range ce.textElements {
		if ce.assigned[elem] || foundSet[elem] {
			continue
		}
		// Must be STRICTLY within grid band — no padding
		if elem.CenterY() < gridBandMin || elem.CenterY() > gridBandMax {
			continue
		}
		// Must be in same X column (cell bounds only, minimal padding)
		if elem.CenterX() < bounds.X-1 || elem.CenterX() > bounds.X+bounds.Width+1 {
			continue
		}
		// Must be visually connected to a found element
		if ce.isVisualContinuation(elem, found) {
			added = append(added, elem)
		}
	}

	return append(found, added...)
}

// findGridBand returns the Y range of the grid row band that contains
// the given Y range. Grid rows define bands between consecutive Y values.
func (ce *CellExtractor) findGridBand(yMin, yMax float64) (float64, float64) {
	if len(ce.gridRows) < 2 {
		return yMin, yMax
	}
	bandMin := ce.gridRows[0]
	bandMax := ce.gridRows[len(ce.gridRows)-1]
	for i := 0; i < len(ce.gridRows)-1; i++ {
		if yMin >= ce.gridRows[i]-cellBoundsPadding && yMin <= ce.gridRows[i+1]+cellBoundsPadding {
			bandMin = ce.gridRows[i]
			bandMax = ce.gridRows[i+1]
			break
		}
	}
	return bandMin, bandMax
}

// isVisualContinuation checks if elem is a visual continuation of any
// found element. Strict criteria to prevent pulling course titles from
// adjacent rows:
//   - Same X start (within 3pt)
//   - Adjacent Y (within 1.5× fontSize)
//   - Same font size
//   - Text must look like section codes (short, comma-separated),
//     NOT a course title (multi-word ALL CAPS)
func (ce *CellExtractor) isVisualContinuation(elem *TextElement, found []*TextElement) bool {
	// Don't pull course-title-like text as continuation
	text := strings.TrimSpace(elem.Text)
	if isCourseTitle(text) {
		return false
	}

	for _, f := range found {
		if abs(elem.FontSize-f.FontSize) > 1.0 {
			continue
		}
		if abs(elem.X-f.X) > 3.0 {
			continue
		}
		yDist := abs(elem.Y - f.Y)
		maxDist := f.FontSize * 1.5
		if maxDist < 12 {
			maxDist = 12
		}
		if yDist > 0.5 && yDist <= maxDist {
			return true
		}
	}
	return false
}

// buildTextFromFoundElements quickly joins found element text for truncation check.
func (ce *CellExtractor) buildTextFromFoundElements(elements []*TextElement) string {
	var sb strings.Builder
	for _, e := range elements {
		sb.WriteString(e.Text)
	}
	return sb.String()
}

// isCourseTitle checks if text looks like a course title (multi-word ALL CAPS,
// length > 10 chars, contains spaces). Used to prevent pulling titles as
// visual continuations.
func isCourseTitle(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 5 {
		return false
	}
	if !strings.ContainsRune(s, ' ') {
		return false
	}
	// ALL CAPS with no commas → likely course title
	if s == strings.ToUpper(s) && !strings.ContainsRune(s, ',') {
		return true
	}
	return false
}

// MarkAssigned marks the given text elements as assigned to a cell.
// Once assigned, elements will not appear in subsequent FindElementsInBounds calls.
func (ce *CellExtractor) MarkAssigned(elements []*TextElement) {
	for _, e := range elements {
		ce.assigned[e] = true
	}
}

// textLine represents a line of text elements at the same Y position.
type textLine struct {
	y        float64        // Average Y position of line (for final sorting)
	minY     float64        // Minimum Y in line
	maxY     float64        // Maximum Y in line
	elements []*TextElement // Elements in this line
}

// groupByLine groups text elements into lines based on Y position.
//
// Elements are considered on the same line if their Y positions are
// within a threshold.
//
// Threshold = 1.5x average font size:
//   - For 10pt font: 15pt tolerance
//   - This accommodates multi-line cells (typical line spacing = 1.2-1.5x font size)
//   - Previous threshold (0.5x) worked for Alfa-Bank (single-line cells)
//     but failed for VTB (multi-line cells with 12-15pt spacing)
//
// See: ANALYSIS_VTB_TABLE_MULTI_LINE_CELLS.md for detailed analysis
func (ce *CellExtractor) groupByLine(elements []*TextElement) []*textLine {
	if len(elements) == 0 {
		return nil
	}

	// Calculate average font size for threshold
	avgFontSize := ce.calculateAverageFontSize(elements)
	threshold := avgFontSize * 0.3 // Group elements on same baseline (< 3pt for 10pt font, tighter for Alfa-Bank)

	// Group elements by line
	var lines []*textLine

	for _, elem := range elements {
		// Find line with similar Y position
		// Check if element Y is within threshold of the line's Y range [minY, maxY]
		var targetLine *textLine
		for _, line := range lines {
			// Check if element is within threshold of existing line
			// Use the closest edge of the line's Y range
			minDist := abs(elem.Y - line.minY)
			maxDist := abs(elem.Y - line.maxY)
			closestDist := minDist
			if maxDist < minDist {
				closestDist = maxDist
			}

			if closestDist < threshold {
				targetLine = line
				break
			}
		}

		// Create new line if not found
		if targetLine == nil {
			targetLine = &textLine{
				y:        elem.Y,
				minY:     elem.Y,
				maxY:     elem.Y,
				elements: []*TextElement{},
			}
			lines = append(lines, targetLine)
		}

		// Add element to line
		targetLine.elements = append(targetLine.elements, elem)

		// Update line Y range and average
		if elem.Y < targetLine.minY {
			targetLine.minY = elem.Y
		}
		if elem.Y > targetLine.maxY {
			targetLine.maxY = elem.Y
		}

		// Update average Y for sorting
		sum := 0.0
		for _, e := range targetLine.elements {
			sum += e.Y
		}
		targetLine.y = sum / float64(len(targetLine.elements))
	}

	return lines
}

// sortLines sorts lines from top to bottom (descending Y).
//
// PDF coordinates have Y increasing upward, so higher Y means higher on page.
func (ce *CellExtractor) sortLines(lines []*textLine) {
	sort.Slice(lines, func(i, j int) bool {
		return lines[i].y > lines[j].y // Top to bottom
	})

	// Sort elements within each line left to right
	for _, line := range lines {
		sort.Slice(line.elements, func(i, j int) bool {
			return line.elements[i].X < line.elements[j].X // Left to right
		})
	}
}

// buildTextFromLines constructs the final text from sorted lines.
//
// Text elements within a line are joined with spaces.
// Lines are joined with newlines.
func (ce *CellExtractor) buildTextFromLines(lines []*textLine) string {
	var result strings.Builder

	for i, line := range lines {
		// Add newline between lines
		if i > 0 {
			result.WriteString("\n")
		}

		// Join elements in line with spaces
		for j, elem := range line.elements {
			if j > 0 {
				// Add space if elements are not immediately adjacent
				prevElem := line.elements[j-1]
				gap := elem.X - prevElem.Right()
				if gap > 2.0 { // Threshold: 2 points
					result.WriteString(" ")
				}
			}
			result.WriteString(elem.Text)
		}
	}

	return strings.TrimSpace(result.String())
}

// calculateAverageFontSize calculates the average font size of elements.
func (ce *CellExtractor) calculateAverageFontSize(elements []*TextElement) float64 {
	if len(elements) == 0 {
		return 12.0 // Default
	}

	sum := 0.0
	for _, elem := range elements {
		sum += elem.FontSize
	}
	return sum / float64(len(elements))
}

// abs returns the absolute value of x.
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
