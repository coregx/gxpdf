package extractor

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
)

// CMapTable represents a Character Map that maps glyph IDs to Unicode code points.
//
// CMap (Character Map) defines the mapping between character codes (glyph IDs)
// used in a PDF font and the corresponding Unicode values. This is essential for
// extracting readable text from PDFs, especially for custom encodings and non-Latin
// scripts like Cyrillic, Chinese, Japanese, etc.
//
// The mapping is stored as glyph ID (uint16) → Unicode rune (int32).
//
// Reference: PDF 1.7 specification, Section 9.7.5 (ToUnicode CMaps).
type CMapTable struct {
	// mappings stores glyph ID to Unicode code point mappings
	// Key: glyph ID (character code from PDF)
	// Value: Unicode code point
	mappings map[uint16]rune

	// name is the CMap name (e.g., "Adobe-Identity-UCS")
	name string

	// CodeBytes is the number of bytes per character code as declared in
	// begincodespacerange. 1 for single-byte fonts, 2 for CIDFonts with
	// Identity-H/V encoding. Defaults to 1 when not parsed.
	CodeBytes int
}

// NewCMapTable creates a new empty CMapTable.
func NewCMapTable(name string) *CMapTable {
	return &CMapTable{
		mappings:  make(map[uint16]rune),
		name:      name,
		CodeBytes: 1, // default; overridden by begincodespacerange
	}
}

// AddMapping adds a single glyph ID to Unicode mapping.
func (t *CMapTable) AddMapping(glyphID uint16, unicode rune) {
	t.mappings[glyphID] = unicode
}

// AddRangeMapping adds a range of glyph IDs to consecutive Unicode values.
//
// For example: AddRangeMapping(0x10, 0x20, 0x0430) maps:
//   - Glyph 0x10 → U+0430 ('а')
//   - Glyph 0x11 → U+0431 ('б')
//   - ...
//   - Glyph 0x20 → U+0440 ('р')
func (t *CMapTable) AddRangeMapping(startGlyphID, endGlyphID uint16, startUnicode rune) {
	// Use uint32 to avoid wraparound when endGlyphID is 0xFFFF
	// (uint16 wraps from 65535 to 0, causing infinite loop)
	for glyphID := uint32(startGlyphID); glyphID <= uint32(endGlyphID); glyphID++ {
		offset := glyphID - uint32(startGlyphID)
		t.mappings[uint16(glyphID)] = startUnicode + rune(offset)
	}
}

// GetUnicode returns the Unicode code point for a given glyph ID.
//
// Returns the Unicode rune and true if mapping exists, or 0 and false if not found.
func (t *CMapTable) GetUnicode(glyphID uint16) (rune, bool) {
	unicode, ok := t.mappings[glyphID]
	return unicode, ok
}

// Size returns the number of mappings in the table.
func (t *CMapTable) Size() int {
	return len(t.mappings)
}

// Name returns the CMap name.
func (t *CMapTable) Name() string {
	return t.name
}

// CMapParser parses CMap (Character Map) streams from PDF ToUnicode entries.
//
// CMap Format (simplified):
//
//	/CIDInit /ProcSet findresource begin
//	12 dict begin
//	begincmap
//	/CMapName /Adobe-Identity-UCS def
//	/CMapType 2 def
//
//	% Single character mappings
//	10 beginbfchar
//	<0001> <0412>  % Glyph 0x01 → U+0412 'В'
//	<0002> <044B>  % Glyph 0x02 → U+044B 'ы'
//	<0003> <043F>  % Glyph 0x03 → U+043F 'п'
//	endbfchar
//
//	% Range mappings
//	2 beginbfrange
//	<0010> <0020> <0430>  % Glyphs 0x10-0x20 → U+0430-0x0440
//	endbfrange
//
//	endcmap
//
// Reference: PDF 1.7 specification, Section 9.7.5 (ToUnicode CMaps).
type CMapParser struct {
	data   []byte
	pos    int
	length int
}

// NewCMapParser creates a new CMapParser for the given stream data.
//
// The stream should be the decoded content of a ToUnicode CMap stream.
func NewCMapParser(data []byte) *CMapParser {
	return &CMapParser{
		data:   data,
		pos:    0,
		length: len(data),
	}
}

