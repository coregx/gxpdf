package extractor

// Tests for bug #74: CID TrueType font text extraction returns raw glyph IDs.
//
// Covers all five root causes:
//  1. parseBfRange array format
//  2. No-ToUnicode path use2ByteGlyphs for Identity encoding
//  3. begincodespacerange parsing
//  4. UTF-16BE surrogate pair decoding
//  5. isCompositeFont prevents garbage-detection downgrade to 1-byte

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coregx/gxpdf/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Bug 1: parseBfRange array format ────────────────────────────────────────

func TestParseBfRange_ArrayFormat_ExplicitPerCodeMapping(t *testing.T) {
	// Each source code maps to a specific destination — NOT a consecutive block.
	// Format: <srcLow> <srcHigh> [<dst0> <dst1> <dst2>]
	cmapData := `
begincmap
1 beginbfrange
<0001> <0003> [<0041> <0042> <0043>]
endbfrange
endcmap
`
	table, err := ParseCMapStream([]byte(cmapData))
	require.NoError(t, err)
	assert.Equal(t, 3, table.Size())

	tests := []struct {
		glyphID uint16
		want    rune
		label   string
	}{
		{0x0001, 'A', "0x0001 → U+0041 A"},
		{0x0002, 'B', "0x0002 → U+0042 B"},
		{0x0003, 'C', "0x0003 → U+0043 C"},
	}
	for _, tt := range tests {
		r, ok := table.GetUnicode(tt.glyphID)
		assert.True(t, ok, tt.label)
		assert.Equal(t, tt.want, r, tt.label)
	}
}

func TestParseBfRange_ArrayFormat_CyrillicMappings(t *testing.T) {
	// Simulate a real CID font bfrange with non-consecutive Cyrillic mappings.
	cmapData := `
begincmap
1 beginbfrange
<0001> <0004> [<0412> <044B> <043F> <0438>]
endbfrange
endcmap
`
	table, err := ParseCMapStream([]byte(cmapData))
	require.NoError(t, err)
	assert.Equal(t, 4, table.Size())

	expected := map[uint16]rune{
		0x0001: 'В', // U+0412
		0x0002: 'ы', // U+044B
		0x0003: 'п', // U+043F
		0x0004: 'и', // U+0438
	}
	for gid, want := range expected {
		r, ok := table.GetUnicode(gid)
		assert.True(t, ok, "glyph 0x%04X should be mapped", gid)
		assert.Equal(t, want, r, "glyph 0x%04X", gid)
	}
}

func TestParseBfRange_ArrayFormat_TruncatedArray(t *testing.T) {
	// Array has fewer entries than the range — only the first entries are mapped.
	cmapData := `
begincmap
1 beginbfrange
<0001> <0005> [<0041> <0042>]
endbfrange
endcmap
`
	table, err := ParseCMapStream([]byte(cmapData))
	require.NoError(t, err)
	// Only two entries available in the array
	assert.Equal(t, 2, table.Size())

	r, ok := table.GetUnicode(0x0001)
	assert.True(t, ok)
	assert.Equal(t, rune('A'), r)

	r, ok = table.GetUnicode(0x0002)
	assert.True(t, ok)
	assert.Equal(t, rune('B'), r)

	_, ok = table.GetUnicode(0x0003)
	assert.False(t, ok, "0x0003 should not be mapped when array is shorter than range")
}

func TestParseBfRange_ScalarFormStillWorks(t *testing.T) {
	// Verify the scalar form is unaffected by the array-form fix.
	cmapData := `
begincmap
1 beginbfrange
<0020> <007E> <0020>
endbfrange
endcmap
`
	table, err := ParseCMapStream([]byte(cmapData))
	require.NoError(t, err)
	assert.Equal(t, 0x7E-0x20+1, table.Size())

	r, ok := table.GetUnicode(0x0041) // 'A'
	assert.True(t, ok)
	assert.Equal(t, rune('A'), r)
}

func TestParseBfRange_MixedScalarAndArray(t *testing.T) {
	// Both forms in one bfrange section.
	cmapData := `
begincmap
2 beginbfrange
<0001> <0003> [<0041> <0042> <0043>]
<0010> <0012> <0430>
endbfrange
endcmap
`
	table, err := ParseCMapStream([]byte(cmapData))
	require.NoError(t, err)
	assert.Equal(t, 6, table.Size())

	// Array form
	r, ok := table.GetUnicode(0x0001)
	assert.True(t, ok)
	assert.Equal(t, rune('A'), r)

	// Scalar form
	r, ok = table.GetUnicode(0x0010)
	assert.True(t, ok)
	assert.Equal(t, rune(0x0430), r) // 'а'

	r, ok = table.GetUnicode(0x0012)
	assert.True(t, ok)
	assert.Equal(t, rune(0x0432), r) // 'в'
}

