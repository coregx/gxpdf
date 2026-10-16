package creator

import (
	"fmt"

	"github.com/coregx/gxpdf/internal/fonts"
)

// CustomFont represents an embedded TrueType/OpenType font.
//
// Custom fonts allow you to use any TTF/OTF font file in your PDFs,
// including fonts with Unicode support (Cyrillic, CJK, etc.).
//
// The font is embedded as a subset containing only the glyphs
// used in the document, reducing file size.
//
// Example:
//
//	font, err := LoadFont("fonts/OpenSans-Regular.ttf")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	p := NewParagraph("Текст на русском")
//	p.SetCustomFont(font, 12)
type CustomFont struct {
	// ttfFont is the parsed TrueType font.
	ttfFont *fonts.TTFFont

	// subset is the font subset containing only used glyphs.
	subset *fonts.FontSubset

	// isBuilt indicates whether the subset has been built.
	isBuilt bool
}

// LoadFont loads a TrueType/OpenType font file.
//
// Supported formats:
//   - TrueType (.ttf)
//   - OpenType with TrueType outlines (.otf)
//
// Not yet supported:
//   - OpenType with CFF outlines (.otf with PostScript outlines)
//   - TrueType Collections (.ttc)
//
// Returns an error if the file cannot be read or is not a valid font.
func LoadFont(path string) (*CustomFont, error) {
	ttf, err := fonts.LoadTTF(path)
	if err != nil {
		return nil, fmt.Errorf("load TTF: %w", err)
	}

	return &CustomFont{
		ttfFont: ttf,
		subset:  fonts.NewFontSubset(ttf),
		isBuilt: false,
	}, nil
}

// LoadFontFromBytes loads a TrueType/OpenType font from its file contents,
// with the same format support as LoadFont. The font keeps a reference to
// data for embedding; the caller must not modify it afterwards.
//
// Returns an error if data is not a valid font.
func LoadFontFromBytes(data []byte) (*CustomFont, error) {
	ttf, err := fonts.LoadTTFFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("load TTF: %w", err)
	}

	return &CustomFont{
		ttfFont: ttf,
		subset:  fonts.NewFontSubset(ttf),
		isBuilt: false,
	}, nil
}

// UseChar marks a character as used (for subsetting).
//
// This is called automatically by text rendering functions.
// You don't need to call this manually.
func (f *CustomFont) UseChar(ch rune) {
	f.subset.UseChar(ch)
	f.isBuilt = false // Invalidate built subset.
}

// UseString marks all characters in a string as used.
//
// This is called automatically by text rendering functions.
// You don't need to call this manually.
func (f *CustomFont) UseString(text string) {
	f.subset.UseString(text)
	f.isBuilt = false // Invalidate built subset.
}

// MeasureString returns the width of a string in points at the given size.
//
// This is used for layout calculations (word wrapping, alignment, etc.).
func (f *CustomFont) MeasureString(text string, size float64) float64 {
	return f.subset.MeasureString(text, size)
}

// Build builds the font subset.
//
// This must be called before writing the PDF.
// It's automatically called by the Creator when finalizing the document.
func (f *CustomFont) Build() error {
	if f.isBuilt {
		return nil
	}

	if err := f.subset.Build(); err != nil {
		return fmt.Errorf("build subset: %w", err)
	}

	f.isBuilt = true
	return nil
}

// PostScriptName returns the PostScript name of the font.
//
// This is used as the font name in the PDF.
func (f *CustomFont) PostScriptName() string {
	return f.ttfFont.PostScriptName
}

// UnitsPerEm returns the units per em for this font.
func (f *CustomFont) UnitsPerEm() uint16 {
	return f.ttfFont.UnitsPerEm
}

// GetSubset returns the font subset (for internal use).
func (f *CustomFont) GetSubset() *fonts.FontSubset {
	return f.subset
}

// GetTTF returns the parsed TrueType font (for internal use).
func (f *CustomFont) GetTTF() *fonts.TTFFont {
	return f.ttfFont
}

// ID returns a unique identifier for this font instance.
//
// The ID is used to track fonts across pages and avoid duplicates.
func (f *CustomFont) ID() string {
	// Use PostScript name or derive from file path. A font loaded from bytes
	// without a PostScript name has neither; identify it by instance so two
	// such fonts are not merged.
	if f.ttfFont.PostScriptName != "" {
		return f.ttfFont.PostScriptName
	}
	if f.ttfFont.FilePath != "" {
		return f.ttfFont.FilePath
	}
	return fmt.Sprintf("font-%p", f.ttfFont)
}

// Ascender returns the ascender height in points at the given size.
//
// The ascender is the distance from the baseline to the top of the tallest
// character. Uses the font's hhea table Ascender value.
func (f *CustomFont) Ascender(size float64) float64 {
	return float64(f.ttfFont.Ascender) * size / float64(f.ttfFont.UnitsPerEm)
}

// Descender returns the descender depth in points at the given size.
//
// The descender is the distance from the baseline to the bottom of descending
// characters. This value is negative. Uses the font's hhea table Descender value.
func (f *CustomFont) Descender(size float64) float64 {
	return float64(f.ttfFont.Descender) * size / float64(f.ttfFont.UnitsPerEm)
}

// LineHeight returns the natural line height in points at the given size.
//
// Calculated as (Ascender - Descender) * size / UnitsPerEm.
func (f *CustomFont) LineHeight(size float64) float64 {
	return float64(f.ttfFont.Ascender-f.ttfFont.Descender) * size / float64(f.ttfFont.UnitsPerEm)
}

// CapHeight returns the cap height in points at the given size.
//
// The cap height is the height of uppercase letters like 'H'.
func (f *CustomFont) CapHeight(size float64) float64 {
	return float64(f.ttfFont.CapHeight) * size / float64(f.ttfFont.UnitsPerEm)
}
