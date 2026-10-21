package gxpdf

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/coregx/gxpdf/internal/testutil"
)

const (
	geometryFixture = "testdata/pdfs/transformed_text_geometry.pdf"
	latticeFixture  = "testdata/pdfs/transformed_text_lattice.pdf"
)

func TestTransformedTextDirectAndIdentityFormHaveOneGeometryModel(t *testing.T) {
	document, err := Open(geometryFixture)
	if err != nil {
		t.Fatal(err)
	}
	defer document.Close()

	elements, err := document.ExtractTextElementsFromPage(1)
	if err != nil {
		t.Fatal(err)
	}
	byText := textElementsByText(t, elements)
	directLabels := []string{"D1", "D2", "D2.01", "D12"}
	formLabels := []string{"F1", "F2", "F2.01", "F12"}
	for index, size := range []float64{1, 2, 2.01, 12} {
		direct := byText[directLabels[index]]
		form := byText[formLabels[index]]
		assertElementGeometry(t, form, direct.X, direct.Y, direct.Width, direct.Height, direct.FontSize)

		y := float64(20 + index*30)
		// Independent corner calculation for [2 .25 100; .5 3 50].
		paintWidth := float64(len(directLabels[index])) * 0.6 * size
		points := [][2]float64{{0, 0}, {paintWidth, 0}, {0, size}, {paintWidth, size}}
		minX, minY := math.Inf(1), math.Inf(1)
		maxX, maxY := math.Inf(-1), math.Inf(-1)
		for _, point := range points {
			x := 2*(point[0]+10) + 0.25*(point[1]+y) + 100
			yValue := 0.5*(point[0]+10) + 3*(point[1]+y) + 50
			minX, minY = math.Min(minX, x), math.Min(minY, yValue)
			maxX, maxY = math.Max(maxX, x), math.Max(maxY, yValue)
		}
		assertElementGeometry(t, direct, minX, minY, maxX-minX, maxY-minY, math.Hypot(0.25*size, 3*size))
	}
}

func TestTransformedTextExplicitCMAndFormMatrixAreEquivalent(t *testing.T) {
	document, err := Open(geometryFixture)
	if err != nil {
		t.Fatal(err)
	}
	defer document.Close()
	elements, err := document.ExtractTextElementsFromPage(2)
	if err != nil {
		t.Fatal(err)
	}
	byText := textElementsByText(t, elements)
	direct, form := byText["CM"], byText["FM"]
	assertElementGeometry(t, form, direct.X, direct.Y, direct.Width, direct.Height, direct.FontSize)
	assertElementGeometry(t, direct, 4, 33, 12, 14.4, 12)
}

func TestTransformedTextNestedRepeatedFormsComposeOnce(t *testing.T) {
	document, err := Open(geometryFixture)
	if err != nil {
		t.Fatal(err)
	}
	defer document.Close()
	elements, err := document.ExtractTextElementsFromPage(3)
	if err != nil {
		t.Fatal(err)
	}
	if len(elements) != 2 {
		t.Fatalf("elements = %d, want 2: %#v", len(elements), elements)
	}
	if elements[0].Text != "Nested" || elements[1].Text != "Nested" {
		t.Fatalf("texts = %q, %q, want two Nested runs", elements[0].Text, elements[1].Text)
	}
	// The repeated invocation changes only the caller translation by (100,100).
	assertNearGeometry(t, "repeat X", elements[1].X-elements[0].X, 100)
	assertNearGeometry(t, "repeat Y", elements[1].Y-elements[0].Y, 100)
}

func TestPageRotateMetadataDoesNotRebaseExtractedGeometry(t *testing.T) {
	document, err := Open(geometryFixture)
	if err != nil {
		t.Fatal(err)
	}
	defer document.Close()

	var baseline []TextElement
	for pageNumber := 4; pageNumber <= 7; pageNumber++ {
		elements, extractErr := document.ExtractTextElementsFromPage(pageNumber)
		if extractErr != nil {
			t.Fatalf("page %d: %v", pageNumber, extractErr)
		}
		if pageNumber == 4 {
			baseline = elements
			continue
		}
		if len(elements) != len(baseline) {
			t.Fatalf("page %d elements = %d, want %d", pageNumber, len(elements), len(baseline))
		}
		for index := range elements {
			assertElementGeometry(t, elements[index], baseline[index].X, baseline[index].Y,
				baseline[index].Width, baseline[index].Height, baseline[index].FontSize)
		}
	}
}

