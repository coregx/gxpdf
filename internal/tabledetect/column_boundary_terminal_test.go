package tabledetect

import (
	"math"
	"testing"

	"github.com/coregx/gxpdf/internal/extractor"
	"github.com/stretchr/testify/assert"
)

func TestColumnBoundaryDetectorRecoverTerminalRightBoundary(t *testing.T) {
	tests := []struct {
		name       string
		boundaries []float64
		elements   []*extractor.TextElement
		want       []float64
	}{
		{
			name:       "two value columns",
			boundaries: []float64{0, 100, 200},
			elements:   terminalRecoveryRows([]float64{40, 150}, 205, []float64{100, 80}),
			want:       []float64{0, 100, 200, 225},
		},
		{
			name:       "terminal left edge exactly matches retained boundary",
			boundaries: []float64{0, 100, 200},
			elements:   terminalRecoveryRows([]float64{40, 150}, 200, []float64{100, 80}),
			want:       []float64{0, 100, 200, 220},
		},
		{
			name:       "three value columns",
			boundaries: []float64{0, 80, 140, 200},
			elements:   terminalRecoveryRows([]float64{40, 110, 170}, 205, []float64{100, 80, 60}),
			want:       []float64{0, 80, 140, 200, 225},
		},
		{
			name:       "four value columns and arbitrary non Latin text",
			boundaries: []float64{0, 60, 110, 155, 200},
			elements:   terminalRecoveryRows([]float64{30, 85, 130, 180}, 205, []float64{100, 80}),
			want:       []float64{0, 60, 110, 155, 200, 225},
		},
		{
			name:       "empty interior interval is preserved by append only recovery",
			boundaries: []float64{0, 80, 120, 200},
			elements:   terminalRecoveryRows([]float64{40, 160}, 205, []float64{100, 80}),
			want:       []float64{0, 80, 120, 200, 225},
		},
		{
			name:       "interior column outside candidate rows is preserved",
			boundaries: []float64{0, 80, 120, 200},
			elements: append(
				terminalRecoveryRows([]float64{40, 160}, 205, []float64{100, 80}),
				terminalElement("interior", 90, 60, 10),
			),
			want: []float64{0, 80, 120, 200, 225},
		},
		{
			name:       "one supporting row is insufficient",
			boundaries: []float64{0, 100, 200},
			elements:   terminalRecoveryRows([]float64{40, 150}, 205, []float64{100}),
			want:       []float64{0, 100, 200},
		},
		{
			name:       "right edge alone does not create an empty sliver",
			boundaries: []float64{0, 170, 200},
			elements: []*extractor.TextElement{
				terminalElement("left one", 20, 100, 20), terminalElement("overlap one", 195, 100, 6),
				terminalElement("left two", 20, 80, 20), terminalElement("overlap two", 195, 80, 6),
			},
			want: []float64{0, 170, 200},
		},
		{
			name:       "isolated adjacent table edge is too far away",
			boundaries: []float64{0, 100, 200},
			elements:   terminalRecoveryRows([]float64{40, 150}, 245, []float64{100, 80}),
			want:       []float64{0, 100, 200},
		},
		{
			name:       "distant adjacent table does not obscure unique local recovery",
			boundaries: []float64{0, 100, 200},
			elements: append(
				terminalRecoveryRows([]float64{40, 150}, 205, []float64{100, 80}),
				terminalRecoveryRows([]float64{340, 390}, 420, []float64{60, 40})...,
			),
			want: []float64{0, 100, 200, 225},
		},
		{
			name:       "two supported candidates are ambiguous",
			boundaries: []float64{0, 100, 200},
			elements: []*extractor.TextElement{
				terminalElement("left", 35, 100, 10), terminalElement("first", 201, 100, 19),
				terminalElement("left", 35, 80, 10), terminalElement("first", 201, 80, 19),
				terminalElement("left", 35, 60, 10), terminalElement("second", 201, 60, 27),
				terminalElement("left", 35, 40, 10), terminalElement("second", 201, 40, 27),
			},
			want: []float64{0, 100, 200},
		},
		{
			name:       "supporting rows need a common retained interval",
			boundaries: []float64{0, 100, 200},
			elements: []*extractor.TextElement{
				terminalElement("alpha", 20, 100, 20), terminalElement("right", 205, 100, 20),
				terminalElement("beta", 130, 80, 20), terminalElement("right", 205, 80, 20),
			},
			want: []float64{0, 100, 200},
		},
		{
			name:       "already complete structure stays byte for byte unchanged",
			boundaries: []float64{0, 100, 200, 240},
			elements:   terminalRecoveryRows([]float64{40, 150, 220}, 215, []float64{100, 80}),
			want:       []float64{0, 100, 200, 240},
		},
		{
			name:       "nonfinite geometry fails closed",
			boundaries: []float64{0, 100, 200},
			elements: append(
				terminalRecoveryRows([]float64{40, 150}, 205, []float64{100, 80}),
				terminalElement("invalid", math.NaN(), 60, 10),
			),
			want: []float64{0, 100, 200},
		},
	}

	detector := NewColumnBoundaryDetector()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			orders := [][]*extractor.TextElement{
				test.elements,
				reverseTerminalElements(test.elements),
				interleaveTerminalElements(test.elements),
			}
			for _, elements := range orders {
				got := detector.recoverTerminalRightBoundary(elements, append([]float64(nil), test.boundaries...))
				assert.Equal(t, test.want, got)
				assert.Equal(t, test.boundaries[0], got[0], "recovery must never prepend a boundary")
			}
		})
	}
}

