package creator

import (
	"fmt"
	"math"
)

// Path represents a vector path for drawing, filling, and clipping.
//
// A Path consists of one or more subpaths, where each subpath is a sequence
// of connected lines and curves. Subpaths can be open or closed.
//
// Path uses a fluent builder API for easy construction:
//
//	path := NewPath().
//	    MoveTo(100, 100).
//	    LineTo(200, 100).
//	    QuadraticTo(250, 150, 200, 200).
//	    CubicTo(150, 250, 100, 250, 100, 200).
//	    Close()
//
// Shape helpers provide shortcuts for common shapes:
//
//	circle := NewPath().AddCircle(Point{150, 150}, 50)
//	rect := NewPath().AddRect(Rect{X: 10, Y: 10, Width: 100, Height: 50})
//
// Reference: PDF 1.7 Specification, Section 8.5 (Path Construction).
type Path struct {
	// commands stores the path construction commands.
	commands []pathCommand

	// currentPoint tracks the current point for relative operations.
	// This is updated by MoveTo, LineTo, CurveTo, etc.
	currentPoint Point

	// subpathStart tracks the starting point of the current subpath.
	// Used by Close() to connect back to the subpath start.
	subpathStart Point

	// hasCurrentPoint indicates whether currentPoint is valid.
	// False after NewPath() and Close(), true after MoveTo/LineTo/etc.
	hasCurrentPoint bool
}

// pathCommand represents a single path construction operation.
//
// Each command corresponds to a PDF path operator (m, l, c, v, y, h, re).
type pathCommand struct {
	op   pathOp
	args []float64
}

// pathOp defines the path operation type.
type pathOp int

const (
	pathOpMoveTo pathOp = iota
	pathOpLineTo
	pathOpCubicTo // Cubic Bézier curve
	pathOpClose   // Close subpath
	pathOpRect    // Rectangle (special case for optimization)
)

// String returns the PDF operator for this path operation.
func (op pathOp) String() string {
	switch op {
	case pathOpMoveTo:
		return "m"
	case pathOpLineTo:
		return "l"
	case pathOpCubicTo:
		return "c"
	case pathOpClose:
		return "h"
	case pathOpRect:
		return "re"
	default:
		return "?"
	}
}

// NewPath creates a new empty path.
//
// The path starts with no current point. Use MoveTo to start a new subpath.
//
// Example:
//
//	path := NewPath()
//	path.MoveTo(100, 100)
//	path.LineTo(200, 200)
func NewPath() *Path {
	return &Path{
		commands:        make([]pathCommand, 0, 8), // Pre-allocate for typical paths
		hasCurrentPoint: false,
	}
}

// MoveTo starts a new subpath at the specified point.
//
// If there is a current subpath, it is left open (not closed).
// The specified point becomes the current point and the subpath start.
//
// PDF operator: x y m
//
// Parameters:
//   - x, y: Coordinates of the new subpath start
//
// Example:
//
//	path := NewPath().MoveTo(100, 100)
func (p *Path) MoveTo(x, y float64) *Path {
	p.commands = append(p.commands, pathCommand{
		op:   pathOpMoveTo,
		args: []float64{x, y},
	})
	p.currentPoint = Point{X: x, Y: y}
	p.subpathStart = Point{X: x, Y: y}
	p.hasCurrentPoint = true
	return p
}

// LineTo appends a straight line to the path.
//
// The line goes from the current point to the specified point.
// The specified point becomes the new current point.
//
// PDF operator: x y l
//
// Panics if there is no current point (call MoveTo first).
//
// Parameters:
//   - x, y: Coordinates of the line endpoint
//
// Example:
//
//	path := NewPath().
//	    MoveTo(100, 100).
//	    LineTo(200, 100).
//	    LineTo(200, 200)
func (p *Path) LineTo(x, y float64) *Path {
	if !p.hasCurrentPoint {
		panic("LineTo called without current point (call MoveTo first)")
	}
	p.commands = append(p.commands, pathCommand{
		op:   pathOpLineTo,
		args: []float64{x, y},
	})
	p.currentPoint = Point{X: x, Y: y}
	return p
}

