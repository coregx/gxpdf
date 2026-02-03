package creator

// Fill represents fill configuration for shapes.
//
// This is a placeholder for feat-050 (Fill/Stroke Separation).
// Full implementation will include:
//   - Paint interface (Color, Gradient, Pattern)
//   - Opacity control
//   - Fill rule (NonZero, EvenOdd)
//
// Example (planned for feat-050):
//
//	fill := &Fill{
//	    Paint:   NewLinearGradient(...),
//	    Opacity: 0.8,
//	    Rule:    FillRuleNonZero,
//	}
type Fill struct {
	// TODO: Implement in feat-050
}

// FillRule defines how to determine which areas are "inside" a path.
type FillRule int

const (
	// FillRuleNonZero uses the non-zero winding rule.
	FillRuleNonZero FillRule = iota

	// FillRuleEvenOdd uses the even-odd rule.
	FillRuleEvenOdd
)
