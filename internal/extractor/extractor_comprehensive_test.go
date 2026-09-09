package extractor

import (
	"bytes"
	"compress/zlib"
	"encoding/ascii85"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/coregx/gxpdf/internal/parser"
)

// ---------- Matrix additional tests (not covered by text_state_test.go) ----------

func TestMatrix_MultiplyIdentityResult(t *testing.T) {
	m := NewMatrix(2, 3, 4, 5, 6, 7)
	id := Identity()
	result := m.Multiply(id)
	if result.A != m.A || result.B != m.B || result.C != m.C ||
		result.D != m.D || result.E != m.E || result.F != m.F {
		t.Errorf("m × Identity should equal m, got %+v", result)
	}
}

func TestMatrix_IsIdentityFalse(t *testing.T) {
	m := NewMatrix(2, 0, 0, 1, 0, 0)
	if m.IsIdentity() {
		t.Error("Non-identity matrix should not pass IsIdentity()")
	}
}

// ---------- TextElement additional tests ----------

func TestTextElement_BoundsMethods(t *testing.T) {
	te := NewTextElement("Text", 10, 20, 50, 12, "F1", 12)
	if te.Left() != 10 {
		t.Errorf("Left() = %f, want 10", te.Left())
	}
	if te.Right() != 60 {
		t.Errorf("Right() = %f, want 60", te.Right())
	}
	if te.Bottom() != 20 {
		t.Errorf("Bottom() = %f, want 20", te.Bottom())
	}
	if te.Top() != 32 {
		t.Errorf("Top() = %f, want 32", te.Top())
	}
	if te.CenterX() != 35 {
		t.Errorf("CenterX() = %f, want 35", te.CenterX())
	}
	if te.CenterY() != 26 {
		t.Errorf("CenterY() = %f, want 26", te.CenterY())
	}
}

func TestTextElement_VerticalOverlapRatio_FullOverlap(t *testing.T) {
	te1 := NewTextElement("A", 0, 10, 10, 10, "F1", 10)
	te2 := NewTextElement("B", 20, 10, 10, 10, "F1", 10)
	ratio := te1.VerticalOverlapRatio(te2)
	if ratio < 0.99 {
		t.Errorf("Same Y range: overlap ratio = %f, want ~1.0", ratio)
	}
}

func TestTextElement_VerticalOverlapRatio_NoOverlap(t *testing.T) {
	te1 := NewTextElement("A", 0, 0, 10, 10, "F1", 10)
	te2 := NewTextElement("B", 0, 20, 10, 10, "F1", 10)
	ratio := te1.VerticalOverlapRatio(te2)
	if ratio != 0.0 {
		t.Errorf("Non-overlapping: overlap ratio = %f, want 0.0", ratio)
	}
}

func TestTextElement_VerticalOverlapRatio_PartialOverlap(t *testing.T) {
	te1 := NewTextElement("A", 0, 0, 10, 10, "F1", 10)
	te2 := NewTextElement("B", 0, 5, 10, 10, "F1", 10)
	ratio := te1.VerticalOverlapRatio(te2)
	if ratio <= 0 || ratio >= 1 {
		t.Errorf("Partial overlap: ratio = %f, want between 0 and 1", ratio)
	}
}

func TestTextElement_VerticalOverlapRatio_OtherInsideThis(t *testing.T) {
	te1 := NewTextElement("A", 0, 0, 10, 20, "F1", 10)
	te2 := NewTextElement("B", 0, 5, 10, 5, "F1", 10)
	ratio := te1.VerticalOverlapRatio(te2)
	if ratio <= 0 {
		t.Errorf("Other inside this: ratio = %f, want > 0", ratio)
	}
}

func TestTextElement_VerticalOverlapRatio_ZeroDelta(t *testing.T) {
	te1 := NewTextElement("A", 0, 10, 10, 0, "F1", 0)
	te2 := NewTextElement("B", 0, 10, 10, 0, "F1", 0)
	ratio := te1.VerticalOverlapRatio(te2)
	if ratio != 0.0 {
		t.Errorf("Zero height overlap: ratio = %f, want 0.0", ratio)
	}
}

// ---------- Rectangle additional tests ----------

func TestRectangle_BoundsMethods(t *testing.T) {
	r := NewRectangle(10, 20, 100, 50)
	if r.Left() != 10 {
		t.Errorf("Left() = %f, want 10", r.Left())
	}
	if r.Right() != 110 {
		t.Errorf("Right() = %f, want 110", r.Right())
	}
	if r.Bottom() != 20 {
		t.Errorf("Bottom() = %f, want 20", r.Bottom())
	}
	if r.Top() != 70 {
		t.Errorf("Top() = %f, want 70", r.Top())
	}
}

func TestRectangle_StringReturnsNonEmpty(t *testing.T) {
	r := NewRectangle(1, 2, 3, 4)
	s := r.String()
	if s == "" {
		t.Error("Rectangle.String() returned empty")
	}
}

// ---------- TextChunk additional tests ----------

func TestNewTextChunk_EmptyInit(t *testing.T) {
	chunk := NewTextChunk(nil)
	if chunk == nil {
		t.Fatal("NewTextChunk(nil) returned nil")
	}
	if chunk.Len() != 0 {
		t.Errorf("Empty chunk Len = %d, want 0", chunk.Len())
	}
	if chunk.Text() != "" {
		t.Errorf("Empty chunk Text = %q, want empty", chunk.Text())
	}
}

func TestNewTextChunk_WithTwoElements(t *testing.T) {
	elems := []*TextElement{
		NewTextElement("Hello", 10, 20, 30, 12, "F1", 12),
		NewTextElement("World", 50, 20, 25, 12, "F1", 12),
	}
	chunk := NewTextChunk(elems)
	if chunk.Len() != 2 {
		t.Errorf("chunk.Len() = %d, want 2", chunk.Len())
	}
	if chunk.Text() != "HelloWorld" {
		t.Errorf("chunk.Text() = %q, want HelloWorld", chunk.Text())
	}
	if chunk.Bounds.X != 10 {
		t.Errorf("chunk.Bounds.X = %f, want 10", chunk.Bounds.X)
	}
}

// ---------- Operator additional tests ----------

func TestNewOperator_FieldsCorrect(t *testing.T) {
	ops := []parser.PdfObject{
		parser.NewString("hello"),
		parser.NewInteger(12),
	}
	op := NewOperator("Tj", ops)
	if op == nil {
		t.Fatal("NewOperator returned nil")
	}
	if op.Name != "Tj" {
		t.Errorf("Name = %q, want Tj", op.Name)
	}
	if len(op.Operands) != 2 {
		t.Errorf("Operands len = %d, want 2", len(op.Operands))
	}
}

// ---------- ContentParser additional tests ----------

func TestContentParser_ParseOperators_SimpleStream(t *testing.T) {
	content := []byte("BT /F1 12 Tf 100.00 700.00 Td (Hello) Tj ET")
	cp := NewContentParser(content)
	ops, err := cp.ParseOperators()
	if err != nil {
		t.Fatalf("ParseOperators() error = %v", err)
	}
	if len(ops) == 0 {
		t.Fatal("ParseOperators() returned no operators")
	}
	opNames := make(map[string]bool)
	for _, op := range ops {
		opNames[op.Name] = true
	}
	for _, want := range []string{"BT", "ET", "Tf", "Td", "Tj"} {
		if !opNames[want] {
			t.Errorf("Operator %q not found in parsed operators", want)
		}
	}
}

func TestContentParser_ParseOperators_EmptyContent(t *testing.T) {
	cp := NewContentParser([]byte{})
	ops, err := cp.ParseOperators()
	if err != nil {
		t.Fatalf("ParseOperators(empty) error = %v", err)
	}
	if len(ops) != 0 {
		t.Errorf("ParseOperators(empty) = %d ops, want 0", len(ops))
	}
}

func TestContentParser_ParseOperators_GraphicsOps(t *testing.T) {
	content := []byte("q 1 0 0 1 100 200 cm 0.5 G 0.5 w 100 200 m 200 300 l S Q")
	cp := NewContentParser(content)
	ops, err := cp.ParseOperators()
	if err != nil {
		t.Fatalf("ParseOperators(graphics) error = %v", err)
	}
	if len(ops) == 0 {
		t.Fatal("ParseOperators(graphics) returned no operators")
	}
}

