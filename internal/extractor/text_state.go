package extractor

import (
	"fmt"
	"math"
)

// Matrix represents a transformation matrix used in PDF graphics and text.
//
// PDF uses 3x3 transformation matrices in homogeneous coordinate space:
//
//	[ a  b  0 ]
//	[ c  d  0 ]
//	[ e  f  1 ]
//
// The matrix is specified by six numbers: [a b c d e f]
//
// Transformations:
//   - Translation: [1 0 0 1 tx ty] - moves by (tx, ty)
//   - Scaling: [sx 0 0 sy 0 0] - scales by (sx, sy)
//   - Rotation: [cos θ sin θ -sin θ cos θ 0 0] - rotates by θ
//   - Skewing: [1 tan α tan β 1 0 0] - skews by angles α, β
//
// Reference: PDF 1.7 specification, Section 8.3.3 (Common Transformations).
type Matrix struct {
	A, B, C, D, E, F float64
}

// NewMatrix creates a new Matrix with the given values.
func NewMatrix(a, b, c, d, e, f float64) Matrix {
	return Matrix{A: a, B: b, C: c, D: d, E: e, F: f}
}

// Identity returns the identity matrix [1 0 0 1 0 0].
//
// The identity matrix performs no transformation.
func Identity() Matrix {
	return Matrix{A: 1, B: 0, C: 0, D: 1, E: 0, F: 0}
}

// Translation creates a translation matrix that moves by (tx, ty).
func Translation(tx, ty float64) Matrix {
	return Matrix{A: 1, B: 0, C: 0, D: 1, E: tx, F: ty}
}

// Scaling creates a scaling matrix that scales by (sx, sy).
func Scaling(sx, sy float64) Matrix {
	return Matrix{A: sx, B: 0, C: 0, D: sy, E: 0, F: 0}
}

// Rotation creates a rotation matrix that rotates by angle (in radians).
func Rotation(angle float64) Matrix {
	cos := math.Cos(angle)
	sin := math.Sin(angle)
	return Matrix{A: cos, B: sin, C: -sin, D: cos, E: 0, F: 0}
}

// Transform applies the matrix transformation to a point (x, y).
//
// The transformation formula is:
//
//	x' = a*x + c*y + e
//	y' = b*x + d*y + f
//
// This is used to convert text coordinates from text space to user space.
//
// Reference: PDF 1.7 specification, Section 8.3.2 (Coordinate Spaces).
func (m Matrix) Transform(x, y float64) (float64, float64) {
	nx := m.A*x + m.C*y + m.E
	ny := m.B*x + m.D*y + m.F
	return nx, ny
}

func (m Matrix) isFinite() bool {
	return isFiniteNumber(m.A) && isFiniteNumber(m.B) &&
		isFiniteNumber(m.C) && isFiniteNumber(m.D) &&
		isFiniteNumber(m.E) && isFiniteNumber(m.F)
}

// Multiply multiplies this matrix by another matrix (m * other).
//
// Matrix multiplication is used to combine transformations. Matrices act on
// column vectors, so m.Multiply(other) applies other first, then m.
//
// The formula for matrix multiplication:
//
//	[ a1 b1 0 ]   [ a2 b2 0 ]   [ a1*a2+b1*c2  a1*b2+b1*d2  0 ]
//	[ c1 d1 0 ] × [ c2 d2 0 ] = [ c1*a2+d1*c2  c1*b2+d1*d2  0 ]
//	[ e1 f1 1 ]   [ e2 f2 1 ]   [ e1*a2+f1*c2+e2  e1*b2+f1*d2+f2  1 ]
//
// Reference: PDF 1.7 specification, Section 8.3.4 (Transformation Matrices).
func (m Matrix) Multiply(other Matrix) Matrix {
	return Matrix{
		A: m.A*other.A + m.C*other.B,
		B: m.B*other.A + m.D*other.B,
		C: m.A*other.C + m.C*other.D,
		D: m.B*other.C + m.D*other.D,
		E: m.A*other.E + m.C*other.F + m.E,
		F: m.B*other.E + m.D*other.F + m.F,
	}
}

// IsIdentity checks if the matrix is the identity matrix.
func (m Matrix) IsIdentity() bool {
	const epsilon = 1e-6
	return math.Abs(m.A-1) < epsilon &&
		math.Abs(m.B) < epsilon &&
		math.Abs(m.C) < epsilon &&
		math.Abs(m.D-1) < epsilon &&
		math.Abs(m.E) < epsilon &&
		math.Abs(m.F) < epsilon
}

// String returns a string representation of the matrix.
func (m Matrix) String() string {
	return fmt.Sprintf("[%.3f %.3f %.3f %.3f %.3f %.3f]", m.A, m.B, m.C, m.D, m.E, m.F)
}

