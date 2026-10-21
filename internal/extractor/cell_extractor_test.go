package extractor

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCellExtractor(t *testing.T) {
	elements := []*TextElement{
		NewTextElement("test", 10, 20, 30, 10, "Arial", 12),
	}

	extractor := NewCellExtractor(elements)
	assert.NotNil(t, extractor)
	assert.Len(t, extractor.textElements, 1)
}

func TestCellExtractor_ExtractCellContent_Empty(t *testing.T) {
	// Empty extractor
	extractor := NewCellExtractor([]*TextElement{})
	bounds := NewRectangle(0, 0, 100, 100)

	content := extractor.ExtractCellContent(bounds)
	assert.Equal(t, "", content)
}

func TestCellExtractor_ExtractCellContent_SingleElement(t *testing.T) {
	elements := []*TextElement{
		NewTextElement("Hello", 10, 10, 30, 10, "Arial", 12),
	}

	extractor := NewCellExtractor(elements)
	bounds := NewRectangle(0, 0, 50, 50)

	content := extractor.ExtractCellContent(bounds)
	assert.Equal(t, "Hello", content)
}

func TestCellExtractor_ExtractCellContent_MultipleElementsOneLine(t *testing.T) {
	// Elements on same line (same Y), should be joined with space
	elements := []*TextElement{
		NewTextElement("Hello", 10, 20, 30, 10, "Arial", 12),
		NewTextElement("World", 50, 20, 30, 10, "Arial", 12),
	}

	extractor := NewCellExtractor(elements)
	bounds := NewRectangle(0, 0, 100, 50)

	content := extractor.ExtractCellContent(bounds)
	assert.Equal(t, "Hello World", content)
}

func TestCellExtractor_ExtractCellContent_MultipleLines(t *testing.T) {
	// Elements on different lines (different Y), should be joined with newline
	// PDF Y increases upward, so line 1 is higher (Y=30), line 2 is lower (Y=10)
	elements := []*TextElement{
		NewTextElement("Line 1", 10, 30, 40, 10, "Arial", 12), // Top line
		NewTextElement("Line 2", 10, 10, 40, 10, "Arial", 12), // Bottom line
	}

	extractor := NewCellExtractor(elements)
	bounds := NewRectangle(0, 0, 100, 50)

	content := extractor.ExtractCellContent(bounds)
	// Should be ordered top to bottom
	assert.Equal(t, "Line 1\nLine 2", content)
}

func TestCellExtractor_ExtractCellContent_OutsideBounds(t *testing.T) {
	elements := []*TextElement{
		NewTextElement("Inside", 10, 10, 30, 10, "Arial", 12),
		NewTextElement("Outside", 200, 200, 30, 10, "Arial", 12),
	}

	extractor := NewCellExtractor(elements)
	bounds := NewRectangle(0, 0, 50, 50)

	content := extractor.ExtractCellContent(bounds)
	// Only "Inside" should be included
	assert.Equal(t, "Inside", content)
}

func TestCellExtractor_ExtractCellContent_AdjacentWords(t *testing.T) {
	// Words that are immediately adjacent (no gap)
	elements := []*TextElement{
		NewTextElement("Hello", 10, 20, 30, 10, "Arial", 12),
		NewTextElement("World", 40, 20, 30, 10, "Arial", 12), // Exactly adjacent (10+30=40)
	}

	extractor := NewCellExtractor(elements)
	bounds := NewRectangle(0, 0, 100, 50)

	content := extractor.ExtractCellContent(bounds)
	// Should not add space between adjacent words
	assert.Equal(t, "HelloWorld", content)
}

func TestCellExtractor_ExtractCellContent_ComplexTable(t *testing.T) {
	// Simulate a complex cell with multiple lines and words
	elements := []*TextElement{
		// Line 1 (Y=50)
		NewTextElement("Product", 10, 50, 40, 10, "Arial", 12),
		NewTextElement("Name:", 55, 50, 30, 10, "Arial", 12),
		// Line 2 (Y=35)
		NewTextElement("Widget", 10, 35, 35, 10, "Arial", 12),
		NewTextElement("Pro", 50, 35, 20, 10, "Arial", 12),
		// Line 3 (Y=20)
		NewTextElement("v2.0", 10, 20, 25, 10, "Arial", 12),
	}

	extractor := NewCellExtractor(elements)
	bounds := NewRectangle(0, 0, 100, 70)

	content := extractor.ExtractCellContent(bounds)
	expected := "Product Name:\nWidget Pro\nv2.0"
	assert.Equal(t, expected, content)
}

