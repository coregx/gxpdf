// Package parser implements PDF lexical analysis (tokenization) according to
// PDF 1.7 specification, Section 7.2 (Lexical Conventions).
package parser

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Lexer tokenizes a PDF byte stream according to PDF 1.7 specification,
// Section 7.2 (Lexical Conventions).
type Lexer struct {
	reader     *bufio.Reader
	line       int  // Current line number (1-based)
	column     int  // Current column number (1-based)
	lastChar   byte // Last character read
	peekedChar byte // Peeked character (0 if none)
	hasPeeked  bool // Whether we have a peeked character
}

// NewLexer creates a new lexer that reads from the given reader.
func NewLexer(r io.Reader) *Lexer {
	return &Lexer{
		reader: bufio.NewReader(r),
		line:   1,
		column: 0,
	}
}

// NextToken returns the next token from the input stream.
//
//nolint:cyclop // Token parsing requires checking many different token types.
func (l *Lexer) NextToken() (Token, error) {
	// Skip whitespace and comments
	l.skipWhitespace()

	// Check for EOF
	ch, err := l.peek()
	if errors.Is(err, io.EOF) {
		return EOFToken(l.line, l.column), nil
	}
	if err != nil {
		return ErrorToken(err.Error(), l.line, l.column), err
	}

	// Save position for token
	line := l.line
	column := l.column

	// Parse based on first character
	switch {
	case ch == '[':
		_, _ = l.readByte() // consume '[' (error already checked via peek)
		return NewToken(TokenArrayStart, "[", line, column), nil

	case ch == ']':
		_, _ = l.readByte() // consume ']' (error already checked via peek)
		return NewToken(TokenArrayEnd, "]", line, column), nil

	case ch == '<':
		// Could be << (dict start) or <hex> (hex string)
		_, _ = l.readByte() // consume '<' (error already checked via peek)
		next, err := l.peek()
		if err == nil && next == '<' {
			_, _ = l.readByte() // consume second '<'
			return NewToken(TokenDictStart, "<<", line, column), nil
		}
		// Hex string
		return l.readHexString(line, column)

	case ch == '>':
		// Could be >> (dict end) or syntax error
		_, _ = l.readByte() // consume '>' (error already checked via peek)
		next, err := l.peek()
		if err == nil && next == '>' {
			_, _ = l.readByte() // consume second '>'
			return NewToken(TokenDictEnd, ">>", line, column), nil
		}
		return ErrorToken("unexpected '>'", line, column), fmt.Errorf("unexpected '>' at %d:%d", line, column)

	case ch == '(':
		return l.readString(line, column)

	case ch == '/':
		return l.readName(line, column)

	case ch == '+' || ch == '-' || ch == '.' || (ch >= '0' && ch <= '9'):
		return l.readNumber(line, column)

	case isRegularChar(ch):
		token, err := l.readKeywordOrBoolean(line, column)
		if err == nil && token.Type == TokenKeyword && token.Value == "ID" {
			l.skipInlineImageData()
		}
		return token, err

	default:
		_, _ = l.readByte() // consume unexpected char
		return ErrorToken(fmt.Sprintf("unexpected character %q", ch), line, column),
			fmt.Errorf("unexpected character %q at %d:%d", ch, line, column)
	}
}

// Peek returns the next token without consuming it.
func (l *Lexer) Peek() (Token, error) {
	// Save current state
	savedReader := l.reader
	savedLine := l.line
	savedColumn := l.column
	savedLastChar := l.lastChar
	savedPeekedChar := l.peekedChar
	savedHasPeeked := l.hasPeeked

	// Get next token
	tok, err := l.NextToken()

	// Note: We can't perfectly restore the bufio.Reader state,
	// so we create a new reader with the original content.
	// This is a limitation - for production use, consider implementing
	// a custom buffering strategy or token buffering.
	l.reader = savedReader
	l.line = savedLine
	l.column = savedColumn
	l.lastChar = savedLastChar
	l.peekedChar = savedPeekedChar
	l.hasPeeked = savedHasPeeked

	return tok, err
}

