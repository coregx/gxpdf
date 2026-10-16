package parser

import (
	"strconv"
	"testing"

	"github.com/coregx/gxpdf/internal/testutil"
)

func openTestPDF(t *testing.T, objects ...string) *Reader {
	t.Helper()
	r, err := OpenPDFFromBytes(testutil.BuildPDF(objects...))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

const testCatalog = "<< /Type /Catalog /Pages 2 0 R >>"

// TestPageTree_Hostile covers page trees that previously crashed the process
// (unbounded recursion) or made per-page loops run billions of times.
func TestPageTree_Hostile(t *testing.T) {
	tests := []struct {
		name      string
		objects   []string
		wantPages int
	}{
		{
			name: "root lists itself as a kid",
			objects: []string{testCatalog,
				"<< /Type /Pages /Kids [2 0 R] /Count 1 >>"},
			wantPages: 0,
		},
		{
			name: "two intermediate nodes reference each other",
			objects: []string{testCatalog,
				"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
				"<< /Type /Pages /Parent 2 0 R /Kids [4 0 R 2 0 R] /Count 1 >>",
				"<< /Type /Page /Parent 3 0 R >>"},
			wantPages: 1,
		},
		{
			name: "same page listed repeatedly is counted once",
			objects: []string{testCatalog,
				"<< /Type /Pages /Kids [3 0 R 3 0 R 3 0 R] /Count 3 >>",
				"<< /Type /Page /Parent 2 0 R >>"},
			wantPages: 1,
		},
		{
			name: "declared /Count far exceeds real pages",
			objects: []string{testCatalog,
				"<< /Type /Pages /Kids [3 0 R] /Count 2000000000 >>",
				"<< /Type /Page /Parent 2 0 R >>"},
			wantPages: 1,
		},
		{
			name: "unresolvable and foreign kids are skipped",
			objects: []string{testCatalog,
				"<< /Type /Pages /Kids [99 0 R 3 0 R 4 0 R] /Count 3 >>",
				"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
				"<< /Type /Page /Parent 2 0 R >>"},
			wantPages: 1,
		},
		{
			name: "missing /Type is inferred from /Kids",
			objects: []string{testCatalog,
				"<< /Type /Pages /Kids [3 0 R] /Count 2 >>",
				"<< /Kids [4 0 R 5 0 R] /Count 2 >>",
				"<< /Parent 3 0 R >>",
				"<< /Type /Page /Parent 3 0 R >>"},
			wantPages: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := openTestPDF(t, tt.objects...)
			count, err := r.GetPageCount()
			if tt.wantPages == 0 {
				if err == nil {
					t.Fatalf("GetPageCount = %d, want error for a tree without pages", count)
				}
			} else if err != nil || count != tt.wantPages {
				t.Fatalf("GetPageCount = %d, %v; want %d", count, err, tt.wantPages)
			}
			for i := 0; i < tt.wantPages; i++ {
				if _, err := r.GetPage(i); err != nil {
					t.Errorf("GetPage(%d): %v", i, err)
				}
			}
			if _, err := r.GetPage(tt.wantPages); err == nil {
				t.Errorf("GetPage(%d) beyond the last page: want error", tt.wantPages)
			}
		})
	}
}

// TestPageTree_DocumentOrder verifies the index keeps the depth-first order
// of the tree (ISO 32000-1 §7.7.3.2).
func TestPageTree_DocumentOrder(t *testing.T) {
	r := openTestPDF(t, testCatalog,
		"<< /Type /Pages /Kids [3 0 R 6 0 R] /Count 3 >>",
		"<< /Type /Pages /Parent 2 0 R /Kids [4 0 R 5 0 R] /Count 2 >>",
		"<< /Type /Page /Parent 3 0 R /Tag 1 >>",
		"<< /Type /Page /Parent 3 0 R /Tag 2 >>",
		"<< /Type /Page /Parent 2 0 R /Tag 3 >>")
	for i := 0; i < 3; i++ {
		page, err := r.GetPage(i)
		if err != nil {
			t.Fatal(err)
		}
		if got := page.GetInteger("Tag"); got != int64(i+1) {
			t.Errorf("page %d has /Tag %d, want %d", i, got, i+1)
		}
	}
}

// TestPageTree_DepthLimit verifies a chain of /Pages nodes deeper than
// maxPageTreeDepth is cut off instead of being followed.
func TestPageTree_DepthLimit(t *testing.T) {
	const chain = maxPageTreeDepth + 10
	objects := []string{testCatalog}
	for i := 0; i < chain; i++ {
		objects = append(objects, "<< /Type /Pages /Kids ["+strconv.Itoa(i+3)+" 0 R] /Count 1 >>")
	}
	objects = append(objects, "<< /Type /Page >>")
	r := openTestPDF(t, objects...)
	if count, err := r.GetPageCount(); err == nil {
		t.Fatalf("GetPageCount = %d; the only page sits below the depth limit and must not be reached", count)
	}
}

// TestPageTree_ConcurrentAccess exercises the lazy index under concurrency
// (run with -race).
func TestPageTree_ConcurrentAccess(t *testing.T) {
	r := openTestPDF(t, testCatalog,
		"<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>",
		"<< /Type /Page /Parent 2 0 R >>",
		"<< /Type /Page /Parent 2 0 R >>")
	done := make(chan struct{})
	for g := 0; g < 8; g++ {
		go func() {
			defer func() { done <- struct{}{} }()
			if n, err := r.GetPageCount(); err != nil || n != 2 {
				t.Errorf("GetPageCount = %d, %v", n, err)
			}
			if _, err := r.GetPage(1); err != nil {
				t.Errorf("GetPage(1): %v", err)
			}
		}()
	}
	for g := 0; g < 8; g++ {
		<-done
	}
}