func TestContentParser_ParseOperators_TJArrayContent(t *testing.T) {
	content := []byte("BT [(Hello) -250 (World)] TJ ET")
	cp := NewContentParser(content)
	ops, err := cp.ParseOperators()
	if err != nil {
		t.Fatalf("ParseOperators(TJ) error = %v", err)
	}
	opNames := make(map[string]bool)
	for _, op := range ops {
		opNames[op.Name] = true
	}
	if !opNames["TJ"] {
		t.Error("TJ operator not found")
	}
}

func TestContentParser_ParseOperators_TextPositioning(t *testing.T) {
	content := []byte("BT 10 20 Td 1 0 0 1 50 100 Tm T* ET")
	cp := NewContentParser(content)
	ops, err := cp.ParseOperators()
	if err != nil {
		t.Fatalf("ParseOperators() error = %v", err)
	}
	opNames := make(map[string]bool)
	for _, op := range ops {
		opNames[op.Name] = true
	}
	if !opNames["Td"] {
		t.Error("Td not found")
	}
	if !opNames["Tm"] {
		t.Error("Tm not found")
	}
}

// ---------- FontDecoder additional tests ----------

func TestNewFontDecoder_NilCMap(t *testing.T) {
	fd := NewFontDecoder(nil, "", false)
	if fd == nil {
		t.Fatal("NewFontDecoder returned nil")
	}
}

func TestNewFontDecoder_WithEncoding(t *testing.T) {
	fd := NewFontDecoder(nil, "WinAnsiEncoding", false)
	if fd == nil {
		t.Fatal("NewFontDecoder returned nil")
	}
	if fd.encoding != "WinAnsiEncoding" {
		t.Errorf("encoding = %q, want WinAnsiEncoding", fd.encoding)
	}
}

func TestFontDecoder_DecodeString_ASCIIInput(t *testing.T) {
	fd := NewFontDecoder(nil, "", false)
	result := fd.DecodeString([]byte("Hello"))
	if result != "Hello" {
		t.Errorf("DecodeString(Hello) = %q, want Hello", result)
	}
}

func TestFontDecoder_DecodeString_EmptyInput(t *testing.T) {
	fd := NewFontDecoder(nil, "", false)
	result := fd.DecodeString([]byte{})
	if result != "" {
		t.Errorf("DecodeString(empty) = %q, want empty", result)
	}
}

func TestNewFontDecoderWithCMap_NilCMap(t *testing.T) {
	fd := NewFontDecoderWithCMap(nil)
	if fd == nil {
		t.Fatal("NewFontDecoderWithCMap(nil) returned nil")
	}
}

// ---------- TextExtractor tests with real PDFs ----------

func openExtractorTestReader(t *testing.T, path string) *parser.Reader {
	t.Helper()
	r, err := parser.OpenPDF(path)
	if err != nil {
		t.Skipf("Cannot open %s: %v", path, err)
	}
	return r
}

func TestNewTextExtractor_Init(t *testing.T) {
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	defer r.Close()

	te := NewTextExtractor(r)
	if te == nil {
		t.Fatal("NewTextExtractor returned nil")
	}
	if te.reader == nil {
		t.Error("reader is nil")
	}
}

func TestTextExtractor_ExtractFromPage_FirstPage(t *testing.T) {
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	defer r.Close()

	te := NewTextExtractor(r)
	elems, err := te.ExtractFromPage(0)
	if err != nil {
		t.Fatalf("ExtractFromPage(0) error = %v", err)
	}
	t.Logf("Extracted %d text elements from page 0", len(elems))
}

func TestTextExtractor_ExtractFromPage_OutOfBounds(t *testing.T) {
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	defer r.Close()

	te := NewTextExtractor(r)
	_, err := te.ExtractFromPage(9999)
	if err == nil {
		t.Error("ExtractFromPage(9999) should return error for out-of-bounds page")
	}
}

func TestTextExtractor_ExtractFromPage_MultiPagePDF(t *testing.T) {
	r := openExtractorTestReader(t, "../../testdata/pdfs/multipage.pdf")
	defer r.Close()

	te := NewTextExtractor(r)
	for i := 0; i < 2; i++ {
		elems, err := te.ExtractFromPage(i)
		if err != nil {
			t.Logf("ExtractFromPage(%d) error = %v (acceptable for some PDFs)", i, err)
			continue
		}
		t.Logf("Page %d: %d text elements", i, len(elems))
	}
}

func TestTextExtractor_ExtractFromPage_ReuseSameExtractor(t *testing.T) {
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	defer r.Close()

	te := NewTextExtractor(r)
	elems1, err1 := te.ExtractFromPage(0)
	elems2, err2 := te.ExtractFromPage(0)
	if err1 != nil || err2 != nil {
		t.Logf("errors: %v, %v", err1, err2)
	}
	if len(elems1) != len(elems2) {
		t.Errorf("Second extraction gave different result: %d vs %d elements", len(elems1), len(elems2))
	}
}

// ---------- processOperator integration tests ----------

func newTestExtractor(t *testing.T) *TextExtractor {
	t.Helper()
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	te := NewTextExtractor(r)
	te.textState = NewTextState()
	te.elements = []*TextElement{}
	te.fontDecoders = make(map[string]*FontDecoder)
	te.pageResources = parser.NewDictionary()
	return te
}

func TestProcessOperator_BT_ET(t *testing.T) {
	te := newTestExtractor(t)
	content := []byte("BT /F1 12 Tf 100 700 Td (Hello World) Tj ET")
	cp := NewContentParser(content)
	ops, err := cp.ParseOperators()
	if err != nil {
		t.Fatalf("ParseOperators error: %v", err)
	}
	for _, op := range ops {
		te.processOperator(op)
	}
	if len(te.elements) == 0 {
		t.Error("Expected at least one text element after processing Tj operator")
	}
}

func TestProcessOperator_TextStateOps(t *testing.T) {
	te := newTestExtractor(t)
	// Tc=1, Tw=2, Tz=90, TL=14, Ts=2, Tr=0
	content := []byte("BT 1 Tc 2 Tw 90 Tz 14 TL 2 Ts 0 Tr ET")
	cp := NewContentParser(content)
	ops, err := cp.ParseOperators()
	if err != nil {
		t.Fatalf("ParseOperators error: %v", err)
	}
	for _, op := range ops {
		te.processOperator(op)
	}
	if te.textState.CharSpace != 1 {
		t.Errorf("CharSpace = %f, want 1", te.textState.CharSpace)
	}
	if te.textState.WordSpace != 2 {
		t.Errorf("WordSpace = %f, want 2", te.textState.WordSpace)
	}
	if te.textState.HorizScale != 90 {
		t.Errorf("HorizScale = %f, want 90", te.textState.HorizScale)
	}
	if te.textState.Leading != 14 {
		t.Errorf("Leading = %f, want 14", te.textState.Leading)
	}
	if te.textState.Rise != 2 {
		t.Errorf("Rise = %f, want 2", te.textState.Rise)
	}
}

func TestProcessOperator_TD_SetsLeading(t *testing.T) {
	te := newTestExtractor(t)
	content := []byte("BT 0 -14 TD ET")
	cp := NewContentParser(content)
	ops, _ := cp.ParseOperators()
	for _, op := range ops {
		te.processOperator(op)
	}
	if te.textState.Leading != 14 {
		t.Errorf("TD(0,-14) should set Leading=14, got %f", te.textState.Leading)
	}
}

func TestProcessOperator_SingleQuote(t *testing.T) {
	te := newTestExtractor(t)
	// ' operator: move to next line and show text
	content := []byte("BT /F1 12 Tf 0 0 Td (first) ' (second) ' ET")
	cp := NewContentParser(content)
	ops, _ := cp.ParseOperators()
	for _, op := range ops {
		te.processOperator(op)
	}
	// Should not panic; elements may be produced
}

