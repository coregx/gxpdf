package writer

import (
	"bytes"
	"fmt"
	"math"

	"github.com/coregx/gxpdf/internal/fonts"
)

// PageContent represents the content and resources for a single page.
//
// This structure bridges the Creator API (which tracks text operations)
// with the Writer infrastructure (which generates PDF bytes).
type PageContent struct {
	// TextOperations are the text drawing operations for this page.
	TextOperations []TextOp

	// GraphicsOperations are the graphics drawing operations for this page.
	GraphicsOperations []GraphicsOp

	// Resources tracks fonts and other resources used on this page.
	Resources *ResourceDictionary
}

// TextOp represents a text drawing operation.
//
// This is an infrastructure-level representation of text operations
// from the creator package.
type TextOp struct {
	Text      string  // Text to display
	X         float64 // Horizontal position (points from left)
	Y         float64 // Vertical position (points from bottom)
	Font      string  // Font name (e.g., "Helvetica") - for Standard14 fonts
	Size      float64 // Font size in points
	Color     RGB     // Text color (RGB)
	ColorCMYK *CMYK   // Text color (CMYK, optional - takes precedence over RGB)

	// CustomFont is an embedded TrueType/OpenType font (optional).
	// When set, this takes precedence over the Font field.
	// The font must be registered with the document before use.
	CustomFont *EmbeddedFont
}

// EmbeddedFont represents a custom TrueType/OpenType font for embedding.
//
// This is used internally to pass font data from Creator to Writer.
type EmbeddedFont struct {
	// TTF is the parsed TrueType font data.
	TTF *fonts.TTFFont

	// Subset is the font subset containing only used glyphs.
	Subset *fonts.FontSubset

	// ID is a unique identifier for this font instance.
	ID string
}

// RGB represents an RGB color (0.0 to 1.0 range).
type RGB struct {
	R float64
	G float64
	B float64
}

// CMYK represents a CMYK color (0.0 to 1.0 range).
type CMYK struct {
	C float64 // Cyan
	M float64 // Magenta
	Y float64 // Yellow
	K float64 // blacK
}

// Point represents a 2D point.
type Point struct {
	X float64
	Y float64
}

// BezierSegment represents a cubic Bézier curve segment.
type BezierSegment struct {
	Start Point
	C1    Point
	C2    Point
	End   Point
}

// ImageData represents image data for embedding in PDF.
type ImageData struct {
	Data             []byte // Raw image data (JPEG bytes or compressed PNG pixels)
	AlphaMask        []byte // Alpha mask data for PNG with transparency
	Width            int    // Image width in pixels
	Height           int    // Image height in pixels
	ColorSpace       string // Color space: "DeviceRGB", "DeviceCMYK", "DeviceGray"
	Format           string // Image format: "jpeg" or "png"
	BitsPerComponent int    // Bits per component (usually 8)
}

// GraphicsOp represents a graphics drawing operation.
//
// This is an infrastructure-level representation of graphics operations
// from the creator package.
type GraphicsOp struct {
	Type int // 0=line, 1=rect, 2=circle, 3=image, 4=watermark, 5=polygon, 6=polyline, 7=ellipse, 8=bezier

	// Common fields
	X float64
	Y float64

	// Line fields
	X2 float64
	Y2 float64

	// Rectangle fields
	Width  float64
	Height float64

	// Circle fields
	Radius float64

	// Ellipse fields
	RX float64 // Horizontal radius
	RY float64 // Vertical radius

	// Polygon/Polyline fields
	Vertices []Point

	// Bezier fields
	BezierSegs []BezierSegment
	Closed     bool // For Bezier curves

	// Image fields (for Type == 3)
	Image *ImageData

	// Appearance
	StrokeColor     *RGB
	StrokeColorCMYK *CMYK // If set, takes precedence over StrokeColor
	FillColor       *RGB
	FillColorCMYK   *CMYK       // If set, takes precedence over FillColor
	FillGradient    *GradientOp // Gradient fill
	StrokeWidth     float64
	Dashed          bool
	DashArray       []float64
	DashPhase       float64

	// Clipping
	IsClipPath bool // If true, this shape defines a clipping path (not drawn)

	// TextBlock fields (for Type == 22)
	Text       string
	TextFont   *EmbeddedFont
	TextSize   float64
	TextColorR float64
	TextColorG float64
	TextColorB float64

	// Watermark fields (for Type == 4)
	// Text, TextSize, TextColorR/G/B are reused
	WatermarkFont     string  // Font name (Standard14)
	WatermarkOpacity  float64 // Opacity (0.0-1.0)
	WatermarkRotation float64 // Rotation in degrees
}

