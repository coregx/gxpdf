package creator

import (
	"errors"
)

// BezierSegment represents a cubic Bézier curve segment.
//
// A cubic Bézier curve is defined by:
//   - Start point (P0)
//   - First control point (C1)
//   - Second control point (C2)
//   - End point (P1)
//
// The curve starts at P0, is pulled toward C1 and C2, and ends at P1.
type BezierSegment struct {
	// Start is the starting point (P0).
	// For the first segment, this is the curve's starting point.
	// For subsequent segments, this should match the previous segment's End point.
	Start Point

	// C1 is the first control point.
	C1 Point

	// C2 is the second control point.
	C2 Point

	// End is the ending point (P1).
	End Point
}

// BezierOptions configures Bézier curve drawing.
type BezierOptions struct {
	// Color is the curve color (RGB, 0.0 to 1.0 range).
	// If ColorCMYK is set, this field is ignored.
	Color Color

	// ColorCMYK is the curve color in CMYK color space (optional).
	// If set, this takes precedence over Color (RGB).
	ColorCMYK *ColorCMYK

	// Width is the curve width in points (default: 1.0).
	Width float64

	// Dashed enables dashed curve rendering.
	Dashed bool

	// DashArray defines the dash pattern (e.g., [3, 1] for "3 on, 1 off").
	// Only used when Dashed is true.
	DashArray []float64

	// DashPhase is the starting offset into the dash pattern.
	// Only used when Dashed is true.
	DashPhase float64

	// Closed determines if the curve path should be closed.
	// If true, a line is drawn from the last segment's end point
	// back to the first segment's start point.
	Closed bool

	// FillColor is the fill color for closed curves (nil = no fill).
	// Only used when Closed is true.
	// Mutually exclusive with FillGradient.
	FillColor *Color

	// FillGradient is the gradient fill for closed curves (nil = no gradient fill).
	// Only used when Closed is true.
	// Mutually exclusive with FillColor.
	FillGradient *Gradient

	// Opacity is the bezier curve opacity (0.0 = transparent, 1.0 = opaque).
	// Optional. If set, applies transparency via ExtGState.
	// Affects both stroke and fill (if Closed is true).
	// Range: [0.0, 1.0]
	Opacity *float64
}

// DrawBezierCurve draws a complex curve composed of one or more cubic Bézier segments.
//
// A Bézier curve provides smooth, flowing curves that are widely used in vector graphics.
// Multiple segments can be connected to create complex paths.
//
// Parameters:
//   - segments: Array of Bézier curve segments (minimum 1 segment)
//   - opts: Curve options (color, width, dash pattern, closed path)
//
// Example (simple S-curve):
//
//	opts := &creator.BezierOptions{
//	    Color: creator.Blue,
//	    Width: 2.0,
//	}
//	segments := []creator.BezierSegment{
//	    {
//	        Start: creator.Point{X: 100, Y: 100},
//	        C1:    creator.Point{X: 150, Y: 200},
//	        C2:    creator.Point{X: 200, Y: 200},
//	        End:   creator.Point{X: 250, Y: 100},
//	    },
//	}
//	err := page.DrawBezierCurve(segments, opts)
//
// Example (closed filled shape):
//
//	opts := &creator.BezierOptions{
//	    Color:     creator.Black,
//	    Width:     1.0,
//	    Closed:    true,
//	    FillColor: &creator.Yellow,
//	}
//	segments := []creator.BezierSegment{
//	    // ... multiple segments forming a closed shape
//	}
//	err := page.DrawBezierCurve(segments, opts)
func (p *Page) DrawBezierCurve(segments []BezierSegment, opts *BezierOptions) error {
	if opts == nil {
		return errors.New("bezier curve options cannot be nil")
	}

	// Validate segments
	if len(segments) == 0 {
		return errors.New("bezier curve must have at least 1 segment")
	}

	// Validate segment continuity (each segment's start should match previous segment's end)
	for i := 1; i < len(segments); i++ {
		prev := segments[i-1].End
		curr := segments[i].Start
		// Allow small floating-point differences
		const epsilon = 0.001
		if abs(prev.X-curr.X) > epsilon || abs(prev.Y-curr.Y) > epsilon {
			return errors.New("bezier segments must be continuous (segment start point must match previous segment end point)")
		}
	}

	// Validate options
	if err := validateBezierOptions(opts); err != nil {
		return err
	}

	// Store graphics operation
	p.graphicsOps = append(p.graphicsOps, GraphicsOperation{
		Type:       GraphicsOpBezier,
		BezierSegs: segments,
		BezierOpts: opts,
	})

	return nil
}

// validateBezierOptions validates Bézier curve drawing options.
func validateBezierOptions(opts *BezierOptions) error {
	// Validate color components
	if err := validateColor(opts.Color); err != nil {
		return err
	}

	// Validate width
	if opts.Width < 0 {
		return errors.New("curve width must be non-negative")
	}

	// Validate fill color if provided
	if opts.FillColor != nil {
		if err := validateColor(*opts.FillColor); err != nil {
			return errors.New("fill " + err.Error())
		}
	}

	// Fill color only makes sense for closed curves
	if opts.FillColor != nil && !opts.Closed {
		return errors.New("fill color requires closed curve (set Closed: true)")
	}

	// FillColor and FillGradient are mutually exclusive
	if opts.FillColor != nil && opts.FillGradient != nil {
		return errors.New("cannot use both fill color and fill gradient")
	}

	// Validate gradient if provided
	if opts.FillGradient != nil {
		if !opts.Closed {
			return errors.New("fill gradient requires closed curve (set Closed: true)")
		}
		if err := opts.FillGradient.Validate(); err != nil {
			return errors.New("fill gradient: " + err.Error())
		}
	}

	return nil
}

// abs returns the absolute value of a float64.
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
