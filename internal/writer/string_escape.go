// Package writer implements PDF writing infrastructure.
package writer

import (
	"encoding/hex"
	"strings"
	"unicode/utf16"
)

// EscapePDFString escapes a string for use in PDF literal strings.
//
// PDF literal strings are enclosed in parentheses: (Hello World)
//
// Escapes:
//   - \ → \\
//   - ( → \(
//   - ) → \)
//   - \n → \n (newline)
//   - \r → \r (carriage return)
//   - \t → \t (tab)
//   - \b → \b (backspace)
//   - \f → \f (form feed)
//
// Unicode characters (including Cyrillic) are passed through as-is.
// The caller is responsible for encoding them appropriately (usually UTF-16BE
// for text strings in PDF).
//
// Example:
//
//	EscapePDFString("Hello")              // "Hello"
//	EscapePDFString("Price: $50 (USD)")   // "Price: $50 \\(USD\\)"
//	EscapePDFString("Line1\nLine2")       // "Line1\\nLine2"
//	EscapePDFString("C:\\path")           // "C:\\\\path"
//	EscapePDFString("Привет")             // "Привет" (unchanged)
//
// Reference: PDF 1.7 Spec, Table 3 (Escape sequences in literal strings).
func EscapePDFString(s string) string {
	// Order is critical: backslash must be escaped first!
	// Otherwise we'll double-escape the backslashes we just added.
	s = strings.ReplaceAll(s, "\\", "\\\\") // \ → \\

	// Escape parentheses
	s = strings.ReplaceAll(s, "(", "\\(") // ( → \(
	s = strings.ReplaceAll(s, ")", "\\)") // ) → \)

	// Escape control characters
	// Note: These are Go escape sequences that represent actual control characters.
	// We convert them to PDF escape sequences.
	s = strings.ReplaceAll(s, "\n", "\\n") // newline (0x0A)
	s = strings.ReplaceAll(s, "\r", "\\r") // carriage return (0x0D)
	s = strings.ReplaceAll(s, "\t", "\\t") // tab (0x09)
	s = strings.ReplaceAll(s, "\b", "\\b") // backspace (0x08)
	s = strings.ReplaceAll(s, "\f", "\\f") // form feed (0x0C)

	return s
}

// isASCII reports whether every rune in s is in the ASCII range (U+0000–U+007F).
func isASCII(s string) bool {
	for _, r := range s {
		if r > 0x7F {
			return false
		}
	}
	return true
}

// EncodeTextString encodes a Go string as a PDF "text string" (ISO 32000-1 §7.9.2).
//
// PDF text strings appear in Info dictionary fields (Title, Author, …), annotation
// contents and titles, form field names and values, and signature metadata. They must
// be encoded as either:
//   - PDFDocEncoding literal string — for pure ASCII text: (Hello World)
//   - UTF-16BE hex string with BOM — for any non-ASCII text: <FEFF0048…>
//
// Choosing the right form based on content avoids mojibake for Cyrillic, CJK,
// Arabic, Greek, emoji, and other non-Latin scripts in compliant PDF readers.
//
// ASCII path: wraps the EscapePDFString result in parentheses.
// Non-ASCII path: encodes as UTF-16BE (with surrogate pairs for U+10000+) and
// returns a hex string prefixed with the BOM FE FF.
//
// Empty string always returns "()" — the empty literal string.
//
// Examples:
//
//	EncodeTextString("")          // "()"
//	EncodeTextString("Hello")    // "(Hello)"
//	EncodeTextString("Привет")   // "<FEFF041F04400438043204350442>"
//	EncodeTextString("你好")      // "<FEFF4F60597D>"
//	EncodeTextString("😀")        // "<FEFFD83DDE00>" (surrogate pair)
//
// Reference: PDF 1.7 Spec §7.9.2, §7.3.4.2, §7.3.4.3.
func EncodeTextString(s string) string {
	if s == "" {
		return "()"
	}

	if isASCII(s) {
		return "(" + EscapePDFString(s) + ")"
	}

	// Encode as UTF-16BE with BOM.
	// utf16.Encode handles surrogate pairs for codepoints above U+FFFF automatically.
	runes := []rune(s)
	u16 := utf16.Encode(runes)

	// Each uint16 encodes as 2 bytes, big-endian.
	b := make([]byte, len(u16)*2)
	for i, v := range u16 {
		b[i*2] = byte(v >> 8)
		b[i*2+1] = byte(v & 0xFF)
	}

	return "<FEFF" + strings.ToUpper(hex.EncodeToString(b)) + ">"
}
