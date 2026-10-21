package extractor

import (
	"fmt"
	"math"
	"testing"

	"github.com/coregx/gxpdf/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAffineTextExtractor(matrix Matrix) *TextExtractor {
	extractor := NewTextExtractor(nil)
	extractor.ctm = matrix
	extractor.textState.SetFont("F1", 10)
	extractor.fontMetrics["F1"] = &fontMetrics{
		widths:       map[uint16]float64{32: 250, 65: 600, 66: 600},
		defaultWidth: 600,
		precise:      true,
	}
	return extractor
}

func TestAffineTextRunAssemblyIsInvariantAcrossPageTransforms(t *testing.T) {
	tests := []struct {
		name   string
		matrix Matrix
	}{
		{name: "scaled", matrix: NewMatrix(2, 0, 0, 3, 10, 20)},
		{name: "rotated", matrix: Rotation(math.Pi / 2).Multiply(Translation(10, 20))},
		{name: "reflected", matrix: NewMatrix(-2, 0, 0, 3, 100, 20)},
		{name: "skewed", matrix: NewMatrix(1, 0.8, -0.45, 1, 10, 20)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			extractor := newAffineTextExtractor(test.matrix)
			extractor.textState.Rise = 2
			extractor.addTextBytes([]byte("A"))
			extractor.addTextBytes([]byte("B"))

			merged := mergeAdjacentElements(extractor.elements)
			require.Len(t, merged, 1)
			assert.Equal(t, "AB", merged[0].Text)
			assert.Equal(t, "AB", AssembleText(extractor.elements))
			verticalX := test.matrix.C * 10
			verticalY := test.matrix.D * 10
			assert.InDelta(t, math.Hypot(verticalX, verticalY), merged[0].FontSize, 0.001)
		})
	}
}

func TestAffineTextSpacingAndTJUseBaselineDisplacement(t *testing.T) {
	matrix := NewMatrix(0, 2, -3, 0.5, 100, 200)

	t.Run("character spacing separates painted runs", func(t *testing.T) {
		extractor := newAffineTextExtractor(matrix)
		extractor.textState.CharSpace = 5
		extractor.addTextBytes([]byte("A"))
		extractor.addTextBytes([]byte("B"))
		require.Len(t, mergeAdjacentElements(extractor.elements), 2)
		assert.Equal(t, "A B", AssembleText(extractor.elements))
	})

	t.Run("negative TJ value advances and positive value retreats", func(t *testing.T) {
		forward := newAffineTextExtractor(matrix)
		forward.processTextArray(parser.NewArrayFromSlice([]parser.PdfObject{
			parser.NewString("A"), parser.NewInteger(-500), parser.NewString("B"),
		}))
		require.Len(t, mergeAdjacentElements(forward.elements), 2)
		assert.Equal(t, "A B", AssembleText(forward.elements))

		retreat := newAffineTextExtractor(matrix)
		retreat.processTextArray(parser.NewArrayFromSlice([]parser.PdfObject{
			parser.NewString("A"), parser.NewInteger(200), parser.NewString("B"),
		}))
		require.Len(t, mergeAdjacentElements(retreat.elements), 1)
		assert.Equal(t, "AB", AssembleText(retreat.elements))
	})

	t.Run("horizontal scale changes displacement and not vertical em", func(t *testing.T) {
		extractor := newAffineTextExtractor(matrix)
		extractor.textState.HorizScale = 50
		extractor.textState.WordSpace = 4
		extractor.addTextBytes([]byte("A "))
		require.Len(t, extractor.elements, 1)
		assert.InDelta(t, math.Hypot(matrix.C*10, matrix.D*10), extractor.elements[0].FontSize, 0.001)
		// The trailing word space advances the next origin but is not part of
		// this run's painted page-space envelope.
		paintedEndX, paintedEndY := extractor.elements[0].flow.endX, extractor.elements[0].flow.endY
		nextOriginX, nextOriginY := extractor.ctm.Multiply(extractor.textState.Tm).Transform(0, 0)
		assert.Greater(t, math.Hypot(nextOriginX-paintedEndX, nextOriginY-paintedEndY), 0.0)
	})

	t.Run("negative horizontal scale keeps reflected reading flow", func(t *testing.T) {
		extractor := newAffineTextExtractor(matrix)
		extractor.textState.HorizScale = -100
		extractor.addTextBytes([]byte("A"))
		extractor.addTextBytes([]byte("B"))
		merged := mergeAdjacentElements(extractor.elements)
		require.Len(t, merged, 1)
		assert.Equal(t, "AB", merged[0].Text)
		assert.Equal(t, "AB", AssembleText(extractor.elements))
	})
}

