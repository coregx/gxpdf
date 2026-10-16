// Package forms provides form field reading and manipulation functionality.
package forms

import (
	"fmt"

	"github.com/coregx/gxpdf/internal/parser"
)

// FieldType represents the type of a form field.
type FieldType string

const (
	FieldTypeText      FieldType = "Tx"  // Text field
	FieldTypeButton    FieldType = "Btn" // Button (checkbox, radio, pushbutton)
	FieldTypeChoice    FieldType = "Ch"  // Choice (list, combo)
	FieldTypeSignature FieldType = "Sig" // Signature
)

// FieldInfo contains information about a form field.
type FieldInfo struct {
	// Name is the fully qualified field name.
	Name string

	// Type is the field type (Tx, Btn, Ch, Sig).
	Type FieldType

	// Value is the current field value.
	Value interface{}

	// DefaultValue is the default field value.
	DefaultValue interface{}

	// Flags is the field flags bitmask (Ff).
	Flags int

	// Rect is the field rectangle [x1, y1, x2, y2].
	Rect [4]float64

	// Options contains choice field options.
	Options []string
}

// Reader reads form fields from a PDF document.
type Reader struct {
	pdfReader *parser.Reader
}

// NewReader creates a new form field reader.
func NewReader(pdfReader *parser.Reader) *Reader {
	return &Reader{pdfReader: pdfReader}
}

// GetFields returns all form fields in the document.
func (r *Reader) GetFields() ([]*FieldInfo, error) {
	acroForm, err := r.pdfReader.GetAcroForm()
	if err != nil {
		return nil, fmt.Errorf("failed to get AcroForm: %w", err)
	}

	if acroForm == nil {
		return nil, nil // No form fields
	}

	// Get the Fields array
	fieldsObj := acroForm.Get("Fields")
	if fieldsObj == nil {
		return nil, nil // No fields
	}

	fieldsArray, err := r.pdfReader.ResolveArray(fieldsObj)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve Fields array: %w", err)
	}

	guard := newFieldTreeGuard(r.pdfReader)
	var fields []*FieldInfo
	for i := 0; i < fieldsArray.Len(); i++ {
		fieldObj := fieldsArray.Get(i)
		fieldInfos, err := r.parseField(fieldObj, "", guard, 0)
		if err != nil {
			continue // Skip invalid fields
		}
		fields = append(fields, fieldInfos...)
	}

	return fields, nil
}

// GetFieldByName returns a specific field by its fully qualified name.
func (r *Reader) GetFieldByName(name string) (*FieldInfo, error) {
	fields, err := r.GetFields()
	if err != nil {
		return nil, err
	}

	for _, field := range fields {
		if field.Name == name {
			return field, nil
		}
	}

	return nil, fmt.Errorf("field not found: %s", name)
}

// parseField parses a field dictionary and its children.
//
// depth is the nesting level of obj in the field hierarchy (0 for an entry of
// /Fields); guard bounds the walk (see fieldTreeGuard).
func (r *Reader) parseField(obj parser.PdfObject, parentName string, guard *fieldTreeGuard, depth int) ([]*FieldInfo, error) {
	dict, ok := guard.enter(obj, depth)
	if !ok {
		return nil, fmt.Errorf("field is not a dictionary or must not be visited")
	}

	fieldName := r.extractFieldName(dict, parentName)

	// Handle child fields
	if fields := r.parseKids(dict, fieldName, guard, depth); fields != nil {
		return fields, nil
	}

	// Terminal field - create FieldInfo
	info := r.createFieldInfo(dict, fieldName)
	return []*FieldInfo{info}, nil
}

// extractFieldName builds the fully qualified field name.
func (r *Reader) extractFieldName(dict *parser.Dictionary, parentName string) string {
	fieldName := parentName
	nameObj := dict.Get("T")
	if nameObj == nil {
		return fieldName
	}

	nameStr, ok := r.pdfReader.ResolveReferences(nameObj).(*parser.String)
	if !ok {
		return fieldName
	}

	if fieldName != "" {
		fieldName += "."
	}
	fieldName += nameStr.Value()
	return fieldName
}

// parseKids parses child fields if present.
// Returns nil if there are no kids (terminal field).
// depth is the nesting level of dict; its kids are visited at depth+1.
func (r *Reader) parseKids(dict *parser.Dictionary, fieldName string, guard *fieldTreeGuard, depth int) []*FieldInfo {
	kidsObj := dict.Get("Kids")
	if kidsObj == nil {
		return nil
	}

	kidsArray, err := r.pdfReader.ResolveArray(kidsObj)
	if err != nil {
		return nil
	}

	var fields []*FieldInfo
	for i := 0; i < kidsArray.Len(); i++ {
		kidObj := kidsArray.Get(i)
		kidFields, err := r.parseField(kidObj, fieldName, guard, depth+1)
		if err != nil {
			continue
		}
		fields = append(fields, kidFields...)
	}
	return fields
}

