package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
)

// Use keyword constants from token.go

// Parser parses PDF objects from a token stream.
// It builds higher-level objects (arrays, dictionaries, streams, indirect objects)
// from tokens produced by the Lexer.
//
// Reference: PDF 1.7 specification, Section 7.3 (Objects).
type Parser struct {
	lexer   *Lexer
	current Token
	peek    Token
	hasPeek bool
}

// NewParser creates a new parser that reads from the given reader.
func NewParser(r io.Reader) *Parser {
	lexer := NewLexer(r)
	p := &Parser{
		lexer: lexer,
	}
	// Prime the parser by reading the first token
	_ = p.advance()
	return p
}

// NewParserFromLexer creates a new parser from an existing lexer.
func NewParserFromLexer(lexer *Lexer) *Parser {
	p := &Parser{
		lexer: lexer,
	}
	// Prime the parser by reading the first token
	_ = p.advance()
	return p
}

// advance moves to the next token.
func (p *Parser) advance() error {
	if p.hasPeek {
		p.current = p.peek
		p.hasPeek = false
		return nil
	}

	tok, err := p.lexer.NextToken()
	if err != nil && tok.Type != TokenEOF {
		return err
	}
	p.current = tok
	return nil
}

// peekToken returns the next token without consuming it.
func (p *Parser) peekToken() (Token, error) {
	if p.hasPeek {
		return p.peek, nil
	}

	tok, err := p.lexer.NextToken()
	if err != nil && tok.Type != TokenEOF {
		return tok, err
	}

	p.peek = tok
	p.hasPeek = true
	return tok, nil
}

// expect checks if current token is of expected type and advances.
func (p *Parser) expect(expected TokenType) error {
	if p.current.Type != expected {
		return fmt.Errorf("expected %s, got %s at %d:%d",
			expected, p.current.Type, p.current.Line, p.current.Column)
	}
	return p.advance()
}

// match checks if current token matches the expected type.
func (p *Parser) match(expected TokenType) bool {
	return p.current.Type == expected
}

// ParseObject parses any PDF direct object.
// Returns the parsed object or an error.
//
//nolint:cyclop,funlen // Object parsing inherently requires checking many types.
func (p *Parser) ParseObject() (PdfObject, error) {
	switch p.current.Type {
	case TokenInteger:
		// Could be an integer, or start of indirect reference (N G R)
		// Save the current integer
		firstInt, err := strconv.ParseInt(p.current.Value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid integer %q at %d:%d: %w",
				p.current.Value, p.current.Line, p.current.Column, err)
		}

		// Advance past this integer
		_ = p.advance()

		// Peek ahead to check for indirect reference pattern
		if p.current.Type == TokenInteger {
			// Could be "N G R" pattern - need to check further
			secondInt, err2 := strconv.ParseInt(p.current.Value, 10, 64)
			if err2 != nil {
				return nil, fmt.Errorf("invalid integer %q at %d:%d: %w",
					p.current.Value, p.current.Line, p.current.Column, err2)
			}

			// Check if next token is "R"
			peek2, err3 := p.peekToken()
			if err3 == nil && peek2.Type == TokenKeyword && peek2.Value == "R" {
				// It's an indirect reference!
				_ = p.advance() // move to "R"
				_ = p.advance() // consume "R"
				return NewIndirectReference(int(firstInt), int(secondInt)), nil
			}
		}

		// Just a single integer (current token is already advanced)
		return NewInteger(firstInt), nil

	case TokenReal:
		value, err := strconv.ParseFloat(p.current.Value, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid real number %q at %d:%d: %w",
				p.current.Value, p.current.Line, p.current.Column, err)
		}
		_ = p.advance()
		return NewReal(value), nil

	case TokenString:
		value := p.current.Value
		_ = p.advance()
		return NewString(value), nil

	case TokenHexString:
		value := p.current.Value
		_ = p.advance()
		return NewHexString(value), nil

	case TokenName:
		value := p.current.Value
		_ = p.advance()
		return NewName(value), nil

	case TokenBoolean:
		value := p.current.Value == "true"
		_ = p.advance()
		return NewBoolean(value), nil

	case TokenNull:
		_ = p.advance()
		return NewNull(), nil

	case TokenArrayStart:
		return p.parseArray()

	case TokenDictStart:
		return p.parseDictionary()

	case TokenEOF:
		return nil, io.EOF

	default:
		return nil, fmt.Errorf("unexpected token %s at %d:%d",
			p.current.Type, p.current.Line, p.current.Column)
	}
}

