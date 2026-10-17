// Package internal provides internal implementation details for the builder
// package. It must not be imported by external packages.
package internal

import (
	"strings"

	"github.com/coregx/gxpdf/creator"
	"github.com/coregx/gxpdf/internal/linebreak"
	"github.com/coregx/gxpdf/layout"
)

// FontBridge implements layout.FontResolver by delegating to creator's font
// subsystem. It bridges between the layout engine (which uses FontRef) and
// the creator package (which has Standard 14 fonts and CustomFont).
//
// Families are resolved by a FontSet shared with the renderer, so text is
// measured with the same font it is drawn with.
type FontBridge struct {
	fonts *FontSet
}

// NewFontBridge creates a FontBridge resolving families through fonts.
// A nil fonts means no custom fonts and no fallback.
func NewFontBridge(fonts *FontSet) *FontBridge {
	if fonts == nil {
		fonts = NewFontSet(nil, "")
	}
	return &FontBridge{fonts: fonts}
}

// MeasureString implements layout.FontResolver.
// Returns the width of text in PDF points at the given font and size.
func (fb *FontBridge) MeasureString(font layout.FontRef, text string, size float64) float64 {
	f := fb.fonts.Resolve(font)
	if f.Custom != nil {
		return f.Custom.MeasureString(text, size)
	}
	return creator.MeasureText(f.Standard, text, size)
}

// LineHeight implements layout.FontResolver.
// Returns the total line height (ascender + |descender|) in PDF points.
func (fb *FontBridge) LineHeight(font layout.FontRef, size float64) float64 {
	f := fb.fonts.Resolve(font)
	if f.Custom != nil {
		return f.Custom.LineHeight(size)
	}
	return creator.FontLineHeight(f.Standard, size)
}

// Ascender implements layout.FontResolver.
// Returns the ascender height above the baseline in PDF points.
func (fb *FontBridge) Ascender(font layout.FontRef, size float64) float64 {
	f := fb.fonts.Resolve(font)
	if f.Custom != nil {
		return f.Custom.Ascender(size)
	}
	return creator.FontAscender(f.Standard, size)
}

// Descender implements layout.FontResolver.
// Returns the magnitude of the descender below the baseline in PDF points.
// Always returns a positive value.
func (fb *FontBridge) Descender(font layout.FontRef, size float64) float64 {
	f := fb.fonts.Resolve(font)
	var d float64
	if f.Custom != nil {
		d = f.Custom.Descender(size)
	} else {
		d = creator.FontDescender(f.Standard, size)
	}
	if d < 0 {
		d = -d
	}
	return d
}

// LineBreak implements layout.FontResolver. It splits text into lines that
// each fit within maxWidth points.
//
// Lines break at whitespace, which is collapsed to single spaces, and inside
// words at the break opportunities of CJK text (see internal/linebreak): a
// CJK word continues the current line and wraps between its characters.
// A piece wider than maxWidth on its own is split at character boundaries.
func (fb *FontBridge) LineBreak(font layout.FontRef, text string, size float64, maxWidth float64) []string {
	if text == "" {
		return []string{""}
	}
	if maxWidth <= 0 {
		return []string{text}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	current := ""

	for _, word := range words {
		for i, piece := range linebreak.SplitWord(word) {
			candidate := piece
			if current != "" {
				if i == 0 {
					candidate = current + " " + piece
				} else {
					candidate = current + piece
				}
			}
			if fb.MeasureString(font, candidate, size) <= maxWidth {
				current = candidate
				continue
			}

			// The piece doesn't fit on the current line.
			if current != "" {
				lines = append(lines, current)
			}
			current = piece
			if fb.MeasureString(font, piece, size) > maxWidth {
				// Force-split the oversized piece at character level.
				if chars := fb.breakAtChars(font, piece, size, maxWidth); len(chars) > 1 {
					lines, current = splitLast(lines, chars)
				}
			}
		}
	}

	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) == 0 {
		return []string{text}
	}
	return lines
}

// splitLast appends all pieces but the last to lines and returns the last
// piece as the line still being filled ("" when there are no pieces).
func splitLast(lines, pieces []string) (newLines []string, current string) {
	if len(pieces) == 0 {
		return lines, ""
	}
	return append(lines, pieces[:len(pieces)-1]...), pieces[len(pieces)-1]
}

// breakAtChars splits a single token at character boundaries to fit maxWidth.
func (fb *FontBridge) breakAtChars(font layout.FontRef, word string, size, maxWidth float64) []string {
	var lines []string
	current := ""

	for _, r := range word {
		candidate := current + string(r)
		if fb.MeasureString(font, candidate, size) <= maxWidth {
			current = candidate
		} else {
			if current != "" {
				lines = append(lines, current)
			}
			current = string(r)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
