package creator

import (
	"errors"
)

// EllipseOptions configures ellipse drawing.
type EllipseOptions struct {
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

	// Opacity is the ellipse opacity (0.0 = transparent, 1.0 = opaque).
	// Optional. If set, applies transparency via ExtGState.
	// Affects both fill and stroke.
	// Range: [0.0, 1.0]
	Opacity *float64
}

// DrawEllipse draws an ellipse at center (cx, cy) with horizontal radius rx and vertical radius ry.
//
// An ellipse is approximated using 4 cubic Bézier curves.
// For a circle, set rx = ry.
//
// The ellipse can be stroked, filled, or both, depending on the options.
//
// Parameters:
//   - cx, cy: Center coordinates
//   - rx: Horizontal radius (half-width)
//   - ry: Vertical radius (half-height)
//   - opts: Ellipse options (stroke color, fill color, stroke width)
//
// Example:
//
//	opts := &creator.EllipseOptions{
//	    StrokeColor: &creator.Black,
//	    StrokeWidth: 2.0,
//	    FillColor:   &creator.Green,
//	}
//	err := page.DrawEllipse(150, 200, 100, 50, opts)  // Horizontal ellipse
func (p *Page) DrawEllipse(cx, cy, rx, ry float64, opts *EllipseOptions) error {
	if opts == nil {
		return errors.New("ellipse options cannot be nil")
	}

	// Validate radii
	if rx < 0 {
		return errors.New("horizontal radius must be non-negative")
	}
	if ry < 0 {
		return errors.New("vertical radius must be non-negative")
	}

	// Validate options
	if err := validateEllipseOptions(opts); err != nil {
		return err
	}

	// Store graphics operation
	p.graphicsOps = append(p.graphicsOps, GraphicsOperation{
		Type:        GraphicsOpEllipse,
		X:           cx,
		Y:           cy,
		RX:          rx,
		RY:          ry,
		EllipseOpts: opts,
	})

	return nil
}

// validateEllipseOptions validates ellipse drawing options.
func validateEllipseOptions(opts *EllipseOptions) error {
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
		return errors.New("ellipse must have at least stroke, fill color, or gradient")
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
