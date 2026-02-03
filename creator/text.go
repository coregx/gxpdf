package creator

// Color represents an RGB color with values in the range [0.0, 1.0].
//
// PDF uses RGB color space where:
// - 0.0 = no intensity (black for all channels)
// - 1.0 = full intensity (white for all channels)
//
// Example:
//
//	black := Color{0, 0, 0}     // RGB(0, 0, 0)
//	white := Color{1, 1, 1}     // RGB(255, 255, 255)
//	red := Color{1, 0, 0}       // RGB(255, 0, 0)
//	gray := Color{0.5, 0.5, 0.5} // RGB(128, 128, 128)
type Color struct {
	R float64 // Red component (0.0 to 1.0)
	G float64 // Green component (0.0 to 1.0)
	B float64 // Blue component (0.0 to 1.0)
}

// Predefined colors for common use cases.
var (
	// Black is pure black (RGB: 0, 0, 0).
	Black = Color{0, 0, 0}

	// White is pure white (RGB: 255, 255, 255).
	White = Color{1, 1, 1}

	// Red is pure red (RGB: 255, 0, 0).
	Red = Color{1, 0, 0}

	// Green is pure green (RGB: 0, 255, 0).
	Green = Color{0, 1, 0}

	// Blue is pure blue (RGB: 0, 0, 255).
	Blue = Color{0, 0, 1}

	// Gray is 50% gray (RGB: 128, 128, 128).
	Gray = Color{0.5, 0.5, 0.5}

	// DarkGray is 25% gray (RGB: 64, 64, 64).
	DarkGray = Color{0.25, 0.25, 0.25}

	// LightGray is 75% gray (RGB: 192, 192, 192).
	LightGray = Color{0.75, 0.75, 0.75}

	// Yellow is pure yellow (RGB: 255, 255, 0).
	Yellow = Color{1, 1, 0}

	// Cyan is pure cyan (RGB: 0, 255, 255).
	Cyan = Color{0, 1, 1}

	// Magenta is pure magenta (RGB: 255, 0, 255).
	Magenta = Color{1, 0, 1}
)

// ColorCMYK represents a CMYK color with values in the range [0.0, 1.0].
//
// CMYK (Cyan, Magenta, Yellow, blacK) is a subtractive color model used in
// professional printing. Unlike RGB (additive), CMYK works by subtracting
// colors from white.
//
// PDF supports CMYK color space natively via DeviceCMYK.
//
// Example:
//
//	black := ColorCMYK{0, 0, 0, 1}       // 100% black
//	white := ColorCMYK{0, 0, 0, 0}       // No ink (paper white)
//	cyan := ColorCMYK{1, 0, 0, 0}        // 100% cyan
//	magenta := ColorCMYK{0, 1, 0, 0}     // 100% magenta
//	yellow := ColorCMYK{0, 0, 1, 0}      // 100% yellow
//	red := ColorCMYK{0, 1, 1, 0}         // Magenta + Yellow = Red
//
// Reference: PDF 1.7 Specification, Section 8.6.4.4 (DeviceCMYK Color Space).
type ColorCMYK struct {
	C float64 // Cyan component (0.0 to 1.0)
	M float64 // Magenta component (0.0 to 1.0)
	Y float64 // Yellow component (0.0 to 1.0)
	K float64 // blacK component (0.0 to 1.0)
}

// NewColorCMYK creates a new CMYK color.
//
// Parameters:
//   - c: Cyan component (0.0 to 1.0)
//   - m: Magenta component (0.0 to 1.0)
//   - y: Yellow component (0.0 to 1.0)
//   - k: blacK component (0.0 to 1.0)
//
// Example:
//
//	cyan := creator.NewColorCMYK(1.0, 0.0, 0.0, 0.0)
//	red := creator.NewColorCMYK(0.0, 1.0, 1.0, 0.0)
func NewColorCMYK(c, m, y, k float64) ColorCMYK {
	return ColorCMYK{C: c, M: m, Y: y, K: k}
}

// ToRGB converts a CMYK color to RGB.
//
// Conversion formula:
//
//	R = (1 - C) * (1 - K)
//	G = (1 - M) * (1 - K)
//	B = (1 - Y) * (1 - K)
//
// Note: This is a simple conversion. For precise color matching in
// professional printing, use ICC color profiles.
//
// Example:
//
//	cmyk := ColorCMYK{0, 1, 1, 0} // Red in CMYK
//	rgb := cmyk.ToRGB()            // Converts to RGB{1, 0, 0}
func (c ColorCMYK) ToRGB() Color {
	r := (1.0 - c.C) * (1.0 - c.K)
	g := (1.0 - c.M) * (1.0 - c.K)
	b := (1.0 - c.Y) * (1.0 - c.K)
	return Color{R: r, G: g, B: b}
}

