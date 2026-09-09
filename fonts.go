package gxpdf

import (
	"fmt"

	"github.com/coregx/gxpdf/internal/extractor"
)

// EmbeddedFont holds metadata and raw binary data for a font embedded in a PDF.
//
// The Data field contains the complete TTF/OTF binary after FlateDecode
// decompression and can be round-tripped through fonts.LoadTTFFromBytes:
//
//	embFonts, err := doc.GetEmbeddedFonts()
//	for _, ef := range embFonts {
//	    ttf, err := fonts.LoadTTFFromBytes(ef.Data)
//	    // ttf has the same metrics as fonts.LoadTTF would produce.
//	}
type EmbeddedFont struct {
	// Name is the PostScript name of the font as stored in the PDF /BaseFont entry.
	Name string

	// Subtype is the PDF font subtype: "TrueType", "CIDFontType2", etc.
	Subtype string

	// Data is the raw font binary (TTF/OTF bytes after FlateDecode decompression).
	// Nil when the font is not embedded (Standard 14 fonts such as Helvetica).
	Data []byte

	// Encoding is the PDF encoding name: "WinAnsiEncoding", "Identity-H", etc.
	// Empty string when no /Encoding entry is present.
	Encoding string
}

// GetEmbeddedFonts returns all embedded fonts found across all pages in the document.
//
// Duplicate fonts that appear on multiple pages are deduplicated by Name+Subtype
// and returned only once.
//
// Fonts that are not embedded (Standard 14 fonts such as Helvetica, Times-Roman,
// Courier) are not included in the result. An empty slice (not an error) is
// returned when the document has no embedded fonts.
//
// Example:
//
//	embFonts, err := doc.GetEmbeddedFonts()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, f := range embFonts {
//	    fmt.Printf("Font: %s (%s), %d bytes\n", f.Name, f.Subtype, len(f.Data))
//	}
func (d *Document) GetEmbeddedFonts() ([]EmbeddedFont, error) {
	fe := extractor.NewFontExtractorWithContext(d.reader, d.ctx)
	internal, err := fe.ExtractFromDocument()
	if err != nil {
		return nil, fmt.Errorf("gxpdf: extract embedded fonts: %w", err)
	}
	return convertEmbeddedFonts(internal), nil
}

// GetEmbeddedFontsForPage returns embedded fonts referenced on the given page (1-based).
//
// Returns an empty slice (not an error) when the page has no embedded fonts.
// Returns an error when the page number is out of range.
//
// Example:
//
//	fonts, err := doc.GetEmbeddedFontsForPage(1)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, f := range fonts {
//	    fmt.Printf("Page 1 font: %s (%s)\n", f.Name, f.Subtype)
//	}
func (d *Document) GetEmbeddedFontsForPage(pageNum int) ([]EmbeddedFont, error) {
	if pageNum < 1 || pageNum > d.PageCount() {
		return nil, fmt.Errorf("gxpdf: page %d out of range (1-%d)", pageNum, d.PageCount())
	}

	fe := extractor.NewFontExtractorWithContext(d.reader, d.ctx)
	internal, err := fe.ExtractFromPage(pageNum - 1) // convert to 0-based
	if err != nil {
		return nil, fmt.Errorf("gxpdf: extract embedded fonts from page %d: %w", pageNum, err)
	}
	return convertEmbeddedFonts(internal), nil
}

// convertEmbeddedFonts maps the internal extractor type to the public API type.
func convertEmbeddedFonts(internal []extractor.EmbeddedFont) []EmbeddedFont {
	if len(internal) == 0 {
		return nil
	}
	result := make([]EmbeddedFont, len(internal))
	for i, ef := range internal {
		result[i] = EmbeddedFont{
			Name:     ef.Name,
			Subtype:  ef.Subtype,
			Data:     ef.Data,
			Encoding: ef.Encoding,
		}
	}
	return result
}
