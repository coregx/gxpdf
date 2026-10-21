//go:build ignore

// Generator for the transformed_text_geometry.pdf and
// transformed_text_lattice.pdf regression fixtures.
//
// Run from the repository root with:
//
//	go run testdata/generators/transformed_text_geometry.go
package main

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/coregx/gxpdf/internal/testutil"
)

func writeFixture(outputDir, name string, data []byte) {
	path := filepath.Join(outputDir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		panic(err)
	}
}

func main() {
	outputDir := flag.String("output-dir", filepath.Join("testdata", "pdfs"), "directory for generated PDF fixtures")
	flag.Parse()
	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		panic(err)
	}
	writeFixture(*outputDir, "transformed_text_geometry.pdf", testutil.BuildTransformedTextGeometryPDF())
	writeFixture(*outputDir, "transformed_text_lattice.pdf", testutil.BuildTransformedTextLatticePDF())
}