func TestProcessOperator_DoubleQuote(t *testing.T) {
	te := newTestExtractor(t)
	// " operator: set word/char spacing, move to next line, show text
	content := []byte("BT /F1 12 Tf 1 2 (text) \" ET")
	cp := NewContentParser(content)
	ops, _ := cp.ParseOperators()
	for _, op := range ops {
		te.processOperator(op)
	}
	if te.textState.WordSpace != 1 {
		t.Errorf("\" op: WordSpace = %f, want 1", te.textState.WordSpace)
	}
	if te.textState.CharSpace != 2 {
		t.Errorf("\" op: CharSpace = %f, want 2", te.textState.CharSpace)
	}
}

func TestProcessOperator_TJ_ProducesElements(t *testing.T) {
	te := newTestExtractor(t)
	content := []byte("BT /F1 12 Tf [(A) -100 (B) -50 (C)] TJ ET")
	cp := NewContentParser(content)
	ops, _ := cp.ParseOperators()
	for _, op := range ops {
		te.processOperator(op)
	}
	if len(te.elements) == 0 {
		t.Error("TJ operator should produce text elements")
	}
}

// ---------- getNumber helper tests ----------

func TestGetNumber_Integer(t *testing.T) {
	num := getNumber(parser.NewInteger(42))
	if num == nil {
		t.Fatal("getNumber(Integer) returned nil")
	}
	if *num != 42.0 {
		t.Errorf("getNumber(Integer) = %f, want 42.0", *num)
	}
}

func TestGetNumber_Real(t *testing.T) {
	num := getNumber(parser.NewReal(3.14))
	if num == nil {
		t.Fatal("getNumber(Real) returned nil")
	}
	if math.Abs(*num-3.14) > 1e-9 {
		t.Errorf("getNumber(Real) = %f, want 3.14", *num)
	}
}

func TestGetNumber_NonNumber(t *testing.T) {
	num := getNumber(parser.NewString("hello"))
	if num != nil {
		t.Error("getNumber(String) should return nil")
	}
}

func TestGetNumber_Null(t *testing.T) {
	num := getNumber(parser.NewNull())
	if num != nil {
		t.Error("getNumber(Null) should return nil")
	}
}

// ---------- GraphicsParser tests ----------

func TestNewGraphicsParser_Init(t *testing.T) {
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	defer r.Close()

	gp := NewGraphicsParser(r)
	if gp == nil {
		t.Fatal("NewGraphicsParser returned nil")
	}
	if gp.reader == nil {
		t.Error("reader should not be nil")
	}
	if gp.state == nil {
		t.Error("state should not be nil")
	}
}

func TestGraphicsParser_ParseFromPage_Valid(t *testing.T) {
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	defer r.Close()

	gp := NewGraphicsParser(r)
	elems, err := gp.ParseFromPage(0)
	if err != nil {
		t.Fatalf("ParseFromPage(0) error = %v", err)
	}
	t.Logf("Extracted %d graphics elements from page 0", len(elems))
}

func TestGraphicsParser_ParseFromPage_OutOfBounds(t *testing.T) {
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	defer r.Close()

	gp := NewGraphicsParser(r)
	_, err := gp.ParseFromPage(9999)
	if err == nil {
		t.Error("ParseFromPage(9999) should return an error")
	}
}

func TestGraphicsParser_ParseFromPage_Multipage(t *testing.T) {
	r := openExtractorTestReader(t, "../../testdata/pdfs/multipage.pdf")
	defer r.Close()

	gp := NewGraphicsParser(r)
	for i := 0; i < 2; i++ {
		elems, err := gp.ParseFromPage(i)
		if err != nil {
			t.Logf("ParseFromPage(%d) error = %v (acceptable)", i, err)
			continue
		}
		t.Logf("Page %d: %d graphics elements", i, len(elems))
	}
}

// ---------- ImageExtractor tests ----------

func TestNewImageExtractor_Init(t *testing.T) {
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	defer r.Close()

	ie := NewImageExtractor(r)
	if ie == nil {
		t.Fatal("NewImageExtractor returned nil")
	}
	if ie.reader == nil {
		t.Error("reader should not be nil")
	}
}

func TestImageExtractor_ExtractFromPage_Valid(t *testing.T) {
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	defer r.Close()

	ie := NewImageExtractor(r)
	imgs, err := ie.ExtractFromPage(0)
	if err != nil {
		t.Fatalf("ExtractFromPage(0) error = %v", err)
	}
	t.Logf("Extracted %d images from page 0", len(imgs))
}

func TestImageExtractor_ExtractFromPage_OutOfBounds(t *testing.T) {
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	defer r.Close()

	ie := NewImageExtractor(r)
	_, err := ie.ExtractFromPage(9999)
	if err == nil {
		t.Error("ExtractFromPage(9999) should return an error")
	}
}

func TestImageExtractor_ExtractFromDocument_Valid(t *testing.T) {
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	defer r.Close()

	ie := NewImageExtractor(r)
	imgs, err := ie.ExtractFromDocument()
	if err != nil {
		t.Fatalf("ExtractFromDocument() error = %v", err)
	}
	t.Logf("Extracted %d images total from document", len(imgs))
}

func TestImageExtractor_ExtractFromDocument_Multipage(t *testing.T) {
	r := openExtractorTestReader(t, "../../testdata/pdfs/multipage.pdf")
	defer r.Close()

	ie := NewImageExtractor(r)
	imgs, err := ie.ExtractFromDocument()
	if err != nil {
		t.Fatalf("ExtractFromDocument() error = %v", err)
	}
	t.Logf("Extracted %d images from multipage document", len(imgs))
}

// ---------- FontDecoder readGlyphID / decodeGlyph tests ----------

func TestFontDecoder_ReadGlyphID_OneByte(t *testing.T) {
	fd := NewFontDecoder(nil, "", false)
	id, n := fd.readGlyphID([]byte{0x41})
	if id != 0x41 {
		t.Errorf("readGlyphID 1-byte = %d, want 0x41", id)
	}
	if n != 1 {
		t.Errorf("readGlyphID 1-byte n = %d, want 1", n)
	}
}

func TestFontDecoder_ReadGlyphID_TwoByte(t *testing.T) {
	fd := NewFontDecoder(nil, "", true) // use2ByteGlyphs=true
	id, n := fd.readGlyphID([]byte{0x00, 0x41})
	if id != 0x41 {
		t.Errorf("readGlyphID 2-byte = %d, want 0x41", id)
	}
	if n != 2 {
		t.Errorf("readGlyphID 2-byte n = %d, want 2", n)
	}
}

func TestFontDecoder_ReadGlyphID_Empty(t *testing.T) {
	fd := NewFontDecoder(nil, "", false)
	id, n := fd.readGlyphID([]byte{})
	if id != 0 || n != 0 {
		t.Errorf("readGlyphID empty: id=%d n=%d, want 0,0", id, n)
	}
}

func TestFontDecoder_ReadGlyphID_TwoByte_Truncated(t *testing.T) {
	// 2-byte mode but only 1 byte available — should fall back to 1-byte
	fd := NewFontDecoder(nil, "", true)
	id, n := fd.readGlyphID([]byte{0x42})
	if id != 0x42 {
		t.Errorf("readGlyphID truncated = %d, want 0x42", id)
	}
	if n != 1 {
		t.Errorf("readGlyphID truncated n = %d, want 1", n)
	}
}

// ---------- NewFontDecoderWithCustomEncoding tests ----------

func TestNewFontDecoderWithCustomEncoding_Basic(t *testing.T) {
	diffs := map[uint16]string{
		0x31: "one",
		0x32: "two",
		0x33: "three",
	}
	fd := NewFontDecoderWithCustomEncoding(diffs, "WinAnsiEncoding", false)
	if fd == nil {
		t.Fatal("NewFontDecoderWithCustomEncoding returned nil")
	}
	if fd.encoding != "WinAnsiEncoding" {
		t.Errorf("encoding = %q, want WinAnsiEncoding", fd.encoding)
	}
}

func TestNewFontDecoderWithCustomEncoding_Empty(t *testing.T) {
	fd := NewFontDecoderWithCustomEncoding(map[uint16]string{}, "", false)
	if fd == nil {
		t.Fatal("NewFontDecoderWithCustomEncoding(empty) returned nil")
	}
}

