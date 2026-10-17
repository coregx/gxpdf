package gxpdf

import (
	"math"
	"strings"
	"testing"

	"github.com/coregx/gxpdf/builder"
	"github.com/coregx/gxpdf/creator"
)

// winAnsiSample has characters from each part of WinAnsiEncoding outside
// ASCII: Latin-1 letters and signs, and the 0x80-0x9F block.
var winAnsiSample = []string{
	"Jürgen Grüße façade naïve ½ © °",
	"€ – — „“ ” ‘’ … • † ‰ ™ Š š Ž ž Œ œ Ÿ ƒ",
}

// TestStandard14_NonASCIIRoundTrip is the bug-011 repro: non-ASCII text in a
// Standard 14 font used to be written as UTF-8 bytes and read back as mojibake
// ("06:00 â€“ 18:00"). It is now written in WinAnsiEncoding.
func TestStandard14_NonASCIIRoundTrip(t *testing.T) {
	lines := append([]string{"06:00 – 18:00", "Müller", "€ 5"}, winAnsiSample...)
	families := []string{"Helvetica", "Times", "Courier"}

	doc := builder.NewBuilder(builder.WithPageSize(builder.A4))
	doc.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) {
			for _, f := range families {
				for _, l := range lines {
					c.Text(l, builder.FontFamily(f))
				}
			}
		})
	})
	data, err := doc.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	text := pageText(t, data)
	for _, l := range lines {
		if n := strings.Count(text, l); n != len(families) {
			t.Errorf("%q found %d times, want %d\npage text:\n%s", l, n, len(families), text)
		}
	}
}

// TestStandard14_CreatorAndWatermarkEncoding covers the creator text and
// watermark paths, which are written separately from builder text.
func TestStandard14_CreatorAndWatermarkEncoding(t *testing.T) {
	c := creator.New()
	page, err := c.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	if err := page.AddText("Größe: 5 €", 72, 700, creator.TimesRoman, 12); err != nil {
		t.Fatal(err)
	}
	wm := creator.NewTextWatermark("ÉBAUCHE – NON VALIDÉ")
	if err := page.DrawWatermark(wm); err != nil {
		t.Fatal(err)
	}
	data, err := c.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	// The watermark's font used to have no font object: /F1 0 0 R.
	if strings.Contains(string(data), " 0 0 R") {
		t.Errorf("PDF references object 0")
	}
	text := pageText(t, data)
	for _, want := range []string{"Größe: 5 €", "ÉBAUCHE – NON VALIDÉ"} {
		if !strings.Contains(text, want) {
			t.Errorf("%q not found in page text:\n%s", want, text)
		}
	}
}

// TestStandard14_RightAlignedNonASCII checks that non-ASCII text is placed by
// the widths of the glyphs it is drawn with: right-aligned lines end at the
// same x whatever their characters. The right edge is computed from the AFM
// widths, not TextElement.Width, which the extractor only estimates for
// Standard 14 fonts.
func TestStandard14_RightAlignedNonASCII(t *testing.T) {
	lines := []string{"Total", "– € 5 —", "Müller ½", "Привет"}

	doc := builder.NewBuilder(builder.WithPageSize(builder.A4))
	doc.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) {
			for _, l := range lines {
				c.Text(l, builder.AlignRight())
			}
		})
	})
	data, err := doc.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	pdf, err := OpenFromBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	defer pdf.Close()
	elems, err := pdf.ExtractTextElementsFromPage(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != len(lines) {
		t.Fatalf("got %d text runs, want %d: %+v", len(elems), len(lines), elems)
	}

	// Hand-computed Helvetica widths at 12pt: the line is placed by these.
	afm := []float64{
		(611 + 556 + 278 + 556 + 222) * 0.012,                   // Total
		(556 + 278 + 556 + 278 + 556 + 278 + 1000) * 0.012,      // – € 5 —
		(833 + 556 + 222 + 222 + 556 + 333 + 278 + 834) * 0.012, // Müller ½
		6 * 556 * 0.012, // ?????? (Cyrillic is drawn as '?')
	}
	right := elems[0].X + afm[0]
	for i, e := range elems {
		if r := e.X + afm[i]; math.Abs(r-right) > 0.05 {
			t.Errorf("%q ends at x=%.2f, want %.2f", e.Text, r, right)
		}
	}
	// Text outside WinAnsiEncoding is drawn as '?' and measured as '?'.
	if got := elems[3].Text; got != "??????" {
		t.Errorf("Cyrillic in Helvetica = %q, want \"??????\"", got)
	}
}

func pageText(t *testing.T, data []byte) string {
	t.Helper()
	pdf, err := OpenFromBytes(data)
	if err != nil {
		t.Fatalf("OpenFromBytes: %v", err)
	}
	defer pdf.Close()
	text, err := pdf.ExtractTextFromPage(1)
	if err != nil {
		t.Fatalf("ExtractTextFromPage: %v", err)
	}
	return text
}
