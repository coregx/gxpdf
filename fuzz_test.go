package gxpdf

import (
	"os"
	"path/filepath"
	"testing"
)

// Fuzz the PDF parse entry with mutated real PDFs. A panic fails the run and
// the fuzzing engine records the crasher with its full stack.
func FuzzOpenFromBytes(f *testing.F) {
	// Seed with every PDF under testdata.
	_ = filepath.Walk("testdata", func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Ext(path) == ".pdf" {
			if data, e := os.ReadFile(path); e == nil {
				f.Add(data)
			}
		}
		return nil
	})

	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := OpenFromBytes(data)
		if err == nil && doc != nil {
			// Exercise a bit of the parsed document too. Extracting page text
			// resolves a content stream, which is what reaches
			// parseStreamContent (PageCount alone does not).
			// Pages are 1-based.
			for i := 1; i <= min(doc.PageCount(), 3); i++ {
				_, _ = doc.ExtractTextFromPage(i)
			}
		}
	})
}
