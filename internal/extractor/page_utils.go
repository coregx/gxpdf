package extractor

import "github.com/coregx/gxpdf/internal/parser"

// ReadPageHeight extracts the page height from the MediaBox dictionary entry.
// Returns 0 if MediaBox is missing or cannot be parsed.
func ReadPageHeight(reader *parser.Reader, pageNum int) float64 {
	page, err := reader.GetPage(pageNum)
	if err != nil {
		return 0
	}

	mb := page.Get("MediaBox")
	if mb == nil {
		return 0
	}

	if ref, ok := mb.(*parser.IndirectReference); ok {
		resolved, err := reader.GetObject(ref.Number)
		if err != nil {
			return 0
		}
		mb = resolved
	}

	arr, ok := mb.(*parser.Array)
	if !ok || arr.Len() < 4 {
		return 0
	}

	// MediaBox = [llx lly urx ury]
	if ury := getNumber(arr.Get(3)); ury != nil {
		if lly := getNumber(arr.Get(1)); lly != nil {
			return *ury - *lly
		}
		return *ury
	}
	return 0
}
