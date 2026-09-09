package tabledetect

import (
	"testing"

	"github.com/coregx/gxpdf/internal/extractor"
	"github.com/stretchr/testify/assert"
)

func TestElementsWithinColumnExtentExcludesPageText(t *testing.T) {
	tableLabel := extractor.NewTextElement("Revenue", 20, 100, 50, 10, "F1", 10)
	tableValue := extractor.NewTextElement("1450", 376, 100, 24, 10, "F1", 10)
	pageNumber := extractor.NewTextElement("Page 1", 700, 20, 30, 10, "F1", 10)

	got := elementsWithinColumnExtent(
		[]*extractor.TextElement{tableLabel, tableValue, pageNumber},
		[]float64{20, 170, 376, 400},
	)

	assert.Equal(t, []*extractor.TextElement{tableLabel, tableValue}, got)
}

func TestSupportedRowAdjacencyUsesLowerMedianGap(t *testing.T) {
	assert.Equal(t, 30.0, supportedRowAdjacency([]float64{80, 100, 200}))
}