// ToCMYK converts an RGB color to CMYK.
//
// Conversion formula:
//
//	K = 1 - max(R, G, B)
//	C = (1 - R - K) / (1 - K)
//	M = (1 - G - K) / (1 - K)
//	Y = (1 - B - K) / (1 - K)
//
// Special case: Pure black (R=0, G=0, B=0) is represented as K=1, C=M=Y=0.
//
// Note: This is a simple conversion. For precise color matching in
// professional printing, use ICC color profiles.
//
// Example:
//
//	rgb := Color{1, 0, 0}  // Red in RGB
//	cmyk := rgb.ToCMYK()   // Converts to CMYK{0, 1, 1, 0}
func (c Color) ToCMYK() ColorCMYK {
	// Find the maximum component to calculate K
	maxComponent := c.R
	if c.G > maxComponent {
		maxComponent = c.G
	}
	if c.B > maxComponent {
		maxComponent = c.B
	}

	// Calculate K (black component)
	k := 1.0 - maxComponent

	// Special case: pure black (avoid division by zero)
	if k >= 1.0 {
		return ColorCMYK{C: 0, M: 0, Y: 0, K: 1}
	}

	// Calculate CMY components
	cyan := (1.0 - c.R - k) / (1.0 - k)
	magenta := (1.0 - c.G - k) / (1.0 - k)
	yellow := (1.0 - c.B - k) / (1.0 - k)

	return ColorCMYK{C: cyan, M: magenta, Y: yellow, K: k}
}

// Predefined CMYK colors for common print use cases.
var (
	// CMYKBlack is pure black (100% black ink).
	CMYKBlack = ColorCMYK{0, 0, 0, 1}

	// CMYKWhite is white (no ink - paper color).
	CMYKWhite = ColorCMYK{0, 0, 0, 0}

	// CMYKCyan is pure cyan (100% cyan ink).
	CMYKCyan = ColorCMYK{1, 0, 0, 0}

	// CMYKMagenta is pure magenta (100% magenta ink).
	CMYKMagenta = ColorCMYK{0, 1, 0, 0}

	// CMYKYellow is pure yellow (100% yellow ink).
	CMYKYellow = ColorCMYK{0, 0, 1, 0}

	// CMYKRed is red (magenta + yellow).
	CMYKRed = ColorCMYK{0, 1, 1, 0}

	// CMYKGreen is green (cyan + yellow).
	CMYKGreen = ColorCMYK{1, 0, 1, 0}

	// CMYKBlue is blue (cyan + magenta).
	CMYKBlue = ColorCMYK{1, 1, 0, 0}
)

// ColorRGBA represents an RGBA color with alpha channel (transparency).
//
// All values are in the range [0.0, 1.0]:
// - R, G, B: Color components (0.0 = no intensity, 1.0 = full intensity)
// - A: Alpha channel (0.0 = fully transparent, 1.0 = fully opaque)
//
// PDF uses ExtGState (Extended Graphics State) with /ca parameter to implement
// transparency. The alpha value controls both fill and stroke opacity.
//
// Example:
//
//	// Semi-transparent red
//	transparentRed := ColorRGBA{1, 0, 0, 0.5}
//
//	// Fully transparent (invisible)
//	invisible := ColorRGBA{0, 0, 0, 0}
//
//	// Fully opaque (same as Color)
//	opaque := ColorRGBA{1, 0, 0, 1}
//
// Reference: PDF 1.7 Specification, Section 8.4.5 (Extended Graphics State).
type ColorRGBA struct {
	R float64 // Red component (0.0 to 1.0)
	G float64 // Green component (0.0 to 1.0)
	B float64 // Blue component (0.0 to 1.0)
	A float64 // Alpha component (0.0 = transparent, 1.0 = opaque)
}

// NewColorRGBA creates a new RGBA color with alpha channel.
//
// Parameters:
//   - r: Red component (0.0 to 1.0)
//   - g: Green component (0.0 to 1.0)
//   - b: Blue component (0.0 to 1.0)
//   - a: Alpha component (0.0 = transparent, 1.0 = opaque)
//
// Example:
//
//	// 50% transparent red
//	red := creator.NewColorRGBA(1.0, 0.0, 0.0, 0.5)
//
//	// 30% transparent blue
//	blue := creator.NewColorRGBA(0.0, 0.0, 1.0, 0.3)
func NewColorRGBA(r, g, b, a float64) ColorRGBA {
	return ColorRGBA{R: r, G: g, B: b, A: a}
}

// ToColor converts RGBA to RGB (discards alpha channel).
//
// Returns the RGB color without transparency.
func (c ColorRGBA) ToColor() Color {
	return Color{R: c.R, G: c.G, B: c.B}
}

// WithAlpha returns a new ColorRGBA with the specified alpha value.
//
// Parameters:
//   - alpha: New alpha value (0.0 to 1.0)
//
// Example:
//
//	red := ColorRGBA{1, 0, 0, 1}
//	transparentRed := red.WithAlpha(0.5)  // 50% transparent red
func (c ColorRGBA) WithAlpha(alpha float64) ColorRGBA {
	return ColorRGBA{R: c.R, G: c.G, B: c.B, A: alpha}
}

