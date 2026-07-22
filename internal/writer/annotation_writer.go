package writer

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/coregx/gxpdf/internal/document"
)

// WriteAllAnnotations writes all annotations from a page and returns annotation objects.
//
// This handles link, text, markup, and stamp annotations.
//
// Returns:
//   - annotObjs: Array of annotation indirect objects
//   - annotRefs: Array of annotation object numbers (for /Annots array)
//   - error: Any error that occurred
func (w *PdfWriter) WriteAllAnnotations(
	page *document.Page,
) ([]*IndirectObject, []int, error) {
	var annotObjs []*IndirectObject
	var annotRefs []int

	// Write link annotations.
	linkAnnots := page.LinkAnnotations()
	if len(linkAnnots) > 0 {
		objs, refs, err := w.writeLinkAnnotations(linkAnnots)
		if err != nil {
			return nil, nil, err
		}
		annotObjs = append(annotObjs, objs...)
		annotRefs = append(annotRefs, refs...)
	}

	// Write text annotations.
	textAnnots := page.TextAnnotations()
	if len(textAnnots) > 0 {
		objs, refs, err := w.writeTextAnnotations(textAnnots)
		if err != nil {
			return nil, nil, err
		}
		annotObjs = append(annotObjs, objs...)
		annotRefs = append(annotRefs, refs...)
	}

	// Write markup annotations.
	markupAnnots := page.MarkupAnnotations()
	if len(markupAnnots) > 0 {
		objs, refs, err := w.writeMarkupAnnotations(markupAnnots)
		if err != nil {
			return nil, nil, err
		}
		annotObjs = append(annotObjs, objs...)
		annotRefs = append(annotRefs, refs...)
	}

	// Write stamp annotations.
	stampAnnots := page.StampAnnotations()
	if len(stampAnnots) > 0 {
		objs, refs, err := w.writeStampAnnotations(stampAnnots)
		if err != nil {
			return nil, nil, err
		}
		annotObjs = append(annotObjs, objs...)
		annotRefs = append(annotRefs, refs...)
	}

	return annotObjs, annotRefs, nil
}

// WriteAnnotations writes link annotations and returns annotation objects.
//
// DEPRECATED: Use WriteAllAnnotations instead for pages, or writeLinkAnnotations for specific link annotations.
// This method is kept for backward compatibility.
//
// For each annotation, creates an indirect object with the annotation dictionary.
//
// Returns:
//   - annotObjs: Array of annotation indirect objects
//   - annotRefs: Array of annotation object numbers (for /Annots array)
//   - error: Any error that occurred
func (w *PdfWriter) WriteAnnotations(
	annotations []*document.LinkAnnotation,
) ([]*IndirectObject, []int, error) {
	return w.writeLinkAnnotations(annotations)
}

// writeLinkAnnotations writes link annotations.
func (w *PdfWriter) writeLinkAnnotations(
	annotations []*document.LinkAnnotation,
) ([]*IndirectObject, []int, error) {
	if len(annotations) == 0 {
		return nil, nil, nil
	}

	annotObjs := make([]*IndirectObject, 0, len(annotations))
	annotRefs := make([]int, 0, len(annotations))

	for _, annot := range annotations {
		// Allocate object number for this annotation.
		objNum := w.allocateObjNum()
		annotRefs = append(annotRefs, objNum)

		// Create annotation object.
		annotObj, err := createLinkAnnotationObject(objNum, annot)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create link annotation %d: %w", objNum, err)
		}

		annotObjs = append(annotObjs, annotObj)
	}

	return annotObjs, annotRefs, nil
}

// writeTextAnnotations writes text annotations.
func (w *PdfWriter) writeTextAnnotations(
	annotations []*document.TextAnnotation,
) ([]*IndirectObject, []int, error) {
	if len(annotations) == 0 {
		return nil, nil, nil
	}

	annotObjs := make([]*IndirectObject, 0, len(annotations))
	annotRefs := make([]int, 0, len(annotations))

	for _, annot := range annotations {
		objNum := w.allocateObjNum()
		annotRefs = append(annotRefs, objNum)

		annotObj := createTextAnnotationObject(objNum, annot)
		annotObjs = append(annotObjs, annotObj)
	}

	return annotObjs, annotRefs, nil
}

