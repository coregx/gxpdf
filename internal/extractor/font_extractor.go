package extractor

import (
	"errors"
	"fmt"

	"github.com/coregx/gxpdf/internal/parser"
)

// ErrUnsupportedFontType is returned when a font type is not supported
// for data extraction (e.g., Type1 fonts stored in /FontFile).
//
// Callers treat this as a soft error: the font exists in the PDF but its
// binary data cannot be extracted by this implementation. GetEmbeddedFonts
// skips such fonts gracefully.
var ErrUnsupportedFontType = errors.New("extractor: unsupported font type for extraction")

// EmbeddedFont holds raw binary data and metadata for a font embedded in a PDF.
//
// The Data field contains the complete TTF/OTF binary after FlateDecode
// decompression. It can be round-tripped via fonts.LoadTTFFromBytes:
//
//	ttf, err := fonts.LoadTTFFromBytes(ef.Data)
//	// ttf matches the metrics you would get from fonts.LoadTTF.
type EmbeddedFont struct {
	// Name is the PostScript name of the font as stored in the PDF /BaseFont entry.
	Name string

	// Subtype is the PDF font subtype: "TrueType", "CIDFontType2", etc.
	Subtype string

	// Data is the raw font binary (TTF/OTF bytes after FlateDecode decompression).
	// Nil when the font is not embedded (Standard 14 fonts).
	Data []byte

	// Encoding is the PDF encoding name: "WinAnsiEncoding", "Identity-H", etc.
	// Empty string when no /Encoding entry is present.
	Encoding string
}

// FontExtractor walks PDF page resources and extracts embedded font data.
//
// Supported font types:
//   - Simple TrueType (/Subtype /TrueType) with /FontDescriptor → /FontFile2
//   - Composite CIDFontType2 (/Subtype /Type0) with /DescendantFonts → /FontDescriptor → /FontFile2
//
// Type1 and CFF fonts are skipped gracefully (no error is propagated).
type FontExtractor struct {
	reader *parser.Reader
}

// NewFontExtractor creates a FontExtractor backed by the given PDF reader.
func NewFontExtractor(reader *parser.Reader) *FontExtractor {
	return &FontExtractor{reader: reader}
}

// ExtractFromDocument extracts embedded fonts from all pages in the document.
//
// Duplicate fonts (same Name+Subtype) that appear on multiple pages are
// deduplicated — each unique font is returned only once.
func (fe *FontExtractor) ExtractFromDocument() ([]EmbeddedFont, error) {
	pageCount, err := fe.reader.GetPageCount()
	if err != nil {
		return nil, fmt.Errorf("font extractor: get page count: %w", err)
	}

	seen := make(map[string]struct{})
	var result []EmbeddedFont

	for i := 0; i < pageCount; i++ {
		pageFonts, err := fe.ExtractFromPage(i)
		if err != nil {
			// Skip pages that cannot be processed — not a fatal error.
			continue
		}
		for _, ef := range pageFonts {
			key := ef.Name + "\x00" + ef.Subtype
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, ef)
		}
	}

	return result, nil
}

// ExtractFromPage extracts embedded fonts from the specified page (0-based).
//
// Returns an empty slice (not an error) when the page has no embedded fonts.
func (fe *FontExtractor) ExtractFromPage(pageNum int) ([]EmbeddedFont, error) {
	page, err := fe.reader.GetPage(pageNum)
	if err != nil {
		return nil, fmt.Errorf("font extractor: get page %d: %w", pageNum, err)
	}

	fontsDict, err := fe.getPageFontsDict(page)
	if err != nil || fontsDict == nil {
		// No font resources on this page is a valid state.
		return nil, nil //nolint:nilerr
	}

	var result []EmbeddedFont
	for _, fontKey := range fontsDict.Keys() {
		fontObj := fontsDict.Get(fontKey)
		if fontObj == nil {
			continue
		}

		fontDict, err := fe.resolveDict(fontObj)
		if err != nil || fontDict == nil {
			continue
		}

		ef, err := fe.extractFontData(fontDict)
		if err != nil {
			// Unsupported or not embedded — skip gracefully.
			continue
		}
		if ef != nil {
			result = append(result, *ef)
		}
	}

	return result, nil
}

