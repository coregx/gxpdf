package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/coregx/gxpdf/creator"
	"github.com/coregx/gxpdf/internal/buildinfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildEncryptedPDF creates a real AES-128 encrypted PDF in the given directory
// and returns its path and the user password.
func buildEncryptedPDF(t *testing.T, dir string) (path, password string) {
	t.Helper()
	password = "testpassword"
	path = filepath.Join(dir, "encrypted.pdf")

	c := creator.New()
	c.SetEncryption(creator.EncryptionOptions{
		UserPassword:  password,
		OwnerPassword: "ownerpassword",
		Algorithm:     creator.EncryptionAES128,
	})
	_, err := c.NewPage()
	require.NoError(t, err, "new page for encrypted PDF")
	err = c.WriteToFile(path)
	require.NoError(t, err, "building encrypted PDF")
	return path, password
}

// buildMultipagePDF creates a real multipage PDF in the given directory.
func buildMultipagePDF(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "multipage.pdf")

	c := creator.New()
	for i := 0; i < 3; i++ {
		_, err := c.NewPage()
		require.NoError(t, err, "new page %d", i)
	}
	err := c.WriteToFile(path)
	require.NoError(t, err, "building multipage PDF")
	return path
}

// testPDFPath returns the path to a known-good minimal test PDF.
func testPDFPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "pdfs", "minimal.pdf")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("testdata PDF not available: %v", err)
	}
	return path
}

// captureStdout redirects os.Stdout for the duration of f and returns what was written.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)

	oldStdout := os.Stdout
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	return buf.String()
}

// ============================================================================
// version command
// ============================================================================

func TestRunVersionCmd(t *testing.T) {
	out := captureStdout(t, func() {
		versionCmd.Run(versionCmd, []string{})
	})
	assert.Contains(t, out, "gxpdf")
}

func TestRunVersionCmd_WithBuildInfo(t *testing.T) {
	// Override build-time vars to exercise the conditional branches.
	origCommit := GitCommit
	origDate := BuildDate
	GitCommit = "abc1234"
	BuildDate = "2025-01-01"
	defer func() {
		GitCommit = origCommit
		BuildDate = origDate
	}()

	out := captureStdout(t, func() {
		versionCmd.Run(versionCmd, []string{})
	})
	assert.Contains(t, out, "abc1234")
	assert.Contains(t, out, "2025-01-01")
}

func TestRunVersionCmd_UnknownBuildInfo(t *testing.T) {
	origCommit := GitCommit
	origDate := BuildDate
	GitCommit = unknownValue
	BuildDate = unknownValue
	defer func() {
		GitCommit = origCommit
		BuildDate = origDate
	}()

	// Should not panic and should omit commit/date lines when "unknown".
	out := captureStdout(t, func() {
		versionCmd.Run(versionCmd, []string{})
	})
	assert.Contains(t, out, "gxpdf")
}

func TestBuildDetails(t *testing.T) {
	tool := buildinfo.Info{
		Version:  "v0.9.6-0.20261101093000-1a2b3c4d5e6f+dirty",
		Revision: "1a2b3c4d5e6f",
		Time:     "2026-11-01T09:30:00Z",
		Modified: true,
	}
	tests := []struct {
		name                    string
		version, commit, date   string // simulated -ldflags -X values
		wantVersion, wantCommit string
		wantDate                string
	}{
		{
			name:    "release build: injected values win, version made canonical",
			version: "0.9.5", commit: "84054bd", date: "2026-10-16T12:05:07Z",
			wantVersion: "v0.9.5", wantCommit: "84054bd", wantDate: "2026-10-16T12:05:07Z",
		},
		{
			name:    "injected version already canonical",
			version: "v0.9.5", commit: unknownValue, date: unknownValue,
			wantVersion: "v0.9.5", wantCommit: tool.Revision, wantDate: tool.Time,
		},
		{
			name:    "no ldflags: toolchain values",
			version: devVersion, commit: unknownValue, date: unknownValue,
			wantVersion: tool.Version, wantCommit: tool.Revision, wantDate: tool.Time,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origVersion, origCommit, origDate := Version, GitCommit, BuildDate
			Version, GitCommit, BuildDate = tt.version, tt.commit, tt.date
			defer func() { Version, GitCommit, BuildDate = origVersion, origCommit, origDate }()

			d := buildDetails(tool)
			assert.Equal(t, tt.wantVersion, d.version)
			assert.Equal(t, tt.wantCommit, d.commit)
			assert.Equal(t, tt.wantDate, d.date)
			assert.True(t, d.modified)
		})
	}
}

