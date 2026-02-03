package creator

// GraphicsOpType represents the type of graphics operation.
type GraphicsOpType int

const (
	// GraphicsOpLine draws a line from (X,Y) to (X2,Y2).
	GraphicsOpLine GraphicsOpType = iota

	// GraphicsOpRect draws a rectangle (stroke only, fill only, or both).
	GraphicsOpRect

	// GraphicsOpCircle draws a circle at center (X,Y) with Radius.
	GraphicsOpCircle

	// GraphicsOpImage draws an image at (X,Y) with Width,Height.
	GraphicsOpImage

	// GraphicsOpWatermark draws a text watermark at (X,Y) with rotation and opacity.
	GraphicsOpWatermark

	// GraphicsOpPolygon draws a closed polygon through N vertices.
	GraphicsOpPolygon

	// GraphicsOpPolyline draws an open path through N vertices.
	GraphicsOpPolyline

	// GraphicsOpEllipse draws an ellipse at center (X,Y) with radii RX and RY.
	GraphicsOpEllipse

	// GraphicsOpBezier draws a complex curve composed of Bézier segments.
	GraphicsOpBezier

	// Reserved 9-19 for future graphics ops.

	// GraphicsOpBeginClip begins a rectangular clipping region.
	// All subsequent drawing is clipped to the rectangle (X, Y, Width, Height).
	// Must be followed by GraphicsOpEndClip to restore the previous clipping state.
	GraphicsOpBeginClip GraphicsOpType = 20

	// GraphicsOpEndClip ends a clipping region started by GraphicsOpBeginClip.
	GraphicsOpEndClip GraphicsOpType = 21

	// GraphicsOpTextBlock renders text inline with graphics operations.
	// Used for clipped text where ordering matters.
	GraphicsOpTextBlock GraphicsOpType = 22
)

// LineOptions configures line drawing.
type LineOptions struct {
	// Color is the line color (RGB, 0.0 to 1.0 range).
	// If ColorCMYK is set, this field is ignored.
	Color Color

	// ColorCMYK is the line color in CMYK color space (optional).
	// If set, this takes precedence over Color (RGB).
	ColorCMYK *ColorCMYK

	// Width is the line width in points (default: 1.0).
	Width float64

	// Dashed enables dashed line rendering.
	Dashed bool

	// DashArray defines the dash pattern (e.g., [3, 1] for "3 on, 1 off").
	// Only used when Dashed is true.
	DashArray []float64

	// DashPhase is the starting offset into the dash pattern.
	// Only used when Dashed is true.
	DashPhase float64

	// Opacity is the line opacity (0.0 = transparent, 1.0 = opaque).
	// Optional. If set, applies transparency via ExtGState.
	// Range: [0.0, 1.0]
	Opacity *float64
}

// RectOptions configures rectangle drawing.
type RectOptions struct {
	// StrokeColor is the border color (nil = no stroke).
	// If StrokeColorCMYK is set, this field is ignored.
	StrokeColor *Color

	// StrokeColorCMYK is the border color in CMYK (nil = no stroke).
	// If set, this takes precedence over StrokeColor (RGB).
	StrokeColorCMYK *ColorCMYK

	// StrokeWidth is the border width in points (default: 1.0).
	StrokeWidth float64

	// FillColor is the fill color (nil = no fill).
	// Mutually exclusive with FillGradient and FillColorCMYK.
	// If FillColorCMYK is set, this field is ignored.
	FillColor *Color

	// FillColorCMYK is the fill color in CMYK (nil = no fill).
	// If set, this takes precedence over FillColor (RGB).
	// Mutually exclusive with FillGradient.
	FillColorCMYK *ColorCMYK

	// FillGradient is the gradient fill (nil = no gradient fill).
	// Mutually exclusive with FillColor and FillColorCMYK.
	FillGradient *Gradient

	// Dashed enables dashed border rendering.
	Dashed bool

	// DashArray defines the dash pattern for the border.
	// Only used when Dashed is true.
	DashArray []float64

	// DashPhase is the starting offset into the dash pattern.
	// Only used when Dashed is true.
	DashPhase float64

	// Opacity is the rectangle opacity (0.0 = transparent, 1.0 = opaque).
	// Optional. If set, applies transparency via ExtGState.
	// Affects both fill and stroke.
	// Range: [0.0, 1.0]
	Opacity *float64
}