func TestNewFontDecoderWithCustomEncoding_UnknownGlyph(t *testing.T) {
	// Use a glyph name NOT in the Adobe Glyph List
	diffs := map[uint16]string{
		0x01: "unknownGlyph999",
		0x02: "A", // single char fallback
	}
	fd := NewFontDecoderWithCustomEncoding(diffs, "", false)
	if fd == nil {
		t.Fatal("NewFontDecoderWithCustomEncoding returned nil")
	}
	// Glyph 0x02 "A" should map to 'A' as single-char fallback
	if r, ok := fd.customEncoding[0x02]; !ok || r != 'A' {
		t.Errorf("Single-char fallback: encoding[0x02] = %q (ok=%v), want 'A'", r, ok)
	}
}

// ---------- isDelimiter (cmap_parser) tests ----------

func TestIsDelimiter_Delimiters(t *testing.T) {
	delimiters := []byte{'(', ')', '<', '>', '[', ']', '{', '}', '/', '%'}
	for _, b := range delimiters {
		if !isDelimiter(b) {
			t.Errorf("isDelimiter(%q) = false, want true", b)
		}
	}
}

func TestIsDelimiter_NonDelimiters(t *testing.T) {
	nonDelimiters := []byte{'a', 'Z', '0', ' ', '\n', '_', '-'}
	for _, b := range nonDelimiters {
		if isDelimiter(b) {
			t.Errorf("isDelimiter(%q) = true, want false", b)
		}
	}
}

// ---------- parseDifferencesArray tests ----------

func TestParseDifferencesArray_NoDifferences(t *testing.T) {
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	defer r.Close()

	te := NewTextExtractor(r)
	te.textState = NewTextState()
	te.fontDecoders = make(map[string]*FontDecoder)
	te.pageResources = parser.NewDictionary()

	encodingDict := parser.NewDictionary()
	result := te.parseDifferencesArray(encodingDict)
	if len(result) != 0 {
		t.Errorf("No Differences: len = %d, want 0", len(result))
	}
}

func TestParseDifferencesArray_WithDifferences(t *testing.T) {
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	defer r.Close()

	te := NewTextExtractor(r)
	te.textState = NewTextState()
	te.fontDecoders = make(map[string]*FontDecoder)
	te.pageResources = parser.NewDictionary()

	// Build Differences array: [1 /one /two /three]
	arr := parser.NewArray()
	arr.Append(parser.NewInteger(1))
	arr.Append(parser.NewName("one"))
	arr.Append(parser.NewName("two"))
	arr.Append(parser.NewName("three"))

	encodingDict := parser.NewDictionary()
	encodingDict.Set("Differences", arr)

	result := te.parseDifferencesArray(encodingDict)
	if len(result) == 0 {
		t.Error("parseDifferencesArray with valid data should return non-empty map")
	}
}

// ---------- ContentParser parseDictionary coverage ----------

func TestContentParser_ParseOperators_WithDictionary(t *testing.T) {
	// Dictionary inline in content stream (unusual but valid)
	content := []byte("BT << /Type /Font >> ET")
	cp := NewContentParser(content)
	ops, err := cp.ParseOperators()
	if err != nil {
		t.Fatalf("ParseOperators with dict error = %v", err)
	}
	_ = ops
}

func TestContentParser_ParseOperators_NullBoolean(t *testing.T) {
	// Test null and boolean tokens in operands
	content := []byte("BT null true false Tr ET")
	cp := NewContentParser(content)
	ops, err := cp.ParseOperators()
	if err != nil {
		t.Fatalf("ParseOperators(null/bool) error = %v", err)
	}
	_ = ops
}

func TestContentParser_ParseOperators_HexString(t *testing.T) {
	// Hex string in TJ operator
	content := []byte("BT <48656c6c6f> Tj ET")
	cp := NewContentParser(content)
	ops, err := cp.ParseOperators()
	if err != nil {
		t.Fatalf("ParseOperators(hex string) error = %v", err)
	}
	_ = ops
}

// ---------- loadFontDecoder unit tests (direct state manipulation) ----------

func newExtractorWithResources(t *testing.T) *TextExtractor {
	t.Helper()
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	te := NewTextExtractor(r)
	te.textState = NewTextState()
	te.elements = []*TextElement{}
	te.fontDecoders = make(map[string]*FontDecoder)
	te.pageResources = parser.NewDictionary()
	return te
}

func TestLoadFontDecoder_AlreadyLoaded(t *testing.T) {
	te := newExtractorWithResources(t)
	// Pre-load a decoder
	te.fontDecoders["F1"] = NewFontDecoder(nil, "", false)
	// Call again — should return early without panic
	te.loadFontDecoder("F1")
	if len(te.fontDecoders) != 1 {
		t.Errorf("fontDecoders len = %d, want 1", len(te.fontDecoders))
	}
}

func TestLoadFontDecoder_NoFontInResources(t *testing.T) {
	te := newExtractorWithResources(t)
	// pageResources has no "Font" key
	te.loadFontDecoder("F1")
	if _, ok := te.fontDecoders["F1"]; !ok {
		t.Error("loadFontDecoder should create default decoder when no Font in resources")
	}
}

func TestLoadFontDecoder_FontDictIsNotDictionary(t *testing.T) {
	te := newExtractorWithResources(t)
	// Font is an integer (wrong type) — should fall back
	te.pageResources.Set("Font", parser.NewInteger(42))
	te.loadFontDecoder("F1")
	if _, ok := te.fontDecoders["F1"]; !ok {
		t.Error("loadFontDecoder should create default decoder when Font is wrong type")
	}
}

func TestLoadFontDecoder_FontNotInFontDict(t *testing.T) {
	te := newExtractorWithResources(t)
	fontsDict := parser.NewDictionary()
	// No "F1" key in fonts dict
	te.pageResources.Set("Font", fontsDict)
	te.loadFontDecoder("F1")
	if _, ok := te.fontDecoders["F1"]; !ok {
		t.Error("loadFontDecoder should create default decoder when font not found")
	}
}

func TestLoadFontDecoder_FontObjIsNotDict(t *testing.T) {
	te := newExtractorWithResources(t)
	fontsDict := parser.NewDictionary()
	// F1 is an integer (wrong type)
	fontsDict.Set("F1", parser.NewInteger(1))
	te.pageResources.Set("Font", fontsDict)
	te.loadFontDecoder("F1")
	if _, ok := te.fontDecoders["F1"]; !ok {
		t.Error("loadFontDecoder should create default decoder when font object is wrong type")
	}
}

func TestLoadFontDecoder_FontDictWithNameEncoding(t *testing.T) {
	te := newExtractorWithResources(t)
	fontDict := parser.NewDictionary()
	fontDict.Set("Encoding", parser.NewName("WinAnsiEncoding"))
	fontsDict := parser.NewDictionary()
	fontsDict.Set("F1", fontDict)
	te.pageResources.Set("Font", fontsDict)
	te.loadFontDecoder("F1")
	decoder, ok := te.fontDecoders["F1"]
	if !ok {
		t.Fatal("loadFontDecoder should create decoder with WinAnsiEncoding")
	}
	if decoder.encoding != "WinAnsiEncoding" {
		t.Errorf("encoding = %q, want WinAnsiEncoding", decoder.encoding)
	}
}

func TestLoadFontDecoder_FontDictWithEncodingDict(t *testing.T) {
	te := newExtractorWithResources(t)

	// Build encoding dict with BaseEncoding + Differences
	diffsArr := parser.NewArray()
	diffsArr.Append(parser.NewInteger(1))
	diffsArr.Append(parser.NewName("one"))
	diffsArr.Append(parser.NewName("two"))

	encDict := parser.NewDictionary()
	encDict.Set("BaseEncoding", parser.NewName("WinAnsiEncoding"))
	encDict.Set("Differences", diffsArr)

	fontDict := parser.NewDictionary()
	fontDict.Set("Encoding", encDict)

	fontsDict := parser.NewDictionary()
	fontsDict.Set("F1", fontDict)
	te.pageResources.Set("Font", fontsDict)

	te.loadFontDecoder("F1")
	if _, ok := te.fontDecoders["F1"]; !ok {
		t.Fatal("loadFontDecoder should create decoder with encoding dict")
	}
}

