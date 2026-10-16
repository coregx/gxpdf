package forms

import (
	"strconv"
	"strings"
	"testing"

	"github.com/coregx/gxpdf/internal/parser"
	"github.com/coregx/gxpdf/internal/testutil"
)

func openFormPDF(t *testing.T, fieldObjects ...string) *parser.Reader {
	t.Helper()
	objects := append([]string{
		"<< /Type /Catalog /Pages 2 0 R /AcroForm << /Fields [4 0 R] >> >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>",
	}, fieldObjects...)
	r, err := parser.OpenPDFFromBytes(testutil.BuildPDF(objects...))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

// TestFieldTree_Hostile covers field hierarchies that previously recursed
// without bound (fatal stack overflow or out-of-memory).
func TestFieldTree_Hostile(t *testing.T) {
	tests := []struct {
		name       string
		fields     []string // objects 4, 5, … ; object 4 is the only /Fields entry
		wantFields []string // fully qualified names, in order
	}{
		{
			name:       "field lists itself as a kid",
			fields:     []string{"<< /T (a) /FT /Tx /Kids [4 0 R] >>"},
			wantFields: []string{"a"},
		},
		{
			name: "parent and kid reference each other",
			fields: []string{
				"<< /T (a) /Kids [5 0 R] >>",
				"<< /T (b) /FT /Tx /Parent 4 0 R /Kids [4 0 R] >>",
			},
			wantFields: []string{"a.b"},
		},
		{
			name: "same kid listed twice is visited once",
			fields: []string{
				"<< /T (a) /Kids [5 0 R 5 0 R] >>",
				"<< /T (b) /FT /Tx /Parent 4 0 R >>",
			},
			wantFields: []string{"a.b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdf := openFormPDF(t, tt.fields...)
			fields, err := NewReader(pdf).GetFields()
			if err != nil {
				t.Fatalf("GetFields: %v", err)
			}
			var names []string
			for _, f := range fields {
				names = append(names, f.Name)
			}
			if strings.Join(names, ",") != strings.Join(tt.wantFields, ",") {
				t.Errorf("fields = %v, want %v", names, tt.wantFields)
			}

			// The flattener walks the same hierarchy to locate widgets.
			if _, err := NewFlattener(pdf).findFieldWidget("does.not.exist"); err != nil {
				t.Errorf("findFieldWidget: %v", err)
			}
		})
	}
}

// TestFieldTree_DepthLimit verifies a field chain nested deeper than
// maxFieldDepth is cut off instead of being followed.
func TestFieldTree_DepthLimit(t *testing.T) {
	const chain = maxFieldDepth + 10
	var objects []string
	for i := 0; i < chain; i++ {
		objects = append(objects, "<< /T (n) /Kids ["+strconv.Itoa(i+5)+" 0 R] >>")
	}
	objects = append(objects, "<< /T (leaf) /FT /Tx >>")
	pdf := openFormPDF(t, objects...)

	fields, err := NewReader(pdf).GetFields()
	if err != nil {
		t.Fatalf("GetFields: %v", err)
	}
	for _, f := range fields {
		if strings.HasSuffix(f.Name, ".leaf") {
			t.Fatalf("reached %q below the depth limit", f.Name)
		}
	}
	if found, err := NewFlattener(pdf).findFieldWidget(strings.Repeat("n.", chain) + "leaf"); err != nil || found != nil {
		t.Fatalf("findFieldWidget below the depth limit = %v, %v; want nil, nil", found, err)
	}
}
