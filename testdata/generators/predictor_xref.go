//go:build ignore

// Generator for testdata/pdfs/predictor_xref.pdf
//
// This creates a minimal PDF 1.5 with a predictor-encoded xref stream.
// The xref stream uses PNG Up filter (Predictor 12) to compress the
// cross-reference table entries.
//
// Run with: go run predictor_xref.go
package main

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	var pdf bytes.Buffer

	// Header
	pdf.WriteString("%PDF-1.5\n%\xe2\xe3\xcf\xd3\n")

	// Object 1: Catalog
	off1 := pdf.Len()
	pdf.WriteString("1 0 obj\n<</Type/Catalog/Pages 2 0 R>>\nendobj\n")

	// Object 2: Pages
	off2 := pdf.Len()
	pdf.WriteString("2 0 obj\n<</Type/Pages/Kids[3 0 R]/Count 1>>\nendobj\n")

	// Object 3: Page with font resource
	off3 := pdf.Len()
	pdf.WriteString("3 0 obj\n<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]/Contents 4 0 R/Resources<</Font<</F1 5 0 R>>>>>>\nendobj\n")

	// Object 4: Content stream
	off4 := pdf.Len()
	content := []byte("BT /F1 24 Tf 100 700 Td (PNG Predictor Test) Tj ET")
	pdf.WriteString(fmt.Sprintf("4 0 obj\n<</Length %d>>\nstream\n", len(content)))
	pdf.Write(content)
	pdf.WriteString("\nendstream\nendobj\n")

	// Object 5: Font (Helvetica - built-in)
	off5 := pdf.Len()
	pdf.WriteString("5 0 obj\n<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>\nendobj\n")

	// XRef stream at Object 6
	xrefOff := pdf.Len()

	// Build raw xref: 6 entries, /W [1 2 1] = 4 bytes each
	offsets := []int{0, off1, off2, off3, off4, off5}
	var rawXref bytes.Buffer
	for i, off := range offsets {
		if i == 0 {
			rawXref.Write([]byte{0, 0, 0, 0}) // free entry
		} else {
			rawXref.Write([]byte{1, byte(off >> 8), byte(off & 0xFF), 0})
		}
	}

	// Apply PNG Up predictor (filter type 2): store delta from previous row
	var predData bytes.Buffer
	raw := rawXref.Bytes()
	prevRow := []byte{0, 0, 0, 0}
	for i := 0; i < len(raw); i += 4 {
		row := raw[i : i+4]
		predData.WriteByte(2) // Up filter
		for j := 0; j < 4; j++ {
			predData.WriteByte(row[j] - prevRow[j])
		}
		copy(prevRow, row)
	}

	// Compress with zlib
	var compressed bytes.Buffer
	w := zlib.NewWriter(&compressed)
	w.Write(predData.Bytes())
	w.Close()
	compData := compressed.Bytes()

	// Write xref stream object
	pdf.WriteString(fmt.Sprintf("6 0 obj\n<</Type/XRef/Size 6/W[1 2 1]/Root 1 0 R/DecodeParms<</Columns 4/Predictor 12>>/Filter/FlateDecode/Length %d>>\nstream\n", len(compData)))
	pdf.Write(compData)
	pdf.WriteString("\nendstream\nendobj\n")

	// startxref and EOF
	pdf.WriteString(fmt.Sprintf("startxref\n%d\n%%%%EOF\n", xrefOff))

	// Write to testdata/pdfs
	outputPath := filepath.Join("..", "pdfs", "predictor_xref.pdf")
	err := os.WriteFile(outputPath, pdf.Bytes(), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing PDF: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Created %s (%d bytes)\n", outputPath, pdf.Len())
}
