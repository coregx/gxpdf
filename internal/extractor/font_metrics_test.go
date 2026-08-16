package extractor

import (
	"testing"

	"github.com/coregx/gxpdf/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCompositeFontMetricsClampsHostileNegativeRange(t *testing.T) {
	widths := parser.NewArrayFromSlice([]parser.PdfObject{
		parser.NewInteger(-2_000_000_000),
		parser.NewInteger(2),
		parser.NewInteger(500),
	})
	descendant := parser.NewDictionary()
	descendant.Set("W", widths)
	font := parser.NewDictionary()
	font.Set("DescendantFonts", parser.NewArrayFromSlice([]parser.PdfObject{descendant}))

	metrics := (&TextExtractor{}).loadCompositeFontMetrics(font)
	require.NotNil(t, metrics)
	assert.Equal(t, map[uint16]float64{0: 500, 1: 500, 2: 500}, metrics.widths)
}