// parseArray parses a PDF array: [ obj1 obj2 ... ].
func (p *Parser) parseArray() (*Array, error) {
	// Expect '['
	if err := p.expect(TokenArrayStart); err != nil {
		return nil, err
	}

	arr := NewArray()

	// Parse elements until ']'
	for !p.match(TokenArrayEnd) {
		if p.match(TokenEOF) {
			return nil, fmt.Errorf("unexpected EOF in array at %d:%d",
				p.current.Line, p.current.Column)
		}

		obj, err := p.ParseObject()
		if err != nil {
			return nil, fmt.Errorf("failed to parse array element: %w", err)
		}

		arr.Append(obj)
	}

	// Consume ']'
	if err := p.expect(TokenArrayEnd); err != nil {
		return nil, err
	}

	return arr, nil
}

// parseDictionary parses a PDF dictionary: << /Key1 value1 /Key2 value2 >>.
func (p *Parser) parseDictionary() (*Dictionary, error) {
	// Expect '<<'
	if err := p.expect(TokenDictStart); err != nil {
		return nil, err
	}

	dict := NewDictionary()

	// Parse key-value pairs until '>>'
	for !p.match(TokenDictEnd) {
		if p.match(TokenEOF) {
			return nil, fmt.Errorf("unexpected EOF in dictionary at %d:%d",
				p.current.Line, p.current.Column)
		}

		// Expect a name (key)
		if !p.match(TokenName) {
			return nil, fmt.Errorf("expected name for dictionary key, got %s at %d:%d",
				p.current.Type, p.current.Line, p.current.Column)
		}
		key := p.current.Value
		_ = p.advance()

		// Parse value
		value, err := p.ParseObject()
		if err != nil {
			return nil, fmt.Errorf("failed to parse dictionary value for key %q: %w", key, err)
		}

		dict.Set(key, value)
	}

	// Consume '>>'
	if err := p.expect(TokenDictEnd); err != nil {
		return nil, err
	}

	return dict, nil
}

// ParseIndirectObject parses an indirect object: N G obj ... endobj.
//
//nolint:cyclop // Indirect object parsing requires multiple validation steps.
func (p *Parser) ParseIndirectObject() (*IndirectObject, error) {
	// Read object number
	if !p.match(TokenInteger) {
		return nil, fmt.Errorf("expected object number, got %s at %d:%d",
			p.current.Type, p.current.Line, p.current.Column)
	}
	objNum, err := strconv.Atoi(p.current.Value)
	if err != nil {
		return nil, fmt.Errorf("invalid object number %q at %d:%d: %w",
			p.current.Value, p.current.Line, p.current.Column, err)
	}
	_ = p.advance()

	// Read generation number
	if !p.match(TokenInteger) {
		return nil, fmt.Errorf("expected generation number, got %s at %d:%d",
			p.current.Type, p.current.Line, p.current.Column)
	}
	genNum, err := strconv.Atoi(p.current.Value)
	if err != nil {
		return nil, fmt.Errorf("invalid generation number %q at %d:%d: %w",
			p.current.Value, p.current.Line, p.current.Column, err)
	}
	_ = p.advance()

	// Expect 'obj' keyword
	if !p.match(TokenKeyword) || p.current.Value != KeywordObj {
		return nil, fmt.Errorf("expected 'obj' keyword, got %s at %d:%d",
			p.current.Type, p.current.Line, p.current.Column)
	}
	_ = p.advance()

	// Parse the object (could be any PDF object)
	obj, err := p.ParseObject()
	if err != nil {
		return nil, fmt.Errorf("failed to parse indirect object content: %w", err)
	}

	// Check for stream (if object is a dictionary followed by 'stream')
	if p.match(TokenKeyword) && p.current.Value == KeywordStream {
		dict, ok := obj.(*Dictionary)
		if !ok {
			return nil, fmt.Errorf("stream must be preceded by dictionary, got %T at %d:%d",
				obj, p.current.Line, p.current.Column)
		}

		stream, err := p.parseStreamContent(dict)
		if err != nil {
			return nil, err
		}
		obj = stream
	}

	// Expect 'endobj' keyword
	if !p.match(TokenKeyword) || p.current.Value != "endobj" {
		return nil, fmt.Errorf("expected 'endobj' keyword, got %s(%q) at %d:%d",
			p.current.Type, p.current.Value, p.current.Line, p.current.Column)
	}
	_ = p.advance()

	return NewIndirectObject(objNum, genNum, obj), nil
}

