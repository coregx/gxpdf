package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test file paths
const (
	testDataDir    = "../../testdata/pdfs"
	minimalPDF     = "minimal.pdf"
	multipagePDF   = "multipage.pdf"
	nestedPagesPDF = "nested_pages.pdf"
)

// getTestFilePath returns the absolute path to a test PDF file.
func getTestFilePath(filename string) string {
	return filepath.Join(testDataDir, filename)
}

// TestNewReader tests creating a new Reader.
func TestNewReader(t *testing.T) {
	reader := NewReader("test.pdf")
	require.NotNil(t, reader)
	assert.Equal(t, "test.pdf", reader.filename)
	assert.NotNil(t, reader.objectCache)
	assert.Len(t, reader.objectCache, 0)
}

// TestReader_Open_MinimalPDF tests opening a minimal valid PDF.
func TestReader_Open_MinimalPDF(t *testing.T) {
	pdfPath := getTestFilePath(minimalPDF)
	reader := NewReader(pdfPath)
	require.NotNil(t, reader)

	err := reader.Open()
	require.NoError(t, err)
	defer reader.Close()

	// Verify version
	assert.Equal(t, "1.7", reader.Version())

	// Verify catalog loaded
	catalog, err := reader.GetCatalog()
	require.NoError(t, err)
	require.NotNil(t, catalog)

	// Verify catalog type
	typeObj := catalog.GetName("Type")
	require.NotNil(t, typeObj)
	assert.Equal(t, "Catalog", typeObj.Value())

	// Verify pages loaded
	pages, err := reader.GetPages()
	require.NoError(t, err)
	require.NotNil(t, pages)

	// Verify page count
	count, err := reader.GetPageCount()
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// TestReader_Open_MultipagePDF tests opening a PDF with multiple pages.
func TestReader_Open_MultipagePDF(t *testing.T) {
	pdfPath := getTestFilePath(multipagePDF)
	reader := NewReader(pdfPath)
	require.NotNil(t, reader)

	err := reader.Open()
	require.NoError(t, err)
	defer reader.Close()

	// Verify version
	assert.Equal(t, "1.4", reader.Version())

	// Verify page count
	count, err := reader.GetPageCount()
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

// TestReader_Open_NestedPagesPDF tests opening a PDF with nested page tree.
func TestReader_Open_NestedPagesPDF(t *testing.T) {
	pdfPath := getTestFilePath(nestedPagesPDF)
	reader := NewReader(pdfPath)
	require.NotNil(t, reader)

	err := reader.Open()
	require.NoError(t, err)
	defer reader.Close()

	// Verify version
	assert.Equal(t, "1.5", reader.Version())

	// Verify page count
	count, err := reader.GetPageCount()
	require.NoError(t, err)
	assert.Equal(t, 4, count)
}

// TestReader_Open_FileNotFound tests opening a non-existent file.
func TestReader_Open_FileNotFound(t *testing.T) {
	reader := NewReader("nonexistent.pdf")
	err := reader.Open()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open file")
}

// TestReader_Open_InvalidHeader tests opening a file with invalid PDF header.
func TestReader_Open_InvalidHeader(t *testing.T) {
	// Create temp file with invalid header
	tmpFile, err := os.CreateTemp("", "invalid-*.pdf")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString("NOT A PDF\n")
	require.NoError(t, err)
	tmpFile.Close()

	reader := NewReader(tmpFile.Name())
	err = reader.Open()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid PDF header")
}

// TestReader_Open_MissingStartXRef tests opening a PDF without startxref.
func TestReader_Open_MissingStartXRef(t *testing.T) {
	// Create temp file without startxref
	tmpFile, err := os.CreateTemp("", "nostartxref-*.pdf")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString("%PDF-1.7\n%%EOF\n")
	require.NoError(t, err)
	tmpFile.Close()

	reader := NewReader(tmpFile.Name())
	err = reader.Open()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "startxref")
}

// TestReader_Close tests closing the reader.
func TestReader_Close(t *testing.T) {
	pdfPath := getTestFilePath(minimalPDF)
	reader := NewReader(pdfPath)

	err := reader.Open()
	require.NoError(t, err)

	err = reader.Close()
	assert.NoError(t, err)

	// Verify file is closed (file handle should be nil)
	assert.Nil(t, reader.file)

	// Closing again should not error
	err = reader.Close()
	assert.NoError(t, err)
}

// TestReader_GetObject tests retrieving objects by number.
func TestReader_GetObject(t *testing.T) {
	pdfPath := getTestFilePath(minimalPDF)
	reader := NewReader(pdfPath)

	err := reader.Open()
	require.NoError(t, err)
	defer reader.Close()

	// Get catalog object (object 1)
	obj, err := reader.GetObject(1)
	require.NoError(t, err)
	require.NotNil(t, obj)

	// Should be a dictionary
	dict, ok := obj.(*Dictionary)
	require.True(t, ok, "object 1 should be a dictionary")

	// Verify it's the catalog
	typeObj := dict.GetName("Type")
	require.NotNil(t, typeObj)
	assert.Equal(t, "Catalog", typeObj.Value())

	// Get pages object (object 2)
	obj2, err := reader.GetObject(2)
	require.NoError(t, err)
	require.NotNil(t, obj2)

	dict2, ok := obj2.(*Dictionary)
	require.True(t, ok)
	typeObj2 := dict2.GetName("Type")
	require.NotNil(t, typeObj2)
	assert.Equal(t, "Pages", typeObj2.Value())
}

// TestReader_GetObject_NotFound tests retrieving a non-existent object.
func TestReader_GetObject_NotFound(t *testing.T) {
	pdfPath := getTestFilePath(minimalPDF)
	reader := NewReader(pdfPath)

	err := reader.Open()
	require.NoError(t, err)
	defer reader.Close()

	// Try to get non-existent object
	_, err = reader.GetObject(999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestReader_GetObject_Caching tests that objects are cached.
func TestReader_GetObject_Caching(t *testing.T) {
	pdfPath := getTestFilePath(minimalPDF)
	reader := NewReader(pdfPath)

	err := reader.Open()
	require.NoError(t, err)
	defer reader.Close()

	// Get object first time
	obj1, err := reader.GetObject(1)
	require.NoError(t, err)

	// Verify it's cached (at least object 1 should be cached)
	assert.Greater(t, len(reader.objectCache), 0)
	_, cached := reader.objectCache[1]
	assert.True(t, cached, "object 1 should be in cache")

	// Get same object again
	obj2, err := reader.GetObject(1)
	require.NoError(t, err)

	// Should be the same instance (from cache)
	assert.Equal(t, obj1, obj2)
}

// TestReader_GetPage tests retrieving pages.
func TestReader_GetPage(t *testing.T) {
	pdfPath := getTestFilePath(multipagePDF)
	reader := NewReader(pdfPath)

	err := reader.Open()
	require.NoError(t, err)
	defer reader.Close()

	// Get first page (index 0)
	page0, err := reader.GetPage(0)
	require.NoError(t, err)
	require.NotNil(t, page0)

	typeObj := page0.GetName("Type")
	require.NotNil(t, typeObj)
	assert.Equal(t, "Page", typeObj.Value())

	// Get second page (index 1)
	page1, err := reader.GetPage(1)
	require.NoError(t, err)
	require.NotNil(t, page1)

	// Get third page (index 2)
	page2, err := reader.GetPage(2)
	require.NoError(t, err)
	require.NotNil(t, page2)

	// Verify they're different objects
	assert.NotEqual(t, page0, page1)
	assert.NotEqual(t, page1, page2)
}

// TestReader_GetPage_NestedTree tests retrieving pages from nested page tree.
func TestReader_GetPage_NestedTree(t *testing.T) {
	pdfPath := getTestFilePath(nestedPagesPDF)
	reader := NewReader(pdfPath)

	err := reader.Open()
	require.NoError(t, err)
	defer reader.Close()

	// Get all 4 pages
	for i := 0; i < 4; i++ {
		page, err := reader.GetPage(i)
		require.NoError(t, err, "failed to get page %d", i)
		require.NotNil(t, page, "page %d is nil", i)

		typeObj := page.GetName("Type")
		require.NotNil(t, typeObj, "page %d missing /Type", i)
		assert.Equal(t, "Page", typeObj.Value(), "page %d wrong type", i)
	}
}

// TestReader_GetPage_InvalidIndex tests retrieving pages with invalid index.
func TestReader_GetPage_InvalidIndex(t *testing.T) {
	pdfPath := getTestFilePath(minimalPDF)
	reader := NewReader(pdfPath)

	err := reader.Open()
	require.NoError(t, err)
	defer reader.Close()

	// Negative index
	_, err = reader.GetPage(-1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid page number")

	// Index too large
	_, err = reader.GetPage(999)
	require.Error(t, err)
}

// TestReader_GetPage_NotOpened tests calling GetPage before Open.
func TestReader_GetPage_NotOpened(t *testing.T) {
	reader := NewReader("test.pdf")

	_, err := reader.GetPage(0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not loaded")
}

// TestReader_GetCatalog tests retrieving the catalog.
func TestReader_GetCatalog(t *testing.T) {
	pdfPath := getTestFilePath(minimalPDF)
	reader := NewReader(pdfPath)

	err := reader.Open()
	require.NoError(t, err)
	defer reader.Close()

	catalog, err := reader.GetCatalog()
	require.NoError(t, err)
	require.NotNil(t, catalog)

	// Verify catalog has required entries
	assert.True(t, catalog.Has("Type"))
	assert.True(t, catalog.Has("Pages"))
}

// TestReader_GetCatalog_NotOpened tests calling GetCatalog before Open.
func TestReader_GetCatalog_NotOpened(t *testing.T) {
	reader := NewReader("test.pdf")

	_, err := reader.GetCatalog()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not loaded")
}

// TestReader_GetPages tests retrieving the page tree root.
func TestReader_GetPages(t *testing.T) {
	pdfPath := getTestFilePath(minimalPDF)
	reader := NewReader(pdfPath)

	err := reader.Open()
	require.NoError(t, err)
	defer reader.Close()

	pages, err := reader.GetPages()
	require.NoError(t, err)
	require.NotNil(t, pages)

	// Verify pages has required entries
	assert.True(t, pages.Has("Type"))
	assert.True(t, pages.Has("Kids"))
	assert.True(t, pages.Has("Count"))
}

// TestReader_GetPageCount tests retrieving page count.
func TestReader_GetPageCount(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		expected int
	}{
		{"Minimal PDF", minimalPDF, 1},
		{"Multipage PDF", multipagePDF, 3},
		{"Nested Pages PDF", nestedPagesPDF, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdfPath := getTestFilePath(tt.file)
			reader := NewReader(pdfPath)

			err := reader.Open()
			require.NoError(t, err)
			defer reader.Close()

			count, err := reader.GetPageCount()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, count)
		})
	}
}

// TestReader_Trailer tests retrieving the trailer dictionary.
func TestReader_Trailer(t *testing.T) {
	pdfPath := getTestFilePath(minimalPDF)
	reader := NewReader(pdfPath)

	err := reader.Open()
	require.NoError(t, err)
	defer reader.Close()

	trailer := reader.Trailer()
	require.NotNil(t, trailer)

	// Verify trailer has required entries
	assert.True(t, trailer.Has("Size"))
	assert.True(t, trailer.Has("Root"))

	// Verify Size
	size := trailer.GetInteger("Size")
	assert.Greater(t, size, int64(0))
}

// TestReader_XRefTable tests retrieving the xref table.
func TestReader_XRefTable(t *testing.T) {
	pdfPath := getTestFilePath(minimalPDF)
	reader := NewReader(pdfPath)

	err := reader.Open()
	require.NoError(t, err)
	defer reader.Close()

	xref := reader.XRefTable()
	require.NotNil(t, xref)

	// Verify xref has entries
	assert.Greater(t, xref.Size(), 0)

	// Verify object 1 exists
	entry, ok := xref.GetEntry(1)
	require.True(t, ok)
	assert.Equal(t, XRefEntryInUse, entry.Type)
}

// TestReader_Version tests retrieving PDF version.
func TestReader_Version(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		expected string
	}{
		{"PDF 1.7", minimalPDF, "1.7"},
		{"PDF 1.4", multipagePDF, "1.4"},
		{"PDF 1.5", nestedPagesPDF, "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdfPath := getTestFilePath(tt.file)
			reader := NewReader(pdfPath)

			err := reader.Open()
			require.NoError(t, err)
			defer reader.Close()

			version := reader.Version()
			assert.Equal(t, tt.expected, version)
		})
	}
}

