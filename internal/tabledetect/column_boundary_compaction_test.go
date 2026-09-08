package tabledetect

import (
	"testing"

	"github.com/coregx/gxpdf/internal/extractor"
	"github.com/stretchr/testify/assert"
)

func TestCompleteAndCompactBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		boundaries []float64
		elements   []*extractor.TextElement
		want       []float64
	}{
		{
			name:       "restores terminal edge and removes whitespace interval",
			boundaries: []float64{60, 99, 185, 235},
			elements: tableElements(
				[]float64{60, 185, 235},
				[][]float64{{39, 15, 15}, {27, 12, 12}, {21, 12, 12}},
			),
			want: []float64{60, 185, 235, 250},
		},
		{
			name:       "keeps a column represented in only one row",
			boundaries: []float64{20, 80, 150, 210},
			elements: []*extractor.TextElement{
				extractor.NewTextElement("label", 20, 100, 50, 10, "F1", 10),
				extractor.NewTextElement("first", 150, 100, 20, 10, "F1", 10),
				extractor.NewTextElement("second", 210, 80, 20, 10, "F1", 10),
			},
			want: []float64{20, 150, 210, 230},
		},
		{
			name:       "preserves already compact boundaries",
			boundaries: []float64{10, 100, 200},
			elements: []*extractor.TextElement{
				extractor.NewTextElement("left", 10, 50, 20, 10, "F1", 10),
				extractor.NewTextElement("right", 120, 50, 20, 10, "F1", 10),
			},
			want: []float64{10, 100, 200},
		},
		{
			name:       "empty input",
			boundaries: nil,
			elements:   nil,
			want:       nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detector := NewColumnBoundaryDetector()
			assert.Equal(t, test.want, detector.completeAndCompactBoundaries(test.boundaries, test.elements))
		})
	}
}

func tableElements(starts []float64, rowWidths [][]float64) []*extractor.TextElement {
	var elements []*extractor.TextElement
	for row, widths := range rowWidths {
		for column, width := range widths {
			elements = append(elements, extractor.NewTextElement(
				"cell", starts[column], 100-float64(row)*15, width, 10, "F1", 10,
			))
		}
	}
	return elements
}