// CubicTo appends a cubic Bézier curve to the path.
//
// The curve goes from the current point to (x, y) using (c1x, c1y)
// and (c2x, c2y) as control points.
//
// PDF operator: c1x c1y c2x c2y x y c
//
// Panics if there is no current point (call MoveTo first).
//
// Parameters:
//   - c1x, c1y: First control point
//   - c2x, c2y: Second control point
//   - x, y: Endpoint of the curve
//
// Example:
//
//	// Smooth S-curve
//	path := NewPath().
//	    MoveTo(100, 100).
//	    CubicTo(150, 50, 200, 150, 250, 100)
func (p *Path) CubicTo(c1x, c1y, c2x, c2y, x, y float64) *Path {
	if !p.hasCurrentPoint {
		panic("CubicTo called without current point (call MoveTo first)")
	}
	p.commands = append(p.commands, pathCommand{
		op:   pathOpCubicTo,
		args: []float64{c1x, c1y, c2x, c2y, x, y},
	})
	p.currentPoint = Point{X: x, Y: y}
	return p
}

// QuadraticTo appends a quadratic Bézier curve to the path.
//
// The curve goes from the current point to (x, y) using (cx, cy)
// as the control point.
//
// PDF does not support quadratic Bézier curves directly, so this method
// converts the quadratic curve to a cubic curve using the standard algorithm:
//
//	CP1 = P0 + 2/3 * (CP - P0)
//	CP2 = P1 + 2/3 * (CP - P1)
//
// Where P0 is the current point, CP is (cx, cy), and P1 is (x, y).
//
// Panics if there is no current point (call MoveTo first).
//
// Parameters:
//   - cx, cy: Control point
//   - x, y: Endpoint of the curve
//
// Example:
//
//	// Smooth parabolic curve
//	path := NewPath().
//	    MoveTo(100, 100).
//	    QuadraticTo(150, 50, 200, 100)
//
// Reference: Converting between quadratic and cubic curves.
func (p *Path) QuadraticTo(cx, cy, x, y float64) *Path {
	if !p.hasCurrentPoint {
		panic("QuadraticTo called without current point (call MoveTo first)")
	}

	// Convert quadratic to cubic Bézier
	// Quadratic: P0 → CP → P1
	// Cubic:    P0 → CP1 → CP2 → P1
	// Where:
	//   CP1 = P0 + 2/3 * (CP - P0)
	//   CP2 = P1 + 2/3 * (CP - P1)

	p0 := p.currentPoint
	cp1x := p0.X + (2.0/3.0)*(cx-p0.X)
	cp1y := p0.Y + (2.0/3.0)*(cy-p0.Y)
	cp2x := x + (2.0/3.0)*(cx-x)
	cp2y := y + (2.0/3.0)*(cy-y)

	return p.CubicTo(cp1x, cp1y, cp2x, cp2y, x, y)
}

// Close closes the current subpath.
//
// A straight line is appended from the current point to the subpath start.
// The subpath is marked as closed for filling purposes.
//
// PDF operator: h
//
// After Close(), there is no current point (call MoveTo to start a new subpath).
//
// Example:
//
//	// Closed triangle
//	path := NewPath().
//	    MoveTo(100, 100).
//	    LineTo(200, 100).
//	    LineTo(150, 200).
//	    Close()
func (p *Path) Close() *Path {
	if !p.hasCurrentPoint {
		// No current point, nothing to close
		return p
	}
	p.commands = append(p.commands, pathCommand{
		op:   pathOpClose,
		args: nil,
	})
	// After close, current point becomes invalid
	p.hasCurrentPoint = false
	return p
}

