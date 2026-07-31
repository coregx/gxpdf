package gxpdf

import (
	"fmt"

	"github.com/coregx/gxpdf/internal/extractor"
	"github.com/coregx/gxpdf/internal/tabledetect"
	"github.com/coregx/gxpdf/logging"
)

// Page represents a single page in a PDF document.
type Page struct {
	doc   *Document
	index int
}

// Index returns the page index (0-based).
func (p *Page) Index() int {
	return p.index
}

// Number returns the page number (1-based, for display).
func (p *Page) Number() int {
	return p.index + 1
}

// ExtractText extracts all text from the page.
//
// Returns the text content as a single string.
// Errors are logged via slog. For error handling, use Document.ExtractTextFromPage.
//
// Example:
//
//	text := page.ExtractText()
//	fmt.Println(text)
func (p *Page) ExtractText() string {
	textExtractor := extractor.NewTextExtractor(p.doc.reader)
	elements, err := textExtractor.ExtractFromPage(p.index)
	if err != nil {
		logging.Logger().Error("failed to extract text from page",
			"page", p.index,
			"error", err)
		return ""
	}

	return extractor.AssembleText(elements)
}

// ExtractTextElements extracts positioned text elements from the page.
//
// Unlike ExtractText, which returns a plain string, ExtractTextElements
// returns each text run with its position, size, and font information.
// This is useful for layout analysis, text highlighting, and advanced
// document intelligence tasks.
//
// Coordinates follow the PDF convention: origin (0,0) is at the bottom-left
// of the page, X increases to the right, Y increases upward, in points
// (1 pt = 1/72 inch).
//
// Example:
//
//	elements, err := page.ExtractTextElements()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, e := range elements {
//	    fmt.Printf("%q at (%.1f, %.1f) size=%.1f\n", e.Text, e.X, e.Y, e.FontSize)
//	}
func (p *Page) ExtractTextElements() ([]TextElement, error) {
	textExtractor := extractor.NewTextExtractor(p.doc.reader)
	internal, err := textExtractor.ExtractFromPage(p.index)
	if err != nil {
		return nil, fmt.Errorf("gxpdf: failed to extract text elements from page %d: %w", p.index, err)
	}

	elements := make([]TextElement, len(internal))
	for i, e := range internal {
		elements[i] = TextElement{
			Text:     e.Text,
			X:        e.X,
			Y:        e.Y,
			Width:    e.Width,
			Height:   e.Height,
			FontName: e.FontName,
			FontSize: e.FontSize,
		}
	}
	return elements, nil
}

// ExtractTables extracts all tables from this page.
//
// Errors are logged via slog. For error handling, use ExtractTablesWithOptions.
//
// Example:
//
//	tables := page.ExtractTables()
//	for _, t := range tables {
//	    fmt.Println(t.Rows())
//	}
func (p *Page) ExtractTables() []*Table {
	tables, err := p.ExtractTablesWithOptions(nil)
	if err != nil {
		logging.Logger().Error("failed to extract tables from page",
			"page", p.index,
			"error", err)
	}
	return tables
}

// ExtractTablesWithOptions extracts tables with custom options.
func (p *Page) ExtractTablesWithOptions(opts *ExtractionOptions) ([]*Table, error) {
	if opts == nil {
		opts = DefaultExtractionOptions()
	}

	textExtractor := extractor.NewTextExtractor(p.doc.reader)
	textElements, err := textExtractor.ExtractFromPage(p.index)
	if err != nil {
		return nil, err
	}

	tableDetector := tabledetect.NewDefaultTableDetector()

	var detectedTables []*tabledetect.TableRegion

	switch opts.Method {
	case MethodLattice:
		graphicsParser := extractor.NewGraphicsParser(p.doc.reader)
		graphicsElements, gErr := graphicsParser.ParseFromPage(p.index)
		if gErr != nil {
			return nil, fmt.Errorf("gxpdf: failed to extract graphics from page %d: %w", p.index, gErr)
		}
		detectedTables, err = tableDetector.DetectTablesLattice(textElements, graphicsElements)
	case MethodStream:
		detectedTables, err = tableDetector.DetectTablesStream(textElements)
	default:
		graphicsParser := extractor.NewGraphicsParser(p.doc.reader)
		graphicsElements, gErr := graphicsParser.ParseFromPage(p.index)
		if gErr != nil {
			return nil, fmt.Errorf("gxpdf: failed to extract graphics from page %d: %w", p.index, gErr)
		}
		detectedTables, err = tableDetector.DetectTables(textElements, graphicsElements)
	}

	if err != nil {
		return nil, err
	}

	var tables []*Table

	// For Lattice tables, text and graphics may be in different coordinate
	// spaces (text: raw Tm/Td, graphics: CTM-normalised). Detect mismatch
	// and transform text coordinates to match the grid.
	normalizedText := textElements
	if opts.Method != MethodStream && len(detectedTables) > 0 {
		pageHeight := extractor.ReadPageHeight(p.doc.reader, p.index)
		if pageHeight > 0 {
			for _, region := range detectedTables {
				if region.Grid != nil {
					bounds := region.Grid.Bounds()
					cn := extractor.NewCoordinateNormalizer(pageHeight)
					normalizedText = cn.NormalizeTextToGridSpace(
						normalizedText, bounds.Y, bounds.Y+bounds.Height,
					)
					break
				}
			}
		}
	}

	tableExtractor := tabledetect.NewTableExtractor(normalizedText)

	for _, region := range detectedTables {
		extracted, err := tableExtractor.ExtractTable(region)
		if err != nil {
			continue
		}
		extracted.PageNum = p.index
		tables = append(tables, &Table{internal: extracted})
	}

	return tables, nil
}

// GetImages extracts all images from this page.
//
// Returns all images found on the page as a slice.
// Errors are logged via slog. For error handling, use GetImagesWithError.
//
// Example:
//
//	images := page.GetImages()
//	for i, img := range images {
//	    fmt.Printf("Image %d: %dx%d\n", i, img.Width(), img.Height())
//	    img.SaveToFile(fmt.Sprintf("page%d_image%d.jpg", page.Number(), i))
//	}
func (p *Page) GetImages() []*Image {
	images, err := p.GetImagesWithError()
	if err != nil {
		logging.Logger().Error("failed to extract images from page",
			"page", p.index,
			"error", err)
	}
	return images
}

// GetImagesWithError extracts all images from this page, returning any errors.
//
// Use this when you need error handling for image extraction.
func (p *Page) GetImagesWithError() ([]*Image, error) {
	imageExtractor := extractor.NewImageExtractor(p.doc.reader)
	internalImages, err := imageExtractor.ExtractFromPage(p.index)
	if err != nil {
		return nil, err
	}

	// Wrap internal images in public API
	images := make([]*Image, len(internalImages))
	for i, internal := range internalImages {
		images[i] = &Image{internal: internal}
	}

	return images, nil
}