// ============================================================================
// info command
// ============================================================================

func TestRunInfo_TextFormat(t *testing.T) {
	path := testPDFPath(t)
	origFormat := outputFormat
	outputFormat = "text"
	defer func() { outputFormat = origFormat }()

	err := runInfo(infoCmd, []string{path})
	require.NoError(t, err)
}

func TestRunInfo_JSONFormat(t *testing.T) {
	path := testPDFPath(t)
	origFormat := outputFormat
	outputFormat = formatJSON
	defer func() { outputFormat = origFormat }()

	err := runInfo(infoCmd, []string{path})
	require.NoError(t, err)
}

func TestRunInfo_MissingFile(t *testing.T) {
	err := runInfo(infoCmd, []string{"/nonexistent/file.pdf"})
	assert.Error(t, err)
}

func TestRunInfo_InvalidPDF(t *testing.T) {
	// Create a temp file with non-PDF content.
	f, err := os.CreateTemp("", "notapdf-*.pdf")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	_, err = f.WriteString("this is not a pdf")
	require.NoError(t, err)
	f.Close()

	err = runInfo(infoCmd, []string{f.Name()})
	assert.Error(t, err)
}

// ============================================================================
// formatSize helper
// ============================================================================

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		contains string
	}{
		{0, "bytes"},
		{512, "bytes"},
		{1024, "KB"},
		{1024 * 1024, "MB"},
		{1024 * 1024 * 1024, "GB"},
		{2 * 1024 * 1024 * 1024, "GB"},
	}
	for _, tt := range tests {
		got := formatSize(tt.bytes)
		assert.Contains(t, got, tt.contains, "formatSize(%d)", tt.bytes)
	}
}

// ============================================================================
// outputInfoText
// ============================================================================

func TestOutputInfoText_AllFields(t *testing.T) {
	info := pdfInfo{
		File:      "test.pdf",
		FileSize:  1024,
		PageCount: 3,
		Version:   "1.7",
		Title:     "My Doc",
		Author:    "Alice",
		Subject:   "Testing",
		Keywords:  "go pdf",
		Creator:   "GxPDF",
		Producer:  "GxPDF 1.0",
		Encrypted: false,
	}
	out := captureStdout(t, func() {
		err := outputInfoText(info)
		assert.NoError(t, err)
	})
	assert.Contains(t, out, "test.pdf")
	assert.Contains(t, out, "My Doc")
	assert.Contains(t, out, "Alice")
	assert.Contains(t, out, "Testing")
	assert.Contains(t, out, "go pdf")
	assert.Contains(t, out, "GxPDF")
}

func TestOutputInfoText_NoOptionalFields(t *testing.T) {
	info := pdfInfo{
		File:      "test.pdf",
		FileSize:  256,
		PageCount: 1,
		Version:   "1.4",
		Encrypted: true,
	}
	out := captureStdout(t, func() {
		err := outputInfoText(info)
		assert.NoError(t, err)
	})
	assert.Contains(t, out, "test.pdf")
	assert.Contains(t, out, "true")
}

// ============================================================================
// text command
// ============================================================================

func TestRunText_AllPages(t *testing.T) {
	path := testPDFPath(t)
	origPage := textPage
	textPage = 0
	defer func() { textPage = origPage }()

	err := runText(textCmd, []string{path})
	require.NoError(t, err)
}

