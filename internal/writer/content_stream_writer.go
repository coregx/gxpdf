// Package writer implements PDF writing infrastructure.
package writer

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/coregx/gxpdf/internal/encoding"
)

// ContentStreamWriter builds PDF content streams.
//
// A content stream is a sequence of PDF operators and operands that describe
// page content (text, graphics, images).
//
// Example:
//
//	csw := NewContentStreamWriter()
//	csw.BeginText()
//	csw.SetFont("F1", 12)
//	csw.MoveTextPosition(100, 700)
//	csw.ShowText("Hello World")
//	csw.EndText()
//	content := csw.Bytes()
//
// Reference: PDF 1.7 Specification, Section 8.2 (Content Streams and Resources).
type ContentStreamWriter struct {
	buf         bytes.Buffer
	compression CompressionLevel // Compression level (default: DefaultCompression)
}

// NewContentStreamWriter creates a new content stream writer.
//
// By default, compression is enabled with DefaultCompression level.
// Use SetCompression to change the compression level.
func NewContentStreamWriter() *ContentStreamWriter {
	return &ContentStreamWriter{
		compression: DefaultCompression,
	}
}

// Bytes returns the accumulated content stream data.
func (csw *ContentStreamWriter) Bytes() []byte {
	return csw.buf.Bytes()
}

// String returns the content stream as a string (for debugging).
func (csw *ContentStreamWriter) String() string {
	return csw.buf.String()
}

// Len returns the length of the accumulated content.
func (csw *ContentStreamWriter) Len() int {
	return csw.buf.Len()
}

// Reset clears the content stream buffer.
func (csw *ContentStreamWriter) Reset() {
	csw.buf.Reset()
}

// writeOp writes an operator with optional operands.
func (csw *ContentStreamWriter) writeOp(operands string, operator string) {
	if operands != "" {
		csw.buf.WriteString(operands)
		csw.buf.WriteString(" ")
	}
	csw.buf.WriteString(operator)
	csw.buf.WriteString("\n")
}

// --- TEXT OPERATORS ---

// BeginText begins a text object (BT operator).
//
// Reference: PDF 1.7 Spec, Section 9.4 (Text Objects).
func (csw *ContentStreamWriter) BeginText() {
	csw.writeOp("", "BT")
}

// EndText ends a text object (ET operator).
//
// Reference: PDF 1.7 Spec, Section 9.4 (Text Objects).
func (csw *ContentStreamWriter) EndText() {
	csw.writeOp("", "ET")
}

// SetFont sets the text font and size (Tf operator).
//
// Parameters:
//   - fontName: Font resource name (e.g., "F1")
//   - size: Font size in points
//
// Reference: PDF 1.7 Spec, Section 9.3 (Text State Parameters and Operators).
func (csw *ContentStreamWriter) SetFont(fontName string, size float64) {
	csw.writeOp(fmt.Sprintf("/%s %.2f", fontName, size), "Tf")
}

// MoveTextPosition moves to the start of the next line (Td operator).
//
// Parameters:
//   - tx: Horizontal translation
//   - ty: Vertical translation
//
// Reference: PDF 1.7 Spec, Section 9.4.2 (Text-Positioning Operators).
func (csw *ContentStreamWriter) MoveTextPosition(tx, ty float64) {
	csw.writeOp(fmt.Sprintf("%.2f %.2f", tx, ty), "Td")
}

// MoveTextPositionSetLeading moves to next line and sets leading (TD operator).
//
// Parameters:
//   - tx: Horizontal translation
//   - ty: Vertical translation (also sets leading to -ty)
//
// Reference: PDF 1.7 Spec, Section 9.4.2 (Text-Positioning Operators).
func (csw *ContentStreamWriter) MoveTextPositionSetLeading(tx, ty float64) {
	csw.writeOp(fmt.Sprintf("%.2f %.2f", tx, ty), "TD")
}