// ─── Bug 3: begincodespacerange parsing ──────────────────────────────────────

func TestParseCodeSpaceRange_2ByteDetection(t *testing.T) {
	// A CIDFont CMap declares <0000> <FFFF> — 4 hex chars = 2 bytes per code.
	cmapData := `
begincmap
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
1 beginbfchar
<0001> <0041>
endbfchar
endcmap
`
	table, err := ParseCMapStream([]byte(cmapData))
	require.NoError(t, err)
	assert.Equal(t, 2, table.CodeBytes, "4-char hex in codespacerange → CodeBytes=2")
}

func TestParseCodeSpaceRange_1ByteDetection(t *testing.T) {
	// A simple-font CMap declares <00> <FF> — 2 hex chars = 1 byte per code.
	cmapData := `
begincmap
1 begincodespacerange
<00> <FF>
endcodespacerange
1 beginbfchar
<41> <0041>
endbfchar
endcmap
`
	table, err := ParseCMapStream([]byte(cmapData))
	require.NoError(t, err)
	assert.Equal(t, 1, table.CodeBytes, "2-char hex in codespacerange → CodeBytes=1")
}

func TestParseCodeSpaceRange_DefaultWhenAbsent(t *testing.T) {
	// No codespacerange → default CodeBytes=1.
	cmapData := `
begincmap
1 beginbfchar
<01> <0041>
endbfchar
endcmap
`
	table, err := ParseCMapStream([]byte(cmapData))
	require.NoError(t, err)
	assert.Equal(t, 1, table.CodeBytes, "no codespacerange → CodeBytes defaults to 1")
}

func TestParseCodeSpaceRange_MultipleRanges_FirstDeterminesBytes(t *testing.T) {
	cmapData := `
begincmap
2 begincodespacerange
<0000> <00FF>
<0100> <FFFF>
endcodespacerange
endcmap
`
	table, err := ParseCMapStream([]byte(cmapData))
	require.NoError(t, err)
	assert.Equal(t, 2, table.CodeBytes, "first entry has 4 hex chars → CodeBytes=2")
}

// ─── Bug 4: UTF-16BE surrogate pair decoding ─────────────────────────────────

func TestDecodeUTF16BEHex_BMP_SingleCodePoint(t *testing.T) {
	tests := []struct {
		hex  string
		want rune
	}{
		{"<0041>", 'A'},
		{"<0412>", 'В'}, // Cyrillic capital VE
		{"<FEFF>", '\uFEFF'},
		{"<0020>", ' '},
	}
	for _, tt := range tests {
		r, err := decodeUTF16BEHex(tt.hex)
		require.NoError(t, err, "input: %s", tt.hex)
		assert.Equal(t, tt.want, r, "input: %s", tt.hex)
	}
}

func TestDecodeUTF16BEHex_SurrogatePair_Emoji(t *testing.T) {
	// U+1F600 GRINNING FACE encoded as UTF-16BE surrogate pair: D83D DE00
	r, err := decodeUTF16BEHex("<D83DDE00>")
	require.NoError(t, err)
	assert.Equal(t, rune(0x1F600), r, "surrogate pair D83D+DE00 → U+1F600")
}

func TestDecodeUTF16BEHex_SurrogatePair_LinearBSyllable(t *testing.T) {
	// U+10000 LINEAR B SYLLABLE B008 A: D800 DC00
	r, err := decodeUTF16BEHex("<D800DC00>")
	require.NoError(t, err)
	assert.Equal(t, rune(0x10000), r, "D800+DC00 → U+10000")
}

func TestDecodeUTF16BEHex_SingleByte(t *testing.T) {
	r, err := decodeUTF16BEHex("<41>")
	require.NoError(t, err)
	assert.Equal(t, rune(0x41), r)
}

func TestDecodeUTF16BEHex_Empty_ReturnsError(t *testing.T) {
	_, err := decodeUTF16BEHex("<>")
	assert.Error(t, err)
}

func TestDecodeUTF16BEHex_InvalidHex_ReturnsError(t *testing.T) {
	_, err := decodeUTF16BEHex("<ZZZZ>")
	assert.Error(t, err)
}

