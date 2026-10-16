package fonts

import (
	"fmt"
	"io"
)

// Standard14Font represents one of the 14 Type 1 fonts built into all PDF readers.
// These fonts do not require embedding and are guaranteed to be available.
type Standard14Font struct {
	// Name is the PostScript name of the font (e.g., "Helvetica", "Times-Roman").
	Name string

	// Family is the font family name (e.g., "Helvetica", "Times").
	Family string

	// Weight is the font weight (e.g., "Regular", "Bold").
	Weight string

	// Style is the font style (e.g., "Normal", "Oblique", "Italic").
	Style string

	// IsSymbolic indicates whether this is a symbolic font (Symbol, ZapfDingbats).
	// Symbolic fonts use different encoding and character sets.
	IsSymbolic bool
}

// Family, weight and style names of the Standard 14 fonts.
const (
	familyTimes        = "Times"
	familyHelvetica    = "Helvetica"
	familyCourier      = "Courier"
	familySymbol       = "Symbol"
	familyZapfDingbats = "ZapfDingbats"

	weightRegular = "Regular"
	weightBold    = "Bold"

	styleNormal  = "Normal"
	styleItalic  = "Italic"
	styleOblique = "Oblique"
)

// Standard 14 Type 1 Fonts - Serif Family (Times).
var (
	// TimesRoman is the Times Roman font (Regular weight, Normal style).
	TimesRoman = &Standard14Font{
		Name:       "Times-Roman",
		Family:     familyTimes,
		Weight:     weightRegular,
		Style:      styleNormal,
		IsSymbolic: false,
	}

	// TimesBold is the Times Bold font.
	TimesBold = &Standard14Font{
		Name:       "Times-Bold",
		Family:     familyTimes,
		Weight:     weightBold,
		Style:      styleNormal,
		IsSymbolic: false,
	}

	// TimesItalic is the Times Italic font.
	TimesItalic = &Standard14Font{
		Name:       "Times-Italic",
		Family:     familyTimes,
		Weight:     weightRegular,
		Style:      styleItalic,
		IsSymbolic: false,
	}

	// TimesBoldItalic is the Times Bold Italic font.
	TimesBoldItalic = &Standard14Font{
		Name:       "Times-BoldItalic",
		Family:     familyTimes,
		Weight:     weightBold,
		Style:      styleItalic,
		IsSymbolic: false,
	}
)

// Standard 14 Type 1 Fonts - Sans-Serif Family (Helvetica).
var (
	// Helvetica is the Helvetica font (Regular weight, Normal style).
	Helvetica = &Standard14Font{
		Name:       "Helvetica",
		Family:     familyHelvetica,
		Weight:     weightRegular,
		Style:      styleNormal,
		IsSymbolic: false,
	}

	// HelveticaBold is the Helvetica Bold font.
	HelveticaBold = &Standard14Font{
		Name:       "Helvetica-Bold",
		Family:     familyHelvetica,
		Weight:     weightBold,
		Style:      styleNormal,
		IsSymbolic: false,
	}

	// HelveticaOblique is the Helvetica Oblique font.
	HelveticaOblique = &Standard14Font{
		Name:       "Helvetica-Oblique",
		Family:     familyHelvetica,
		Weight:     weightRegular,
		Style:      styleOblique,
		IsSymbolic: false,
	}

	// HelveticaBoldOblique is the Helvetica Bold Oblique font.
	HelveticaBoldOblique = &Standard14Font{
		Name:       "Helvetica-BoldOblique",
		Family:     familyHelvetica,
		Weight:     weightBold,
		Style:      styleOblique,
		IsSymbolic: false,
	}
)

// Standard 14 Type 1 Fonts - Monospace Family (Courier).
var (
	// Courier is the Courier font (Regular weight, Normal style).
	Courier = &Standard14Font{
		Name:       "Courier",
		Family:     familyCourier,
		Weight:     weightRegular,
		Style:      styleNormal,
		IsSymbolic: false,
	}

	// CourierBold is the Courier Bold font.
	CourierBold = &Standard14Font{
		Name:       "Courier-Bold",
		Family:     familyCourier,
		Weight:     weightBold,
		Style:      styleNormal,
		IsSymbolic: false,
	}

	// CourierOblique is the Courier Oblique font.
	CourierOblique = &Standard14Font{
		Name:       "Courier-Oblique",
		Family:     familyCourier,
		Weight:     weightRegular,
		Style:      styleOblique,
		IsSymbolic: false,
	}

	// CourierBoldOblique is the Courier Bold Oblique font.
	CourierBoldOblique = &Standard14Font{
		Name:       "Courier-BoldOblique",
		Family:     familyCourier,
		Weight:     weightBold,
		Style:      styleOblique,
		IsSymbolic: false,
	}
)

// Standard 14 Type 1 Fonts - Symbolic Fonts.
var (
	// Symbol is the Symbol font (symbolic characters, mathematical symbols).
	Symbol = &Standard14Font{
		Name:       "Symbol",
		Family:     familySymbol,
		Weight:     weightRegular,
		Style:      styleNormal,
		IsSymbolic: true,
	}

	// ZapfDingbats is the ZapfDingbats font (symbolic characters, dingbats).
	ZapfDingbats = &Standard14Font{
		Name:       "ZapfDingbats",
		Family:     familyZapfDingbats,
		Weight:     weightRegular,
		Style:      styleNormal,
		IsSymbolic: true,
	}
)

// PDFName returns the PostScript name of the font for use in PDF objects.
// This is the /BaseFont value in the font dictionary.
func (f *Standard14Font) PDFName() string {
	return f.Name
}

// WriteFontObject writes a PDF font object for this Standard 14 font.
// The object number is specified by objNum, and the output is written to w.
//
// Format:
//
//	5 0 obj
//	<< /Type /Font
//	   /Subtype /Type1
//	   /BaseFont /Helvetica
//	   /Encoding /WinAnsiEncoding
//	>>
//	endobj
//
// Note: Symbolic fonts (Symbol, ZapfDingbats) do not include /Encoding.
func (f *Standard14Font) WriteFontObject(objNum int, w io.Writer) error {
	// Write object header.
	if _, err := fmt.Fprintf(w, "%d 0 obj\n", objNum); err != nil {
		return fmt.Errorf("write object header: %w", err)
	}

	// Write font dictionary.
	if _, err := fmt.Fprintf(w, "<< /Type /Font\n"); err != nil {
		return fmt.Errorf("write font type: %w", err)
	}

	if _, err := fmt.Fprintf(w, "   /Subtype /Type1\n"); err != nil {
		return fmt.Errorf("write font subtype: %w", err)
	}

	if _, err := fmt.Fprintf(w, "   /BaseFont /%s\n", f.Name); err != nil {
		return fmt.Errorf("write base font: %w", err)
	}

	// Symbolic fonts (Symbol, ZapfDingbats) do not use WinAnsiEncoding.
	if !f.IsSymbolic {
		if _, err := fmt.Fprintf(w, "   /Encoding /WinAnsiEncoding\n"); err != nil {
			return fmt.Errorf("write encoding: %w", err)
		}
	}

	if _, err := fmt.Fprintf(w, ">>\n"); err != nil {
		return fmt.Errorf("write dictionary close: %w", err)
	}

	// Write object footer.
	if _, err := fmt.Fprintf(w, "endobj\n"); err != nil {
		return fmt.Errorf("write object footer: %w", err)
	}

	return nil
}