// SetTextMatrix sets the text matrix (Tm operator).
//
// The text matrix determines text positioning and scaling.
//
// Parameters:
//   - a, b, c, d: Matrix coefficients for scaling/rotation
//   - e, f: Translation (horizontal, vertical)
//
// Reference: PDF 1.7 Spec, Section 9.4.2 (Text-Positioning Operators).
func (csw *ContentStreamWriter) SetTextMatrix(a, b, c, d, e, f float64) {
	csw.writeOp(fmt.Sprintf("%.2f %.2f %.2f %.2f %.2f %.2f", a, b, c, d, e, f), "Tm")
}

// ShowText shows a text string (Tj operator).
//
// Parameters:
//   - text: Text to display (will be escaped)
//
// Reference: PDF 1.7 Spec, Section 9.4.3 (Text-Showing Operators).
func (csw *ContentStreamWriter) ShowText(text string) {
	escaped := EscapePDFString(text)
	csw.writeOp(fmt.Sprintf("(%s)", escaped), "Tj")
}

// ShowTextEncoded shows pre-encoded text using hex string (Tj operator).
//
// This is used for embedded fonts where the text is already encoded
// as a hex string (e.g., "<0048006500>").
//
// Parameters:
//   - encodedText: Hex-encoded string including angle brackets
//
// Reference: PDF 1.7 Spec, Section 9.4.3 (Text-Showing Operators).
func (csw *ContentStreamWriter) ShowTextEncoded(encodedText string) {
	// encodedText is already in the format "<XXXX>" so use directly.
	csw.writeOp(encodedText, "Tj")
}

// ShowTextNextLine moves to next line and shows text (' operator).
//
// Equivalent to: T* followed by Tj.
//
// Parameters:
//   - text: Text to display (will be escaped)
//
// Reference: PDF 1.7 Spec, Section 9.4.3 (Text-Showing Operators).
func (csw *ContentStreamWriter) ShowTextNextLine(text string) {
	escaped := EscapePDFString(text)
	csw.writeOp(fmt.Sprintf("(%s)", escaped), "'")
}

// SetLeading sets the text leading (TL operator).
//
// Leading is the vertical distance between text lines.
//
// Parameters:
//   - leading: Leading value in text space units
//
// Reference: PDF 1.7 Spec, Section 9.3.5 (Text State Parameters).
func (csw *ContentStreamWriter) SetLeading(leading float64) {
	csw.writeOp(fmt.Sprintf("%.2f", leading), "TL")
}

// MoveToNextLine moves to the start of the next line (T* operator).
//
// Reference: PDF 1.7 Spec, Section 9.4.2 (Text-Positioning Operators).
func (csw *ContentStreamWriter) MoveToNextLine() {
	csw.writeOp("", "T*")
}

// --- GRAPHICS OPERATORS ---

// MoveTo begins a new subpath (m operator).
//
// Parameters:
//   - x, y: Starting point coordinates
//
// Reference: PDF 1.7 Spec, Section 8.5.2 (Path Construction Operators).
func (csw *ContentStreamWriter) MoveTo(x, y float64) {
	csw.writeOp(fmt.Sprintf("%.2f %.2f", x, y), "m")
}

// LineTo appends a straight line segment (l operator).
//
// Parameters:
//   - x, y: End point coordinates
//
// Reference: PDF 1.7 Spec, Section 8.5.2 (Path Construction Operators).
func (csw *ContentStreamWriter) LineTo(x, y float64) {
	csw.writeOp(fmt.Sprintf("%.2f %.2f", x, y), "l")
}

// CurveTo appends a cubic Bezier curve (c operator).
//
// Parameters:
//   - x1, y1: First control point
//   - x2, y2: Second control point
//   - x3, y3: End point
//
// Reference: PDF 1.7 Spec, Section 8.5.2 (Path Construction Operators).
func (csw *ContentStreamWriter) CurveTo(x1, y1, x2, y2, x3, y3 float64) {
	csw.writeOp(fmt.Sprintf("%.2f %.2f %.2f %.2f %.2f %.2f", x1, y1, x2, y2, x3, y3), "c")
}