func TestLoadFontDecoder_FontDictWithToUnicodeNotStream(t *testing.T) {
	te := newExtractorWithResources(t)
	fontDict := parser.NewDictionary()
	// ToUnicode is an integer (not a stream)
	fontDict.Set("ToUnicode", parser.NewInteger(99))

	fontsDict := parser.NewDictionary()
	fontsDict.Set("F1", fontDict)
	te.pageResources.Set("Font", fontsDict)

	te.loadFontDecoder("F1")
	if _, ok := te.fontDecoders["F1"]; !ok {
		t.Fatal("loadFontDecoder should create decoder even when ToUnicode is not a stream")
	}
}

// ---------- getPageResources branch coverage ----------

func TestGetPageResources_WithDirectDict(t *testing.T) {
	te := newExtractorWithResources(t)

	page := parser.NewDictionary()
	resDict := parser.NewDictionary()
	resDict.Set("Font", parser.NewDictionary())
	page.Set("Resources", resDict)

	result := te.getPageResources(page)
	if result == nil {
		t.Fatal("getPageResources should return non-nil dictionary")
	}
}

func TestGetPageResources_NilResources(t *testing.T) {
	te := newExtractorWithResources(t)

	page := parser.NewDictionary()
	// No Resources key

	result := te.getPageResources(page)
	if result == nil {
		t.Fatal("getPageResources(no resources) should return empty dict, not nil")
	}
}

func TestGetPageResources_WrongType(t *testing.T) {
	te := newExtractorWithResources(t)

	page := parser.NewDictionary()
	page.Set("Resources", parser.NewInteger(42))

	result := te.getPageResources(page)
	if result == nil {
		t.Fatal("getPageResources(wrong type) should return empty dict, not nil")
	}
}

// ---------- decodeTextBytes coverage ----------

func TestDecodeTextBytes_WithNoDecoder(t *testing.T) {
	te := newExtractorWithResources(t)
	te.textState.FontName = "UnknownFont"
	// No decoder registered for UnknownFont
	result := te.decodeTextBytes([]byte("Hello"))
	if result != "Hello" {
		t.Errorf("decodeTextBytes(no decoder) = %q, want Hello", result)
	}
}

func TestDecodeTextBytes_WithDecoder(t *testing.T) {
	te := newExtractorWithResources(t)
	te.textState.FontName = "F1"
	te.fontDecoders["F1"] = NewFontDecoder(nil, "", false)
	result := te.decodeTextBytes([]byte("World"))
	if result != "World" {
		t.Errorf("decodeTextBytes(with decoder) = %q, want World", result)
	}
}

// ---------- ImageExtractor helper method tests ----------

func newTestImageExtractor(t *testing.T) *ImageExtractor {
	t.Helper()
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	return NewImageExtractor(r)
}

func TestGetColorSpaceName_Nil(t *testing.T) {
	ie := newTestImageExtractor(t)
	result := ie.getColorSpaceName(nil)
	if result != colorSpaceDeviceRGB {
		t.Errorf("getColorSpaceName(nil) = %q, want DeviceRGB", result)
	}
}

func TestGetColorSpaceName_DirectName(t *testing.T) {
	ie := newTestImageExtractor(t)
	result := ie.getColorSpaceName(parser.NewName("DeviceGray"))
	if result != "DeviceGray" {
		t.Errorf("getColorSpaceName(Name) = %q, want DeviceGray", result)
	}
}

func TestGetColorSpaceName_Array(t *testing.T) {
	ie := newTestImageExtractor(t)
	arr := parser.NewArray()
	arr.Append(parser.NewName("Indexed"))
	result := ie.getColorSpaceName(arr)
	if result != "Indexed" {
		t.Errorf("getColorSpaceName(Array) = %q, want Indexed", result)
	}
}

func TestGetColorSpaceName_EmptyArray(t *testing.T) {
	ie := newTestImageExtractor(t)
	arr := parser.NewArray()
	result := ie.getColorSpaceName(arr)
	if result != colorSpaceDeviceRGB {
		t.Errorf("getColorSpaceName(empty array) = %q, want DeviceRGB", result)
	}
}

func TestGetColorSpaceName_WrongType(t *testing.T) {
	ie := newTestImageExtractor(t)
	result := ie.getColorSpaceName(parser.NewInteger(42))
	if result != colorSpaceDeviceRGB {
		t.Errorf("getColorSpaceName(Integer) = %q, want DeviceRGB", result)
	}
}

func TestDecodeImageData_NoFilter(t *testing.T) {
	ie := newTestImageExtractor(t)
	dict := parser.NewDictionary()
	stream := parser.NewStream(dict, []byte{0xFF, 0xD8, 0xFF})
	data, filter, err := ie.decodeImageData(stream)
	if err != nil {
		t.Fatalf("decodeImageData(no filter) error = %v", err)
	}
	if filter != "" {
		t.Errorf("filter = %q, want empty", filter)
	}
	if len(data) != 3 {
		t.Errorf("data len = %d, want 3", len(data))
	}
}

func TestDecodeImageData_DCTDecode(t *testing.T) {
	ie := newTestImageExtractor(t)
	dict := parser.NewDictionary()
	dict.Set("Filter", parser.NewName("DCTDecode"))
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	stream := parser.NewStream(dict, jpegData)
	data, filter, err := ie.decodeImageData(stream)
	if err != nil {
		t.Fatalf("decodeImageData(DCT) error = %v", err)
	}
	if filter != "/DCTDecode" {
		t.Errorf("filter = %q, want /DCTDecode", filter)
	}
	if !bytes.Equal(data, jpegData) {
		t.Errorf("DCT image path changed encoded payload: got %x, want %x", data, jpegData)
	}
}

func TestDecodeImageData_UnsupportedFilter(t *testing.T) {
	ie := newTestImageExtractor(t)
	dict := parser.NewDictionary()
	dict.Set("Filter", parser.NewName("JBIG2Decode"))
	stream := parser.NewStream(dict, []byte{0x01, 0x02})
	_, _, err := ie.decodeImageData(stream)
	if err == nil {
		t.Error("decodeImageData(unsupported) should return error")
	}
}

func TestExtractImageFromStream_DecodesFilterChainBeforeDCT(t *testing.T) {
	ie := newTestImageExtractor(t)
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	encoded := make([]byte, ascii85.MaxEncodedLen(len(jpegData)))
	written := ascii85.Encode(encoded, jpegData)
	encoded = append(encoded[:written], '~', '>')
	filters := parser.NewArray()
	filters.Append(parser.NewName("ASCII85Decode"))
	filters.Append(parser.NewName("DCTDecode"))
	dict := parser.NewDictionary()
	dict.Set("Width", parser.NewInteger(1))
	dict.Set("Height", parser.NewInteger(1))
	dict.Set("BitsPerComponent", parser.NewInteger(8))
	dict.Set("ColorSpace", parser.NewName(colorSpaceDeviceRGB))
	dict.Set("Filter", filters)

	image, err := ie.extractImageFromStream(parser.NewStream(dict, encoded), "Im1")
	if err != nil {
		t.Fatalf("extractImageFromStream() error = %v", err)
	}
	if !bytes.Equal(image.Data(), jpegData) {
		t.Errorf("image data = %x, want %x", image.Data(), jpegData)
	}
	if image.Filter() != "/DCTDecode" {
		t.Errorf("filter = %q, want /DCTDecode", image.Filter())
	}
}

func TestExtractImageFromStream_InvalidDimensions(t *testing.T) {
	ie := newTestImageExtractor(t)
	dict := parser.NewDictionary()
	// Width=0, Height=0 → invalid
	stream := parser.NewStream(dict, []byte{})
	_, err := ie.extractImageFromStream(stream, "Im1")
	if err == nil {
		t.Error("extractImageFromStream(zero dimensions) should return error")
	}
}