// writeMarkupAnnotations writes markup annotations.
func (w *PdfWriter) writeMarkupAnnotations(
	annotations []*document.MarkupAnnotation,
) ([]*IndirectObject, []int, error) {
	if len(annotations) == 0 {
		return nil, nil, nil
	}

	annotObjs := make([]*IndirectObject, 0, len(annotations))
	annotRefs := make([]int, 0, len(annotations))

	for _, annot := range annotations {
		objNum := w.allocateObjNum()
		annotRefs = append(annotRefs, objNum)

		annotObj := createMarkupAnnotationObject(objNum, annot)
		annotObjs = append(annotObjs, annotObj)
	}

	return annotObjs, annotRefs, nil
}

// writeStampAnnotations writes stamp annotations.
func (w *PdfWriter) writeStampAnnotations(
	annotations []*document.StampAnnotation,
) ([]*IndirectObject, []int, error) {
	if len(annotations) == 0 {
		return nil, nil, nil
	}

	annotObjs := make([]*IndirectObject, 0, len(annotations))
	annotRefs := make([]int, 0, len(annotations))

	for _, annot := range annotations {
		objNum := w.allocateObjNum()
		annotRefs = append(annotRefs, objNum)

		annotObj := createStampAnnotationObject(objNum, annot)
		annotObjs = append(annotObjs, annotObj)
	}

	return annotObjs, annotRefs, nil
}

// createLinkAnnotationObject creates a link annotation indirect object.
//
// PDF annotation format (external link):
//
//	<<
//	  /Type /Annot
//	  /Subtype /Link
//	  /Rect [x1 y1 x2 y2]
//	  /Border [0 0 0]
//	  /A << /Type /Action /S /URI /URI (https://example.com) >>
//	>>
//
// PDF annotation format (internal link):
//
//	<<
//	  /Type /Annot
//	  /Subtype /Link
//	  /Rect [x1 y1 x2 y2]
//	  /Border [0 0 0]
//	  /Dest [pageRef 0 R /Fit]
//	>>
func createLinkAnnotationObject(objNum int, annot *document.LinkAnnotation) (*IndirectObject, error) {
	var buf bytes.Buffer

	buf.WriteString("<<")
	buf.WriteString(" /Type /Annot")
	buf.WriteString(" /Subtype /Link")

	// Write rectangle (clickable area).
	buf.WriteString(fmt.Sprintf(
		" /Rect [%.2f %.2f %.2f %.2f]",
		annot.Rect[0], annot.Rect[1], annot.Rect[2], annot.Rect[3],
	))

	// Write border (0 0 0 = no visible border, or use BorderWidth).
	buf.WriteString(fmt.Sprintf(" /Border [0 0 %.2f]", annot.BorderWidth))

	// Write action or destination based on link type.
	if annot.IsInternal {
		// Internal link: /Dest [pageRef 0 R /Fit]
		// Note: We need the actual page object reference.
		// For now, we use pageNum + 1 as a placeholder.
		// This will need to be updated when we have actual page references.
		pageRef := annot.DestPage + 1 // Placeholder: assume page objects start at 1
		buf.WriteString(fmt.Sprintf(" /Dest [%d 0 R /Fit]", pageRef))
	} else {
		// External link: /A << /Type /Action /S /URI /URI (url) >>
		buf.WriteString(" /A <<")
		buf.WriteString(" /Type /Action")
		buf.WriteString(" /S /URI")
		// Escape the URI string.
		escapedURI := EscapePDFString(annot.URI)
		buf.WriteString(fmt.Sprintf(" /URI (%s)", escapedURI))
		buf.WriteString(" >>")
	}

	buf.WriteString(" >>")

	return NewIndirectObject(objNum, 0, buf.Bytes()), nil
}

// createTextAnnotationObject creates a text annotation indirect object.
//
// PDF annotation format:
//
//	<<
//	  /Type /Annot
//	  /Subtype /Text
//	  /Rect [x1 y1 x2 y2]
//	  /Contents (This is a comment)
//	  /T (John Doe)
//	  /C [1 1 0]
//	  /Open false
//	>>
func createTextAnnotationObject(objNum int, annot *document.TextAnnotation) *IndirectObject {
	var buf bytes.Buffer

	buf.WriteString("<<")
	buf.WriteString(" /Type /Annot")
	buf.WriteString(" /Subtype /Text")

	// Rectangle.
	buf.WriteString(fmt.Sprintf(
		" /Rect [%.2f %.2f %.2f %.2f]",
		annot.Rect[0], annot.Rect[1], annot.Rect[2], annot.Rect[3],
	))

	// Contents (pop-up text).
	if annot.Contents != "" {
		buf.WriteString(fmt.Sprintf(" /Contents %s", EncodeTextString(annot.Contents)))
	}

	// Title (author).
	if annot.Title != "" {
		buf.WriteString(fmt.Sprintf(" /T %s", EncodeTextString(annot.Title)))
	}

	// Color.
	buf.WriteString(fmt.Sprintf(" /C [%.2f %.2f %.2f]",
		annot.Color[0], annot.Color[1], annot.Color[2]))

	// Open flag.
	if annot.Open {
		buf.WriteString(" /Open true")
	} else {
		buf.WriteString(" /Open false")
	}

	buf.WriteString(" >>")

	return NewIndirectObject(objNum, 0, buf.Bytes())
}