// Rectangle appends a rectangle (re operator).
//
// Parameters:
//   - x, y: Lower-left corner
//   - width, height: Rectangle dimensions
//
// Reference: PDF 1.7 Spec, Section 8.5.2 (Path Construction Operators).
func (csw *ContentStreamWriter) Rectangle(x, y, width, height float64) {
	csw.writeOp(fmt.Sprintf("%.2f %.2f %.2f %.2f", x, y, width, height), "re")
}

// ClosePath closes the current subpath (h operator).
//
// Reference: PDF 1.7 Spec, Section 8.5.2 (Path Construction Operators).
func (csw *ContentStreamWriter) ClosePath() {
	csw.writeOp("", "h")
}

// Stroke strokes the path (S operator).
//
// Reference: PDF 1.7 Spec, Section 8.5.3 (Path-Painting Operators).
func (csw *ContentStreamWriter) Stroke() {
	csw.writeOp("", "S")
}

// CloseAndStroke closes and strokes the path (s operator).
//
// Reference: PDF 1.7 Spec, Section 8.5.3 (Path-Painting Operators).
func (csw *ContentStreamWriter) CloseAndStroke() {
	csw.writeOp("", "s")
}

// Fill fills the path (f operator).
//
// Uses nonzero winding number rule.
//
// Reference: PDF 1.7 Spec, Section 8.5.3 (Path-Painting Operators).
func (csw *ContentStreamWriter) Fill() {
	csw.writeOp("", "f")
}

// FillEvenOdd fills the path using even-odd rule (f* operator).
//
// Reference: PDF 1.7 Spec, Section 8.5.3 (Path-Painting Operators).
func (csw *ContentStreamWriter) FillEvenOdd() {
	csw.writeOp("", "f*")
}

// FillAndStroke fills and strokes the path (B operator).
//
// Uses nonzero winding number rule.
//
// Reference: PDF 1.7 Spec, Section 8.5.3 (Path-Painting Operators).
func (csw *ContentStreamWriter) FillAndStroke() {
	csw.writeOp("", "B")
}

// FillAndStrokeEvenOdd fills and strokes using even-odd rule (B* operator).
//
// Reference: PDF 1.7 Spec, Section 8.5.3 (Path-Painting Operators).
func (csw *ContentStreamWriter) FillAndStrokeEvenOdd() {
	csw.writeOp("", "B*")
}

// EndPath ends the path without filling or stroking (n operator).
//
// Reference: PDF 1.7 Spec, Section 8.5.3 (Path-Painting Operators).
func (csw *ContentStreamWriter) EndPath() {
	csw.writeOp("", "n")
}

// Clip sets the clipping path using nonzero winding rule (W operator).
//
// Must be called after defining a path (e.g., Rectangle) and before EndPath.
// Subsequent drawing operations will be clipped to this path.
//
// Example:
//
//	csw.SaveState()
//	csw.Rectangle(x, y, w, h)
//	csw.Clip()
//	csw.EndPath()
//	// ... draw content that will be clipped ...
//	csw.RestoreState()
//
// Reference: PDF 1.7 Spec, Section 8.5.4 (Clipping Path Operators).
func (csw *ContentStreamWriter) Clip() {
	csw.writeOp("", "W")
}

// ClipEvenOdd sets the clipping path using even-odd rule (W* operator).
//
// Reference: PDF 1.7 Spec, Section 8.5.4 (Clipping Path Operators).
func (csw *ContentStreamWriter) ClipEvenOdd() {
	csw.writeOp("", "W*")
}

// --- GRAPHICS STATE OPERATORS ---

// SaveState saves the graphics state (q operator).
//
// Reference: PDF 1.7 Spec, Section 8.4.2 (Graphics State Stack).
func (csw *ContentStreamWriter) SaveState() {
	csw.writeOp("", "q")
}

// RestoreState restores the graphics state (Q operator).
//
// Reference: PDF 1.7 Spec, Section 8.4.2 (Graphics State Stack).
func (csw *ContentStreamWriter) RestoreState() {
	csw.writeOp("", "Q")
}