func TestOccupiesRecoveredTerminalInterval(t *testing.T) {
	last := 200.0
	paddedEdge := last + extractor.CellBoundsPadding
	centerBelowPadding := math.Nextafter(paddedEdge, math.Inf(-1))
	centerAbovePadding := math.Nextafter(paddedEdge, math.Inf(1))
	elementAtCenter := func(center, width float64) *extractor.TextElement {
		return terminalElement("value", center-width/2, 100, width)
	}
	tests := []struct {
		name    string
		element *extractor.TextElement
		want    bool
	}{
		{"nil", nil, false},
		{"repeated edge centered left", terminalElement("value", 195, 100, 6), false},
		{"center exactly on boundary", terminalElement("value", 197, 100, 6), false},
		{"center right but owned by preceding padding", terminalElement("value", 199, 100, 4), false},
		{"center exactly on padded edge", elementAtCenter(paddedEdge, 2), false},
		{"center one ulp below padded edge", elementAtCenter(centerBelowPadding, 2), false},
		{"center one ulp beyond padded edge", elementAtCenter(centerAbovePadding, 2), true},
		{"left edge on boundary", terminalElement("value", last, 100, 6), true},
		{"zero width on right", terminalElement("value", math.Nextafter(last, math.Inf(1)), 100, 0), false},
		{"nonfinite boundary", terminalElement("value", last, 100, 6), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			boundary := last
			if test.name == "nonfinite boundary" {
				boundary = math.NaN()
			}
			assert.Equal(t, test.want, occupiesRecoveredTerminalInterval(test.element, boundary))
		})
	}
}

func reverseTerminalElements(elements []*extractor.TextElement) []*extractor.TextElement {
	result := append([]*extractor.TextElement(nil), elements...)
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return result
}

func TestColumnBoundaryDetectorRecoverTerminalRightBoundaryUsesMedianEdge(t *testing.T) {
	detector := NewColumnBoundaryDetector()
	elements := []*extractor.TextElement{
		terminalElement("row one", 20, 100, 30), terminalElement("middle", 140, 100, 20), terminalElement("one", 205, 100, 19),
		terminalElement("row two", 20, 80, 30), terminalElement("middle", 140, 80, 20), terminalElement("two", 205, 80, 21),
		terminalElement("row three", 20, 60, 30), terminalElement("middle", 140, 60, 20), terminalElement("three", 205, 60, 20),
	}
	assert.Equal(t, []float64{0, 100, 200, 225},
		detector.recoverTerminalRightBoundary(elements, []float64{0, 100, 200}))
}

func TestColumnBoundaryDetectorTerminalRecoveryWorkIsBounded(t *testing.T) {
	detector := NewColumnBoundaryDetector()
	boundaries := make([]float64, 101)
	for index := range boundaries {
		boundaries[index] = float64(index * 30)
	}
	elements := make([]*extractor.TextElement, 0, 12_000)
	for row := 0; row < 2_000; row++ {
		y := float64(4_000 - row*2)
		for column := 0; column < 5; column++ {
			elements = append(elements, terminalElement("cell", float64(column*600+10), y, 10))
		}
		// Two well-supported terminal candidates deliberately make recovery
		// ambiguous without multiplying validation work by rows and boundaries.
		elements = append(elements, terminalElement("right-a", 3_000, y, 18+float64(row%2)))
		if row%2 == 0 {
			elements = append(elements, terminalElement("right-b", 3_000, y, 27))
		}
	}

	orders := [][]*extractor.TextElement{
		elements,
		reverseTerminalElements(elements),
		interleaveTerminalElements(elements),
	}
	for _, ordered := range orders {
		work := 0
		got := detector.recoverTerminalRightBoundaryWithWork(ordered, boundaries, &work)
		assert.Equal(t, boundaries, got)
		assert.LessOrEqual(t, work, 40*len(elements)+len(boundaries),
			"validation work must remain O(E log E + E log B) with a bounded candidate window")
	}
}

func terminalRecoveryRows(inRangeCenters []float64, terminalX float64, rows []float64) []*extractor.TextElement {
	result := make([]*extractor.TextElement, 0, len(rows)*(len(inRangeCenters)+1))
	labels := []string{"任意", "Δ", "текст", "row"}
	for rowIndex, y := range rows {
		for column, center := range inRangeCenters {
			result = append(result, terminalElement(labels[(rowIndex+column)%len(labels)], center-5, y, 10))
		}
		result = append(result, terminalElement(labels[rowIndex%len(labels)], terminalX, y, 20))
	}
	return result
}

func terminalElement(text string, x, y, width float64) *extractor.TextElement {
	return extractor.NewTextElement(text, x, y, width, 10, "/F1", 10)
}

func interleaveTerminalElements(elements []*extractor.TextElement) []*extractor.TextElement {
	result := make([]*extractor.TextElement, 0, len(elements))
	for left, right := 0, len(elements)-1; left <= right; left, right = left+1, right-1 {
		result = append(result, elements[left])
		if left != right {
			result = append(result, elements[right])
		}
	}
	return result
}