// TextState tracks the current text state during content stream parsing.
//
// The PDF text state includes all parameters that affect how text is rendered:
//   - Text matrix (Tm): Current text position and transformation
//   - Text line matrix (Tlm): Position of the start of the current line
//   - Font and size
//   - Character/word spacing
//   - Horizontal scaling
//   - Text leading (line spacing)
//   - Text rise (vertical offset)
//
// These parameters are modified by text operators (Tf, Tc, Tw, Tz, TL, Ts, Tm, Td, etc.)
// and affect how text showing operators (Tj, TJ, ', ") render text.
//
// Reference: PDF 1.7 specification, Section 9.3 (Text State Parameters).
type TextState struct {
	// Text matrices (Section 9.4.2)
	Tm  Matrix // Current text matrix
	Tlm Matrix // Text line matrix (start of line)

	// Text state parameters (Section 9.3)
	FontName   string  // Current font name (from Tf operator)
	FontSize   float64 // Current font size in points (from Tf operator)
	CharSpace  float64 // Character spacing (from Tc operator)
	WordSpace  float64 // Word spacing (from Tw operator)
	HorizScale float64 // Horizontal scaling as percentage (from Tz operator, 100 = normal)
	Leading    float64 // Text leading in points (from TL operator)
	Rise       float64 // Text rise in points (from Ts operator)

	// Current position (derived from Tm)
	CurrentX float64
	CurrentY float64
}

// NewTextState creates a new TextState with default values.
//
// Default values:
//   - Identity matrices for Tm and Tlm
//   - Empty font name, 0 font size
//   - 0 character spacing, word spacing, rise
//   - 100% horizontal scaling
//   - 0 leading
//
// Reference: PDF 1.7 specification, Section 9.3.1 (Text State Parameters and Operators).
func NewTextState() *TextState {
	return &TextState{
		Tm:         Identity(),
		Tlm:        Identity(),
		FontName:   "",
		FontSize:   0,
		CharSpace:  0,
		WordSpace:  0,
		HorizScale: 100, // 100% = normal
		Leading:    0,
		Rise:       0,
		CurrentX:   0,
		CurrentY:   0,
	}
}

// Reset resets the text state to default values.
//
// This is called when a BT (Begin Text) operator is encountered.
// According to the PDF spec, BT initializes the text matrix and line matrix to identity.
//
// Reference: PDF 1.7 specification, Section 9.4.1 (Text Objects).
func (ts *TextState) Reset() {
	ts.Tm = Identity()
	ts.Tlm = Identity()
	ts.CurrentX = 0
	ts.CurrentY = 0
	// Font and text state parameters are NOT reset by BT
}

// SetTextMatrix sets the text matrix (Tm operator).
//
// The Tm operator replaces the current text matrix with a new matrix specified
// by six numbers: Tm a b c d e f
//
// This also updates the text line matrix (Tlm = Tm).
//
// Reference: PDF 1.7 specification, Section 9.4.2 (Text Positioning Operators).
func (ts *TextState) SetTextMatrix(a, b, c, d, e, f float64) {
	ts.Tm = NewMatrix(a, b, c, d, e, f)
	ts.Tlm = ts.Tm
	ts.updateCurrentPosition()
}

// Translate moves the text position by (tx, ty) (Td operator).
//
// The Td operator updates both the text matrix and text line matrix:
//
//	Tlm = Tlm * [1 0 0 1 tx ty]
//	Tm = Tlm
//
// Reference: PDF 1.7 specification, Section 9.4.2 (Text Positioning Operators).
func (ts *TextState) Translate(tx, ty float64) {
	translation := Translation(tx, ty)
	ts.Tlm = ts.Tlm.Multiply(translation)
	ts.Tm = ts.Tlm
	ts.updateCurrentPosition()
}

// TranslateSetLeading moves the text position and sets leading (TD operator).
//
// The TD operator is equivalent to:
//   - TL -ty (set leading to -ty)
//   - Td tx ty (translate)
//
// Reference: PDF 1.7 specification, Section 9.4.2 (Text Positioning Operators).
func (ts *TextState) TranslateSetLeading(tx, ty float64) {
	ts.Leading = -ty
	ts.Translate(tx, ty)
}

// MoveToNextLine moves to the start of the next line (T* operator).
//
// The T* operator is equivalent to: Td 0 -Tl
// where Tl is the current leading.
//
// Reference: PDF 1.7 specification, Section 9.4.2 (Text Positioning Operators).
func (ts *TextState) MoveToNextLine() {
	ts.Translate(0, -ts.Leading)
}

// SetFont sets the current font and size (Tf operator).
//
// The Tf operator takes a font name and size:
//
//	Tf /FontName size
//
// Reference: PDF 1.7 specification, Section 9.3 (Text State Parameters).
func (ts *TextState) SetFont(fontName string, fontSize float64) {
	ts.FontName = fontName
	ts.FontSize = fontSize
}

// updateCurrentPosition updates CurrentX and CurrentY from the text matrix.
//
// The current position is the transformation of the origin (0, 0) by the text matrix.
func (ts *TextState) updateCurrentPosition() {
	ts.CurrentX, ts.CurrentY = ts.Tm.Transform(0, 0)
}

// AdvanceX advances the current X position by the given width.
//
// This is used when showing text to move the text position by the width of the text.
// The width should account for character spacing, word spacing, and horizontal scaling.
func (ts *TextState) AdvanceX(width float64) {
	// Update text matrix by translating in text space
	translation := Translation(width, 0)
	ts.Tm = ts.Tm.Multiply(translation)
	ts.updateCurrentPosition()
}

// String returns a string representation of the text state.
func (ts *TextState) String() string {
	return fmt.Sprintf("TextState{Tm=%s, font=%s, size=%.1f, pos=(%.2f, %.2f)}",
		ts.Tm.String(), ts.FontName, ts.FontSize, ts.CurrentX, ts.CurrentY)
}
