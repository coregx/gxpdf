package fonts

// substituteCode is drawn, and measured, in place of a character the font's
// encoding cannot represent.
const substituteCode = '?'

// winAnsiHigh maps the characters of WinAnsiEncoding codes 0x80-0x9F, where it
// differs from Latin-1. Codes 0x81, 0x8D, 0x8F, 0x90 and 0x9D are unassigned.
//
// Reference: ISO 32000-1, Annex D.2 (Latin Character Set and Encodings).
//
//nolint:gochecknoglobals // Encoding table is intentionally global
var winAnsiHigh = map[rune]byte{
	0x20AC: 0x80, // Euro
	0x201A: 0x82, // quotesinglbase
	0x0192: 0x83, // florin
	0x201E: 0x84, // quotedblbase
	0x2026: 0x85, // ellipsis
	0x2020: 0x86, // dagger
	0x2021: 0x87, // daggerdbl
	0x02C6: 0x88, // circumflex
	0x2030: 0x89, // perthousand
	0x0160: 0x8A, // Scaron
	0x2039: 0x8B, // guilsinglleft
	0x0152: 0x8C, // OE
	0x017D: 0x8E, // Zcaron
	0x2018: 0x91, // quoteleft
	0x2019: 0x92, // quoteright
	0x201C: 0x93, // quotedblleft
	0x201D: 0x94, // quotedblright
	0x2022: 0x95, // bullet
	0x2013: 0x96, // endash
	0x2014: 0x97, // emdash
	0x02DC: 0x98, // tilde
	0x2122: 0x99, // trademark
	0x0161: 0x9A, // scaron
	0x203A: 0x9B, // guilsinglright
	0x0153: 0x9C, // oe
	0x017E: 0x9E, // zcaron
	0x0178: 0x9F, // Ydieresis
}

// WinAnsiCode returns the WinAnsiEncoding code of r. It reports false for
// characters the encoding cannot represent, such as Cyrillic or CJK.
//
// ASCII, including control characters, and Latin-1 U+00A0-U+00FF map to the
// same code; the C1 controls U+0080-U+009F are not representable because
// WinAnsiEncoding assigns those codes to other characters.
func WinAnsiCode(r rune) (byte, bool) {
	switch {
	case r < 0x80:
		return byte(r), true
	case r >= 0xA0 && r <= 0xFF:
		return byte(r), true
	}
	b, ok := winAnsiHigh[r]
	return b, ok
}

// code returns the character code that draws r in this font, and the
// character whose width that code has: r itself, or substituteCode when the
// font's encoding cannot represent r.
//
// Text fonts use WinAnsiEncoding. Symbol and ZapfDingbats use their built-in
// encodings, and their text is given as the codes themselves (U+0000-U+00FF),
// which is also how their widths are indexed.
func (m *FontMetrics) code(r rune) (byte, rune) {
	if m.symbolic {
		if r <= 0xFF {
			return byte(r), r
		}
		return substituteCode, substituteCode
	}
	b, ok := WinAnsiCode(r)
	if !ok {
		return substituteCode, substituteCode
	}
	// WinAnsiEncoding draws the space and hyphen glyphs at these codes too
	// (ISO 32000-1, Annex D.2, notes 5 and 6).
	switch b {
	case 0xA0:
		return b, ' '
	case 0xAD:
		return b, '-'
	}
	return b, r
}

// Encode returns text as the character codes of this font, one byte per
// character. Characters the font's encoding cannot represent become '?'.
func (m *FontMetrics) Encode(text string) []byte {
	out := make([]byte, 0, len(text))
	for _, r := range text {
		b, _ := m.code(r)
		out = append(out, b)
	}
	return out
}

// EncodeWinAnsi returns text in WinAnsiEncoding, one byte per character.
// Characters the encoding cannot represent become '?'.
func EncodeWinAnsi(text string) []byte {
	return helveticaMetrics.Encode(text)
}

// EncodeText returns text as the character codes of the Standard 14 font
// fontName, as written in a content stream: WinAnsiEncoding for the text
// fonts, the built-in encoding for Symbol and ZapfDingbats. Characters the
// encoding cannot represent become '?'. An unrecognized fontName is encoded
// as WinAnsiEncoding.
//
// Text outside WinAnsiEncoding (Cyrillic, Greek, CJK, ...) needs an embedded
// TrueType font.
func EncodeText(fontName, text string) []byte {
	m := GetMetrics(fontName)
	if m == nil {
		m = helveticaMetrics
	}
	return m.Encode(text)
}
