package extractor

// CoordinateNormalizer detects and aligns coordinate spaces between text
// elements and graphics elements on a PDF page.
//
// PDF pages can use different coordinate systems depending on the CTM
// (Current Transformation Matrix) set up by the content stream. Common
// patterns:
//
//   - Bottom-left origin (PDF native): Y=0 at bottom, increases upward
//   - Top-left origin (wkhtmltopdf, Chrome): Y=0 at top, increases downward
//     (achieved via "1 0 0 -1 0 pageHeight cm" in the content stream)
//
// TextExtractor uses raw Tm/Td positions without applying CTM.
// GraphicsParser applies CTM and normalises Y via normalizeCoordinates().
// This can put text and graphics in different coordinate spaces.
//
// CoordinateNormalizer detects the mismatch and transforms text elements
// to match graphics space, ensuring CellExtractor finds text within grid cells.
//
// Detection algorithm:
//  1. Compute Y centroids of text elements and grid cells
//  2. If they overlap (centroid within page bounds) → same space, no transform
//  3. If they don't overlap → flip text Y: Y' = pageHeight - Y
type CoordinateNormalizer struct {
	pageHeight float64
}

// NewCoordinateNormalizer creates a normalizer for a page with the given height.
func NewCoordinateNormalizer(pageHeight float64) *CoordinateNormalizer {
	return &CoordinateNormalizer{pageHeight: pageHeight}
}

// NormalizeTextToGridSpace detects whether text elements and grid bounds are
// in the same coordinate space. If not, it returns a new slice of TextElements
// with Y coordinates flipped: Y' = pageHeight - Y.
//
// Detection: compute how many text elements fall inside the grid bounds with
// original coordinates vs flipped coordinates. If flipped produces more hits,
// the text needs transformation.
//
// The original slice is not modified; a new slice with copied elements is returned.
func (cn *CoordinateNormalizer) NormalizeTextToGridSpace(
	textElements []*TextElement,
	gridMinY, gridMaxY float64,
) []*TextElement {
	if cn.pageHeight <= 0 || len(textElements) == 0 {
		return textElements
	}

	// Count text elements inside grid bounds in both spaces.
	originalHits := 0
	flippedHits := 0
	for _, e := range textElements {
		if e.Y >= gridMinY && e.Y <= gridMaxY {
			originalHits++
		}
		flippedY := cn.pageHeight - e.Y
		if flippedY >= gridMinY && flippedY <= gridMaxY {
			flippedHits++
		}
	}

	if originalHits >= flippedHits {
		return textElements
	}

	// Flipped space produces more hits — transform all text coordinates.
	normalized := make([]*TextElement, len(textElements))
	for i, e := range textElements {
		ne := *e
		ne.Y = cn.pageHeight - e.Y
		normalized[i] = &ne
	}
	return normalized
}
