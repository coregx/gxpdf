package creator

import (
	"errors"
)

// Point represents a 2D point in PDF coordinate space.
type Point struct {
	X float64 // Horizontal position (points from left)
	Y float64 // Vertical position (points from bottom)
}

// PolygonOptions configures polygon drawing.
type PolygonOptions struct {
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

	// Opacity is the polygon opacity (0.0 = transparent, 1.0 = opaque).
	// Optional. If set, applies transparency via ExtGState.
	// Affects both fill and stroke.
	// Range: [0.0, 1.0]
	Opacity *float64
}

// DrawPolygon draws a closed polygon through the specified vertices.
//
// A polygon is a closed shape with N vertices. The path automatically
// closes from the last vertex back to the first.
//
// The polygon can be stroked, filled, or both, depending on the options.
//
// Parameters:
//   - vertices: Array of points defining the polygon vertices (minimum 3 points)
//   - opts: Polygon options (stroke color, fill color, width, dash pattern)
//
// Example:
//
//	opts := &creator.PolygonOptions{
//	    StrokeColor: &creator.Black,
//	    StrokeWidth: 2.0,
//	    FillColor:   &creator.Blue,
//	}
//	vertices := []creator.Point{
//	    {X: 100, Y: 100},
//	    {X: 150, Y: 50},
//	    {X: 200, Y: 100},
//	    {X: 175, Y: 150},
//	    {X: 125, Y: 150},
//	}
//	err := page.DrawPolygon(vertices, opts)
func (p *Page) DrawPolygon(vertices []Point, opts *PolygonOptions) error {
	if opts == nil {
		return errors.New("polygon options cannot be nil")
	}

	// Validate vertices
	if len(vertices) < 3 {
		return errors.New("polygon must have at least 3 vertices")
	}

	// Validate options
	if err := validatePolygonOptions(opts); err != nil {
		return err
	}

	// Store graphics operation
	p.graphicsOps = append(p.graphicsOps, GraphicsOperation{
		Type:        GraphicsOpPolygon,
		Vertices:    vertices,
		PolygonOpts: opts,
	})

	return nil
}

// validatePolygonOptions validates polygon drawing options.
func validatePolygonOptions(opts *PolygonOptions) error {
	// Validate stroke color if provided
	if opts.StrokeColor != nil {
		if err := validateColor(*opts.StrokeColor); err != nil {
			return errors.New("stroke " + err.Error())
		}
	}

	// Validate fill color if provided
	if opts.FillColor != nil {
		if err := validateColor(*opts.FillColor); err != nil {
			return errors.New("fill " + err.Error())
		}
	}

	// Validate stroke width
	if opts.StrokeWidth < 0 {
		return errors.New("stroke width must be non-negative")
	}

	// At least one of stroke or fill must be set
	if opts.StrokeColor == nil && opts.FillColor == nil && opts.FillGradient == nil {
		return errors.New("polygon must have at least stroke, fill color, or gradient")
	}

	// FillColor and FillGradient are mutually exclusive
	if opts.FillColor != nil && opts.FillGradient != nil {
		return errors.New("cannot use both fill color and fill gradient")
	}

	// Validate gradient if provided
	if opts.FillGradient != nil {
		if err := opts.FillGradient.Validate(); err != nil {
			return errors.New("fill gradient: " + err.Error())
		}
	}

	return nil
}