// createFieldInfo creates a FieldInfo from a field dictionary.
func (r *Reader) createFieldInfo(dict *parser.Dictionary, fieldName string) *FieldInfo {
	info := &FieldInfo{
		Name:         fieldName,
		Type:         r.extractFieldType(dict),
		Flags:        r.extractFieldFlags(dict),
		Value:        r.extractValue(dict, "V"),
		DefaultValue: r.extractValue(dict, "DV"),
		Rect:         r.extractRect(dict),
	}

	if info.Type == FieldTypeChoice {
		info.Options = r.extractChoiceOptions(dict)
	}

	return info
}

// extractFieldType extracts the field type from a dictionary.
func (r *Reader) extractFieldType(dict *parser.Dictionary) FieldType {
	ftObj := dict.Get("FT")
	if ftObj == nil {
		return ""
	}

	ftName, ok := r.pdfReader.ResolveReferences(ftObj).(*parser.Name)
	if !ok {
		return ""
	}

	return FieldType(ftName.Value())
}

// extractFieldFlags extracts the field flags from a dictionary.
func (r *Reader) extractFieldFlags(dict *parser.Dictionary) int {
	ffObj := dict.Get("Ff")
	if ffObj == nil {
		return 0
	}

	ffInt, ok := r.pdfReader.ResolveReferences(ffObj).(*parser.Integer)
	if !ok {
		return 0
	}

	return int(ffInt.Value())
}

// extractRect extracts the field rectangle from a dictionary.
func (r *Reader) extractRect(dict *parser.Dictionary) [4]float64 {
	var rect [4]float64

	rectObj := dict.Get("Rect")
	if rectObj == nil {
		return rect
	}

	rectArray, err := r.pdfReader.ResolveArray(rectObj)
	if err != nil || rectArray.Len() != 4 {
		return rect
	}

	for i := 0; i < 4; i++ {
		if num := r.extractNumber(rectArray.Get(i)); num != nil {
			rect[i] = *num
		}
	}

	return rect
}

// extractChoiceOptions extracts options for choice fields.
func (r *Reader) extractChoiceOptions(dict *parser.Dictionary) []string {
	optObj := dict.Get("Opt")
	if optObj == nil {
		return nil
	}

	optArray, err := r.pdfReader.ResolveArray(optObj)
	if err != nil {
		return nil
	}

	var options []string
	for i := 0; i < optArray.Len(); i++ {
		opt := r.extractOptionValue(optArray.Get(i))
		if opt != "" {
			options = append(options, opt)
		}
	}

	return options
}

// extractOptionValue extracts a single option value.
func (r *Reader) extractOptionValue(obj parser.PdfObject) string {
	obj = r.pdfReader.ResolveReferences(obj)

	// Simple string option
	if optStr, ok := obj.(*parser.String); ok {
		return optStr.Value()
	}

	// Array option: [export_value, display_value]
	optArr, ok := obj.(*parser.Array)
	if !ok || optArr.Len() < 2 {
		return ""
	}

	displayObj := r.pdfReader.ResolveReferences(optArr.Get(1))
	if displayStr, ok := displayObj.(*parser.String); ok {
		return displayStr.Value()
	}

	return ""
}

// extractValue extracts a field value from a dictionary.
func (r *Reader) extractValue(dict *parser.Dictionary, key string) interface{} {
	obj := dict.Get(key)
	if obj == nil {
		return nil
	}

	obj = r.pdfReader.ResolveReferences(obj)

	switch v := obj.(type) {
	case *parser.String:
		return v.Value()
	case *parser.Name:
		return v.Value()
	case *parser.Integer:
		return v.Value()
	case *parser.Real:
		return v.Value()
	case *parser.Boolean:
		return v.Value()
	case *parser.Array:
		return r.extractArrayValues(v)
	default:
		return nil
	}
}

// extractArrayValues extracts string values from an array.
func (r *Reader) extractArrayValues(arr *parser.Array) []string {
	var values []string
	for i := 0; i < arr.Len(); i++ {
		item := r.pdfReader.ResolveReferences(arr.Get(i))
		if str, ok := item.(*parser.String); ok {
			values = append(values, str.Value())
		}
	}
	return values
}

// extractNumber extracts a numeric value from a PDF object.
func (r *Reader) extractNumber(obj parser.PdfObject) *float64 {
	obj = r.pdfReader.ResolveReferences(obj)

	switch v := obj.(type) {
	case *parser.Integer:
		val := float64(v.Value())
		return &val
	case *parser.Real:
		val := v.Value()
		return &val
	default:
		return nil
	}
}
