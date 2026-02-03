package creator

import (
	"errors"
	"fmt"
)

// Fill represents fill configuration for shapes.
//
// Fill controls how shapes are filled with paint (color or gradient).
// It includes the paint source, opacity, and fill rule.
//
// Example:
//
//	// Solid color fill
//	fill := &Fill{
//	    Paint:   Red,
//	    Opacity: 1.0,
//	    Rule:    FillRuleNonZero,
//	}
//
//	// Gradient fill with transparency
//	grad := NewLinearGradient(0, 0, 100, 0)
//	grad.AddColorStop(0, Red)
//	grad.AddColorStop(1, Blue)
//	fill := &Fill{
//	    Paint:   grad,
//	    Opacity: 0.8,
//	    Rule:    FillRuleNonZero,
//	}
type Fill struct {
	// Paint is the paint source (Color, ColorRGBA, ColorCMYK, or Gradient).
	Paint Paint

	// Opacity is the fill opacity (0.0 = transparent, 1.0 = opaque).
	// This is multiplied with the paint's alpha if using ColorRGBA.
	Opacity float64

	// Rule is the fill rule (NonZero or EvenOdd).
	// Determines which areas are "inside" the path for complex shapes.
	Rule FillRule
}

// FillRule defines how to determine which areas are "inside" a path.
//
// PDF supports two fill rules:
//   - NonZero winding rule (default): Counts path direction crossings
//   - EvenOdd rule: Counts total path crossings
//
// Reference: PDF 1.7 Specification, Section 8.5.3.3 (Filling).
type FillRule int

const (
	// FillRuleNonZero uses the non-zero winding rule.
	//
	// A point is inside if the winding number is non-zero.
	// Winding number counts +1 for left-to-right crossings,
	// -1 for right-to-left crossings.
	//
	// PDF operator: f (fill) or F (deprecated).
	FillRuleNonZero FillRule = iota

	// FillRuleEvenOdd uses the even-odd rule.
	//
	// A point is inside if a ray from the point crosses
	// the path an odd number of times.
	//
	// PDF operator: f* (eofill).
	FillRuleEvenOdd
)

// String returns the PDF fill rule name.
func (r FillRule) String() string {
	switch r {
	case FillRuleNonZero:
		return "NonZero"
	case FillRuleEvenOdd:
		return "EvenOdd"
	default:
		return "NonZero"
	}
}

// NewFill creates a new Fill with the specified paint.
//
// Default values:
//   - Opacity: 1.0 (fully opaque)
//   - Rule: FillRuleNonZero
//
// Parameters:
//   - paint: Paint source (Color, ColorRGBA, ColorCMYK, or Gradient)
//
// Example:
//
//	fill := NewFill(Red)
//	fill := NewFill(NewLinearGradient(0, 0, 100, 0))
func NewFill(paint Paint) *Fill {
	return &Fill{
		Paint:   paint,
		Opacity: 1.0,
		Rule:    FillRuleNonZero,
	}
}

// WithOpacity returns a new Fill with the specified opacity.
//
// Parameters:
//   - opacity: Opacity value (0.0 = transparent, 1.0 = opaque)
//
// Example:
//
//	fill := NewFill(Red).WithOpacity(0.5)
func (f *Fill) WithOpacity(opacity float64) *Fill {
	f.Opacity = opacity
	return f
}

// WithRule returns a new Fill with the specified fill rule.
//
// Parameters:
//   - rule: Fill rule (FillRuleNonZero or FillRuleEvenOdd)
//
// Example:
//
//	fill := NewFill(Red).WithRule(FillRuleEvenOdd)
func (f *Fill) WithRule(rule FillRule) *Fill {
	f.Rule = rule
	return f
}

// Validate validates the fill configuration.
//
// Checks:
//   - Paint is not nil
//   - Opacity is in range [0, 1]
//   - If paint is a Gradient, validate gradient
//
// Returns an error if validation fails.
func (f *Fill) Validate() error {
	if f.Paint == nil {
		return errors.New("fill paint cannot be nil")
	}

	if f.Opacity < 0 || f.Opacity > 1 {
		return fmt.Errorf("fill opacity must be in range [0, 1], got: %f", f.Opacity)
	}

	// Validate paint based on type
	switch paint := f.Paint.(type) {
	case Color:
		if err := validateColor(paint); err != nil {
			return fmt.Errorf("fill color: %w", err)
		}
	case ColorRGBA:
		if err := validateColorRGBA(paint); err != nil {
			return fmt.Errorf("fill color: %w", err)
		}
	case ColorCMYK:
		if err := validateColorCMYK(paint); err != nil {
			return fmt.Errorf("fill color: %w", err)
		}
	case *Gradient:
		if err := paint.Validate(); err != nil {
			return fmt.Errorf("fill gradient: %w", err)
		}
	}

	return nil
}

// Errors
var (
	// ErrInvalidFill is returned when a fill configuration is invalid.
	ErrInvalidFill = errors.New("invalid fill configuration")
)
