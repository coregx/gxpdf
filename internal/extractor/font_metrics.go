package extractor

import (
	"math"
	"sort"

	"github.com/coregx/gxpdf/internal/parser"
)

const type0FontSubtype = "Type0"

// fontMetrics contains glyph advances in the PDF's standard 1000-unit text
// space. Simple fonts define /FirstChar and /Widths; composite Type0 fonts
// define /DW and /W on their descendant CIDFont.
type fontMetrics struct {
	widths       map[uint16]float64
	intervals    []fontWidthInterval
	defaultWidth float64
	precise      bool
}

// fontWidthInterval stores one composite-font /W range without expanding it
// into up to 65,536 map entries. loadCompositeFontMetrics normalizes these
// into ascending, non-overlapping intervals so width lookup stays logarithmic.
type fontWidthInterval struct {
	first uint16
	last  uint16
	width float64
}

func (fm *fontMetrics) width(glyphID uint16) float64 {
	if fm == nil {
		return 600
	}
	if width, ok := fm.widths[glyphID]; ok {
		return width
	}
	intervalIndex := sort.Search(len(fm.intervals), func(i int) bool {
		return fm.intervals[i].last >= glyphID
	})
	if intervalIndex < len(fm.intervals) && fm.intervals[intervalIndex].first <= glyphID {
		return fm.intervals[intervalIndex].width
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
	widthsObj := resolveObject(te.reader, fontDict.Get("Widths"))
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
		if width := getNumber(widthsArray.Get(i)); width != nil && isFiniteNumber(*width) &&
			firstChar+i >= 0 && firstChar+i <= 65535 {
			metrics.widths[uint16(firstChar+i)] = *width
		}
	}

	if descriptor, ok := resolveObject(te.reader, fontDict.Get("FontDescriptor")).(*parser.Dictionary); ok {
		if missingWidth := getNumber(descriptor.Get("MissingWidth")); missingWidth != nil {
			if isFiniteNumber(*missingWidth) {
				metrics.defaultWidth = *missingWidth
			}
		}
	}
	return metrics
}

func (te *TextExtractor) loadCompositeFontMetrics(fontDict *parser.Dictionary) *fontMetrics {
	descendants, ok := resolveObject(te.reader, fontDict.Get("DescendantFonts")).(*parser.Array)
	if !ok || descendants.Len() == 0 {
		return nil
	}
	descendant, ok := resolveObject(te.reader, descendants.Get(0)).(*parser.Dictionary)
	if !ok {
		return nil
	}

	metrics := &fontMetrics{
		defaultWidth: 1000,
		precise:      true,
	}
	if defaultWidth := getNumber(descendant.Get("DW")); defaultWidth != nil {
		if isFiniteNumber(*defaultWidth) {
			metrics.defaultWidth = *defaultWidth
		}
	}
	widths, _ := resolveObject(te.reader, descendant.Get("W")).(*parser.Array)
	if widths == nil {
		return metrics
	}

	intervals := make([]fontWidthInterval, 0, widths.Len()/2)
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

		if explicit, ok := resolveObject(te.reader, widths.Get(i)).(*parser.Array); ok {
			for offset := 0; offset < explicit.Len(); offset++ {
				glyphID := first + int64(offset)
				if width := getNumber(explicit.Get(offset)); width != nil &&
					glyphID >= 0 && glyphID <= 65535 && isFiniteNumber(*width) {
					cid := uint16(glyphID)
					intervals = append(intervals, fontWidthInterval{first: cid, last: cid, width: *width})
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
		if width == nil || !isFiniteNumber(*width) {
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
		if first <= last && last >= 0 && first <= 65535 {
			intervals = append(intervals, fontWidthInterval{
				first: uint16(first),
				last:  uint16(last),
				width: *width,
			})
		}
		i += 2
	}
	metrics.intervals = normalizeFontWidthIntervals(intervals)
	return metrics
}

func normalizeFontWidthIntervals(intervals []fontWidthInterval) []fontWidthInterval {
	if len(intervals) == 0 {
		return nil
	}
	sort.SliceStable(intervals, func(i, j int) bool {
		if intervals[i].first != intervals[j].first {
			return intervals[i].first < intervals[j].first
		}
		return intervals[i].last < intervals[j].last
	})

	normalized := intervals[:0]
	for _, current := range intervals {
		if len(normalized) > 0 {
			previous := &normalized[len(normalized)-1]
			if current.first <= previous.last {
				if current.last <= previous.last || previous.last == math.MaxUint16 {
					continue
				}
				current.first = previous.last + 1
			}
			if previous.last+1 == current.first && previous.width == current.width {
				previous.last = current.last
				continue
			}
		}
		normalized = append(normalized, current)
	}
	return normalized
}

func isFiniteNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