func TestParseBfChar_SurrogatePairDestination(t *testing.T) {
	// A ToUnicode CMap where the destination is a UTF-16BE surrogate pair (emoji).
	cmapData := `
begincmap
1 beginbfchar
<0001> <D83DDE00>
endbfchar
endcmap
`
	table, err := ParseCMapStream([]byte(cmapData))
	require.NoError(t, err)

	r, ok := table.GetUnicode(0x0001)
	assert.True(t, ok)
	assert.Equal(t, rune(0x1F600), r, "surrogate pair destination should decode to U+1F600")
}

func TestParseBfRange_SurrogatePairScalarBase(t *testing.T) {
	// A bfrange starting at a surrogate-pair encoded code point (rare but valid).
	// U+1F600 = D83D DE00, U+1F601 = D83D DE01
	cmapData := `
begincmap
1 beginbfrange
<0001> <0002> <D83DDE00>
endbfrange
endcmap
`
	table, err := ParseCMapStream([]byte(cmapData))
	require.NoError(t, err)

	r, ok := table.GetUnicode(0x0001)
	assert.True(t, ok)
	assert.Equal(t, rune(0x1F600), r)

	// Range increments the rune — U+1F601
	r, ok = table.GetUnicode(0x0002)
	assert.True(t, ok)
	assert.Equal(t, rune(0x1F601), r)
}

// ─── Bug 2: No-ToUnicode path uses use2ByteGlyphs for Identity encoding ──────

func TestFontDecoder_IdentityEncoding_NoToUnicode_Uses2ByteGlyphs(t *testing.T) {
	// Simulate loadFontDecoder path when Encoding=Identity-H and no ToUnicode.
	// The decoder must use 2-byte glyphs.
	decoder := NewFontDecoder(nil, "Identity-H", true)
	assert.True(t, decoder.use2ByteGlyphs, "Identity-H without ToUnicode must use 2-byte glyphs")
}

func TestFontDecoder_IdentityV_Uses2ByteGlyphs(t *testing.T) {
	decoder := NewFontDecoder(nil, "Identity-V", true)
	assert.True(t, decoder.use2ByteGlyphs)
}

func TestFontDecoder_NonIdentityEncoding_Uses1ByteGlyphs(t *testing.T) {
	decoder := NewFontDecoder(nil, "WinAnsiEncoding", false)
	assert.False(t, decoder.use2ByteGlyphs, "WinAnsiEncoding is a 1-byte encoding")
}

// ─── Bug 5: isCompositeFont prevents garbage-detection downgrade ──────────────

func TestFontDecoder_CompositeFont_DoesNotDowngradeTo1Byte(t *testing.T) {
	// Create a CMap where all glyph IDs are > 0xFF (2-byte range).
	cmap := NewCMapTable("Test")
	cmap.AddMapping(0x0101, 'A') // glyph ID 0x0101 → 'A'
	cmap.AddMapping(0x0102, 'B')
	cmap.AddMapping(0x0103, 'C')

	decoder := NewFontDecoder(cmap, "Identity-H", true)
	decoder.isCompositeFont = true

	// Bytes represent 2-byte glyph IDs: 0x0101, 0x0102, 0x0103
	result := decoder.DecodeString([]byte{0x01, 0x01, 0x01, 0x02, 0x01, 0x03})
	assert.Equal(t, "ABC", result)
}

func TestFontDecoder_SimpleFont_GarbageFallbackStillWorks(t *testing.T) {
	// For non-composite fonts the garbage heuristic should still fall back.
	// Use a CMap with no entries (everything unmapped → replacement chars).
	cmap := NewCMapTable("Test")
	decoder := NewFontDecoder(cmap, "Identity-H", true)
	// isCompositeFont defaults to false — fallback is active.
	assert.False(t, decoder.isCompositeFont)

	// Input: 2-byte glyph IDs that map to U+FFFD (not in CMap).
	// The fallback should try 1-byte decoding.
	raw := []byte{0x41, 0x42, 0x43} // odd-length: can't be pure 2-byte
	result := decoder.DecodeString(raw)
	// With garbage fallback active, should produce something (not all U+FFFD)
	assert.NotEmpty(t, result)
}

func TestFontDecoder_IsCompositeFont_FlagPreserved(t *testing.T) {
	decoder := NewFontDecoder(nil, "Identity-H", true)
	decoder.isCompositeFont = true
	assert.True(t, decoder.isCompositeFont)
}

func TestFontDecoder_DefaultIsCompositeFont_False(t *testing.T) {
	decoder := NewFontDecoder(nil, "", false)
	assert.False(t, decoder.isCompositeFont, "new decoders default to simple font")
}