func TestExtractImageFromStream_ValidRaw(t *testing.T) {
	ie := newTestImageExtractor(t)
	dict := parser.NewDictionary()
	dict.Set("Width", parser.NewInteger(2))
	dict.Set("Height", parser.NewInteger(2))
	dict.Set("BitsPerComponent", parser.NewInteger(8))
	dict.Set("ColorSpace", parser.NewName(colorSpaceDeviceRGB))
	// Raw RGB data: 2x2 pixels = 12 bytes
	rawData := make([]byte, 12)
	stream := parser.NewStream(dict, rawData)

	img, err := ie.extractImageFromStream(stream, "Im1")
	if err != nil {
		t.Fatalf("extractImageFromStream(valid) error = %v", err)
	}
	if img == nil {
		t.Fatal("extractImageFromStream returned nil image")
	}
}

// ---------- GraphicsParser processOperator tests (direct) ----------

func newTestGraphicsParser(t *testing.T) *GraphicsParser {
	t.Helper()
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	return NewGraphicsParser(r)
}

func TestGraphicsParser_ProcessOperator_MoveTo(t *testing.T) {
	gp := newTestGraphicsParser(t)
	ops := []parser.PdfObject{parser.NewReal(10), parser.NewReal(20)}
	gp.processOperator(NewOperator("m", ops))
	if len(gp.state.CurrentPath) != 1 {
		t.Errorf("After m: path len = %d, want 1", len(gp.state.CurrentPath))
	}
}

func TestGraphicsParser_ProcessOperator_LineTo(t *testing.T) {
	gp := newTestGraphicsParser(t)
	// Start path first
	gp.processOperator(NewOperator("m", []parser.PdfObject{parser.NewReal(0), parser.NewReal(0)}))
	gp.processOperator(NewOperator("l", []parser.PdfObject{parser.NewReal(100), parser.NewReal(200)}))
	if len(gp.state.CurrentPath) != 2 {
		t.Errorf("After m+l: path len = %d, want 2", len(gp.state.CurrentPath))
	}
}

func TestGraphicsParser_ProcessOperator_Rectangle(t *testing.T) {
	gp := newTestGraphicsParser(t)
	ops := []parser.PdfObject{
		parser.NewReal(10), parser.NewReal(20),
		parser.NewReal(100), parser.NewReal(50),
	}
	gp.processOperator(NewOperator("re", ops))
	if len(gp.state.CurrentPath) == 0 {
		t.Error("After re: path should be set")
	}
}

func TestGraphicsParser_ProcessOperator_Stroke(t *testing.T) {
	gp := newTestGraphicsParser(t)
	gp.processOperator(NewOperator("m", []parser.PdfObject{parser.NewReal(0), parser.NewReal(0)}))
	gp.processOperator(NewOperator("l", []parser.PdfObject{parser.NewReal(100), parser.NewReal(0)}))
	gp.processOperator(NewOperator("S", nil))
	if len(gp.elements) == 0 {
		t.Error("After m+l+S: should have at least one element")
	}
}

func TestGraphicsParser_ProcessOperator_CloseStroke(t *testing.T) {
	gp := newTestGraphicsParser(t)
	gp.processOperator(NewOperator("m", []parser.PdfObject{parser.NewReal(0), parser.NewReal(0)}))
	gp.processOperator(NewOperator("l", []parser.PdfObject{parser.NewReal(50), parser.NewReal(50)}))
	gp.processOperator(NewOperator("s", nil))
	// s = close + stroke
}

func TestGraphicsParser_ProcessOperator_Fill(t *testing.T) {
	gp := newTestGraphicsParser(t)
	gp.processOperator(NewOperator("m", []parser.PdfObject{parser.NewReal(0), parser.NewReal(0)}))
	gp.processOperator(NewOperator("f", nil))
	if len(gp.state.CurrentPath) != 0 {
		t.Error("After f: path should be cleared")
	}
}

func TestGraphicsParser_ProcessOperator_FillF(t *testing.T) {
	gp := newTestGraphicsParser(t)
	gp.processOperator(NewOperator("m", []parser.PdfObject{parser.NewReal(0), parser.NewReal(0)}))
	gp.processOperator(NewOperator("F", nil))
}

func TestGraphicsParser_ProcessOperator_ClosePath(t *testing.T) {
	gp := newTestGraphicsParser(t)
	gp.processOperator(NewOperator("m", []parser.PdfObject{parser.NewReal(0), parser.NewReal(0)}))
	gp.processOperator(NewOperator("l", []parser.PdfObject{parser.NewReal(100), parser.NewReal(0)}))
	gp.processOperator(NewOperator("h", nil))
}

func TestGraphicsParser_ProcessOperator_LineWidth(t *testing.T) {
	gp := newTestGraphicsParser(t)
	gp.processOperator(NewOperator("w", []parser.PdfObject{parser.NewReal(2.5)}))
	if gp.state.LineWidth != 2.5 {
		t.Errorf("LineWidth = %f, want 2.5", gp.state.LineWidth)
	}
}

func TestGraphicsParser_ProcessOperator_RGBStroke(t *testing.T) {
	gp := newTestGraphicsParser(t)
	ops := []parser.PdfObject{parser.NewReal(1.0), parser.NewReal(0.0), parser.NewReal(0.0)}
	gp.processOperator(NewOperator("RG", ops))
	if gp.state.StrokeColor.R != 1.0 {
		t.Errorf("StrokeColor.R = %f, want 1.0", gp.state.StrokeColor.R)
	}
}

func TestGraphicsParser_ProcessOperator_RGBFill(t *testing.T) {
	gp := newTestGraphicsParser(t)
	ops := []parser.PdfObject{parser.NewReal(0.0), parser.NewReal(1.0), parser.NewReal(0.0)}
	gp.processOperator(NewOperator("rg", ops))
	if gp.state.FillColor.G != 1.0 {
		t.Errorf("FillColor.G = %f, want 1.0", gp.state.FillColor.G)
	}
}

func TestGraphicsParser_ProcessOperator_GrayscaleStroke(t *testing.T) {
	gp := newTestGraphicsParser(t)
	gp.processOperator(NewOperator("G", []parser.PdfObject{parser.NewReal(0.5)}))
	if gp.state.StrokeColor.R != 0.5 {
		t.Errorf("StrokeColor.R = %f, want 0.5", gp.state.StrokeColor.R)
	}
}

func TestGraphicsParser_ProcessOperator_GrayscaleFill(t *testing.T) {
	gp := newTestGraphicsParser(t)
	gp.processOperator(NewOperator("g", []parser.PdfObject{parser.NewReal(0.3)}))
	if gp.state.FillColor.R != 0.3 {
		t.Errorf("FillColor.R = %f, want 0.3", gp.state.FillColor.R)
	}
}

// ---------- TextExtractor decodeStream tests ----------

func TestDecodeStream_NoFilter(t *testing.T) {
	te := newExtractorWithResources(t)
	dict := parser.NewDictionary()
	stream := parser.NewStream(dict, []byte("plain text"))
	data, err := te.decodeStream(stream)
	if err != nil {
		t.Fatalf("decodeStream(no filter) error = %v", err)
	}
	if string(data) != "plain text" {
		t.Errorf("decodeStream(no filter) = %q, want 'plain text'", string(data))
	}
}

func TestDecodeStream_FlateDecode(t *testing.T) {
	te := newExtractorWithResources(t)

	// Compress content with zlib
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	_, _ = w.Write([]byte("BT /F1 12 Tf ET"))
	_ = w.Close()

	dict := parser.NewDictionary()
	dict.Set("Filter", parser.NewName(filterFlateDecode))
	stream := parser.NewStream(dict, buf.Bytes())

	data, err := te.decodeStream(stream)
	if err != nil {
		t.Fatalf("decodeStream(FlateDecode) error = %v", err)
	}
	if string(data) != "BT /F1 12 Tf ET" {
		t.Errorf("decodeStream(FlateDecode) = %q, want 'BT /F1 12 Tf ET'", string(data))
	}
}