func TestTransformedTextKeepsNonzeroPageOriginAndOutOfBoxCoordinates(t *testing.T) {
	document, err := Open(geometryFixture)
	if err != nil {
		t.Fatal(err)
	}
	defer document.Close()
	elements, err := document.ExtractTextElementsFromPage(8)
	if err != nil {
		t.Fatal(err)
	}
	if len(elements) != 1 {
		t.Fatalf("elements = %d, want 1", len(elements))
	}
	assertElementGeometry(t, elements[0], -75, 850, 43.2, 12, 12)
}

func TestTransformedTextBoundaryWhitespaceIsNotDuplicated(t *testing.T) {
	document, err := Open(geometryFixture)
	if err != nil {
		t.Fatal(err)
	}
	defer document.Close()
	elements, err := document.ExtractTextElementsFromPage(9)
	if err != nil {
		t.Fatal(err)
	}
	if len(elements) != 8 {
		t.Fatalf("elements = %d, want eight merged direct/Form runs: %#v", len(elements), elements)
	}
	var labels, annotations int
	for _, element := range elements {
		switch element.Text {
		case "A ":
			labels++
			assertNearGeometry(t, "12pt label", element.FontSize, 12)
		case "B":
			annotations++
			assertNearGeometry(t, "8pt horizontally-scaled annotation", element.FontSize, 8)
		}
	}
	if labels != 2 || annotations != 2 {
		t.Fatalf("mixed-size direct/Form runs = %d labels, %d annotations; want 2 each", labels, annotations)
	}
	text, err := document.ExtractTextFromPage(9)
	if err != nil {
		t.Fatal(err)
	}
	want := "A B\nC D\nEF\nA B\nC D\nEF"
	if text != want {
		t.Fatalf("assembled direct/Form text = %q, want %q", text, want)
	}
}

func TestTransformedTextLatticeHandoffThroughPageAndDocument(t *testing.T) {
	document, err := Open(latticeFixture)
	if err != nil {
		t.Fatal(err)
	}
	defer document.Close()
	opts := DefaultExtractionOptions().WithMethod(MethodLattice).WithPages(0)
	documentTables, err := document.ExtractTablesWithOptions(opts)
	if err != nil {
		t.Fatal(err)
	}
	pageTables, err := document.Page(0).ExtractTablesWithOptions(DefaultExtractionOptions().WithMethod(MethodLattice))
	if err != nil {
		t.Fatal(err)
	}
	assertLatticeValues := func(label string, tables []*Table) {
		t.Helper()
		if len(tables) != 1 {
			t.Fatalf("%s tables = %d, want 1", label, len(tables))
		}
		if got, want := tables[0].Rows(), [][]string{{"Line Item", "2024"}, {"Revenue", "1200"}}; !equalRows(got, want) {
			t.Fatalf("%s rows = %#v, want %#v", label, got, want)
		}
	}
	assertLatticeValues("document", documentTables)
	assertLatticeValues("page", pageTables)
}

func TestTransformedAffineTextLatticeOrderingThroughPageAndDocument(t *testing.T) {
	document, err := Open(latticeFixture)
	if err != nil {
		t.Fatal(err)
	}
	defer document.Close()
	documentTables, err := document.ExtractTablesWithOptions(
		DefaultExtractionOptions().WithMethod(MethodLattice).WithPages(1),
	)
	if err != nil {
		t.Fatal(err)
	}
	pageTables, err := document.Page(1).ExtractTablesWithOptions(DefaultExtractionOptions().WithMethod(MethodLattice))
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"Gross Profit\nNet Income\nNote", "Rotated Header\nSecond Line"},
		{"EF\nC D\nA B", "EF\nC D\nA B"},
	}
	for label, tables := range map[string][]*Table{"document": documentTables, "page": pageTables} {
		if len(tables) != 1 {
			t.Fatalf("%s tables = %d, want 1", label, len(tables))
		}
		if got := tables[0].Rows(); !equalRows(got, want) {
			t.Fatalf("%s rows = %#v, want %#v", label, got, want)
		}
	}
}

