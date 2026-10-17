// Package linebreak finds line break opportunities inside words: text without
// whitespace that may still be broken, as Chinese, Japanese and Korean text.
//
// Breaks between words, at whitespace, are left to the caller. The rules are
// a subset of UAX #14 as CSS applies it with line-break: normal:
//
//   - a break is allowed between two characters when either of them is CJK
//     (ideographs, kana, Hangul, CJK and fullwidth punctuation);
//   - except before a closing punctuation mark (、。」) ! ?) and after an
//     opening one (「（ ( [), which stay with the text they enclose or end
//     (kinsoku shori).
//
// Small kana and the prolonged sound mark ー may start a line, as with CSS
// line-break: normal (strict forbids it).
package linebreak

import "unicode"

// noBreakBefore holds the characters a line may not start with: closing
// punctuation, full stops, commas, exclamation and question marks, and
// iteration marks (UAX #14 classes CL, CP, EX, IS and NS).
//
//nolint:gochecknoglobals // Character class table is intentionally global
var noBreakBefore = runeSet(
	")]},.:;!?" +
		"、。，．・：；？！）］｝」』】〕〉》〙〗〟’”" +
		"ゝゞヽヾ々〻゛゜" +
		"｡｣､･",
)

// noBreakAfter holds the characters a line may not end with: opening
// punctuation (UAX #14 class OP and opening quotation marks).
//
//nolint:gochecknoglobals // Character class table is intentionally global
var noBreakAfter = runeSet("([{（［｛「『【〔〈《〘〖〝‘“｢")

func runeSet(s string) map[rune]struct{} {
	m := make(map[rune]struct{})
	for _, r := range s {
		m[r] = struct{}{}
	}
	return m
}

// IsCJK reports whether r is a character of CJK text that allows a line break
// next to it without a space: Han ideographs, Hiragana, Katakana, Hangul,
// CJK symbols and punctuation, and fullwidth and halfwidth forms.
func IsCJK(r rune) bool {
	switch {
	case r >= 0x3000 && r <= 0x303F: // CJK Symbols and Punctuation
		return true
	case r >= 0xFF00 && r <= 0xFFEF: // Halfwidth and Fullwidth Forms
		return true
	}
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}

// CanBreak reports whether a line may break between a and b, two adjacent
// characters of a word.
func CanBreak(a, b rune) bool {
	if !IsCJK(a) && !IsCJK(b) {
		return false
	}
	if _, ok := noBreakBefore[b]; ok {
		return false
	}
	_, ok := noBreakAfter[a]
	return !ok
}

// SplitWord splits word, text without whitespace, at its line break
// opportunities. Joined, the pieces give word back. A word without
// opportunities is returned as the only piece; an empty word gives none.
func SplitWord(word string) []string {
	var pieces []string
	start := 0
	prev := rune(-1)
	for i, r := range word {
		if prev >= 0 && CanBreak(prev, r) {
			pieces = append(pieces, word[start:i])
			start = i
		}
		prev = r
	}
	if start < len(word) {
		pieces = append(pieces, word[start:])
	}
	return pieces
}