// TestReader_String tests the String() method.
func TestReader_String(t *testing.T) {
	pdfPath := getTestFilePath(minimalPDF)
	reader := NewReader(pdfPath)

	err := reader.Open()
	require.NoError(t, err)
	defer reader.Close()

	str := reader.String()
	assert.Contains(t, str, "PDFReader")
	assert.Contains(t, str, "minimal.pdf")
	assert.Contains(t, str, "version=\"1.7\"")
	assert.Contains(t, str, "pages=1")
}

// TestOpenPDF tests the convenience function OpenPDF.
func TestOpenPDF(t *testing.T) {
	pdfPath := getTestFilePath(minimalPDF)
	reader, err := OpenPDF(pdfPath)
	require.NoError(t, err)
	require.NotNil(t, reader)
	defer reader.Close()

	// Verify it's opened and ready
	assert.Equal(t, "1.7", reader.Version())

	count, err := reader.GetPageCount()
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// TestOpenPDF_Error tests OpenPDF with invalid file.
func TestOpenPDF_Error(t *testing.T) {
	_, err := OpenPDF("nonexistent.pdf")
	require.Error(t, err)
}

// TestReadPDFInfo tests the convenience function ReadPDFInfo.
func TestReadPDFInfo(t *testing.T) {
	pdfPath := getTestFilePath(multipagePDF)
	version, pageCount, err := ReadPDFInfo(pdfPath)
	require.NoError(t, err)

	assert.Equal(t, "1.4", version)
	assert.Equal(t, 3, pageCount)
}

// TestReadPDFInfo_Error tests ReadPDFInfo with invalid file.
func TestReadPDFInfo_Error(t *testing.T) {
	_, _, err := ReadPDFInfo("nonexistent.pdf")
	require.Error(t, err)
}

// TestReader_ResolveReferences tests indirect reference resolution.
func TestReader_ResolveReferences(t *testing.T) {
	pdfPath := getTestFilePath(minimalPDF)
	reader := NewReader(pdfPath)

	err := reader.Open()
	require.NoError(t, err)
	defer reader.Close()

	// Create an indirect reference
	ref := NewIndirectReference(1, 0)

	// Resolve it
	resolved := reader.resolveReferences(ref)

	// Should be the catalog dictionary
	dict, ok := resolved.(*Dictionary)
	require.True(t, ok)

	typeObj := dict.GetName("Type")
	require.NotNil(t, typeObj)
	assert.Equal(t, "Catalog", typeObj.Value())
}

// TestReader_ResolveReferences_Array tests resolving references in arrays.
func TestReader_ResolveReferences_Array(t *testing.T) {
	pdfPath := getTestFilePath(minimalPDF)
	reader := NewReader(pdfPath)

	err := reader.Open()
	require.NoError(t, err)
	defer reader.Close()

	// Create array with indirect reference
	arr := NewArray()
	arr.Append(NewIndirectReference(1, 0))
	arr.Append(NewInteger(42))

	// Resolve references
	resolved := reader.resolveReferences(arr)

	// Should still be an array
	resolvedArr, ok := resolved.(*Array)
	require.True(t, ok)
	assert.Equal(t, 2, resolvedArr.Len())

	// First element should be resolved to catalog
	elem0 := resolvedArr.Get(0)
	_, ok = elem0.(*Dictionary)
	require.True(t, ok)

	// Second element should still be integer
	elem1 := resolvedArr.Get(1)
	intObj, ok := elem1.(*Integer)
	require.True(t, ok)
	assert.Equal(t, int64(42), intObj.Value())
}

// TestReader_ResolveReferences_Dictionary tests resolving references in dictionaries.
func TestReader_ResolveReferences_Dictionary(t *testing.T) {
	pdfPath := getTestFilePath(minimalPDF)
	reader := NewReader(pdfPath)

	err := reader.Open()
	require.NoError(t, err)
	defer reader.Close()

	// Create dictionary with indirect reference
	dict := NewDictionary()
	dict.Set("Catalog", NewIndirectReference(1, 0))
	dict.Set("Number", NewInteger(123))

	// Resolve references
	resolved := reader.resolveReferences(dict)

	// Should still be a dictionary
	resolvedDict, ok := resolved.(*Dictionary)
	require.True(t, ok)
	assert.Equal(t, 2, resolvedDict.Len())

	// Catalog should be resolved
	catalogObj := resolvedDict.Get("Catalog")
	catalogDict, ok := catalogObj.(*Dictionary)
	require.True(t, ok)
	typeObj := catalogDict.GetName("Type")
	require.NotNil(t, typeObj)
	assert.Equal(t, "Catalog", typeObj.Value())

	// Number should still be integer
	numObj := resolvedDict.Get("Number")
	intObj, ok := numObj.(*Integer)
	require.True(t, ok)
	assert.Equal(t, int64(123), intObj.Value())
}

// TestReader_ConcurrentAccess tests thread-safe concurrent object access.
func TestReader_ConcurrentAccess(t *testing.T) {
	pdfPath := getTestFilePath(multipagePDF)
	reader := NewReader(pdfPath)

	err := reader.Open()
	require.NoError(t, err)
	defer reader.Close()

	// Launch multiple goroutines accessing objects concurrently
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(pageNum int) {
			// Get page
			page, err := reader.GetPage(pageNum % 3)
			if err != nil {
				t.Errorf("failed to get page: %v", err)
			}

			// Verify it's a page
			if page != nil {
				typeObj := page.GetName("Type")
				if typeObj == nil || typeObj.Value() != "Page" {
					t.Errorf("expected Page, got %v", typeObj)
				}
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

// TestReader_HeaderValidation tests PDF header validation for error cases.
func TestReader_HeaderValidation(t *testing.T) {
	tests := []struct {
		name    string
		content string
		errMsg  string
	}{
		{
			name:    "Invalid prefix",
			content: "PDF-1.7\n",
			errMsg:  "invalid PDF header",
		},
		{
			name:    "Missing version",
			content: "%PDF-\n",
			errMsg:  "invalid PDF version",
		},
		{
			name:    "Empty file",
			content: "",
			errMsg:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpFile, err := os.CreateTemp("", "test-*.pdf")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())

			_, err = tmpFile.WriteString(tt.content)
			require.NoError(t, err)
			tmpFile.Close()

			// Test reading
			reader := NewReader(tmpFile.Name())
			err = reader.Open()

			require.Error(t, err)
			if tt.errMsg != "" {
				assert.Contains(t, err.Error(), tt.errMsg)
			}
		})
	}
}

// TestReader_HeaderWithLeadingWhitespace tests that PDFs with leading whitespace
// before the %PDF- header are parsed correctly. Some PDF generators produce files
// with leading tabs, spaces, or newlines before the header.
func TestReader_HeaderWithLeadingWhitespace(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		wantVer string
	}{
		{
			name:    "Leading newlines and tabs",
			prefix:  "\r\n\t\t\t\t \r\n",
			wantVer: "1.4",
		},
		{
			name:    "Leading spaces",
			prefix:  "   ",
			wantVer: "1.7",
		},
		{
			name:    "Leading CRLF",
			prefix:  "\r\n\r\n",
			wantVer: "2.0",
		},
		{
			name:    "UTF-8 BOM",
			prefix:  "\xef\xbb\xbf",
			wantVer: "1.7",
		},
		{
			name:    "UTF-8 BOM with whitespace",
			prefix:  "\xef\xbb\xbf  \r\n",
			wantVer: "1.4",
		},
		{
			name:    "Header near 1024 byte boundary",
			prefix:  strings.Repeat(" ", 1000), // 1000 spaces, header at ~1008
			wantVer: "1.7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build PDF content without prefix first to calculate correct offsets
			pdfContent := "%PDF-" + tt.wantVer + "\n" +
				"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
				"2 0 obj\n<< /Type /Pages /Kids [] /Count 0 >>\nendobj\n"

			// Calculate where xref starts (without prefix)
			xrefOffset := len(pdfContent)

			// Build xref table - offsets are relative to %PDF- (not file start)
			obj1Offset := len("%PDF-" + tt.wantVer + "\n")
			obj2Offset := obj1Offset + len("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

			xrefTable := "xref\n0 3\n" +
				"0000000000 65535 f \n" +
				fmt.Sprintf("%010d", obj1Offset) + " 00000 n \n" +
				fmt.Sprintf("%010d", obj2Offset) + " 00000 n \n"

			trailer := "trailer\n<< /Root 1 0 R /Size 3 >>\n" +
				"startxref\n" + fmt.Sprintf("%d", xrefOffset) + "\n%%EOF\n"

			// Now prepend the whitespace prefix
			content := tt.prefix + pdfContent + xrefTable + trailer

			tmpFile, err := os.CreateTemp("", "whitespace-*.pdf")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())

			_, err = tmpFile.WriteString(content)
			require.NoError(t, err)
			tmpFile.Close()

			reader := NewReader(tmpFile.Name())
			err = reader.Open()

			require.NoError(t, err, "PDF with leading whitespace should open successfully")
			defer reader.Close()

			assert.Equal(t, tt.wantVer, reader.Version())

			catalog, err := reader.GetCatalog()
			require.NoError(t, err, "Should be able to read catalog")
			require.NotNil(t, catalog)

			typeObj := catalog.GetName("Type")
			require.NotNil(t, typeObj)
			assert.Equal(t, "Catalog", typeObj.Value())
		})
	}
}