// Parse parses the CMap stream and returns a CMapTable.
//
// The parser handles:
//   - beginbfchar/endbfchar: Single character mappings
//   - beginbfrange/endbfrange: Range mappings
//
// Unsupported operators are silently ignored for graceful degradation.
func (p *CMapParser) Parse() (*CMapTable, error) {
	// Create table with default name (will be updated if found)
	table := NewCMapTable("Unknown")

	// Parse tokens until end of stream
	for p.pos < p.length {
		token := p.nextToken()
		if token == "" {
			break
		}

		switch token {
		case "/CMapName":
			// Get CMap name: /CMapName /Adobe-Identity-UCS def
			name := p.nextToken()
			if strings.HasPrefix(name, "/") {
				table.name = strings.TrimPrefix(name, "/")
			}

		case "begincodespacerange":
			// Parse code space range to determine bytes-per-character-code.
			// Format:
			//   N begincodespacerange
			//   <low> <high>
			//   ...
			//   endcodespacerange
			p.parseCodeSpaceRange(table)

		case "beginbfchar":
			// Parse single character mappings
			if err := p.parseBfChar(table); err != nil {
				return nil, fmt.Errorf("failed to parse beginbfchar: %w", err)
			}

		case "beginbfrange":
			// Parse range mappings
			if err := p.parseBfRange(table); err != nil {
				return nil, fmt.Errorf("failed to parse beginbfrange: %w", err)
			}

		case "endcmap":
			// End of CMap - we're done
			break
		}
	}

	return table, nil
}

// parseBfChar parses beginbfchar...endbfchar section.
//
// Format:
//
//	10 beginbfchar
//	<srcCode1> <dstCode1>
//	<srcCode2> <dstCode2>
//	...
//	endbfchar
//
// The destination code may be a UTF-16BE surrogate pair (8 hex chars = 4 bytes).
func (p *CMapParser) parseBfChar(table *CMapTable) error {
	for {
		token := p.nextToken()
		if token == "" || token == "endbfchar" {
			break
		}

		// Should be a hex string: <0001>
		if !strings.HasPrefix(token, "<") {
			continue
		}

		srcCode := token
		dstCode := p.nextToken()

		if dstCode == "" || !strings.HasPrefix(dstCode, "<") {
			return fmt.Errorf("invalid bfchar mapping: missing destination code")
		}

		// Parse source glyph ID
		glyphID, err := parseHexString(srcCode)
		if err != nil {
			// Skip invalid mappings
			continue
		}

		// Parse destination as UTF-16BE (handles surrogate pairs)
		r, err := decodeUTF16BEHex(dstCode)
		if err != nil {
			// Skip invalid mappings
			continue
		}

		table.AddMapping(uint16(glyphID), r)
	}

	return nil
}

// parseBfRange parses beginbfrange...endbfrange section.
//
// Supports two forms:
//
//  1. Scalar form (consecutive Unicode block):
//     <srcLow> <srcHigh> <dstLow>
//     Maps srcLow+i → dstLow+i for each i in [0, srcHigh-srcLow].
//
//  2. Array form (explicit per-code mapping):
//     <srcLow> <srcHigh> [<dst0> <dst1> ... <dstN>]
//     Maps srcLow+i → dst_i exactly.
//
// Reference: PDF 1.7 specification, Section 9.7.5 (ToUnicode CMaps).
func (p *CMapParser) parseBfRange(table *CMapTable) error {
	for {
		token := p.nextToken()
		if token == "" || token == "endbfrange" {
			break
		}

		// Should be a hex string: <0001>
		if !strings.HasPrefix(token, "<") {
			continue
		}

		srcLow := token
		srcHigh := p.nextToken()
		dstToken := p.nextToken()

		if srcHigh == "" || dstToken == "" {
			return fmt.Errorf("invalid bfrange mapping: incomplete range")
		}

		if !strings.HasPrefix(srcHigh, "<") {
			continue
		}

		// Parse source range boundaries
		startGlyphID, err := parseHexString(srcLow)
		if err != nil {
			continue
		}

		endGlyphID, err := parseHexString(srcHigh)
		if err != nil {
			continue
		}

		// Array form: [<dst0> <dst1> ... <dstN>]
		// nextToken() returns the full "[...]" as a single token.
		if strings.HasPrefix(dstToken, "[") {
			p.parseBfRangeArray(table, uint16(startGlyphID), uint16(endGlyphID), dstToken)
			continue
		}

		// Scalar form: <dstLow> — consecutive Unicode block
		if !strings.HasPrefix(dstToken, "<") {
			continue
		}

		startUnicode, err := decodeUTF16BEHex(dstToken)
		if err != nil {
			continue
		}

		table.AddRangeMapping(uint16(startGlyphID), uint16(endGlyphID), startUnicode)
	}

	return nil
}

