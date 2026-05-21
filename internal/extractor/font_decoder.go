package extractor

import (
	"encoding/binary"
	"strings"
	"unicode/utf16"
)

// FontDecoder decodes glyph byte sequences to Unicode strings using CMap tables.
//
// PDF fonts can use various encodings for text strings:
//   - Built-in encodings: WinAnsiEncoding, MacRomanEncoding, etc.
//   - Custom encodings: ToUnicode CMap (most common for non-Latin scripts)
//   - Identity encodings: Direct mapping (no decoding needed)
//
// The FontDecoder handles all these cases and converts raw glyph bytes
// to readable Unicode text.
//
// Reference: PDF 1.7 specification, Section 9.6.6 (Character Encoding).
type FontDecoder struct {
	// cmap is the ToUnicode CMap table (if available)
	cmap *CMapTable

	// encoding is the base encoding name (e.g., "WinAnsiEncoding")
	encoding string

	// use2ByteGlyphs indicates if glyphs are 2 bytes (true) or 1 byte (false)
	// CIDFonts typically use 2-byte glyphs
	use2ByteGlyphs bool

	// customEncoding is a custom glyph ID → Unicode mapping from /Encoding/Differences
	// This is used when a font defines custom glyph mappings via the Differences array.
	customEncoding map[uint16]rune

	// isCompositeFont is true when the source font is a Type0 (composite) font.
	// For composite fonts the 2-byte character codes are authoritative — the
	// garbage heuristic fallback to 1-byte must never be applied.
	isCompositeFont bool
}

// NewFontDecoder creates a new FontDecoder with the given CMap and encoding.
//
// Parameters:
//   - cmap: ToUnicode CMap table (can be nil if not available)
//   - encoding: Base encoding name (e.g., "WinAnsiEncoding", "Identity-H")
//   - use2ByteGlyphs: true for 2-byte glyphs (CIDFonts), false for 1-byte glyphs
func NewFontDecoder(cmap *CMapTable, encoding string, use2ByteGlyphs bool) *FontDecoder {
	return &FontDecoder{
		cmap:           cmap,
		encoding:       encoding,
		use2ByteGlyphs: use2ByteGlyphs,
		customEncoding: nil,
	}
}

// NewFontDecoderWithCMap creates a FontDecoder that uses only CMap decoding.
//
// This is a convenience constructor for the most common case: custom fonts
// with ToUnicode CMap (e.g., embedded fonts with Cyrillic text).
func NewFontDecoderWithCMap(cmap *CMapTable) *FontDecoder {
	// Auto-detect glyph size from CMap mappings
	// If any glyph ID > 255, we need 2-byte glyphs
	use2Byte := false
	if cmap != nil {
		for glyphID := range cmap.mappings {
			if glyphID > 255 {
				use2Byte = true
				break
			}
		}
	}

	return NewFontDecoder(cmap, "", use2Byte)
}

// DecodeString decodes a glyph byte sequence to a Unicode string.
//
// The decoding process:
//  1. Split bytes into glyphs (1-byte or 2-byte depending on font)
//  2. For each glyph ID, look up Unicode in CMap table
//  3. If CMap lookup fails, try built-in encoding (WinAnsi, MacRoman)
//  4. If all else fails, treat as ISO-8859-1 (Latin-1) for ASCII compatibility
//
// Returns the decoded Unicode string. Invalid glyphs are replaced with
// Unicode replacement character (U+FFFD).
func (d *FontDecoder) DecodeString(glyphBytes []byte) string {
	if len(glyphBytes) == 0 {
		return ""
	}

	// Check if this looks like UTF-16BE (starts with BOM or has null bytes)
	// BUT: Skip UTF-16 detection for Identity-H/Identity-V encodings!
	// These encodings use 2-byte glyphs which look like UTF-16 but must be decoded via CMap
	isIdentityEncoding := strings.Contains(d.encoding, "Identity")
	if !isIdentityEncoding && d.looksLikeUTF16(glyphBytes) {
		return d.decodeUTF16BE(glyphBytes)
	}

	// Try decoding with configured glyph size (1 or 2 bytes)
	decodedStr := d.decodeWithGlyphSize(glyphBytes, d.use2ByteGlyphs)

	// FALLBACK: If result contains too many non-printable chars (garbage),
	// try alternate glyph size (1-byte if was 2-byte, vice versa).
	//
	// Exception: composite (Type0) fonts MUST NOT fall back to 1-byte decoding.
	// Their 2-byte character codes are authoritative; applying a 1-byte fallback
	// would silently corrupt every CID glyph ID above 0x00FF.
	if d.use2ByteGlyphs && !d.isCompositeFont && d.looksLikeGarbage(decodedStr) {
		// Try 1-byte glyphs instead
		decodedStr1Byte := d.decodeWithGlyphSize(glyphBytes, false)
		if !d.looksLikeGarbage(decodedStr1Byte) {
			decodedStr = decodedStr1Byte
		}
	}

	return decodedStr
}