// parseStreamContent parses stream content after a dictionary.
// Expects current token to be 'stream' keyword.
//
//nolint:cyclop // Stream content parsing requires multiple validation and reading steps.
func (p *Parser) parseStreamContent(dict *Dictionary) (*Stream, error) {
	// Consume 'stream' keyword
	if !p.match(TokenKeyword) || p.current.Value != KeywordStream {
		return nil, fmt.Errorf("expected 'stream' keyword, got %s at %d:%d",
			p.current.Type, p.current.Line, p.current.Column)
	}

	// Get stream length from dictionary
	length := dict.GetInteger("Length")
	if length <= 0 {
		// If length is not set or invalid, we need to scan for 'endstream'
		// This is a fallback for malformed PDFs
		return p.parseStreamUntilEndstream(dict)
	}

	// Read exactly 'length' bytes from the underlying reader
	content := make([]byte, length)

	// We need to read raw bytes from the lexer's reader
	// Skip the newline after 'stream' keyword first
	reader := p.getReaderFromLexer()

	// Skip whitespace/newline after stream
	b, err := reader.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read after stream keyword: %w", err)
	}
	// If it's CR, check for CRLF
	if b == '\r' {
		next, _ := reader.ReadByte()
		if next != '\n' {
			_ = reader.UnreadByte()
		}
	} else if b != '\n' {
		// No newline, put it back
		_ = reader.UnreadByte()
	}

	n, err := io.ReadFull(reader, content)
	if err != nil {
		return nil, fmt.Errorf("failed to read stream content: %w", err)
	}
	if n != int(length) {
		return nil, fmt.Errorf("expected %d bytes, got %d", length, n)
	}

	// Skip optional whitespace/newline before endstream
	p.lexer.skipWhitespace()

	// Expect 'endstream' keyword
	tok, err := p.lexer.NextToken()
	if err != nil {
		return nil, fmt.Errorf("expected 'endstream' after stream content: %w", err)
	}
	if tok.Type != TokenKeyword || tok.Value != KeywordEndstream {
		return nil, fmt.Errorf("expected 'endstream', got %s(%q) at %d:%d",
			tok.Type, tok.Value, tok.Line, tok.Column)
	}

	// Update current token
	p.current = tok
	_ = p.advance()

	return NewStream(dict, content), nil
}

// parseStreamUntilEndstream is a fallback parser for streams without proper Length.
func (p *Parser) parseStreamUntilEndstream(dict *Dictionary) (*Stream, error) {
	var content []byte
	reader := p.getReaderFromLexer()

	buf := make([]byte, 1)
	lookback := make([]byte, 0, 32)

	for {
		_, err := reader.Read(buf)
		if err != nil {
			return nil, fmt.Errorf("unexpected EOF while reading stream: %w", err)
		}

		lookback = append(lookback, buf[0])
		content = append(content, buf[0])

		// Keep lookback buffer reasonable size
		if len(lookback) > 32 {
			lookback = lookback[1:]
		}

		// Check for "endstream" in lookback
		if len(lookback) >= len(KeywordEndstream) {
			idx := -1
			for i := len(lookback) - len(KeywordEndstream); i >= 0; i-- {
				if string(lookback[i:i+len(KeywordEndstream)]) == KeywordEndstream {
					idx = i
					break
				}
			}

			if idx >= 0 {
				// Found endstream - trim it from content
				contentLen := len(content) - (len(lookback) - idx)
				content = content[:contentLen]
				break
			}
		}
	}

	// Update lexer state - skip whitespace and read the next token
	// Unlike the normal stream parsing path (which reads "endstream" then advances to "endobj"),
	// here we've already consumed "endstream" by scanning raw bytes, so NextToken reads "endobj" directly
	p.lexer.skipWhitespace()
	p.current, _ = p.lexer.NextToken()

	return NewStream(dict, content), nil
}