func TestCellExtractor_FindElementsInBounds(t *testing.T) {
	elements := []*TextElement{
		NewTextElement("A", 10, 10, 10, 10, "Arial", 12), // Center at (15, 15)
		NewTextElement("B", 50, 50, 10, 10, "Arial", 12), // Center at (55, 55)
		NewTextElement("C", 90, 90, 10, 10, "Arial", 12), // Center at (95, 95) - outside
	}

	extractor := NewCellExtractor(elements)
	bounds := NewRectangle(0, 0, 80, 80)

	found := extractor.FindElementsInBounds(bounds)
	require.Len(t, found, 2)
	assert.Equal(t, "A", found[0].Text)
	assert.Equal(t, "B", found[1].Text)
}

func TestCellExtractor_groupByLine(t *testing.T) {
	elements := []*TextElement{
		NewTextElement("A", 10, 50, 10, 10, "Arial", 12), // Y=50
		NewTextElement("B", 30, 51, 10, 10, "Arial", 12), // Y=51 (same line)
		NewTextElement("C", 10, 30, 10, 10, "Arial", 12), // Y=30 (different line)
		NewTextElement("D", 30, 29, 10, 10, "Arial", 12), // Y=29 (same line as C)
	}

	extractor := NewCellExtractor(elements)
	lines := extractor.groupByLine(elements)

	require.Len(t, lines, 2)

	// Lines should contain correct elements
	// Note: Order of lines is not guaranteed at this point
	var line1, line2 *textLine
	for _, line := range lines {
		if len(line.elements) == 2 {
			if line.elements[0].Text == "A" || line.elements[0].Text == "B" {
				line1 = line
			} else {
				line2 = line
			}
		}
	}

	require.NotNil(t, line1)
	require.NotNil(t, line2)
	assert.Len(t, line1.elements, 2)
	assert.Len(t, line2.elements, 2)
}

func TestCellExtractor_sortLines(t *testing.T) {
	// Create lines in random order
	lines := []*textLine{
		{y: 10, elements: []*TextElement{NewTextElement("Bottom", 10, 10, 10, 10, "Arial", 12)}},
		{y: 50, elements: []*TextElement{NewTextElement("Top", 10, 50, 10, 10, "Arial", 12)}},
		{y: 30, elements: []*TextElement{NewTextElement("Middle", 10, 30, 10, 10, "Arial", 12)}},
	}

	extractor := NewCellExtractor(nil)
	extractor.sortLines(lines)

	// Should be sorted top to bottom (descending Y)
	require.Len(t, lines, 3)
	assert.Equal(t, "Top", lines[0].elements[0].Text)
	assert.Equal(t, "Middle", lines[1].elements[0].Text)
	assert.Equal(t, "Bottom", lines[2].elements[0].Text)
}

func TestCellExtractor_sortLines_WithinLine(t *testing.T) {
	// Elements within a line should be sorted left to right
	lines := []*textLine{
		{
			y: 50,
			elements: []*TextElement{
				NewTextElement("C", 50, 50, 10, 10, "Arial", 12), // X=50
				NewTextElement("A", 10, 50, 10, 10, "Arial", 12), // X=10
				NewTextElement("B", 30, 50, 10, 10, "Arial", 12), // X=30
			},
		},
	}

	extractor := NewCellExtractor(nil)
	extractor.sortLines(lines)

	require.Len(t, lines[0].elements, 3)
	assert.Equal(t, "A", lines[0].elements[0].Text)
	assert.Equal(t, "B", lines[0].elements[1].Text)
	assert.Equal(t, "C", lines[0].elements[2].Text)
}

func TestCellExtractor_buildTextFromLines(t *testing.T) {
	lines := []*textLine{
		{
			y: 50,
			elements: []*TextElement{
				NewTextElement("Hello", 10, 50, 30, 10, "Arial", 12),
				NewTextElement("World", 50, 50, 30, 10, "Arial", 12),
			},
		},
		{
			y: 30,
			elements: []*TextElement{
				NewTextElement("Second", 10, 30, 35, 10, "Arial", 12),
				NewTextElement("Line", 50, 30, 25, 10, "Arial", 12),
			},
		},
	}

	extractor := NewCellExtractor(nil)
	text := extractor.buildTextFromLines(lines)

	expected := "Hello World\nSecond Line"
	assert.Equal(t, expected, text)
}

