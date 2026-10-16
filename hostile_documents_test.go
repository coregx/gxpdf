package gxpdf

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/coregx/gxpdf/internal/testutil"
)

// TestHostileDocuments exercises crafted or corrupted object graphs through
// the public API. Each case previously crashed the process (fatal stack
// overflow / out of memory, which recover cannot catch) or ran for minutes.
func TestHostileDocuments(t *testing.T) {
	formBomb := func() []string {
		// Eight Forms, each invoking the next ten times: 10^7 invocations.
		const levels = 8
		var xobjects strings.Builder
		for i := 1; i <= levels; i++ {
			fmt.Fprintf(&xobjects, "/X%d %d 0 R ", i, 5+i)
		}
		res := "<< /Font << /F1 4 0 R >> /XObject << " + xobjects.String() + ">> >>"
		objects := []string{
			"<< /Type /Catalog /Pages 2 0 R >>",
			"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources " + res + " /Contents 5 0 R >>",
			"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
			testutil.Stream("", "/X1 Do"),
		}
		for i := 1; i <= levels; i++ {
			content := "BT /F1 12 Tf 72 700 Td (A) Tj ET"
			if i < levels {
				content += strings.Repeat(fmt.Sprintf(" /X%d Do", i+1), 10)
			}
			objects = append(objects, testutil.Stream("/Type /XObject /Subtype /Form /BBox [0 0 612 792] /Resources "+res, content))
		}
		return objects
	}

	tests := []struct {
		name    string
		objects []string
		check   func(t *testing.T, doc *Document)
	}{
		{
			name: "page tree cycle",
			objects: []string{
				"<< /Type /Catalog /Pages 2 0 R >>",
				"<< /Type /Pages /Kids [2 0 R] /Count 1 >>",
			},
			check: func(t *testing.T, doc *Document) {
				if n := doc.PageCount(); n != 0 {
					t.Errorf("PageCount = %d, want 0", n)
				}
				if _, err := doc.ExtractTextFromPage(1); err == nil {
					t.Error("ExtractTextFromPage on a document without pages: want error")
				}
			},
		},
		{
			name: "inflated /Count",
			objects: []string{
				"<< /Type /Catalog /Pages 2 0 R >>",
				"<< /Type /Pages /Kids [3 0 R] /Count 2000000000 >>",
				"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>",
			},
			check: func(t *testing.T, doc *Document) {
				if n := doc.PageCount(); n != 1 {
					t.Errorf("PageCount = %d, want 1", n)
				}
				if _, err := doc.GetImagesWithError(); err != nil {
					t.Errorf("GetImagesWithError: %v", err)
				}
			},
		},
		{
			name: "form field cycle",
			objects: []string{
				"<< /Type /Catalog /Pages 2 0 R /AcroForm << /Fields [4 0 R] >> >>",
				"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
				"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>",
				"<< /T (f) /FT /Tx /Kids [4 0 R] >>",
			},
			check: func(t *testing.T, doc *Document) {
				if _, err := doc.GetFormFields(); err != nil {
					t.Errorf("GetFormFields: %v", err)
				}
			},
		},
		{
			// Spec-conformant, not hostile: kids point back to their parent.
			name: "radio group read twice",
			objects: []string{
				"<< /Type /Catalog /Pages 2 0 R /AcroForm << /Fields [4 0 R] >> >>",
				"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
				"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Annots [5 0 R 6 0 R] >>",
				"<< /FT /Btn /Ff 49152 /T (choice) /Kids [5 0 R 6 0 R] >>",
				"<< /Type /Annot /Subtype /Widget /Parent 4 0 R /P 3 0 R /Rect [50 700 70 720] /AS /Off >>",
				"<< /Type /Annot /Subtype /Widget /Parent 4 0 R /P 3 0 R /Rect [50 670 70 690] /AS /Off >>",
			},
			check: func(t *testing.T, doc *Document) {
				for i := 0; i < 2; i++ {
					if _, err := doc.GetFormFields(); err != nil {
						t.Errorf("GetFormFields call %d: %v", i+1, err)
					}
				}
			},
		},
		{
			name:    "exponential Form XObject graph",
			objects: formBomb(),
			check: func(t *testing.T, doc *Document) {
				_, err := doc.ExtractTextFromPage(1)
				if !errors.Is(err, ErrResourceLimitExceeded) {
					t.Errorf("ExtractTextFromPage error = %v, want ErrResourceLimitExceeded", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := OpenFromBytes(testutil.BuildPDF(tt.objects...))
			if err != nil {
				t.Fatalf("OpenFromBytes: %v", err)
			}
			defer doc.Close()

			done := make(chan struct{})
			go func() {
				defer close(done)
				tt.check(t, doc)
			}()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("did not finish within 10s")
			}
		})
	}
}
