package creator

import "github.com/coregx/gxpdf/internal/fonts"

// MeasureText returns the width of text in points for a Standard 14 font.
//
// This is used for layout calculations such as word wrapping, column sizing,
// and text alignment.
//
// Standard 14 text is drawn in WinAnsiEncoding (Symbol and ZapfDingbats: their
// built-in encodings); characters outside the encoding are drawn, and
// measured, as '?'. Use an embedded font (LoadFont) for other scripts.
//
// Returns 0 if the font is not recognized.
func MeasureText(font FontName, text string, size float64) float64 {
	return fonts.MeasureString(string(font), text, size)
}

// FontAscender returns the ascender height in points for a Standard 14 font.
//
// The ascender is the distance from the baseline to the top of the tallest
// character (typically uppercase letters and accented characters).
//
// Returns 0 if the font is not recognized.
func FontAscender(font FontName, size float64) float64 {
	m := fonts.GetMetrics(string(font))
	if m == nil {
		return 0
	}
	return float64(m.Ascender) * size / 1000.0
}

// FontDescender returns the descender depth in points for a Standard 14 font.
//
// The descender is the distance from the baseline to the bottom of descending
// characters (like 'g', 'p', 'y'). This value is negative.
//
// Returns 0 if the font is not recognized.
func FontDescender(font FontName, size float64) float64 {
	m := fonts.GetMetrics(string(font))
	if m == nil {
		return 0
	}
	return float64(m.Descender) * size / 1000.0
}

// FontCapHeight returns the cap height in points for a Standard 14 font.
//
// The cap height is the distance from the baseline to the top of uppercase
// letters like 'H' and 'I'.
//
// Returns 0 if the font is not recognized.
func FontCapHeight(font FontName, size float64) float64 {
	m := fonts.GetMetrics(string(font))
	if m == nil {
		return 0
	}
	return float64(m.CapHeight) * size / 1000.0
}

// FontLineHeight returns the natural line height in points for a Standard 14 font.
//
// Calculated as (Ascender - Descender) * size / 1000. This represents the
// minimum vertical space needed to display one line without overlapping.
//
// Returns 0 if the font is not recognized.
func FontLineHeight(font FontName, size float64) float64 {
	m := fonts.GetMetrics(string(font))
	if m == nil {
		return 0
	}
	return float64(m.Ascender-m.Descender) * size / 1000.0
}
