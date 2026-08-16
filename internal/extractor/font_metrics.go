package extractor

import "github.com/coregx/gxpdf/internal/parser"

const type0FontSubtype = "Type0"

// fontMetrics contains glyph advances in the PDF's standard 1000-unit text
// space. Simple fonts define /FirstChar and /Widths; composite Type0 fonts
// define /DW and /W on their descendant CIDFont.
type fontMetrics struct {
	widths       map[uint16]float64
	defaultWidth float64
	precise      bool
}

func (fm *fontMetrics) width(glyphID uint16) float64 {
	if fm == nil {
		return 600
	}
	if width, ok := fm.widths[glyphID]; ok {
		return width
	}
	return fm.defaultWidth
}

// measureGlyphBytes returns the text advance, visible width, and whether the
// width came from authoritative PDF font metrics.
func (te *TextExtractor) measureGlyphBytes(glyphBytes []byte) (float64, float64, bool) {
	decoder := te.fontDecoders[te.textState.FontName]
	metrics := te.fontMetrics[te.textState.FontName]
	horizontalScale := te.textState.HorizScale / 100.0

	advance := 0.0
	visibleWidth := 0.0
	for position := 0; position < len(glyphBytes); {
		glyphID, bytesRead := uint16(glyphBytes[position]), 1
		if decoder != nil {
			glyphID, bytesRead = decoder.readGlyphID(glyphBytes[position:])
		}
		if bytesRead <= 0 {
			break
		}

		glyphWidth := metrics.width(glyphID) / 1000.0 * te.textState.FontSize * horizontalScale
		visibleWidth = advance + glyphWidth

		spacing := te.textState.CharSpace
		if glyphID == 32 && (decoder == nil || !decoder.isCompositeFont) {
			spacing += te.textState.WordSpace
		}
		advance = visibleWidth + spacing*horizontalScale
		position += bytesRead
	}

	return advance, visibleWidth, metrics != nil && metrics.precise
}

func (te *TextExtractor) loadFontMetrics(fontDict *parser.Dictionary) *fontMetrics {
	if fontDict == nil {
		return nil
	}

	if subtype, ok := fontDict.Get("Subtype").(*parser.Name); ok && subtype.Value() == type0FontSubtype {
		return te.loadCompositeFontMetrics(fontDict)
	}
	return te.loadSimpleFontMetrics(fontDict)
}

func (te *TextExtractor) loadSimpleFontMetrics(fontDict *parser.Dictionary) *fontMetrics {
	firstCharObj, ok := fontDict.Get("FirstChar").(*parser.Integer)
	if !ok {
		return nil
	}
	widthsObj := te.resolveObject(fontDict.Get("Widths"))
	widthsArray, ok := widthsObj.(*parser.Array)
	if !ok || widthsArray.Len() == 0 {
		return nil
	}

	metrics := &fontMetrics{
		widths:       make(map[uint16]float64, widthsArray.Len()),
		defaultWidth: 600,
		precise:      true,
	}
	firstChar := int(firstCharObj.Value())
	for i := 0; i < widthsArray.Len(); i++ {
		if width := getNumber(widthsArray.Get(i)); width != nil && firstChar+i >= 0 && firstChar+i <= 65535 {
			metrics.widths[uint16(firstChar+i)] = *width
		}
	}

	if descriptor, ok := te.resolveObject(fontDict.Get("FontDescriptor")).(*parser.Dictionary); ok {
		if missingWidth := getNumber(descriptor.Get("MissingWidth")); missingWidth != nil {
			metrics.defaultWidth = *missingWidth
		}
	}
	return metrics
}

func (te *TextExtractor) loadCompositeFontMetrics(fontDict *parser.Dictionary) *fontMetrics {
	descendants, ok := te.resolveObject(fontDict.Get("DescendantFonts")).(*parser.Array)
	if !ok || descendants.Len() == 0 {
		return nil
	}
	descendant, ok := te.resolveObject(descendants.Get(0)).(*parser.Dictionary)
	if !ok {
		return nil
	}

	metrics := &fontMetrics{
		widths:       make(map[uint16]float64),
		defaultWidth: 1000,
		precise:      true,
	}
	if defaultWidth := getNumber(descendant.Get("DW")); defaultWidth != nil {
		metrics.defaultWidth = *defaultWidth
	}
	widths, _ := te.resolveObject(descendant.Get("W")).(*parser.Array)
	if widths == nil {
		return metrics
	}

	for i := 0; i < widths.Len(); {
		firstObj, ok := widths.Get(i).(*parser.Integer)
		if !ok {
			break
		}
		first := firstObj.Value()
		i++
		if i >= widths.Len() {
			break
		}

		if explicit, ok := te.resolveObject(widths.Get(i)).(*parser.Array); ok {
			for offset := 0; offset < explicit.Len(); offset++ {
				glyphID := first + int64(offset)
				if width := getNumber(explicit.Get(offset)); width != nil && glyphID >= 0 && glyphID <= 65535 {
					metrics.widths[uint16(glyphID)] = *width
				}
			}
			i++
			continue
		}

		lastObj, ok := widths.Get(i).(*parser.Integer)
		if !ok || i+1 >= widths.Len() {
			break
		}
		width := getNumber(widths.Get(i + 1))
		if width == nil {
			break
		}
		last := lastObj.Value()
		// CID values are unsigned 16-bit identifiers. Clamp both ends before
		// iterating so a hostile negative cFirst cannot turn a malformed /W
		// range into billions of no-op loop iterations.
		if first < 0 {
			first = 0
		}
		if last > 65535 {
			last = 65535
		}
		for glyphID := first; glyphID <= last; glyphID++ {
			metrics.widths[uint16(glyphID)] = *width
		}
		i += 2
	}
	return metrics
}

func (te *TextExtractor) resolveObject(object parser.PdfObject) parser.PdfObject {
	if ref, ok := object.(*parser.IndirectReference); ok {
		resolved, err := te.reader.GetObject(ref.Number)
		if err != nil {
			return nil
		}
		return resolved
	}
	return object
}
