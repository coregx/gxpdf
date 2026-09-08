package gxpdf

import (
	"path/filepath"
	"testing"
)

func TestStreamFilterFixturesExtractEquivalentText(t *testing.T) {
	fixtures := []string{
		"unfiltered.pdf",
		"flate.pdf",
		"ascii85.pdf",
		"asciihex.pdf",
		"runlength.pdf",
		"lzw.pdf",
		"ascii85_flate.pdf",
	}
	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			document, err := Open(filepath.Join("testdata", "pdfs", "stream_filters", fixture))
			if err != nil {
				t.Fatal(err)
			}
			defer document.Close()

			elements, err := document.ExtractTextElementsFromPage(1)
			if err != nil {
				t.Fatal(err)
			}
			if len(elements) != 1 {
				t.Fatalf("elements = %d, want 1", len(elements))
			}
			element := elements[0]
			if element.Text != "Filtered fixture" {
				t.Errorf("text = %q, want %q", element.Text, "Filtered fixture")
			}
			if element.X != 72 || element.Y != 700 || element.FontSize != 12 {
				t.Errorf("geometry = (%.2f, %.2f) size %.2f, want (72, 700) size 12", element.X, element.Y, element.FontSize)
			}
		})
	}
}

func TestInvalidStreamFilterFixturesFailClosed(t *testing.T) {
	fixtures := []string{"malformed_ascii85.pdf", "unsupported_filter.pdf"}
	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			document, err := Open(filepath.Join("testdata", "pdfs", "stream_filters", fixture))
			if err != nil {
				t.Fatal(err)
			}
			defer document.Close()
			if _, err := document.ExtractTextElementsFromPage(1); err == nil {
				t.Fatal("ExtractTextElementsFromPage() error = nil, want filter decode error")
			}
		})
	}
}