func TestCellExtractor_AffineReadingDirection(t *testing.T) {
	affineElement := func(text string, x, y, endX, endY, dirX, dirY float64) *TextElement {
		element := NewTextElement(text, minFloat(x, endX), minFloat(y, endY), abs(endX-x)+8, abs(endY-y)+8, "/F1", 10)
		element.preciseWidth = true
		element.flow = &textFlowGeometry{
			originX: x, originY: y, endX: endX, endY: endY,
			directionX: dirX, directionY: dirY, emAdvance: 10, lineHeight: 10,
		}
		return element
	}
	tests := []struct {
		name     string
		elements []*TextElement
		want     string
	}{
		{
			name: "horizontal reflection",
			elements: []*TextElement{
				affineElement("Income", 45, 20, 15, 20, -1, 0),
				affineElement("Net", 100, 20, 80, 20, -1, 0),
			},
			want: "Net Income",
		},
		{
			name: "quarter turn",
			elements: []*TextElement{
				affineElement("Header", 20, 65, 20, 95, 0, 1),
				affineElement("Rotated", 20, 10, 20, 45, 0, 1),
			},
			want: "Rotated Header",
		},
		{
			name: "three quarter turn",
			elements: []*TextElement{
				affineElement("Second", 20, 50, 20, 20, 0, -1),
				affineElement("First", 20, 100, 20, 70, 0, -1),
			},
			want: "First Second",
		},
		{
			name: "half turn",
			elements: []*TextElement{
				affineElement("Line", 45, 20, 15, 20, -1, 0),
				affineElement("Second", 100, 20, 65, 20, -1, 0),
			},
			want: "Second Line",
		},
		{
			name: "shear",
			elements: []*TextElement{
				affineElement("Flow", 60, 35, 90, 50, 0.8944271909999159, 0.4472135954999579),
				affineElement("Skewed", 10, 10, 45, 27.5, 0.8944271909999159, 0.4472135954999579),
			},
			want: "Skewed Flow",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			content := NewCellExtractor(test.elements).ExtractCellContent(NewRectangle(-20, -20, 160, 160))
			assert.Equal(t, test.want, content)
		})
	}
}

func TestCellExtractor_AffineLinesUseSpatialReadingOrder(t *testing.T) {
	affineElement := func(text string, x, y, endX, endY, dirX, dirY, size float64) *TextElement {
		element := NewTextElement(text, minFloat(x, endX), minFloat(y, endY), abs(endX-x)+size, abs(endY-y)+size, "/F1", size)
		element.preciseWidth = true
		element.flow = &textFlowGeometry{
			originX: x, originY: y, endX: endX, endY: endY,
			directionX: dirX, directionY: dirY, emAdvance: size, lineHeight: size,
		}
		return element
	}
	tests := []struct {
		name     string
		elements []*TextElement
		want     string
	}{
		{
			name: "reflected horizontal lines",
			elements: []*TextElement{
				affineElement("Bottom", 100, 20, 60, 20, -1, 0, 10),
				affineElement("Top", 100, 50, 80, 50, -1, 0, 10),
			},
			want: "Top\nBottom",
		},
		{
			name: "quarter turn lines",
			elements: []*TextElement{
				affineElement("Right", 80, 10, 80, 40, 0, 1, 10),
				affineElement("Left", 20, 10, 20, 35, 0, 1, 10),
			},
			want: "Left\nRight",
		},
		{
			name: "three quarter turn lines",
			elements: []*TextElement{
				affineElement("Right", 80, 70, 80, 40, 0, -1, 10),
				affineElement("Left", 20, 70, 20, 45, 0, -1, 10),
			},
			want: "Left\nRight",
		},
		{
			name: "sheared lines",
			elements: []*TextElement{
				affineElement("Lower", 0, 0, 36, 18, 0.8944271909999159, 0.4472135954999579, 10),
				affineElement("Upper", -8.944, 17.889, 27.056, 35.889, 0.8944271909999159, 0.4472135954999579, 10),
			},
			want: "Upper\nLower",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			content := NewCellExtractor(test.elements).ExtractCellContent(NewRectangle(-50, -50, 200, 200))
			assert.Equal(t, test.want, content)
		})
	}
}

