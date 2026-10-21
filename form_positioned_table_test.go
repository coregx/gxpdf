package gxpdf

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/coregx/gxpdf/internal/extractor"
	"github.com/coregx/gxpdf/internal/tabledetect"
	"github.com/coregx/gxpdf/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var formPositionedTableRows = [][]string{
	{"Income Statement", "", ""},
	{"Line Item", "2023", "2024"},
	{"Revenue", "1200", "1450"},
	{"Cost of Sales", "(400)", "(570)"},
}

func TestFormPositionedTableFixtureIsReproducible(t *testing.T) {
	path := filepath.Join("testdata", "pdfs", "form_positioned_table.pdf")
	committed, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(testutil.FormPositionedTablePDF(), committed),
		"run go run testdata/generators/form_positioned_table.go")
}

func TestFormPositionedTableStreamBoundaries(t *testing.T) {
	document, err := Open(filepath.Join("testdata", "pdfs", "form_positioned_table.pdf"))
	require.NoError(t, err)
	defer document.Close()
	publicElements, err := document.ExtractTextElementsFromPage(1)
	require.NoError(t, err)
	elements := make([]*extractor.TextElement, len(publicElements))
	for index, element := range publicElements {
		elements[index] = extractor.NewTextElement(
			element.Text, element.X, element.Y, element.Width, element.Height, element.FontName, element.FontSize,
		)
	}
	boundaries := tabledetect.NewColumnBoundaryDetector().DetectBoundaries(elements)
	assert.Equal(t, []float64{60, 185, 235, 247}, boundaries)
}

// TestFormPositionedGeometryReachesTableDetection freezes ownership at the
// public boundary. Stream mode owns whitespace column recovery; Auto and
// Lattice use the visible ruling grid. Hybrid currently follows Auto's default
// branch and remains covered here only as a public compatibility alias.
func TestFormPositionedGeometryReachesTableDetection(t *testing.T) {
	methods := []ExtractionMethod{MethodStream, MethodAuto, MethodLattice, MethodHybrid}
	for _, method := range methods {
		t.Run(method.String(), func(t *testing.T) {
			document, err := Open(filepath.Join("testdata", "pdfs", "form_positioned_table.pdf"))
			require.NoError(t, err)
			defer document.Close()

			tables, err := document.ExtractTablesWithOptions(DefaultExtractionOptions().WithMethod(method))
			require.NoError(t, err)
			require.Len(t, tables, 1)
			assert.Equal(t, 3, tables[0].ColumnCount())
			assert.Equal(t, formPositionedTableRows, tables[0].Rows())
		})
	}
}
