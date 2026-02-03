// Package creator provides a high-level API for creating and modifying PDF documents.
package creator

import (
	"context"
	"fmt"

	"github.com/coregx/gxpdf/internal/application/forms"
	"github.com/coregx/gxpdf/internal/document"
	"github.com/coregx/gxpdf/internal/parser"
	"github.com/coregx/gxpdf/internal/reader"
	"github.com/coregx/gxpdf/internal/writer"
)

// Appender provides functionality to modify existing PDF documents.
//
// It allows you to:
//   - Open an existing PDF for modification
//   - Add new pages to the document
//   - Add content to existing pages (watermarks, stamps, annotations)
//   - Merge multiple PDFs into one
//   - Save changes incrementally (append mode) or create a new file
//
// Example - Add watermark to all pages:
//
//	app, err := creator.NewAppender("input.pdf")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer app.Close()
//
//	for i := 0; i < app.PageCount(); i++ {
//	    page, err := app.GetPage(i)
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    page.AddText("CONFIDENTIAL", 300, 400, creator.HelveticaBold, 48)
//	}
//
//	err = app.WriteToFile("output.pdf")
//
// Example - Append new page:
//
//	app, err := creator.NewAppender("input.pdf")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer app.Close()
//
//	newPage, err := app.AddPage(creator.A4)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	newPage.AddText("New content", 100, 700, creator.Helvetica, 12)
//
//	err = app.WriteToFile("output.pdf")
type Appender struct {
	// PDF reader for accessing existing document.
	pdfReader *reader.PdfReader

	// Domain document (reconstructed from existing PDF).
	doc *document.Document

	// Creator pages (wraps domain pages for high-level API).
	pages []*Page

	// Track which pages were modified.
	modifiedPages map[int]bool

	// Track new pages added.
	newPages []*Page

	// Form field writer for setting field values.
	formWriter *forms.Writer

	// Track fields to flatten when writing.
	flattenedFields []*forms.FlattenInfo
}

// NewAppender opens an existing PDF file for modification.
//
// The file is opened and parsed immediately. Remember to call Close()
// when done to release resources.
//
// Returns an error if:
//   - File cannot be opened
//   - File is not a valid PDF
//   - PDF is encrypted (not yet supported)
//
// Example:
//
//	app, err := creator.NewAppender("existing.pdf")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer app.Close()
func NewAppender(path string) (*Appender, error) {
	// Open PDF file for reading.
	pdfReader, err := reader.NewPdfReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF: %w", err)
	}

	// Reconstruct domain document from existing PDF.
	doc, pages, err := reconstructDocument(pdfReader)
	if err != nil {
		_ = pdfReader.Close()
		return nil, fmt.Errorf("failed to reconstruct document: %w", err)
	}

	return &Appender{
		pdfReader:     pdfReader,
		doc:           doc,
		pages:         pages,
		modifiedPages: make(map[int]bool),
		newPages:      make([]*Page, 0),
	}, nil
}

// reconstructDocument rebuilds domain document from existing PDF.
//
// This reads the PDF structure and creates domain entities.
func reconstructDocument(pdfReader *reader.PdfReader) (*document.Document, []*Page, error) {
	// Create new domain document.
	doc := document.NewDocument()

	// Get page count from PDF.
	pageCount := pdfReader.PageCount()
	if pageCount == 0 {
		return nil, nil, fmt.Errorf("PDF has no pages")
	}

	// Reconstruct pages.
	pages := make([]*Page, pageCount)
	for i := 0; i < pageCount; i++ {
		// Get page dictionary from parser.
		pageDict, err := pdfReader.GetPage(i)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get page %d: %w", i, err)
		}

		// Extract page dimensions.
		width, height, err := extractPageSize(pageDict)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to extract page %d size: %w", i, err)
		}

		// Find closest matching standard size or use Custom.
		pageSize := matchStandardSize(width, height)

		// Create domain page.
		domainPage, err := doc.AddPage(pageSize)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to add page %d: %w", i, err)
		}

		// Create creator page wrapper.
		creatorPage := &Page{
			page: domainPage,
			margins: Margins{
				Top:    72,
				Right:  72,
				Bottom: 72,
				Left:   72,
			},
			textOps:     make([]TextOperation, 0),
			graphicsOps: make([]GraphicsOperation, 0),
		}

		pages[i] = creatorPage
	}

	return doc, pages, nil
}

