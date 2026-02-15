package writer

import (
	"bytes"
	"fmt"

	"github.com/coregx/gxpdf/internal/document"
)

// hasTextBlockOps checks if any graphics operations contain TextBlock (type 22).
func hasTextBlockOps(graphicsOps []GraphicsOp) bool {
	for _, gop := range graphicsOps {
		if gop.Type == 22 { // TextBlock
			return true
		}
	}
	return false
}

// createPageTreeWithContent creates the Pages tree with content operations.
//
// This version accepts page content operations and generates content streams.
//
// Returns:
//   - objects: All page-related objects (Pages root + Page objects + Content streams + Fonts)
//   - rootRef: Object number of the Pages root
//   - error: Any error that occurred
func (w *PdfWriter) createPageTreeWithContent(
	doc *document.Document,
	pageContents map[int][]TextOp,
) ([]*IndirectObject, int, error) {
	objects := make([]*IndirectObject, 0)

	// Allocate object number for Pages root
	pagesRootRef := w.allocateObjNum()

	// Create individual Page objects with content
	pageRefs := make([]int, 0, doc.PageCount())
	for i := 0; i < doc.PageCount(); i++ {
		page, err := doc.Page(i)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get page %d: %w", i, err)
		}

		pageRef := w.allocateObjNum()
		pageRefs = append(pageRefs, pageRef)

		// Get content operations for this page
		textOps := pageContents[i]

		// Create page with content
		pageObj, contentObj, fontObjs := w.createPageWithContent(page, pageRef, pagesRootRef, textOps)
		objects = append(objects, pageObj)

		// Add content stream object if present
		if contentObj != nil {
			objects = append(objects, contentObj)
		}

		// Add font objects
		objects = append(objects, fontObjs...)
	}

	// Create Pages root object
	pagesRootObj := w.createPagesRoot(pagesRootRef, pageRefs, doc.PageCount())
	objects = append([]*IndirectObject{pagesRootObj}, objects...)

	return objects, pagesRootRef, nil
}

// createPageTreeWithAllContent creates the Pages tree with both text and graphics content.
//
// Returns:
//   - objects: All page-related objects
//   - rootRef: Object number of the Pages root
//   - error: Any error that occurred
func (w *PdfWriter) createPageTreeWithAllContent(
	doc *document.Document,
	textContents map[int][]TextOp,
	graphicsContents map[int][]GraphicsOp,
) ([]*IndirectObject, int, error) {
	objects := make([]*IndirectObject, 0)

	// Allocate object number for Pages root
	pagesRootRef := w.allocateObjNum()

	// Create individual Page objects with content
	pageRefs := make([]int, 0, doc.PageCount())
	for i := 0; i < doc.PageCount(); i++ {
		page, err := doc.Page(i)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get page %d: %w", i, err)
		}

		pageRef := w.allocateObjNum()
		pageRefs = append(pageRefs, pageRef)

		// Get content operations for this page
		textOps := textContents[i]
		graphicsOps := graphicsContents[i]

		// Create page with all content
		pageObj, contentObj, fontObjs := w.createPageWithAllContent(page, pageRef, pagesRootRef, textOps, graphicsOps)
		objects = append(objects, pageObj)

		// Add content stream object if present
		if contentObj != nil {
			objects = append(objects, contentObj)
		}

		// Add font objects
		objects = append(objects, fontObjs...)
	}

	// Create Pages root object
	pagesRootObj := w.createPagesRoot(pagesRootRef, pageRefs, doc.PageCount())
	objects = append([]*IndirectObject{pagesRootObj}, objects...)

	return objects, pagesRootRef, nil
}

// createPageTree creates the Pages tree for the document.
//
// PDF uses a tree structure for pages to optimize navigation in large documents.
// For simplicity, this implementation creates a flat tree (one Pages node with all pages).
//
// Structure:
//
//	Pages (root)
//	  /Kids [Page1, Page2, Page3, ...]
//	  /Count N
//
// Returns:
//   - objects: All page-related objects (Pages root + individual Page objects)
//   - rootRef: Object number of the Pages root
//   - error: Any error that occurred
func (w *PdfWriter) createPageTree(doc *document.Document) ([]*IndirectObject, int, error) {
	// Delegate to createPageTreeWithContent with no content
	return w.createPageTreeWithContent(doc, make(map[int][]TextOp))
}