func TestCellExtractor_AffineLineSortIsStableForSpatialTies(t *testing.T) {
	first := &TextElement{Text: "First", Width: 10, Height: 10, flow: &textFlowGeometry{originX: 0, directionX: 1}}
	second := &TextElement{Text: "Second", Width: 10, Height: 10, flow: &textFlowGeometry{originX: 0, directionX: 1}}
	lines := []*textLine{
		{affine: true, dirX: 1, elements: []*TextElement{first}},
		{affine: true, dirX: 1, elements: []*TextElement{second}},
	}

	NewCellExtractor(nil).sortLines(lines)
	assert.Same(t, first, lines[0].elements[0])
	assert.Same(t, second, lines[1].elements[0])
}

func TestCellExtractor_MixedAffineOrientationsSortEachCohortSpatially(t *testing.T) {
	affineElement := func(text string, x, y, endX, endY, dirX, dirY, size float64) *TextElement {
		element := NewTextElement(text, minFloat(x, endX), minFloat(y, endY), abs(endX-x)+size, abs(endY-y)+size, "/F1", size)
		element.preciseWidth = true
		element.flow = &textFlowGeometry{
			originX: x, originY: y, endX: endX, endY: endY,
			directionX: dirX, directionY: dirY, emAdvance: size, lineHeight: size,
		}
		return element
	}
	elements := []*TextElement{
		// The rotated annotation is painted first. It must not disable sorting
		// of the bottom-first horizontal cohort.
		affineElement("Annotation", 90, 30, 90, 65, 0, 1, 10),
		affineElement("Bottom", 10, 20, 50, 20, 1, 0, 10),
		affineElement("Top", 10, 80, 30, 80, 1, 0, 10),
	}

	content := NewCellExtractor(elements).ExtractCellContent(NewRectangle(0, 0, 140, 120))
	assert.Equal(t, "Top\nBottom\nAnnotation", content)
}

func TestCellExtractor_MultipleAffineOrientationCohortsHaveDeterministicPageOrder(t *testing.T) {
	affineElement := func(text string, x, y, endX, endY, dirX, dirY, size float64) *TextElement {
		element := NewTextElement(text, minFloat(x, endX), minFloat(y, endY), abs(endX-x)+size, abs(endY-y)+size, "/F1", size)
		element.flow = &textFlowGeometry{
			originX: x, originY: y, endX: endX, endY: endY,
			directionX: dirX, directionY: dirY, emAdvance: size, lineHeight: size,
		}
		return element
	}
	elements := []*TextElement{
		// Every cohort is deliberately painted in reverse spatial order.
		affineElement("VerticalRight", 80, 10, 80, 45, 0, 1, 8),
		affineElement("VerticalLeft", 20, 10, 20, 40, 0, 1, 8),
		affineElement("ReflectedBottom", 100, 80, 50, 80, -1, 0, 8),
		affineElement("ReflectedTop", 100, 110, 60, 110, -1, 0, 8),
		affineElement("ShearedLower", 10, 150, 50, 170, 0.8944271909999159, 0.4472135954999579, 8),
		affineElement("ShearedUpper", 1, 168, 41, 188, 0.8944271909999159, 0.4472135954999579, 8),
	}

	want := "ShearedUpper\nShearedLower\nReflectedTop\nReflectedBottom\nVerticalLeft\nVerticalRight"
	orders := [][]int{
		{0, 1, 2, 3, 4, 5},
		{5, 4, 3, 2, 1, 0},
		{2, 5, 0, 4, 1, 3},
	}
	for _, order := range orders {
		ordered := make([]*TextElement, len(order))
		for index, source := range order {
			ordered[index] = elements[source]
		}
		content := NewCellExtractor(ordered).ExtractCellContent(NewRectangle(-20, 0, 180, 220))
		assert.Equal(t, want, content)
	}
}

func TestCellExtractor_AffineCohortGeometryBreaksOrientationTiesBeforePaintOrder(t *testing.T) {
	horizontal := NewTextElement("Horizontal", 0, 0, 20, 20, "/F1", 10)
	horizontal.flow = &textFlowGeometry{originX: 0, originY: 0, endX: 20, directionX: 1, lineHeight: 10}
	vertical := NewTextElement("Vertical", 0, 0, 20, 20, "/F1", 10)
	vertical.flow = &textFlowGeometry{originX: 0, originY: 0, endY: 20, directionY: 1, lineHeight: 10}

	// Bounds are identical and vertical is painted first. The documented
	// orientation key, rather than content order, provides the deterministic tie.
	content := NewCellExtractor([]*TextElement{vertical, horizontal}).ExtractCellContent(NewRectangle(-10, -10, 50, 50))
	assert.Equal(t, "Horizontal\nVertical", content)
}

