package internal

import (
	"strings"
	"testing"

	"github.com/coregx/gxpdf/layout"
)

// newTestBridge creates a FontBridge with no custom fonts (Standard 14 only).
func newTestBridge() *FontBridge {
	return NewFontBridge(nil)
}

func TestFontBridge_MeasureString_Standard14(t *testing.T) {
	fb := newTestBridge()
	helvetica := layout.FontRef{Family: "Helvetica", Weight: layout.WeightNormal}

	// Non-empty string should have positive width.
	width := fb.MeasureString(helvetica, "Hello", 12)
	if width <= 0 {
		t.Errorf("MeasureString(Helvetica, Hello, 12) = %f, want > 0", width)
	}

	// Empty string should have zero width.
	emptyWidth := fb.MeasureString(helvetica, "", 12)
	if emptyWidth != 0 {
		t.Errorf("MeasureString(Helvetica, empty, 12) = %f, want 0", emptyWidth)
	}
}

func TestFontBridge_MeasureString_ScalesWithSize(t *testing.T) {
	fb := newTestBridge()
	font := layout.FontRef{Family: "Helvetica"}

	w12 := fb.MeasureString(font, "Test", 12)
	w24 := fb.MeasureString(font, "Test", 24)

	if w12 <= 0 || w24 <= 0 {
		t.Fatal("widths must be positive")
	}
	// 24pt should be roughly twice as wide as 12pt.
	ratio := w24 / w12
	if ratio < 1.8 || ratio > 2.2 {
		t.Errorf("width ratio 24pt/12pt = %f, expected ~2.0", ratio)
	}
}

func TestFontBridge_LineHeight_Standard14(t *testing.T) {
	fb := newTestBridge()
	font := layout.FontRef{Family: "Helvetica"}

	lh := fb.LineHeight(font, 12)
	if lh <= 0 {
		t.Errorf("LineHeight(Helvetica, 12) = %f, want > 0", lh)
	}
	// Line height should scale proportionally with font size.
	lh24 := fb.LineHeight(font, 24)
	if lh24 <= 0 {
		t.Errorf("LineHeight(Helvetica, 24) = %f, want > 0", lh24)
	}
	// Doubling the font size should roughly double the line height.
	ratio := lh24 / lh
	if ratio < 1.8 || ratio > 2.2 {
		t.Errorf("LineHeight ratio 24pt/12pt = %f, expected ~2.0", ratio)
	}
}

func TestFontBridge_Ascender_Standard14(t *testing.T) {
	fb := newTestBridge()
	font := layout.FontRef{Family: "Helvetica"}

	asc := fb.Ascender(font, 12)
	if asc <= 0 {
		t.Errorf("Ascender(Helvetica, 12) = %f, want > 0", asc)
	}
}

func TestFontBridge_Descender_AlwaysPositive(t *testing.T) {
	fb := newTestBridge()
	font := layout.FontRef{Family: "Helvetica"}

	desc := fb.Descender(font, 12)
	if desc < 0 {
		t.Errorf("Descender should always be non-negative, got %f", desc)
	}
}

func TestFontBridge_LineBreak_SimpleText(t *testing.T) {
	fb := newTestBridge()
	font := layout.FontRef{Family: "Helvetica"}

	// Wide area — all text fits on one line.
	lines := fb.LineBreak(font, "Hello World", 12, 1000)
	if len(lines) != 1 {
		t.Errorf("expected 1 line for wide area, got %d: %v", len(lines), lines)
	}
	if lines[0] != "Hello World" {
		t.Errorf("line = %q, want %q", lines[0], "Hello World")
	}
}

func TestFontBridge_LineBreak_EmptyString(t *testing.T) {
	fb := newTestBridge()
	font := layout.FontRef{Family: "Helvetica"}

	lines := fb.LineBreak(font, "", 12, 100)
	if len(lines) != 1 || lines[0] != "" {
		t.Errorf("empty string should return [%q], got %v", "", lines)
	}
}

func TestFontBridge_LineBreak_NarrowArea(t *testing.T) {
	fb := newTestBridge()
	font := layout.FontRef{Family: "Helvetica"}

	// Very narrow area forces line wrapping.
	lines := fb.LineBreak(font, "Hello World Test", 12, 30)
	if len(lines) <= 1 {
		t.Errorf("narrow area should produce multiple lines, got %d: %v", len(lines), lines)
	}
}

func TestFontBridge_LineBreak_ZeroMaxWidth(t *testing.T) {
	fb := newTestBridge()
	font := layout.FontRef{Family: "Helvetica"}

	lines := fb.LineBreak(font, "Hello World", 12, 0)
	// Zero max width should return text as-is.
	if len(lines) == 0 {
		t.Error("should return at least one line for zero maxWidth")
	}
}

func TestFontBridge_LineBreak_PreservesContent(t *testing.T) {
	fb := newTestBridge()
	font := layout.FontRef{Family: "Helvetica"}

	text := "one two three four five six seven eight nine ten"
	lines := fb.LineBreak(font, text, 12, 80)

	// Rejoin lines and verify no words are lost.
	rejoined := strings.Join(lines, " ")
	originalWords := strings.Fields(text)
	rejoinedWords := strings.Fields(rejoined)

	if len(originalWords) != len(rejoinedWords) {
		t.Errorf("word count mismatch: original %d, rejoined %d\nlines: %v",
			len(originalWords), len(rejoinedWords), lines)
	}
}

func TestFontBridge_ContainsCJK(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"hello", false},
		{"你好", true},
		{"こんにちは", true},
		{"안녕", true},
		{"hello世界", true},
		{"", false},
	}
	for _, tc := range tests {
		got := containsCJK(tc.input)
		if got != tc.want {
			t.Errorf("containsCJK(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestFontBridge_IsCJKRune(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{'A', false},
		{'z', false},
		{'1', false},
		{'你', true},      // CJK Unified Ideograph
		{'あ', true},      // Hiragana
		{'ア', true},      // Katakana
		{'가', true},      // Hangul
		{0x4E00, true},   // First CJK Unified Ideograph
		{0x9FFF, true},   // Last CJK Unified Ideograph (basic)
		{0x10000, false}, // Outside CJK range
	}
	for _, tc := range tests {
		got := isCJKRune(tc.r)
		if got != tc.want {
			t.Errorf("isCJKRune(%q / U+%04X) = %v, want %v", tc.r, tc.r, got, tc.want)
		}
	}
}

func TestFontBridge_BreakAtChars(t *testing.T) {
	fb := newTestBridge()
	font := layout.FontRef{Family: "Helvetica"}

	// A very long word that exceeds a narrow width.
	word := "superlongwordthatexceedsbounds"
	lines := fb.breakAtChars(font, word, 12, 30)

	if len(lines) <= 1 {
		t.Errorf("long word at narrow width should produce multiple lines, got %d", len(lines))
	}
	// All characters should be present.
	rejoined := strings.Join(lines, "")
	if rejoined != word {
		t.Errorf("breakAtChars lost chars: got %q, want %q", rejoined, word)
	}
}