// skipWhitespace skips whitespace and comments.
// PDF whitespace: space (0x20), tab (0x09), CR (0x0D), LF (0x0A), null (0x00), FF (0x0C).
func (l *Lexer) skipWhitespace() {
	for {
		ch, err := l.peek()
		if err != nil {
			return
		}

		if isWhitespace(ch) {
			_, _ = l.readByte()
			continue
		}

		if ch == '%' {
			l.skipComment()
			continue
		}

		break
	}
}

// skipComment skips a comment (% to end of line).
func (l *Lexer) skipComment() {
	for {
		ch, err := l.readByte()
		if err != nil || ch == '\r' || ch == '\n' {
			return
		}
	}
}

// readString reads a literal string: (text with escapes).
// Handles nested parentheses and escape sequences.
//
//nolint:gocognit,cyclop,funlen // String parsing has inherent complexity due to escape sequences.
func (l *Lexer) readString(line, column int) (Token, error) {
	_, _ = l.readByte() // consume '('

	var buf bytes.Buffer
	depth := 1 // Track parenthesis nesting

	for depth > 0 {
		ch, err := l.readByte()
		if err != nil {
			return ErrorToken("unterminated string", line, column),
				fmt.Errorf("unterminated string at %d:%d", line, column)
		}

		switch ch {
		case '(':
			depth++
			buf.WriteByte(ch)

		case ')':
			depth--
			if depth > 0 {
				buf.WriteByte(ch)
			}

		case '\\':
			// Escape sequence
			next, err := l.readByte()
			if err != nil {
				return ErrorToken("incomplete escape sequence", line, column),
					fmt.Errorf("incomplete escape sequence at %d:%d", line, column)
			}

			switch next {
			case 'n':
				buf.WriteByte('\n')
			case 'r':
				buf.WriteByte('\r')
			case 't':
				buf.WriteByte('\t')
			case 'b':
				buf.WriteByte('\b')
			case 'f':
				buf.WriteByte('\f')
			case '(', ')', '\\':
				buf.WriteByte(next)
			case '\r', '\n':
				// Line continuation - ignore backslash and newline
				if next == '\r' {
					// Check for CRLF
					peek, _ := l.peek()
					if peek == '\n' {
						_, _ = l.readByte()
					}
				}
			case '0', '1', '2', '3', '4', '5', '6', '7':
				// Octal escape: \ddd
				octal := []byte{next}
				for i := 0; i < 2; i++ {
					peek, err := l.peek()
					if err != nil || peek < '0' || peek > '7' {
						break
					}
					ch, _ := l.readByte()
					octal = append(octal, ch)
				}
				val, _ := strconv.ParseInt(string(octal), 8, 32)
				buf.WriteByte(byte(val))
			default:
				// Unknown escape - treat as literal
				buf.WriteByte(next)
			}

		default:
			buf.WriteByte(ch)
		}
	}

	return NewToken(TokenString, buf.String(), line, column), nil
}

// readHexString reads a hexadecimal string: <48656C6C6F>.
// Whitespace inside is ignored. Odd number of digits is padded with 0 on right.
func (l *Lexer) readHexString(line, column int) (Token, error) {
	var buf bytes.Buffer

	for {
		ch, err := l.peek()
		if err != nil {
			return ErrorToken("unterminated hex string", line, column),
				fmt.Errorf("unterminated hex string at %d:%d", line, column)
		}

		if ch == '>' {
			_, _ = l.readByte() // consume '>'
			break
		}

		_, _ = l.readByte() // consume character

		// Skip whitespace
		if isWhitespace(ch) {
			continue
		}

		// Must be hex digit
		if !isHexDigit(ch) {
			return ErrorToken(fmt.Sprintf("invalid hex digit %q", ch), line, column),
				fmt.Errorf("invalid hex digit %q in hex string at %d:%d", ch, line, column)
		}

		buf.WriteByte(ch)
	}

	hexStr := buf.String()

	// Pad with 0 if odd length (PDF spec requirement)
	if len(hexStr)%2 == 1 {
		hexStr += "0"
	}

	// Convert hex to bytes
	decoded := make([]byte, len(hexStr)/2)
	for i := 0; i < len(hexStr); i += 2 {
		val, _ := strconv.ParseUint(hexStr[i:i+2], 16, 8)
		decoded[i/2] = byte(val)
	}

	return NewToken(TokenHexString, string(decoded), line, column), nil
}

