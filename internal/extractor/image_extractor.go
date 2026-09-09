// Package extractor provides use cases for extracting content from PDF documents.
package extractor

import (
	"context"
	"fmt"

	"github.com/coregx/gxpdf/internal/models/types"
	"github.com/coregx/gxpdf/internal/parser"
)

// colorSpaceDeviceRGB is the default PDF color space for RGB images.
const colorSpaceDeviceRGB = "DeviceRGB"

// ImageExtractor extracts images from PDF pages.
//
// This is an application service that coordinates image extraction
// from PDF documents using the domain model and infrastructure services.
//
// Example:
//
//	reader, _ := parser.OpenPDF("document.pdf")
//	defer reader.Close()
//
//	extractor := NewImageExtractor(reader)
//	images, _ := extractor.ExtractFromPage(0)
//	for _, img := range images {
//	    img.SaveToFile(fmt.Sprintf("image_%d.jpg", i))
//	}
type ImageExtractor struct {
	reader *parser.Reader
	ctx    context.Context
}

// NewImageExtractor creates a new image extractor.
//
// Parameters:
//   - reader: PDF reader providing access to document structure
//
// Returns a configured ImageExtractor ready to extract images.
func NewImageExtractor(reader *parser.Reader) *ImageExtractor {
	return NewImageExtractorWithContext(reader, context.Background())
}

// NewImageExtractorWithContext creates an ImageExtractor whose stream
// decoding observes ctx.
func NewImageExtractorWithContext(reader *parser.Reader, ctx context.Context) *ImageExtractor {
	return &ImageExtractor{
		reader: reader,
		ctx:    contextOrBackground(ctx),
	}
}

// ExtractFromDocument extracts all images from all pages in the document.
//
// This iterates through all pages and extracts images from each.
//
// Returns a slice of all images found in the document, or error if extraction fails.
func (e *ImageExtractor) ExtractFromDocument() ([]*types.Image, error) {
	if err := contextOrBackground(e.ctx).Err(); err != nil {
		return nil, err
	}
	pageCount, err := e.reader.GetPageCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get page count: %w", err)
	}

	var allImages []*types.Image

	for i := 0; i < pageCount; i++ {
		images, err := e.ExtractFromPage(i)
		if err != nil {
			if contextErr := contextOrBackground(e.ctx).Err(); contextErr != nil {
				return nil, contextErr
			}
			// Log error but continue with other pages
			continue
		}
		allImages = append(allImages, images...)
	}

	return allImages, nil
}

// ExtractFromPage extracts all images from a specific page.
//
// This finds all image XObjects in the page's resources and extracts them.
//
// Parameters:
//   - pageIndex: 0-based page index
//
// Returns a slice of images found on the page, or error if extraction fails.
func (e *ImageExtractor) ExtractFromPage(pageIndex int) ([]*types.Image, error) {
	if err := contextOrBackground(e.ctx).Err(); err != nil {
		return nil, err
	}
	// Get page dictionary
	pageDict, err := e.reader.GetPageWithContext(e.ctx, pageIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to get page %d: %w", pageIndex, err)
	}

	// Get page resources
	resourcesObj := pageDict.Get("Resources")
	if resourcesObj == nil {
		// No resources means no images
		return []*types.Image{}, nil
	}

	resourcesDict, ok := resourcesObj.(*parser.Dictionary)
	if !ok {
		// Try to resolve indirect reference
		if ref, ok := resourcesObj.(*parser.IndirectReference); ok {
			resolvedObj, err := e.reader.GetObjectWithContext(e.ctx, ref.Number)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve resources reference: %w", err)
			}
			resourcesDict, ok = resolvedObj.(*parser.Dictionary)
			if !ok {
				return nil, fmt.Errorf("resources is not a dictionary: %T", resolvedObj)
			}
		} else {
			return nil, fmt.Errorf("resources is not a dictionary: %T", resourcesObj)
		}
	}

	// Get XObject dictionary from resources
	xobjectObj := resourcesDict.Get("XObject")
	if xobjectObj == nil {
		// No XObjects means no images
		return []*types.Image{}, nil
	}

	xobjectDict, ok := xobjectObj.(*parser.Dictionary)
	if !ok {
		// Try to resolve indirect reference
		if ref, ok := xobjectObj.(*parser.IndirectReference); ok {
			resolvedObj, err := e.reader.GetObjectWithContext(e.ctx, ref.Number)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve XObject reference: %w", err)
			}
			xobjectDict, ok = resolvedObj.(*parser.Dictionary)
			if !ok {
				return nil, fmt.Errorf("XObject is not a dictionary: %T", resolvedObj)
			}
		} else {
			return nil, fmt.Errorf("XObject is not a dictionary: %T", xobjectObj)
		}
	}

	// Extract images from XObject dictionary
	var images []*types.Image

	// Iterate over all entries in the XObject dictionary
	keys := xobjectDict.Keys()

	for _, name := range keys {
		if err := e.ctx.Err(); err != nil {
			return nil, err
		}
		xobjRef := xobjectDict.Get(name)
		if xobjRef == nil {
			continue
		}

		// Resolve XObject reference
		var xobj parser.PdfObject
		if ref, ok := xobjRef.(*parser.IndirectReference); ok {
			resolvedObj, err := e.reader.GetObjectWithContext(e.ctx, ref.Number)
			if err != nil {
				if contextErr := e.ctx.Err(); contextErr != nil {
					return nil, contextErr
				}
				continue // Skip this XObject on error
			}
			xobj = resolvedObj
		} else {
			xobj = xobjRef
		}

		// Check if this XObject is an image
		stream, ok := xobj.(*parser.Stream)
		if !ok {
			continue // Not a stream, skip
		}

		subtypeObj := stream.Dictionary().Get("Subtype")
		if subtypeObj == nil {
			continue
		}

		subtype, ok := subtypeObj.(*parser.Name)
		if !ok || subtype.Value() != "Image" {
			continue // Not an image, skip
		}

		// Extract image from stream
		img, err := e.extractImageFromStream(stream, name)
		if err != nil {
			if contextErr := contextOrBackground(e.ctx).Err(); contextErr != nil {
				return nil, contextErr
			}
			// Log error but continue with other images
			continue
		}

		images = append(images, img)
	}

	return images, nil
}

