package parser

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTokenType_String tests the String method of TokenType.
func TestTokenType_String(t *testing.T) {
	tests := []struct {
		name     string
		tokType  TokenType
		expected string
	}{
		{"Error", TokenError, "ERROR"},
		{"EOF", TokenEOF, "EOF"},
		{"Integer", TokenInteger, "INTEGER"},
		{"Real", TokenReal, "REAL"},
		{"String", TokenString, "STRING"},
		{"HexString", TokenHexString, "HEX_STRING"},
		{"Name", TokenName, "NAME"},
		{"Boolean", TokenBoolean, "BOOLEAN"},
		{"Null", TokenNull, "NULL"},
		{"Keyword", TokenKeyword, "KEYWORD"},
		{"ArrayStart", TokenArrayStart, "ARRAY_START"},
		{"ArrayEnd", TokenArrayEnd, "ARRAY_END"},
		{"DictStart", TokenDictStart, "DICT_START"},
		{"DictEnd", TokenDictEnd, "DICT_END"},
		{"Unknown", TokenType(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.tokType.String())
		})
	}
}

// TestToken_String tests the String method of Token.
func TestToken_String(t *testing.T) {
	tok := NewToken(TokenInteger, "123", 1, 5)
	expected := `INTEGER("123") at 1:5`
	assert.Equal(t, expected, tok.String())
}

// TestIsKeyword tests the IsKeyword function.
func TestIsKeyword(t *testing.T) {
	tests := []struct {
		name     string
		word     string
		expected bool
	}{
		{"obj", "obj", true},
		{"endobj", "endobj", true},
		{"stream", "stream", true},
		{"endstream", "endstream", true},
		{"xref", "xref", true},
		{"trailer", "trailer", true},
		{"startxref", "startxref", true},
		{"R", "R", true},
		{"n", "n", true},
		{"f", "f", true},
		{"not keyword", "notakeyword", false},
		{"true", "true", false},
		{"false", "false", false},
		{"null", "null", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsKeyword(tt.word))
		})
	}
}

// TestLexer_Integers tests tokenization of integers.
func TestLexer_Integers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"positive", "123", "123"},
		{"negative", "-456", "-456"},
		{"explicit positive", "+789", "+789"},
		{"zero", "0", "0"},
		{"large", "2147483647", "2147483647"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(strings.NewReader(tt.input))
			tok, err := lexer.NextToken()
			require.NoError(t, err)
			assert.Equal(t, TokenInteger, tok.Type)
			assert.Equal(t, tt.expected, tok.Value)
		})
	}
}

// TestLexer_Reals tests tokenization of real numbers.
func TestLexer_Reals(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "3.14", "3.14"},
		{"negative", "-2.5", "-2.5"},
		{"positive", "+0.001", "+0.001"},
		{"leading dot", ".5", ".5"},
		{"trailing dot", "123.", "123."},
		{"zero", "0.0", "0.0"},
		{"negative leading dot", "-.5", "-.5"},
		{"positive leading dot", "+.5", "+.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(strings.NewReader(tt.input))
			tok, err := lexer.NextToken()
			require.NoError(t, err)
			assert.Equal(t, TokenReal, tok.Type)
			assert.Equal(t, tt.expected, tok.Value)
		})
	}
}

// TestLexer_Strings tests tokenization of literal strings.
func TestLexer_Strings(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "(Hello)", "Hello"},
		{"with spaces", "(Hello World)", "Hello World"},
		{"empty", "()", ""},
		{"newline escape", `(Line1\nLine2)`, "Line1\nLine2"},
		{"return escape", `(Line1\rLine2)`, "Line1\rLine2"},
		{"tab escape", `(Col1\tCol2)`, "Col1\tCol2"},
		{"backspace escape", `(Text\b)`, "Text\b"},
		{"formfeed escape", `(Text\f)`, "Text\f"},
		{"backslash escape", `(C:\\Path)`, `C:\Path`},
		{"paren escapes", `(\(nested\))`, "(nested)"},
		{"nested parens", "(outer (inner) text)", "outer (inner) text"},
		{"octal escape", `(\101\102\103)`, "ABC"},
		{"single digit octal", `(\1)`, "\001"},
		{"two digit octal", `(\12)`, "\012"},
		{"mixed", `(Hello\nWorld\t\101)`, "Hello\nWorld\tA"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(strings.NewReader(tt.input))
			tok, err := lexer.NextToken()
			require.NoError(t, err)
			assert.Equal(t, TokenString, tok.Type)
			assert.Equal(t, tt.expected, tok.Value)
		})
	}
}