// createPagesRoot creates the Pages root object.
//
// Format:
//
//	<< /Type /Pages /Kids [N 0 R ...] /Count N >>
func (w *PdfWriter) createPagesRoot(objNum int, pageRefs []int, count int) *IndirectObject {
	var pages bytes.Buffer
	pages.WriteString("<<")
	pages.WriteString(" /Type /Pages")

	// Write Kids array
	pages.WriteString(" /Kids [")
	for i, ref := range pageRefs {
		if i > 0 {
			pages.WriteString(" ")
		}
		pages.WriteString(fmt.Sprintf("%d 0 R", ref))
	}
	pages.WriteString("]")

	// Write Count
	pages.WriteString(fmt.Sprintf(" /Count %d", count))

	pages.WriteString(" >>")

	return NewIndirectObject(objNum, 0, pages.Bytes())
}

// createPage creates an individual Page object.
//
// Format:
//
//	<<
//	  /Type /Page
//	  /Parent N 0 R
//	  /MediaBox [0 0 width height]
//	  /Resources << /Font << /F1 5 0 R >> >>
//	  /Contents N 0 R
//	>>
//
// Parameters:
//   - page: Domain Page entity
//   - objNum: Object number for this page
//   - parentRef: Object number of parent Pages node
//   - pageContent: Content operations for this page (optional)
//
// Returns:
//   - pageObj: The page dictionary object
//   - contentObj: The content stream object (nil if no content)
//   - fontObjs: Font dictionary objects
func (w *PdfWriter) createPageWithContent(
	page *document.Page,
	objNum int,
	parentRef int,
	textOps []TextOp,
) (pageObj *IndirectObject, contentObj *IndirectObject, fontObjs []*IndirectObject) {
	var pageDict bytes.Buffer
	pageDict.WriteString("<<")
	pageDict.WriteString(" /Type /Page")
	pageDict.WriteString(fmt.Sprintf(" /Parent %d 0 R", parentRef))

	// MediaBox
	mediaBox := page.MediaBox()
	llx, lly := mediaBox.LowerLeft()
	urx, ury := mediaBox.UpperRight()
	pageDict.WriteString(fmt.Sprintf(" /MediaBox [%.2f %.2f %.2f %.2f]", llx, lly, urx, ury))

	// CropBox (if set)
	if cropBox := page.CropBox(); cropBox != nil {
		llx, lly := cropBox.LowerLeft()
		urx, ury := cropBox.UpperRight()
		pageDict.WriteString(fmt.Sprintf(" /CropBox [%.2f %.2f %.2f %.2f]", llx, lly, urx, ury))
	}

	// Rotation (if not 0)
	if page.Rotation() != 0 {
		pageDict.WriteString(fmt.Sprintf(" /Rotate %d", page.Rotation()))
	}

	// Generate content stream and resources
	if len(textOps) > 0 {
		// Generate content stream
		content, resources, err := GenerateContentStream(textOps)
		if err != nil {
			// For now, skip content on error
			// TODO: Better error handling
			pageDict.WriteString(" /Resources << >>")
			pageDict.WriteString(" >>")
			return NewIndirectObject(objNum, 0, pageDict.Bytes()), nil, nil
		}

		// Create font objects and assign object numbers
		fontMap, err := CreateFontObjects(textOps)
		if err != nil {
			pageDict.WriteString(" /Resources << >>")
			pageDict.WriteString(" >>")
			return NewIndirectObject(objNum, 0, pageDict.Bytes()), nil, nil
		}

		fontObjs = make([]*IndirectObject, 0)
		for fontName, fontDef := range fontMap {
			fontObjNum := w.allocateObjNum()

			// Create font object using WriteFontObject
			var fontBuf bytes.Buffer
			if err := fontDef.WriteFontObject(fontObjNum, &fontBuf); err != nil {
				continue
			}

			// Extract just the dictionary part (without N 0 obj and endobj)
			fontBytes := fontBuf.Bytes()
			// Find the start of the dictionary (after "N 0 obj\n")
			dictStart := bytes.Index(fontBytes, []byte("<<"))
			dictEnd := bytes.LastIndex(fontBytes, []byte(">>")) + 2

			if dictStart >= 0 && dictEnd > dictStart {
				fontDict := fontBytes[dictStart:dictEnd]
				fontObjs = append(fontObjs, NewIndirectObject(fontObjNum, 0, fontDict))

				// Update resource dictionary using font ID.
				fontKey := "std:" + fontName
				resources.SetFontObjNumByID(fontKey, fontObjNum)
			}
		}

		// Write resources dictionary
		pageDict.WriteString(" /Resources ")
		pageDict.Write(resources.Bytes())

		// Create content stream object with compression enabled
		contentObjNum := w.allocateObjNum()
		contentObj = CreateContentStreamObject(contentObjNum, content, true)

		// Reference content stream
		pageDict.WriteString(fmt.Sprintf(" /Contents %d 0 R", contentObjNum))
	} else {
		// No content - empty resources
		pageDict.WriteString(" /Resources << >>")
	}

	pageDict.WriteString(" >>")

	return NewIndirectObject(objNum, 0, pageDict.Bytes()), contentObj, fontObjs
}