func TestCellExtractor_AffineAxialSeamsArePermutationInvariant(t *testing.T) {
	for _, seam := range []float64{0, 45, 90, 135} {
		seam := seam
		t.Run(fmt.Sprintf("%.0f_degrees", seam), func(t *testing.T) {
			angles := []float64{seam - 0.001, seam + 180.001, seam + 0.0005, seam + 179.9995}
			seamRadians := seam * math.Pi / 180
			normalX, normalY := -math.Sin(seamRadians), math.Cos(seamRadians)
			if normalY < 0 || math.Abs(normalY) < 1e-12 && normalX > 0 {
				normalX, normalY = -normalX, -normalY
			}
			linePositions := []float64{80, 60, 40, 20}
			elements := make([]*TextElement, len(angles))
			for index, angle := range angles {
				x := 50 + normalX*linePositions[index]
				y := 20 + normalY*linePositions[index]
				elements[index] = affineCellElementAtAngle(fmt.Sprintf("L%d", index+1), x, y, angle)
			}
			for _, order := range [][]int{{0, 1, 2, 3}, {3, 2, 1, 0}, {1, 3, 0, 2}} {
				ordered := reorderAffineCellElements(elements, order)
				lines := groupByAffineLine(ordered)
				require.Len(t, buildAffineLineCohorts(lines), 1)
				content := NewCellExtractor(ordered).ExtractCellContent(NewRectangle(-100, -100, 300, 300))
				assert.Equal(t, "L1\nL2\nL3\nL4", content)
			}
		})
	}
}

func TestCellExtractor_AffineCohortsHaveBoundedDiameterWithoutSingleLinkChaining(t *testing.T) {
	angles := []float64{0, 185, 10, 195, 20, 205, 30, 215, 40}
	elements := make([]*TextElement, len(angles))
	for index, angle := range angles {
		elements[index] = affineCellElementAtAngle(fmt.Sprintf("A%d", index), 10, float64(100-index*15), angle)
	}
	wantCohorts := []string{"A0,A1", "A2,A3", "A4,A5", "A6,A7", "A8"}
	orders := [][]int{
		{0, 1, 2, 3, 4, 5, 6, 7, 8},
		{8, 7, 6, 5, 4, 3, 2, 1, 0},
		{2, 0, 8, 4, 1, 7, 3, 6, 5},
		{5, 3, 1, 7, 0, 8, 6, 4, 2},
	}
	for offset := 1; offset < len(elements); offset++ {
		order := make([]int, len(elements))
		for index := range order {
			order[index] = (index + offset) % len(elements)
		}
		orders = append(orders, order)
	}
	for _, order := range orders {
		cohorts := buildAffineLineCohorts(groupByAffineLine(reorderAffineCellElements(elements, order)))
		got := make([]string, 0, len(cohorts))
		for _, cohort := range cohorts {
			names := make([]string, 0, len(cohort.entries))
			for _, entry := range cohort.entries {
				names = append(names, entry.line.elements[0].Text)
			}
			sort.Strings(names)
			got = append(got, strings.Join(names, ","))
			span := cohort.entries[len(cohort.entries)-1].unwrappedKey - cohort.entries[0].unwrappedKey
			assert.LessOrEqual(t, span, quantizeAffineAngle(math.Acos(affineOrientationDot)))
		}
		sort.Strings(got)
		assert.Equal(t, wantCohorts, got)
	}
}

func TestCellExtractor_AffineCohortCompatibilityBoundaryIsQuantized(t *testing.T) {
	toleranceDegrees := math.Acos(affineOrientationDot) * 180 / math.Pi
	elements := []*TextElement{
		affineCellElementAtAngle("Base", 10, 60, 0),
		affineCellElementAtAngle("Within", 10, 40, toleranceDegrees-0.000001),
		affineCellElementAtAngle("Outside", 10, 20, toleranceDegrees+0.000001),
	}
	cohorts := buildAffineLineCohorts(groupByAffineLine(elements))
	require.Len(t, cohorts, 2)
	assert.Len(t, cohorts[0].entries, 2)
	assert.Len(t, cohorts[1].entries, 1)
}