// TestLexer_Strings_LineContinuation tests line continuation in strings.
func TestLexer_Strings_LineContinuation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"LF continuation", "(Line1\\\nLine2)", "Line1Line2"},
		{"CR continuation", "(Line1\\\rLine2)", "Line1Line2"},
		{"CRLF continuation", "(Line1\\\r\nLine2)", "Line1Line2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(strings.NewReader(tt.input))
			tok, err := lexer.NextToken()
			require.NoError(t, err)
			assert.Equal(t, TokenString, tok.Type)
			assert.Equal(t, tt.expected, tok.Value)
		})
	}
}

// TestLexer_Strings_Errors tests error cases in string parsing.
func TestLexer_Strings_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unterminated", "(Hello"},
		{"unterminated nested", "(outer (inner)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(strings.NewReader(tt.input))
			tok, err := lexer.NextToken()
			assert.Error(t, err)
			assert.Equal(t, TokenError, tok.Type)
		})
	}
}

// TestLexer_HexStrings tests tokenization of hexadecimal strings.
func TestLexer_HexStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "<48656C6C6F>", "Hello"},
		{"uppercase", "<48656C6C6F>", "Hello"},
		{"lowercase", "<48656c6c6f>", "Hello"},
		{"with whitespace", "<48 65 6C 6C 6F>", "Hello"},
		{"with tabs", "<48\t65\t6C\t6C\t6F>", "Hello"},
		{"with newlines", "<48\n65\n6C\n6C\n6F>", "Hello"},
		{"empty", "<>", ""},
		{"odd digits (padded)", "<123>", "\x12\x30"},
		{"single digit", "<4>", "\x40"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(strings.NewReader(tt.input))
			tok, err := lexer.NextToken()
			require.NoError(t, err)
			assert.Equal(t, TokenHexString, tok.Type)
			assert.Equal(t, tt.expected, tok.Value)
		})
	}
}

// TestLexer_HexStrings_Errors tests error cases in hex string parsing.
func TestLexer_HexStrings_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unterminated", "<48656C"},
		{"invalid char", "<48G5>"},
		{"invalid char space", "<Hello>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(strings.NewReader(tt.input))
			tok, err := lexer.NextToken()
			assert.Error(t, err)
			assert.Equal(t, TokenError, tok.Type)
		})
	}
}

// TestLexer_Names tests tokenization of name objects.
func TestLexer_Names(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "/Type", "Type"},
		{"lowercase", "/page", "page"},
		{"with numbers", "/Page1", "Page1"},
		{"with special chars", "/Name.With-Special_Chars", "Name.With-Special_Chars"},
		{"hash escape space", "/Name#20With#20Spaces", "Name With Spaces"},
		{"hash escape special", "/A#42B", "ABB"}, // #42 = 'B'
		{"multiple escapes", "/A#20B#20C", "A B C"},
		{"empty", "/", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(strings.NewReader(tt.input))
			tok, err := lexer.NextToken()
			require.NoError(t, err)
			assert.Equal(t, TokenName, tok.Type)
			assert.Equal(t, tt.expected, tok.Value)
		})
	}
}

// TestLexer_Names_Errors tests error cases in name parsing.
func TestLexer_Names_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"incomplete escape", "/Name#2"},
		{"invalid escape", "/Name#GG"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(strings.NewReader(tt.input))
			tok, err := lexer.NextToken()
			assert.Error(t, err)
			assert.Equal(t, TokenError, tok.Type)
		})
	}
}

// TestLexer_Booleans tests tokenization of boolean values.
func TestLexer_Booleans(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"true", "true", "true"},
		{"false", "false", "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(strings.NewReader(tt.input))
			tok, err := lexer.NextToken()
			require.NoError(t, err)
			assert.Equal(t, TokenBoolean, tok.Type)
			assert.Equal(t, tt.expected, tok.Value)
		})
	}
}

// TestLexer_Null tests tokenization of null value.
func TestLexer_Null(t *testing.T) {
	lexer := NewLexer(strings.NewReader("null"))
	tok, err := lexer.NextToken()
	require.NoError(t, err)
	assert.Equal(t, TokenNull, tok.Type)
	assert.Equal(t, "null", tok.Value)
}

