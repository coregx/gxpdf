package extractor

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/coregx/gxpdf/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

// makeFlateStream builds a *parser.Stream whose content is zlib-compressed data
// and whose dictionary has Filter=/FlateDecode.
func makeFlateStream(t *testing.T, raw []byte) *parser.Stream {
	t.Helper()
	compressed := zlibCompress(t, raw)
	dict := parser.NewDictionary()
	dict.Set("Filter", parser.NewName("FlateDecode"))
	dict.Set("Length", parser.NewInteger(int64(len(compressed))))
	return parser.NewStream(dict, compressed)
}

// makeRawStream builds a *parser.Stream with no Filter (raw content).
func makeRawStream(content []byte) *parser.Stream {
	dict := parser.NewDictionary()
	return parser.NewStream(dict, content)
}

// zlibCompress is a test helper to compress bytes with zlib.
func zlibCompress(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	_, err := w.Write(data)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes()
}

// ─── decodeStreamData ─────────────────────────────────────────────────────────

func TestDecodeStreamData_NoFilter(t *testing.T) {
	want := []byte("hello world")
	stream := makeRawStream(want)

	got, err := decodeStreamData(stream)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestDecodeStreamData_FlateDecode(t *testing.T) {
	want := []byte("The quick brown fox jumps over the lazy dog")
	stream := makeFlateStream(t, want)

	got, err := decodeStreamData(stream)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestDecodeStreamData_UnsupportedFilterReturnsError(t *testing.T) {
	dict := parser.NewDictionary()
	dict.Set("Filter", parser.NewName("CCITTFaxDecode"))
	stream := parser.NewStream(dict, []byte("compressed data"))

	_, err := decodeStreamData(stream)
	require.Error(t, err)
	assert.ErrorContains(t, err, "unsupported filter")
}

func TestDecodeStreamData_FilterArray_FirstFilter(t *testing.T) {
	want := []byte("array filter test data")
	compressed := zlibCompress(t, want)

	arr := parser.NewArray()
	arr.Append(parser.NewName("FlateDecode"))
	dict := parser.NewDictionary()
	dict.Set("Filter", arr)
	stream := parser.NewStream(dict, compressed)

	got, err := decodeStreamData(stream)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestDecodeStreamData_InvalidZlib_ReturnsError(t *testing.T) {
	dict := parser.NewDictionary()
	dict.Set("Filter", parser.NewName("FlateDecode"))
	stream := parser.NewStream(dict, []byte("not valid zlib data at all"))

	_, err := decodeStreamData(stream)
	assert.Error(t, err)
}

// ─── EmbeddedFont struct ──────────────────────────────────────────────────────

func TestEmbeddedFont_Fields(t *testing.T) {
	ef := EmbeddedFont{
		Name:     "DejaVuSans",
		Subtype:  "TrueType",
		Data:     []byte{0x00, 0x01, 0x00, 0x00},
		Encoding: "WinAnsiEncoding",
	}
	assert.Equal(t, "DejaVuSans", ef.Name)
	assert.Equal(t, "TrueType", ef.Subtype)
	assert.NotEmpty(t, ef.Data)
	assert.Equal(t, "WinAnsiEncoding", ef.Encoding)
}

// ─── FontExtractor helper method unit tests ───────────────────────────────────

func TestFontExtractor_NameValue_Name(t *testing.T) {
	fe := &FontExtractor{}
	assert.Equal(t, "TrueType", fe.nameValue(parser.NewName("TrueType")))
}

func TestFontExtractor_NameValue_NonName(t *testing.T) {
	fe := &FontExtractor{}
	assert.Equal(t, "", fe.nameValue(parser.NewInteger(1)))
}

func TestFontExtractor_NameValue_Nil(t *testing.T) {
	fe := &FontExtractor{}
	assert.Equal(t, "", fe.nameValue(nil))
}

func TestFontExtractor_NameOrStringValue_String(t *testing.T) {
	fe := &FontExtractor{}
	assert.Equal(t, "Arial-Bold", fe.nameOrStringValue(parser.NewString("Arial-Bold")))
}

func TestFontExtractor_NameOrStringValue_Name(t *testing.T) {
	fe := &FontExtractor{}
	assert.Equal(t, "Arial-Bold", fe.nameOrStringValue(parser.NewName("Arial-Bold")))
}

func TestFontExtractor_NameOrStringValue_Other(t *testing.T) {
	fe := &FontExtractor{}
	assert.Equal(t, "", fe.nameOrStringValue(parser.NewBoolean(true)))
}

func TestFontExtractor_EncodingValue_Name(t *testing.T) {
	fe := &FontExtractor{reader: nil}
	assert.Equal(t, "WinAnsiEncoding", fe.encodingValue(parser.NewName("WinAnsiEncoding")))
}

func TestFontExtractor_EncodingValue_DictWithBaseEncoding(t *testing.T) {
	fe := &FontExtractor{reader: nil}
	dict := parser.NewDictionary()
	dict.Set("BaseEncoding", parser.NewName("MacRomanEncoding"))
	assert.Equal(t, "MacRomanEncoding", fe.encodingValue(dict))
}

func TestFontExtractor_EncodingValue_DictNoBaseEncoding(t *testing.T) {
	fe := &FontExtractor{reader: nil}
	dict := parser.NewDictionary()
	assert.Equal(t, "", fe.encodingValue(dict))
}

func TestFontExtractor_EncodingValue_Nil(t *testing.T) {
	fe := &FontExtractor{reader: nil}
	assert.Equal(t, "", fe.encodingValue(nil))
}

// ─── ErrUnsupportedFontType ───────────────────────────────────────────────────

func TestErrUnsupportedFontType_NotNil(t *testing.T) {
	assert.Error(t, ErrUnsupportedFontType)
	assert.Contains(t, ErrUnsupportedFontType.Error(), "unsupported")
}

// ─── round-trip integration test ─────────────────────────────────────────────

// TestFontExtractor_RoundTrip_TrueType creates a minimal PDF with a /FontFile2
// stream, writes it to a temp file, parses it, and verifies that the FontExtractor
// finds and correctly decodes the embedded font data.
func TestFontExtractor_RoundTrip_TrueType(t *testing.T) {
	// Build synthetic "font data" — we are testing the extraction path, not
	// that the bytes are a valid TTF. Use a recognizable sentinel.
	sentinel := []byte("FAKE_TTF_BYTES_FOR_ROUNDTRIP_TEST")
	pdfBytes := buildMinimalPDFWithTrueTypeFont(t, sentinel)

	// Write to temp file so parser.OpenPDF can read it.
	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "roundtrip.pdf")
	require.NoError(t, os.WriteFile(pdfPath, pdfBytes, 0600))

	rd, err := parser.OpenPDF(pdfPath)
	require.NoError(t, err)
	defer func() { _ = rd.Close() }()

	fe := NewFontExtractor(rd)

	// Test ExtractFromPage (page 0).
	pageFonts, err := fe.ExtractFromPage(0)
	require.NoError(t, err)
	require.Len(t, pageFonts, 1, "expected exactly 1 embedded font on page 0")

	ef := pageFonts[0]
	assert.Equal(t, "FakeFont", ef.Name)
	assert.Equal(t, "TrueType", ef.Subtype)
	assert.Equal(t, "WinAnsiEncoding", ef.Encoding)
	assert.Equal(t, sentinel, ef.Data, "extracted font data must match original")

	// Test ExtractFromDocument (deduplication).
	allFonts, err := fe.ExtractFromDocument()
	require.NoError(t, err)
	require.Len(t, allFonts, 1, "expected exactly 1 unique embedded font in document")
}

// TestFontExtractor_NoEmbeddedFonts_ReturnsEmpty verifies that a PDF with no
// font resources returns an empty slice, not an error.
func TestFontExtractor_NoEmbeddedFonts_ReturnsEmpty(t *testing.T) {
	pdfBytes := buildMinimalPDFNoFonts(t)

	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "nofont.pdf")
	require.NoError(t, os.WriteFile(pdfPath, pdfBytes, 0600))

	rd, err := parser.OpenPDF(pdfPath)
	require.NoError(t, err)
	defer func() { _ = rd.Close() }()

	fe := NewFontExtractor(rd)

	pageFonts, err := fe.ExtractFromPage(0)
	require.NoError(t, err)
	assert.Empty(t, pageFonts, "should be empty, not error, when no fonts present")

	allFonts, err := fe.ExtractFromDocument()
	require.NoError(t, err)
	assert.Empty(t, allFonts)
}

// TestFontExtractor_OutOfRangePage verifies that an invalid page number returns error.
func TestFontExtractor_OutOfRangePage(t *testing.T) {
	pdfBytes := buildMinimalPDFNoFonts(t)

	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "oob.pdf")
	require.NoError(t, os.WriteFile(pdfPath, pdfBytes, 0600))

	rd, err := parser.OpenPDF(pdfPath)
	require.NoError(t, err)
	defer func() { _ = rd.Close() }()

	fe := NewFontExtractor(rd)
	_, err = fe.ExtractFromPage(99) // page 99 doesn't exist in a 1-page PDF
	assert.Error(t, err)
}