func TestTransformedGeometryFixturesAreReproducible(t *testing.T) {
	generated := map[string][]byte{
		"transformed_text_geometry.pdf": testutil.BuildTransformedTextGeometryPDF(),
		"transformed_text_lattice.pdf":  testutil.BuildTransformedTextLatticePDF(),
	}

	for name, generatedBytes := range generated {
		committed, err := os.ReadFile(filepath.Join("testdata", "pdfs", name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(generatedBytes, []byte("%PDF-1.7\n")) || !bytes.HasSuffix(generatedBytes, []byte("%%EOF\n")) {
			t.Fatalf("%s is not a complete deterministic PDF fixture", name)
		}
		if !bytes.Equal(generatedBytes, committed) {
			t.Fatalf("%s differs from deterministic generator output; run go run testdata/generators/transformed_text_geometry.go", name)
		}
	}
}

func TestIssue79TransformedGeometryKeepsWrappedTitlesWithTheirSections(t *testing.T) {
	document, err := Open("testdata/pdfs/issue79/sample.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer document.Close()
	want := map[string]string{
		"BANGLADESH ECONOMIC STUDIES AND \nCONTEMPORARY ISSUES":             "A",
		"BASIC MECHANICAL ENGINEERING":                                      "A,B,C,D,E,F",
		"BUILDING SERVICES - III : ELECTRICAL INSTALLATIONS \nIN BUILDINGS": "A",
		"CARBOHYDRATE METABOLISM":                                           "A",
		"TEACHING GRAMMAR AND VOCABULARY":                                   "A",
		"STRUCTURE I: BASIC MECHANICS OF SOLIDS":                            "A",
	}
	foundRows := make(map[string]string, len(want))
	for _, page := range []int{0, 1} {
		tables, extractErr := document.ExtractTablesWithOptions(
			DefaultExtractionOptions().WithMethod(MethodLattice).WithPages(page),
		)
		if extractErr != nil {
			t.Fatalf("page %d: %v", page, extractErr)
		}
		if len(tables) == 0 {
			t.Fatalf("page %d: no Issue79 table extracted", page)
		}
		for _, row := range tables[0].Rows() {
			if len(row) > 2 {
				foundRows[row[1]] = row[2]
			}
		}
	}
	for title, sections := range want {
		got, found := foundRows[title]
		if !found {
			t.Errorf("did not find semantic row %q", title)
		} else if got != sections {
			t.Errorf("%q sections = %q, want %q", title, got, sections)
		}
	}
}

func textElementsByText(t *testing.T, elements []TextElement) map[string]TextElement {
	t.Helper()
	result := make(map[string]TextElement, len(elements))
	for _, element := range elements {
		if _, duplicate := result[element.Text]; duplicate {
			t.Fatalf("duplicate text %q", element.Text)
		}
		result[element.Text] = element
	}
	return result
}

func assertElementGeometry(t *testing.T, element TextElement, x, y, width, height, fontSize float64) {
	t.Helper()
	assertNearGeometry(t, element.Text+" X", element.X, x)
	assertNearGeometry(t, element.Text+" Y", element.Y, y)
	assertNearGeometry(t, element.Text+" Width", element.Width, width)
	assertNearGeometry(t, element.Text+" Height", element.Height, height)
	assertNearGeometry(t, element.Text+" FontSize", element.FontSize, fontSize)
}

func assertNearGeometry(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.001 {
		t.Errorf("%s = %.6f, want %.6f", label, got, want)
	}
}

func equalRows(left, right [][]string) bool {
	if len(left) != len(right) {
		return false
	}
	for row := range left {
		if len(left[row]) != len(right[row]) {
			return false
		}
		for column := range left[row] {
			if left[row][column] != right[row][column] {
				return false
			}
		}
	}
	return true
}
