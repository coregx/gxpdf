package creator

import (
	"errors"
	"fmt"
)

// Stroke represents stroke configuration for shapes.
//
// Stroke controls how shape outlines are drawn with paint (color or gradient).
// It includes the paint source, line width, cap style, join style, and dash pattern.
//
// Example:
//
//	// Solid color stroke
//	stroke := &Stroke{
//	    Paint:      Black,
//	    Width:      2.0,
//	    LineCap:    LineCapRound,
//	    LineJoin:   LineJoinMiter,
//	    MiterLimit: 10.0,
//	}
//
//	// Dashed stroke
//	stroke := &Stroke{
//	    Paint:     Black,
//	    Width:     1.0,
//	    DashArray: []float64{5, 3},
//	    DashPhase: 0,
//	}
type Stroke struct {
	// Paint is the paint source (Color, ColorRGBA, ColorCMYK, or Gradient).
	Paint Paint

	// Width is the line width in user space units.
	// Default: 1.0
	Width float64

	// LineCap defines how line ends are rendered.
	// Default: LineCapButt
	LineCap LineCap

	// LineJoin defines how corners are rendered.
	// Default: LineJoinMiter
	LineJoin LineJoin

	// MiterLimit is the maximum miter length for LineJoinMiter.
	// When miter length exceeds this, a bevel join is used instead.
	// Default: 10.0
	MiterLimit float64

	// DashArray defines the dash pattern.
	// Alternating on/off lengths: [on1, off1, on2, off2, ...]
	// Empty array = solid line (no dashing).
	DashArray []float64

	// DashPhase is the offset into the dash pattern.
	// Default: 0
	DashPhase float64
}

// LineCap defines how line ends are rendered.
//
// PDF supports three line cap styles.
// Reference: PDF 1.7 Specification, Section 8.4.3.3 (Line Cap Style).
type LineCap int

const (
	// LineCapButt ends exactly at the endpoint.
	//
	// The line terminates squarely at the endpoint.
	// PDF value: 0
	LineCapButt LineCap = iota

	// LineCapRound adds a semicircular cap.
	//
	// A semicircle with diameter equal to line width is added.
	// PDF value: 1
	LineCapRound

	// LineCapSquare adds a square cap extending past the endpoint.
	//
	// A square extending half the line width beyond the endpoint.
	// PDF value: 2
	LineCapSquare
)

// String returns the PDF line cap name.
func (c LineCap) String() string {
	switch c {
	case LineCapButt:
		return "Butt"
	case LineCapRound:
		return "Round"
	case LineCapSquare:
		return "Square"
	default:
		return "Butt"
	}
}

// LineJoin defines how corners are rendered.
//
// PDF supports three line join styles.
// Reference: PDF 1.7 Specification, Section 8.4.3.4 (Line Join Style).
type LineJoin int

const (
	// LineJoinMiter extends lines to form a sharp corner.
	//
	// Lines are extended until they meet at an angle.
	// If miter length exceeds MiterLimit, a bevel is used.
	// PDF value: 0
	LineJoinMiter LineJoin = iota

	// LineJoinRound rounds the corner.
	//
	// A circular arc with radius equal to half line width.
	// PDF value: 1
	LineJoinRound

	// LineJoinBevel cuts off the corner.
	//
	// A straight line connects the outer endpoints.
	// PDF value: 2
	LineJoinBevel
)

// String returns the PDF line join name.
func (j LineJoin) String() string {
	switch j {
	case LineJoinMiter:
		return "Miter"
	case LineJoinRound:
		return "Round"
	case LineJoinBevel:
		return "Bevel"
	default:
		return "Miter"
	}
}

// NewStroke creates a new Stroke with the specified paint.
//
// Default values:
//   - Width: 1.0
//   - LineCap: LineCapButt
//   - LineJoin: LineJoinMiter
//   - MiterLimit: 10.0
//   - DashArray: nil (solid line)
//   - DashPhase: 0
//
// Parameters:
//   - paint: Paint source (Color, ColorRGBA, ColorCMYK, or Gradient)
//
// Example:
//
//	stroke := NewStroke(Black)
//	stroke := NewStroke(NewLinearGradient(0, 0, 100, 0))
func NewStroke(paint Paint) *Stroke {
	return &Stroke{
		Paint:      paint,
		Width:      1.0,
		LineCap:    LineCapButt,
		LineJoin:   LineJoinMiter,
		MiterLimit: 10.0,
		DashArray:  nil,
		DashPhase:  0,
	}
}