// extractPageSize extracts width and height from page dictionary.
func extractPageSize(pageDict *parser.Dictionary) (float64, float64, error) {
	// Get MediaBox (required for all pages).
	mediaBoxObj := pageDict.Get("MediaBox")
	if mediaBoxObj == nil {
		return 0, 0, fmt.Errorf("MediaBox not found")
	}

	// MediaBox is an array [x1 y1 x2 y2].
	mediaBoxArray, ok := mediaBoxObj.(*parser.Array)
	if !ok {
		return 0, 0, fmt.Errorf("MediaBox is not an array")
	}

	if mediaBoxArray.Len() != 4 {
		return 0, 0, fmt.Errorf("MediaBox must have 4 elements, got %d", mediaBoxArray.Len())
	}

	// Extract coordinates.
	x1, err := getNumericValue(mediaBoxArray, 0)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid MediaBox x1: %w", err)
	}

	y1, err := getNumericValue(mediaBoxArray, 1)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid MediaBox y1: %w", err)
	}

	x2, err := getNumericValue(mediaBoxArray, 2)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid MediaBox x2: %w", err)
	}

	y2, err := getNumericValue(mediaBoxArray, 3)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid MediaBox y2: %w", err)
	}

	width := x2 - x1
	height := y2 - y1

	return width, height, nil
}

// getNumericValue extracts a numeric value from an array at the given index.
func getNumericValue(arr *parser.Array, index int) (float64, error) {
	obj := arr.Get(index)
	if obj == nil {
		return 0, fmt.Errorf("element %d not found", index)
	}

	switch v := obj.(type) {
	case *parser.Integer:
		return float64(v.Value()), nil
	case *parser.Real:
		return v.Value(), nil
	default:
		return 0, fmt.Errorf("element %d is not numeric: %T", index, obj)
	}
}

// matchStandardSize finds the closest matching standard size.
//
// Matches with tolerance of ±5 points to account for rounding variations.
// Returns document.Custom if no match found.
func matchStandardSize(width, height float64) document.PageSize {
	const tolerance = 5.0

	// Standard sizes to check.
	sizes := []struct {
		size   document.PageSize
		width  float64
		height float64
	}{
		{document.A4, 595, 842},
		{document.A3, 842, 1191},
		{document.A5, 420, 595},
		{document.Letter, 612, 792},
		{document.Legal, 612, 1008},
		{document.Tabloid, 792, 1224},
		{document.B4, 709, 1001},
		{document.B5, 499, 709},
	}

	for _, s := range sizes {
		if absFloat(width-s.width) <= tolerance && absFloat(height-s.height) <= tolerance {
			return s.size
		}
	}

	// No match - use Custom.
	return document.Custom
}

// absFloat returns the absolute value of a float64.
func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// Close closes the underlying PDF file and releases resources.
//
// It's safe to call Close() multiple times.
//
// Example:
//
//	app, err := creator.NewAppender("input.pdf")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer app.Close()
func (a *Appender) Close() error {
	if a.pdfReader != nil {
		return a.pdfReader.Close()
	}
	return nil
}

// PageCount returns the total number of pages in the document.
//
// This includes both original pages and newly added pages.
//
// Example:
//
//	count := app.PageCount()
//	fmt.Printf("Document has %d pages\n", count)
func (a *Appender) PageCount() int {
	return len(a.pages) + len(a.newPages)
}

// GetPage returns the page at the specified index (0-based).
//
// This allows you to add content to existing pages.
// The page can be modified by calling methods like AddText, DrawLine, etc.
//
// Returns an error if the page index is out of bounds.
//
// Example:
//
//	page, err := app.GetPage(0)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	page.AddText("Watermark", 300, 400, creator.HelveticaBold, 48)
func (a *Appender) GetPage(index int) (*Page, error) {
	totalPages := a.PageCount()
	if index < 0 || index >= totalPages {
		return nil, fmt.Errorf("page index %d out of bounds (0-%d)", index, totalPages-1)
	}

	// Check if it's an original page or new page.
	if index < len(a.pages) {
		// Original page - mark as modified.
		a.modifiedPages[index] = true
		return a.pages[index], nil
	}

	// New page.
	newPageIndex := index - len(a.pages)
	return a.newPages[newPageIndex], nil
}

// AddPage adds a new page with the specified size.
//
// The new page is appended to the end of the document.
// Returns the newly created page for adding content.
//
// Example:
//
//	page, err := app.AddPage(creator.A4)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	page.AddText("New content", 100, 700, creator.Helvetica, 12)
func (a *Appender) AddPage(size PageSize) (*Page, error) {
	// Add page to domain document.
	domainSize := size.toDomainSize()
	domainPage, err := a.doc.AddPage(domainSize)
	if err != nil {
		return nil, fmt.Errorf("failed to add page: %w", err)
	}

	// Create creator page wrapper.
	creatorPage := &Page{
		page: domainPage,
		margins: Margins{
			Top:    72,
			Right:  72,
			Bottom: 72,
			Left:   72,
		},
		textOps:     make([]TextOperation, 0),
		graphicsOps: make([]GraphicsOperation, 0),
	}

	// Track new page.
	a.newPages = append(a.newPages, creatorPage)

	return creatorPage, nil
}