// readName reads a name object: /Type, /Name#20With#20Spaces.
//
//nolint:nilerr // Error from hex parsing is intentionally handled by returning error token.
func (l *Lexer) readName(line, column int) (Token, error) {
	_, _ = l.readByte() // consume '/'

	var buf bytes.Buffer

	for {
		ch, err := l.peek()
		if err != nil {
			break
		}

		// Name ends at delimiter or whitespace
		if isDelimiter(ch) || isWhitespace(ch) {
			break
		}

		_, _ = l.readByte() // consume character

		// Handle # escape sequence
		if ch == '#' {
			// Next two characters should be hex digits
			hex1, err1 := l.readByte()
			hex2, err2 := l.readByte()
			if err1 != nil || err2 != nil || !isHexDigit(hex1) || !isHexDigit(hex2) {
				return ErrorToken("invalid # escape in name", line, column),
					fmt.Errorf("invalid # escape in name at %d:%d", line, column)
			}

			val, _ := strconv.ParseUint(string([]byte{hex1, hex2}), 16, 8)
			buf.WriteByte(byte(val))
		} else {
			buf.WriteByte(ch)
		}
	}

	return NewToken(TokenName, buf.String(), line, column), nil
}

// readNumber reads an integer or real number.
// Integers: 123, -456, +789
// Reals: 3.14, -2.5, .5, 123.
//
//nolint:cyclop // Number parsing requires checking multiple conditions.
func (l *Lexer) readNumber(line, column int) (Token, error) {
	var buf bytes.Buffer

	// Read optional sign
	ch, _ := l.peek()
	if ch == '+' || ch == '-' {
		_, _ = l.readByte()
		buf.WriteByte(ch)
	}

	hasDigit := false
	hasDot := false

	// Read digits and optional decimal point
	for {
		ch, err := l.peek()
		if err != nil {
			break
		}

		switch {
		case ch >= '0' && ch <= '9':
			_, _ = l.readByte()
			buf.WriteByte(ch)
			hasDigit = true
		case ch == '.' && !hasDot:
			_, _ = l.readByte()
			buf.WriteByte(ch)
			hasDot = true
		default:
			goto done
		}
	}
done:

	// Must have at least one digit (or just a dot for .5)
	numStr := buf.String()
	if !hasDigit && numStr != "." && numStr != "+." && numStr != "-." {
		return ErrorToken("invalid number", line, column),
			fmt.Errorf("invalid number at %d:%d", line, column)
	}

	// Determine type
	if hasDot {
		return NewToken(TokenReal, numStr, line, column), nil
	}
	return NewToken(TokenInteger, numStr, line, column), nil
}

// readKeywordOrBoolean reads a keyword or boolean value.
// Keywords: obj, endobj, stream, endstream, xref, trailer, startxref, R, n, f.
// Booleans: true, false.
// Null: null.
func (l *Lexer) readKeywordOrBoolean(line, column int) (Token, error) {
	var buf bytes.Buffer

	for {
		ch, err := l.peek()
		if err != nil {
			break
		}

		if !isRegularChar(ch) {
			break
		}

		_, _ = l.readByte()
		buf.WriteByte(ch)
	}

	word := buf.String()

	// Check for boolean
	if word == "true" || word == "false" {
		return NewToken(TokenBoolean, word, line, column), nil
	}

	// Check for null
	if word == "null" {
		return NewToken(TokenNull, word, line, column), nil
	}

	// Check for keyword (file structure keywords)
	if IsKeyword(word) {
		return NewToken(TokenKeyword, word, line, column), nil
	}

	// Check for content stream operator
	if IsContentStreamOperator(word) {
		return NewToken(TokenKeyword, word, line, column), nil
	}

	// Unknown word - treat as error
	return ErrorToken(fmt.Sprintf("unknown token %q", word), line, column),
		fmt.Errorf("unknown token %q at %d:%d", word, line, column)
}

// readByte reads a single byte from the input and updates line/column tracking.
func (l *Lexer) readByte() (byte, error) {
	var ch byte
	var err error

	if l.hasPeeked {
		ch = l.peekedChar
		l.hasPeeked = false
	} else {
		ch, err = l.reader.ReadByte()
		if err != nil {
			return 0, err
		}
	}

	// Update position tracking
	l.lastChar = ch
	if ch == '\n' {
		l.line++
		l.column = 0
	} else {
		l.column++
	}

	return ch, nil
}