// AddRect adds a rectangle to the path.
//
// This is a convenience method that adds a closed rectangular subpath.
// The rectangle is drawn counterclockwise (for NonZero fill rule).
//
// PDF operator: x y width height re
//
// Parameters:
//   - rect: Rectangle to add
//
// Example:
//
//	path := NewPath().
//	    AddRect(Rect{X: 10, Y: 10, Width: 100, Height: 50}).
//	    AddRect(Rect{X: 150, Y: 10, Width: 100, Height: 50})
func (p *Path) AddRect(rect Rect) *Path {
	// Use PDF's optimized rectangle operator
	p.commands = append(p.commands, pathCommand{
		op:   pathOpRect,
		args: []float64{rect.X, rect.Y, rect.Width, rect.Height},
	})
	// Rectangle is a closed subpath, so no current point after
	p.hasCurrentPoint = false
	return p
}

// AddRoundedRect adds a rounded rectangle to the path.
//
// The rectangle has rounded corners with the specified radius.
// Corners are drawn as quarter-circle arcs.
//
// Parameters:
//   - rect: Rectangle bounds
//   - radius: Corner radius (must be non-negative)
//
// Example:
//
//	// Rounded rectangle with 10-unit radius corners
//	path := NewPath().AddRoundedRect(
//	    Rect{X: 10, Y: 10, Width: 100, Height: 50},
//	    10,
//	)
func (p *Path) AddRoundedRect(rect Rect, radius float64) *Path {
	if radius <= 0 {
		return p.AddRect(rect)
	}

	// Clamp radius to not exceed half the smallest dimension
	maxRadius := math.Min(rect.Width/2, rect.Height/2)
	if radius > maxRadius {
		radius = maxRadius
	}

	x, y, w, h := rect.X, rect.Y, rect.Width, rect.Height

	// Draw rounded rectangle as 4 lines + 4 arcs
	// Start at top-left corner (after radius)
	p.MoveTo(x+radius, y)

	// Top edge + top-right corner
	p.LineTo(x+w-radius, y)
	p.addArc(x+w-radius, y+radius, radius, 270, 360)

	// Right edge + bottom-right corner
	p.LineTo(x+w, y+h-radius)
	p.addArc(x+w-radius, y+h-radius, radius, 0, 90)

	// Bottom edge + bottom-left corner
	p.LineTo(x+radius, y+h)
	p.addArc(x+radius, y+h-radius, radius, 90, 180)

	// Left edge + top-left corner
	p.LineTo(x, y+radius)
	p.addArc(x+radius, y+radius, radius, 180, 270)

	return p.Close()
}

// AddCircle adds a circle to the path.
//
// The circle is approximated using 4 cubic Bézier curves.
// This provides a very accurate approximation (error < 0.06% of radius).
//
// Parameters:
//   - center: Circle center point
//   - radius: Circle radius (must be positive)
//
// Example:
//
//	path := NewPath().AddCircle(Point{150, 150}, 50)
func (p *Path) AddCircle(center Point, radius float64) *Path {
	if radius <= 0 {
		panic(fmt.Sprintf("AddCircle: radius must be positive, got: %f", radius))
	}
	return p.AddEllipse(Rect{
		X:      center.X - radius,
		Y:      center.Y - radius,
		Width:  radius * 2,
		Height: radius * 2,
	})
}