// createPageWithAllContent creates a Page object with both text and graphics content.
//
// Similar to createPageWithContent but accepts both text and graphics operations.
//
// Returns:
//   - pageObj: The Page dictionary object
//   - contentObj: The content stream object (nil if no content)
//   - fontObjs: Font dictionary objects
func (w *PdfWriter) createPageWithAllContent(
	page *document.Page,
	objNum int,
	parentRef int,
	textOps []TextOp,
	graphicsOps []GraphicsOp,
) (pageObj *IndirectObject, contentObj *IndirectObject, fontObjs []*IndirectObject) {
	var pageDict bytes.Buffer
	pageDict.WriteString("<<")
	pageDict.WriteString(" /Type /Page")
	pageDict.WriteString(fmt.Sprintf(" /Parent %d 0 R", parentRef))

	// MediaBox
	mediaBox := page.MediaBox()
	llx, lly := mediaBox.LowerLeft()
	urx, ury := mediaBox.UpperRight()
	pageDict.WriteString(fmt.Sprintf(" /MediaBox [%.2f %.2f %.2f %.2f]", llx, lly, urx, ury))

	// CropBox (if set)
	if cropBox := page.CropBox(); cropBox != nil {
		llx, lly := cropBox.LowerLeft()
		urx, ury := cropBox.UpperRight()
		pageDict.WriteString(fmt.Sprintf(" /CropBox [%.2f %.2f %.2f %.2f]", llx, lly, urx, ury))
	}

	// Rotation (if not 0)
	if page.Rotation() != 0 {
		pageDict.WriteString(fmt.Sprintf(" /Rotate %d", page.Rotation()))
	}

	// Generate content stream with graphics and text
	if len(textOps) > 0 || len(graphicsOps) > 0 {
		fontObjs = make([]*IndirectObject, 0)
		hasTextContent := len(textOps) > 0 || hasTextBlockOps(graphicsOps)

		// STEP 1: Collect fonts and BUILD SUBSETS FIRST.
		// This is critical: content stream encoding needs GlyphMapping from built subsets.
		var fontCollection *FontCollection
		if hasTextContent {
			var err error
			fontCollection, err = CreateFontCollectionWithGraphics(textOps, graphicsOps)
			if err != nil {
				pageDict.WriteString(" /Resources << >>")
				pageDict.WriteString(" >>")
				return NewIndirectObject(objNum, 0, pageDict.Bytes()), nil, nil
			}

			// Build all embedded font subsets BEFORE generating content stream.
			for _, embFont := range fontCollection.Embedded {
				if embFont.Subset != nil {
					_ = embFont.Subset.Build() // Ignore errors for now, will handle below.
				}
			}
		}

		// STEP 2: Generate content stream (now subsets are built, GlyphMapping available).
		content, resources, err := GenerateContentStreamWithGraphics(textOps, graphicsOps)
		if err != nil {
			pageDict.WriteString(" /Resources << >>")
			pageDict.WriteString(" >>")
			return NewIndirectObject(objNum, 0, pageDict.Bytes()), nil, nil
		}

		// STEP 3: Create font objects and assign object numbers.
		if fontCollection != nil {
			// Process Standard14 fonts.
			for fontName, fontDef := range fontCollection.Standard14 {
				fontObjNum := w.allocateObjNum()

				var fontBuf bytes.Buffer
				if err := fontDef.WriteFontObject(fontObjNum, &fontBuf); err != nil {
					continue
				}

				fontBytes := fontBuf.Bytes()
				dictStart := bytes.Index(fontBytes, []byte("<<"))
				dictEnd := bytes.LastIndex(fontBytes, []byte(">>")) + 2

				if dictStart >= 0 && dictEnd > dictStart {
					fontDict := fontBytes[dictStart:dictEnd]
					fontObjs = append(fontObjs, NewIndirectObject(fontObjNum, 0, fontDict))

					fontKey := "std:" + fontName
					resources.SetFontObjNumByID(fontKey, fontObjNum)
				}
			}

			// Process embedded TrueType fonts (subsets already built in STEP 1).
			for fontID, embFont := range fontCollection.Embedded {
				fontWriter := NewTrueTypeFontWriter(embFont.TTF, embFont.Subset, w.allocateObjNum)
				fontObjects, refs, err := fontWriter.WriteFont()
				if err != nil {
					continue
				}

				fontObjs = append(fontObjs, fontObjects...)

				fontKey := "custom:" + fontID
				resources.SetFontObjNumByID(fontKey, refs.FontObjNum)
			}
		}

		// STEP 3.5: Create image XObjects for image operations and assign object numbers.
		imageObjs, err := w.createAndAssignImageXObjects(graphicsOps, resources)
		if err != nil {
			// Log error but continue - don't fail the whole page
			// TODO: Add logging when available
			_ = err
		} else {
			fontObjs = append(fontObjs, imageObjs...)
		}

		// Write resources dictionary
		pageDict.WriteString(" /Resources ")
		pageDict.Write(resources.Bytes())

		// Create content stream object with compression enabled
		contentObjNum := w.allocateObjNum()
		contentObj = CreateContentStreamObject(contentObjNum, content, true)

		// Reference content stream
		pageDict.WriteString(fmt.Sprintf(" /Contents %d 0 R", contentObjNum))
	} else {
		// No content - empty resources
		pageDict.WriteString(" /Resources << >>")
	}

	// Add annotations if present (all types).
	if page.AnnotationCount() > 0 {
		// Create annotation objects for all annotation types.
		annotObjs, annotRefs, err := w.WriteAllAnnotations(page)
		if err == nil && len(annotRefs) > 0 {
			// Write /Annots array.
			pageDict.WriteString(" /Annots [")
			for i, ref := range annotRefs {
				if i > 0 {
					pageDict.WriteString(" ")
				}
				pageDict.WriteString(fmt.Sprintf("%d 0 R", ref))
			}
			pageDict.WriteString("]")

			// Add annotation objects to font objects list (reuse parameter).
			fontObjs = append(fontObjs, annotObjs...)
		}
	}

	pageDict.WriteString(" >>")

	return NewIndirectObject(objNum, 0, pageDict.Bytes()), contentObj, fontObjs
}