// CircleOptions configures circle drawing.
type CircleOptions struct {
	// StrokeColor is the border color (nil = no stroke).
	// If StrokeColorCMYK is set, this field is ignored.
	StrokeColor *Color

	// StrokeColorCMYK is the border color in CMYK (nil = no stroke).
	// If set, this takes precedence over StrokeColor (RGB).
	StrokeColorCMYK *ColorCMYK

	// StrokeWidth is the border width in points (default: 1.0).
	StrokeWidth float64

	// FillColor is the fill color (nil = no fill).
	// Mutually exclusive with FillGradient and FillColorCMYK.
	// If FillColorCMYK is set, this field is ignored.
	FillColor *Color

	// FillColorCMYK is the fill color in CMYK (nil = no fill).
	// If set, this takes precedence over FillColor (RGB).
	// Mutually exclusive with FillGradient.
	FillColorCMYK *ColorCMYK

	// FillGradient is the gradient fill (nil = no gradient fill).
	// Mutually exclusive with FillColor and FillColorCMYK.
	FillGradient *Gradient

	// Opacity is the circle opacity (0.0 = transparent, 1.0 = opaque).
	// Optional. If set, applies transparency via ExtGState.
	// Affects both fill and stroke.
	// Range: [0.0, 1.0]
	Opacity *float64
}

// GraphicsOperation represents a graphics drawing operation.
//
// The fields used depend on the Type:
// - GraphicsOpLine: X, Y, X2, Y2, LineOpts.
// - GraphicsOpRect: X, Y, Width, Height, RectOpts.
// - GraphicsOpCircle: X, Y, Radius, CircleOpts.
// - GraphicsOpImage: X, Y, Width, Height, Image.
// - GraphicsOpWatermark: X, Y, WatermarkOp.
// - GraphicsOpPolygon: Vertices, PolygonOpts.
// - GraphicsOpPolyline: Vertices, PolylineOpts.
// - GraphicsOpEllipse: X, Y, RX, RY, EllipseOpts.
// - GraphicsOpBezier: BezierSegs, BezierOpts.
type GraphicsOperation struct {
	// Type is the graphics operation type.
	Type GraphicsOpType

	// X is the x-coordinate (start point for line, lower-left for rect/image, center for circle/ellipse).
	X float64

	// Y is the y-coordinate (start point for line, lower-left for rect/image, center for circle/ellipse).
	Y float64

	// X2 is the end x-coordinate (only for line).
	X2 float64

	// Y2 is the end y-coordinate (only for line).
	Y2 float64

	// Width is the rectangle/image width (only for rect/image).
	Width float64

	// Height is the rectangle/image height (only for rect/image).
	Height float64

	// Radius is the circle radius (only for circle).
	Radius float64

	// RX is the horizontal radius (only for ellipse).
	RX float64

	// RY is the vertical radius (only for ellipse).
	RY float64

	// Vertices is the array of points (only for polygon/polyline).
	Vertices []Point

	// BezierSegs is the array of Bézier segments (only for bezier).
	BezierSegs []BezierSegment

	// LineOpts are line options (only for line).
	LineOpts *LineOptions

	// RectOpts are rectangle options (only for rect).
	RectOpts *RectOptions

	// CircleOpts are circle options (only for circle).
	CircleOpts *CircleOptions

	// PolygonOpts are polygon options (only for polygon).
	PolygonOpts *PolygonOptions

	// PolylineOpts are polyline options (only for polyline).
	PolylineOpts *PolylineOptions

	// EllipseOpts are ellipse options (only for ellipse).
	EllipseOpts *EllipseOptions

	// BezierOpts are Bézier curve options (only for bezier).
	BezierOpts *BezierOptions

	// Image is the image to draw (only for image).
	Image *Image

	// WatermarkOp is the watermark operation (only for watermark).
	WatermarkOp *TextWatermark

	// TextBlock fields (only for GraphicsOpTextBlock).
	Text      string      // Text content
	TextFont  *CustomFont // Custom font for text
	TextSize  float64     // Font size
	TextColor *Color      // Text color (RGB)
}
