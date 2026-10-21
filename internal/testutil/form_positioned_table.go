package testutil

import (
	"fmt"
	"strings"
)

// FormPositionedTablePDF returns the deterministic fixture used to exercise a
// unit-glyph Form table whose terminal text edge is closer than the detector's
// minimum column spacing. The direct-page rulings deliberately remain present
// so public tests can distinguish Lattice ownership from Stream recovery.
func FormPositionedTablePDF() []byte {
	formContent := strings.Join([]string{
		positionedGlyphRunWithAdvance(100, 700, "Income Statement", 3),
		positionedGlyphRun(100, 650, "Line Item"),
		positionedGlyphRun(350, 650, "2023"),
		positionedGlyphRun(450, 650, "2024"),
		positionedGlyphRun(100, 620, "Revenue"),
		positionedGlyphRun(350, 620, "1200"),
		positionedGlyphRun(450, 620, "1450"),
		positionedGlyphRunWithAdvance(100, 590, "Cost of Sales", 4),
		positionedGlyphRun(350, 590, "(400)"),
		positionedGlyphRun(450, 590, "(570)"),
	}, "\n")
	fontWidths := strings.TrimSpace(strings.Repeat("600 ", 95))
	pageContent := strings.Join([]string{
		"% The ruling grid owns Auto/Lattice; Stream must infer the same columns.",
		"50 350 m 270 350 l 270 302 l 50 302 l h S",
		"170 350 m 170 302 l S",
		"220 350 m 220 302 l S",
		"50 332 m 270 332 l S",
		"50 317 m 270 317 l S",
		"q 0.5 0 0 0.5 0 0 cm /Fm1 Do Q",
	}, "\n")
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /XObject << /Fm1 5 0 R >> >> /Contents 4 0 R >>",
		Stream("", pageContent),
		Stream("/Type /XObject /Subtype /Form /BBox [0 0 612 792] /Matrix [1 0 0 1 20 30] /Resources << /Font << /F1 6 0 R >> >>", formContent),
		fmt.Sprintf("<< /Type /Font /Subtype /TrueType /BaseFont /Synthetic /FirstChar 32 /LastChar 126 /Widths [%s] >>", fontWidths),
	}
	return BuildPDFWithOptions(PDFBuildOptions{BinaryComment: true, CompactXRef: true}, objects...)
}

func positionedGlyphRun(x, y int, value string) string {
	return positionedGlyphRunWithAdvance(x, y, value, 6)
}

func positionedGlyphRunWithAdvance(x, y int, value string, advance int) string {
	var content strings.Builder
	for _, glyph := range value {
		fmt.Fprintf(&content, "q 10 0 0 10 %d %d cm BT /F1 1 Tf 1 0 0 1 0 0 Tm (%s) Tj ET Q\n", x, y, escapeFormPDFText(string(glyph)))
		x += advance
	}
	return strings.TrimSpace(content.String())
}

func escapeFormPDFText(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "(", `\(`)
	return strings.ReplaceAll(value, ")", `\)`)
}
