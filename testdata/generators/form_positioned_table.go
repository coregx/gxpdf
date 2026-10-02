//go:build ignore

// Generator for testdata/pdfs/form_positioned_table.pdf.
//
// It mimics PDF generators that place unit-sized glyphs at the text origin and
// supply their real page position through nested page/Form/glyph transforms.
// Run from the repository root with:
//
//	go run testdata/generators/form_positioned_table.go
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type pdfObject []byte

func main() {
	formContent := strings.Join([]string{
		positionedGlyphRun(100, 700, "Income Statement"),
		positionedGlyphRun(100, 650, "Line Item"),
		positionedGlyphRun(350, 650, "2023"),
		positionedGlyphRun(450, 650, "2024"),
		positionedGlyphRun(100, 620, "Revenue"),
		positionedGlyphRun(350, 620, "1200"),
		positionedGlyphRun(450, 620, "1450"),
		positionedGlyphRun(100, 590, "Cost of Sales"),
		positionedGlyphRun(350, 590, "(400)"),
		positionedGlyphRun(450, 590, "(570)"),
	}, "\n")
	fontWidths := strings.TrimSpace(strings.Repeat("600 ", 95))
	pageContent := strings.Join([]string{
		// A direct-page grid surrounds the three transformed text columns.
		"50 350 m 270 350 l 270 302 l 50 302 l h S",
		"170 350 m 170 302 l S",
		"220 350 m 220 302 l S",
		"50 332 m 270 332 l S",
		"50 317 m 270 317 l S",
		"q 0.5 0 0 0.5 0 0 cm /Fm1 Do Q",
	}, "\n")
	objects := []pdfObject{
		pdfObject("<< /Type /Catalog /Pages 2 0 R >>"),
		pdfObject("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		pdfObject("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /XObject << /Fm1 5 0 R >> >> /Contents 4 0 R >>"),
		streamObject([]byte(pageContent), ""),
		streamObject([]byte(formContent), "/Type /XObject /Subtype /Form /BBox [0 0 612 792] /Matrix [1 0 0 1 20 30] /Resources << /Font << /F1 6 0 R >> >>"),
		pdfObject(fmt.Sprintf("<< /Type /Font /Subtype /TrueType /BaseFont /Synthetic /FirstChar 32 /LastChar 126 /Widths [%s] >>", fontWidths)),
	}
	path := filepath.Join("testdata", "pdfs", "form_positioned_table.pdf")
	if err := os.WriteFile(path, buildPDF(objects), 0o644); err != nil {
		panic(err)
	}
}

func positionedGlyphRun(x, y int, value string) string {
	var content strings.Builder
	for _, glyph := range value {
		fmt.Fprintf(&content, "q 10 0 0 10 %d %d cm BT /F1 1 Tf 1 0 0 1 0 0 Tm (%s) Tj ET Q\n", x, y, escapePDFText(string(glyph)))
		x += 6
	}
	return strings.TrimSpace(content.String())
}

func streamObject(data []byte, extra string) pdfObject {
	prefix := fmt.Sprintf("<< %s /Length %d >>\nstream\n", extra, len(data))
	result := make([]byte, 0, len(prefix)+len(data)+11)
	result = append(result, prefix...)
	result = append(result, data...)
	result = append(result, []byte("\nendstream")...)
	return result
}

func buildPDF(objects []pdfObject) []byte {
	var out bytes.Buffer
	out.WriteString("%PDF-1.7\n%\xE2\xE3\xCF\xD3\n")
	offsets := make([]int, len(objects)+1)
	for index, object := range objects {
		offsets[index+1] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n", index+1)
		out.Write(object)
		out.WriteString("\nendobj\n")
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n", len(objects)+1)
	out.WriteString("0000000000 65535 f\n")
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n\n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return out.Bytes()
}

func escapePDFText(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "(", `\(`)
	return strings.ReplaceAll(value, ")", `\)`)
}