func TestDecodeStream_FilterArray(t *testing.T) {
	te := newExtractorWithResources(t)

	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	_, _ = w.Write([]byte("content"))
	_ = w.Close()

	filterArr := parser.NewArray()
	filterArr.Append(parser.NewName(filterFlateDecode))

	dict := parser.NewDictionary()
	dict.Set("Filter", filterArr)
	stream := parser.NewStream(dict, buf.Bytes())

	data, err := te.decodeStream(stream)
	if err != nil {
		t.Fatalf("decodeStream(filter array) error = %v", err)
	}
	if string(data) != "content" {
		t.Errorf("decodeStream(filter array) = %q, want 'content'", string(data))
	}
}

func TestDecodeStream_UnsupportedFilter(t *testing.T) {
	te := newExtractorWithResources(t)
	dict := parser.NewDictionary()
	dict.Set("Filter", parser.NewName("JBIG2Decode"))
	stream := parser.NewStream(dict, []byte("raw"))
	if _, err := te.decodeStream(stream); err == nil {
		t.Fatal("decodeStream(unsupported) error = nil, want error")
	}
}

// ---------- TextExtractor getPageContent tests ----------

func TestGetPageContent_NoContents(t *testing.T) {
	te := newExtractorWithResources(t)
	page := parser.NewDictionary()
	data, err := te.getPageContent(page)
	if err != nil {
		t.Fatalf("getPageContent(no Contents) error = %v", err)
	}
	if len(data) != 0 {
		t.Errorf("getPageContent(no Contents) len = %d, want 0", len(data))
	}
}

func TestGetPageContent_DirectStream(t *testing.T) {
	te := newExtractorWithResources(t)
	page := parser.NewDictionary()
	streamDict := parser.NewDictionary()
	stream := parser.NewStream(streamDict, []byte("BT ET"))
	page.Set("Contents", stream)

	data, err := te.getPageContent(page)
	if err != nil {
		t.Fatalf("getPageContent(direct stream) error = %v", err)
	}
	if string(data) != "BT ET" {
		t.Errorf("getPageContent(direct stream) = %q, want 'BT ET'", string(data))
	}
}

func TestGetPageContent_ArrayOfStreams(t *testing.T) {
	te := newExtractorWithResources(t)
	page := parser.NewDictionary()

	// Build array of two streams
	stream1 := parser.NewStream(parser.NewDictionary(), []byte("BT "))
	stream2 := parser.NewStream(parser.NewDictionary(), []byte("ET"))
	arr := parser.NewArray()
	arr.Append(stream1)
	arr.Append(stream2)
	page.Set("Contents", arr)

	data, err := te.getPageContent(page)
	if err != nil {
		t.Fatalf("getPageContent(array) error = %v", err)
	}
	if len(data) == 0 {
		t.Error("getPageContent(array) should return non-empty data")
	}
}

func TestGetPageContent_ArrayFailsClosedOnUndecodableStream(t *testing.T) {
	te := newExtractorWithResources(t)
	page := parser.NewDictionary()
	unsupportedDictionary := parser.NewDictionary()
	unsupportedDictionary.Set("Filter", parser.NewName("CCITTFaxDecode"))
	contents := parser.NewArray()
	contents.Append(parser.NewStream(parser.NewDictionary(), []byte("BT ET")))
	contents.Append(parser.NewStream(unsupportedDictionary, []byte("encoded")))
	page.Set("Contents", contents)

	_, err := te.getPageContent(page)
	if err == nil {
		t.Fatal("getPageContent() error = nil, want unsupported-filter error")
	}
	if !strings.Contains(err.Error(), "content stream 1") {
		t.Errorf("getPageContent() error = %q, want stream index", err)
	}
	if !errors.Is(err, parser.ErrUnsupportedStreamFilter) {
		t.Errorf("getPageContent() error = %v, want ErrUnsupportedStreamFilter", err)
	}
}

func TestGetPageContent_UnexpectedType(t *testing.T) {
	te := newExtractorWithResources(t)
	page := parser.NewDictionary()
	page.Set("Contents", parser.NewInteger(999))

	_, err := te.getPageContent(page)
	if err == nil {
		t.Error("getPageContent(unexpected type) should return error")
	}
}

// ---------- FontDecoder decodeBuiltInEncoding / decodeGlyph tests ----------

func TestDecodeBuiltInEncoding_WinAnsi(t *testing.T) {
	fd := NewFontDecoder(nil, "WinAnsiEncoding", false)
	r, ok := fd.decodeBuiltInEncoding(0x41) // 'A' in WinAnsi
	if !ok {
		t.Error("decodeBuiltInEncoding(0x41, WinAnsi) should return ok=true")
	}
	if r != 'A' {
		t.Errorf("decodeBuiltInEncoding(0x41) = %q, want 'A'", r)
	}
}

func TestDecodeBuiltInEncoding_GlyphIDOver255(t *testing.T) {
	fd := NewFontDecoder(nil, "WinAnsiEncoding", false)
	r, ok := fd.decodeBuiltInEncoding(0x0100) // 256 > 255
	if ok {
		t.Error("decodeBuiltInEncoding(256) should return ok=false")
	}
	if r != 0 {
		t.Errorf("decodeBuiltInEncoding(256) rune = %d, want 0", r)
	}
}

func TestDecodeBuiltInEncoding_NonWinAnsiEncoding(t *testing.T) {
	fd := NewFontDecoder(nil, "MacRomanEncoding", false)
	_, ok := fd.decodeBuiltInEncoding(0x41)
	if ok {
		t.Error("decodeBuiltInEncoding(MacRoman) should return ok=false (not implemented)")
	}
}

func TestDecodeGlyph_WithCustomEncoding(t *testing.T) {
	diffs := map[uint16]string{0x01: "one"}
	fd := NewFontDecoderWithCustomEncoding(diffs, "", false)
	// Glyph 0x01 maps to 'one' which maps to '1'
	r := fd.decodeGlyph(0x01)
	if r != '1' {
		t.Errorf("decodeGlyph(0x01 with custom) = %q, want '1'", r)
	}
}

func TestDecodeGlyph_HighGlyphID(t *testing.T) {
	fd := NewFontDecoder(nil, "", false)
	r := fd.decodeGlyph(0x0300) // > 255, no cmap
	if r != '\uFFFD' {
		t.Errorf("decodeGlyph(high ID, no cmap) = %q, want replacement char", r)
	}
}

func TestDecodeGlyph_FallbackLatin1(t *testing.T) {
	fd := NewFontDecoder(nil, "", false)
	r := fd.decodeGlyph(0x41) // 'A' in Latin-1 fallback
	if r != 'A' {
		t.Errorf("decodeGlyph(0x41, Latin-1 fallback) = %q, want 'A'", r)
	}
}

// ---------- ImageExtractor ExtractFromPage with fake XObject dict ----------

func TestImageExtractor_ExtractFromPage_WithXObject(t *testing.T) {
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	defer r.Close()

	ie := NewImageExtractor(r)
	// Page 0 of minimal.pdf has no XObjects — should return empty
	imgs, err := ie.ExtractFromPage(0)
	if err != nil {
		t.Fatalf("ExtractFromPage(0) error = %v", err)
	}
	t.Logf("ExtractFromPage(0): %d images", len(imgs))
}

func TestDecodeImageData_FlateDecode(t *testing.T) {
	ie := newTestImageExtractor(t)

	// Create valid zlib-compressed data
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	// Write 4 bytes of raw RGB data (2x2 grayscale image - 1 byte per pixel)
	_, _ = w.Write([]byte{0xFF, 0x00, 0xFF, 0x00})
	_ = w.Close()

	dict := parser.NewDictionary()
	dict.Set("Filter", parser.NewName("FlateDecode"))
	stream := parser.NewStream(dict, buf.Bytes())
	data, filter, err := ie.decodeImageData(stream)
	if err != nil {
		t.Fatalf("decodeImageData(FlateDecode) error = %v", err)
	}
	if filter != "/FlateDecode" {
		t.Errorf("filter = %q, want /FlateDecode", filter)
	}
	if len(data) == 0 {
		t.Error("FlateDecode should return non-empty data")
	}
}

// ---------- GraphicsParser decodeStream tests ----------