// decodeWithGlyphSize decodes bytes using specified glyph size (1 or 2 bytes per glyph)
func (d *FontDecoder) decodeWithGlyphSize(glyphBytes []byte, use2Byte bool) string {
	var result strings.Builder
	result.Grow(len(glyphBytes)) // Pre-allocate

	pos := 0
	for pos < len(glyphBytes) {
		var glyphID uint16
		var bytesRead int

		if use2Byte && pos+1 < len(glyphBytes) {
			// Read 2-byte glyph ID (big-endian)
			glyphID = uint16(glyphBytes[pos])<<8 | uint16(glyphBytes[pos+1])
			bytesRead = 2
		} else if pos < len(glyphBytes) {
			// Read 1-byte glyph ID
			glyphID = uint16(glyphBytes[pos])
			bytesRead = 1
		} else {
			break
		}

		pos += bytesRead

		// Decode glyph to Unicode
		unicode := d.decodeGlyph(glyphID)
		result.WriteRune(unicode)
	}

	return result.String()
}

// looksLikeGarbage returns true if string contains too many non-printable characters
func (d *FontDecoder) looksLikeGarbage(s string) bool {
	if len(s) == 0 {
		return false
	}

	nonPrintable := 0
	for _, r := range s {
		// Count control chars and replacement chars as garbage
		if r < 32 && r != '\n' && r != '\t' || r == 0xFFFD {
			nonPrintable++
		}
	}

	// If more than 30% non-printable, it's probably garbage
	ratio := float64(nonPrintable) / float64(len([]rune(s)))
	return ratio > 0.3
}

// readGlyphID reads a glyph ID from the byte stream.
//
// Returns the glyph ID (uint16) and number of bytes read.
func (d *FontDecoder) readGlyphID(data []byte) (uint16, int) {
	if len(data) == 0 {
		return 0, 0
	}

	if d.use2ByteGlyphs {
		// 2-byte glyphs (big-endian)
		if len(data) < 2 {
			// Not enough bytes - treat as 1-byte glyph
			return uint16(data[0]), 1
		}
		glyphID := binary.BigEndian.Uint16(data[0:2])
		return glyphID, 2
	}

	// 1-byte glyphs
	return uint16(data[0]), 1
}

// decodeGlyph decodes a single glyph ID to Unicode.
//
// Decoding priority:
//  1. CMap table (if available)
//  2. Custom encoding (Differences array)
//  3. Built-in encoding (WinAnsi, MacRoman, etc.)
//  4. Fallback to Latin-1 (ISO-8859-1)
func (d *FontDecoder) decodeGlyph(glyphID uint16) rune {
	// Try CMap first (highest priority)
	if d.cmap != nil {
		if unicode, ok := d.cmap.GetUnicode(glyphID); ok {
			return unicode
		}
	}

	// Try custom encoding (Differences array)
	if d.customEncoding != nil {
		if unicode, ok := d.customEncoding[glyphID]; ok {
			return unicode
		}
	}

	// Try built-in encoding
	if d.encoding != "" {
		if unicode, ok := d.decodeBuiltInEncoding(glyphID); ok {
			return unicode
		}
	}

	// Fallback to Latin-1 (ISO-8859-1) for single-byte glyphs
	// This works for ASCII and basic Latin characters
	if glyphID <= 255 {
		return rune(glyphID)
	}

	// Unknown glyph - use replacement character
	return '\uFFFD' // Unicode replacement character
}

// decodeBuiltInEncoding decodes a glyph using built-in PDF encodings.
//
// Supported encodings:
//   - WinAnsiEncoding (Windows Code Page 1252)
//   - MacRomanEncoding
//   - StandardEncoding (Adobe standard)
//
// Returns the Unicode rune and true if successful, or 0 and false if not found.
func (d *FontDecoder) decodeBuiltInEncoding(glyphID uint16) (rune, bool) {
	// Only works for single-byte glyphs
	if glyphID > 255 {
		return 0, false
	}

	// For Phase 1, we implement WinAnsiEncoding (most common)
	// Other encodings can be added later if needed
	if strings.Contains(d.encoding, "WinAnsi") {
		return decodeWinAnsi(byte(glyphID)), true
	}

	return 0, false
}