// AddEllipse adds an ellipse to the path.
//
// The ellipse is inscribed in the specified rectangle.
// It is approximated using 4 cubic Bézier curves.
//
// Parameters:
//   - rect: Bounding rectangle of the ellipse
//
// Example:
//
//	// Horizontal ellipse
//	path := NewPath().AddEllipse(Rect{X: 10, Y: 10, Width: 200, Height: 100})
func (p *Path) AddEllipse(rect Rect) *Path {
	// Magic number for approximating a circle with cubic Bézier curves
	// k = 4 * (sqrt(2) - 1) / 3 ≈ 0.5522847498
	const k = 0.5522847498

	cx := rect.X + rect.Width/2
	cy := rect.Y + rect.Height/2
	rx := rect.Width / 2
	ry := rect.Height / 2

	// Control point offset for each axis
	kx := rx * k
	ky := ry * k

	// Start at rightmost point (3 o'clock)
	p.MoveTo(cx+rx, cy)

	// Top-right quadrant (3 o'clock → 12 o'clock)
	p.CubicTo(cx+rx, cy-ky, cx+kx, cy-ry, cx, cy-ry)

	// Top-left quadrant (12 o'clock → 9 o'clock)
	p.CubicTo(cx-kx, cy-ry, cx-rx, cy-ky, cx-rx, cy)

	// Bottom-left quadrant (9 o'clock → 6 o'clock)
	p.CubicTo(cx-rx, cy+ky, cx-kx, cy+ry, cx, cy+ry)

	// Bottom-right quadrant (6 o'clock → 3 o'clock)
	p.CubicTo(cx+kx, cy+ry, cx+rx, cy+ky, cx+rx, cy)

	return p.Close()
}

// AddArc adds a circular arc to the path.
//
// The arc is centered at 'center' with the specified radius.
// It spans from startAngle to endAngle (in degrees).
// Angles are measured counterclockwise from the positive X axis.
//
// If there is a current point, a line is drawn from the current point
// to the arc start. Otherwise, MoveTo is used to start at the arc.
//
// Parameters:
//   - center: Arc center point
//   - radius: Arc radius (must be positive)
//   - startAngle: Starting angle in degrees (0 = right, 90 = top)
//   - endAngle: Ending angle in degrees
//
// Example:
//
//	// Quarter circle arc (90 degrees)
//	path := NewPath().
//	    MoveTo(150, 150).
//	    AddArc(Point{150, 150}, 50, 0, 90)
func (p *Path) AddArc(center Point, radius, startAngle, endAngle float64) *Path {
	if radius <= 0 {
		panic(fmt.Sprintf("AddArc: radius must be positive, got: %f", radius))
	}
	p.addArc(center.X, center.Y, radius, startAngle, endAngle)
	return p
}

// addArc is the internal implementation of AddArc.
// It adds an arc using cubic Bézier curve approximation.
func (p *Path) addArc(cx, cy, radius, startAngle, endAngle float64) {
	// Normalize angles
	for endAngle < startAngle {
		endAngle += 360
	}

	// Convert to radians
	start := startAngle * math.Pi / 180
	end := endAngle * math.Pi / 180
	totalAngle := end - start

	// Split arc into segments (max 90 degrees each for accuracy)
	segments := int(math.Ceil(totalAngle / (math.Pi / 2)))
	if segments < 1 {
		segments = 1
	}
	anglePerSegment := totalAngle / float64(segments)

	// Calculate first point
	x0 := cx + radius*math.Cos(start)
	y0 := cy + radius*math.Sin(start)

	if !p.hasCurrentPoint {
		p.MoveTo(x0, y0)
	} else {
		p.LineTo(x0, y0)
	}

	// Draw each segment as a cubic Bézier
	for i := 0; i < segments; i++ {
		a1 := start + float64(i)*anglePerSegment
		a2 := start + float64(i+1)*anglePerSegment

		// Calculate control points using the arc approximation formula
		alpha := math.Sin(a2-a1) * (math.Sqrt(4+3*math.Tan((a2-a1)/2)*math.Tan((a2-a1)/2)) - 1) / 3

		x1 := x0
		y1 := y0
		x2 := cx + radius*math.Cos(a2)
		y2 := cy + radius*math.Sin(a2)

		q1x := x1 - alpha*radius*math.Sin(a1)
		q1y := y1 + alpha*radius*math.Cos(a1)
		q2x := x2 + alpha*radius*math.Sin(a2)
		q2y := y2 - alpha*radius*math.Cos(a2)

		p.CubicTo(q1x, q1y, q2x, q2y, x2, y2)

		x0 = x2
		y0 = y2
	}
}