// WriteToFile writes the modified PDF to a file.
//
// This creates a new PDF file with all modifications applied.
// The original file is not modified.
//
// For large PDFs, consider using WriteToFileIncremental() instead,
// which appends only the changes (not yet implemented).
//
// Example:
//
//	err := app.WriteToFile("output.pdf")
//	if err != nil {
//	    log.Fatal(err)
//	}
func (a *Appender) WriteToFile(path string) error {
	ctx := context.Background()
	return a.WriteToFileContext(ctx, path)
}

// WriteToFileContext writes the modified PDF with context support.
//
// This allows cancellation and timeout control.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	err := app.WriteToFileContext(ctx, "output.pdf")
func (a *Appender) WriteToFileContext(ctx context.Context, path string) error {
	// Check context.
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Create PDF writer.
	w, err := writer.NewPdfWriter(path)
	if err != nil {
		return fmt.Errorf("failed to create PDF writer: %w", err)
	}
	defer func() {
		if closeErr := w.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	// Collect all page contents (original + modified + new).
	allPages := make([]*Page, 0, len(a.pages)+len(a.newPages))
	allPages = append(allPages, a.pages...)
	allPages = append(allPages, a.newPages...)
	textContents, graphicsContents := a.collectPageContents(allPages)

	// Write document with all content.
	if err := w.WriteWithAllContent(a.doc, textContents, graphicsContents); err != nil {
		return fmt.Errorf("failed to write PDF: %w", err)
	}

	return nil
}

// collectPageContents converts creator operations to writer operations.
func (a *Appender) collectPageContents(pages []*Page) (map[int][]writer.TextOp, map[int][]writer.GraphicsOp) {
	textContents := make(map[int][]writer.TextOp)
	graphicsContents := make(map[int][]writer.GraphicsOp)

	for i, page := range pages {
		// Convert text operations.
		if len(page.textOps) > 0 {
			textOps := make([]writer.TextOp, 0, len(page.textOps))
			for _, op := range page.textOps {
				textOps = append(textOps, writer.TextOp{
					Text:  op.Text,
					X:     op.X,
					Y:     op.Y,
					Font:  string(op.Font),
					Size:  op.Size,
					Color: writer.RGB{R: op.Color.R, G: op.Color.G, B: op.Color.B},
				})
			}
			textContents[i] = textOps
		}

		// Convert graphics operations.
		if len(page.graphicsOps) > 0 {
			graphicsOps := make([]writer.GraphicsOp, 0, len(page.graphicsOps))
			for _, op := range page.graphicsOps {
				gop := writer.GraphicsOp{
					Type:   int(op.Type),
					X:      op.X,
					Y:      op.Y,
					X2:     op.X2,
					Y2:     op.Y2,
					Width:  op.Width,
					Height: op.Height,
					Radius: op.Radius,
				}

				// Convert options.
				convertGraphicsOptions(&gop, &op)
				graphicsOps = append(graphicsOps, gop)
			}
			graphicsContents[i] = graphicsOps
		}
	}

	return textContents, graphicsContents
}

// GetParserReader returns the underlying parser.Reader for advanced operations.
//
// This is useful for extracting text, images, or other content from the original PDF.
// Most users should not need this - use the high-level Appender methods instead.
//
// Example:
//
//	parserReader := app.GetParserReader()
//	// Advanced parser operations...
func (a *Appender) GetParserReader() *parser.Reader {
	return a.pdfReader.GetParserReader()
}

// Document returns the underlying domain document.
//
// This is provided for advanced use cases where you need direct access
// to the domain model. Most users should use the Appender API instead.
//
// Example:
//
//	doc := app.Document()
//	// Direct domain operations...
func (a *Appender) Document() *document.Document {
	return a.doc
}

// SetMetadata updates the document metadata.
//
// This replaces any existing metadata in the original PDF.
//
// Example:
//
//	app.SetMetadata("Modified Document", "John Doe", "Updated Report")
func (a *Appender) SetMetadata(title, author, subject string) {
	a.doc.SetMetadata(title, author, subject)
}

// SetKeywords sets document keywords for search/indexing.
//
// Example:
//
//	app.SetKeywords("modified", "watermarked", "confidential")
func (a *Appender) SetKeywords(keywords ...string) {
	a.doc.SetMetadata("", "", "", keywords...)
}

// SetFieldValue sets a form field value by name.
//
// The value type depends on the field type:
//   - Text field: string
//   - Checkbox: bool or string ("Yes", "Off")
//   - Radio button: string (option name)
//   - Choice field: string or []string (for multi-select)
//
// Returns an error if:
//   - The field is not found
//   - The PDF has no interactive form
//   - The value type is incompatible with the field type
//
// Example:
//
//	app, err := creator.NewAppender("form.pdf")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer app.Close()
//
//	// Fill text field
//	if err := app.SetFieldValue("name", "John Doe"); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Fill checkbox
//	if err := app.SetFieldValue("agree", true); err != nil {
//	    log.Fatal(err)
//	}
//
//	app.WriteToFile("filled.pdf")
func (a *Appender) SetFieldValue(name string, value interface{}) error {
	if a.formWriter == nil {
		parserReader := a.pdfReader.GetParserReader()
		a.formWriter = forms.NewWriter(parserReader)
	}

	// Validate and set the value
	if err := a.formWriter.ValidateFieldValue(name, value); err != nil {
		return err
	}

	return a.formWriter.SetFieldValue(name, value)
}

// GetFieldValue returns the current value of a form field.
//
// Returns the value from pending updates if the field has been modified,
// otherwise returns the original value from the PDF.
//
// Example:
//
//	value, err := app.GetFieldValue("name")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Name: %v\n", value)
func (a *Appender) GetFieldValue(name string) (interface{}, error) {
	// Check for pending update first
	if a.formWriter != nil {
		updates := a.formWriter.GetUpdates()
		if value, exists := updates[name]; exists {
			return value, nil
		}
	}

	// Get original value from PDF
	parserReader := a.pdfReader.GetParserReader()
	reader := forms.NewReader(parserReader)
	field, err := reader.GetFieldByName(name)
	if err != nil {
		return nil, err
	}

	return field.Value, nil
}

// GetFormFields returns all form fields from the PDF.
//
// This includes field metadata such as name, type, current value, and options.
//
// Example:
//
//	fields, err := app.GetFormFields()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, f := range fields {
//	    fmt.Printf("%s (%s): %v\n", f.Name, f.Type, f.Value)
//	}
func (a *Appender) GetFormFields() ([]*forms.FieldInfo, error) {
	parserReader := a.pdfReader.GetParserReader()
	reader := forms.NewReader(parserReader)
	return reader.GetFields()
}

// HasForm returns true if the PDF contains an interactive form.
//
// Example:
//
//	if app.HasForm() {
//	    fields, _ := app.GetFormFields()
//	    fmt.Printf("Found %d form fields\n", len(fields))
//	}
func (a *Appender) HasForm() bool {
	parserReader := a.pdfReader.GetParserReader()
	acroForm, err := parserReader.GetAcroForm()
	return err == nil && acroForm != nil
}

// FlattenForm converts all form fields to static content.
//
// This removes interactivity by rendering field appearances directly onto
// pages. The resulting PDF looks the same but fields are no longer editable.
//
// Use this when:
//   - Creating final versions of filled forms
//   - Preventing further editing
//   - Reducing file complexity
//
// Example:
//
//	app, err := creator.NewAppender("filled_form.pdf")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer app.Close()
//
//	if err := app.FlattenForm(); err != nil {
//	    log.Fatal(err)
//	}
//
//	app.WriteToFile("flattened.pdf")
func (a *Appender) FlattenForm() error {
	return a.FlattenFields() // Flatten all fields
}

// FlattenFields converts specific form fields to static content.
//
// If no names are provided, all fields are flattened.
//
// Example:
//
//	// Flatten only specific fields
//	app.FlattenFields("name", "email", "signature")
func (a *Appender) FlattenFields(names ...string) error {
	parserReader := a.pdfReader.GetParserReader()
	flattener := forms.NewFlattener(parserReader)

	var flattenInfo []*forms.FlattenInfo
	var err error

	if len(names) == 0 {
		flattenInfo, err = flattener.GetFlattenInfo()
	} else {
		flattenInfo, err = flattener.GetFlattenInfoByName(names...)
	}

	if err != nil {
		return fmt.Errorf("failed to get flatten info: %w", err)
	}

	if len(flattenInfo) == 0 {
		return nil // Nothing to flatten
	}

	// Track which fields were flattened for later removal
	a.flattenedFields = append(a.flattenedFields, flattenInfo...)

	return nil
}

// CanFlattenForm returns true if the document has fields that can be flattened.
func (a *Appender) CanFlattenForm() bool {
	parserReader := a.pdfReader.GetParserReader()
	flattener := forms.NewFlattener(parserReader)
	return flattener.CanFlatten()
}