// WithWidth returns a new Stroke with the specified width.
//
// Parameters:
//   - width: Line width in user space units (must be positive)
//
// Example:
//
//	stroke := NewStroke(Black).WithWidth(2.0)
func (s *Stroke) WithWidth(width float64) *Stroke {
	s.Width = width
	return s
}

// WithLineCap returns a new Stroke with the specified line cap style.
//
// Parameters:
//   - cap: Line cap style (LineCapButt, LineCapRound, LineCapSquare)
//
// Example:
//
//	stroke := NewStroke(Black).WithLineCap(LineCapRound)
func (s *Stroke) WithLineCap(cap LineCap) *Stroke {
	s.LineCap = cap
	return s
}

// WithLineJoin returns a new Stroke with the specified line join style.
//
// Parameters:
//   - join: Line join style (LineJoinMiter, LineJoinRound, LineJoinBevel)
//
// Example:
//
//	stroke := NewStroke(Black).WithLineJoin(LineJoinRound)
func (s *Stroke) WithLineJoin(join LineJoin) *Stroke {
	s.LineJoin = join
	return s
}

// WithMiterLimit returns a new Stroke with the specified miter limit.
//
// Parameters:
//   - limit: Maximum miter length (must be >= 1.0)
//
// Example:
//
//	stroke := NewStroke(Black).WithMiterLimit(5.0)
func (s *Stroke) WithMiterLimit(limit float64) *Stroke {
	s.MiterLimit = limit
	return s
}

// WithDash returns a new Stroke with the specified dash pattern.
//
// Parameters:
//   - dashArray: Alternating on/off lengths
//   - dashPhase: Offset into the dash pattern
//
// Example:
//
//	// Dash: 5 on, 3 off
//	stroke := NewStroke(Black).WithDash([]float64{5, 3}, 0)
//
//	// Dash: 10 on, 5 off, 2 on, 5 off
//	stroke := NewStroke(Black).WithDash([]float64{10, 5, 2, 5}, 0)
func (s *Stroke) WithDash(dashArray []float64, dashPhase float64) *Stroke {
	s.DashArray = dashArray
	s.DashPhase = dashPhase
	return s
}

// Validate validates the stroke configuration.
//
// Checks:
//   - Paint is not nil
//   - Width is positive
//   - MiterLimit is >= 1.0
//   - DashArray values are all positive
//   - If paint is a Gradient, validate gradient
//
// Returns an error if validation fails.
func (s *Stroke) Validate() error {
	if s.Paint == nil {
		return errors.New("stroke paint cannot be nil")
	}

	if s.Width <= 0 {
		return fmt.Errorf("stroke width must be positive, got: %f", s.Width)
	}

	if s.MiterLimit < 1.0 {
		return fmt.Errorf("stroke miter limit must be >= 1.0, got: %f", s.MiterLimit)
	}

	// Validate dash array
	for i, dash := range s.DashArray {
		if dash < 0 {
			return fmt.Errorf("stroke dash array[%d] must be non-negative, got: %f", i, dash)
		}
	}

	// Validate paint based on type
	switch paint := s.Paint.(type) {
	case Color:
		if err := validateColor(paint); err != nil {
			return fmt.Errorf("stroke color: %w", err)
		}
	case ColorRGBA:
		if err := validateColorRGBA(paint); err != nil {
			return fmt.Errorf("stroke color: %w", err)
		}
	case ColorCMYK:
		if err := validateColorCMYK(paint); err != nil {
			return fmt.Errorf("stroke color: %w", err)
		}
	case *Gradient:
		if err := paint.Validate(); err != nil {
			return fmt.Errorf("stroke gradient: %w", err)
		}
	}

	return nil
}

// Errors
var (
	// ErrInvalidStroke is returned when a stroke configuration is invalid.
	ErrInvalidStroke = errors.New("invalid stroke configuration")
)