// ─── private helpers ──────────────────────────────────────────────────────────

// getPageFontsDict returns the /Font dictionary from a page's /Resources.
// Returns (nil, nil) when the page has no fonts — not an error.
func (fe *FontExtractor) getPageFontsDict(page *parser.Dictionary) (*parser.Dictionary, error) {
	resourcesObj := page.Get("Resources")
	if resourcesObj == nil {
		return nil, nil
	}

	resourcesDict, err := fe.resolveDict(resourcesObj)
	if err != nil || resourcesDict == nil {
		return nil, nil //nolint:nilerr
	}

	fontObj := resourcesDict.Get("Font")
	if fontObj == nil {
		return nil, nil
	}

	return fe.resolveDict(fontObj)
}

// extractFontData extracts the binary font data from a font dictionary.
//
// Walk order for composite (Type0) fonts:
//
//	Font dict → /DescendantFonts[0] → /FontDescriptor → /FontFile2
//
// Walk order for simple TrueType fonts:
//
//	Font dict → /FontDescriptor → /FontFile2
//
// Falls back to /FontFile and /FontFile3 in that order.
// Returns (nil, nil) for Standard-14 fonts (no embedded data).
//
//nolint:cyclop // Switch over font subtypes is inherently cyclomatic
func (fe *FontExtractor) extractFontData(fontDict *parser.Dictionary) (*EmbeddedFont, error) {
	subtype := fe.nameValue(fontDict.Get("Subtype"))
	fontName := fe.nameOrStringValue(fontDict.Get("BaseFont"))
	encoding := fe.encodingValue(fontDict.Get("Encoding"))

	var descriptorDict *parser.Dictionary
	var err error

	switch subtype {
	case type0FontSubtype:
		// Composite font: descriptor lives inside DescendantFonts.
		descriptorDict, err = fe.descriptorFromType0(fontDict)
		if err != nil || descriptorDict == nil {
			return nil, nil //nolint:nilerr
		}
		// Report subtype as the actual CID font type.
		subtype = "CIDFontType2"

	case "TrueType":
		descriptorDict, err = fe.descriptorFromSimple(fontDict)
		if err != nil || descriptorDict == nil {
			// No descriptor → Standard-14 or unsupported; skip silently.
			return nil, nil //nolint:nilerr
		}

	default:
		// Type1, MMType1, CIDFontType0, etc. — not supported for extraction.
		return nil, ErrUnsupportedFontType
	}

	data, err := fe.fontFileData(descriptorDict)
	if err != nil {
		return nil, err
	}
	if data == nil {
		// Descriptor present but no /FontFile2 stream — font not embedded.
		return nil, nil
	}

	return &EmbeddedFont{
		Name:     fontName,
		Subtype:  subtype,
		Data:     data,
		Encoding: encoding,
	}, nil
}

// descriptorFromType0 resolves the FontDescriptor for a Type0 composite font.
//
//	Type0 Font dict → /DescendantFonts array → [0] CIDFont dict → /FontDescriptor
func (fe *FontExtractor) descriptorFromType0(fontDict *parser.Dictionary) (*parser.Dictionary, error) {
	descendantsObj := fontDict.Get("DescendantFonts")
	if descendantsObj == nil {
		return nil, nil
	}

	arr, err := fe.resolveArray(descendantsObj)
	if err != nil || arr == nil || arr.Len() == 0 {
		return nil, nil //nolint:nilerr
	}

	cidFontDict, err := fe.resolveDict(arr.Get(0))
	if err != nil || cidFontDict == nil {
		return nil, nil //nolint:nilerr
	}

	return fe.descriptorFromSimple(cidFontDict)
}

// descriptorFromSimple resolves the FontDescriptor for a simple or CID font dict.
func (fe *FontExtractor) descriptorFromSimple(fontDict *parser.Dictionary) (*parser.Dictionary, error) {
	descObj := fontDict.Get("FontDescriptor")
	if descObj == nil {
		return nil, nil
	}
	return fe.resolveDict(descObj)
}