// looksLikeUTF16 checks if the byte sequence looks like UTF-16BE.
//
// Heuristics:
//   - Starts with UTF-16BE BOM (0xFE 0xFF)
//   - Has even length and many null bytes (typical for ASCII in UTF-16)
func (d *FontDecoder) looksLikeUTF16(data []byte) bool {
	if len(data) < 2 {
		return false
	}

	// Check for UTF-16BE BOM
	if data[0] == 0xFE && data[1] == 0xFF {
		return true
	}

	// Check for UTF-16 pattern: many null bytes in even positions
	// This catches cases where BOM is missing but content is UTF-16
	if len(data)%2 == 0 && len(data) >= 4 {
		nullCount := 0
		for i := 0; i < len(data) && i < 20; i += 2 {
			if data[i] == 0 {
				nullCount++
			}
		}
		// If >40% of even positions are null, it's likely UTF-16BE
		if float64(nullCount)/float64(len(data)/2) > 0.4 {
			return true
		}
	}

	return false
}

// decodeUTF16BE decodes a UTF-16BE byte sequence.
//
// Handles BOM if present, and decodes surrogate pairs correctly.
func (d *FontDecoder) decodeUTF16BE(data []byte) string {
	// Skip BOM if present
	if len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF {
		data = data[2:]
	}

	// Convert to uint16 slice
	if len(data)%2 != 0 {
		// Odd length - truncate last byte
		data = data[:len(data)-1]
	}

	u16slice := make([]uint16, len(data)/2)
	for i := 0; i < len(u16slice); i++ {
		u16slice[i] = binary.BigEndian.Uint16(data[i*2 : i*2+2])
	}

	// Decode UTF-16 to runes
	runes := utf16.Decode(u16slice)
	return string(runes)
}

// decodeWinAnsi decodes a byte using WinAnsiEncoding (Windows-1252).
//
// WinAnsiEncoding is identical to ISO-8859-1 (Latin-1) except for
// bytes 0x80-0x9F, which map to specific characters in Windows-1252.
//
// Reference: PDF 1.7 specification, Appendix D.2 (Latin Character Set and Encodings).
func decodeWinAnsi(b byte) rune {
	// For bytes 0x00-0x7F and 0xA0-0xFF, WinAnsi is same as ISO-8859-1
	if b < 0x80 || b >= 0xA0 {
		return rune(b)
	}

	// Windows-1252 specific mappings for 0x80-0x9F
	winAnsiTable := [32]rune{
		0x20AC, // 0x80: Euro sign
		0xFFFD, // 0x81: Undefined
		0x201A, // 0x82: Single low-9 quotation mark
		0x0192, // 0x83: Latin small letter f with hook
		0x201E, // 0x84: Double low-9 quotation mark
		0x2026, // 0x85: Horizontal ellipsis
		0x2020, // 0x86: Dagger
		0x2021, // 0x87: Double dagger
		0x02C6, // 0x88: Modifier letter circumflex accent
		0x2030, // 0x89: Per mille sign
		0x0160, // 0x8A: Latin capital letter S with caron
		0x2039, // 0x8B: Single left-pointing angle quotation mark
		0x0152, // 0x8C: Latin capital ligature OE
		0xFFFD, // 0x8D: Undefined
		0x017D, // 0x8E: Latin capital letter Z with caron
		0xFFFD, // 0x8F: Undefined
		0xFFFD, // 0x90: Undefined
		0x2018, // 0x91: Left single quotation mark
		0x2019, // 0x92: Right single quotation mark
		0x201C, // 0x93: Left double quotation mark
		0x201D, // 0x94: Right double quotation mark
		0x2022, // 0x95: Bullet
		0x2013, // 0x96: En dash
		0x2014, // 0x97: Em dash
		0x02DC, // 0x98: Small tilde
		0x2122, // 0x99: Trade mark sign
		0x0161, // 0x9A: Latin small letter s with caron
		0x203A, // 0x9B: Single right-pointing angle quotation mark
		0x0153, // 0x9C: Latin small ligature oe
		0xFFFD, // 0x9D: Undefined
		0x017E, // 0x9E: Latin small letter z with caron
		0x0178, // 0x9F: Latin capital letter Y with diaeresis
	}

	return winAnsiTable[b-0x80]
}

// HasCMap returns true if this decoder has a CMap table.
func (d *FontDecoder) HasCMap() bool {
	return d.cmap != nil
}

// Encoding returns the base encoding name.
func (d *FontDecoder) Encoding() string {
	return d.encoding
}

// String returns a string representation of the decoder's configuration.
func (d *FontDecoder) String() string {
	var parts []string

	if d.cmap != nil {
		parts = append(parts, "CMap:"+d.cmap.Name())
	}
	if d.encoding != "" {
		parts = append(parts, "Encoding:"+d.encoding)
	}
	if d.use2ByteGlyphs {
		parts = append(parts, "2-byte-glyphs")
	} else {
		parts = append(parts, "1-byte-glyphs")
	}

	return "FontDecoder{" + strings.Join(parts, ", ") + "}"
}