func TestAffineTextDifferentRiseProducesDistinctBaselines(t *testing.T) {
	extractor := newAffineTextExtractor(NewMatrix(1, 0.75, -0.5, 1, 20, 30))
	extractor.addTextBytes([]byte("A"))
	extractor.textState.Rise = 20
	extractor.addTextBytes([]byte("B"))
	require.Len(t, mergeAdjacentElements(extractor.elements), 2)
	assert.Equal(t, "A\nB", AssembleText(extractor.elements))
}

func TestAffineTextRelationUsesPerpendicularBaselineDistance(t *testing.T) {
	base := NewTextElement("Label", 0, 0, 20, 12, "/F1", 12)
	base.flow = &textFlowGeometry{
		originX: 0, originY: 0, endX: 16, endY: 12,
		directionX: 0.8, directionY: 0.6, emAdvance: 12, lineHeight: 12,
	}
	annotation := NewTextElement("note", 40, 0, 12, 8, "/F1", 8)
	annotation.flow = &textFlowGeometry{
		originX: 32, originY: 24, endX: 41.6, endY: 31.2,
		directionX: 0.8, directionY: 0.6, emAdvance: 16, lineHeight: 8,
	}

	sameLine, forward, _, _, ok := affineTextRelation(base, annotation)
	assert.True(t, ok)
	assert.True(t, sameLine, "font size and horizontal scale do not define a baseline")
	assert.True(t, forward)
	assert.Equal(t, "Label note", AssembleText([]*TextElement{base, annotation}))

	annotation.flow.originX, annotation.flow.originY = 29.6, 27.2 // Four units along the baseline normal.
	sameLine, _, _, _, _ = affineTextRelation(base, annotation)
	assert.True(t, sameLine)

	annotation.flow.originX, annotation.flow.originY = 27.2, 30.4 // Eight units along the baseline normal.
	sameLine, _, _, _, _ = affineTextRelation(base, annotation)
	assert.False(t, sameLine)
}

func TestAffineEquivalentTransformDecompositionsUseQuantizedPaintOrderTie(t *testing.T) {
	direct := newAffineTextExtractor(Rotation(0.5))
	direct.textState.Tm = Translation(17, 23)
	direct.addTextBytes([]byte("D"))
	nested := newAffineTextExtractor(Rotation(0.3).Multiply(Rotation(0.2)))
	nested.textState.Tm = Translation(17, 23)
	nested.addTextBytes([]byte("N"))
	require.Len(t, direct.elements, 1)
	require.Len(t, nested.elements, 1)
	assert.Equal(t, quantizeAffinePosition(direct.elements[0].flow.originY),
		quantizeAffinePosition(nested.elements[0].flow.originY))

	// The two mathematically equivalent paths can differ by floating-point
	// ulps, depending on whether the platform fuses multiply-add (for example,
	// arm64 does and amd64 does not). Force a one-ulp difference so the tie is exercised on
	// every platform. Quantized projected geometry treats it as a tie and
	// leaves paint order as the final, stable key.
	nested.elements[0].flow.originY = math.Nextafter(direct.elements[0].flow.originY, math.Inf(1))
	require.NotEqual(t, direct.elements[0].flow.originY, nested.elements[0].flow.originY)
	require.Equal(t, quantizeAffinePosition(direct.elements[0].flow.originY),
		quantizeAffinePosition(nested.elements[0].flow.originY))
	assert.Equal(t, "DN", NewCellExtractor([]*TextElement{direct.elements[0], nested.elements[0]}).
		ExtractCellContent(NewRectangle(-100, -100, 200, 200)))
	assert.Equal(t, "ND", NewCellExtractor([]*TextElement{nested.elements[0], direct.elements[0]}).
		ExtractCellContent(NewRectangle(-100, -100, 200, 200)))
}

