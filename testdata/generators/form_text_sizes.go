//go:build ignore

// Generator for testdata/pdfs/form_text_sizes.pdf.
//
// The fixture places four otherwise-equivalent text runs in a transformed Form
// XObject at font sizes around the former two-unit compatibility threshold.
// The page has /Rotate 90 to prove display metadata does not alter user-space
// bounds.
// Run from the repository root with:
//
//	go run testdata/generators/form_text_sizes.go
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
		"BT /F1 1 Tf 1 0 0 1 10 100 Tm (A) Tj ET",
		"BT /F1 2 Tf 1 0 0 1 10 140 Tm (B) Tj ET",
		"BT /F1 2.01 Tf 1 0 0 1 10 180 Tm (C) Tj ET",
		"BT /F1 12 Tf 1 0 0 1 10 220 Tm (D) Tj ET",
	}, "\n")
	objects := []pdfObject{
		pdfObject("<< /Type /Catalog /Pages 2 0 R >>"),
		pdfObject("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		pdfObject("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Rotate 90 /Resources << /XObject << /Fm1 5 0 R >> >> /Contents 4 0 R >>"),
		streamObject([]byte("q 0.5 0 0 0.5 100 50 cm /Fm1 Do Q"), ""),
		streamObject([]byte(formContent), "/Type /XObject /Subtype /Form /BBox [0 0 612 792] /Matrix [2 0 0 3 10 20] /Resources << /Font << /F1 6 0 R >> >>"),
		pdfObject("<< /Type /Font /Subtype /TrueType /BaseFont /Synthetic /FirstChar 65 /LastChar 68 /Widths [600 600 600 600] >>"),
	}
	path := filepath.Join("testdata", "pdfs", "form_text_sizes.pdf")
	if err := os.WriteFile(path, buildPDF(objects), 0o644); err != nil {
		panic(err)
	}
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