// TestReader_HeaderWithInvalidPrefix tests that non-whitespace before the header is rejected.
func TestReader_HeaderWithInvalidPrefix(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
	}{
		{
			name:   "Non-whitespace character before header",
			prefix: "X",
		},
		{
			name:   "Null byte before header",
			prefix: "\x00",
		},
		{
			name:   "HTML before header",
			prefix: "<html>",
		},
		{
			name:   "Header beyond 1024 byte window",
			prefix: strings.Repeat(" ", 1025),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := tt.prefix + "%PDF-1.7\n" +
				"1 0 obj\n<< /Type /Catalog >>\nendobj\n" +
				"xref\n0 2\n0000000000 65535 f \n0000000009 00000 n \n" +
				"trailer\n<< /Root 1 0 R /Size 2 >>\nstartxref\n56\n%%EOF\n"

			tmpFile, err := os.CreateTemp("", "invalid-prefix-*.pdf")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())

			_, err = tmpFile.WriteString(content)
			require.NoError(t, err)
			tmpFile.Close()

			reader := NewReader(tmpFile.Name())
			err = reader.Open()

			require.Error(t, err, "PDF with invalid prefix should be rejected")
		})
	}
}

// TestReader_EmptyFile tests opening an empty file.
func TestReader_EmptyFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "empty-*.pdf")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	reader := NewReader(tmpFile.Name())
	err = reader.Open()
	require.Error(t, err)
	// Should fail at header reading or startxref finding
}