// createMarkupAnnotationObject creates a markup annotation indirect object.
//
// PDF annotation format (highlight):
//
//	<<
//	  /Type /Annot
//	  /Subtype /Highlight
//	  /Rect [x1 y1 x2 y2]
//	  /QuadPoints [x1 y1 x2 y2 x3 y3 x4 y4]
//	  /C [1 1 0]
//	  /T (John Doe)
//	  /Contents (Note text)
//	>>
func createMarkupAnnotationObject(objNum int, annot *document.MarkupAnnotation) *IndirectObject {
	var buf bytes.Buffer

	buf.WriteString("<<")
	buf.WriteString(" /Type /Annot")

	// Subtype based on annotation type.
	switch annot.Type {
	case document.AnnotationTypeHighlight:
		buf.WriteString(" /Subtype /Highlight")
	case document.AnnotationTypeUnderline:
		buf.WriteString(" /Subtype /Underline")
	case document.AnnotationTypeStrikeOut:
		buf.WriteString(" /Subtype /StrikeOut")
	default:
		buf.WriteString(" /Subtype /Highlight") // Default
	}

	// Rectangle.
	buf.WriteString(fmt.Sprintf(
		" /Rect [%.2f %.2f %.2f %.2f]",
		annot.Rect[0], annot.Rect[1], annot.Rect[2], annot.Rect[3],
	))

	// QuadPoints.
	if len(annot.QuadPoints) > 0 {
		buf.WriteString(" /QuadPoints [")
		var parts []string
		for _, quad := range annot.QuadPoints {
			parts = append(parts, fmt.Sprintf("%.2f %.2f %.2f %.2f %.2f %.2f %.2f %.2f",
				quad[0], quad[1], quad[2], quad[3], quad[4], quad[5], quad[6], quad[7]))
		}
		buf.WriteString(strings.Join(parts, " "))
		buf.WriteString("]")
	}

	// Color.
	buf.WriteString(fmt.Sprintf(" /C [%.2f %.2f %.2f]",
		annot.Color[0], annot.Color[1], annot.Color[2]))

	// Title (author).
	if annot.Title != "" {
		buf.WriteString(fmt.Sprintf(" /T %s", EncodeTextString(annot.Title)))
	}

	// Contents (note).
	if annot.Contents != "" {
		buf.WriteString(fmt.Sprintf(" /Contents %s", EncodeTextString(annot.Contents)))
	}

	buf.WriteString(" >>")

	return NewIndirectObject(objNum, 0, buf.Bytes())
}

// createStampAnnotationObject creates a stamp annotation indirect object.
//
// PDF annotation format:
//
//	<<
//	  /Type /Annot
//	  /Subtype /Stamp
//	  /Rect [x1 y1 x2 y2]
//	  /Name /Approved
//	  /C [0 1 0]
//	  /T (John Doe)
//	  /Contents (Approved on 2025-01-06)
//	>>
func createStampAnnotationObject(objNum int, annot *document.StampAnnotation) *IndirectObject {
	var buf bytes.Buffer

	buf.WriteString("<<")
	buf.WriteString(" /Type /Annot")
	buf.WriteString(" /Subtype /Stamp")

	// Rectangle.
	buf.WriteString(fmt.Sprintf(
		" /Rect [%.2f %.2f %.2f %.2f]",
		annot.Rect[0], annot.Rect[1], annot.Rect[2], annot.Rect[3],
	))

	// Name (stamp type).
	buf.WriteString(fmt.Sprintf(" /Name /%s", annot.Name))

	// Color.
	buf.WriteString(fmt.Sprintf(" /C [%.2f %.2f %.2f]",
		annot.Color[0], annot.Color[1], annot.Color[2]))

	// Title (author).
	if annot.Title != "" {
		buf.WriteString(fmt.Sprintf(" /T %s", EncodeTextString(annot.Title)))
	}

	// Contents (note).
	if annot.Contents != "" {
		buf.WriteString(fmt.Sprintf(" /Contents %s", EncodeTextString(annot.Contents)))
	}

	buf.WriteString(" >>")

	return NewIndirectObject(objNum, 0, buf.Bytes())
}
