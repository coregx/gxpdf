package writer

import (
	"bytes"
	"fmt"

	"github.com/coregx/gxpdf/internal/document"
	"github.com/coregx/gxpdf/logging"
)

// hasTextGraphicsOps checks if any graphics operations draw text: TextBlock
// (type 22) or Watermark (type 4). Their fonts need font objects.
func hasTextGraphicsOps(graphicsOps []GraphicsOp) bool {
	for _, gop := range graphicsOps {
		if gop.Type == 22 || gop.Type == 4 { // TextBlock, Watermark
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

		// Create ExtGState objects for opacity and assign object numbers.
		gsObjs := w.createExtGStateObjects(resources)
		fontObjs = append(fontObjs, gsObjs...)

		// Create Shading objects for gradient fills.
		shadingObjs := w.createShadingObjects(resources)
		fontObjs = append(fontObjs, shadingObjs...)

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
		hasTextContent := len(textOps) > 0 || hasTextGraphicsOps(graphicsOps)

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
			logging.Logger().Warn("failed to create image XObjects", "error", err)
		} else {
			fontObjs = append(fontObjs, imageObjs...)
		}

		// STEP 3.6: Create ExtGState objects for opacity and assign object numbers.
		gsObjs := w.createExtGStateObjects(resources)
		fontObjs = append(fontObjs, gsObjs...)

		// STEP 3.7: Create Shading objects for gradient fills.
		shadingObjs := w.createShadingObjects(resources)
		fontObjs = append(fontObjs, shadingObjs...)

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

// createExtGStateObjects creates ExtGState PDF indirect objects for all registered graphics states
// in the resource dictionary and assigns their object numbers.
//
// This fills the gap where ExtGState entries were registered during content stream generation
// (with placeholder object number 0) but never materialized as actual PDF objects.
//
// Each ExtGState dictionary has the format:
//
//	<< /Type /ExtGState /ca {opacity} /CA {opacity} >>
//
// where /ca controls fill opacity and /CA controls stroke opacity.
//
// Returns the created IndirectObject slice.
func (w *PdfWriter) createExtGStateObjects(resources *ResourceDictionary) []*IndirectObject {
	entries := resources.ExtGStateEntries()
	if len(entries) == 0 {
		return nil
	}

	objects := make([]*IndirectObject, 0, len(entries))

	for gsName, opacity := range entries {
		objNum := w.allocateObjNum()

		var buf bytes.Buffer
		buf.WriteString("<< /Type /ExtGState")
		buf.WriteString(fmt.Sprintf(" /ca %.2f /CA %.2f", opacity, opacity))
		buf.WriteString(" >>")

		objects = append(objects, NewIndirectObject(objNum, 0, buf.Bytes()))
		resources.SetExtGStateObjNum(gsName, objNum)

		logging.Logger().Debug("created ExtGState object",
			"gsName", gsName,
			"objNum", objNum,
			"opacity", opacity,
		)
	}

	return objects
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

// createShadingObjects creates Shading PDF objects for all registered gradient resources
// in the resource dictionary and assigns their object numbers.
//
// For each gradient, this creates:
//  1. One or more Function objects (Type 2 exponential interpolation, optionally stitched with Type 3)
//  2. A Shading dictionary referencing the function
//
// The Shading dictionary object number is assigned back to the resource dictionary.
//
// Returns the created IndirectObject slice (functions + shading dicts).
func (w *PdfWriter) createShadingObjects(resources *ResourceDictionary) []*IndirectObject {
	entries := resources.ShadingEntries()
	if len(entries) == 0 {
		return nil
	}

	objects := make([]*IndirectObject, 0, len(entries)*2)

	for shName, entry := range entries {
		grad := entry.Gradient
		if grad == nil || len(grad.ColorStops) < 2 {
			continue
		}

		// Create function object(s) for color interpolation.
		funcObjs, topFuncObjNum := w.createGradientFunction(grad)
		objects = append(objects, funcObjs...)

		// Create shading dictionary referencing the function.
		shadingObjNum := w.allocateObjNum()
		shadingObj := w.createShadingDict(shadingObjNum, grad, topFuncObjNum)
		objects = append(objects, shadingObj)

		resources.SetShadingObjNum(shName, shadingObjNum)

		logging.Logger().Debug("created Shading object",
			"shName", shName,
			"objNum", shadingObjNum,
			"type", grad.Type,
			"stops", len(grad.ColorStops),
		)
	}

	return objects
}

// createGradientFunction creates PDF Function objects for a gradient's color interpolation.
//
// For 2 color stops: a single Type 2 (exponential interpolation) function.
// For 3+ stops: N-1 Type 2 functions stitched together with a Type 3 function.
//
// Returns the function objects and the object number of the top-level function.
func (w *PdfWriter) createGradientFunction(grad *GradientOp) ([]*IndirectObject, int) {
	stops := grad.ColorStops

	if len(stops) == 2 {
		// Simple case: single Type 2 function.
		objNum := w.allocateObjNum()
		c0 := stops[0].Color
		c1 := stops[1].Color

		var buf bytes.Buffer
		buf.WriteString("<< /FunctionType 2 /Domain [0 1]")
		buf.WriteString(fmt.Sprintf(" /C0 [%.4f %.4f %.4f]", c0.R, c0.G, c0.B))
		buf.WriteString(fmt.Sprintf(" /C1 [%.4f %.4f %.4f]", c1.R, c1.G, c1.B))
		buf.WriteString(" /N 1 >>")

		return []*IndirectObject{NewIndirectObject(objNum, 0, buf.Bytes())}, objNum
	}

	// Multi-stop: create N-1 Type 2 functions, then a Type 3 stitching function.
	segFuncs := make([]*IndirectObject, 0, len(stops)-1)
	segRefs := make([]int, 0, len(stops)-1)

	for i := 0; i < len(stops)-1; i++ {
		objNum := w.allocateObjNum()
		c0 := stops[i].Color
		c1 := stops[i+1].Color

		var buf bytes.Buffer
		buf.WriteString("<< /FunctionType 2 /Domain [0 1]")
		buf.WriteString(fmt.Sprintf(" /C0 [%.4f %.4f %.4f]", c0.R, c0.G, c0.B))
		buf.WriteString(fmt.Sprintf(" /C1 [%.4f %.4f %.4f]", c1.R, c1.G, c1.B))
		buf.WriteString(" /N 1 >>")

		segFuncs = append(segFuncs, NewIndirectObject(objNum, 0, buf.Bytes()))
		segRefs = append(segRefs, objNum)
	}

	// Type 3 stitching function.
	stitchObjNum := w.allocateObjNum()
	var stitchBuf bytes.Buffer
	stitchBuf.WriteString("<< /FunctionType 3 /Domain [0 1]")

	// /Functions array
	stitchBuf.WriteString(" /Functions [")
	for i, ref := range segRefs {
		if i > 0 {
			stitchBuf.WriteString(" ")
		}
		stitchBuf.WriteString(fmt.Sprintf("%d 0 R", ref))
	}
	stitchBuf.WriteString("]")

	// /Bounds array (inner boundaries between segments)
	stitchBuf.WriteString(" /Bounds [")
	for i := 1; i < len(stops)-1; i++ {
		if i > 1 {
			stitchBuf.WriteString(" ")
		}
		stitchBuf.WriteString(fmt.Sprintf("%.4f", stops[i].Position))
	}
	stitchBuf.WriteString("]")

	// /Encode array (each segment maps [0 1])
	stitchBuf.WriteString(" /Encode [")
	for i := 0; i < len(stops)-1; i++ {
		if i > 0 {
			stitchBuf.WriteString(" ")
		}
		stitchBuf.WriteString("0 1")
	}
	stitchBuf.WriteString("]")

	stitchBuf.WriteString(" >>")

	allObjs := append(segFuncs, NewIndirectObject(stitchObjNum, 0, stitchBuf.Bytes()))
	return allObjs, stitchObjNum
}

// createShadingDict creates a PDF Shading dictionary object.
//
// Linear (ShadingType 2):
//
//	<< /ShadingType 2 /ColorSpace /DeviceRGB /Coords [x1 y1 x2 y2]
//	   /Function N 0 R /Extend [true true] >>
//
// Radial (ShadingType 3):
//
//	<< /ShadingType 3 /ColorSpace /DeviceRGB /Coords [x0 y0 r0 x1 y1 r1]
//	   /Function N 0 R /Extend [true true] >>
func (w *PdfWriter) createShadingDict(objNum int, grad *GradientOp, funcObjNum int) *IndirectObject {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("<< /ShadingType %d", int(grad.Type)))
	buf.WriteString(" /ColorSpace /DeviceRGB")

	switch grad.Type {
	case GradientTypeLinear:
		buf.WriteString(fmt.Sprintf(" /Coords [%.2f %.2f %.2f %.2f]",
			grad.X1, grad.Y1, grad.X2, grad.Y2))
	case GradientTypeRadial:
		buf.WriteString(fmt.Sprintf(" /Coords [%.2f %.2f %.2f %.2f %.2f %.2f]",
			grad.X0, grad.Y0, grad.R0, grad.X1, grad.Y1, grad.R1))
	}

	buf.WriteString(fmt.Sprintf(" /Function %d 0 R", funcObjNum))

	// Extend flags
	extStart := "false"
	extEnd := "false"
	if grad.ExtendStart {
		extStart = "true"
	}
	if grad.ExtendEnd {
		extEnd = "true"
	}
	buf.WriteString(fmt.Sprintf(" /Extend [%s %s]", extStart, extEnd))

	buf.WriteString(" >>")

	return NewIndirectObject(objNum, 0, buf.Bytes())
}