func TestCellExtractor_AffineCohortsHandleOppositeAndNearCancelingDirections(t *testing.T) {
	elements := []*TextElement{
		affineCellElementAtAngle("NearZero", 10, 80, 1),
		affineCellElementAtAngle("NearPi", 10, 60, 179),
		affineCellElementAtAngle("NearVerticalA", 10, 40, 89),
		affineCellElementAtAngle("NearVerticalB", 10, 20, 271),
	}
	for _, order := range [][]int{{0, 1, 2, 3}, {3, 1, 0, 2}, {2, 0, 3, 1}} {
		cohorts := buildAffineLineCohorts(groupByAffineLine(reorderAffineCellElements(elements, order)))
		require.Len(t, cohorts, 2)
		for _, cohort := range cohorts {
			require.Len(t, cohort.entries, 2)
			assert.InDelta(t, 1, math.Hypot(cohort.normalX, cohort.normalY), 1e-12)
		}
	}
}

func TestCellExtractor_AffineOrderingUsesQuantizedTotalKeys(t *testing.T) {
	newLine := func(text string, delta float64) *textLine {
		element := NewTextElement(text, 10, 20+delta, 10, 10, "/F1", 10)
		element.flow = &textFlowGeometry{
			originX: 10, originY: 20 + delta, endX: 20, endY: 20 + delta,
			directionX: math.Cos(1e-13), directionY: math.Sin(1e-13), lineHeight: 10,
		}
		return &textLine{
			elements: []*TextElement{element}, affine: true,
			dirX: element.flow.directionX, dirY: element.flow.directionY,
			originX: element.flow.originX, originY: element.flow.originY,
		}
	}

	first := newLine("First", 0)
	ulpEquivalent := newLine("ULP", 0.4e-6)
	lines := []*textLine{first, ulpEquivalent}
	sortAffineLineCohorts(lines)
	assert.Same(t, first, lines[0], "sub-quantum transform differences fall back to paint order")

	aboveQuantum := newLine("Higher", 0.6e-6)
	lines = []*textLine{first, aboveQuantum}
	sortAffineLineCohorts(lines)
	assert.Same(t, aboveQuantum, lines[0], "a distinct quantized position keeps spatial order")
}

func affineCellElementAtAngle(text string, x, y, angleDegrees float64) *TextElement {
	angle := angleDegrees * math.Pi / 180
	directionX, directionY := math.Cos(angle), math.Sin(angle)
	element := NewTextElement(text, x, y, 10, 10, "/F1", 10)
	element.flow = &textFlowGeometry{
		originX: x, originY: y,
		endX: x + directionX*8, endY: y + directionY*8,
		directionX: directionX, directionY: directionY, emAdvance: 10, lineHeight: 10,
	}
	return element
}

func reorderAffineCellElements(elements []*TextElement, order []int) []*TextElement {
	ordered := make([]*TextElement, len(order))
	for index, source := range order {
		ordered[index] = elements[source]
	}
	return ordered
}

func TestCellExtractor_AffineSameBaselineAllowsDifferentRenderedSizes(t *testing.T) {
	label := NewTextElement("Label", 0, 0, 10, 12, "/F1", 12)
	label.preciseWidth = true
	label.flow = &textFlowGeometry{originX: 0, endX: 10, directionX: 1, emAdvance: 12, lineHeight: 12}
	note := NewTextElement("note", 20, 0, 5, 8, "/F1", 8)
	note.preciseWidth = true
	note.flow = &textFlowGeometry{originX: 20, endX: 25, directionX: 1, emAdvance: 16, lineHeight: 8}

	content := NewCellExtractor([]*TextElement{note, label}).ExtractCellContent(NewRectangle(-10, -10, 100, 50))
	assert.Equal(t, "Label note", content)
	assert.False(t, canMerge(label, note), "merge-size compatibility remains independent of baseline grouping")
}

func TestCellExtractor_MixedAffineAndLegacyUsesLegacyOrdering(t *testing.T) {
	affine := NewTextElement("Right", 50, 20, 20, 10, "/F1", 10)
	affine.flow = &textFlowGeometry{originX: 70, originY: 20, endX: 50, endY: 20, directionX: -1, emAdvance: 10, lineHeight: 10}
	legacy := NewTextElement("Left", 10, 20, 20, 10, "/F1", 10)

	content := NewCellExtractor([]*TextElement{affine, legacy}).ExtractCellContent(NewRectangle(0, 0, 100, 50))
	assert.Equal(t, "Left Right", content)
}

