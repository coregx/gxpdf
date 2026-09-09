//go:build ignore

// Generator for testdata/pdfs/form_large_text_table.pdf.
//
// The table text uses an ordinary 12-point font inside a Form XObject. The
// ruling lines live directly in page content, so lattice extraction only works
// when Form and direct-page geometry share page space.
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
		"BT /F1 12 Tf 1 0 0 1 100 650 Tm (Line Item) Tj ET",
		"BT /F1 12 Tf 1 0 0 1 350 650 Tm (2023) Tj ET",
		"BT /F1 12 Tf 1 0 0 1 450 650 Tm (2024) Tj ET",
		"BT /F1 12 Tf 1 0 0 1 100 620 Tm (Revenue) Tj ET",
		"BT /F1 12 Tf 1 0 0 1 350 620 Tm (1200) Tj ET",
		"BT /F1 12 Tf 1 0 0 1 450 620 Tm (1450) Tj ET",
		"BT /F1 12 Tf 1 0 0 1 100 590 Tm (Cost of Sales) Tj ET",
		"BT /F1 12 Tf 1 0 0 1 350 590 Tm ((400)) Tj ET",
		"BT /F1 12 Tf 1 0 0 1 450 590 Tm ((570)) Tj ET",
	}, "\n")
	pageContent := strings.Join([]string{
		"50 640 250 25 re S 300 640 100 25 re S 400 640 120 25 re S",
		"50 610 250 30 re S 300 610 100 30 re S 400 610 120 30 re S",
		"50 580 250 30 re S 300 580 100 30 re S 400 580 120 30 re S",
		"/Fm1 Do",
	}, "\n")
	fontWidths := strings.TrimSpace(strings.Repeat("600 ", 95))
	objects := []pdfObject{
		pdfObject("<< /Type /Catalog /Pages 2 0 R >>"),
		pdfObject("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		pdfObject("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /XObject << /Fm1 5 0 R >> >> /Contents 4 0 R >>"),
		streamObject([]byte(pageContent), ""),
		streamObject([]byte(formContent), "/Type /XObject /Subtype /Form /BBox [0 0 612 792] /Resources << /Font << /F1 6 0 R >> >>"),
		pdfObject(fmt.Sprintf("<< /Type /Font /Subtype /TrueType /BaseFont /Synthetic /FirstChar 32 /LastChar 126 /Widths [%s] >>", fontWidths)),
	}
	path := filepath.Join("testdata", "pdfs", "form_large_text_table.pdf")
	if err := os.WriteFile(path, buildPDF(objects), 0o644); err != nil {
		panic(err)
	}
}

func streamObject(data []byte, extra string) pdfObject {
	prefix := fmt.Sprintf("<< %s /Length %d >>\nstream\n", extra, len(data))
	result := append([]byte(prefix), data...)
	return append(result, []byte("\nendstream")...)
}

func buildPDF(objects []pdfObject) []byte {
	var output bytes.Buffer
	output.WriteString("%PDF-1.7\n%\xE2\xE3\xCF\xD3\n")
	offsets := make([]int, len(objects)+1)
	for index, object := range objects {
		offsets[index+1] = output.Len()
		fmt.Fprintf(&output, "%d 0 obj\n", index+1)
		output.Write(object)
		output.WriteString("\nendobj\n")
	}
	xref := output.Len()
	fmt.Fprintf(&output, "xref\n0 %d\n", len(objects)+1)
	output.WriteString("0000000000 65535 f\n")
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&output, "%010d 00000 n\n", offset)
	}
	fmt.Fprintf(&output, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return output.Bytes()
}