// ============================================================================
// /Prev Chain and /XRefStm Integration Tests (Issue #19)
// ============================================================================

// buildPrevChainPDF creates a synthetic PDF with two xref sections linked by /Prev.
// Section 1 (older): objects 0-4 (catalog, pages, page, content stream)
// Section 2 (newer): object 5 (new info dict) with /Prev pointing to section 1
func buildPrevChainPDF() []byte {
	// Section 1: Traditional xref at a known offset
	body := "%PDF-1.7\n" +
		// Object 1: Catalog
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
		// Object 2: Pages
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n" +
		// Object 3: Page
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << >> >> >>\nendobj\n" +
		// Object 4: Content stream
		"4 0 obj\n<< /Length 44 >>\nstream\nBT\n/F1 12 Tf\n100 700 Td\n(Hello World) Tj\nET\nendstream\nendobj\n"

	xref1Offset := len(body)

	xref1 := fmt.Sprintf("xref\n0 5\n"+
		"0000000000 65535 f \n"+
		"%010d 00000 n \n"+
		"%010d 00000 n \n"+
		"%010d 00000 n \n"+
		"%010d 00000 n \n",
		9,   // obj 1 offset
		58,  // obj 2 offset
		115, // obj 3 offset
		231, // obj 4 offset
	)

	trailer1 := fmt.Sprintf("trailer\n<< /Size 6 /Root 1 0 R >>\n")

	// Now add object 5 (Info dict) in the "update"
	afterXref1 := body + xref1 + trailer1

	obj5Offset := len(afterXref1)
	obj5 := "5 0 obj\n<< /Title (Test Document) /Author (Test Author) >>\nendobj\n"
	afterObj5 := afterXref1 + obj5

	xref2Offset := len(afterObj5)

	xref2 := fmt.Sprintf("xref\n5 1\n"+
		"%010d 00000 n \n", obj5Offset)

	trailer2 := fmt.Sprintf("trailer\n<< /Size 6 /Root 1 0 R /Info 5 0 R /Prev %d >>\n"+
		"startxref\n%d\n%%%%EOF\n",
		xref1Offset, xref2Offset)

	return []byte(afterObj5 + xref2 + trailer2)
}