// parseBfRangeArray handles the array form of bfrange:
//
//	<srcLow> <srcHigh> [<dst0> <dst1> ... <dstN>]
//
// Each element in the array is a hex string that maps to the corresponding
// source code: srcLow+i → dst_i. The array token has already been consumed
// by nextToken() as the full "[...]" string.
func (p *CMapParser) parseBfRangeArray(table *CMapTable, srcLow, srcHigh uint16, arrayToken string) {
	// Strip the enclosing brackets
	inner := strings.TrimPrefix(arrayToken, "[")
	inner = strings.TrimSuffix(inner, "]")
	inner = strings.TrimSpace(inner)

	if inner == "" {
		return
	}

	// Parse individual hex tokens from the bracket content.
	// Each token is of the form <XXXX>.
	hexTokens := extractHexTokensFromArray(inner)

	srcCode := srcLow
	for _, hexToken := range hexTokens {
		if srcCode > srcHigh {
			break
		}

		r, err := decodeUTF16BEHex(hexToken)
		if err != nil {
			srcCode++
			continue
		}

		table.AddMapping(srcCode, r)
		srcCode++
	}
}

// extractHexTokensFromArray parses the interior of a [...] bracket, extracting
// individual <hex> tokens. Non-hex content is silently skipped.
func extractHexTokensFromArray(s string) []string {
	var tokens []string
	for i := 0; i < len(s); {
		// Skip whitespace
		if s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r' {
			i++
			continue
		}
		if s[i] == '<' {
			// Find the closing >
			end := strings.IndexByte(s[i:], '>')
			if end < 0 {
				break
			}
			tokens = append(tokens, s[i:i+end+1])
			i += end + 1
			continue
		}
		i++
	}
	return tokens
}

// parseCodeSpaceRange parses the begincodespacerange...endcodespacerange section.
//
// Format:
//
//	N begincodespacerange
//	<low> <high>
//	...
//	endcodespacerange
//
// The hex string length of <low> or <high> reveals the bytes-per-code:
// 2 hex chars = 1 byte, 4 hex chars = 2 bytes.
// We use the first entry to set table.CodeBytes.
func (p *CMapParser) parseCodeSpaceRange(table *CMapTable) {
	first := true
	for {
		token := p.nextToken()
		if token == "" || token == "endcodespacerange" {
			break
		}

		if !strings.HasPrefix(token, "<") {
			continue
		}

		// Consume the matching high end token
		high := p.nextToken()
		if high == "" {
			break
		}

		if first {
			// Hex string body length: strip < and >
			body := strings.TrimPrefix(token, "<")
			body = strings.TrimSuffix(body, ">")
			// 4 hex chars = 2 bytes (CIDFont), 2 hex chars = 1 byte (simple font)
			if len(body) >= 4 {
				table.CodeBytes = 2
			} else {
				table.CodeBytes = 1
			}
			first = false
		}
	}
}

// decodeUTF16BEHex decodes a hex string (e.g., "<0041>" or "<D83DDE00>") to a rune.
//
// The hex string may encode:
//   - A single BMP code point (2 bytes / 4 hex chars): decode as big-endian uint16
//   - A UTF-16BE surrogate pair (4 bytes / 8 hex chars): decode via utf16.DecodeRune
//
// Returns an error for empty or malformed input.
func decodeUTF16BEHex(hexStr string) (rune, error) {
	body := strings.TrimPrefix(hexStr, "<")
	body = strings.TrimSuffix(body, ">")

	if body == "" {
		return 0, fmt.Errorf("empty hex string")
	}

	// Pad odd-length hex strings to even (shouldn't normally happen)
	if len(body)%2 != 0 {
		body = "0" + body
	}

	// Parse raw bytes
	rawBytes, err := hexStringToBytes(body)
	if err != nil {
		return 0, err
	}

	switch len(rawBytes) {
	case 1:
		return rune(rawBytes[0]), nil
	case 2:
		// Single BMP code point
		u16 := uint16(rawBytes[0])<<8 | uint16(rawBytes[1])
		return rune(u16), nil
	case 4:
		// Possible UTF-16BE surrogate pair
		high := uint16(rawBytes[0])<<8 | uint16(rawBytes[1])
		low := uint16(rawBytes[2])<<8 | uint16(rawBytes[3])
		if high >= 0xD800 && high <= 0xDBFF {
			// Confirmed surrogate pair
			return utf16.DecodeRune(rune(high), rune(low)), nil
		}
		// Not a surrogate — treat as 32-bit code point (rare)
		cp := rune(rawBytes[0])<<24 | rune(rawBytes[1])<<16 | rune(rawBytes[2])<<8 | rune(rawBytes[3])
		return cp, nil
	default:
		// For > 4 bytes, fall back to integer parsing
		val, err2 := strconv.ParseInt(body, 16, 64)
		if err2 != nil {
			return 0, fmt.Errorf("invalid hex string %q: %w", hexStr, err2)
		}
		return rune(val), nil
	}
}

