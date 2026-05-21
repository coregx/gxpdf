package extractor

// Tests for mergeAdjacentElements and AssembleText.
//
// Root cause addressed: wkhtmltopdf (and similar generators) emit one Tj
// operator per glyph with explicit Td kern moves between them, producing
// one TextElement per character.  When callers concatenate elements with
// a separating space they produce "D O M I C I L I O" instead of
// "DOMICILIO ESTANDAR".
//
// Fix: mergeAdjacentElements collapses runs of same-line, positionally-adjacent
// TextElements into word-level runs.  AssembleText reconstructs a readable string
// from merged elements, inserting spaces only at genuine word boundaries.

import (
	"os"
	"strings"
	"testing"

	"github.com/coregx/gxpdf/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── canMerge unit tests ─────────────────────────────────────────────────────

func TestCanMerge_AdjacentSameFontSameLine_True(t *testing.T) {
	// Two glyphs right next to each other on the same line.
	a := &TextElement{Text: "D", X: 10, Y: 100, Width: 7, Height: 12, FontName: "F1", FontSize: 12}
	b := &TextElement{Text: "O", X: 17, Y: 100, Width: 7, Height: 12, FontName: "F1", FontSize: 12}
	assert.True(t, canMerge(a, b), "adjacent glyphs on same line must be merge-eligible")
}

func TestCanMerge_DifferentFont_False(t *testing.T) {
	a := &TextElement{Text: "D", X: 10, Y: 100, Width: 7, Height: 12, FontName: "F1", FontSize: 12}
	b := &TextElement{Text: "O", X: 17, Y: 100, Width: 7, Height: 12, FontName: "F2", FontSize: 12}
	assert.False(t, canMerge(a, b), "different font names must not merge")
}

func TestCanMerge_DifferentLine_False(t *testing.T) {
	a := &TextElement{Text: "D", X: 10, Y: 100, Width: 7, Height: 12, FontName: "F1", FontSize: 12}
	b := &TextElement{Text: "O", X: 10, Y: 80, Width: 7, Height: 12, FontName: "F1", FontSize: 12}
	// Y difference = 20, half height = 6, threshold = 6 — 20 > 6 so different line
	assert.False(t, canMerge(a, b), "glyphs on different lines must not merge")
}

func TestCanMerge_GapBelowThreshold_True(t *testing.T) {
	// Gap = 5, threshold = 12 * 1.5 = 18 — should merge.
	a := &TextElement{Text: "D", X: 10, Y: 100, Width: 7, Height: 12, FontName: "F1", FontSize: 12}
	b := &TextElement{Text: "O", X: 22, Y: 100, Width: 7, Height: 12, FontName: "F1", FontSize: 12}
	// gap = 22 - (10+7) = 5
	assert.True(t, canMerge(a, b), "gap %v < threshold %v must merge", 5, 18)
}

func TestCanMerge_GapAboveThreshold_False(t *testing.T) {
	// Gap = 30 (word boundary), threshold = 12 * 1.5 = 18 — must NOT merge.
	a := &TextElement{Text: "D", X: 10, Y: 100, Width: 7, Height: 12, FontName: "F1", FontSize: 12}
	b := &TextElement{Text: "O", X: 47, Y: 100, Width: 7, Height: 12, FontName: "F1", FontSize: 12}
	// gap = 47 - 17 = 30
	assert.False(t, canMerge(a, b), "gap 30 > threshold 18 must not merge")
}

func TestCanMerge_OverlappingElements_TreatedAsMerge(t *testing.T) {
	// Slight overlap (negative gap) is treated as merge-eligible — width estimation
	// is only a rough heuristic for CID fonts.
	a := &TextElement{Text: "D", X: 10, Y: 100, Width: 10, Height: 12, FontName: "F1", FontSize: 12}
	b := &TextElement{Text: "O", X: 18, Y: 100, Width: 10, Height: 12, FontName: "F1", FontSize: 12}
	// gap = 18 - 20 = -2 (overlap)
	assert.True(t, canMerge(a, b), "overlapping glyphs (CID font width heuristic) must merge")
}

func TestCanMerge_BMovesLeft_False(t *testing.T) {
	// b.X < a.X — right-to-left flow must not merge.
	a := &TextElement{Text: "D", X: 50, Y: 100, Width: 7, Height: 12, FontName: "F1", FontSize: 12}
	b := &TextElement{Text: "O", X: 30, Y: 100, Width: 7, Height: 12, FontName: "F1", FontSize: 12}
	assert.False(t, canMerge(a, b), "element moving left must not merge")
}

func TestCanMerge_DifferentFontSize_False(t *testing.T) {
	a := &TextElement{Text: "D", X: 10, Y: 100, Width: 7, Height: 12, FontName: "F1", FontSize: 12}
	b := &TextElement{Text: "O", X: 17, Y: 100, Width: 7, Height: 14, FontName: "F1", FontSize: 14}
	assert.False(t, canMerge(a, b), "different font sizes must not merge")
}

// ─── mergeAdjacentElements unit tests ────────────────────────────────────────

func TestMergeAdjacentElements_Empty(t *testing.T) {
	result := mergeAdjacentElements([]*TextElement{})
	assert.Empty(t, result)
}

func TestMergeAdjacentElements_Single(t *testing.T) {
	elem := &TextElement{Text: "A", X: 10, Y: 100, Width: 7, Height: 12, FontName: "F1", FontSize: 12}
	result := mergeAdjacentElements([]*TextElement{elem})
	require.Len(t, result, 1)
	assert.Equal(t, "A", result[0].Text)
}

func TestMergeAdjacentElements_PerGlyphCIDFont_MergedIntoWord(t *testing.T) {
	// Simulate wkhtmltopdf pattern: individual glyphs with Td moves.
	// "DOMICILIO" = 9 glyphs, each ~7 pts wide at 12pt font.
	word := "DOMICILIO"
	glyphWidth := 7.0
	elements := make([]*TextElement, len(word))
	for i, ch := range word {
		elements[i] = &TextElement{
			Text:     string(ch),
			X:        float64(i) * glyphWidth,
			Y:        100,
			Width:    glyphWidth,
			Height:   12,
			FontName: "F7",
			FontSize: 12,
		}
	}

	result := mergeAdjacentElements(elements)
	require.Len(t, result, 1, "9 adjacent glyphs must merge into 1 element")
	assert.Equal(t, "DOMICILIO", result[0].Text)
	assert.Equal(t, 0.0, result[0].X, "merged element starts at X of first glyph")
}

func TestMergeAdjacentElements_TwoWords_TwoElements(t *testing.T) {
	// "AB CD" — two words separated by a genuine word gap.
	// Gap = 30 pts, threshold = 12 * 1.5 = 18 pts → word boundary.
	elems := []*TextElement{
		{Text: "A", X: 0, Y: 100, Width: 8, Height: 12, FontName: "F1", FontSize: 12},
		{Text: "B", X: 8, Y: 100, Width: 8, Height: 12, FontName: "F1", FontSize: 12},
		// gap = 38 - 16 = 22 > 18 → word boundary
		{Text: "C", X: 38, Y: 100, Width: 8, Height: 12, FontName: "F1", FontSize: 12},
		{Text: "D", X: 46, Y: 100, Width: 8, Height: 12, FontName: "F1", FontSize: 12},
	}

	result := mergeAdjacentElements(elems)
	require.Len(t, result, 2, "two words must produce two merged elements")
	assert.Equal(t, "AB", result[0].Text)
	assert.Equal(t, "CD", result[1].Text)
}

func TestMergeAdjacentElements_DifferentFonts_NotMerged(t *testing.T) {
	// Font change mid-word (e.g. bold glyph embedded in regular word).
	elems := []*TextElement{
		{Text: "A", X: 0, Y: 100, Width: 8, Height: 12, FontName: "F1", FontSize: 12},
		{Text: "B", X: 8, Y: 100, Width: 8, Height: 12, FontName: "F2", FontSize: 12},
	}
	result := mergeAdjacentElements(elems)
	require.Len(t, result, 2, "font change must prevent merge")
}

func TestMergeAdjacentElements_MultiLine_NotMerged(t *testing.T) {
	// Two lines of text must not merge across lines.
	elems := []*TextElement{
		{Text: "X", X: 0, Y: 100, Width: 8, Height: 12, FontName: "F1", FontSize: 12},
		{Text: "Y", X: 8, Y: 80, Width: 8, Height: 12, FontName: "F1", FontSize: 12}, // different line
	}
	result := mergeAdjacentElements(elems)
	require.Len(t, result, 2, "elements on different lines must not merge")
}

func TestMergeAdjacentElements_Idempotent(t *testing.T) {
	// Running merge twice on already-merged output must produce the same result.
	elems := []*TextElement{
		{Text: "DOMICILIO", X: 0, Y: 100, Width: 63, Height: 12, FontName: "F1", FontSize: 12},
	}
	once := mergeAdjacentElements(elems)
	twice := mergeAdjacentElements(once)
	require.Len(t, twice, 1)
	assert.Equal(t, "DOMICILIO", twice[0].Text)
}

func TestMergeAdjacentElements_PreservesBoundingBox(t *testing.T) {
	// The merged element must span from the left edge of the first glyph to the
	// right edge of the last glyph.
	elems := []*TextElement{
		{Text: "A", X: 10, Y: 100, Width: 7, Height: 12, FontName: "F1", FontSize: 12},
		{Text: "B", X: 17, Y: 100, Width: 8, Height: 12, FontName: "F1", FontSize: 12},
		{Text: "C", X: 25, Y: 100, Width: 6, Height: 12, FontName: "F1", FontSize: 12},
	}
	result := mergeAdjacentElements(elems)
	require.Len(t, result, 1)
	assert.Equal(t, 10.0, result[0].X, "merged X = first glyph X")
	// Width = (last.X + last.Width) - first.X = (25 + 6) - 10 = 21
	assert.Equal(t, 21.0, result[0].Width, "merged width spans all glyphs")
}

// ─── AssembleText unit tests ──────────────────────────────────────────────────

func TestAssembleText_Empty(t *testing.T) {
	assert.Equal(t, "", AssembleText([]*TextElement{}))
}

func TestAssembleText_SingleElement(t *testing.T) {
	elems := []*TextElement{
		{Text: "Hello", X: 0, Y: 100, Width: 30, Height: 12, FontName: "F1", FontSize: 12},
	}
	assert.Equal(t, "Hello", AssembleText(elems))
}

func TestAssembleText_TwoWordsWithGap_SpaceInserted(t *testing.T) {
	elems := []*TextElement{
		{Text: "DOMICILIO", X: 0, Y: 100, Width: 54, Height: 12, FontName: "F1", FontSize: 12},
		// gap = 74 - 54 = 20 > 12 * 1.0 = 12 → space inserted
		{Text: "ESTANDAR", X: 74, Y: 100, Width: 48, Height: 12, FontName: "F1", FontSize: 12},
	}
	assert.Equal(t, "DOMICILIO ESTANDAR", AssembleText(elems))
}

func TestAssembleText_TwoWordsNoGap_NoExtraSpace(t *testing.T) {
	// Elements are adjacent (no gap) — still no space because they are
	// distinct merged runs on the same line without a word-gap.
	// Gap = 0, threshold = 12 — 0 <= 12, so no space.
	elems := []*TextElement{
		{Text: "AB", X: 0, Y: 100, Width: 16, Height: 12, FontName: "F1", FontSize: 12},
		{Text: "CD", X: 16, Y: 100, Width: 16, Height: 12, FontName: "F1", FontSize: 12},
	}
	assert.Equal(t, "ABCD", AssembleText(elems))
}

func TestAssembleText_DifferentLines_NewlineInserted(t *testing.T) {
	elems := []*TextElement{
		{Text: "Line1", X: 0, Y: 100, Width: 30, Height: 12, FontName: "F1", FontSize: 12},
		{Text: "Line2", X: 0, Y: 80, Width: 30, Height: 12, FontName: "F1", FontSize: 12},
	}
	result := AssembleText(elems)
	assert.Equal(t, "Line1\nLine2", result)
}

func TestAssembleText_MultipleWordsAndLines(t *testing.T) {
	// Gaps > wordSpaceGapFactor*fontSize (1.0*12=12) trigger a space.
	// Gaps <= 12 are treated as intra-word (no space).
	// Use gap=20 between each word (20 > 12) so spaces are inserted.
	elems := []*TextElement{
		{Text: "N°", X: 0, Y: 200, Width: 20, Height: 12, FontName: "F1", FontSize: 12},
		// gap from right edge 20 to X=40 → gap=20 > 12 → space
		{Text: "de", X: 40, Y: 200, Width: 15, Height: 12, FontName: "F1", FontSize: 12},
		// gap from right edge 55 to X=75 → gap=20 > 12 → space
		{Text: "seguimiento:", X: 75, Y: 200, Width: 70, Height: 12, FontName: "F1", FontSize: 12},
		// different line (Y 185 vs 200, delta=15 > height/2=6)
		{Text: "360002865586280", X: 0, Y: 185, Width: 90, Height: 12, FontName: "F1", FontSize: 12},
	}
	result := AssembleText(elems)
	assert.Equal(t, "N° de seguimiento:\n360002865586280", result)
}

// ─── Integration test: real Andreani shipping label PDF ───────────────────────

// TestAndreaniShippingLabel_NoPerGlyphSpaces verifies that extracting text from
// the Andreani shipping-label PDF no longer produces "D O M I C I L I O" but
// instead correctly merges per-glyph TextElements into word-level runs.
//
// The PDF is generated by wkhtmltopdf 0.12 / Qt 4.8 and uses CID TrueType fonts
// (Roboto-Medium, Roboto-Bold) with Identity-H encoding.  Each glyph is encoded
// as a separate Tj operator with an explicit Td kern move, resulting in one
// TextElement per character without this fix.
func TestAndreaniShippingLabel_NoPerGlyphSpaces(t *testing.T) {
	const pdfPath = "../../testdata/pdfs/cid-fonts/andreani_shipping_label.pdf"
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Skipf("test PDF not found at %s", pdfPath)
	}

	rd, err := parser.OpenPDF(pdfPath)
	require.NoError(t, err)
	defer func() { _ = rd.Close() }()

	te := NewTextExtractor(rd)
	elements, err := te.ExtractFromPage(0)
	require.NoError(t, err)
	require.NotEmpty(t, elements)

	// Verify that the merged elements contain multi-character words.
	// Before the fix, every element was a single character.
	multiCharCount := 0
	for _, el := range elements {
		if len([]rune(el.Text)) > 1 {
			multiCharCount++
		}
	}
	assert.Greater(t, multiCharCount, 5,
		"at least 5 multi-character TextElements expected after merge; got %d", multiCharCount)
}

