package gxpdf

import (
	"math"
	"path/filepath"
	"testing"
)

func TestFormTextGeometryIsIndependentOfFontSizeThreshold(t *testing.T) {
	document, err := Open(filepath.Join("testdata", "pdfs", "form_text_sizes.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer document.Close()

	elements, err := document.ExtractTextElementsFromPage(1)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct {
		x, y, width, height, fontSize float64
	}{
		"A": {115, 210, 0.6, 1.5, 1.5},
		"B": {115, 270, 1.2, 3, 3},
		"C": {115, 330, 1.206, 3.015, 3.015},
		"D": {115, 390, 7.2, 18, 18},
	}
	if len(elements) != len(want) {
		t.Fatalf("elements = %d, want %d: %#v", len(elements), len(want), elements)
	}
	actual := make(map[string]TextElement, len(elements))
	for _, element := range elements {
		if _, ok := want[element.Text]; !ok {
			t.Errorf("unexpected text element %q", element.Text)
			continue
		}
		if _, duplicate := actual[element.Text]; duplicate {
			t.Errorf("duplicate text element %q", element.Text)
			continue
		}
		actual[element.Text] = element
	}
	for text, expected := range want {
		element, ok := actual[text]
		if !ok {
			t.Errorf("missing text element %q", text)
			continue
		}
		assertNear(t, "X", element.X, expected.x)
		assertNear(t, "Y", element.Y, expected.y)
		assertNear(t, "Width", element.Width, expected.width)
		assertNear(t, "Height", element.Height, expected.height)
		assertNear(t, "FontSize", element.FontSize, expected.fontSize)
	}
}

func assertNear(t *testing.T, field string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.001 {
		t.Errorf("%s = %.6f, want %.6f", field, got, want)
	}
}