// ─── PDF builders ─────────────────────────────────────────────────────────────

// buildMinimalPDFWithTrueTypeFont creates a minimal valid PDF in which:
//   - Page 0 has a /Font resource F1 of /Subtype /TrueType
//   - F1 has a /FontDescriptor with /FontFile2 pointing to a stream
//   - The stream contains zlib-compressed fontData
func buildMinimalPDFWithTrueTypeFont(t *testing.T, fontData []byte) []byte {
	t.Helper()
	compressed := zlibCompress(t, fontData)
	return buildRawPDF(t, compressed, true)
}

// buildMinimalPDFNoFonts creates a minimal valid 1-page PDF with no font resources.
func buildMinimalPDFNoFonts(t *testing.T) []byte {
	t.Helper()
	return buildRawPDF(t, nil, false)
}

// buildRawPDF constructs a syntactically valid minimal PDF.
// If withFont is true it embeds a TrueType font with the given compressed bytes.
// Object numbering:
//
//	1 = Catalog, 2 = Pages, 3 = Page,
//	4 = Font (if withFont), 5 = FontDescriptor (if withFont), 6 = FontFile2 stream (if withFont)
//
//nolint:funlen // PDF construction is inherently verbose
func buildRawPDF(t *testing.T, compressedFont []byte, withFont bool) []byte {
	t.Helper()
	var b bytes.Buffer

	b.WriteString("%PDF-1.7\n")

	offsets := make([]int64, 10) // 1-based

	// Object 1: Catalog
	offsets[1] = int64(b.Len())
	b.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	// Object 2: Pages
	offsets[2] = int64(b.Len())
	b.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	// Object 3: Page
	offsets[3] = int64(b.Len())
	if withFont {
		b.WriteString("3 0 obj\n<< /Type /Page /Parent 2 0 R\n")
		b.WriteString("   /MediaBox [0 0 612 792]\n")
		b.WriteString("   /Resources << /Font << /F1 4 0 R >> >>\n")
		b.WriteString(">>\nendobj\n")
	} else {
		b.WriteString("3 0 obj\n<< /Type /Page /Parent 2 0 R\n")
		b.WriteString("   /MediaBox [0 0 612 792]\n")
		b.WriteString(">>\nendobj\n")
	}

	numObjs := 3
	if withFont {
		// Object 4: TrueType Font dict
		offsets[4] = int64(b.Len())
		b.WriteString("4 0 obj\n")
		b.WriteString("<< /Type /Font /Subtype /TrueType\n")
		b.WriteString("   /BaseFont /FakeFont\n")
		b.WriteString("   /Encoding /WinAnsiEncoding\n")
		b.WriteString("   /FontDescriptor 5 0 R\n")
		b.WriteString(">>\nendobj\n")

		// Object 5: FontDescriptor
		offsets[5] = int64(b.Len())
		b.WriteString("5 0 obj\n")
		b.WriteString("<< /Type /FontDescriptor\n")
		b.WriteString("   /FontName /FakeFont\n")
		b.WriteString("   /Flags 32\n")
		b.WriteString("   /FontBBox [0 -200 1000 800]\n")
		b.WriteString("   /ItalicAngle 0\n")
		b.WriteString("   /Ascent 800\n")
		b.WriteString("   /Descent -200\n")
		b.WriteString("   /CapHeight 700\n")
		b.WriteString("   /StemV 80\n")
		b.WriteString("   /FontFile2 6 0 R\n")
		b.WriteString(">>\nendobj\n")

		// Object 6: FontFile2 stream
		offsets[6] = int64(b.Len())
		b.WriteString("6 0 obj\n")
		b.WriteString(fmt.Sprintf("<< /Filter /FlateDecode /Length1 %d /Length %d >>\n",
			len(compressedFont), len(compressedFont)))
		b.WriteString("stream\n")
		b.Write(compressedFont)
		b.WriteString("\nendstream\nendobj\n")

		numObjs = 6
	}

	// Cross-reference table
	xrefOffset := int64(b.Len())
	b.WriteString("xref\n")
	b.WriteString(fmt.Sprintf("0 %d\n", numObjs+1))
	b.WriteString("0000000000 65535 f \n")
	for i := 1; i <= numObjs; i++ {
		b.WriteString(fmt.Sprintf("%010d 00000 n \n", offsets[i]))
	}

	// Trailer
	b.WriteString("trailer\n")
	b.WriteString(fmt.Sprintf("<< /Size %d /Root 1 0 R >>\n", numObjs+1))
	b.WriteString("startxref\n")
	b.WriteString(fmt.Sprintf("%d\n", xrefOffset))
	b.WriteString("%%EOF\n")

	return b.Bytes()
}