// TestLexer_Keywords tests tokenization of keywords.
func TestLexer_Keywords(t *testing.T) {
	keywords := []string{"obj", "endobj", "stream", "endstream", "xref", "trailer", "startxref", "R"}

	for _, kw := range keywords {
		t.Run(kw, func(t *testing.T) {
			lexer := NewLexer(strings.NewReader(kw))
			tok, err := lexer.NextToken()
			require.NoError(t, err)
			assert.Equal(t, TokenKeyword, tok.Type)
			assert.Equal(t, kw, tok.Value)
		})
	}
}

// TestLexer_Delimiters tests tokenization of delimiters.
func TestLexer_Delimiters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		tokType  TokenType
		expected string
	}{
		{"array start", "[", TokenArrayStart, "["},
		{"array end", "]", TokenArrayEnd, "]"},
		{"dict start", "<<", TokenDictStart, "<<"},
		{"dict end", ">>", TokenDictEnd, ">>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(strings.NewReader(tt.input))
			tok, err := lexer.NextToken()
			require.NoError(t, err)
			assert.Equal(t, tt.tokType, tok.Type)
			assert.Equal(t, tt.expected, tok.Value)
		})
	}
}

// TestLexer_Whitespace tests whitespace handling.
func TestLexer_Whitespace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []TokenType
	}{
		{"space", "123 456", []TokenType{TokenInteger, TokenInteger, TokenEOF}},
		{"tab", "123\t456", []TokenType{TokenInteger, TokenInteger, TokenEOF}},
		{"newline", "123\n456", []TokenType{TokenInteger, TokenInteger, TokenEOF}},
		{"CR", "123\r456", []TokenType{TokenInteger, TokenInteger, TokenEOF}},
		{"CRLF", "123\r\n456", []TokenType{TokenInteger, TokenInteger, TokenEOF}},
		{"null", "123\x00456", []TokenType{TokenInteger, TokenInteger, TokenEOF}},
		{"formfeed", "123\f456", []TokenType{TokenInteger, TokenInteger, TokenEOF}},
		{"mixed", "123 \t\r\n 456", []TokenType{TokenInteger, TokenInteger, TokenEOF}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(strings.NewReader(tt.input))
			for _, expectedType := range tt.expected {
				tok, err := lexer.NextToken()
				require.NoError(t, err)
				assert.Equal(t, expectedType, tok.Type)
			}
		})
	}
}

// TestLexer_Comments tests comment handling.
func TestLexer_Comments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []TokenType
	}{
		{"single line", "123 % comment\n456", []TokenType{TokenInteger, TokenInteger, TokenEOF}},
		{"at start", "% comment\n123", []TokenType{TokenInteger, TokenEOF}},
		{"at end", "123 % comment", []TokenType{TokenInteger, TokenEOF}},
		{"multiple", "123 % comment1\n456 % comment2\n789", []TokenType{TokenInteger, TokenInteger, TokenInteger, TokenEOF}},
		{"empty comment", "123 %\n456", []TokenType{TokenInteger, TokenInteger, TokenEOF}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(strings.NewReader(tt.input))
			for _, expectedType := range tt.expected {
				tok, err := lexer.NextToken()
				require.NoError(t, err)
				assert.Equal(t, expectedType, tok.Type)
			}
		})
	}
}

// TestLexer_EOF tests EOF handling.
func TestLexer_EOF(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"whitespace only", "   \t\n"},
		{"comment only", "% just a comment"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(strings.NewReader(tt.input))
			tok, err := lexer.NextToken()
			require.NoError(t, err)
			assert.Equal(t, TokenEOF, tok.Type)
		})
	}
}

