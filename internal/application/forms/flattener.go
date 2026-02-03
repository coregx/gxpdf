// Package forms provides form field reading and manipulation functionality.
package forms

import (
	"fmt"

	"github.com/coregx/gxpdf/internal/parser"
)

// Flattener converts form fields to static page content.
//
// Form flattening removes interactivity by rendering field appearances
// directly onto pages, making them non-editable.
type Flattener struct {
	pdfReader *parser.Reader
}

// NewFlattener creates a new form flattener.
func NewFlattener(pdfReader *parser.Reader) *Flattener {
	return &Flattener{pdfReader: pdfReader}
}

// FlattenInfo contains information needed to flatten a field.
type FlattenInfo struct {
	// FieldName is the fully qualified field name.
	FieldName string

	// PageIndex is the 0-based page number where the field appears.
	PageIndex int

	// Rect is the field rectangle [x1, y1, x2, y2].
	Rect [4]float64

	// AppearanceStream is the content stream for the field appearance.
	AppearanceStream []byte

	// Resources contains resources used by the appearance stream.
	Resources *parser.Dictionary
}

// GetFlattenInfo returns flattening information for all form fields.
//
// This extracts the appearance streams and positions needed to render
// fields as static content.
func (f *Flattener) GetFlattenInfo() ([]*FlattenInfo, error) {
	if f.pdfReader == nil {
		return nil, nil
	}

	reader := NewReader(f.pdfReader)
	fields, err := reader.GetFields()
	if err != nil {
		return nil, fmt.Errorf("failed to get form fields: %w", err)
	}

	if len(fields) == 0 {
		return nil, nil
	}

	result := make([]*FlattenInfo, 0, len(fields))
	for _, field := range fields {
		info, err := f.getFieldFlattenInfo(field)
		if err != nil {
			// Skip fields that can't be flattened
			continue
		}
		if info != nil {
			result = append(result, info)
		}
	}

	return result, nil
}

// GetFlattenInfoByName returns flattening information for specific fields.
func (f *Flattener) GetFlattenInfoByName(names ...string) ([]*FlattenInfo, error) {
	reader := NewReader(f.pdfReader)
	result := make([]*FlattenInfo, 0, len(names))

	for _, name := range names {
		field, err := reader.GetFieldByName(name)
		if err != nil {
			return nil, err
		}

		info, err := f.getFieldFlattenInfo(field)
		if err != nil {
			return nil, fmt.Errorf("failed to get flatten info for %q: %w", name, err)
		}
		if info != nil {
			result = append(result, info)
		}
	}

	return result, nil
}

// getFieldFlattenInfo extracts flattening information for a single field.
func (f *Flattener) getFieldFlattenInfo(field *FieldInfo) (*FlattenInfo, error) {
	// Find the field widget annotation
	widgetDict, err := f.findFieldWidget(field.Name)
	if err != nil {
		return nil, err
	}
	if widgetDict == nil {
		return nil, nil
	}

	// Get appearance stream
	appearanceStream, resources, err := f.extractAppearanceStream(widgetDict)
	if err != nil {
		return nil, err
	}
	if appearanceStream == nil {
		return nil, nil
	}

	// Get page index
	pageIndex, err := f.getFieldPageIndex(widgetDict)
	if err != nil {
		pageIndex = 0 // Default to first page
	}

	return &FlattenInfo{
		FieldName:        field.Name,
		PageIndex:        pageIndex,
		Rect:             field.Rect,
		AppearanceStream: appearanceStream,
		Resources:        resources,
	}, nil
}

// findFieldWidget finds the widget annotation for a field.
func (f *Flattener) findFieldWidget(fieldName string) (*parser.Dictionary, error) {
	acroForm, err := f.pdfReader.GetAcroForm()
	if err != nil || acroForm == nil {
		return nil, err
	}

	fieldsObj := acroForm.Get("Fields")
	if fieldsObj == nil {
		return nil, nil
	}

	fieldsArray, err := f.pdfReader.ResolveArray(fieldsObj)
	if err != nil {
		return nil, err
	}

	return f.searchFieldInArray(fieldsArray, fieldName, "")
}

// searchFieldInArray recursively searches for a field by name.
func (f *Flattener) searchFieldInArray(arr *parser.Array, targetName, parentName string) (*parser.Dictionary, error) {
	for i := 0; i < arr.Len(); i++ {
		obj := f.pdfReader.ResolveReferences(arr.Get(i))
		dict, ok := obj.(*parser.Dictionary)
		if !ok {
			continue
		}

		fieldName := f.buildFieldName(dict, parentName)

		// Check if this is the target field
		if fieldName == targetName {
			return dict, nil
		}

		// Check children
		found, err := f.searchKids(dict, targetName, fieldName)
		if err != nil {
			return nil, err
		}
		if found != nil {
			return found, nil
		}
	}

	return nil, nil
}

