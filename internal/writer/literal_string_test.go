package writer

import (
	"strings"
	"testing"
)

func TestLiteralString(t *testing.T) {
	tests := []struct {
		codes []byte
		want  string
	}{
		{nil, "()"},
		{[]byte("Hello"), "(Hello)"},
		{[]byte(`a(b)\c`), `(a\(b\)\\c)`},
		{[]byte("1\n2\r3\t4\b5\f6"), `(1\n2\r3\t4\b5\f6)`},
		{[]byte{0x00, 0x1F, 0x7F}, `(\000\037\177)`},
		// An octal escape is always three digits, so a following digit is
		// not read as part of it.
		{[]byte("06:00 \x96 18:00"), `(06:00 \226 18:00)`},
		{[]byte("\x801"), `(\2001)`},
		{[]byte{0xFF}, `(\377)`},
	}
	for _, tt := range tests {
		if got := literalString(tt.codes); got != tt.want {
			t.Errorf("literalString(%q) = %s, want %s", tt.codes, got, tt.want)
		}
	}
}

// TestShowText_WinAnsi checks that text drawn in a Standard 14 font is written
// in WinAnsiEncoding, not as UTF-8 (bug-011).
func TestShowText_WinAnsi(t *testing.T) {
	csw := NewContentStreamWriter()
	csw.ShowText("Müller – €")
	csw.ShowTextNextLine("Привет")
	got := string(csw.Bytes())
	for _, want := range []string{`(M\374ller \226 \200) Tj`, `(??????) '`} {
		if !strings.Contains(got, want) {
			t.Errorf("content stream %q does not contain %q", got, want)
		}
	}
}

// TestGenerateContentStream_FontEncoding checks that text is encoded for the
// font it is drawn in: WinAnsiEncoding for text fonts, character codes for
// the symbolic fonts.
func TestGenerateContentStream_FontEncoding(t *testing.T) {
	content, _, err := GenerateContentStream([]TextOp{
		{Text: "Grüße", Font: "Times-Roman", Size: 12},
		{Text: string([]rune{0xB7, 0xE5}), Font: "Symbol", Size: 12},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(content)
	for _, want := range []string{`(Gr\374\337e) Tj`, `(\267\345) Tj`} {
		if !strings.Contains(got, want) {
			t.Errorf("content stream %q does not contain %q", got, want)
		}
	}
}
