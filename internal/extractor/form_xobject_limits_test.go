package extractor

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/coregx/gxpdf/internal/parser"
	"github.com/coregx/gxpdf/internal/testutil"
)

// formPage builds a one-page PDF whose page content is pageContent and whose
// resources (shared by the page and every Form) define /F1 and the given Forms.
// forms maps a resource name (e.g. "X1") to its content stream.
func formPage(t *testing.T, pageContent string, forms map[string]string, order []string) *parser.Reader {
	t.Helper()
	// Objects: 1 catalog, 2 pages, 3 page, 4 font, 5 page content, 6.. forms.
	var xobjects strings.Builder
	for i, name := range order {
		fmt.Fprintf(&xobjects, "/%s %d 0 R ", name, 6+i)
	}
	resources := fmt.Sprintf("<< /Font << /F1 4 0 R >> /XObject << %s>> >>", xobjects.String())
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources " + resources + " /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		testutil.Stream("", pageContent),
	}
	for _, name := range order {
		objects = append(objects, testutil.Stream("/Type /XObject /Subtype /Form /BBox [0 0 612 792] /Resources "+resources, forms[name]))
	}
	r, err := parser.OpenPDFFromBytes(testutil.BuildPDF(objects...))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func extractWithin(t *testing.T, r *parser.Reader, limit time.Duration) ([]*TextElement, error) {
	t.Helper()
	type result struct {
		elements []*TextElement
		err      error
	}
	done := make(chan result, 1)
	go func() {
		elements, err := NewTextExtractor(r).ExtractFromPage(0)
		done <- result{elements, err}
	}()
	select {
	case res := <-done:
		return res.elements, res.err
	case <-time.After(limit):
		t.Fatalf("extraction did not finish within %v", limit)
		return nil, nil
	}
}

func joinedText(elements []*TextElement) string {
	var sb strings.Builder
	for _, e := range elements {
		sb.WriteString(e.Text)
	}
	return sb.String()
}

// TestFormXObject_CyclesAreNotReentered covers Form graphs that re-enter a Form
// already executing. With a depth limit alone, a 1 KB file whose Form invokes
// itself six times expands to 6^8 ≈ 1.7 million invocations.
func TestFormXObject_CyclesAreNotReentered(t *testing.T) {
	tests := []struct {
		name  string
		forms map[string]string
		order []string
		want  string
	}{
		{
			name:  "self-invocation with fan-out",
			forms: map[string]string{"X": "BT /F1 12 Tf 72 700 Td (A) Tj ET" + strings.Repeat(" /X Do", 10)},
			order: []string{"X"},
			want:  "A",
		},
		{
			name: "two Forms invoking each other",
			forms: map[string]string{
				"X": "BT /F1 12 Tf 72 700 Td (A) Tj ET /Y Do /Y Do",
				"Y": "BT /F1 12 Tf 72 680 Td (B) Tj ET /X Do /X Do",
			},
			order: []string{"X", "Y"},
			want:  "ABB",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := formPage(t, "/X Do", tt.forms, tt.order)
			elements, err := extractWithin(t, r, 5*time.Second)
			if err != nil {
				t.Fatalf("ExtractFromPage: %v", err)
			}
			if got := joinedText(elements); got != tt.want {
				t.Errorf("text = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFormXObject_InvocationBudget covers an acyclic Form graph that expands
// exponentially: each of eight levels invokes the next one ten times.
func TestFormXObject_InvocationBudget(t *testing.T) {
	const levels = maxXObjectDepth
	forms := make(map[string]string, levels)
	order := make([]string, 0, levels)
	for i := 1; i <= levels; i++ {
		name := fmt.Sprintf("X%d", i)
		content := "BT /F1 12 Tf 72 700 Td (A) Tj ET"
		if i < levels {
			content += strings.Repeat(fmt.Sprintf(" /X%d Do", i+1), 10)
		}
		forms[name] = content
		order = append(order, name)
	}
	r := formPage(t, "/X1 Do", forms, order)

	_, err := extractWithin(t, r, 10*time.Second)
	if !errors.Is(err, ErrResourceLimitExceeded) {
		t.Fatalf("ExtractFromPage error = %v, want ErrResourceLimitExceeded", err)
	}
}

// TestFormXObject_RepeatedInvocationsUseCachedContent verifies that a Form
// invoked several times produces its content each time (the parsed-content
// cache must not change results).
func TestFormXObject_RepeatedInvocationsUseCachedContent(t *testing.T) {
	r := formPage(t, "/X Do /X Do /X Do",
		map[string]string{"X": "BT /F1 12 Tf 72 700 Td (A) Tj ET"}, []string{"X"})
	elements, err := extractWithin(t, r, 5*time.Second)
	if err != nil {
		t.Fatalf("ExtractFromPage: %v", err)
	}
	if got := strings.Count(joinedText(elements), "A"); got != 3 {
		t.Errorf("Form content emitted %d times, want 3", got)
	}
}