func TestReader_Open_PrevChain(t *testing.T) {
	data := buildPrevChainPDF()

	tmpFile, err := os.CreateTemp("", "prevchain-*.pdf")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.Write(data)
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	reader := NewReader(tmpFile.Name())
	err = reader.Open()
	require.NoError(t, err)
	defer reader.Close()

	// Must see objects from both sections
	assert.Equal(t, "1.7", reader.Version())

	count, err := reader.GetPageCount()
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Object 1 (catalog from old section) must be accessible
	obj1, err := reader.GetObject(1)
	require.NoError(t, err)
	dict1, ok := obj1.(*Dictionary)
	require.True(t, ok)
	assert.Equal(t, "Catalog", dict1.GetName("Type").Value())

	// Object 5 (from new section) must be accessible
	obj5, err := reader.GetObject(5)
	require.NoError(t, err)
	dict5, ok := obj5.(*Dictionary)
	require.True(t, ok)
	assert.Equal(t, "Test Document", dict5.GetString("Title"))

	// Newest trailer should have /Info
	trailer := reader.Trailer()
	require.NotNil(t, trailer)
	assert.NotNil(t, trailer.Get("Info"), "newest trailer should have /Info")
}

func TestReader_Open_PrevChainCycleDetection(t *testing.T) {
	// Build a PDF where /Prev points back to itself
	body := "%PDF-1.7\n" +
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n" +
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>\nendobj\n"

	xrefOffset := len(body)

	xref := fmt.Sprintf("xref\n0 4\n"+
		"0000000000 65535 f \n"+
		"%010d 00000 n \n"+
		"%010d 00000 n \n"+
		"%010d 00000 n \n",
		9, 58, 115)

	// /Prev points to itself — cycle!
	trailer := fmt.Sprintf("trailer\n<< /Size 4 /Root 1 0 R /Prev %d >>\n"+
		"startxref\n%d\n%%%%EOF\n",
		xrefOffset, xrefOffset)

	data := []byte(body + xref + trailer)

	tmpFile, err := os.CreateTemp("", "cycle-*.pdf")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.Write(data)
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	reader := NewReader(tmpFile.Name())
	err = reader.Open()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle detected")
}