func TestGraphicsParser_DecodeStream_NoFilter(t *testing.T) {
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	defer r.Close()

	gp := NewGraphicsParser(r)
	dict := parser.NewDictionary()
	stream := parser.NewStream(dict, []byte("q Q"))
	data, err := gp.decodeStream(stream)
	if err != nil {
		t.Fatalf("graphics decodeStream(no filter) error = %v", err)
	}
	if string(data) != "q Q" {
		t.Errorf("graphics decodeStream = %q, want 'q Q'", string(data))
	}
}

func TestGraphicsParser_DecodeStream_FlateDecode(t *testing.T) {
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	defer r.Close()

	gp := NewGraphicsParser(r)

	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	_, _ = w.Write([]byte("q Q"))
	_ = w.Close()

	dict := parser.NewDictionary()
	dict.Set("Filter", parser.NewName(filterFlateDecode))
	stream := parser.NewStream(dict, buf.Bytes())

	data, err := gp.decodeStream(stream)
	if err != nil {
		t.Fatalf("graphics decodeStream(FlateDecode) error = %v", err)
	}
	if string(data) != "q Q" {
		t.Errorf("graphics decodeStream(FlateDecode) = %q, want 'q Q'", string(data))
	}
}

func TestGraphicsParser_DecodeStream_FilterArray(t *testing.T) {
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	defer r.Close()

	gp := NewGraphicsParser(r)

	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	_, _ = w.Write([]byte("q Q"))
	_ = w.Close()

	filterArr := parser.NewArray()
	filterArr.Append(parser.NewName(filterFlateDecode))

	dict := parser.NewDictionary()
	dict.Set("Filter", filterArr)
	stream := parser.NewStream(dict, buf.Bytes())

	data, err := gp.decodeStream(stream)
	if err != nil {
		t.Fatalf("graphics decodeStream(filter array) error = %v", err)
	}
	if string(data) != "q Q" {
		t.Errorf("graphics decodeStream(filter array) = %q, want 'q Q'", string(data))
	}
}

func TestGraphicsParser_DecodeStream_UnsupportedFilter(t *testing.T) {
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	defer r.Close()

	gp := NewGraphicsParser(r)
	dict := parser.NewDictionary()
	dict.Set("Filter", parser.NewName("JBIG2Decode"))
	stream := parser.NewStream(dict, []byte("raw"))
	if _, err := gp.decodeStream(stream); err == nil {
		t.Fatal("graphics decodeStream(unsupported) error = nil, want error")
	}
}

// ---------- GraphicsParser getPageContent direct tests ----------

func newTestGraphicsParserWithState(t *testing.T) *GraphicsParser {
	t.Helper()
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	return NewGraphicsParser(r)
}

func TestGraphicsParser_GetPageContent_NoContents(t *testing.T) {
	gp := newTestGraphicsParserWithState(t)
	page := parser.NewDictionary()
	data, err := gp.getPageContent(page)
	if err != nil {
		t.Fatalf("getPageContent(no Contents) error = %v", err)
	}
	if len(data) != 0 {
		t.Errorf("getPageContent(no Contents) len = %d, want 0", len(data))
	}
}

func TestGraphicsParser_GetPageContent_DirectStream(t *testing.T) {
	gp := newTestGraphicsParserWithState(t)
	page := parser.NewDictionary()
	streamDict := parser.NewDictionary()
	stream := parser.NewStream(streamDict, []byte("q Q"))
	page.Set("Contents", stream)

	data, err := gp.getPageContent(page)
	if err != nil {
		t.Fatalf("getPageContent(direct stream) error = %v", err)
	}
	if string(data) != "q Q" {
		t.Errorf("getPageContent(direct stream) = %q, want 'q Q'", string(data))
	}
}

func TestGraphicsParser_GetPageContent_ArrayOfStreams(t *testing.T) {
	gp := newTestGraphicsParserWithState(t)
	page := parser.NewDictionary()

	stream1 := parser.NewStream(parser.NewDictionary(), []byte("q "))
	stream2 := parser.NewStream(parser.NewDictionary(), []byte("Q"))
	arr := parser.NewArray()
	arr.Append(stream1)
	arr.Append(stream2)
	page.Set("Contents", arr)

	data, err := gp.getPageContent(page)
	if err != nil {
		t.Fatalf("getPageContent(array) error = %v", err)
	}
	if len(data) == 0 {
		t.Error("getPageContent(array) should return data")
	}
}

func TestGraphicsParser_GetPageContent_UnexpectedType(t *testing.T) {
	gp := newTestGraphicsParserWithState(t)
	page := parser.NewDictionary()
	page.Set("Contents", parser.NewInteger(42))

	_, err := gp.getPageContent(page)
	if err == nil {
		t.Error("getPageContent(unexpected type) should return error")
	}
}

// ---------- parseDifferencesArray with edge cases ----------

func TestParseDifferencesArray_NotAnArray(t *testing.T) {
	te := newExtractorWithResources(t)
	encodingDict := parser.NewDictionary()
	encodingDict.Set("Differences", parser.NewInteger(42)) // not an array
	result := te.parseDifferencesArray(encodingDict)
	if len(result) != 0 {
		t.Errorf("parseDifferencesArray(not array) len = %d, want 0", len(result))
	}
}

func TestParseDifferencesArray_WithSlashPrefix(t *testing.T) {
	te := newExtractorWithResources(t)
	arr := parser.NewArray()
	arr.Append(parser.NewInteger(10))
	arr.Append(parser.NewName("/space")) // with slash prefix
	encodingDict := parser.NewDictionary()
	encodingDict.Set("Differences", arr)
	result := te.parseDifferencesArray(encodingDict)
	// Should handle the slash removal
	_ = result
}

// ---------- getPageResources with indirect reference ----------

func TestGetPageResources_WithIndirectRef(t *testing.T) {
	// Test the indirect reference branch — needs real reader
	r := openExtractorTestReader(t, "../../testdata/pdfs/minimal.pdf")
	defer r.Close()

	te := NewTextExtractor(r)
	te.textState = NewTextState()
	te.fontDecoders = make(map[string]*FontDecoder)
	te.pageResources = parser.NewDictionary()

	// Extract from page 0 which will call getPageResources internally
	_, err := te.ExtractFromPage(0)
	if err != nil {
		t.Logf("ExtractFromPage error (acceptable): %v", err)
	}
	// Just verifying no panic
}

// ---------- loadFontDecoder with ToUnicode stream ----------

func TestLoadFontDecoder_WithToUnicodeStream(t *testing.T) {
	te := newExtractorWithResources(t)

	// Build a minimal CMap stream
	cmapContent := `/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def
/CMapName /Adobe-Identity-UCS def
/CMapType 2 def
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
1 beginbfchar
<0041> <0041>
endbfchar
endcmap
CMapName currentdict /CMap defineresource pop
end
end`

	streamDict := parser.NewDictionary()
	toUnicodeStream := parser.NewStream(streamDict, []byte(cmapContent))

	fontDict := parser.NewDictionary()
	fontDict.Set("ToUnicode", toUnicodeStream)

	fontsDict := parser.NewDictionary()
	fontsDict.Set("F2", fontDict)
	te.pageResources.Set("Font", fontsDict)

	te.loadFontDecoder("F2")
	decoder, ok := te.fontDecoders["F2"]
	if !ok {
		t.Fatal("loadFontDecoder with ToUnicode should create decoder")
	}
	_ = decoder
}

// ---------- content_parser tokenToObject and parseDictionary coverage ----------

func TestContentParser_TokenToObject_DictEOF(t *testing.T) {
	// Trigger EOF inside dictionary parsing
	content := []byte("<< /Key ")
	cp := NewContentParser(content)
	ops, _ := cp.ParseOperators()
	// Should not panic, may return error or partial ops
	_ = ops
}

func TestContentParser_ParseArray_EOF(t *testing.T) {
	// Unterminated array
	content := []byte("[ 1 2 3")
	cp := NewContentParser(content)
	ops, _ := cp.ParseOperators()
	_ = ops
}

func TestContentParser_ParseOperators_DictEnd(t *testing.T) {
	// Test dict end at top level (unbalanced)
	content := []byte("/Key >> Tf")
	cp := NewContentParser(content)
	ops, _ := cp.ParseOperators()
	_ = ops
}