// getReaderFromLexer returns the underlying reader from the lexer.
// This is a helper to access raw bytes during stream parsing.
func (p *Parser) getReaderFromLexer() *bufio.Reader {
	return p.lexer.reader
}

// ParseObjectStream parses an Object Stream (PDF 1.5+) and returns the contained objects.
//
// Object Streams compress multiple objects together for efficiency. The format is:
//
//	N 0 obj
//	<< /Type /ObjStm /N numObjects /First firstByteOffset /Length ... >>
//	stream
//	obj1_num offset1 obj2_num offset2 ... objN_num offsetN
//	[object1_data] [object2_data] ... [objectN_data]
//	endstream
//	endobj
//
// The first part contains pairs of (object_number, offset_from_First).
// The second part (starting at /First) contains the actual object data.
//
// Reference: PDF 1.7 specification, Section 7.5.7 (Object Streams).
//
// Parameters:
//   - decodedData: The decoded stream content (after decompression)
//   - numObjects: The /N value (number of objects)
//   - firstOffset: The /First value (offset to first object data)
//
// Returns: Map of object number -> parsed object.
func (p *Parser) ParseObjectStream(decodedData []byte, numObjects, firstOffset int) (map[int]PdfObject, error) {
	if numObjects <= 0 {
		return nil, fmt.Errorf("invalid number of objects: %d", numObjects)
	}
	if firstOffset < 0 || firstOffset > len(decodedData) {
		return nil, fmt.Errorf("invalid first offset: %d (data length: %d)", firstOffset, len(decodedData))
	}

	// Parse the header section (object numbers and offsets)
	headerData := decodedData[:firstOffset]
	headerParser := NewParser(io.NopCloser(bytes.NewReader(headerData)))

	// Read object number/offset pairs
	type objInfo struct {
		number int
		offset int
	}
	objects := make([]objInfo, 0, numObjects)

	for i := 0; i < numObjects; i++ {
		// Read object number
		if !headerParser.match(TokenInteger) {
			return nil, fmt.Errorf("expected object number at index %d, got %s", i, headerParser.current.Type)
		}
		objNum, err := strconv.Atoi(headerParser.current.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid object number at index %d: %w", i, err)
		}
		_ = headerParser.advance()

		// Read offset (relative to /First)
		if !headerParser.match(TokenInteger) {
			return nil, fmt.Errorf("expected offset at index %d, got %s", i, headerParser.current.Type)
		}
		offset, err := strconv.Atoi(headerParser.current.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid offset at index %d: %w", i, err)
		}
		_ = headerParser.advance()

		objects = append(objects, objInfo{number: objNum, offset: offset})
	}

	// Parse the objects section
	objectData := decodedData[firstOffset:]
	result := make(map[int]PdfObject, numObjects)

	for i, info := range objects {
		// Calculate the end offset (start of next object, or end of data)
		endOffset := len(objectData)
		if i+1 < len(objects) {
			endOffset = objects[i+1].offset
		}

		if info.offset < 0 || info.offset > len(objectData) {
			return nil, fmt.Errorf("invalid offset %d for object %d", info.offset, info.number)
		}
		if endOffset > len(objectData) {
			endOffset = len(objectData)
		}

		// Extract this object's data
		objData := objectData[info.offset:endOffset]

		// Parse the object
		objParser := NewParser(io.NopCloser(bytes.NewReader(objData)))
		obj, err := objParser.ParseObject()
		if err != nil {
			return nil, fmt.Errorf("failed to parse object %d in stream: %w", info.number, err)
		}

		result[info.number] = obj
	}

	return result, nil
}

// Position returns the current parser position (line, column).
func (p *Parser) Position() (line, column int) {
	return p.current.Line, p.current.Column
}

// Reset resets the parser with a new reader.
func (p *Parser) Reset(r io.Reader) {
	p.lexer.Reset(r)
	p.hasPeek = false
	_ = p.advance()
}