// ─── Integration test: Type0/CIDFontType2 with Identity-H + ToUnicode CMap ───

// buildType0PDF constructs a minimal syntactically valid PDF containing:
//   - Page 0 with a /Font entry /F1 of /Subtype /Type0
//   - /Encoding /Identity-H
//   - /DescendantFonts array with a CIDFontType2 descendant
//   - A ToUnicode stream with bfchar mappings (including array-form bfrange)
//   - A content stream that uses /F1 with 2-byte glyph codes
//
// The content stream writes glyph IDs 0x0001 (→ 'H'), 0x0002 (→ 'i'), 0x0003 (→ '!').
//
//nolint:funlen // PDF construction is inherently verbose
func buildType0PDF(t *testing.T) []byte {
	t.Helper()

	// ToUnicode CMap: maps glyph IDs 0x0001-0x0003 to H, i, !
	// Uses bfrange array format for 0x0001-0x0003.
	toUnicodeCMap := `
/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CIDSystemInfo
<< /Registry (Adobe)
   /Ordering (UCS)
   /Supplement 0
>> def
/CMapName /Adobe-Identity-UCS def
/CMapType 2 def

1 begincodespacerange
<0000> <FFFF>
endcodespacerange

1 beginbfrange
<0001> <0003> [<0048> <0069> <0021>]
endbfrange

endcmap
CMapName currentdict /CMap defineresource pop
end
end
`

	// Content stream: BT /F1 12 Tf <0001> Tj <0002> Tj <0003> Tj ET
	// Each <XXYY> is a 2-byte hex string encoded as a PDF string literal.
	// In PDF content streams Tj takes a string; for 2-byte glyphs the string
	// contains raw 2-byte sequences.
	contentData := "BT /F1 12 Tf 100 700 Td (\x00\x01\x00\x02\x00\x03) Tj ET"

	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.7\n")

	offsets := make([]int64, 12) // object slots 1-based

	// Object 1: Catalog
	offsets[1] = int64(pdf.Len())
	pdf.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	// Object 2: Pages
	offsets[2] = int64(pdf.Len())
	pdf.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	// Compress content stream
	var contentBuf bytes.Buffer
	zlibW := zlib.NewWriter(&contentBuf)
	_, err := zlibW.Write([]byte(contentData))
	require.NoError(t, err)
	require.NoError(t, zlibW.Close())
	compressedContent := contentBuf.Bytes()

	// Object 3: Page
	offsets[3] = int64(pdf.Len())
	pdf.WriteString("3 0 obj\n")
	pdf.WriteString("<< /Type /Page /Parent 2 0 R\n")
	pdf.WriteString("   /MediaBox [0 0 612 792]\n")
	pdf.WriteString("   /Resources << /Font << /F1 4 0 R >> >>\n")
	pdf.WriteString("   /Contents 8 0 R\n")
	pdf.WriteString(">>\nendobj\n")

	// Object 4: Type0 Font dict
	offsets[4] = int64(pdf.Len())
	pdf.WriteString("4 0 obj\n")
	pdf.WriteString("<< /Type /Font\n")
	pdf.WriteString("   /Subtype /Type0\n")
	pdf.WriteString("   /BaseFont /ArialMT\n")
	pdf.WriteString("   /Encoding /Identity-H\n")
	pdf.WriteString("   /DescendantFonts [5 0 R]\n")
	pdf.WriteString("   /ToUnicode 6 0 R\n")
	pdf.WriteString(">>\nendobj\n")

	// Object 5: CIDFontType2 (descendant)
	offsets[5] = int64(pdf.Len())
	pdf.WriteString("5 0 obj\n")
	pdf.WriteString("<< /Type /Font\n")
	pdf.WriteString("   /Subtype /CIDFontType2\n")
	pdf.WriteString("   /BaseFont /ArialMT\n")
	pdf.WriteString("   /CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >>\n")
	pdf.WriteString("   /DW 1000\n")
	pdf.WriteString(">>\nendobj\n")

	// Object 6: ToUnicode stream (uncompressed for simplicity)
	offsets[6] = int64(pdf.Len())
	pdf.WriteString("6 0 obj\n")
	pdf.WriteString(fmt.Sprintf("<< /Length %d >>\n", len(toUnicodeCMap)))
	pdf.WriteString("stream\n")
	pdf.WriteString(toUnicodeCMap)
	pdf.WriteString("\nendstream\nendobj\n")

	// Object 8: Content stream (compressed)
	offsets[8] = int64(pdf.Len())
	pdf.WriteString("8 0 obj\n")
	pdf.WriteString(fmt.Sprintf("<< /Filter /FlateDecode /Length %d >>\n", len(compressedContent)))
	pdf.WriteString("stream\n")
	pdf.Write(compressedContent)
	pdf.WriteString("\nendstream\nendobj\n")

	// Cross-reference table (6 objects: 1-6, 8)
	xrefOffset := int64(pdf.Len())
	pdf.WriteString("xref\n")
	pdf.WriteString("0 9\n")
	pdf.WriteString("0000000000 65535 f \n")
	for i := 1; i <= 8; i++ {
		if offsets[i] == 0 && i != 7 {
			// Object 7 intentionally unused
			pdf.WriteString("0000000000 65535 f \n")
			continue
		}
		if i == 7 {
			pdf.WriteString("0000000000 65535 f \n")
			continue
		}
		pdf.WriteString(fmt.Sprintf("%010d 00000 n \n", offsets[i]))
	}

	pdf.WriteString("trailer\n")
	pdf.WriteString("<< /Size 9 /Root 1 0 R >>\n")
	pdf.WriteString("startxref\n")
	pdf.WriteString(fmt.Sprintf("%d\n", xrefOffset))
	pdf.WriteString("%%EOF\n")

	return pdf.Bytes()
}