// hexStringToBytes converts a hex string (no angle brackets) to a byte slice.
func hexStringToBytes(hex string) ([]byte, error) {
	if len(hex)%2 != 0 {
		return nil, fmt.Errorf("odd-length hex string: %q", hex)
	}
	result := make([]byte, len(hex)/2)
	for i := range result {
		nibbles := hex[i*2 : i*2+2]
		val, err := strconv.ParseUint(nibbles, 16, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid hex byte %q in %q: %w", nibbles, hex, err)
		}
		result[i] = byte(val)
	}
	return result, nil
}

// nextToken reads the next token from the stream.
//
// Tokens are separated by whitespace. Hex strings like <0001> are returned as-is.
func (p *CMapParser) nextToken() string {
	// Skip whitespace
	for p.pos < p.length && isWhitespace(p.data[p.pos]) {
		p.pos++
	}

	if p.pos >= p.length {
		return ""
	}

	start := p.pos

	// Check for dictionary: << ... >> or hex string: <...>
	if p.data[p.pos] == '<' {
		// Check if it's a dictionary <<
		if p.pos+1 < p.length && p.data[p.pos+1] == '<' {
			// Dictionary << ... >>
			p.pos += 2 // Move past '<<'
			depth := 1
			for p.pos < p.length && depth > 0 {
				if p.pos+1 < p.length && p.data[p.pos] == '<' && p.data[p.pos+1] == '<' {
					depth++
					p.pos += 2
				} else if p.pos+1 < p.length && p.data[p.pos] == '>' && p.data[p.pos+1] == '>' {
					depth--
					p.pos += 2
				} else {
					p.pos++
				}
			}
			return string(p.data[start:p.pos])
		}

		// Hex string <...>
		p.pos++ // Move past '<'
		for p.pos < p.length && p.data[p.pos] != '>' {
			p.pos++
		}
		if p.pos < p.length && p.data[p.pos] == '>' {
			p.pos++ // Include closing '>'
		}
		return string(p.data[start:p.pos])
	}

	// Check for array: [...]
	if p.data[p.pos] == '[' {
		p.pos++ // Move past '['
		depth := 1
		for p.pos < p.length && depth > 0 {
			if p.data[p.pos] == '[' {
				depth++
			} else if p.data[p.pos] == ']' {
				depth--
			}
			p.pos++
		}
		return string(p.data[start:p.pos])
	}

	// Check for string: (...)
	if p.data[p.pos] == '(' {
		p.pos++ // Move past '('
		depth := 1
		for p.pos < p.length && depth > 0 {
			if p.data[p.pos] == '\\' {
				// Skip escaped character
				p.pos += 2
				continue
			}
			if p.data[p.pos] == '(' {
				depth++
			} else if p.data[p.pos] == ')' {
				depth--
			}
			p.pos++
		}
		return string(p.data[start:p.pos])
	}

	// Regular token (name, operator, number)
	// Names can start with '/'
	if p.data[p.pos] == '/' {
		p.pos++ // Include '/' in token
	}

	for p.pos < p.length && !isWhitespace(p.data[p.pos]) && p.data[p.pos] != '<' && p.data[p.pos] != '>' && p.data[p.pos] != '[' && p.data[p.pos] != ']' {
		p.pos++
	}

	return string(p.data[start:p.pos])
}

// parseHexString parses a hex string like <0001> or <0412> to an integer.
//
// Returns the numeric value of the hex string.
func parseHexString(hexStr string) (int, error) {
	// Remove < and >
	hexStr = strings.TrimPrefix(hexStr, "<")
	hexStr = strings.TrimSuffix(hexStr, ">")

	if hexStr == "" {
		return 0, fmt.Errorf("empty hex string")
	}

	// Parse hex value
	value, err := strconv.ParseInt(hexStr, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid hex string %q: %w", hexStr, err)
	}

	return int(value), nil
}

// isWhitespace returns true if the byte is a whitespace character.
//
// PDF whitespace: space (0x20), tab (0x09), line feed (0x0A), carriage return (0x0D), null (0x00), form feed (0x0C).
func isWhitespace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '\x00', '\f':
		return true
	default:
		return false
	}
}

// isDelimiter returns true if the byte is a PDF delimiter.
//
// PDF delimiters: ( ) < > [ ] { } / %
func isDelimiter(b byte) bool {
	switch b {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	default:
		return false
	}
}

// ParseCMapStream is a convenience function that parses a CMap stream.
//
// This is equivalent to:
//
//	parser := NewCMapParser(data)
//	return parser.Parse()
func ParseCMapStream(data []byte) (*CMapTable, error) {
	// Check if stream looks like a CMap (contains "begincmap" or "beginbfchar")
	if !bytes.Contains(data, []byte("begincmap")) && !bytes.Contains(data, []byte("beginbfchar")) {
		// Not a CMap stream - return empty table
		return NewCMapTable("None"), nil
	}

	parser := NewCMapParser(data)
	return parser.Parse()
}