// createPage creates an individual Page object (backward compatibility).
//
// This is kept for existing code that doesn't have content operations.
func (w *PdfWriter) createPage(page *document.Page, objNum int, parentRef int) *IndirectObject {
	pageObj, _, _ := w.createPageWithContent(page, objNum, parentRef, nil)
	return pageObj
}

// createAndAssignImageXObjects creates image XObject dictionary objects for all image operations
// and assigns their object numbers to the resource dictionary.
//
// This function:
// 1. Collects all image operations from graphicsOps
// 2. For each image, allocates an object number and creates the XObject
// 3. Creates an SMask (soft mask) for images with alpha transparency
// 4. Assigns the object numbers to the resource dictionary entries created during content stream generation
//
// Note: The resource dictionary already has placeholder image entries (Im1, Im2, etc.)
// created during content stream generation. This function assigns real object numbers to them.
//
// Returns:
//   - objects: Image XObject dictionary objects (and SMask objects)
//   - error: Any error that occurred
func (w *PdfWriter) createAndAssignImageXObjects(graphicsOps []GraphicsOp, resources *ResourceDictionary) ([]*IndirectObject, error) {
	objects := make([]*IndirectObject, 0)

	// Collect all images from graphics operations
	images := make([]*ImageData, 0)
	for _, gop := range graphicsOps {
		if gop.Type == 3 && gop.Image != nil {
			images = append(images, gop.Image)
		}
	}

	// Create XObject for each image
	for i, img := range images {
		// Allocate object number for the image XObject
		imageObjNum := w.allocateObjNum()

		// Handle alpha mask (SMask) for PNG with transparency
		var smaskObjNum int
		if len(img.AlphaMask) > 0 {
			smaskObjNum = w.allocateObjNum()
			smaskObj := w.createSMaskObject(smaskObjNum, img)
			objects = append(objects, smaskObj)
		}

		// Create the image XObject
		imageObj := w.createImageXObject(imageObjNum, img, smaskObjNum)
		objects = append(objects, imageObj)

		// Set the object number in the resource dictionary
		// The resource names (Im1, Im2, ...) were created during content stream generation
		// We need to update them with the actual object numbers
		imageResName := fmt.Sprintf("Im%d", i+1)
		w.setImageResourceObjNum(resources, imageResName, imageObjNum)
	}

	return objects, nil
}

