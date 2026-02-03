package creator

import (
	"errors"
)

// PolylineOptions configures polyline drawing.
type PolylineOptions struct {
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

	// Opacity is the polyline opacity (0.0 = transparent, 1.0 = opaque).
	// Optional. If set, applies transparency via ExtGState.
	// Range: [0.0, 1.0]
	Opacity *float64
}

// DrawPolyline draws an open path through the specified vertices.
//
// A polyline is an open path (not closed) that connects N vertices.
// Unlike a polygon, the path does NOT automatically close back to the first point.
//
// Parameters:
//   - vertices: Array of points defining the polyline path (minimum 2 points)
//   - opts: Polyline options (color, width, dash pattern)
//
// Example:
//
//	opts := &creator.PolylineOptions{
//	    Color: creator.Red,
//	    Width: 2.0,
//	}
//	vertices := []creator.Point{
//	    {X: 100, Y: 100},
//	    {X: 150, Y: 50},
//	    {X: 200, Y: 100},
//	    {X: 250, Y: 75},
//	}
//	err := page.DrawPolyline(vertices, opts)
func (p *Page) DrawPolyline(vertices []Point, opts *PolylineOptions) error {
	if opts == nil {
		return errors.New("polyline options cannot be nil")
	}

	// Validate vertices
	if len(vertices) < 2 {
		return errors.New("polyline must have at least 2 vertices")
	}

	// Validate options
	if err := validatePolylineOptions(opts); err != nil {
		return err
	}

	// Store graphics operation
	p.graphicsOps = append(p.graphicsOps, GraphicsOperation{
		Type:         GraphicsOpPolyline,
		Vertices:     vertices,
		PolylineOpts: opts,
	})

	return nil
}

// validatePolylineOptions validates polyline drawing options.
func validatePolylineOptions(opts *PolylineOptions) error {
	// Validate color components
	if err := validateColor(opts.Color); err != nil {
		return err
	}

	// Validate width
	if opts.Width < 0 {
		return errors.New("line width must be non-negative")
	}

	return nil
}
