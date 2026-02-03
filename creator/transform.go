package creator

import "math"

// Transform represents a 2D affine transformation matrix.
//
// PDF uses column-major matrix notation:
//
//	[ a  b  0 ]
//	[ c  d  0 ]
//	[ e  f  1 ]
//
// Where:
//
//	a, d: Scale
//	b, c: Skew/Rotation
//	e, f: Translation
type Transform struct {
	A, B, C, D, E, F float64
}

// Identity returns the identity transformation (no change).
func Identity() Transform {
	return Transform{
		A: 1, B: 0,
		C: 0, D: 1,
		E: 0, F: 0,
	}
}

// Translate returns a translation transformation.
//
// Example:
//
//	t := Translate(100, 200)  // Move right 100, up 200
func Translate(tx, ty float64) Transform {
	return Transform{
		A: 1, B: 0,
		C: 0, D: 1,
		E: tx, F: ty,
	}
}

// Scale returns a scaling transformation.
//
// Example:
//
//	t := Scale(2, 2)     // Double size
//	t := Scale(1, -1)    // Flip vertically
func Scale(sx, sy float64) Transform {
	return Transform{
		A: sx, B: 0,
		C: 0, D: sy,
		E: 0, F: 0,
	}
}

// Rotate returns a rotation transformation.
//
// The angle is in degrees, clockwise.
//
// Example:
//
//	t := Rotate(45)   // Rotate 45° clockwise
//	t := Rotate(-90)  // Rotate 90° counter-clockwise
func Rotate(degrees float64) Transform {
	rad := degrees * math.Pi / 180.0
	cos := math.Cos(rad)
	sin := math.Sin(rad)

	return Transform{
		A: cos, B: sin,
		C: -sin, D: cos,
		E: 0, F: 0,
	}
}

// RotateAround returns a rotation transformation around a specific point.
//
// Example:
//
//	t := RotateAround(45, 100, 200)  // Rotate 45° around point (100, 200)
func RotateAround(degrees, cx, cy float64) Transform {
	// Translate to origin, rotate, translate back
	t1 := Translate(-cx, -cy)
	rot := Rotate(degrees)
	t2 := Translate(cx, cy)

	return t1.Then(rot).Then(t2)
}

// Skew returns a skew transformation.
//
// Example:
//
//	t := Skew(15, 0)  // Skew horizontally by 15°
func Skew(angleX, angleY float64) Transform {
	radX := angleX * math.Pi / 180.0
	radY := angleY * math.Pi / 180.0

	return Transform{
		A: 1, B: math.Tan(radY),
		C: math.Tan(radX), D: 1,
		E: 0, F: 0,
	}
}

// Then combines this transformation with another.
//
// The result is equivalent to applying this transform first, then the other.
//
// Example:
//
//	t := Translate(100, 0).Then(Rotate(45))  // Move, then rotate
func (t Transform) Then(other Transform) Transform {
	// Matrix multiplication: this * other
	return Transform{
		A: t.A*other.A + t.B*other.C,
		B: t.A*other.B + t.B*other.D,
		C: t.C*other.A + t.D*other.C,
		D: t.C*other.B + t.D*other.D,
		E: t.E*other.A + t.F*other.C + other.E,
		F: t.E*other.B + t.F*other.D + other.F,
	}
}

// TransformPoint applies the transformation to a point.
//
// Returns the transformed coordinates.
func (t Transform) TransformPoint(x, y float64) (float64, float64) {
	return t.A*x + t.C*y + t.E,
		t.B*x + t.D*y + t.F
}

// ToPDFMatrix returns the transformation as a PDF CTM (Current Transformation Matrix).
//
// Returns 6 values: [a b c d e f]
func (t Transform) ToPDFMatrix() [6]float64 {
	return [6]float64{t.A, t.B, t.C, t.D, t.E, t.F}
}
