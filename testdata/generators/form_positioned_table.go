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
	"os"
	"path/filepath"

	"github.com/coregx/gxpdf/internal/testutil"
)

func main() {
	path := filepath.Join("testdata", "pdfs", "form_positioned_table.pdf")
	if err := os.WriteFile(path, testutil.FormPositionedTablePDF(), 0o644); err != nil {
		panic(err)
	}
}