func TestAndreaniShippingLabel_DomicilioEstandarMerged(t *testing.T) {
	const pdfPath = "../../testdata/pdfs/cid-fonts/andreani_shipping_label.pdf"
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Skipf("test PDF not found at %s", pdfPath)
	}

	rd, err := parser.OpenPDF(pdfPath)
	require.NoError(t, err)
	defer func() { _ = rd.Close() }()

	te := NewTextExtractor(rd)
	elements, err := te.ExtractFromPage(0)
	require.NoError(t, err)

	// Collect all element texts.
	var allTexts []string
	for _, el := range elements {
		allTexts = append(allTexts, el.Text)
	}

	// "DOMICILIO" must appear as a single element, not as individual letters.
	foundDomicilio := false
	for _, text := range allTexts {
		if text == "DOMICILIO" {
			foundDomicilio = true
			break
		}
	}
	assert.True(t, foundDomicilio,
		"'DOMICILIO' must be a single merged TextElement; got elements: %v", allTexts[:min(20, len(allTexts))])

	// "ESTANDAR" must also appear as a single element.
	foundEstandar := false
	for _, text := range allTexts {
		if text == "ESTANDAR" {
			foundEstandar = true
			break
		}
	}
	assert.True(t, foundEstandar,
		"'ESTANDAR' must be a single merged TextElement")
}