// ConcatMatrix modifies the current transformation matrix (cm operator).
//
// Parameters:
//   - a, b, c, d, e, f: Matrix coefficients
//
// Reference: PDF 1.7 Spec, Section 8.4.4 (Coordinate Systems).
func (csw *ContentStreamWriter) ConcatMatrix(a, b, c, d, e, f float64) {
	csw.writeOp(fmt.Sprintf("%.2f %.2f %.2f %.2f %.2f %.2f", a, b, c, d, e, f), "cm")
}

// SetLineWidth sets the line width (w operator).
//
// Parameters:
//   - width: Line width in user space units
//
// Reference: PDF 1.7 Spec, Section 8.4.3 (Graphics State Parameters).
func (csw *ContentStreamWriter) SetLineWidth(width float64) {
	csw.writeOp(fmt.Sprintf("%.2f", width), "w")
}

// SetLineCap sets the line cap style (J operator).
//
// Parameters:
//   - style: 0 = butt cap, 1 = round cap, 2 = projecting square cap
//
// Reference: PDF 1.7 Spec, Section 8.4.3 (Graphics State Parameters).
func (csw *ContentStreamWriter) SetLineCap(style int) {
	csw.writeOp(fmt.Sprintf("%d", style), "J")
}

// SetLineJoin sets the line join style (j operator).
//
// Parameters:
//   - style: 0 = miter join, 1 = round join, 2 = bevel join
//
// Reference: PDF 1.7 Spec, Section 8.4.3 (Graphics State Parameters).
func (csw *ContentStreamWriter) SetLineJoin(style int) {
	csw.writeOp(fmt.Sprintf("%d", style), "j")
}

// SetMiterLimit sets the miter limit (M operator).
//
// Parameters:
//   - limit: Maximum ratio of miter length to line width
//
// Reference: PDF 1.7 Spec, Section 8.4.3 (Graphics State Parameters).
func (csw *ContentStreamWriter) SetMiterLimit(limit float64) {
	csw.writeOp(fmt.Sprintf("%.2f", limit), "M")
}

// SetDashPattern sets the line dash pattern (d operator).
//
// Parameters:
//   - dashArray: Array of dash and gap lengths
//   - dashPhase: Starting offset into the pattern
//
// Reference: PDF 1.7 Spec, Section 8.4.3 (Graphics State Parameters).
func (csw *ContentStreamWriter) SetDashPattern(dashArray []float64, dashPhase float64) {
	parts := make([]string, 0, len(dashArray))
	for _, v := range dashArray {
		parts = append(parts, fmt.Sprintf("%.2f", v))
	}
	csw.writeOp(fmt.Sprintf("[%s] %.2f", strings.Join(parts, " "), dashPhase), "d")
}

// SetStrokeColorRGB sets the stroke color in RGB (RG operator).
//
// Parameters:
//   - r, g, b: RGB values (0.0 to 1.0)
//
// Reference: PDF 1.7 Spec, Section 8.6.8 (Color Operators).
func (csw *ContentStreamWriter) SetStrokeColorRGB(r, g, b float64) {
	csw.writeOp(fmt.Sprintf("%.2f %.2f %.2f", r, g, b), "RG")
}

// SetFillColorRGB sets the fill color in RGB (rg operator).
//
// Parameters:
//   - r, g, b: RGB values (0.0 to 1.0)
//
// Reference: PDF 1.7 Spec, Section 8.6.8 (Color Operators).
func (csw *ContentStreamWriter) SetFillColorRGB(r, g, b float64) {
	csw.writeOp(fmt.Sprintf("%.2f %.2f %.2f", r, g, b), "rg")
}

// SetStrokeColorGray sets the stroke color in grayscale (G operator).
//
// Parameters:
//   - gray: Grayscale value (0.0 = black, 1.0 = white)
//
// Reference: PDF 1.7 Spec, Section 8.6.8 (Color Operators).
func (csw *ContentStreamWriter) SetStrokeColorGray(gray float64) {
	csw.writeOp(fmt.Sprintf("%.2f", gray), "G")
}