// extractImageFromStream extracts an image from a PDF stream object.
//
// This handles different image encodings and color spaces.
func (e *ImageExtractor) extractImageFromStream(stream *parser.Stream, name string) (*types.Image, error) {
	// Get image properties
	dict := stream.Dictionary()
	width := int(dict.GetInteger("Width"))
	height := int(dict.GetInteger("Height"))
	bitsPerComponent := int(dict.GetInteger("BitsPerComponent"))

	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid image dimensions: %dx%d", width, height)
	}

	if bitsPerComponent <= 0 {
		bitsPerComponent = 8 // Default to 8 bits
	}

	// Get color space
	colorSpaceObj := dict.Get("ColorSpace")
	colorSpace := e.getColorSpaceName(colorSpaceObj)

	// Decode stream data. JPEG payloads stay encoded for export, while
	// preceding filters and all raw-pixel filters use the parser's
	// canonical bounded pipeline.
	data, filter, err := e.decodeImageData(stream)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image data: %w", err)
	}

	// Create Image value object
	img, err := types.NewImage(data, width, height, colorSpace, bitsPerComponent, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to create image: %w", err)
	}

	// Set name
	img.SetName(name)

	return img, nil
}

// decodeImageData decodes an image through the canonical bounded stream
// pipeline. Terminal DCT data remains encoded so callers can export the
// original JPEG payload without a lossy decode/re-encode cycle. JPXDecode is
// rejected by the canonical pipeline until the Image value object has an
// explicit JPEG 2000 representation; it must never fall through as raw pixels.
func (e *ImageExtractor) decodeImageData(stream *parser.Stream) ([]byte, string, error) {
	data, terminalFilter, err := stream.DecodePreservingTerminalWithContext(
		contextOrBackground(e.ctx),
		parser.DefaultStreamDecodeOptions(),
		"DCTDecode",
	)
	if err != nil {
		return nil, "", err
	}
	if terminalFilter == "" {
		return data, "", nil
	}
	return data, "/" + terminalFilter, nil
}

// getColorSpaceName extracts the color space name from a PDF object.
func (e *ImageExtractor) getColorSpaceName(obj parser.PdfObject) string {
	if obj == nil {
		return colorSpaceDeviceRGB // Default color space
	}

	// Direct name (e.g., /DeviceRGB)
	if name, ok := obj.(*parser.Name); ok {
		return name.Value()
	}

	// Array (e.g., [/Indexed ...])
	if arr, ok := obj.(*parser.Array); ok {
		if arr.Len() > 0 {
			if name, ok := arr.Get(0).(*parser.Name); ok {
				return name.Value()
			}
		}
	}

	return colorSpaceDeviceRGB // Default
}