func TestAssembleTextRecognizesUnicodeAndASCIILineBoundaries(t *testing.T) {
	for _, boundary := range []string{"\n", "\r", "\u0085", "\u2028", "\u2029"} {
		t.Run("trailing_"+fmt.Sprintf("%U", []rune(boundary)[0]), func(t *testing.T) {
			elements := []*TextElement{
				NewTextElement("A"+boundary, 0, 20, 10, 10, "/F1", 10),
				NewTextElement("B", 0, 0, 10, 10, "/F1", 10),
			}
			assert.Equal(t, "A"+boundary+"B", AssembleText(elements))
		})
		t.Run("leading_"+fmt.Sprintf("%U", []rune(boundary)[0]), func(t *testing.T) {
			elements := []*TextElement{
				NewTextElement("A", 0, 20, 10, 10, "/F1", 10),
				NewTextElement(boundary+"B", 0, 0, 10, 10, "/F1", 10),
			}
			assert.Equal(t, "A"+boundary+"B", AssembleText(elements))
		})
	}
	for _, spacingControl := range []string{"\t", "\v", "\f"} {
		t.Run("non_line_control_"+fmt.Sprintf("%U", []rune(spacingControl)[0]), func(t *testing.T) {
			trailing := []*TextElement{
				NewTextElement("A"+spacingControl, 0, 20, 10, 10, "/F1", 10),
				NewTextElement("B", 0, 0, 10, 10, "/F1", 10),
			}
			assert.Equal(t, "A"+spacingControl+"\nB", AssembleText(trailing))

			leading := []*TextElement{
				NewTextElement("A", 0, 20, 10, 10, "/F1", 10),
				NewTextElement(spacingControl+"B", 0, 0, 10, 10, "/F1", 10),
			}
			assert.Equal(t, "A\n"+spacingControl+"B", AssembleText(leading))
		})
	}
}

func TestGraphicsAndVectorCMUseCanonicalNonCommutingComposition(t *testing.T) {
	outer := NewMatrix(2, 0.5, -0.25, 3, 10, 20)
	inner := NewMatrix(0, 1, -1, 0, 5, 7)
	point := [2]float64{4, 6}
	innerX, innerY := inner.Transform(point[0], point[1])
	wantX, wantY := outer.Transform(innerX, innerY)
	cmOperands := func(matrix Matrix) []parser.PdfObject {
		return []parser.PdfObject{
			parser.NewReal(matrix.A), parser.NewReal(matrix.B), parser.NewReal(matrix.C),
			parser.NewReal(matrix.D), parser.NewReal(matrix.E), parser.NewReal(matrix.F),
		}
	}

	graphics := NewGraphicsParser(nil)
	graphics.processOperator(&Operator{Name: "cm", Operands: cmOperands(outer)})
	graphics.processOperator(&Operator{Name: "cm", Operands: cmOperands(inner)})
	got := graphics.transformPoint(point[0], point[1])
	assert.InDelta(t, wantX, got.X, 1e-12)
	assert.InDelta(t, wantY, got.Y, 1e-12)

	vector := NewVectorParser(nil)
	vector.processVectorOperator(&Operator{Name: "cm", Operands: cmOperands(outer)})
	vector.processVectorOperator(&Operator{Name: "cm", Operands: cmOperands(inner)})
	vector.processVectorOperator(&Operator{Name: "m", Operands: []parser.PdfObject{
		parser.NewReal(point[0]), parser.NewReal(point[1]),
	}})
	require.Len(t, vector.curPath, 1)
	assert.InDelta(t, wantX, vector.curPath[0].coords[0], 1e-12)
	assert.InDelta(t, wantY, vector.curPath[0].coords[1], 1e-12)
}
