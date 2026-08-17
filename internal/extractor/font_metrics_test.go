package extractor

import (
	"testing"

	"github.com/coregx/gxpdf/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCompositeFontMetricsBoundsWidthDefinitions(t *testing.T) {
	tests := []struct {
		name         string
		widths       []parser.PdfObject
		defaultWidth int64
		want         map[uint16]float64
	}{
		{
			name: "hostile negative range",
			widths: []parser.PdfObject{
				parser.NewInteger(-2_000_000_000), parser.NewInteger(2), parser.NewInteger(500),
			},
			defaultWidth: 750,
			want:         map[uint16]float64{0: 500, 1: 500, 2: 500},
		},
		{
			name: "hostile upper range",
			widths: []parser.PdfObject{
				parser.NewInteger(65_534), parser.NewInteger(2_000_000_000), parser.NewInteger(600),
			},
			defaultWidth: 800,
			want:         map[uint16]float64{65_534: 600, 65_535: 600},
		},
		{
			name: "range outside cid space",
			widths: []parser.PdfObject{
				parser.NewInteger(-20), parser.NewInteger(-10), parser.NewInteger(400),
			},
			defaultWidth: 900,
			want:         map[uint16]float64{},
		},
		{
			name: "explicit widths crossing zero",
			widths: []parser.PdfObject{
				parser.NewInteger(-1), parser.NewArrayFromSlice([]parser.PdfObject{
					parser.NewInteger(100), parser.NewInteger(200), parser.NewInteger(300),
				}),
			},
			defaultWidth: 1_000,
			want:         map[uint16]float64{0: 200, 1: 300},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			descendant := parser.NewDictionary()
			descendant.Set("DW", parser.NewInteger(test.defaultWidth))
			descendant.Set("W", parser.NewArrayFromSlice(test.widths))
			font := parser.NewDictionary()
			font.Set("DescendantFonts", parser.NewArrayFromSlice([]parser.PdfObject{descendant}))

			metrics := (&TextExtractor{}).loadCompositeFontMetrics(font)
			require.NotNil(t, metrics)
			assert.Equal(t, float64(test.defaultWidth), metrics.defaultWidth)
			assert.Equal(t, test.want, metrics.widths)
		})
	}
}
