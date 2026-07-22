package writer

import (
	"bytes"
	"fmt"

	"github.com/coregx/gxpdf/internal/document"
)

// writeFormFields writes form field widget annotations.
//
// Form fields are special annotations that combine field properties with
// widget appearance.
//
// Returns:
//   - formFieldObjs: Array of form field indirect objects
//   - formFieldRefs: Array of form field object numbers (for /Fields and /Annots arrays)
//   - error: Any error that occurred
func (w *PdfWriter) writeFormFields(
	fields []*document.FormField,
) ([]*IndirectObject, []int, error) {
	if len(fields) == 0 {
		return nil, nil, nil
	}

	fieldObjs := make([]*IndirectObject, 0, len(fields))
	fieldRefs := make([]int, 0, len(fields))

	for _, field := range fields {
		objNum := w.allocateObjNum()
		fieldRefs = append(fieldRefs, objNum)

		fieldObj := createFormFieldObject(objNum, field)
		fieldObjs = append(fieldObjs, fieldObj)
	}

	return fieldObjs, fieldRefs, nil
}

// createFormFieldObject creates a form field widget annotation indirect object.
//
// PDF form field format (text field example):
//
//	<<
//	  /Type /Annot
//	  /Subtype /Widget
//	  /FT /Tx                   % Field Type: Text
//	  /T (fieldName)            % Field name
//	  /V (value)                % Field value
//	  /DV (defaultValue)        % Default value
//	  /Rect [x1 y1 x2 y2]       % Position
//	  /F 4                      % Annotation flags (4 = Print)
//	  /Ff 0                     % Field flags
//	  /DA (/Helv 12 Tf 0 g)     % Default appearance
//	  /MaxLen 100               % Max length (text fields only)
//	  /MK <<                    % Appearance characteristics
//	    /BC [0 0 0]             % Border color
//	    /BG [1 1 1]             % Background color
//	  >>
//	>>
func createFormFieldObject(objNum int, field *document.FormField) *IndirectObject {
	var buf bytes.Buffer

	buf.WriteString("<<")
	buf.WriteString(" /Type /Annot")
	buf.WriteString(" /Subtype /Widget")

	// Field type (/FT)
	buf.WriteString(fmt.Sprintf(" /FT /%s", field.FieldType()))

	// Field name (/T)
	buf.WriteString(fmt.Sprintf(" /T %s", EncodeTextString(field.Name())))

	// Field value (/V)
	if field.Value() != "" {
		buf.WriteString(fmt.Sprintf(" /V %s", EncodeTextString(field.Value())))
	}

	// Default value (/DV)
	if field.DefaultValue() != "" {
		buf.WriteString(fmt.Sprintf(" /DV %s", EncodeTextString(field.DefaultValue())))
	}

	// Alternate text for accessibility (/TU)
	if field.AlternateText() != "" {
		buf.WriteString(fmt.Sprintf(" /TU %s", EncodeTextString(field.AlternateText())))
	}

	// Rectangle (/Rect)
	rect := field.Rect()
	buf.WriteString(fmt.Sprintf(
		" /Rect [%.2f %.2f %.2f %.2f]",
		rect[0], rect[1], rect[2], rect[3],
	))

	// Annotation flags (/F) - 4 = Print flag
	buf.WriteString(fmt.Sprintf(" /F %d", field.AnnotationFlags()))

	// Field flags (/Ff)
	if field.Flags() != 0 {
		buf.WriteString(fmt.Sprintf(" /Ff %d", field.Flags()))
	}

	// Default appearance (/DA)
	if field.Appearance() != "" {
		buf.WriteString(fmt.Sprintf(" /DA (%s)", field.Appearance()))
	}

	// Text field specific: MaxLen
	if field.FieldType() == "Tx" && field.MaxLength() > 0 {
		buf.WriteString(fmt.Sprintf(" /MaxLen %d", field.MaxLength()))
	}

	// Appearance characteristics (/MK)
	if field.BorderColor() != nil || field.FillColor() != nil {
		buf.WriteString(" /MK <<")

		// Border color (/BC)
		if bc := field.BorderColor(); bc != nil {
			buf.WriteString(fmt.Sprintf(" /BC [%.2f %.2f %.2f]", bc[0], bc[1], bc[2]))
		}

		// Background fill color (/BG)
		if fc := field.FillColor(); fc != nil {
			buf.WriteString(fmt.Sprintf(" /BG [%.2f %.2f %.2f]", fc[0], fc[1], fc[2]))
		}

		buf.WriteString(" >>")
	}

	buf.WriteString(" >>")

	return NewIndirectObject(objNum, 0, buf.Bytes())
}

// CreateAcroFormDict creates the AcroForm dictionary for the catalog.
//
// The AcroForm dictionary is required when a document contains form fields.
// It includes:
//   - /Fields: Array of all form field references
//   - /NeedAppearances: true (let the PDF reader generate appearances)
//   - /DR: Default resources (fonts)
//   - /DA: Default appearance string
//
// PDF structure:
//
//	<<
//	  /Fields [101 0 R 102 0 R ...]
//	  /NeedAppearances true
//	  /DR <<
//	    /Font <<
//	      /Helv 5 0 R
//	      /Cour 6 0 R
//	      /TiRo 7 0 R
//	    >>
//	  >>
//	  /DA (/Helv 12 Tf 0 g)
//	>>
//
// Parameters:
//   - fieldRefs: Array of form field object numbers
//   - fontObjNum: Object number of Helvetica font (for default appearance)
//
// Returns the AcroForm dictionary as a PDF object string.
func CreateAcroFormDict(fieldRefs []int, fontObjNum int) string {
	if len(fieldRefs) == 0 {
		return ""
	}

	var buf bytes.Buffer

	buf.WriteString("<<")

	// Fields array
	buf.WriteString(" /Fields [")
	for i, ref := range fieldRefs {
		if i > 0 {
			buf.WriteString(" ")
		}
		buf.WriteString(fmt.Sprintf("%d 0 R", ref))
	}
	buf.WriteString("]")

	// NeedAppearances flag (let PDF reader generate field appearances)
	buf.WriteString(" /NeedAppearances true")

	// Default resources (fonts)
	if fontObjNum > 0 {
		buf.WriteString(" /DR <<")
		buf.WriteString(" /Font <<")
		buf.WriteString(fmt.Sprintf(" /Helv %d 0 R", fontObjNum))
		buf.WriteString(" >>")
		buf.WriteString(" >>")
	}

	// Default appearance string
	buf.WriteString(" /DA (/Helv 12 Tf 0 g)")

	buf.WriteString(" >>")

	return buf.String()
}
