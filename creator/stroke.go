package creator

// Stroke represents stroke configuration for shapes.
//
// This is a placeholder for feat-050 (Fill/Stroke Separation).
// Full implementation will include:
//   - Paint interface (Color, Gradient, Pattern)
//   - Line width, cap, join
//   - Dash patterns
//
// Example (planned for feat-050):
//
//	stroke := &Stroke{
//	    Paint:      Black,
//	    Width:      2.0,
//	    LineCap:    LineCapRound,
//	    LineJoin:   LineJoinMiter,
//	    DashArray:  []float64{5, 3},
//	}
type Stroke struct {
	// TODO: Implement in feat-050
}

// LineCap defines how line ends are rendered.
type LineCap int

const (
	// LineCapButt ends exactly at the endpoint.
	LineCapButt LineCap = iota

	// LineCapRound adds a semicircular cap.
	LineCapRound

	// LineCapSquare adds a square cap extending past the endpoint.
	LineCapSquare
)

// LineJoin defines how corners are rendered.
type LineJoin int

const (
	// LineJoinMiter extends lines to form a sharp corner.
	LineJoinMiter LineJoin = iota

	// LineJoinRound rounds the corner.
	LineJoinRound

	// LineJoinBevel cuts off the corner.
	LineJoinBevel
)
