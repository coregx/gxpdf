package gxpdf

import (
	"os"
	"path/filepath"
	"testing"
)

// Fuzz the PDF parse entry with mutated real PDFs, catching panics.
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
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic on OpenFromBytes: %v", r)
			}
		}()
		doc, err := OpenFromBytes(data)
		if err == nil && doc != nil {
			// Exercise a bit of the parsed document too.
			_ = doc.PageCount()
		}
	})
}
