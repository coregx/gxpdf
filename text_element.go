package gxpdf

// TextElement represents a single text run extracted from a PDF page,
// with full positional and font metadata.
//
// X, Y, Width, and Height describe the axis-aligned bounding box of the
// painted run in transformed PDF page user space (ISO 32000-1, §8.3.2).
// Page and Form matrices are applied exactly once. Coordinates are not
// rebased to MediaBox/CropBox, and page /Rotate remains display metadata.
// All measurements are in points (1 pt = 1/72 inch).
//
// Use Page.ExtractTextElements or Document.ExtractTextElementsFromPage to
// obtain slices of TextElement.
type TextElement struct {
	// Text is the actual string content of this run.
	Text string

	// X is the left edge of the text run in points.
	X float64

	// Y is the bottom edge of the text run in points.
	Y float64

	// Width is the horizontal extent of the text run in points.
	Width float64

	// Height is the vertical extent of the text run in points.
	Height float64

	// FontName is the PDF internal font resource name (e.g. "/F1", "/Helvetica").
	FontName string

	// FontSize is the rendered vertical em size in page-space points.
	FontSize float64
}
