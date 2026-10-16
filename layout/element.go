// Package layout is a pure computation package implementing the GxPDF layout
// engine. It has zero imports from creator/ or any PDF-specific package,
// enabling testing of layout logic without PDF generation and supporting
// swappable rendering backends.
//
// The core concept is the Element interface: each element receives an Area
// describing available space and returns an immutable Plan describing what
// fits, how much space was consumed, and an Overflow element carrying any
// remainder for the next page.
package layout

// Element is the core layout interface. Every layoutable type implements it.
// PlanLayout must be pure — no side effects, no mutation of the receiver.
// The caller supplies the available Area; the returned Plan describes what
// was placed and what (if anything) could not fit. Implementations decide
// what fits with Fits, so that all elements apply the same tolerance.
type Element interface {
	PlanLayout(area Area) Plan
}

// Area represents available space for a layout computation.
type Area struct {
	// Width is the available horizontal space in PDF points.
	Width float64
	// Height is the remaining vertical space on the current page in PDF points.
	Height float64
	// AtTop reports that nothing has been placed on the current page yet, so
	// moving content to the next page cannot help. An element that would
	// return Nothing (to be retried on a fresh page) must place what it can
	// instead when AtTop is true; an element that avoids breaks, such as a
	// table row-span group, must break.
	AtTop bool
}

// Plan is the immutable result of a layout computation.
type Plan struct {
	// Status indicates how much content fit in the available area.
	Status Status
	// Consumed is the vertical space used by this plan in PDF points.
	Consumed float64
	// Blocks holds the positioned, ready-to-render content atoms.
	Blocks []Block
	// Overflow carries the portion of the element that did not fit.
	// It is nil when Status == Full.
	Overflow Element
}

// Status indicates how much content fit in the available area during layout.
type Status int

const (
	// Full means all content fit within the area.
	Full Status = iota
	// Partial means some content fit; Plan.Overflow contains the remainder.
	Partial
	// Nothing means no content fit at all (not even one line or row).
	Nothing
)

// Block is a positioned piece of content ready for rendering. It is the
// atomic rendering unit produced by layout and consumed by the renderer.
type Block struct {
	// X and Y are the position of this block within its parent coordinate space,
	// measured in PDF points from the top-left corner.
	X, Y float64
	// Width and Height are the block's dimensions in PDF points.
	Width, Height float64
	// Draw is the rendering closure. It captures all layout data needed to
	// emit PDF content and calls Renderer methods when invoked.
	Draw func(r Renderer)
	// Children holds nested blocks, enabling a recursive block tree for
	// PDF/UA tagged PDF structure (P, H1, Table, etc.).
	Children []Block
	// Tag is the PDF structure element tag (e.g. "P", "H1", "Table", "TD").
	// Empty string means untagged.
	Tag string
	// AltText is the accessibility alternative text for images and figures.
	AltText string
	// Links contains clickable regions within this block.
	Links []LinkArea
	// drawData holds captured rendering data for special blocks (e.g. page numbers)
	// that need their Draw closure rebuilt after pagination. Unexported.
	drawData *pageNumberDrawData
}

// LinkArea defines a hyperlink region within a Block.
type LinkArea struct {
	// X and Y are the offset from the parent Block's top-left corner.
	X, Y float64
	// Width and Height define the clickable region.
	Width, Height float64
	// URL is the target of the hyperlink.
	URL string
}

// Renderer is an abstract PDF rendering target. Layout closures call these
// methods to emit content without importing any PDF-specific package.
// The concrete implementation in builder/internal bridges to creator/.
type Renderer interface {
	// DrawText renders a text string at the given position using the
	// specified font, size, and color with optional decoration options.
	DrawText(text string, x, y float64, font FontRef, size float64, color Color, options TextDrawOptions)
	// DrawRect renders a filled and/or stroked rectangle.
	DrawRect(x, y, width, height float64, fill *Color, stroke *Color, strokeWidth float64)
	// DrawLine renders a line segment.
	DrawLine(x1, y1, x2, y2 float64, color Color, width float64)
	// DrawImage renders image data (PNG, JPEG) scaled to the given bounds.
	DrawImage(data []byte, x, y, width, height float64)
	// PushState saves the current graphics state.
	PushState()
	// PopState restores the most recently saved graphics state.
	PopState()
	// SetClipRect defines a rectangular clipping region.
	SetClipRect(x, y, width, height float64)
}

// TextDrawOptions carries optional text decoration parameters for DrawText.
type TextDrawOptions struct {
	// LetterSpacing adds extra spacing between each character in points.
	LetterSpacing float64
	// WordSpacing adds extra spacing between words in points (used for justification).
	WordSpacing float64
	// Underline requests an underline decoration.
	Underline bool
	// Strikethrough requests a strikethrough decoration.
	Strikethrough bool
}

// Measurable is an optional interface that elements may implement to
// report their intrinsic width range. This is used by table column
// auto-sizing and flow layout algorithms.
type Measurable interface {
	// MinWidth returns the minimum width the element can occupy without
	// losing content (e.g. width of the longest unbreakable word).
	MinWidth() float64
	// MaxWidth returns the width the element would occupy if given
	// unlimited horizontal space (e.g. full text on one line).
	MaxWidth() float64
}

// Color represents an RGB color with components in the [0, 1] range.
type Color struct {
	// R is the red component in [0, 1].
	R float64
	// G is the green component in [0, 1].
	G float64
	// B is the blue component in [0, 1].
	B float64
}

// Black is the standard black color (0, 0, 0).
var Black = Color{R: 0, G: 0, B: 0}

// White is the standard white color (1, 1, 1).
var White = Color{R: 1, G: 1, B: 1}

// RGB constructs a Color from 0-1 float components.
func RGB(r, g, b float64) Color {
	return Color{R: r, G: g, B: b}
}

// RGB255 constructs a Color from 0-255 integer components.
func RGB255(r, g, b uint8) Color {
	return Color{
		R: float64(r) / 255,
		G: float64(g) / 255,
		B: float64(b) / 255,
	}
}
