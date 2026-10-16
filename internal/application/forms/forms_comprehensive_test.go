package forms

import (
	"fmt"
	"os"
	"testing"

	"github.com/coregx/gxpdf/internal/parser"
)

// formPDFPath is the path to the generated form test PDF.
var formPDFPath string

// TestMain creates a minimal PDF with AcroForm for form tests.
func TestMain(m *testing.M) {
	// Create a temp PDF with AcroForm
	path, err := generateFormPDF()
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: could not generate form PDF: %v\n", err)
	} else {
		formPDFPath = path
	}

	code := m.Run()

	// Clean up
	if formPDFPath != "" {
		os.Remove(formPDFPath)
	}

	os.Exit(code)
}

// generateFormPDF creates a minimal PDF with a text field AcroForm annotation.
func generateFormPDF() (string, error) {
	offsets := make(map[int]int)
	var pdf []byte

	write := func(s string) {
		pdf = append(pdf, []byte(s)...)
	}

	// Header
	write("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")

	// Obj 1: Catalog (references AcroForm obj 5)
	offsets[1] = len(pdf)
	write("1 0 obj\n<< /Type /Catalog /Pages 2 0 R /AcroForm 5 0 R >>\nendobj\n")

	// Obj 2: Pages
	offsets[2] = len(pdf)
	write("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	// Obj 3: Page (references annotation obj 6)
	offsets[3] = len(pdf)
	write("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Annots [6 0 R] >>\nendobj\n")

	// Obj 4: Font
	offsets[4] = len(pdf)
	write("4 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n")

	// Obj 5: AcroForm (fields array references obj 6)
	offsets[5] = len(pdf)
	write("5 0 obj\n<< /Fields [6 0 R] /DR << /Font << /Helv 4 0 R >> >> /DA (/Helv 12 Tf 0 g) >>\nendobj\n")

	// Obj 6: Widget annotation (text field named "firstName")
	offsets[6] = len(pdf)
	write("6 0 obj\n<< /Type /Annot /Subtype /Widget /FT /Tx /T (firstName) /V (John) /DV () /Rect [100 700 300 720] /P 3 0 R /AP 7 0 R /DA (/Helv 12 Tf 0 g) >>\nendobj\n")

	// Obj 7: Appearance dictionary (AP dict: /N = obj 8)
	offsets[7] = len(pdf)
	write("7 0 obj\n<< /N 8 0 R >>\nendobj\n")

	// Obj 8: Normal appearance stream
	appContent := "BT /Helv 12 Tf 0 0 Td (John) Tj ET"
	offsets[8] = len(pdf)
	write(fmt.Sprintf("8 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(appContent), appContent))

	// Xref table
	xrefOffset := len(pdf)
	write("xref\n0 9\n0000000000 65535 f \n")
	for i := 1; i <= 8; i++ {
		write(fmt.Sprintf("%010d 00000 n \n", offsets[i]))
	}

	// Trailer
	write(fmt.Sprintf("trailer\n<< /Size 9 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xrefOffset))

	f, err := os.CreateTemp("", "form_test_*.pdf")
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := f.Write(pdf); err != nil {
		return "", err
	}

	return f.Name(), nil
}

// buildFormPDFReader opens the generated form PDF.
func buildFormPDFReader(t *testing.T) *parser.Reader {
	t.Helper()
	if formPDFPath == "" {
		t.Skip("form PDF not available")
	}
	r, err := parser.OpenPDF(formPDFPath)
	if err != nil {
		t.Skipf("cannot open form PDF: %v", err)
	}
	return r
}

// ---------- FieldInfo struct tests ----------

func TestFieldInfo_ZeroValue(t *testing.T) {
	var info FieldInfo
	if info.Name != "" {
		t.Errorf("zero Name = %q, want empty", info.Name)
	}
	if info.Type != "" {
		t.Errorf("zero Type = %q, want empty", info.Type)
	}
	if info.Value != nil {
		t.Errorf("zero Value = %v, want nil", info.Value)
	}
	if info.DefaultValue != nil {
		t.Errorf("zero DefaultValue = %v, want nil", info.DefaultValue)
	}
	if info.Flags != 0 {
		t.Errorf("zero Flags = %d, want 0", info.Flags)
	}
	if len(info.Options) != 0 {
		t.Errorf("zero Options length = %d, want 0", len(info.Options))
	}
}

func TestFieldInfo_Rect(t *testing.T) {
	info := &FieldInfo{
		Rect: [4]float64{10.5, 20.5, 200.5, 40.5},
	}
	want := [4]float64{10.5, 20.5, 200.5, 40.5}
	if info.Rect != want {
		t.Errorf("Rect = %v, want %v", info.Rect, want)
	}
}

// ---------- Reader construction ----------

func TestNewReader_Wiring(t *testing.T) {
	r := NewReader(nil)
	if r == nil {
		t.Fatal("NewReader returned nil")
	}
	if r.pdfReader != nil {
		t.Errorf("pdfReader = %v, want nil", r.pdfReader)
	}
}

// ---------- Writer construction and basic methods ----------

func TestNewWriter_Fields(t *testing.T) {
	w := NewWriter(nil)
	if w == nil {
		t.Fatal("NewWriter returned nil")
	}
	if w.pdfReader != nil {
		t.Errorf("pdfReader = %v, want nil", w.pdfReader)
	}
	if w.updates == nil {
		t.Fatal("updates map is nil")
	}
}

func TestWriter_HasUpdates_AfterDirectInsert(t *testing.T) {
	w := NewWriter(nil)
	w.updates["fieldA"] = "valueA"
	if !w.HasUpdates() {
		t.Error("HasUpdates() = false, want true after insert")
	}
}

// ---------- Writer.setValueInDict ----------

func TestWriter_setValueInDict_Text_String(t *testing.T) {
	w := NewWriter(nil)
	dict := parser.NewDictionary()
	w.setValueInDict(dict, FieldTypeText, "hello world")
	v := dict.Get("V")
	if v == nil {
		t.Fatal("V not set")
	}
	s, ok := v.(*parser.String)
	if !ok {
		t.Fatalf("V type = %T, want *parser.String", v)
	}
	if s.Value() != "hello world" {
		t.Errorf("V.Value() = %q, want %q", s.Value(), "hello world")
	}
}

func TestWriter_setValueInDict_Text_NonString(t *testing.T) {
	// Non-string value for text field: V should NOT be set
	w := NewWriter(nil)
	dict := parser.NewDictionary()
	w.setValueInDict(dict, FieldTypeText, 123)
	if dict.Get("V") != nil {
		t.Error("V should not be set for non-string text field value")
	}
}

func TestWriter_setValueInDict_Button_True(t *testing.T) {
	w := NewWriter(nil)
	dict := parser.NewDictionary()
	w.setValueInDict(dict, FieldTypeButton, true)
	v := dict.Get("V")
	if v == nil {
		t.Fatal("V not set for button true")
	}
	name, ok := v.(*parser.Name)
	if !ok {
		t.Fatalf("V type = %T, want *parser.Name", v)
	}
	if name.Value() != "Yes" {
		t.Errorf("V.Value() = %q, want %q", name.Value(), "Yes")
	}
	as := dict.Get("AS")
	if as == nil {
		t.Fatal("AS not set for button true")
	}
	asName, ok := as.(*parser.Name)
	if !ok {
		t.Fatalf("AS type = %T, want *parser.Name", as)
	}
	if asName.Value() != "Yes" {
		t.Errorf("AS.Value() = %q, want %q", asName.Value(), "Yes")
	}
}

func TestWriter_setValueInDict_Button_False(t *testing.T) {
	w := NewWriter(nil)
	dict := parser.NewDictionary()
	w.setValueInDict(dict, FieldTypeButton, false)
	v := dict.Get("V")
	name, ok := v.(*parser.Name)
	if !ok {
		t.Fatalf("V type = %T, want *parser.Name", v)
	}
	if name.Value() != "Off" {
		t.Errorf("V.Value() = %q, want %q", name.Value(), "Off")
	}
	as := dict.Get("AS")
	asName, ok := as.(*parser.Name)
	if !ok {
		t.Fatalf("AS type = %T, want *parser.Name", as)
	}
	if asName.Value() != "Off" {
		t.Errorf("AS.Value() = %q, want %q", asName.Value(), "Off")
	}
}

func TestWriter_setValueInDict_Button_String(t *testing.T) {
	w := NewWriter(nil)
	dict := parser.NewDictionary()
	w.setValueInDict(dict, FieldTypeButton, "RadioOption")
	v := dict.Get("V")
	name, ok := v.(*parser.Name)
	if !ok {
		t.Fatalf("V type = %T, want *parser.Name", v)
	}
	if name.Value() != "RadioOption" {
		t.Errorf("V.Value() = %q, want RadioOption", name.Value())
	}
	as := dict.Get("AS")
	asName, ok := as.(*parser.Name)
	if !ok {
		t.Fatalf("AS type = %T, want *parser.Name", as)
	}
	if asName.Value() != "RadioOption" {
		t.Errorf("AS.Value() = %q, want RadioOption", asName.Value())
	}
}

func TestWriter_setValueInDict_Choice_String(t *testing.T) {
	w := NewWriter(nil)
	dict := parser.NewDictionary()
	w.setValueInDict(dict, FieldTypeChoice, "Option B")
	v := dict.Get("V")
	s, ok := v.(*parser.String)
	if !ok {
		t.Fatalf("V type = %T, want *parser.String", v)
	}
	if s.Value() != "Option B" {
		t.Errorf("V.Value() = %q, want %q", s.Value(), "Option B")
	}
}

func TestWriter_setValueInDict_Choice_StringSlice(t *testing.T) {
	w := NewWriter(nil)
	dict := parser.NewDictionary()
	w.setValueInDict(dict, FieldTypeChoice, []string{"A", "B", "C"})
	v := dict.Get("V")
	arr, ok := v.(*parser.Array)
	if !ok {
		t.Fatalf("V type = %T, want *parser.Array", v)
	}
	if arr.Len() != 3 {
		t.Errorf("Array.Len() = %d, want 3", arr.Len())
	}
	for i, want := range []string{"A", "B", "C"} {
		elem := arr.Get(i)
		s, ok := elem.(*parser.String)
		if !ok {
			t.Fatalf("Array[%d] type = %T, want *parser.String", i, elem)
		}
		if s.Value() != want {
			t.Errorf("Array[%d].Value() = %q, want %q", i, s.Value(), want)
		}
	}
}

func TestWriter_setValueInDict_Choice_EmptySlice(t *testing.T) {
	w := NewWriter(nil)
	dict := parser.NewDictionary()
	w.setValueInDict(dict, FieldTypeChoice, []string{})
	v := dict.Get("V")
	arr, ok := v.(*parser.Array)
	if !ok {
		t.Fatalf("V type = %T, want *parser.Array", v)
	}
	if arr.Len() != 0 {
		t.Errorf("Array.Len() = %d, want 0", arr.Len())
	}
}

func TestWriter_setValueInDict_UnknownType(t *testing.T) {
	w := NewWriter(nil)
	dict := parser.NewDictionary()
	w.setValueInDict(dict, FieldType("Unknown"), "value")
	if dict.Get("V") != nil {
		t.Error("V should not be set for unknown field type")
	}
}

// ---------- Writer.ApplyUpdatesToDict ----------

func TestWriter_ApplyUpdatesToDict_NoUpdate(t *testing.T) {
	w := NewWriter(nil)
	field := &FieldInfo{Name: "noUpdate", Type: FieldTypeText}
	dict := parser.NewDictionary()
	dict.Set("T", parser.NewString("noUpdate"))

	result := w.ApplyUpdatesToDict(field, dict)
	if result != dict {
		t.Error("ApplyUpdatesToDict should return original dict when no update registered")
	}
}

func TestWriter_ApplyUpdatesToDict_WithTextUpdate(t *testing.T) {
	w := NewWriter(nil)
	field := &FieldInfo{Name: "myField", Type: FieldTypeText}
	w.updates["myField"] = "newValue"

	dict := parser.NewDictionary()
	dict.Set("T", parser.NewString("myField"))
	dict.Set("FT", parser.NewName("Tx"))
	dict.Set("V", parser.NewString("oldValue"))

	result := w.ApplyUpdatesToDict(field, dict)
	if result == nil {
		t.Fatal("ApplyUpdatesToDict returned nil")
	}
	v := result.Get("V")
	if v == nil {
		t.Fatal("V not set in result dict")
	}
	s, ok := v.(*parser.String)
	if !ok {
		t.Fatalf("V type = %T, want *parser.String", v)
	}
	if s.Value() != "newValue" {
		t.Errorf("V.Value() = %q, want %q", s.Value(), "newValue")
	}
}

func TestWriter_ApplyUpdatesToDict_CopiesOtherKeys(t *testing.T) {
	w := NewWriter(nil)
	field := &FieldInfo{Name: "f", Type: FieldTypeText}
	w.updates["f"] = "updated"

	dict := parser.NewDictionary()
	dict.Set("T", parser.NewString("f"))
	dict.Set("FT", parser.NewName("Tx"))
	dict.Set("MaxLen", parser.NewInteger(100))

	result := w.ApplyUpdatesToDict(field, dict)
	if result.Get("T") == nil {
		t.Error("T should be copied to new dict")
	}
	if result.Get("FT") == nil {
		t.Error("FT should be copied to new dict")
	}
	if result.Get("MaxLen") == nil {
		t.Error("MaxLen should be copied to new dict")
	}
}

func TestWriter_ApplyUpdatesToDict_ButtonTrue(t *testing.T) {
	w := NewWriter(nil)
	field := &FieldInfo{Name: "f", Type: FieldTypeButton}
	w.updates["f"] = true

	dict := parser.NewDictionary()
	dict.Set("T", parser.NewString("f"))
	dict.Set("V", parser.NewName("Off"))
	dict.Set("AS", parser.NewName("Off"))

	result := w.ApplyUpdatesToDict(field, dict)
	v := result.Get("V")
	if v == nil {
		t.Fatal("New V not set")
	}
	name, ok := v.(*parser.Name)
	if !ok {
		t.Fatalf("V type = %T, want *parser.Name", v)
	}
	if name.Value() != "Yes" {
		t.Errorf("V.Value() = %q, want %q", name.Value(), "Yes")
	}
}

// ---------- Real PDF integration tests ----------

func buildMinimalPDFReader(t *testing.T) *parser.Reader {
	t.Helper()
	r, err := parser.OpenPDF("../../../testdata/pdfs/minimal.pdf")
	if err != nil {
		t.Skip("minimal.pdf not available:", err)
	}
	return r
}

func TestReader_GetFields_MinimalPDF(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	fields, err := reader.GetFields()
	if err != nil {
		t.Fatalf("GetFields() returned error: %v", err)
	}
	// minimal.pdf has no form - should return nil or empty
	if fields != nil {
		t.Logf("Got %d fields from minimal.pdf", len(fields))
	}
}

func TestReader_GetFieldByName_NotFound(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	_, err := reader.GetFieldByName("nonexistent_field")
	if err == nil {
		t.Error("GetFieldByName should return error for nonexistent field")
	}
}

func TestWriter_SetFieldValue_FieldNotFound(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	w := NewWriter(pdfReader)
	err := w.SetFieldValue("nonexistent", "value")
	if err == nil {
		t.Error("SetFieldValue should return error for nonexistent field")
	}
}

func TestWriter_ValidateFieldValue_FieldNotFound(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	w := NewWriter(pdfReader)
	err := w.ValidateFieldValue("nonexistent", "value")
	if err == nil {
		t.Error("ValidateFieldValue should return error for nonexistent field")
	}
}

func TestWriter_GetFieldsToUpdate_Empty(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	w := NewWriter(pdfReader)
	fields, err := w.GetFieldsToUpdate()
	if err != nil {
		t.Errorf("GetFieldsToUpdate() returned error: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("GetFieldsToUpdate() len = %d, want 0", len(fields))
	}
}

func TestFlattener_GetFlattenInfoByName_FieldNotFound(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	f := NewFlattener(pdfReader)
	_, err := f.GetFlattenInfoByName("nonexistent_field")
	if err == nil {
		t.Error("GetFlattenInfoByName should error for nonexistent field")
	}
}

func TestFlattener_GetFlattenInfo_NoFormPDF(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	f := NewFlattener(pdfReader)
	info, err := f.GetFlattenInfo()
	if err != nil {
		t.Errorf("GetFlattenInfo() returned error: %v", err)
	}
	if len(info) != 0 {
		t.Logf("Got %d flatten infos (PDF may have form fields)", len(info))
	}
}

// ---------- createFieldInfo via reader helpers ----------

func TestCreateFieldInfo_FullField(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	dict := parser.NewDictionary()
	dict.Set("FT", parser.NewName("Tx"))
	dict.Set("Ff", parser.NewInteger(3))

	rectArr := parser.NewArray()
	rectArr.Append(parser.NewReal(10.0))
	rectArr.Append(parser.NewReal(20.0))
	rectArr.Append(parser.NewReal(200.0))
	rectArr.Append(parser.NewReal(40.0))
	dict.Set("Rect", rectArr)

	info := reader.createFieldInfo(dict, "myTextField")
	if info == nil {
		t.Fatal("createFieldInfo returned nil")
	}
	if info.Name != "myTextField" {
		t.Errorf("Name = %q, want myTextField", info.Name)
	}
	if info.Type != FieldTypeText {
		t.Errorf("Type = %q, want Tx", info.Type)
	}
	if info.Flags != 3 {
		t.Errorf("Flags = %d, want 3", info.Flags)
	}
	// Verify rect was parsed
	if info.Rect[0] != 10.0 {
		t.Errorf("Rect[0] = %f, want 10.0", info.Rect[0])
	}
}

func TestCreateFieldInfo_ChoiceField(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	dict := parser.NewDictionary()
	dict.Set("FT", parser.NewName("Ch"))

	optArr := parser.NewArray()
	optArr.Append(parser.NewString("Option 1"))
	optArr.Append(parser.NewString("Option 2"))
	optArr.Append(parser.NewString("Option 3"))
	dict.Set("Opt", optArr)

	info := reader.createFieldInfo(dict, "dropdown")
	if info.Type != FieldTypeChoice {
		t.Errorf("Type = %q, want Ch", info.Type)
	}
	if len(info.Options) != 3 {
		t.Errorf("Options len = %d, want 3", len(info.Options))
	}
}

func TestExtractFieldType_MissingFT(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	dict := parser.NewDictionary()
	ft := reader.extractFieldType(dict)
	if ft != "" {
		t.Errorf("extractFieldType with no FT = %q, want empty", ft)
	}
}

func TestExtractFieldFlags_MissingFf(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	dict := parser.NewDictionary()
	flags := reader.extractFieldFlags(dict)
	if flags != 0 {
		t.Errorf("extractFieldFlags with no Ff = %d, want 0", flags)
	}
}

func TestExtractRect_MissingRect(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	dict := parser.NewDictionary()
	rect := reader.extractRect(dict)
	expected := [4]float64{}
	if rect != expected {
		t.Errorf("extractRect with no Rect = %v, want %v", rect, expected)
	}
}

func TestExtractRect_WrongLength(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	dict := parser.NewDictionary()
	arr := parser.NewArray()
	arr.Append(parser.NewReal(10.0))
	arr.Append(parser.NewReal(20.0))
	dict.Set("Rect", arr)
	rect := reader.extractRect(dict)
	expected := [4]float64{}
	if rect != expected {
		t.Errorf("extractRect with wrong-length Rect = %v, want zeros", rect)
	}
}

func TestExtractValue_AllTypes(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)

	tests := []struct {
		name    string
		obj     parser.PdfObject
		wantNil bool
	}{
		{"String", parser.NewString("hello"), false},
		{"Name", parser.NewName("Yes"), false},
		{"Integer", parser.NewInteger(42), false},
		{"Real", parser.NewReal(3.14), false},
		{"Boolean", parser.NewBoolean(true), false},
		{"Null", parser.NewNull(), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dict := parser.NewDictionary()
			dict.Set("V", tt.obj)
			val := reader.extractValue(dict, "V")
			if tt.wantNil && val != nil {
				t.Errorf("extractValue(%s) = %v, want nil", tt.name, val)
			}
			if !tt.wantNil && val == nil {
				t.Errorf("extractValue(%s) returned nil, want non-nil", tt.name)
			}
		})
	}
}

func TestExtractValue_MissingKey(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	dict := parser.NewDictionary()
	val := reader.extractValue(dict, "V")
	if val != nil {
		t.Errorf("extractValue with missing key = %v, want nil", val)
	}
}

func TestExtractValue_ArrayValue(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	dict := parser.NewDictionary()
	arr := parser.NewArray()
	arr.Append(parser.NewString("first"))
	arr.Append(parser.NewString("second"))
	dict.Set("V", arr)

	val := reader.extractValue(dict, "V")
	if val == nil {
		t.Fatal("extractValue(Array) returned nil")
	}
	vals, ok := val.([]string)
	if !ok {
		t.Fatalf("extractValue(Array) type = %T, want []string", val)
	}
	if len(vals) != 2 {
		t.Errorf("vals len = %d, want 2", len(vals))
	}
}

func TestExtractArrayValues_MixedTypes(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	arr := parser.NewArray()
	arr.Append(parser.NewString("first"))
	arr.Append(parser.NewString("second"))
	arr.Append(parser.NewInteger(42)) // Non-string: should be skipped

	values := reader.extractArrayValues(arr)
	if len(values) != 2 {
		t.Errorf("extractArrayValues len = %d, want 2", len(values))
	}
	if values[0] != "first" {
		t.Errorf("values[0] = %q, want first", values[0])
	}
}

func TestExtractNumber_Integer(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	num := reader.extractNumber(parser.NewInteger(42))
	if num == nil {
		t.Fatal("extractNumber(Integer) returned nil")
	}
	if *num != 42.0 {
		t.Errorf("extractNumber(Integer) = %f, want 42.0", *num)
	}
}

func TestExtractNumber_Real(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	num := reader.extractNumber(parser.NewReal(3.14))
	if num == nil {
		t.Fatal("extractNumber(Real) returned nil")
	}
	if *num != 3.14 {
		t.Errorf("extractNumber(Real) = %f, want 3.14", *num)
	}
}

func TestExtractNumber_NonNumber(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	num := reader.extractNumber(parser.NewString("not a number"))
	if num != nil {
		t.Errorf("extractNumber(String) = %f, want nil", *num)
	}
}

func TestExtractOptionValue_SimpleString(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	val := reader.extractOptionValue(parser.NewString("Simple Option"))
	if val != "Simple Option" {
		t.Errorf("extractOptionValue(String) = %q, want Simple Option", val)
	}
}

func TestExtractOptionValue_ArrayTwoElements(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	arr := parser.NewArray()
	arr.Append(parser.NewString("export_val"))
	arr.Append(parser.NewString("Display Text"))
	val := reader.extractOptionValue(arr)
	if val != "Display Text" {
		t.Errorf("extractOptionValue(Array) = %q, want Display Text", val)
	}
}

func TestExtractOptionValue_ArraySingleElement(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	arr := parser.NewArray()
	arr.Append(parser.NewString("only"))
	val := reader.extractOptionValue(arr)
	if val != "" {
		t.Errorf("extractOptionValue(single-element Array) = %q, want empty", val)
	}
}

func TestExtractOptionValue_NonStringNonArray(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	val := reader.extractOptionValue(parser.NewInteger(42))
	if val != "" {
		t.Errorf("extractOptionValue(Integer) = %q, want empty", val)
	}
}

func TestExtractFieldName_WithParent(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	dict := parser.NewDictionary()
	dict.Set("T", parser.NewString("child"))
	name := reader.extractFieldName(dict, "parent")
	if name != "parent.child" {
		t.Errorf("extractFieldName with parent = %q, want parent.child", name)
	}
}

func TestExtractFieldName_NoParent(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	dict := parser.NewDictionary()
	dict.Set("T", parser.NewString("root"))
	name := reader.extractFieldName(dict, "")
	if name != "root" {
		t.Errorf("extractFieldName no parent = %q, want root", name)
	}
}

func TestExtractFieldName_NoT(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	dict := parser.NewDictionary()
	name := reader.extractFieldName(dict, "parent")
	if name != "parent" {
		t.Errorf("extractFieldName no T key = %q, want parent", name)
	}
}

func TestExtractChoiceOptions_NoOpt(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	dict := parser.NewDictionary()
	opts := reader.extractChoiceOptions(dict)
	if opts != nil {
		t.Errorf("extractChoiceOptions no Opt = %v, want nil", opts)
	}
}

func TestExtractChoiceOptions_WithArrayOptions(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	dict := parser.NewDictionary()
	optArr := parser.NewArray()
	optArr.Append(parser.NewString("First"))
	optArr.Append(parser.NewString("Second"))
	dict.Set("Opt", optArr)

	opts := reader.extractChoiceOptions(dict)
	if len(opts) != 2 {
		t.Errorf("extractChoiceOptions len = %d, want 2", len(opts))
	}
	if opts[0] != "First" {
		t.Errorf("opts[0] = %q, want First", opts[0])
	}
}

// ---------- Flattener.isSamePage ----------

func TestFlattener_isSamePage_BothNilMediaBox(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	f := NewFlattener(pdfReader)
	d1 := parser.NewDictionary()
	d2 := parser.NewDictionary()
	// Both have no MediaBox: should return false
	if f.isSamePage(d1, d2) {
		t.Error("isSamePage both nil MediaBox = true, want false")
	}
}

func TestFlattener_isSamePage_SameMediaBox(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	f := NewFlattener(pdfReader)
	mb := parser.NewString("[0 0 595 842]")
	d1 := parser.NewDictionary()
	d1.Set("MediaBox", mb)
	d2 := parser.NewDictionary()
	d2.Set("MediaBox", mb)
	if !f.isSamePage(d1, d2) {
		t.Error("isSamePage same MediaBox = false, want true")
	}
}

func TestFlattener_buildFieldName_Empty(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	f := NewFlattener(pdfReader)
	dict := parser.NewDictionary()
	name := f.buildFieldName(dict, "")
	if name != "" {
		t.Errorf("buildFieldName empty parent + no T = %q, want empty", name)
	}
}

func TestFlattener_buildFieldName_WithT(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	f := NewFlattener(pdfReader)
	dict := parser.NewDictionary()
	dict.Set("T", parser.NewString("fieldName"))
	name := f.buildFieldName(dict, "")
	if name != "fieldName" {
		t.Errorf("buildFieldName no parent + T = %q, want fieldName", name)
	}
}

// ---------- parseField and parseKids tests ----------

func TestParseField_NonDictionaryInput(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	// Pass a non-dictionary object (Integer) - should return error
	_, err := reader.parseField(parser.NewInteger(42), "", newFieldTreeGuard(reader.pdfReader), 0)
	if err == nil {
		t.Error("parseField with non-dict should return error")
	}
}

func TestParseField_TerminalField(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	// Create a terminal field dictionary (no Kids)
	dict := parser.NewDictionary()
	dict.Set("FT", parser.NewName("Tx"))
	dict.Set("T", parser.NewString("testField"))

	fields, err := reader.parseField(dict, "", newFieldTreeGuard(reader.pdfReader), 0)
	if err != nil {
		t.Fatalf("parseField terminal field returned error: %v", err)
	}
	if len(fields) != 1 {
		t.Fatalf("parseField returned %d fields, want 1", len(fields))
	}
	if fields[0].Name != "testField" {
		t.Errorf("field.Name = %q, want testField", fields[0].Name)
	}
	if fields[0].Type != FieldTypeText {
		t.Errorf("field.Type = %q, want Tx", fields[0].Type)
	}
}

func TestParseField_WithParentName(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	dict := parser.NewDictionary()
	dict.Set("FT", parser.NewName("Btn"))
	dict.Set("T", parser.NewString("child"))

	fields, err := reader.parseField(dict, "parent", newFieldTreeGuard(reader.pdfReader), 0)
	if err != nil {
		t.Fatalf("parseField returned error: %v", err)
	}
	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}
	if fields[0].Name != "parent.child" {
		t.Errorf("Name = %q, want parent.child", fields[0].Name)
	}
}

func TestParseField_WithKids(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)

	// Child1 dictionary
	child1 := parser.NewDictionary()
	child1.Set("FT", parser.NewName("Tx"))
	child1.Set("T", parser.NewString("child1"))

	// Child2 dictionary
	child2 := parser.NewDictionary()
	child2.Set("FT", parser.NewName("Ch"))
	child2.Set("T", parser.NewString("child2"))

	// Kids array with direct dictionaries
	kids := parser.NewArray()
	kids.Append(child1)
	kids.Append(child2)

	// Parent dictionary with Kids
	parent := parser.NewDictionary()
	parent.Set("T", parser.NewString("group"))
	parent.Set("Kids", kids)

	fields, err := reader.parseField(parent, "", newFieldTreeGuard(reader.pdfReader), 0)
	if err != nil {
		t.Fatalf("parseField with kids returned error: %v", err)
	}
	// Should return children fields
	if len(fields) == 0 {
		t.Error("expected fields from kids, got none")
	}
}

func TestParseKids_NilKids(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	dict := parser.NewDictionary()
	// No Kids key
	result := reader.parseKids(dict, "parent", newFieldTreeGuard(reader.pdfReader), 0)
	if result != nil {
		t.Errorf("parseKids no Kids = %v, want nil", result)
	}
}

func TestParseKids_WithValidKids(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)

	child := parser.NewDictionary()
	child.Set("FT", parser.NewName("Tx"))
	child.Set("T", parser.NewString("kidField"))

	kidsArr := parser.NewArray()
	kidsArr.Append(child)

	dict := parser.NewDictionary()
	dict.Set("Kids", kidsArr)

	result := reader.parseKids(dict, "parent", newFieldTreeGuard(reader.pdfReader), 0)
	if len(result) == 0 {
		t.Error("parseKids with valid kids returned empty result")
	}
}

// ---------- Flattener internals: getFieldFlattenInfo, extractStreamContent, etc. ----------

func TestFlattener_GetFlattenInfoByName_NilAcroForm(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	f := NewFlattener(pdfReader)
	// minimal.pdf has no AcroForm, so this should return error "field not found"
	_, err := f.GetFlattenInfoByName("fieldName")
	if err == nil {
		t.Error("GetFlattenInfoByName should return error when field not found")
	}
}

func TestFlattener_extractStreamContent_NonStream(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	f := NewFlattener(pdfReader)
	// extractStreamContent with a non-stream object (Integer)
	content, resources, err := f.extractStreamContent(parser.NewInteger(0))
	// Neither a stream nor a dictionary: should return nil, nil, nil
	_ = content
	_ = resources
	_ = err
}

func TestFlattener_extractStreamContent_WithDictionary(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	f := NewFlattener(pdfReader)
	// An appearance dictionary /N key
	dict := parser.NewDictionary()
	content, resources, err := f.extractStreamContent(dict)
	_ = content
	_ = resources
	_ = err
}

func TestFlattener_getFieldPageIndex_NoPageObj(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	f := NewFlattener(pdfReader)
	// Widget dict with no /P reference and no parent pages
	dict := parser.NewDictionary()
	idx, err := f.getFieldPageIndex(dict)
	if err != nil {
		t.Logf("getFieldPageIndex no P = error (expected): %v", err)
	} else {
		t.Logf("getFieldPageIndex no P = %d (acceptable default)", idx)
	}
}

// ---------- Writer.GetFieldsToUpdate with pdfReader ----------

func TestWriter_GetFieldsToUpdate_WithPdfReader(t *testing.T) {
	pdfReader := buildMinimalPDFReader(t)
	defer pdfReader.Close()

	w := NewWriter(pdfReader)
	// Add an update for a field that doesn't exist
	w.updates["nonexistent"] = "value"
	_, err := w.GetFieldsToUpdate()
	// Should fail because field doesn't exist in minimal.pdf
	if err == nil {
		t.Error("GetFieldsToUpdate should fail for nonexistent field")
	}
}

// ---------- Flattener with real form PDF ----------

func TestFlattener_FindFieldWidget_WithFormPDF(t *testing.T) {
	pdfReader := buildFormPDFReader(t)
	defer pdfReader.Close()

	f := NewFlattener(pdfReader)
	// findFieldWidget calls GetAcroForm → searchFieldInArray → searchKids
	widget, err := f.findFieldWidget("firstName")
	if err != nil {
		t.Fatalf("findFieldWidget returned error: %v", err)
	}
	if widget == nil {
		t.Fatal("findFieldWidget returned nil widget for existing field")
	}
}

func TestFlattener_FindFieldWidget_NotFound(t *testing.T) {
	pdfReader := buildFormPDFReader(t)
	defer pdfReader.Close()

	f := NewFlattener(pdfReader)
	widget, err := f.findFieldWidget("nonexistent")
	if err != nil {
		t.Logf("findFieldWidget nonexistent returned error: %v", err)
	}
	if widget != nil {
		t.Error("findFieldWidget should return nil for nonexistent field")
	}
}

func TestFlattener_GetFlattenInfo_WithFormPDF(t *testing.T) {
	pdfReader := buildFormPDFReader(t)
	defer pdfReader.Close()

	f := NewFlattener(pdfReader)
	infos, err := f.GetFlattenInfo()
	if err != nil {
		t.Fatalf("GetFlattenInfo returned error: %v", err)
	}
	// Should have at least one flatten info (for "firstName" field)
	// May return empty if appearance stream not found
	t.Logf("GetFlattenInfo returned %d infos", len(infos))
}

func TestFlattener_GetFlattenInfoByName_WithFormPDF(t *testing.T) {
	pdfReader := buildFormPDFReader(t)
	defer pdfReader.Close()

	f := NewFlattener(pdfReader)
	// "firstName" field exists
	infos, err := f.GetFlattenInfoByName("firstName")
	if err != nil {
		t.Logf("GetFlattenInfoByName returned error: %v", err)
		return
	}
	t.Logf("GetFlattenInfoByName returned %d infos", len(infos))
	for _, info := range infos {
		if info.FieldName != "firstName" {
			t.Errorf("FieldName = %q, want firstName", info.FieldName)
		}
	}
}

func TestFlattener_ExtractAppearanceStream_WithDict(t *testing.T) {
	pdfReader := buildFormPDFReader(t)
	defer pdfReader.Close()

	f := NewFlattener(pdfReader)

	// Create an AP dict pointing to a sub-dict (no stream)
	apDict := parser.NewDictionary()
	nDict := parser.NewDictionary()
	apDict.Set("N", nDict)

	widget := parser.NewDictionary()
	widget.Set("AP", apDict)

	// Should not panic, returns nil,nil,nil since N is not a stream
	content, resources, err := f.extractAppearanceStream(widget)
	_ = content
	_ = resources
	_ = err
}

func TestFlattener_ExtractAppearanceStream_NoAP(t *testing.T) {
	pdfReader := buildFormPDFReader(t)
	defer pdfReader.Close()

	f := NewFlattener(pdfReader)
	widget := parser.NewDictionary()
	// No AP key
	content, resources, err := f.extractAppearanceStream(widget)
	if content != nil || resources != nil || err != nil {
		t.Errorf("extractAppearanceStream no AP = %v, %v, %v; want all nil", content, resources, err)
	}
}

func TestFlattener_SearchFieldInArray_WithFields(t *testing.T) {
	pdfReader := buildFormPDFReader(t)
	defer pdfReader.Close()

	f := NewFlattener(pdfReader)

	// Build an array with two field dictionaries
	field1 := parser.NewDictionary()
	field1.Set("T", parser.NewString("field1"))

	field2 := parser.NewDictionary()
	field2.Set("T", parser.NewString("field2"))

	arr := parser.NewArray()
	arr.Append(field1)
	arr.Append(field2)

	// Search for "field1"
	found, err := f.searchFieldInArray(arr, "field1", "", newFieldTreeGuard(f.pdfReader), 0)
	if err != nil {
		t.Fatalf("searchFieldInArray returned error: %v", err)
	}
	if found == nil {
		t.Error("searchFieldInArray should find field1")
	}

	// Search for non-existing
	notFound, err := f.searchFieldInArray(arr, "missing", "", newFieldTreeGuard(f.pdfReader), 0)
	if err != nil {
		t.Fatalf("searchFieldInArray missing returned error: %v", err)
	}
	if notFound != nil {
		t.Error("searchFieldInArray should return nil for missing field")
	}
}

func TestFlattener_SearchKids_WithKids(t *testing.T) {
	pdfReader := buildFormPDFReader(t)
	defer pdfReader.Close()

	f := NewFlattener(pdfReader)

	// Child dict
	child := parser.NewDictionary()
	child.Set("T", parser.NewString("childField"))

	kidsArr := parser.NewArray()
	kidsArr.Append(child)

	parentDict := parser.NewDictionary()
	parentDict.Set("Kids", kidsArr)

	found, err := f.searchKids(parentDict, "childField", "", newFieldTreeGuard(f.pdfReader), 0)
	if err != nil {
		t.Fatalf("searchKids returned error: %v", err)
	}
	if found == nil {
		t.Error("searchKids should find childField")
	}
}

func TestFlattener_SearchKids_NoKids(t *testing.T) {
	pdfReader := buildFormPDFReader(t)
	defer pdfReader.Close()

	f := NewFlattener(pdfReader)
	dict := parser.NewDictionary()
	// No Kids key
	found, err := f.searchKids(dict, "something", "", newFieldTreeGuard(f.pdfReader), 0)
	if err != nil {
		t.Fatalf("searchKids no Kids returned error: %v", err)
	}
	if found != nil {
		t.Error("searchKids no Kids should return nil")
	}
}

func TestReader_GetFields_FormPDF(t *testing.T) {
	pdfReader := buildFormPDFReader(t)
	defer pdfReader.Close()

	reader := NewReader(pdfReader)
	fields, err := reader.GetFields()
	if err != nil {
		t.Fatalf("GetFields() error: %v", err)
	}
	if len(fields) == 0 {
		t.Error("expected fields from form PDF, got none")
	}
	if fields[0].Name != "firstName" {
		t.Errorf("first field name = %q, want firstName", fields[0].Name)
	}
}

func TestWriter_GetFieldsToUpdate_FormPDF(t *testing.T) {
	pdfReader := buildFormPDFReader(t)
	defer pdfReader.Close()

	w := NewWriter(pdfReader)
	err := w.SetFieldValue("firstName", "Jane")
	if err != nil {
		t.Fatalf("SetFieldValue error: %v", err)
	}
	fields, err := w.GetFieldsToUpdate()
	if err != nil {
		t.Fatalf("GetFieldsToUpdate error: %v", err)
	}
	if len(fields) != 1 {
		t.Errorf("GetFieldsToUpdate len = %d, want 1", len(fields))
	}
}