func TestReader_Open_SimplePDF_StillWorks(t *testing.T) {
	// Backward compatibility: existing test PDFs must still open correctly
	tests := []struct {
		name      string
		file      string
		version   string
		pageCount int
	}{
		{"Minimal PDF", minimalPDF, "1.7", 1},
		{"Multipage PDF", multipagePDF, "1.4", 3},
		{"Nested Pages PDF", nestedPagesPDF, "1.5", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdfPath := getTestFilePath(tt.file)
			reader := NewReader(pdfPath)

			err := reader.Open()
			require.NoError(t, err)
			defer reader.Close()

			assert.Equal(t, tt.version, reader.Version())

			count, err := reader.GetPageCount()
			require.NoError(t, err)
			assert.Equal(t, tt.pageCount, count)
		})
	}
}

const msWordHybridPDF = "msword_hybrid.pdf"

func TestReader_Open_MSWordPDF(t *testing.T) {
	pdfPath := getTestFilePath(msWordHybridPDF)

	// Skip if test file doesn't exist
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		t.Skip("MS Word hybrid test PDF not available")
	}

	reader := NewReader(pdfPath)
	err := reader.Open()
	require.NoError(t, err, "should open MS Word hybrid-reference PDF without error")
	defer reader.Close()

	// Verify basic structure
	assert.NotEmpty(t, reader.Version())

	// Must be able to get page count
	count, err := reader.GetPageCount()
	require.NoError(t, err)
	assert.Greater(t, count, 0, "should have at least 1 page")

	// Object 1 must be accessible (this was the failing case in issue #19)
	obj1, err := reader.GetObject(1)
	require.NoError(t, err, "object 1 must be found in merged xref table")
	require.NotNil(t, obj1)

	// Verify xref has entries from the /Prev chain
	xref := reader.XRefTable()
	assert.Greater(t, xref.Size(), 5, "xref should have entries from multiple sections")
}

// ============================================================================
// Benchmark Tests
// ============================================================================

// BenchmarkReader_Open benchmarks opening a PDF.
func BenchmarkReader_Open(b *testing.B) {
	pdfPath := getTestFilePath(minimalPDF)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := NewReader(pdfPath)
		if err := reader.Open(); err != nil {
			b.Fatal(err)
		}
		reader.Close()
	}
}

// BenchmarkReader_GetPage benchmarks page retrieval.
func BenchmarkReader_GetPage(b *testing.B) {
	pdfPath := getTestFilePath(multipagePDF)
	reader := NewReader(pdfPath)
	if err := reader.Open(); err != nil {
		b.Fatal(err)
	}
	defer reader.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := reader.GetPage(i % 3)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkReader_GetObject benchmarks object retrieval.
func BenchmarkReader_GetObject(b *testing.B) {
	pdfPath := getTestFilePath(minimalPDF)
	reader := NewReader(pdfPath)
	if err := reader.Open(); err != nil {
		b.Fatal(err)
	}
	defer reader.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := reader.GetObject(1)
		if err != nil {
			b.Fatal(err)
		}
	}
}