// SetFillColorGray sets the fill color in grayscale (g operator).
//
// Parameters:
//   - gray: Grayscale value (0.0 = black, 1.0 = white)
//
// Reference: PDF 1.7 Spec, Section 8.6.8 (Color Operators).
func (csw *ContentStreamWriter) SetFillColorGray(gray float64) {
	csw.writeOp(fmt.Sprintf("%.2f", gray), "g")
}

// SetStrokeColorCMYK sets the stroke color in CMYK (K operator).
//
// Parameters:
//   - c, m, y, k: CMYK values (0.0 to 1.0)
//
// Reference: PDF 1.7 Spec, Section 8.6.8 (Color Operators).
func (csw *ContentStreamWriter) SetStrokeColorCMYK(c, m, y, k float64) {
	csw.writeOp(fmt.Sprintf("%.2f %.2f %.2f %.2f", c, m, y, k), "K")
}

// SetFillColorCMYK sets the fill color in CMYK (k operator).
//
// Parameters:
//   - c, m, y, k: CMYK values (0.0 to 1.0)
//
// Reference: PDF 1.7 Spec, Section 8.6.8 (Color Operators).
func (csw *ContentStreamWriter) SetFillColorCMYK(c, m, y, k float64) {
	csw.writeOp(fmt.Sprintf("%.2f %.2f %.2f %.2f", c, m, y, k), "k")
}

// SetGraphicsState applies an extended graphics state (gs operator).
//
// ExtGState (Extended Graphics State) is used to set advanced graphics
// parameters like transparency (opacity), blend modes, and rendering intent.
//
// Parameters:
//   - name: Graphics state resource name (e.g., "GS1")
//
// Example:
//
//	csw.SaveState()
//	csw.SetGraphicsState("GS1")  // Apply transparency or other state
//	// ... draw content with applied state ...
//	csw.RestoreState()
//
// Reference: PDF 1.7 Spec, Section 8.4.5 (Extended Graphics State).
func (csw *ContentStreamWriter) SetGraphicsState(name string) {
	csw.writeOp(fmt.Sprintf("/%s", name), "gs")
}

// --- COMPRESSION ---

// SetCompression sets the compression level for this content stream.
//
// Parameters:
//   - level: Compression level (NoCompression, BestSpeed, DefaultCompression, BestCompression)
//
// This affects the output of CompressedBytes() method.
//
// Example:
//
//	csw := NewContentStreamWriter()
//	csw.SetCompression(BestCompression)  // Maximum compression
//	// ... add content ...
//	compressed, _ := csw.CompressedBytes()
func (csw *ContentStreamWriter) SetCompression(level CompressionLevel) {
	csw.compression = level
}

// GetCompression returns the current compression level.
func (csw *ContentStreamWriter) GetCompression() CompressionLevel {
	return csw.compression
}

// IsCompressed returns true if compression is enabled.
//
// Compression is disabled when level is NoCompression.
func (csw *ContentStreamWriter) IsCompressed() bool {
	return csw.compression != NoCompression
}

// CompressedBytes returns the content stream compressed using the configured compression level.
//
// If compression is disabled (NoCompression), returns uncompressed bytes.
//
// Returns:
//   - compressed: Compressed bytes (or uncompressed if NoCompression)
//   - error: Any compression error
//
// Example:
//
//	csw := NewContentStreamWriter()
//	csw.BeginText()
//	csw.ShowText("Hello")
//	csw.EndText()
//	compressed, err := csw.CompressedBytes()
func (csw *ContentStreamWriter) CompressedBytes() ([]byte, error) {
	data := csw.Bytes()

	// If compression disabled, return uncompressed
	if csw.compression == NoCompression {
		return data, nil
	}

	// Compress using configured level
	return CompressStream(data, csw.compression)
}

// Compress compresses the content stream using Flate encoding.
//
// Deprecated: Use CompressedBytes() instead, which uses the configured compression level.
//
// Returns compressed bytes or error.
func (csw *ContentStreamWriter) Compress() ([]byte, error) {
	encoder := encoding.NewFlateDecoder() // FlateDecoder has Encode method
	return encoder.Encode(csw.Bytes())
}

// --- HELPERS ---
// String escaping is now in string_escape.go (EscapePDFString function).
