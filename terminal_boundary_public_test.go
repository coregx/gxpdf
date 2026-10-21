package gxpdf

import (
	"testing"

	"github.com/coregx/gxpdf/internal/extractor"
	"github.com/coregx/gxpdf/internal/tabledetect"
	"github.com/coregx/gxpdf/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStreamExtractionDoesNotCreateEmptyTerminalSliver exercises the public
// document path for text whose right edges cross the last retained boundary
// while CellExtractor's padding assigns their centers to the preceding cell.
func TestStreamExtractionDoesNotCreateEmptyTerminalSliver(t *testing.T) {
	content := `BT
/F1 10 Tf
1 0 0 1 20 140 Tm (A) Tj
1 0 0 1 170 140 Tm (00000) Tj
/F2 6.666666666666667 Tf
1 0 0 1 199 140 Tm (i) Tj
/F1 10 Tf
1 0 0 1 20 110 Tm (B) Tj
1 0 0 1 170 110 Tm (00000) Tj
/F2 6.666666666666667 Tf
1 0 0 1 199 110 Tm (i) Tj
ET`
	pdf := testutil.BuildPDF(
		`<< /Type /Catalog /Pages 2 0 R >>`,
		`<< /Type /Pages /Kids [3 0 R] /Count 1 >>`,
		`<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 200]
           /Resources << /Font << /F1 4 0 R /F2 6 0 R >> >> /Contents 5 0 R >>`,
		`<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>`,
		testutil.Stream("", content),
		`<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>`,
	)

	document, err := OpenFromBytes(pdf)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, document.Close()) })
	publicElements, err := document.ExtractTextElementsFromPage(1)
	require.NoError(t, err)
	var crossingElements int
	for _, element := range publicElements {
		if element.Text == "i" {
			crossingElements++
			assert.InDelta(t, 199, element.X, 1e-12)
			assert.InDelta(t, 4, element.Width, 1e-12)
			assert.InDelta(t, 201, element.X+element.Width/2, 1e-12)
		}
	}
	assert.Equal(t, 2, crossingElements, "fixture must exercise both padded-boundary rows")
	elements := make([]*extractor.TextElement, len(publicElements))
	for index, element := range publicElements {
		elements[index] = extractor.NewTextElement(
			element.Text, element.X, element.Y, element.Width, element.Height, element.FontName, element.FontSize,
		)
	}
	assert.Equal(t, []float64{20, 170, 200},
		tabledetect.NewColumnBoundaryDetector().DetectBoundaries(elements))

	tables, err := document.ExtractTablesWithOptions(
		DefaultExtractionOptions().WithMethod(MethodStream),
	)
	require.NoError(t, err)
	require.Len(t, tables, 1)
	assert.Equal(t, 2, tables[0].ColumnCount(), "crossing right edges must not create an empty terminal sliver")
	assert.Equal(t, [][]string{{"A", "00000i"}, {"B", "00000i"}}, tables[0].Rows())
}