func TestAndreaniShippingLabel_AssembledTextContainsKeyPhrases(t *testing.T) {
	const pdfPath = "../../testdata/pdfs/cid-fonts/andreani_shipping_label.pdf"
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Skipf("test PDF not found at %s", pdfPath)
	}

	rd, err := parser.OpenPDF(pdfPath)
	require.NoError(t, err)
	defer func() { _ = rd.Close() }()

	te := NewTextExtractor(rd)
	elements, err := te.ExtractFromPage(0)
	require.NoError(t, err)

	assembled := AssembleText(elements)

	// These phrases must appear verbatim in the assembled text.
	wantPhrases := []string{
		"DOMICILIO",
		"ESTANDAR",
		"Destinatario:",
		"Remitente:",
	}
	for _, phrase := range wantPhrases {
		assert.True(t, strings.Contains(assembled, phrase),
			"assembled text must contain %q; got:\n%s", phrase, assembled[:min(400, len(assembled))])
	}

	// The broken pattern "D O M I C I L I O" must NOT appear.
	assert.False(t, strings.Contains(assembled, "D O M I C I L I O"),
		"per-glyph spacing pattern must not appear in assembled text")
}

func TestAndreaniShippingLabel_NoConsecutiveSingleLetterWords(t *testing.T) {
	const pdfPath = "../../testdata/pdfs/cid-fonts/andreani_shipping_label.pdf"
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Skipf("test PDF not found at %s", pdfPath)
	}

	rd, err := parser.OpenPDF(pdfPath)
	require.NoError(t, err)
	defer func() { _ = rd.Close() }()

	te := NewTextExtractor(rd)
	elements, err := te.ExtractFromPage(0)
	require.NoError(t, err)

	assembled := AssembleText(elements)

	// Check for consecutive single-letter alphabetic tokens — a sign of
	// per-glyph spacing.  Legitimate single-letter tokens ("N", "/", "1")
	// exist in the document but never appear consecutively in a real-word
	// sequence of 3 or more.
	words := strings.Fields(assembled)
	maxConsecutiveSingleLetters := 0
	consecutiveSingleLetters := 0
	for _, w := range words {
		runes := []rune(w)
		if len(runes) == 1 && runes[0] >= 'A' && runes[0] <= 'Z' {
			consecutiveSingleLetters++
			if consecutiveSingleLetters > maxConsecutiveSingleLetters {
				maxConsecutiveSingleLetters = consecutiveSingleLetters
			}
		} else {
			consecutiveSingleLetters = 0
		}
	}

	assert.Less(t, maxConsecutiveSingleLetters, 3,
		"no 3+ consecutive single-letter uppercase words expected (was %d before fix)",
		maxConsecutiveSingleLetters)
}

// min returns the smaller of a and b.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
