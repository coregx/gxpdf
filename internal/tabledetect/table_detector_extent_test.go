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

func TestRowContainsNumericValueRecognizesFinancialFormatsConservatively(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{text: "1,234", want: true},
		{text: "(400)", want: true},
		{text: "−570", want: true},
		{text: "€ 1.450,00", want: true},
		{text: "12.5%", want: true},
		{text: "Page 1", want: false},
		{text: "Annual report 2024", want: false},
		{text: "—", want: false},
		{text: "K-68", want: false},
	}

	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			row := []*extractor.TextElement{
				extractor.NewTextElement(test.text, 20, 80, 40, 10, "F1", 10),
			}
			assert.Equal(t, test.want, rowContainsNumericValue(row))
		})
	}
}

func TestElementsWithinColumnExtentHandlesPageLabels(t *testing.T) {
	tableLabel := extractor.NewTextElement("Revenue", 20, 100, 50, 10, "F1", 10)
	tableValue := extractor.NewTextElement("1450", 376, 100, 24, 10, "F1", 10)
	footerLabel := extractor.NewTextElement("Annual report", 20, 80, 70, 10, "F1", 10)
	pageNumber := extractor.NewTextElement("Page 1 of 12", 190, 80, 60, 10, "F1", 10)
	structuredPageLabel := extractor.NewTextElement("Page 1", 20, 80, 40, 10, "F1", 10)
	structuredPageValue := extractor.NewTextElement("125", 376, 80, 18, 10, "F1", 10)

	tests := []struct {
		name     string
		elements []*extractor.TextElement
		want     []*extractor.TextElement
	}{
		{
			name:     "excludes whole page-number footer",
			elements: []*extractor.TextElement{tableLabel, tableValue, footerLabel, pageNumber},
			want:     []*extractor.TextElement{tableLabel, tableValue},
		},
		{
			name:     "keeps structured row whose label starts with Page",
			elements: []*extractor.TextElement{structuredPageLabel, structuredPageValue},
			want:     []*extractor.TextElement{structuredPageLabel, structuredPageValue},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := elementsWithinColumnExtent(test.elements, []float64{20, 170, 376, 400})
			assert.Equal(t, test.want, got)
		})
	}
}

func TestStreamTableKeepsSparseTerminalValueAfterSectionGap(t *testing.T) {
	elements := []*extractor.TextElement{
		extractor.NewTextElement("Line Item", 20, 120, 60, 10, "F1", 10),
		extractor.NewTextElement("2023", 176, 120, 24, 10, "F1", 10),
		extractor.NewTextElement("Revenue", 20, 100, 50, 10, "F1", 10),
		extractor.NewTextElement("1200", 176, 100, 24, 10, "F1", 10),
		extractor.NewTextElement("Cost", 20, 80, 30, 10, "F1", 10),
		extractor.NewTextElement("(400)", 170, 80, 30, 10, "F1", 10),
		extractor.NewTextElement("Other", 20, 40, 30, 10, "F1", 10),
		extractor.NewTextElement("900", 382, 40, 18, 10, "F1", 10),
	}

	regions, err := NewDefaultTableDetector().DetectTablesStream(elements)
	assert.NoError(t, err)
	if assert.Len(t, regions, 1) {
		table, extractErr := NewTableExtractor(elements).ExtractTable(regions[0])
		assert.NoError(t, extractErr)
		found := false
		for row := range table.RowCount {
			for column := range table.ColCount {
				if cell := table.GetCell(row, column); cell != nil && cell.Text == "900" {
					found = true
				}
			}
		}
		assert.True(t, found)
	}
}