// TestLexer_Complex tests complex PDF content.
//
//nolint:funlen // Test cases require comprehensive validation.
func TestLexer_Complex(t *testing.T) {
	input := `
		% PDF-1.7 header
		1 0 obj
		<<
			/Type /Catalog
			/Pages 2 0 R
			/Name /Test#20Document
		>>
		endobj

		2 0 obj
		<<
			/Type /Pages
			/Kids [3 0 R]
			/Count 1
		>>
		endobj
	`

	expected := []struct {
		tokType TokenType
		value   string
	}{
		{TokenInteger, "1"},
		{TokenInteger, "0"},
		{TokenKeyword, "obj"},
		{TokenDictStart, "<<"},
		{TokenName, "Type"},
		{TokenName, "Catalog"},
		{TokenName, "Pages"},
		{TokenInteger, "2"},
		{TokenInteger, "0"},
		{TokenKeyword, "R"},
		{TokenName, "Name"},
		{TokenName, "Test Document"},
		{TokenDictEnd, ">>"},
		{TokenKeyword, "endobj"},
		{TokenInteger, "2"},
		{TokenInteger, "0"},
		{TokenKeyword, "obj"},
		{TokenDictStart, "<<"},
		{TokenName, "Type"},
		{TokenName, "Pages"},
		{TokenName, "Kids"},
		{TokenArrayStart, "["},
		{TokenInteger, "3"},
		{TokenInteger, "0"},
		{TokenKeyword, "R"},
		{TokenArrayEnd, "]"},
		{TokenName, "Count"},
		{TokenInteger, "1"},
		{TokenDictEnd, ">>"},
		{TokenKeyword, "endobj"},
		{TokenEOF, ""},
	}

	lexer := NewLexer(strings.NewReader(input))
	for i, exp := range expected {
		tok, err := lexer.NextToken()
		require.NoError(t, err, "token %d", i)
		assert.Equal(t, exp.tokType, tok.Type, "token %d type", i)
		assert.Equal(t, exp.value, tok.Value, "token %d value", i)
	}
}

// TestLexer_Position tests line and column tracking.
func TestLexer_Position(t *testing.T) {
	input := "123\n456\n789"
	lexer := NewLexer(strings.NewReader(input))

	// First token: 123 at line 1
	tok, err := lexer.NextToken()
	require.NoError(t, err)
	assert.Equal(t, 1, tok.Line)

	// Second token: 456 at line 2
	tok, err = lexer.NextToken()
	require.NoError(t, err)
	assert.Equal(t, 2, tok.Line)

	// Third token: 789 at line 3
	tok, err = lexer.NextToken()
	require.NoError(t, err)
	assert.Equal(t, 3, tok.Line)
}

// TestLexer_ReadAll tests the ReadAll convenience method.
func TestLexer_ReadAll(t *testing.T) {
	input := "123 true null"
	lexer := NewLexer(strings.NewReader(input))

	tokens, err := lexer.ReadAll()
	require.NoError(t, err)

	assert.Len(t, tokens, 4) // 3 tokens + EOF
	assert.Equal(t, TokenInteger, tokens[0].Type)
	assert.Equal(t, TokenBoolean, tokens[1].Type)
	assert.Equal(t, TokenNull, tokens[2].Type)
	assert.Equal(t, TokenEOF, tokens[3].Type)
}

// TestLexer_Tokenize tests the Tokenize convenience function.
func TestLexer_Tokenize(t *testing.T) {
	input := "[1 2 3]"
	tokens, err := Tokenize(input)
	require.NoError(t, err)

	assert.Len(t, tokens, 6) // [ 1 2 3 ] EOF (6 tokens total)
	assert.Equal(t, TokenArrayStart, tokens[0].Type)
	assert.Equal(t, TokenInteger, tokens[1].Type)
	assert.Equal(t, TokenInteger, tokens[2].Type)
	assert.Equal(t, TokenInteger, tokens[3].Type)
	assert.Equal(t, TokenArrayEnd, tokens[4].Type)
	assert.Equal(t, TokenEOF, tokens[5].Type)
}

// TestLexer_Reset tests the Reset method.
func TestLexer_Reset(t *testing.T) {
	lexer := NewLexer(strings.NewReader("123"))

	tok, err := lexer.NextToken()
	require.NoError(t, err)
	assert.Equal(t, TokenInteger, tok.Type)

	// Reset with new input
	lexer.Reset(strings.NewReader("true"))

	tok, err = lexer.NextToken()
	require.NoError(t, err)
	assert.Equal(t, TokenBoolean, tok.Type)
	assert.Equal(t, "true", tok.Value)
}

// TestLexer_UnexpectedCharacter tests handling of unexpected characters.
func TestLexer_UnexpectedCharacter(t *testing.T) {
	// > without matching < is unexpected
	lexer := NewLexer(strings.NewReader(">"))
	tok, err := lexer.NextToken()
	assert.Error(t, err)
	assert.Equal(t, TokenError, tok.Type)
}