// IsEmpty returns true if the path has no commands.
//
// Example:
//
//	path := NewPath()
//	fmt.Println(path.IsEmpty()) // true
//	path.MoveTo(0, 0)
//	fmt.Println(path.IsEmpty()) // false
func (p *Path) IsEmpty() bool {
	return len(p.commands) == 0
}

// Bounds returns the bounding box of the path.
//
// The bounding box is the smallest rectangle that contains all points
// in the path. This includes line endpoints and Bézier curve control points
// (which may be outside the actual curve).
//
// Returns a zero Rect if the path is empty.
//
// Example:
//
//	path := NewPath().
//	    MoveTo(10, 10).
//	    LineTo(100, 50).
//	    LineTo(10, 90)
//	bounds := path.Bounds() // Rect{X: 10, Y: 10, Width: 90, Height: 80}
func (p *Path) Bounds() Rect {
	if len(p.commands) == 0 {
		return Rect{}
	}

	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64

	for _, cmd := range p.commands {
		switch cmd.op {
		case pathOpMoveTo, pathOpLineTo:
			// x, y
			x, y := cmd.args[0], cmd.args[1]
			minX, maxX = math.Min(minX, x), math.Max(maxX, x)
			minY, maxY = math.Min(minY, y), math.Max(maxY, y)

		case pathOpCubicTo:
			// c1x, c1y, c2x, c2y, x, y
			for i := 0; i < 6; i += 2 {
				x, y := cmd.args[i], cmd.args[i+1]
				minX, maxX = math.Min(minX, x), math.Max(maxX, x)
				minY, maxY = math.Min(minY, y), math.Max(maxY, y)
			}

		case pathOpRect:
			// x, y, width, height
			x, y, w, h := cmd.args[0], cmd.args[1], cmd.args[2], cmd.args[3]
			minX, maxX = math.Min(minX, x), math.Max(maxX, x+w)
			minY, maxY = math.Min(minY, y), math.Max(maxY, y+h)

		case pathOpClose:
			// No coordinates
		}
	}

	if minX > maxX {
		// No coordinates found
		return Rect{}
	}

	return Rect{
		X:      minX,
		Y:      minY,
		Width:  maxX - minX,
		Height: maxY - minY,
	}
}

// Clone creates a deep copy of the path.
//
// The cloned path is independent of the original.
//
// Example:
//
//	original := NewPath().MoveTo(0, 0).LineTo(100, 100)
//	clone := original.Clone()
//	clone.LineTo(200, 0) // Does not affect original
func (p *Path) Clone() *Path {
	clone := &Path{
		commands:        make([]pathCommand, len(p.commands)),
		currentPoint:    p.currentPoint,
		subpathStart:    p.subpathStart,
		hasCurrentPoint: p.hasCurrentPoint,
	}

	for i, cmd := range p.commands {
		clone.commands[i] = pathCommand{
			op:   cmd.op,
			args: append([]float64(nil), cmd.args...), // Deep copy args
		}
	}

	return clone
}

// toPDFOperators converts the path to PDF content stream operators.
//
// This is an internal method used by Surface to render the path.
func (p *Path) toPDFOperators() string {
	if len(p.commands) == 0 {
		return ""
	}

	result := ""
	for _, cmd := range p.commands {
		switch cmd.op {
		case pathOpMoveTo:
			result += fmt.Sprintf("%.2f %.2f m\n", cmd.args[0], cmd.args[1])
		case pathOpLineTo:
			result += fmt.Sprintf("%.2f %.2f l\n", cmd.args[0], cmd.args[1])
		case pathOpCubicTo:
			result += fmt.Sprintf("%.2f %.2f %.2f %.2f %.2f %.2f c\n",
				cmd.args[0], cmd.args[1], cmd.args[2], cmd.args[3], cmd.args[4], cmd.args[5])
		case pathOpClose:
			result += "h\n"
		case pathOpRect:
			result += fmt.Sprintf("%.2f %.2f %.2f %.2f re\n",
				cmd.args[0], cmd.args[1], cmd.args[2], cmd.args[3])
		}
	}
	return result
}