// searchKids searches for a field in the Kids array of a dictionary.
func (f *Flattener) searchKids(dict *parser.Dictionary, targetName, fieldName string) (*parser.Dictionary, error) {
	kidsObj := dict.Get("Kids")
	if kidsObj == nil {
		return nil, nil
	}

	kidsArray, err := f.pdfReader.ResolveArray(kidsObj)
	if err != nil {
		return nil, nil // Ignore malformed Kids
	}

	return f.searchFieldInArray(kidsArray, targetName, fieldName)
}

// buildFieldName builds the fully qualified field name.
func (f *Flattener) buildFieldName(dict *parser.Dictionary, parentName string) string {
	fieldName := parentName
	nameObj := dict.Get("T")
	if nameObj != nil {
		nameStr, ok := f.pdfReader.ResolveReferences(nameObj).(*parser.String)
		if ok {
			if fieldName != "" {
				fieldName += "."
			}
			fieldName += nameStr.Value()
		}
	}
	return fieldName
}

// extractAppearanceStream extracts the normal appearance stream from a widget.
func (f *Flattener) extractAppearanceStream(widgetDict *parser.Dictionary) ([]byte, *parser.Dictionary, error) {
	apObj := widgetDict.Get("AP")
	if apObj == nil {
		return nil, nil, nil
	}

	apObj = f.pdfReader.ResolveReferences(apObj)
	apDict, ok := apObj.(*parser.Dictionary)
	if !ok {
		return nil, nil, nil
	}

	// Get normal appearance
	nObj := apDict.Get("N")
	if nObj == nil {
		return nil, nil, nil
	}

	return f.extractStreamContent(nObj)
}

// extractStreamContent extracts content from a stream object or appearance dictionary.
func (f *Flattener) extractStreamContent(obj parser.PdfObject) ([]byte, *parser.Dictionary, error) {
	obj = f.pdfReader.ResolveReferences(obj)

	switch v := obj.(type) {
	case *parser.Stream:
		content, err := v.Decode()
		if err != nil {
			return nil, nil, err
		}
		var resources *parser.Dictionary
		if v.Dictionary() != nil {
			resObj := v.Dictionary().Get("Resources")
			if resObj != nil {
				resources, _ = f.pdfReader.ResolveReferences(resObj).(*parser.Dictionary)
			}
		}
		return content, resources, nil

	case *parser.Dictionary:
		// Appearance state dictionary - get current state
		// This handles checkboxes/radios with multiple states
		for _, key := range v.Keys() {
			stateObj := v.Get(key)
			content, resources, err := f.extractStreamContent(stateObj)
			if err == nil && content != nil {
				return content, resources, nil
			}
		}
	}

	return nil, nil, nil
}

// getFieldPageIndex determines which page a field is on.
func (f *Flattener) getFieldPageIndex(widgetDict *parser.Dictionary) (int, error) {
	// Check if widget has a /P (page) reference
	pObj := widgetDict.Get("P")
	if pObj == nil {
		return 0, nil
	}

	pObj = f.pdfReader.ResolveReferences(pObj)
	pageDict, ok := pObj.(*parser.Dictionary)
	if !ok {
		return 0, nil
	}

	// Find this page in the page tree
	pageCount, err := f.pdfReader.GetPageCount()
	if err != nil {
		return 0, err
	}

	for i := 0; i < pageCount; i++ {
		page, err := f.pdfReader.GetPage(i)
		if err != nil {
			continue
		}
		// Compare by checking if it's the same dictionary
		if f.isSamePage(page, pageDict) {
			return i, nil
		}
	}

	return 0, nil
}

// isSamePage checks if two page dictionaries are the same.
func (f *Flattener) isSamePage(page1, page2 *parser.Dictionary) bool {
	// Compare by MediaBox as a simple heuristic
	// A more robust solution would compare object references
	mb1 := page1.Get("MediaBox")
	mb2 := page2.Get("MediaBox")
	if mb1 == nil || mb2 == nil {
		return false
	}
	// Use String() method for comparison
	s1, ok1 := mb1.(interface{ String() string })
	s2, ok2 := mb2.(interface{ String() string })
	if ok1 && ok2 {
		return s1.String() == s2.String()
	}
	return false
}

// CanFlatten returns true if the document has fields that can be flattened.
func (f *Flattener) CanFlatten() bool {
	info, err := f.GetFlattenInfo()
	return err == nil && len(info) > 0
}