// TestLexer_MultipleTokens tests reading multiple tokens in sequence.
func TestLexer_MultipleTokens(t *testing.T) {
	input := "123 /Name (string) true [1 2] << /Key /Value >>"

	expected := []struct {
		tokType TokenType
		value   string
	}{
		{TokenInteger, "123"},
		{TokenName, "Name"},
		{TokenString, "string"},
		{TokenBoolean, "true"},
		{TokenArrayStart, "["},
		{TokenInteger, "1"},
		{TokenInteger, "2"},
		{TokenArrayEnd, "]"},
		{TokenDictStart, "<<"},
		{TokenName, "Key"},
		{TokenName, "Value"},
		{TokenDictEnd, ">>"},
		{TokenEOF, ""},
	}

	lexer := NewLexer(strings.NewReader(input))
	for i, exp := range expected {
		tok, err := lexer.NextToken()
		require.NoError(t, err, "token %d", i)
		assert.Equal(t, exp.tokType, tok.Type, "token %d type", i)
		assert.Equal(t, exp.value, tok.Value, "token %d value", i)
	}
}

// TestLexer_EdgeCases tests various edge cases.
func TestLexer_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		tokType  TokenType
		value    string
		hasError bool
	}{
		{"leading dot real", ".5", TokenReal, ".5", false},
		{"trailing dot real", "5.", TokenReal, "5.", false},
		{"just dot", ".", TokenReal, ".", false},
		{"negative dot", "-.", TokenReal, "-.", false},
		{"positive dot", "+.", TokenReal, "+.", false},
		{"empty name", "/", TokenName, "", false},
		{"empty string", "()", TokenString, "", false},
		{"empty hex", "<>", TokenHexString, "", false},
		{"single hex digit", "<A>", TokenHexString, "\xA0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(strings.NewReader(tt.input))
			tok, err := lexer.NextToken()

			if tt.hasError {
				assert.Error(t, err)
				assert.Equal(t, TokenError, tok.Type)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.tokType, tok.Type)
				assert.Equal(t, tt.value, tok.Value)
			}
		})
	}
}

// TestNewToken tests the NewToken helper function.
func TestNewToken(t *testing.T) {
	tok := NewToken(TokenInteger, "123", 5, 10)
	assert.Equal(t, TokenInteger, tok.Type)
	assert.Equal(t, "123", tok.Value)
	assert.Equal(t, 5, tok.Line)
	assert.Equal(t, 10, tok.Column)
}

// TestErrorToken tests the ErrorToken helper function.
func TestErrorToken(t *testing.T) {
	tok := ErrorToken("test error", 5, 10)
	assert.Equal(t, TokenError, tok.Type)
	assert.Equal(t, "test error", tok.Value)
	assert.Equal(t, 5, tok.Line)
	assert.Equal(t, 10, tok.Column)
}

// TestEOFToken tests the EOFToken helper function.
func TestEOFToken(t *testing.T) {
	tok := EOFToken(5, 10)
	assert.Equal(t, TokenEOF, tok.Type)
	assert.Equal(t, "", tok.Value)
	assert.Equal(t, 5, tok.Line)
	assert.Equal(t, 10, tok.Column)
}

// TestLexer_SkipTo tests the skipTo method for error recovery.
func TestLexer_SkipTo(t *testing.T) {
	input := "some garbage endobj 123"
	lexer := NewLexer(strings.NewReader(input))

	err := lexer.skipTo("endobj")
	require.NoError(t, err)

	// After skipping to "endobj", next token should be after it
	tok, err := lexer.NextToken()
	require.NoError(t, err)
	assert.Equal(t, TokenInteger, tok.Type)
	assert.Equal(t, "123", tok.Value)
}

// TestLexer_SkipTo_NotFound tests skipTo when target not found.
func TestLexer_SkipTo_NotFound(t *testing.T) {
	input := "some content without target"
	lexer := NewLexer(strings.NewReader(input))

	err := lexer.skipTo("endobj")
	assert.Error(t, err)
	assert.Equal(t, io.EOF, err)
}

// BenchmarkLexer_SimpleTokens benchmarks tokenization of simple tokens.
func BenchmarkLexer_SimpleTokens(b *testing.B) {
	input := "123 456 789 true false null /Name"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lexer := NewLexer(strings.NewReader(input))
		for {
			tok, err := lexer.NextToken()
			if err != nil || tok.Type == TokenEOF {
				break
			}
		}
	}
}

