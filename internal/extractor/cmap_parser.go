package extractor

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
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
}

// NewCMapTable creates a new empty CMapTable.
func NewCMapTable(name string) *CMapTable {
	return &CMapTable{
		mappings: make(map[uint16]rune),
		name:     name,
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

		// Parse hex strings
		glyphID, err := parseHexString(srcCode)
		if err != nil {
			// Skip invalid mappings
			continue
		}

		unicode, err := parseHexString(dstCode)
		if err != nil {
			// Skip invalid mappings
			continue
		}

		table.AddMapping(uint16(glyphID), rune(unicode))
	}

	return nil
}

// parseBfRange parses beginbfrange...endbfrange section.
//
// Format:
//
//	2 beginbfrange
//	<srcCodeLow1> <srcCodeHigh1> <dstCodeLow1>
//	<srcCodeLow2> <srcCodeHigh2> <dstCodeLow2>
//	...
//	endbfrange
//
// Maps a range of source codes to consecutive destination codes.
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
		dstLow := p.nextToken()

		if srcHigh == "" || dstLow == "" {
			return fmt.Errorf("invalid bfrange mapping: incomplete range")
		}

		if !strings.HasPrefix(srcHigh, "<") || !strings.HasPrefix(dstLow, "<") {
			continue
		}

		// Parse hex strings
		startGlyphID, err := parseHexString(srcLow)
		if err != nil {
			continue
		}

		endGlyphID, err := parseHexString(srcHigh)
		if err != nil {
			continue
		}

		startUnicode, err := parseHexString(dstLow)
		if err != nil {
			continue
		}

		// Check for array format: <srcLow> <srcHigh> [<dst1> <dst2> ...]
		// For now, we only support scalar destination (most common case)
		if strings.HasPrefix(dstLow, "[") {
			// Array format - skip for now (Phase 1)
			// This would map each source code to a specific destination code
			continue
		}

		table.AddRangeMapping(uint16(startGlyphID), uint16(endGlyphID), rune(startUnicode))
	}

	return nil
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