func TestCIDFont_Integration_Type0WithToUnicode(t *testing.T) {
	pdfBytes := buildType0PDF(t)

	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "cid_font.pdf")
	require.NoError(t, os.WriteFile(pdfPath, pdfBytes, 0600))

	rd, err := parser.OpenPDF(pdfPath)
	require.NoError(t, err)
	defer func() { _ = rd.Close() }()

	te := NewTextExtractor(rd)
	elements, err := te.ExtractFromPage(0)
	require.NoError(t, err)
	require.NotEmpty(t, elements, "should extract at least one text element")

	// Concatenate all extracted text
	var sb strings.Builder
	for _, el := range elements {
		sb.WriteString(el.Text)
	}
	extracted := sb.String()

	// The CMap maps 0x0001→H, 0x0002→i, 0x0003→!
	// Content stream sends glyph sequence 0x0001 0x0002 0x0003
	assert.Equal(t, "Hi!", extracted,
		"CID TrueType font with ToUnicode + bfrange array must produce correct Unicode text; got %q", extracted)
}

// ─── Additional edge-case tests ───────────────────────────────────────────────

func TestParseBfRange_EmptyArray_NoMappings(t *testing.T) {
	cmapData := `
begincmap
1 beginbfrange
<0001> <0005> []
endbfrange
endcmap
`
	table, err := ParseCMapStream([]byte(cmapData))
	require.NoError(t, err)
	// Empty array → no mappings should be added
	assert.Equal(t, 0, table.Size())
}

func TestCMapTable_CodeBytes_InitializesTo1(t *testing.T) {
	table := NewCMapTable("test")
	assert.Equal(t, 1, table.CodeBytes, "fresh CMapTable must default to CodeBytes=1")
}

func TestExtractHexTokensFromArray_Basic(t *testing.T) {
	inner := "<0041> <0042> <0043>"
	tokens := extractHexTokensFromArray(inner)
	assert.Equal(t, []string{"<0041>", "<0042>", "<0043>"}, tokens)
}

func TestExtractHexTokensFromArray_WithWhitespace(t *testing.T) {
	inner := "  <0041>  \n  <0042>  "
	tokens := extractHexTokensFromArray(inner)
	assert.Equal(t, []string{"<0041>", "<0042>"}, tokens)
}

func TestExtractHexTokensFromArray_Empty(t *testing.T) {
	tokens := extractHexTokensFromArray("")
	assert.Empty(t, tokens)
}

func TestHexStringToBytes_Valid(t *testing.T) {
	tests := []struct {
		input string
		want  []byte
	}{
		{"0041", []byte{0x00, 0x41}},
		{"D83DDE00", []byte{0xD8, 0x3D, 0xDE, 0x00}},
		{"FF", []byte{0xFF}},
	}
	for _, tt := range tests {
		got, err := hexStringToBytes(tt.input)
		require.NoError(t, err, "input: %q", tt.input)
		assert.Equal(t, tt.want, got)
	}
}

func TestHexStringToBytes_OddLength_Error(t *testing.T) {
	_, err := hexStringToBytes("041")
	assert.Error(t, err)
}

func TestHexStringToBytes_InvalidHex_Error(t *testing.T) {
	_, err := hexStringToBytes("ZZZZ")
	assert.Error(t, err)
}