// BenchmarkLexer_ComplexPDF benchmarks tokenization of complex PDF content.
func BenchmarkLexer_ComplexPDF(b *testing.B) {
	input := `
		1 0 obj
		<< /Type /Catalog /Pages 2 0 R >>
		endobj
		2 0 obj
		<< /Type /Pages /Kids [3 0 R] /Count 1 >>
		endobj
		3 0 obj
		<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>
		endobj
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lexer := NewLexer(strings.NewReader(input))
		_, _ = lexer.ReadAll()
	}
}

// BenchmarkLexer_Strings benchmarks string tokenization.
func BenchmarkLexer_Strings(b *testing.B) {
	input := `(This is a test string with \n escapes and (nested) parentheses)`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lexer := NewLexer(strings.NewReader(input))
		_, _ = lexer.NextToken()
	}
}

// BenchmarkLexer_HexStrings benchmarks hex string tokenization.
func BenchmarkLexer_HexStrings(b *testing.B) {
	input := `<48656C6C6F20576F726C64>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lexer := NewLexer(strings.NewReader(input))
		_, _ = lexer.NextToken()
	}
}

// TestLexerSkipsInlineImageData covers the binary payload of an inline image:
// tokenizing it as content operators is what made pages that embed one fail
// with `unknown token "x"` (the 0x78 of a flate header).
func TestLexerSkipsInlineImageData(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "payload with flate header",
			content: "q\nBI\n/CS/R20\n/W 1\n/H 1\n/BPC 8\n/F/Fl\nID x\x9c\x01\x02\nEI Q\n",
			want:    []string{"q", "BI", "ID", "Q"},
		},
		{
			name:    "payload containing EI without white space before it",
			content: "BI /W 1 /H 1 ID \x01EI\x02EI\x03\nEI Q",
			want:    []string{"BI", "ID", "Q"},
		},
		{
			name:    "CRLF after ID",
			content: "BI /W 1 /H 1 ID\r\n\x9c\x01\x02\nEI Q",
			want:    []string{"BI", "ID", "Q"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(strings.NewReader(tt.content))
			var keywords []string
			for {
				token, err := lexer.NextToken()
				require.NoError(t, err)
				if token.Type == TokenEOF {
					break
				}
				if token.Type == TokenKeyword {
					keywords = append(keywords, token.Value)
				}
			}
			assert.Equal(t, tt.want, keywords)
		})
	}
}

// TestLexerInlineImageWithoutEI checks that a payload with no EI ends the
// stream instead of failing it: the rest of the page is unreadable anyway, and
// an error here would drop the operators parsed before it.
func TestLexerInlineImageWithoutEI(t *testing.T) {
	lexer := NewLexer(strings.NewReader("q BI /W 1 /H 1 ID \x9c\x01\x02"))
	var keywords []string
	for {
		token, err := lexer.NextToken()
		require.NoError(t, err)
		if token.Type == TokenEOF {
			break
		}
		if token.Type == TokenKeyword {
			keywords = append(keywords, token.Value)
		}
	}
	assert.Equal(t, []string{"q", "BI", "ID"}, keywords)
}

// TestLexerInlineImageEdgeCases covers the malformed ends of an inline image:
// none of them may hang, panic, or swallow the rest of the stream.
func TestLexerInlineImageEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "ID at the end of the stream",
			content: "q BI /W 1 /H 1 ID",
			want:    []string{"q", "BI", "ID"},
		},
		{
			name:    "EI at the end of the stream without trailing space",
			content: "BI /W 1 /H 1 ID \x01\x02\nEI",
			want:    []string{"BI", "ID"},
		},
		{
			name:    "BI without ID",
			content: "BI /W 1 /H 1 EI Q",
			want:    []string{"BI", "EI", "Q"},
		},
		{
			name:    "empty payload",
			content: "BI /W 1 /H 1 ID \nEI Q",
			want:    []string{"BI", "ID", "Q"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(strings.NewReader(tt.content))
			var keywords []string
			for {
				token, err := lexer.NextToken()
				require.NoError(t, err)
				if token.Type == TokenEOF {
					break
				}
				if token.Type == TokenKeyword {
					keywords = append(keywords, token.Value)
				}
			}
			assert.Equal(t, tt.want, keywords)
		})
	}
}