// ClipOp represents a clipping operation (begin or end).
type ClipOp struct {
	Type   int // 0 = BeginClip, 1 = EndClip
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// GradientType represents the type of gradient.
type GradientType int

const (
	// GradientTypeLinear is an axial gradient (ShadingType 2).
	GradientTypeLinear GradientType = 2
	// GradientTypeRadial is a radial gradient (ShadingType 3).
	GradientTypeRadial GradientType = 3
)

// ColorStopOp represents a color stop in a gradient.
type ColorStopOp struct {
	Position float64
	Color    RGB
}

// GradientOp represents a gradient fill operation.
type GradientOp struct {
	Type GradientType

	// ColorStops define the color transitions (minimum 2).
	ColorStops []ColorStopOp

	// Linear gradient coordinates
	X1, Y1, X2, Y2 float64

	// Radial gradient coordinates
	X0, Y0, R0, R1 float64

	// Extend flags
	ExtendStart bool
	ExtendEnd   bool
}

// GenerateContentStream generates a PDF content stream from text and graphics operations.
//
// Graphics are drawn BEFORE text (so text appears on top).
//
// Returns:
//   - content: The content stream bytes
//   - resources: The resource dictionary for fonts used
//   - error: Any error that occurred
//
// Example content stream:
//
//	BT
//	0 0 0 rg
//	/F1 24 Tf
//	100 700 Td
//	(Hello World) Tj
//	ET
func GenerateContentStream(textOps []TextOp) (content []byte, resources *ResourceDictionary, err error) {
	return GenerateContentStreamWithGraphics(textOps, nil)
}

// GenerateContentStreamWithGraphics generates a PDF content stream from text and graphics operations.
//
// Graphics are drawn BEFORE text (so text appears on top).
//
// Returns:
//   - content: The content stream bytes
//   - resources: The resource dictionary for fonts used
//   - error: Any error that occurred
func GenerateContentStreamWithGraphics(textOps []TextOp, graphicsOps []GraphicsOp) (content []byte, resources *ResourceDictionary, err error) {
	if len(textOps) == 0 && len(graphicsOps) == 0 {
		// Empty content stream
		return []byte{}, NewResourceDictionary(), nil
	}

	csw := NewContentStreamWriter()
	resources = NewResourceDictionary()

	// STEP 1: Draw graphics FIRST (so text appears on top)
	for _, gop := range graphicsOps {
		if err := renderGraphicsOp(csw, gop, resources); err != nil {
			return nil, nil, fmt.Errorf("failed to render graphics: %w", err)
		}
	}

	// STEP 2: Draw text
	// Track which fonts we've used (to avoid adding duplicates)
	// Key is either standard font name or custom font ID.
	usedFonts := make(map[string]string) // font key -> resource name

	for _, op := range textOps {
		// Determine font key (custom font ID or standard font name).
		var fontKey string
		if op.CustomFont != nil {
			fontKey = "custom:" + op.CustomFont.ID
		} else {
			fontKey = "std:" + op.Font
		}

		// Get or create font resource
		fontResName, exists := usedFonts[fontKey]
		if !exists {
			// Create font object (we'll need to track object numbers)
			// For now, use a placeholder object number that will be replaced
			// by the actual writer. We track fontKey to enable correct matching later.
			fontObjNum := 0 // Will be set by caller via SetFontObjNumByID
			fontResName = resources.AddFontWithID(fontObjNum, fontKey)
			usedFonts[fontKey] = fontResName
		}

		// Begin text object
		csw.BeginText()

		// Set color (CMYK takes precedence over RGB)
		if op.ColorCMYK != nil {
			csw.SetFillColorCMYK(op.ColorCMYK.C, op.ColorCMYK.M, op.ColorCMYK.Y, op.ColorCMYK.K)
		} else {
			csw.SetFillColorRGB(op.Color.R, op.Color.G, op.Color.B)
		}

		// Set font and size
		csw.SetFont(fontResName, op.Size)

		// Set position
		csw.MoveTextPosition(op.X, op.Y)

		// Show text (for custom fonts, encode using glyph IDs)
		if op.CustomFont != nil {
			csw.ShowTextEncoded(encodeTextForEmbeddedFont(op.Text, op.CustomFont))
		} else {
			csw.ShowText(op.Text)
		}

		// End text object
		csw.EndText()
	}

	return csw.Bytes(), resources, nil
}

// renderGraphicsOp renders a single graphics operation to the content stream.
func renderGraphicsOp(csw *ContentStreamWriter, gop GraphicsOp, resources *ResourceDictionary) error {
	// Clipping and text operations manage their own state - don't wrap them.
	if gop.Type == 20 || gop.Type == 21 || gop.Type == 22 {
		switch gop.Type {
		case 20: // BeginClipRect - starts a clipping region
			return renderBeginClipRect(csw, gop)
		case 21: // EndClip - ends clipping region
			return renderEndClip(csw)
		case 22: // TextBlock - text rendered inline with graphics
			return renderTextBlock(csw, gop, resources)
		}
	}

	// Save graphics state for regular drawing operations.
	csw.SaveState()

	switch gop.Type {
	case 0: // Line
		return renderLine(csw, gop)
	case 1: // Rectangle
		return renderRect(csw, gop)
	case 2: // Circle
		return renderCircle(csw, gop)
	case 3: // Image
		return renderImage(csw, gop, resources)
	case 4: // Watermark
		return renderWatermark(csw, gop, resources)
	case 5: // Polygon
		return renderPolygon(csw, gop)
	case 6: // Polyline
		return renderPolyline(csw, gop)
	case 7: // Ellipse
		return renderEllipse(csw, gop)
	case 8: // Bezier
		return renderBezier(csw, gop)
	default:
		return fmt.Errorf("unknown graphics operation type: %d", gop.Type)
	}
}

// setStrokeColor sets the stroke color (CMYK takes precedence over RGB).
func setStrokeColor(csw *ContentStreamWriter, rgb *RGB, cmyk *CMYK) {
	if cmyk != nil {
		csw.SetStrokeColorCMYK(cmyk.C, cmyk.M, cmyk.Y, cmyk.K)
	} else if rgb != nil {
		csw.SetStrokeColorRGB(rgb.R, rgb.G, rgb.B)
	}
}

// setFillColor sets the fill color (CMYK takes precedence over RGB).
func setFillColor(csw *ContentStreamWriter, rgb *RGB, cmyk *CMYK) {
	if cmyk != nil {
		csw.SetFillColorCMYK(cmyk.C, cmyk.M, cmyk.Y, cmyk.K)
	} else if rgb != nil {
		csw.SetFillColorRGB(rgb.R, rgb.G, rgb.B)
	}
}

// renderLine renders a line to the content stream.
func renderLine(csw *ContentStreamWriter, gop GraphicsOp) error {
	// Set line width
	if gop.StrokeWidth > 0 {
		csw.SetLineWidth(gop.StrokeWidth)
	} else {
		csw.SetLineWidth(1.0) // Default
	}

	// Set dash pattern if dashed
	if gop.Dashed && len(gop.DashArray) > 0 {
		csw.SetDashPattern(gop.DashArray, gop.DashPhase)
	}

	// Set stroke color (lines only have stroke, no fill)
	setStrokeColor(csw, gop.StrokeColor, gop.StrokeColorCMYK)

	// Draw line path
	csw.MoveTo(gop.X, gop.Y)
	csw.LineTo(gop.X2, gop.Y2)
	csw.Stroke()

	// Restore graphics state
	csw.RestoreState()
	return nil
}

// renderRect renders a rectangle to the content stream.
func renderRect(csw *ContentStreamWriter, gop GraphicsOp) error {
	// Set line width
	if gop.StrokeWidth > 0 {
		csw.SetLineWidth(gop.StrokeWidth)
	} else {
		csw.SetLineWidth(1.0) // Default
	}

	// Set dash pattern if dashed
	if gop.Dashed && len(gop.DashArray) > 0 {
		csw.SetDashPattern(gop.DashArray, gop.DashPhase)
	}

	// Set stroke color
	setStrokeColor(csw, gop.StrokeColor, gop.StrokeColorCMYK)

	// Draw rectangle path
	csw.Rectangle(gop.X, gop.Y, gop.Width, gop.Height)

	// Handle fill (gradient or solid color)
	hasFill := gop.FillColor != nil || gop.FillColorCMYK != nil || gop.FillGradient != nil
	hasStroke := gop.StrokeColor != nil || gop.StrokeColorCMYK != nil

	if gop.FillGradient != nil {
		// Use gradient fill
		// Note: Full gradient implementation requires shading pattern resource
		// For now, use a simplified approach with color interpolation
		renderGradientFill(csw, gop.FillGradient)
	} else {
		// Use solid color fill
		setFillColor(csw, gop.FillColor, gop.FillColorCMYK)
	}

	// Fill and/or stroke
	if hasStroke && hasFill {
		csw.FillAndStroke()
	} else if hasFill {
		csw.Fill()
	} else {
		csw.Stroke()
	}

	// Restore graphics state
	csw.RestoreState()
	return nil
}

// renderBeginClipRect starts a rectangular clipping region.
//
// This saves the graphics state, defines a rectangle path, and sets it as the clipping path.
// All subsequent drawing operations will be clipped to this rectangle until EndClip is called.
//
// Usage:
//
//	BeginClipRect(x, y, width, height)
//	... draw content that should be clipped ...
//	EndClip()
func renderBeginClipRect(csw *ContentStreamWriter, gop GraphicsOp) error {
	// Save graphics state (so we can restore after clipping).
	csw.SaveState()

	// Define rectangle path.
	csw.Rectangle(gop.X, gop.Y, gop.Width, gop.Height)

	// Set clipping path and end path (W n).
	csw.Clip()
	csw.EndPath()

	// Note: We do NOT restore state here - clipping remains active.
	// The caller must call EndClip (type 21) to restore state.
	return nil
}

// renderEndClip ends a clipping region by restoring the graphics state.
func renderEndClip(csw *ContentStreamWriter) error {
	csw.RestoreState()
	return nil
}

// renderTextBlock renders a text block inline with graphics operations.
//
// This is used for clipped text where the text needs to be rendered between
// BeginClip and EndClip operations.
func renderTextBlock(csw *ContentStreamWriter, gop GraphicsOp, resources *ResourceDictionary) error {
	if gop.TextFont == nil {
		return fmt.Errorf("TextFont is required for TextBlock")
	}

	// Get or create font resource name.
	fontKey := "custom:" + gop.TextFont.ID
	fontResName := resources.GetFontResourceName(fontKey)
	if fontResName == "" {
		// Register the font.
		fontObjNum := 0 // Will be set by caller via SetFontObjNumByID
		fontResName = resources.AddFontWithID(fontObjNum, fontKey)
	}

	// Begin text object.
	csw.BeginText()

	// Set fill color.
	csw.SetFillColorRGB(gop.TextColorR, gop.TextColorG, gop.TextColorB)

	// Set font and size.
	csw.SetFont(fontResName, gop.TextSize)

	// Set position.
	csw.MoveTextPosition(gop.X, gop.Y)

	// Show text (encode using glyph IDs for embedded font).
	csw.ShowTextEncoded(encodeTextForEmbeddedFont(gop.Text, gop.TextFont))

	// End text object.
	csw.EndText()

	return nil
}

// renderCircle renders a circle to the content stream using Bézier curves.
func renderCircle(csw *ContentStreamWriter, gop GraphicsOp) error {
	// Set line width
	if gop.StrokeWidth > 0 {
		csw.SetLineWidth(gop.StrokeWidth)
	} else {
		csw.SetLineWidth(1.0) // Default
	}

	// Set stroke color
	setStrokeColor(csw, gop.StrokeColor, gop.StrokeColorCMYK)

	// Draw circle using 4 Bézier curves
	// kappa = 4/3 * (sqrt(2) - 1) ≈ 0.5522847498
	const kappa = 0.5522847498
	cx, cy, r := gop.X, gop.Y, gop.Radius
	k := r * kappa

	// Start at right (3 o'clock)
	csw.MoveTo(cx+r, cy)

	// Top-right quarter
	csw.CurveTo(cx+r, cy+k, cx+k, cy+r, cx, cy+r)

	// Top-left quarter
	csw.CurveTo(cx-k, cy+r, cx-r, cy+k, cx-r, cy)

	// Bottom-left quarter
	csw.CurveTo(cx-r, cy-k, cx-k, cy-r, cx, cy-r)

	// Bottom-right quarter (back to start)
	csw.CurveTo(cx+k, cy-r, cx+r, cy-k, cx+r, cy)

	// Close path
	csw.ClosePath()

	// Handle fill (gradient or solid color)
	hasFill := gop.FillColor != nil || gop.FillColorCMYK != nil || gop.FillGradient != nil
	hasStroke := gop.StrokeColor != nil || gop.StrokeColorCMYK != nil

	if gop.FillGradient != nil {
		renderGradientFill(csw, gop.FillGradient)
	} else {
		setFillColor(csw, gop.FillColor, gop.FillColorCMYK)
	}

	// Fill and/or stroke
	if hasStroke && hasFill {
		csw.FillAndStroke()
	} else if hasFill {
		csw.Fill()
	} else {
		csw.Stroke()
	}

	// Restore graphics state
	csw.RestoreState()
	return nil
}

// renderPolygon renders a polygon to the content stream.
func renderPolygon(csw *ContentStreamWriter, gop GraphicsOp) error {
	if len(gop.Vertices) < 3 {
		return fmt.Errorf("polygon must have at least 3 vertices")
	}

	// Set line width
	if gop.StrokeWidth > 0 {
		csw.SetLineWidth(gop.StrokeWidth)
	} else {
		csw.SetLineWidth(1.0) // Default
	}

	// Set dash pattern if dashed
	if gop.Dashed && len(gop.DashArray) > 0 {
		csw.SetDashPattern(gop.DashArray, gop.DashPhase)
	}

	// Set stroke color
	setStrokeColor(csw, gop.StrokeColor, gop.StrokeColorCMYK)

	// Draw polygon path
	// Start at first vertex
	csw.MoveTo(gop.Vertices[0].X, gop.Vertices[0].Y)

	// Draw lines to remaining vertices
	for i := 1; i < len(gop.Vertices); i++ {
		csw.LineTo(gop.Vertices[i].X, gop.Vertices[i].Y)
	}

	// Close path (back to first vertex)
	csw.ClosePath()

	// Handle fill (gradient or solid color)
	hasFill := gop.FillColor != nil || gop.FillColorCMYK != nil || gop.FillGradient != nil
	hasStroke := gop.StrokeColor != nil || gop.StrokeColorCMYK != nil

	if gop.FillGradient != nil {
		renderGradientFill(csw, gop.FillGradient)
	} else {
		setFillColor(csw, gop.FillColor, gop.FillColorCMYK)
	}

	// Fill and/or stroke
	if hasStroke && hasFill {
		csw.FillAndStroke()
	} else if hasFill {
		csw.Fill()
	} else {
		csw.Stroke()
	}

	// Restore graphics state
	csw.RestoreState()
	return nil
}

// renderPolyline renders a polyline to the content stream.
func renderPolyline(csw *ContentStreamWriter, gop GraphicsOp) error {
	if len(gop.Vertices) < 2 {
		return fmt.Errorf("polyline must have at least 2 vertices")
	}

	// Set line width
	if gop.StrokeWidth > 0 {
		csw.SetLineWidth(gop.StrokeWidth)
	} else {
		csw.SetLineWidth(1.0) // Default
	}

	// Set dash pattern if dashed
	if gop.Dashed && len(gop.DashArray) > 0 {
		csw.SetDashPattern(gop.DashArray, gop.DashPhase)
	}

	// Set stroke color (polyline only has stroke, no fill)
	setStrokeColor(csw, gop.StrokeColor, gop.StrokeColorCMYK)

	// Draw polyline path
	// Start at first vertex
	csw.MoveTo(gop.Vertices[0].X, gop.Vertices[0].Y)

	// Draw lines to remaining vertices
	for i := 1; i < len(gop.Vertices); i++ {
		csw.LineTo(gop.Vertices[i].X, gop.Vertices[i].Y)
	}

	// DO NOT close path (polyline is open)
	csw.Stroke()

	// Restore graphics state
	csw.RestoreState()
	return nil
}

// renderEllipse renders an ellipse to the content stream using Bézier curves.
func renderEllipse(csw *ContentStreamWriter, gop GraphicsOp) error {
	// Set line width
	if gop.StrokeWidth > 0 {
		csw.SetLineWidth(gop.StrokeWidth)
	} else {
		csw.SetLineWidth(1.0) // Default
	}

	// Set stroke color
	setStrokeColor(csw, gop.StrokeColor, gop.StrokeColorCMYK)

	// Draw ellipse using 4 Bézier curves
	// kappa = 4/3 * (sqrt(2) - 1) ≈ 0.5522847498
	const kappa = 0.5522847498
	cx, cy, rx, ry := gop.X, gop.Y, gop.RX, gop.RY
	kx := rx * kappa
	ky := ry * kappa

	// Start at right (3 o'clock)
	csw.MoveTo(cx+rx, cy)

	// Top-right quarter
	csw.CurveTo(cx+rx, cy+ky, cx+kx, cy+ry, cx, cy+ry)

	// Top-left quarter
	csw.CurveTo(cx-kx, cy+ry, cx-rx, cy+ky, cx-rx, cy)

	// Bottom-left quarter
	csw.CurveTo(cx-rx, cy-ky, cx-kx, cy-ry, cx, cy-ry)

	// Bottom-right quarter (back to start)
	csw.CurveTo(cx+kx, cy-ry, cx+rx, cy-ky, cx+rx, cy)

	// Close path
	csw.ClosePath()

	// Handle fill (gradient or solid color)
	hasFill := gop.FillColor != nil || gop.FillColorCMYK != nil || gop.FillGradient != nil
	hasStroke := gop.StrokeColor != nil || gop.StrokeColorCMYK != nil

	if gop.FillGradient != nil {
		renderGradientFill(csw, gop.FillGradient)
	} else {
		setFillColor(csw, gop.FillColor, gop.FillColorCMYK)
	}

	// Fill and/or stroke
	if hasStroke && hasFill {
		csw.FillAndStroke()
	} else if hasFill {
		csw.Fill()
	} else {
		csw.Stroke()
	}

	// Restore graphics state
	csw.RestoreState()
	return nil
}

// renderGradientFill applies a gradient fill to the current path.
//
// TODO: Full gradient implementation requires:
// 1. Creating shading dictionary with Function objects
// 2. Adding shading to resource dictionary
// 3. Using 'sh' operator to apply shading
//
// For now, this function uses a fallback: the middle color of the gradient.
// This allows the API to work while we build the full infrastructure.
func renderGradientFill(csw *ContentStreamWriter, grad *GradientOp) {
	if grad == nil || len(grad.ColorStops) == 0 {
		return
	}

	// Fallback: use middle color stop
	// In the future, this will create a proper PDF shading pattern
	midIdx := len(grad.ColorStops) / 2
	midColor := grad.ColorStops[midIdx].Color

	csw.SetFillColorRGB(midColor.R, midColor.G, midColor.B)
}

// renderBezier renders a Bézier curve to the content stream.
func renderBezier(csw *ContentStreamWriter, gop GraphicsOp) error {
	if len(gop.BezierSegs) == 0 {
		return fmt.Errorf("bezier curve must have at least 1 segment")
	}

	// Set line width
	if gop.StrokeWidth > 0 {
		csw.SetLineWidth(gop.StrokeWidth)
	} else {
		csw.SetLineWidth(1.0) // Default
	}

	// Set dash pattern if dashed
	if gop.Dashed && len(gop.DashArray) > 0 {
		csw.SetDashPattern(gop.DashArray, gop.DashPhase)
	}

	// Set stroke color
	setStrokeColor(csw, gop.StrokeColor, gop.StrokeColorCMYK)

	// Draw Bézier curve path
	// Start at first segment's start point
	firstSeg := gop.BezierSegs[0]
	csw.MoveTo(firstSeg.Start.X, firstSeg.Start.Y)

	// Draw each segment
	for _, seg := range gop.BezierSegs {
		csw.CurveTo(seg.C1.X, seg.C1.Y, seg.C2.X, seg.C2.Y, seg.End.X, seg.End.Y)
	}

	// Close path if requested
	if gop.Closed {
		csw.ClosePath()
	}

	// Handle fill (gradient or solid color)
	hasFill := (gop.FillColor != nil || gop.FillColorCMYK != nil || gop.FillGradient != nil) && gop.Closed
	hasStroke := gop.StrokeColor != nil || gop.StrokeColorCMYK != nil

	if gop.FillGradient != nil && gop.Closed {
		renderGradientFill(csw, gop.FillGradient)
	} else if gop.Closed {
		setFillColor(csw, gop.FillColor, gop.FillColorCMYK)
	}

	// Fill and/or stroke
	if hasStroke && hasFill {
		csw.FillAndStroke()
	} else if hasFill {
		csw.Fill()
	} else {
		csw.Stroke()
	}

	// Restore graphics state
	csw.RestoreState()
	return nil
}

// renderImage renders an image to the content stream.
//
// This function:
// 1. Registers the image in the resource dictionary (placeholder object number)
// 2. Applies the CTM transformation to position/scale the image
// 3. Draws the image using the Do operator
//
// PDF Image Rendering:
// - Images are XObjects of type /Image
// - The CTM (Current Transformation Matrix) is used to position and scale
// - Format: width 0 0 height x y cm /ImN Do
// - This scales the 1x1 unit square to width×height and translates to (x,y)
//
// Note: The actual image XObject will be created later by the writer
// when it has access to object number allocation.
func renderImage(csw *ContentStreamWriter, gop GraphicsOp, resources *ResourceDictionary) error {
	if gop.Image == nil {
		return fmt.Errorf("image data is nil")
	}

	// Validate dimensions
	if gop.Width <= 0 || gop.Height <= 0 {
		return fmt.Errorf("image dimensions must be positive: width=%.2f, height=%.2f", gop.Width, gop.Height)
	}

	// Register image in resources (object number will be set later)
	imageResName := resources.AddImage(0) // Placeholder object number

	// Apply CTM transformation: width 0 0 height x y cm
	// This scales the 1x1 unit image to width×height and positions it at (x,y)
	csw.ConcatMatrix(gop.Width, 0, 0, gop.Height, gop.X, gop.Y)

	// Draw the image XObject
	csw.writeOp(fmt.Sprintf("/%s", imageResName), "Do")

	// Restore graphics state
	csw.RestoreState()
	return nil
}

// renderWatermark renders a text watermark to the content stream.
//
// This function:
// 1. Sets up transparency using ExtGState (for opacity)
// 2. Applies rotation transformation matrix
// 3. Renders text with specified font, size, and color
//
// PDF Watermark Rendering:
// - Uses ExtGState for transparency (/ca and /CA operators)
// - Applies transformation matrix for rotation around the watermark position
// - Renders text using standard text operators (BT...Tj...ET)
//
// Note: The ExtGState object will be created by the writer when needed.
func renderWatermark(csw *ContentStreamWriter, gop GraphicsOp, resources *ResourceDictionary) error {
	if gop.Text == "" {
		return fmt.Errorf("watermark text is empty")
	}
	if gop.WatermarkFont == "" {
		return fmt.Errorf("watermark font is not set")
	}
	if gop.TextSize <= 0 {
		return fmt.Errorf("watermark font size must be positive: %.2f", gop.TextSize)
	}

	// Get or create font resource
	fontKey := "std:" + gop.WatermarkFont
	fontResName := resources.GetFontResourceName(fontKey)
	if fontResName == "" {
		// Register the font
		fontObjNum := 0 // Will be set by caller via SetFontObjNumByID
		fontResName = resources.AddFontWithID(fontObjNum, fontKey)
	}

	// Set opacity if not fully opaque
	if gop.WatermarkOpacity < 1.0 {
		// Get or create ExtGState for transparency
		opacity := gop.WatermarkOpacity
		if opacity < 0 {
			opacity = 0
		}
		gsName, _ := resources.GetOrCreateExtGState(opacity)
		csw.SetGraphicsState(gsName)
	}

	// Apply rotation transformation if rotation is non-zero
	if gop.WatermarkRotation != 0 {
		// Calculate rotation matrix around point (X, Y)
		// Convert degrees to radians
		radians := gop.WatermarkRotation * math.Pi / 180.0

		// Calculate cos and sin
		cos := math.Cos(radians)
		sin := math.Sin(radians)

		// Apply transformation matrix for rotation around point (X, Y)
		// Matrix: [cos sin -sin cos e f]
		// where e = X - X*cos + Y*sin, f = Y - X*sin - Y*cos
		e := gop.X - gop.X*cos + gop.Y*sin
		f := gop.Y - gop.X*sin - gop.Y*cos
		csw.ConcatMatrix(cos, sin, -sin, cos, e, f)
	}

	// Begin text object
	csw.BeginText()

	// Set text color
	csw.SetFillColorRGB(gop.TextColorR, gop.TextColorG, gop.TextColorB)

	// Set font and size
	csw.SetFont(fontResName, gop.TextSize)

	// Set position (origin if rotated, or actual position if not)
	if gop.WatermarkRotation != 0 {
		// Text is already positioned by the transformation matrix
		csw.MoveTextPosition(0, 0)
	} else {
		csw.MoveTextPosition(gop.X, gop.Y)
	}

	// Show text
	csw.ShowText(gop.Text)

	// End text object
	csw.EndText()

	// Restore graphics state
	csw.RestoreState()
	return nil
}

// FontCollection holds both Standard14 and embedded TrueType fonts.
//
// This is used by the PDF writer to create font objects and manage resources.
type FontCollection struct {
	// Standard14 fonts (built-in PDF fonts).
	Standard14 map[string]*fonts.Standard14Font

	// Embedded TrueType/OpenType fonts.
	Embedded map[string]*EmbeddedFont
}

// CreateFontObjects creates PDF font objects for the fonts used in text operations.
//
// Returns a map of font name -> *Standard14Font.
//
// This allows the writer to create font objects and assign them object numbers.
//
// Deprecated: Use CreateFontCollection for full font support including embedded fonts.
func CreateFontObjects(textOps []TextOp) (map[string]*fonts.Standard14Font, error) {
	fontMap := make(map[string]*fonts.Standard14Font)

	for _, op := range textOps {
		// Skip custom fonts - they're handled separately.
		if op.CustomFont != nil {
			continue
		}

		if _, exists := fontMap[op.Font]; exists {
			continue // Already have this font
		}

		// Map font name to Standard14Font
		font, err := getStandard14Font(op.Font)
		if err != nil {
			return nil, err
		}

		fontMap[op.Font] = font
	}

	return fontMap, nil
}

// CreateFontCollection creates a collection of all fonts used in text operations.
//
// This handles both Standard14 fonts (built-in) and embedded TrueType fonts.
//
// Returns a FontCollection containing:
//   - Standard14: Map of font name -> Standard14Font
//   - Embedded: Map of font ID -> EmbeddedFont
func CreateFontCollection(textOps []TextOp) (*FontCollection, error) {
	return CreateFontCollectionWithGraphics(textOps, nil)
}

// CreateFontCollectionWithGraphics creates a collection of all fonts used in text and graphics operations.
//
// This handles both Standard14 fonts (built-in) and embedded TrueType fonts,
// including fonts used in TextBlock graphics operations.
func CreateFontCollectionWithGraphics(textOps []TextOp, graphicsOps []GraphicsOp) (*FontCollection, error) {
	collection := &FontCollection{
		Standard14: make(map[string]*fonts.Standard14Font),
		Embedded:   make(map[string]*EmbeddedFont),
	}

	// Collect fonts from text operations.
	for _, op := range textOps {
		// Handle custom embedded fonts.
		if op.CustomFont != nil {
			if _, exists := collection.Embedded[op.CustomFont.ID]; !exists {
				collection.Embedded[op.CustomFont.ID] = op.CustomFont
			}
			continue
		}

		// Handle Standard14 fonts.
		if _, exists := collection.Standard14[op.Font]; exists {
			continue // Already have this font
		}

		font, err := getStandard14Font(op.Font)
		if err != nil {
			return nil, err
		}

		collection.Standard14[op.Font] = font
	}

	// Collect fonts from graphics operations (TextBlock).
	for _, gop := range graphicsOps {
		if gop.Type == 22 && gop.TextFont != nil { // Type 22 = TextBlock
			if _, exists := collection.Embedded[gop.TextFont.ID]; !exists {
				collection.Embedded[gop.TextFont.ID] = gop.TextFont
			}
		}
	}

	return collection, nil
}

// HasEmbeddedFonts returns true if the collection contains embedded fonts.
func (fc *FontCollection) HasEmbeddedFonts() bool {
	return len(fc.Embedded) > 0
}

// TotalFontCount returns the total number of fonts in the collection.
func (fc *FontCollection) TotalFontCount() int {
	return len(fc.Standard14) + len(fc.Embedded)
}

// encodeTextForEmbeddedFont encodes text using glyph IDs for embedded TrueType fonts.
//
// For TrueType fonts in PDF, we must use the font's internal glyph IDs
// as character codes, NOT Unicode code points. The ToUnicode CMap provides
// the reverse mapping from glyph IDs back to Unicode for text extraction.
//
// This function returns a hex-encoded string suitable for use with Tj operator.
func encodeTextForEmbeddedFont(text string, font *EmbeddedFont) string {
	if font == nil || font.TTF == nil {
		return "<>"
	}

	var buf bytes.Buffer
	buf.WriteString("<")

	for _, r := range text {
		// Look up glyph ID for this character.
		glyphID, ok := font.TTF.CharToGlyph[r]
		if !ok {
			// Character not in font - use .notdef glyph (0).
			glyphID = 0
		}

		// Write glyph ID as 2-byte hex (TrueType fonts use 16-bit glyph IDs).
		buf.WriteString(fmt.Sprintf("%04X", glyphID))
	}

	buf.WriteString(">")
	return buf.String()
}

// getStandard14Font returns the Standard14Font for the given font name.
func getStandard14Font(name string) (*fonts.Standard14Font, error) {
	switch name {
	case "Helvetica":
		return fonts.Helvetica, nil
	case "Helvetica-Bold":
		return fonts.HelveticaBold, nil
	case "Helvetica-Oblique":
		return fonts.HelveticaOblique, nil
	case "Helvetica-BoldOblique":
		return fonts.HelveticaBoldOblique, nil
	case "Times-Roman":
		return fonts.TimesRoman, nil
	case "Times-Bold":
		return fonts.TimesBold, nil
	case "Times-Italic":
		return fonts.TimesItalic, nil
	case "Times-BoldItalic":
		return fonts.TimesBoldItalic, nil
	case "Courier":
		return fonts.Courier, nil
	case "Courier-Bold":
		return fonts.CourierBold, nil
	case "Courier-Oblique":
		return fonts.CourierOblique, nil
	case "Courier-BoldOblique":
		return fonts.CourierBoldOblique, nil
	case "Symbol":
		return fonts.Symbol, nil
	case "ZapfDingbats":
		return fonts.ZapfDingbats, nil
	default:
		return nil, fmt.Errorf("unknown font: %s", name)
	}
}

// CreateContentStreamObject creates a PDF stream object for content.
//
// Format (uncompressed):
//
//	N 0 obj
//	<< /Length M >>
//	stream
//	... content ...
//	endstream
//	endobj
//
// Format (compressed):
//
//	N 0 obj
//	<< /Length M /Filter /FlateDecode >>
//	stream
//	... compressed content ...
//	endstream
//	endobj
//
// Parameters:
//   - objNum: Object number for this stream
//   - content: Stream content (uncompressed)
//   - compress: If true, compress the content using FlateDecode
//
// Returns the IndirectObject ready to write.
func CreateContentStreamObject(objNum int, content []byte, compress bool) *IndirectObject {
	var buf bytes.Buffer

	// Compress content if requested
	actualContent := content
	if compress && ShouldCompress(content) {
		compressed, err := CompressStream(content, DefaultCompression)
		if err == nil {
			// Compression succeeded, use compressed content
			actualContent = compressed
		}
		// If compression fails, fall back to uncompressed
	}

	// Write stream dictionary
	buf.WriteString("<< /Length ")
	buf.WriteString(fmt.Sprintf("%d", len(actualContent)))

	// Add Filter if compressed
	if compress && len(actualContent) != len(content) {
		buf.WriteString(" /Filter /FlateDecode")
	}

	buf.WriteString(" >>\n")

	// Write stream keyword
	buf.WriteString("stream\n")

	// Write stream data
	buf.Write(actualContent)

	// Ensure newline before endstream (only for uncompressed text streams)
	if !compress && len(actualContent) > 0 && actualContent[len(actualContent)-1] != '\n' {
		buf.WriteString("\n")
	}

	// Write endstream
	buf.WriteString("endstream")

	return NewIndirectObject(objNum, 0, buf.Bytes())
}