// Predefined transparent colors for common use cases.
var (
	// TransparentBlack is fully transparent black (RGBA: 0, 0, 0, 0).
	TransparentBlack = ColorRGBA{0, 0, 0, 0}

	// SemiTransparentBlack is 50% transparent black.
	SemiTransparentBlack = ColorRGBA{0, 0, 0, 0.5}

	// TransparentWhite is fully transparent white.
	TransparentWhite = ColorRGBA{1, 1, 1, 0}

	// SemiTransparentWhite is 50% transparent white.
	SemiTransparentWhite = ColorRGBA{1, 1, 1, 0.5}

	// TransparentRed is fully transparent red.
	TransparentRed = ColorRGBA{1, 0, 0, 0}

	// SemiTransparentRed is 50% transparent red.
	SemiTransparentRed = ColorRGBA{1, 0, 0, 0.5}

	// TransparentGreen is fully transparent green.
	TransparentGreen = ColorRGBA{0, 1, 0, 0}

	// SemiTransparentGreen is 50% transparent green.
	SemiTransparentGreen = ColorRGBA{0, 1, 0, 0.5}

	// TransparentBlue is fully transparent blue.
	TransparentBlue = ColorRGBA{0, 0, 1, 0}

	// SemiTransparentBlue is 50% transparent blue.
	SemiTransparentBlue = ColorRGBA{0, 0, 1, 0.5}
)

// FontName represents one of the Standard 14 fonts built into all PDF readers.
//
// These fonts do not require embedding and are guaranteed to be available
// in all PDF viewers.
//
// Reference: PDF 1.7 Specification, Section 9.6.2.2 (Standard Type 1 Fonts).
type FontName string

// Standard 14 fonts - Serif family (Times).
const (
	// TimesRoman is Times Roman (serif, regular weight, normal style).
	TimesRoman FontName = "Times-Roman"

	// TimesBold is Times Bold (serif, bold weight, normal style).
	TimesBold FontName = "Times-Bold"

	// TimesItalic is Times Italic (serif, regular weight, italic style).
	TimesItalic FontName = "Times-Italic"

	// TimesBoldItalic is Times Bold Italic (serif, bold weight, italic style).
	TimesBoldItalic FontName = "Times-BoldItalic"
)

// Standard 14 fonts - Sans-serif family (Helvetica).
const (
	// Helvetica is Helvetica (sans-serif, regular weight, normal style).
	Helvetica FontName = "Helvetica"

	// HelveticaBold is Helvetica Bold (sans-serif, bold weight, normal style).
	HelveticaBold FontName = "Helvetica-Bold"

	// HelveticaOblique is Helvetica Oblique (sans-serif, regular weight, oblique style).
	HelveticaOblique FontName = "Helvetica-Oblique"

	// HelveticaBoldOblique is Helvetica Bold Oblique (sans-serif, bold weight, oblique style).
	HelveticaBoldOblique FontName = "Helvetica-BoldOblique"
)

// Standard 14 fonts - Monospace family (Courier).
const (
	// Courier is Courier (monospace, regular weight, normal style).
	Courier FontName = "Courier"

	// CourierBold is Courier Bold (monospace, bold weight, normal style).
	CourierBold FontName = "Courier-Bold"

	// CourierOblique is Courier Oblique (monospace, regular weight, oblique style).
	CourierOblique FontName = "Courier-Oblique"

	// CourierBoldOblique is Courier Bold Oblique (monospace, bold weight, oblique style).
	CourierBoldOblique FontName = "Courier-BoldOblique"
)

// Standard 14 fonts - Symbolic fonts.
const (
	// Symbol is Symbol font (symbolic characters, mathematical symbols).
	Symbol FontName = "Symbol"

	// ZapfDingbats is ZapfDingbats font (symbolic characters, dingbats).
	ZapfDingbats FontName = "ZapfDingbats"
)

// TextOperation represents a text drawing operation to be added to a page.
//
// Each TextOperation describes how to render a single text string
// at a specific position with a specific font, size, and color.
//
// Example:
//
//	op := TextOperation{
//	    Text:  "Hello World",
//	    X:     100,
//	    Y:     700,
//	    Font:  Helvetica,
//	    Size:  24,
//	    Color: Black,
//	}
type TextOperation struct {
	// Text is the string to display.
	Text string

	// X is the horizontal position in points (from left edge of page).
	X float64

	// Y is the vertical position in points (from bottom edge of page).
	Y float64

	// Font is the font to use (one of the Standard 14 fonts).
	// Ignored if CustomFont is set.
	Font FontName

	// CustomFont is an embedded TrueType/OpenType font (optional).
	// When set, this takes precedence over Font field.
	// Use for Unicode text (Cyrillic, CJK, symbols, etc.).
	CustomFont *CustomFont

	// Size is the font size in points.
	Size float64

	// Color is the text color (RGB, 0.0 to 1.0 range).
	// If ColorCMYK is set, this field is ignored.
	Color Color

	// ColorCMYK is the text color in CMYK color space (optional).
	// If set, this takes precedence over Color (RGB).
	// Used for professional printing workflows.
	ColorCMYK *ColorCMYK

	// Opacity is the text opacity (0.0 = transparent, 1.0 = opaque).
	// Optional. If set, applies transparency via ExtGState.
	// Works with both Color and ColorCMYK.
	// Range: [0.0, 1.0]
	Opacity *float64
}