// fontFileData extracts and decodes the raw font bytes from a FontDescriptor.
//
// Tries /FontFile2 (TrueType), then /FontFile (Type1), then /FontFile3 (CFF).
// Returns (nil, nil) when none of the entries is present.
func (fe *FontExtractor) fontFileData(descriptor *parser.Dictionary) ([]byte, error) {
	for _, key := range []string{"FontFile2", "FontFile", "FontFile3"} {
		fileObj := descriptor.Get(key)
		if fileObj == nil {
			continue
		}

		stream, err := fe.resolveStream(fileObj)
		if err != nil || stream == nil {
			continue
		}

		data, err := decodeStreamData(stream)
		if err != nil {
			return nil, fmt.Errorf("decode font stream (%s): %w", key, err)
		}
		return data, nil
	}
	return nil, nil
}

// resolveDict resolves an indirect reference and casts to *parser.Dictionary.
func (fe *FontExtractor) resolveDict(obj parser.PdfObject) (*parser.Dictionary, error) {
	obj = fe.resolve(obj)
	if obj == nil {
		return nil, nil
	}
	dict, ok := obj.(*parser.Dictionary)
	if !ok {
		return nil, nil
	}
	return dict, nil
}

// resolveArray resolves an indirect reference and casts to *parser.Array.
func (fe *FontExtractor) resolveArray(obj parser.PdfObject) (*parser.Array, error) {
	obj = fe.resolve(obj)
	if obj == nil {
		return nil, nil
	}
	arr, ok := obj.(*parser.Array)
	if !ok {
		return nil, nil
	}
	return arr, nil
}

// resolveStream resolves an indirect reference and casts to *parser.Stream.
func (fe *FontExtractor) resolveStream(obj parser.PdfObject) (*parser.Stream, error) {
	obj = fe.resolve(obj)
	if obj == nil {
		return nil, nil
	}
	stream, ok := obj.(*parser.Stream)
	if !ok {
		return nil, nil
	}
	return stream, nil
}

// resolve follows a single indirect reference using the reader's object table.
// Non-reference objects are returned as-is. Returns nil on resolution error.
func (fe *FontExtractor) resolve(obj parser.PdfObject) parser.PdfObject {
	ref, ok := obj.(*parser.IndirectReference)
	if !ok {
		return obj
	}
	resolved, err := fe.reader.GetObject(ref.Number)
	if err != nil {
		return nil
	}
	return resolved
}

// nameValue extracts the string value from a *parser.Name object.
// Returns "" for nil or non-Name objects.
func (fe *FontExtractor) nameValue(obj parser.PdfObject) string {
	if obj == nil {
		return ""
	}
	n, ok := obj.(*parser.Name)
	if !ok {
		return ""
	}
	return n.Value()
}

// nameOrStringValue extracts a string from either a *parser.Name or *parser.String.
// Used for /BaseFont which is always a Name in spec but sometimes stored as String.
func (fe *FontExtractor) nameOrStringValue(obj parser.PdfObject) string {
	if obj == nil {
		return ""
	}
	switch v := obj.(type) {
	case *parser.Name:
		return v.Value()
	case *parser.String:
		return v.Value()
	}
	return ""
}

// encodingValue extracts an encoding name from a /Encoding entry.
// The entry can be a Name or a Dictionary (with /BaseEncoding).
func (fe *FontExtractor) encodingValue(obj parser.PdfObject) string {
	if obj == nil {
		return ""
	}
	obj = fe.resolve(obj)
	if obj == nil {
		return ""
	}
	switch v := obj.(type) {
	case *parser.Name:
		return v.Value()
	case *parser.Dictionary:
		// Encoding dictionary: read BaseEncoding.
		base := v.Get("BaseEncoding")
		if base != nil {
			if n, ok := base.(*parser.Name); ok {
				return n.Value()
			}
		}
	}
	return ""
}