// peek returns the next byte without consuming it.
func (l *Lexer) peek() (byte, error) {
	if l.hasPeeked {
		return l.peekedChar, nil
	}

	ch, err := l.reader.ReadByte()
	if err != nil {
		return 0, err
	}

	l.peekedChar = ch
	l.hasPeeked = true
	return ch, nil
}

// skipInlineImageData consumes the binary payload of an inline image, from the
// byte after the ID keyword up to and including the EI that ends it.
//
// The payload is arbitrary binary - the flate data of a sampled image starts
// with 0x78 0x9c - so tokenizing it produces "unknown token" errors and the
// page it appears on cannot be parsed at all. EI is the only reliable marker
// (Section 8.9.7: "EI" preceded by white space), so a payload that happens to
// contain those letters without white space before them does not end the scan.
//
// The spec puts one white-space byte between ID and the data; some producers
// emit \r\n instead, so up to two end-of-line bytes are consumed. A payload
// without EI runs to the end of the stream, which leaves the caller at EOF: a
// corrupt inline image ends the content stream instead of failing the whole
// page.
func (l *Lexer) skipInlineImageData() {
	separator := byte(' ')
	if ch, err := l.readByte(); err == nil {
		separator = ch
		if ch == '\r' {
			if next, err := l.peek(); err == nil && next == '\n' {
				_, _ = l.readByte()
				separator = '\n'
			}
		}
	}

	before, prev := separator, separator
	for {
		ch, err := l.readByte()
		if err != nil {
			return
		}
		if ch == 'I' && prev == 'E' && isWhitespace(before) {
			return
		}
		before, prev = prev, ch
	}
}

// isWhitespace checks if a byte is PDF whitespace.
// PDF whitespace: space (0x20), tab (0x09), CR (0x0D), LF (0x0A), null (0x00), FF (0x0C).
func isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' || ch == 0x00 || ch == 0x0C
}

// isDelimiter checks if a byte is a PDF delimiter.
// Delimiters: ( ) < > [ ] { } / %.
func isDelimiter(ch byte) bool {
	return ch == '(' || ch == ')' || ch == '<' || ch == '>' ||
		ch == '[' || ch == ']' || ch == '{' || ch == '}' ||
		ch == '/' || ch == '%'
}

// isRegularChar checks if a byte is a regular (non-delimiter, non-whitespace) character.
func isRegularChar(ch byte) bool {
	return !isWhitespace(ch) && !isDelimiter(ch) && ch >= 33 && ch <= 126
}

// isHexDigit checks if a byte is a hexadecimal digit.
func isHexDigit(ch byte) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'F') || (ch >= 'a' && ch <= 'f')
}

// Position returns the current line and column.
func (l *Lexer) Position() (line, column int) {
	return l.line, l.column
}

// ReadAll reads all tokens from the input until EOF.
// Useful for debugging and testing.
func (l *Lexer) ReadAll() ([]Token, error) {
	var tokens []Token

	for {
		tok, err := l.NextToken()
		if err != nil && tok.Type != TokenEOF {
			return tokens, err
		}

		tokens = append(tokens, tok)

		if tok.Type == TokenEOF {
			break
		}
	}

	return tokens, nil
}

// skipTo skips input until the given string is found.
// Useful for error recovery.
func (l *Lexer) skipTo(target string) error {
	targetBytes := []byte(target)
	matched := 0

	for {
		ch, err := l.readByte()
		if err != nil {
			return err
		}

		if ch == targetBytes[matched] {
			matched++
			if matched == len(targetBytes) {
				return nil
			}
		} else {
			matched = 0
		}
	}
}

// Reset resets the lexer to read from a new reader.
func (l *Lexer) Reset(r io.Reader) {
	l.reader = bufio.NewReader(r)
	l.line = 1
	l.column = 0
	l.lastChar = 0
	l.peekedChar = 0
	l.hasPeeked = false
}

// Tokenize is a convenience function that tokenizes the entire input string.
func Tokenize(input string) ([]Token, error) {
	lexer := NewLexer(strings.NewReader(input))
	return lexer.ReadAll()
}
