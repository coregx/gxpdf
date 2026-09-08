package tabledetect

import (
	"fmt"
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
			name:       "snaps clustered terminal edge without adding a phantom column",
			boundaries: []float64{20, 50, 199.5, 299.5},
			elements: []*extractor.TextElement{
				extractor.NewTextElement("Revenue", 20, 100, 50, 10, "F1", 10),
				extractor.NewTextElement("9", 199, 100, 1, 10, "F1", 10),
				extractor.NewTextElement("1000", 296, 100, 4, 10, "F1", 10),
				extractor.NewTextElement("Cost", 20, 80, 30, 10, "F1", 10),
				extractor.NewTextElement("(400)", 195, 80, 5, 10, "F1", 10),
				extractor.NewTextElement("(7)", 297, 80, 3, 10, "F1", 10),
			},
			want: []float64{20, 50, 199.5, 300},
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

func TestDetectBoundariesRightAlignedTables(t *testing.T) {
	tests := []struct {
		name         string
		numericCols  int
		missingValue bool
	}{
		{name: "two value columns", numericCols: 2},
		{name: "three value columns", numericCols: 3, missingValue: true},
		{name: "four value columns", numericCols: 4},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			elements := rightAlignedTableElements(test.numericCols, test.missingValue)
			detector := NewColumnBoundaryDetector()
			boundaries := detector.DetectBoundaries(elements)
			assert.Len(t, boundaries, test.numericCols+2)
			_, maxX := detector.findExtent(elements)
			assert.InDelta(t, maxX, boundaries[len(boundaries)-1], 1e-6)
			for index := 1; index < len(boundaries); index++ {
				assert.Greater(t, boundaries[index]-boundaries[index-1], detector.minGapWidth/2)
			}
		})
	}
}

func rightAlignedTableElements(numericCols int, missingValue bool) []*extractor.TextElement {
	elements := []*extractor.TextElement{
		extractor.NewTextElement("Line Item", 20, 120, 60, 10, "F1", 10),
		extractor.NewTextElement("Revenue", 20, 100, 50, 10, "F1", 10),
		extractor.NewTextElement("Cost", 20, 80, 30, 10, "F1", 10),
	}
	for column := 0; column < numericCols; column++ {
		right := 200.0 + float64(column)*100
		values := []string{fmt.Sprintf("20%02d", column), "9", "(400)"}
		widths := []float64{24, 6, 30}
		for row := range values {
			if missingValue && column == 1 && row == 1 {
				continue
			}
			elements = append(elements, extractor.NewTextElement(
				values[row], right-widths[row], 120-float64(row)*20, widths[row], 10, "F1", 10,
			))
		}
	}
	return elements
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
