package fonts

import (
	"bytes"
	"testing"
)

func TestWinAnsiCode(t *testing.T) {
	tests := []struct {
		r    rune
		want byte
		ok   bool
	}{
		{'A', 'A', true},
		{'\t', '\t', true},
		{0x00A0, 0xA0, true}, // no-break space
		{'é', 0xE9, true},
		{'ü', 0xFC, true},
		{'ß', 0xDF, true},
		{'ÿ', 0xFF, true},
		{'€', 0x80, true},
		{'–', 0x96, true},
		{'—', 0x97, true},
		{'“', 0x93, true},
		{'”', 0x94, true},
		{'…', 0x85, true},
		{'Ÿ', 0x9F, true},
		{0x0085, 0, false}, // C1 control: its code holds the ellipsis
		{'Ł', 0, false},
		{'Ж', 0, false},
		{'中', 0, false},
		{'😀', 0, false},
	}
	for _, tt := range tests {
		got, ok := WinAnsiCode(tt.r)
		if ok != tt.ok || (ok && got != tt.want) {
			t.Errorf("WinAnsiCode(%U) = %#x, %v; want %#x, %v", tt.r, got, ok, tt.want, tt.ok)
		}
	}
}

// TestWinAnsiCode_OneCharacterPerCode checks that no two characters share a
// code, and that the 0x80-0x9F block assigns exactly the 27 codes of the spec.
func TestWinAnsiCode_OneCharacterPerCode(t *testing.T) {
	seen := make(map[byte]rune)
	for r := rune(0); r <= 0xFFFF; r++ {
		b, ok := WinAnsiCode(r)
		if !ok {
			continue
		}
		if prev, dup := seen[b]; dup {
			t.Fatalf("code %#x assigned to %U and %U", b, prev, r)
		}
		seen[b] = r
	}
	high := 0
	for b := range seen {
		if b >= 0x80 && b <= 0x9F {
			high++
		}
	}
	if high != 27 {
		t.Errorf("codes 0x80-0x9F assigned: %d, want 27", high)
	}
	for _, unassigned := range []byte{0x81, 0x8D, 0x8F, 0x90, 0x9D} {
		if r, ok := seen[unassigned]; ok {
			t.Errorf("unassigned code %#x mapped from %U", unassigned, r)
		}
	}
}

func TestEncodeText(t *testing.T) {
	tests := []struct {
		font, text string
		want       []byte
	}{
		{"Helvetica", "06:00 – 18:00", []byte("06:00 \x96 18:00")},
		{"Times-Roman", "Müller", []byte("M\xfcller")},
		{"Courier", "€ 5", []byte("\x80 5")},
		{"Helvetica-Bold", "„Grüße“", []byte("\x84Gr\xfc\xdfe\x93")},
		{"Helvetica", "Привет", []byte("??????")},
		{"Helvetica", "a(b)\\c", []byte("a(b)\\c")},
		{"NoSuchFont", "é", []byte{0xE9}},
		// Symbolic fonts take character codes of their built-in encodings.
		{"Symbol", "abc", []byte("abc")},
		{"Symbol", string(rune(0xB7)), []byte{0xB7}},
		{"ZapfDingbats", "4", []byte("4")},
		{"ZapfDingbats", "–", []byte("?")},
	}
	for _, tt := range tests {
		if got := EncodeText(tt.font, tt.text); !bytes.Equal(got, tt.want) {
			t.Errorf("EncodeText(%q, %q) = %q, want %q", tt.font, tt.text, got, tt.want)
		}
	}
}

// TestMeasureString_MatchesDrawnCodes checks that text is measured by the
// glyphs it is drawn with: WinAnsi characters by their own widths, characters
// outside the encoding as the '?' drawn in their place.
func TestMeasureString_MatchesDrawnCodes(t *testing.T) {
	m := GetMetrics("Helvetica")
	if got := m.MeasureString("–", 1000); got != 556 {
		t.Errorf("en dash width = %v, want 556", got)
	}
	if got := m.MeasureString("€", 1000); got != 556 {
		t.Errorf("euro width = %v, want 556", got)
	}
	if got, want := m.MeasureString("Привет", 10), m.MeasureString("??????", 10); got != want {
		t.Errorf("unencodable text width = %v, want width of '?' x6 = %v", got, want)
	}
	if got, want := GetMetrics("Symbol").MeasureString("中", 10), GetMetrics("Symbol").MeasureString("?", 10); got != want {
		t.Errorf("Symbol unencodable width = %v, want %v", got, want)
	}
}

// TestStandard14Widths_CoverWinAnsi checks that every printable character of
// WinAnsiEncoding has a width in every text font, so none is measured with
// DefaultWidth. No-break space and soft hyphen are measured as the space and
// hyphen glyphs they are drawn with.
func TestStandard14Widths_CoverWinAnsi(t *testing.T) {
	var chars []rune
	for r := rune(0x20); r <= 0xFFFF; r++ {
		if b, ok := WinAnsiCode(r); ok && b != 0x7F {
			chars = append(chars, r)
		}
	}
	if len(chars) != 95+27+96 {
		t.Fatalf("printable WinAnsi characters: %d, want %d", len(chars), 95+27+96)
	}
	for _, f := range []*Standard14Font{
		Helvetica, HelveticaBold, HelveticaOblique, HelveticaBoldOblique,
		TimesRoman, TimesBold, TimesItalic, TimesBoldItalic,
		Courier, CourierBold, CourierOblique, CourierBoldOblique,
	} {
		m := f.GetMetrics()
		for _, r := range chars {
			_, drawn := m.code(r)
			if _, ok := m.CharWidths[drawn]; !ok {
				t.Errorf("%s: no width for %U %q", f.Name, r, r)
			}
		}
		if got, want := m.MeasureString(string([]rune{0xA0, 0xAD}), 1000), m.MeasureString(" -", 1000); got != want {
			t.Errorf("%s: no-break space + soft hyphen = %v, want %v", f.Name, got, want)
		}
	}
}