// setImageResourceObjNum sets the object number for an image resource.
//
// This is a helper function to update the resource dictionary after image XObjects are created.
func (w *PdfWriter) setImageResourceObjNum(resources *ResourceDictionary, name string, objNum int) {
	resources.SetImageObjNum(name, objNum)
}

// createImageXObject creates a PDF Image XObject dictionary.
//
// Format (JPEG):
//
//	N 0 obj
//	<< /Type /XObject /Subtype /Image /Width W /Height H
//	   /ColorSpace /DeviceRGB /BitsPerComponent 8
//	   /Filter /DCTDecode /Length L >>
//	stream
//	... JPEG data ...
//	endstream
//	endobj
//
// Format (PNG with alpha):
//
//	N 0 obj
//	<< /Type /XObject /Subtype /Image /Width W /Height H
//	   /ColorSpace /DeviceRGB /BitsPerComponent 8
//	   /Filter /FlateDecode /SMask M 0 R /Length L >>
//	stream
//	... compressed pixel data ...
//	endstream
//	endobj
func (w *PdfWriter) createImageXObject(objNum int, img *ImageData, smaskObjNum int) *IndirectObject {
	var buf bytes.Buffer

	// Write stream dictionary
	buf.WriteString("<< /Type /XObject /Subtype /Image")
	buf.WriteString(fmt.Sprintf(" /Width %d /Height %d", img.Width, img.Height))
	buf.WriteString(fmt.Sprintf(" /ColorSpace /%s", img.ColorSpace))
	buf.WriteString(fmt.Sprintf(" /BitsPerComponent %d", img.BitsPerComponent))

	// Add filter based on format
	if img.Format == "jpeg" {
		buf.WriteString(" /Filter /DCTDecode")
	} else if img.Format == "png" {
		buf.WriteString(" /Filter /FlateDecode")
	}

	// Add SMask reference if alpha mask exists
	if smaskObjNum > 0 {
		buf.WriteString(fmt.Sprintf(" /SMask %d 0 R", smaskObjNum))
	}

	// Write length
	buf.WriteString(fmt.Sprintf(" /Length %d >>\n", len(img.Data)))

	// Write stream
	buf.WriteString("stream\n")
	buf.Write(img.Data)
	buf.WriteString("\nendstream")

	return NewIndirectObject(objNum, 0, buf.Bytes())
}

// createSMaskObject creates a PDF SMask (soft mask) object for image transparency.
//
// Format:
//
//	N 0 obj
//	<< /Type /XObject /Subtype /Image /Width W /Height H
//	   /ColorSpace /DeviceGray /BitsPerComponent 8
//	   /Filter /FlateDecode /Length L >>
//	stream
//	... compressed alpha data ...
//	endstream
//	endobj
func (w *PdfWriter) createSMaskObject(objNum int, img *ImageData) *IndirectObject {
	var buf bytes.Buffer

	// Write stream dictionary
	buf.WriteString("<< /Type /XObject /Subtype /Image")
	buf.WriteString(fmt.Sprintf(" /Width %d /Height %d", img.Width, img.Height))
	buf.WriteString(" /ColorSpace /DeviceGray")
	buf.WriteString(" /BitsPerComponent 8")
	buf.WriteString(" /Filter /FlateDecode")
	buf.WriteString(fmt.Sprintf(" /Length %d >>\n", len(img.AlphaMask)))

	// Write stream
	buf.WriteString("stream\n")
	buf.Write(img.AlphaMask)
	buf.WriteString("\nendstream")

	return NewIndirectObject(objNum, 0, buf.Bytes())
}