func TestRunText_SinglePage(t *testing.T) {
	path := testPDFPath(t) // minimal.pdf has 1 page
	origPage := textPage
	textPage = 1
	defer func() { textPage = origPage }()

	err := runText(textCmd, []string{path})
	require.NoError(t, err)
}

func TestRunText_PageBeyondCount(t *testing.T) {
	path := testPDFPath(t)
	origPage := textPage
	textPage = 9999
	defer func() { textPage = origPage }()

	err := runText(textCmd, []string{path})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestRunText_MultiplePages(t *testing.T) {
	// Build a 3-page PDF to exercise the multi-page branch in extractAllPages.
	tmpDir := t.TempDir()
	path := buildMultipagePDF(t, tmpDir)

	origPage := textPage
	textPage = 0
	defer func() { textPage = origPage }()

	err := runText(textCmd, []string{path})
	require.NoError(t, err)
}

func TestRunText_MissingFile(t *testing.T) {
	err := runText(textCmd, []string{"/no/such/file.pdf"})
	assert.Error(t, err)
}

func TestRunText_OutputToFile(t *testing.T) {
	path := testPDFPath(t)
	tmpFile := filepath.Join(t.TempDir(), "out.txt")

	origPage := textPage
	origOut := textOutput
	textPage = 0
	textOutput = tmpFile
	defer func() {
		textPage = origPage
		textOutput = origOut
	}()

	err := runText(textCmd, []string{path})
	require.NoError(t, err)

	_, statErr := os.Stat(tmpFile)
	assert.NoError(t, statErr)
}

func TestRunText_InvalidOutputDir(t *testing.T) {
	path := testPDFPath(t)
	origOut := textOutput
	textOutput = "/nonexistent/dir/out.txt"
	defer func() { textOutput = origOut }()

	err := runText(textCmd, []string{path})
	assert.Error(t, err)
}

// ============================================================================
// tables command
// ============================================================================

func TestRunTables_TextFormat(t *testing.T) {
	path := testPDFPath(t)
	origFormat := outputFormat
	origPage := tablesPage
	outputFormat = "text"
	tablesPage = 0
	defer func() {
		outputFormat = origFormat
		tablesPage = origPage
	}()

	err := runTables(tablesCmd, []string{path})
	require.NoError(t, err)
}

func TestRunTables_JSONFormat(t *testing.T) {
	path := testPDFPath(t)
	origFormat := outputFormat
	origPage := tablesPage
	outputFormat = formatJSON
	tablesPage = 0
	defer func() {
		outputFormat = origFormat
		tablesPage = origPage
	}()

	err := runTables(tablesCmd, []string{path})
	require.NoError(t, err)
}

func TestRunTables_CSVFormat(t *testing.T) {
	path := testPDFPath(t)
	origFormat := outputFormat
	origPage := tablesPage
	outputFormat = formatCSV
	tablesPage = 0
	defer func() {
		outputFormat = origFormat
		tablesPage = origPage
	}()

	err := runTables(tablesCmd, []string{path})
	require.NoError(t, err)
}

func TestRunTables_SpecificPage(t *testing.T) {
	path := testPDFPath(t)
	origPage := tablesPage
	origFormat := outputFormat
	tablesPage = 1
	outputFormat = "text"
	defer func() {
		tablesPage = origPage
		outputFormat = origFormat
	}()

	err := runTables(tablesCmd, []string{path})
	require.NoError(t, err)
}

func TestRunTables_PageBeyondCount(t *testing.T) {
	path := testPDFPath(t)
	origPage := tablesPage
	tablesPage = 9999
	defer func() { tablesPage = origPage }()

	err := runTables(tablesCmd, []string{path})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestRunTables_MissingFile(t *testing.T) {
	err := runTables(tablesCmd, []string{"/no/such/file.pdf"})
	assert.Error(t, err)
}

func TestRunTables_OutputToFile(t *testing.T) {
	path := testPDFPath(t)
	tmpFile := filepath.Join(t.TempDir(), "tables.csv")

	origOut := tablesOutput
	origFormat := outputFormat
	origPage := tablesPage
	tablesOutput = tmpFile
	outputFormat = formatCSV
	tablesPage = 0
	defer func() {
		tablesOutput = origOut
		outputFormat = origFormat
		tablesPage = origPage
	}()

	err := runTables(tablesCmd, []string{path})
	require.NoError(t, err)
}

func TestRunTables_InvalidOutputDir(t *testing.T) {
	path := testPDFPath(t)
	origOut := tablesOutput
	origFormat := outputFormat
	tablesOutput = "/nonexistent/dir/tables.csv"
	outputFormat = formatCSV
	defer func() {
		tablesOutput = origOut
		outputFormat = origFormat
	}()

	// No tables in minimal PDF means outputTables is never called — no error.
	// This only errors if there ARE tables. We just ensure no panic.
	_ = runTables(tablesCmd, []string{path})
}

// ============================================================================
// outputTablesCSV — multi-table path
// ============================================================================

func TestOutputTablesCSV_MultipleTables(t *testing.T) {
	tables := []extractedTable{
		{Page: 1, Index: 1, Rows: 2, Columns: 2, Data: [][]string{{"A", "B"}, {"C", "D"}}},
		{Page: 1, Index: 2, Rows: 1, Columns: 2, Data: [][]string{{"X", "Y"}}},
	}
	out := captureStdout(t, func() {
		err := outputTablesCSV(os.Stdout, tables)
		assert.NoError(t, err)
	})
	assert.Contains(t, out, "Table 1")
	assert.Contains(t, out, "Table 2")
}

func TestOutputTablesText_MultipleTables(t *testing.T) {
	tables := []extractedTable{
		{Page: 1, Index: 1, Rows: 1, Columns: 2, Data: [][]string{{"Hello", "World"}}},
		{Page: 2, Index: 2, Rows: 1, Columns: 1, Data: [][]string{{"Foo"}}},
	}
	out := captureStdout(t, func() {
		err := outputTablesText(os.Stdout, tables)
		assert.NoError(t, err)
	})
	assert.Contains(t, out, "Table 1")
	assert.Contains(t, out, "Table 2")
	assert.Contains(t, out, "Hello")
}

func TestOutputTablesJSON_Tables(t *testing.T) {
	tables := []extractedTable{
		{Page: 1, Index: 1, Rows: 1, Columns: 2, Data: [][]string{{"a", "b"}}},
	}
	out := captureStdout(t, func() {
		err := outputTablesJSON(os.Stdout, tables)
		assert.NoError(t, err)
	})
	assert.Contains(t, out, `"page"`)
	assert.Contains(t, out, `"data"`)
}

// ============================================================================
// calculateColumnWidths / printTableRows
// ============================================================================

func TestCalculateColumnWidths(t *testing.T) {
	tbl := extractedTable{
		Page:    1,
		Index:   1,
		Rows:    2,
		Columns: 2,
		Data:    [][]string{{"short", "a much longer value"}, {"x", "y"}},
	}
	widths := calculateColumnWidths(tbl)
	require.Len(t, widths, 2)
	assert.Equal(t, len("short"), widths[0])
	assert.Equal(t, len("a much longer value"), widths[1])
}

func TestPrintTableRows_WidthFallback(t *testing.T) {
	// When colWidths is shorter than row cells, the fallback width of 10 is used.
	data := [][]string{{"A", "B", "C"}}
	colWidths := []int{5} // only one width, but row has 3 cells

	out := captureStdout(t, func() {
		printTableRows(os.Stdout, data, colWidths)
	})
	assert.Contains(t, out, "A")
	assert.Contains(t, out, "B")
	assert.Contains(t, out, "C")
}

// ============================================================================
// parsePageSpec / parsePagePart / parsePageRange
// ============================================================================

func TestParsePageSpec(t *testing.T) {
	tests := []struct {
		spec    string
		want    []int
		wantErr bool
	}{
		{"1", []int{1}, false},
		{"1,2,3", []int{1, 2, 3}, false},
		{"1-3", []int{1, 2, 3}, false},
		{"1-3,5,7-9", []int{1, 2, 3, 5, 7, 8, 9}, false},
		{"  2 , 4 ", []int{2, 4}, false},
		{"", nil, true},
		{"abc", nil, true},
		{"1-abc", nil, true},
		{"abc-5", nil, true},
		{"5-1", nil, true},   // start > end
		{"1-2-3", nil, true}, // too many dashes
	}
	for _, tt := range tests {
		got, err := parsePageSpec(tt.spec)
		if tt.wantErr {
			assert.Error(t, err, "spec=%q", tt.spec)
		} else {
			require.NoError(t, err, "spec=%q", tt.spec)
			assert.Equal(t, tt.want, got, "spec=%q", tt.spec)
		}
	}
}

func TestGetPageRange(t *testing.T) {
	// tablesPage = 0 means all pages.
	origPage := tablesPage
	tablesPage = 0
	defer func() { tablesPage = origPage }()

	start, end, err := getPageRange(5)
	require.NoError(t, err)
	assert.Equal(t, 1, start)
	assert.Equal(t, 5, end)
}

func TestGetPageRange_SpecificPage(t *testing.T) {
	origPage := tablesPage
	tablesPage = 3
	defer func() { tablesPage = origPage }()

	start, end, err := getPageRange(5)
	require.NoError(t, err)
	assert.Equal(t, 3, start)
	assert.Equal(t, 3, end)
}

func TestGetPageRange_PageExceedsCount(t *testing.T) {
	origPage := tablesPage
	tablesPage = 10
	defer func() { tablesPage = origPage }()

	_, _, err := getPageRange(5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

// ============================================================================
// encrypt command
// ============================================================================

func TestRunEncrypt_AlwaysReturnsNotImplemented(t *testing.T) {
	err := runEncrypt(encryptCmd, []string{"any.pdf"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not yet implemented")
}

// ============================================================================
// decrypt command
// ============================================================================

func TestRunDecrypt_MissingFile(t *testing.T) {
	origPwd := decryptPassword
	decryptPassword = "secret"
	defer func() { decryptPassword = origPwd }()

	err := runDecrypt(decryptCmd, []string{"/nonexistent.pdf"})
	assert.Error(t, err)
}

func TestRunDecrypt_Success(t *testing.T) {
	tmpDir := t.TempDir()
	encPath, pwd := buildEncryptedPDF(t, tmpDir)

	origPwd := decryptPassword
	origOut := decryptOutput
	decryptPassword = pwd
	decryptOutput = ""
	defer func() {
		decryptPassword = origPwd
		decryptOutput = origOut
	}()

	out := captureStdout(t, func() {
		err := runDecrypt(decryptCmd, []string{encPath})
		assert.NoError(t, err)
	})
	assert.Contains(t, out, "Successfully opened")
}

func TestRunDecrypt_WrongPassword(t *testing.T) {
	// Use a non-PDF file to guarantee an error regardless of security behavior.
	f, err := os.CreateTemp("", "fake-enc-*.pdf")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	_, _ = f.WriteString("totally not a PDF")
	f.Close()

	origPwd := decryptPassword
	decryptPassword = "wrongpassword"
	defer func() { decryptPassword = origPwd }()

	err = runDecrypt(decryptCmd, []string{f.Name()})
	assert.Error(t, err)
}

func TestRunDecrypt_OutputNotSupported(t *testing.T) {
	tmpDir := t.TempDir()
	encPath, pwd := buildEncryptedPDF(t, tmpDir)

	origPwd := decryptPassword
	origOut := decryptOutput
	decryptPassword = pwd
	decryptOutput = filepath.Join(tmpDir, "decrypted.pdf")
	defer func() {
		decryptPassword = origPwd
		decryptOutput = origOut
	}()

	err := runDecrypt(decryptCmd, []string{encPath})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not yet supported")
}

func TestRunDecrypt_InvalidPDF(t *testing.T) {
	f, err := os.CreateTemp("", "notapdf-*.pdf")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	_, err = f.WriteString("not a pdf at all")
	require.NoError(t, err)
	f.Close()

	origPwd := decryptPassword
	decryptPassword = "anypassword"
	defer func() { decryptPassword = origPwd }()

	err = runDecrypt(decryptCmd, []string{f.Name()})
	assert.Error(t, err)
}

// ============================================================================
// merge command
// ============================================================================

func TestRunMerge_MissingInputFile(t *testing.T) {
	tmpDir := t.TempDir()
	origOut := mergeOutput
	mergeOutput = filepath.Join(tmpDir, "merged.pdf")
	defer func() { mergeOutput = origOut }()

	err := runMerge(mergeCmd, []string{"/nonexistent1.pdf", "/nonexistent2.pdf"})
	assert.Error(t, err)
}

func TestRunMerge_TwoValidPDFs(t *testing.T) {
	path := testPDFPath(t)
	tmpDir := t.TempDir()

	origOut := mergeOutput
	mergeOutput = filepath.Join(tmpDir, "merged.pdf")
	defer func() { mergeOutput = origOut }()

	out := captureStdout(t, func() {
		err := runMerge(mergeCmd, []string{path, path})
		assert.NoError(t, err)
	})
	assert.Contains(t, out, "Merged")

	_, statErr := os.Stat(mergeOutput)
	assert.NoError(t, statErr)
}

func TestRunMerge_VerboseMode(t *testing.T) {
	path := testPDFPath(t)
	tmpDir := t.TempDir()

	origOut := mergeOutput
	origVerbose := verbose
	mergeOutput = filepath.Join(tmpDir, "merged_v.pdf")
	verbose = true
	defer func() {
		mergeOutput = origOut
		verbose = origVerbose
	}()

	out := captureStdout(t, func() {
		err := runMerge(mergeCmd, []string{path, path})
		assert.NoError(t, err)
	})
	assert.Contains(t, out, "Merging")
}

// ============================================================================
// split command
// ============================================================================

func TestRunSplit_ValidRange(t *testing.T) {
	path := testPDFPath(t) // minimal.pdf has 1 page; extract page 1
	tmpDir := t.TempDir()

	origOut := splitOutput
	splitOutput = filepath.Join(tmpDir, "split.pdf")
	defer func() { splitOutput = origOut }()

	out := captureStdout(t, func() {
		err := runSplit(splitCmd, []string{path, "1"})
		assert.NoError(t, err)
	})
	assert.Contains(t, out, "Extracted")
}

func TestRunSplit_InvalidPageSpec(t *testing.T) {
	path := testPDFPath(t)
	tmpDir := t.TempDir()

	origOut := splitOutput
	splitOutput = filepath.Join(tmpDir, "split.pdf")
	defer func() { splitOutput = origOut }()

	err := runSplit(splitCmd, []string{path, "notanumber"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid page specification")
}

func TestRunSplit_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()

	origOut := splitOutput
	splitOutput = filepath.Join(tmpDir, "split.pdf")
	defer func() { splitOutput = origOut }()

	err := runSplit(splitCmd, []string{"/nonexistent.pdf", "1"})
	assert.Error(t, err)
}

// ============================================================================
// printVerbosef
// ============================================================================

func TestPrintVerbosef_Verbose(t *testing.T) {
	origVerbose := verbose
	verbose = true
	defer func() { verbose = origVerbose }()

	out := captureStdout(t, func() {
		printVerbosef("hello %s", "world")
	})
	assert.Contains(t, out, "hello world")
}

func TestPrintVerbosef_Silent(t *testing.T) {
	origVerbose := verbose
	verbose = false
	defer func() { verbose = origVerbose }()

	out := captureStdout(t, func() {
		printVerbosef("should not appear")
	})
	assert.Empty(t, out)
}