func TestCellExtractor_DoesNotDuplicateBoundaryWhitespace(t *testing.T) {
	elements := []*TextElement{
		NewTextElement("A ", 10, 20, 10, 10, "/F1", 10),
		NewTextElement("", 25, 20, 0, 10, "/F1", 10),
		NewTextElement("B", 40, 20, 10, 10, "/F1", 10),
	}
	content := NewCellExtractor(elements).ExtractCellContent(NewRectangle(0, 0, 100, 50))
	assert.Equal(t, "A B", content)
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func TestCellExtractor_calculateAverageFontSize(t *testing.T) {
	tests := []struct {
		name     string
		elements []*TextElement
		expected float64
	}{
		{
			name:     "empty",
			elements: []*TextElement{},
			expected: 12.0, // Default
		},
		{
			name: "single element",
			elements: []*TextElement{
				NewTextElement("test", 0, 0, 10, 10, "Arial", 14),
			},
			expected: 14.0,
		},
		{
			name: "multiple elements",
			elements: []*TextElement{
				NewTextElement("a", 0, 0, 10, 10, "Arial", 10),
				NewTextElement("b", 0, 0, 10, 10, "Arial", 12),
				NewTextElement("c", 0, 0, 10, 10, "Arial", 14),
			},
			expected: 12.0, // (10+12+14)/3
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewCellExtractor(nil)
			result := extractor.calculateAverageFontSize(tt.elements)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAbs(t *testing.T) {
	assert.Equal(t, 5.0, abs(5.0))
	assert.Equal(t, 5.0, abs(-5.0))
	assert.Equal(t, 0.0, abs(0.0))
}

// TestCellExtractor_ExtractCellContent_VTB_MultiLineCells tests VTB-style multi-line cells.
//
// VTB bank statements have cells with multi-line content (2-3 lines per cell).
// Line spacing is ~12-15pt for 10pt font.
//
// Example VTB cell:
//
//	Line 1 (Y=220): "17.06.2025"
//	Line 2 (Y=208): "19:14:57"   (12pt below - should be merged)
//	Line 3 (Y=195): "Описание"   (13pt below - should be merged)
//
// With old threshold (0.5x = 5pt): Lines treated as separate rows ❌
// With new threshold (1.5x = 15pt): Lines correctly merged ✅
//
// See: BUG_REPORT_VTB_TABLE_STRUCTURE.md, ANALYSIS_VTB_TABLE_MULTI_LINE_CELLS.md
func TestCellExtractor_ExtractCellContent_VTB_MultiLineCells(t *testing.T) {
	// Simulate VTB cell with 3 lines (date, time, description)
	// Font size = 10pt, line spacing = 12-13pt
	elements := []*TextElement{
		// Line 1 (top): Date
		NewTextElement("17.06.2025", 20, 220, 50, 10, "Arial", 10), // Y=220
		// Line 2 (middle): Time - 12pt below
		NewTextElement("19:14:57", 20, 208, 45, 10, "Arial", 10), // Y=208 (220-12=208)
		// Line 3 (bottom): Description - 13pt below
		NewTextElement("Описание", 20, 195, 55, 10, "Arial", 10), // Y=195 (208-13=195)
	}

	extractor := NewCellExtractor(elements)
	bounds := NewRectangle(0, 0, 100, 250)

	content := extractor.ExtractCellContent(bounds)

	// All 3 lines should be merged with newlines
	expected := "17.06.2025\n19:14:57\nОписание"
	assert.Equal(t, expected, content, "VTB multi-line cell should merge all lines")

	// Verify groupByLine creates 3 separate lines (12-13pt spacing > 5pt threshold)
	lines := extractor.groupByLine(elements)
	assert.Len(t, lines, 3, "Elements with 12-13pt spacing should be in 3 separate lines")
	// Each line should contain one element
	for i, line := range lines {
		assert.Len(t, line.elements, 1, "Line %d should contain 1 element", i)
	}
}

// TestCellExtractor_ExtractCellContent_AlfaBank_Regression tests Alfa-Bank single-line cells.
//
// Alfa-Bank statements have mostly single-line cells with minimal spacing (2-3pt).
// This test ensures we didn't break Alfa-Bank support with the threshold change.
//
// Previous behavior (0.5x = 5pt): ✅ Worked
// New behavior (1.5x = 15pt): ✅ Should still work
func TestCellExtractor_ExtractCellContent_AlfaBank_Regression(t *testing.T) {
	// Simulate Alfa-Bank cell with elements on same line (Y difference < 3pt)
	elements := []*TextElement{
		NewTextElement("01.02.24", 20, 220, 45, 10, "Arial", 10), // Y=220
		NewTextElement("Операция", 70, 221, 55, 10, "Arial", 10), // Y=221 (1pt diff)
		NewTextElement("1000.00", 130, 220, 50, 10, "Arial", 10), // Y=220 (same)
	}

	extractor := NewCellExtractor(elements)
	bounds := NewRectangle(0, 0, 200, 250)

	content := extractor.ExtractCellContent(bounds)

	// All elements on same line - should be joined with spaces (no newlines)
	expected := "01.02.24 Операция 1000.00"
	assert.Equal(t, expected, content, "Alfa-Bank single-line cell should work as before")

	// Verify groupByLine creates single line group
	lines := extractor.groupByLine(elements)
	assert.Len(t, lines, 1, "Elements with Y diff 0-1pt should be in 1 line")
	assert.Len(t, lines[0].elements, 3, "Line should contain all 3 elements")
}

// TestCellExtractor_groupByLine_ThresholdBehavior tests threshold edge cases.
//
// UPDATED (2025-01): Algorithm uses 0.3x font size threshold (3pt for 10pt font)
// This is optimized for Alfa-Bank tight line spacing.
// Verifies that:
//   - Elements within threshold (< 3pt for 10pt font) are grouped together
//   - Elements beyond threshold (>= 3pt) are treated as separate lines
func TestCellExtractor_groupByLine_ThresholdBehavior(t *testing.T) {
	tests := []struct {
		name          string
		elements      []*TextElement
		expectedLines int
		description   string
	}{
		{
			name: "spacing 2pt - same line",
			elements: []*TextElement{
				NewTextElement("A", 10, 100, 10, 10, "Arial", 10), // Y=100
				NewTextElement("B", 10, 98, 10, 10, "Arial", 10),  // Y=98 (2pt diff)
			},
			expectedLines: 1,
			description:   "2pt < 3pt threshold - should be 1 line",
		},
		{
			name: "spacing 3pt - edge case (separate)",
			elements: []*TextElement{
				NewTextElement("A", 10, 100, 10, 10, "Arial", 10), // Y=100
				NewTextElement("B", 10, 97, 10, 10, "Arial", 10),  // Y=97 (3pt diff)
			},
			expectedLines: 2,
			description:   "3pt >= 3pt threshold - should be 2 lines (edge case)",
		},
		{
			name: "spacing 12pt - different lines (VTB)",
			elements: []*TextElement{
				NewTextElement("A", 10, 100, 10, 10, "Arial", 10), // Y=100
				NewTextElement("B", 10, 88, 10, 10, "Arial", 10),  // Y=88 (12pt diff)
			},
			expectedLines: 2,
			description:   "12pt > 3pt threshold - should be 2 lines (VTB multi-line cell)",
		},
		{
			name: "spacing 20pt - different lines",
			elements: []*TextElement{
				NewTextElement("A", 10, 100, 10, 10, "Arial", 10), // Y=100
				NewTextElement("B", 10, 80, 10, 10, "Arial", 10),  // Y=80 (20pt diff)
			},
			expectedLines: 2,
			description:   "20pt > 3pt threshold - should be 2 lines (different rows)",
		},
		{
			name: "mixed spacing - all separate",
			elements: []*TextElement{
				NewTextElement("A", 10, 100, 10, 10, "Arial", 10), // Y=100
				NewTextElement("B", 10, 88, 10, 10, "Arial", 10),  // Y=88 (12pt - separate)
				NewTextElement("C", 10, 76, 10, 10, "Arial", 10),  // Y=76 (12pt - separate)
				NewTextElement("D", 10, 50, 10, 10, "Arial", 10),  // Y=50 (26pt - separate)
			},
			expectedLines: 4,
			description:   "Elements with 12pt+ spacing should each be separate line (> 3pt threshold)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewCellExtractor(tt.elements)
			lines := extractor.groupByLine(tt.elements)

			assert.Len(t, lines, tt.expectedLines, tt.description)
		})
	}
}
