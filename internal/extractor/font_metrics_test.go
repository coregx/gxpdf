package extractor

import (
	"testing"

	"github.com/coregx/gxpdf/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func compositeFontWithWidths(widths []parser.PdfObject, defaultWidth int64) *parser.Dictionary {
	descendant := parser.NewDictionary()
	descendant.Set("DW", parser.NewInteger(defaultWidth))
	descendant.Set("W", parser.NewArrayFromSlice(widths))
	font := parser.NewDictionary()
	font.Set("DescendantFonts", parser.NewArrayFromSlice([]parser.PdfObject{descendant}))
	return font
}

func TestLoadCompositeFontMetricsBoundsWidthDefinitions(t *testing.T) {
	tests := []struct {
		name          string
		widths        []parser.PdfObject
		defaultWidth  int64
		wantWidths    map[uint16]float64
		wantIntervals int
	}{
		{
			name: "hostile negative range",
			widths: []parser.PdfObject{
				parser.NewInteger(-2_000_000_000), parser.NewInteger(2), parser.NewInteger(500),
			},
			defaultWidth:  750,
			wantWidths:    map[uint16]float64{0: 500, 1: 500, 2: 500, 3: 750},
			wantIntervals: 1,
		},
		{
			name: "hostile upper range",
			widths: []parser.PdfObject{
				parser.NewInteger(65_534), parser.NewInteger(2_000_000_000), parser.NewInteger(600),
			},
			defaultWidth:  800,
			wantWidths:    map[uint16]float64{65_533: 800, 65_534: 600, 65_535: 600},
			wantIntervals: 1,
		},
		{
			name: "range outside cid space",
			widths: []parser.PdfObject{
				parser.NewInteger(-20), parser.NewInteger(-10), parser.NewInteger(400),
			},
			defaultWidth:  900,
			wantWidths:    map[uint16]float64{0: 900, 65_535: 900},
			wantIntervals: 0,
		},
		{
			name: "explicit widths crossing zero",
			widths: []parser.PdfObject{
				parser.NewInteger(-1), parser.NewArrayFromSlice([]parser.PdfObject{
					parser.NewInteger(100), parser.NewInteger(200), parser.NewInteger(300),
				}),
			},
			defaultWidth:  1_000,
			wantWidths:    map[uint16]float64{0: 200, 1: 300, 2: 1_000},
			wantIntervals: 2,
		},
		{
			name: "unsorted ranges normalized",
			widths: []parser.PdfObject{
				parser.NewInteger(100), parser.NewInteger(110), parser.NewInteger(400),
				parser.NewInteger(10), parser.NewInteger(20), parser.NewInteger(500),
			},
			defaultWidth:  1_000,
			wantWidths:    map[uint16]float64{9: 1_000, 10: 500, 20: 500, 21: 1_000, 100: 400, 110: 400},
			wantIntervals: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metrics := (&TextExtractor{}).loadCompositeFontMetrics(
				compositeFontWithWidths(test.widths, test.defaultWidth),
			)
			require.NotNil(t, metrics)
			assert.Equal(t, float64(test.defaultWidth), metrics.defaultWidth)
			assert.Len(t, metrics.intervals, test.wantIntervals)
			for cid, want := range test.wantWidths {
				assert.Equal(t, want, metrics.width(cid), "CID %d", cid)
			}
		})
	}
}

func TestCompositeFontFullCIDRangeHasConstantCardinality(t *testing.T) {
	font := compositeFontWithWidths([]parser.PdfObject{
		parser.NewInteger(0), parser.NewInteger(65_535), parser.NewInteger(500),
	}, 1_000)
	metrics := (&TextExtractor{}).loadCompositeFontMetrics(font)
	require.NotNil(t, metrics)
	require.Len(t, metrics.intervals, 1)
	assert.Nil(t, metrics.widths, "a range must not expand into per-CID map entries")
	assert.Equal(t, float64(500), metrics.width(0))
	assert.Equal(t, float64(500), metrics.width(32_768))
	assert.Equal(t, float64(500), metrics.width(65_535))

	fullRangeAllocations := testing.AllocsPerRun(100, func() {
		loaded := (&TextExtractor{}).loadCompositeFontMetrics(font)
		if len(loaded.intervals) != 1 {
			t.Fatalf("interval count = %d, want 1", len(loaded.intervals))
		}
	})
	singleCIDFont := compositeFontWithWidths([]parser.PdfObject{
		parser.NewInteger(0), parser.NewInteger(0), parser.NewInteger(500),
	}, 1_000)
	singleCIDAllocations := testing.AllocsPerRun(100, func() {
		loaded := (&TextExtractor{}).loadCompositeFontMetrics(singleCIDFont)
		if len(loaded.intervals) != 1 {
			t.Fatalf("interval count = %d, want 1", len(loaded.intervals))
		}
	})
	assert.LessOrEqual(t, fullRangeAllocations, singleCIDAllocations+1,
		"full-range parsing must allocate by interval count, not CID count")
}

func TestLoadFontMetricsUsesCompositeWidthsForIdentityEncoding(t *testing.T) {
	font := compositeFontWithWidths([]parser.PdfObject{
		parser.NewInteger(65), parser.NewArrayFromSlice([]parser.PdfObject{parser.NewInteger(1_000)}),
	}, 500)
	font.Set("Subtype", parser.NewName(type0FontSubtype))
	font.Set("Encoding", parser.NewName("Identity-H"))

	metrics := (&TextExtractor{}).loadFontMetrics(font)
	require.NotNil(t, metrics)
	assert.True(t, metrics.precise)
	assert.Equal(t, float64(1_000), metrics.width(65))
}

func TestLoadFontMetricsRejectsNonIdentityCompositeCodeWidths(t *testing.T) {
	font := compositeFontWithWidths([]parser.PdfObject{
		parser.NewInteger(65), parser.NewArrayFromSlice([]parser.PdfObject{parser.NewInteger(1_000)}),
	}, 500)
	font.Set("Subtype", parser.NewName(type0FontSubtype))
	// This Encoding CMap maps character code 1 to CID 65. Reading /W with the
	// raw code would incorrectly use /DW=500 instead of CID 65's width=1000.
	font.Set("Encoding", parser.NewStream(parser.NewDictionary(), []byte(
		"begincmap 1 begincidchar <0001> 65 endcidchar endcmap",
	)))

	extractor := NewTextExtractor(nil)
	assert.Nil(t, extractor.loadFontMetrics(font))

	decoder := NewFontDecoder(nil, "Custom-H", true)
	decoder.isCompositeFont = true
	extractor.textState.SetFont("F1", 10)
	extractor.fontDecoders["F1"] = decoder
	extractor.fontMetrics["F1"] = nil
	advance, width, precise := extractor.measureGlyphBytes([]byte{0, 1})
	assert.False(t, precise)
	assert.InDelta(t, 6, advance, 1e-9)
	assert.InDelta(t, 6, width, 1e-9)
}
