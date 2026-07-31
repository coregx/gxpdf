package gxpdf

import (
	"context"
	"fmt"

	"github.com/coregx/gxpdf/internal/application/forms"
	"github.com/coregx/gxpdf/internal/extractor"
	"github.com/coregx/gxpdf/internal/parser"
	"github.com/coregx/gxpdf/internal/tabledetect"
	"github.com/coregx/gxpdf/logging"
)

// Document represents an opened PDF document.
//
// Document provides methods for reading document properties and extracting content.
// It must be closed after use to release resources.
//
// Example:
//
//	doc, err := gxpdf.Open("document.pdf")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer doc.Close()
//
//	fmt.Printf("Pages: %d\n", doc.PageCount())
//	tables := doc.ExtractTables()
type Document struct {
	reader *parser.Reader
	ctx    context.Context
	path   string
}

// Close closes the document and releases resources.
//
// It is safe to call Close multiple times.
func (d *Document) Close() error {
	if d.reader != nil {
		return d.reader.Close()
	}
	return nil
}

// Path returns the file path of the document.
func (d *Document) Path() string {
	return d.path
}

// PageCount returns the total number of pages in the document.
//
// Returns 0 if an error occurs. Errors are logged via slog.
func (d *Document) PageCount() int {
	count, err := d.reader.GetPageCount()
	if err != nil {
		logging.Logger().Error("failed to get page count",
			"path", d.path,
			"error", err)
		return 0
	}
	return count
}

// Page returns the page at the given index (0-based).
//
// Returns nil if the index is out of bounds.
func (d *Document) Page(index int) *Page {
	if index < 0 || index >= d.PageCount() {
		return nil
	}
	return &Page{
		doc:   d,
		index: index,
	}
}

// Pages returns an iterator over all pages.
//
// Example:
//
//	for _, page := range doc.Pages() {
//	    text := page.ExtractText()
//	    fmt.Println(text)
//	}
func (d *Document) Pages() []*Page {
	count := d.PageCount()
	pages := make([]*Page, count)
	for i := 0; i < count; i++ {
		pages[i] = &Page{doc: d, index: i}
	}
	return pages
}

// ExtractTables extracts all tables from all pages.
//
// This is the simplest way to extract tables - uses automatic detection
// with the 4-Pass Hybrid algorithm for best accuracy.
//
// Errors are logged via slog. For error handling, use ExtractTablesWithOptions.
//
// Example:
//
//	tables := doc.ExtractTables()
//	for _, t := range tables {
//	    fmt.Printf("Table on page %d: %d rows x %d cols\n",
//	        t.PageNumber(), t.RowCount(), t.ColumnCount())
//	}
func (d *Document) ExtractTables() []*Table {
	tables, err := d.ExtractTablesWithOptions(nil)
	if err != nil {
		logging.Logger().Error("failed to extract tables from document",
			"path", d.path,
			"error", err)
	}
	return tables
}

// ExtractTablesWithOptions extracts tables with custom options.
//
// Example:
//
//	opts := &gxpdf.ExtractionOptions{
//	    Method: gxpdf.MethodLattice,
//	    Pages:  []int{0, 1, 2},
//	}
//	tables, err := doc.ExtractTablesWithOptions(opts)
func (d *Document) ExtractTablesWithOptions(opts *ExtractionOptions) ([]*Table, error) {
	if opts == nil {
		opts = DefaultExtractionOptions()
	}

	// Determine pages to process
	pages := opts.Pages
	if len(pages) == 0 {
		count := d.PageCount()
		pages = make([]int, count)
		for i := 0; i < count; i++ {
			pages[i] = i
		}
	}

	// Create extractors
	textExtractor := extractor.NewTextExtractor(d.reader)
	graphicsParser := extractor.NewGraphicsParser(d.reader)

	var allTables []*Table

	for _, pageIndex := range pages {
		// Check context cancellation
		select {
		case <-d.ctx.Done():
			return allTables, d.ctx.Err()
		default:
		}

		// Extract text elements
		textElements, err := textExtractor.ExtractFromPage(pageIndex)
		if err != nil {
			return nil, fmt.Errorf("gxpdf: failed to extract text from page %d: %w", pageIndex, err)
		}

		// Detect tables
		tableDetector := tabledetect.NewDefaultTableDetector()

		var detectedTables []*tabledetect.TableRegion

		switch opts.Method {
		case MethodLattice:
			graphicsElements, gErr := graphicsParser.ParseFromPage(pageIndex)
			if gErr != nil {
				return nil, fmt.Errorf("gxpdf: failed to extract graphics from page %d: %w", pageIndex, gErr)
			}
			detectedTables, err = tableDetector.DetectTablesLattice(textElements, graphicsElements)
		case MethodStream:
			detectedTables, err = tableDetector.DetectTablesStream(textElements)
		default:
			graphicsElements, gErr := graphicsParser.ParseFromPage(pageIndex)
			if gErr != nil {
				return nil, fmt.Errorf("gxpdf: failed to extract graphics from page %d: %w", pageIndex, gErr)
			}
			detectedTables, err = tableDetector.DetectTables(textElements, graphicsElements)
		}

		if err != nil {
			return nil, fmt.Errorf("gxpdf: failed to detect tables on page %d: %w", pageIndex, err)
		}

		// Normalize text coordinates to match grid space for Lattice tables.
		normalizedText := textElements
		if opts.Method != MethodStream && len(detectedTables) > 0 {
			pageHeight := extractor.ReadPageHeight(d.reader, pageIndex)
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
			extracted.PageNum = pageIndex

			allTables = append(allTables, &Table{internal: extracted})
		}
	}

	return allTables, nil
}

// GetImages extracts all images from all pages in the document.
//
// This is the simplest way to extract images - returns all images found
// across all pages.
//
// Errors are logged via slog. For error handling, use GetImagesWithError.
//
// Example:
//
//	images := doc.GetImages()
//	for i, img := range images {
//	    fmt.Printf("Image %d: %dx%d, %s\n", i, img.Width(), img.Height(), img.ColorSpace())
//	    img.SaveToFile(fmt.Sprintf("image_%d.jpg", i))
//	}
func (d *Document) GetImages() []*Image {
	images, err := d.GetImagesWithError()
	if err != nil {
		logging.Logger().Error("failed to extract images from document",
			"path", d.path,
			"error", err)
	}
	return images
}

// GetImagesWithError extracts all images from all pages, returning any errors.
//
// Use this when you need error handling for image extraction.
func (d *Document) GetImagesWithError() ([]*Image, error) {
	imageExtractor := extractor.NewImageExtractor(d.reader)
	internalImages, err := imageExtractor.ExtractFromDocument()
	if err != nil {
		return nil, fmt.Errorf("gxpdf: failed to extract images: %w", err)
	}

	// Wrap internal images in public API
	images := make([]*Image, len(internalImages))
	for i, internal := range internalImages {
		images[i] = &Image{internal: internal}
	}

	return images, nil
}

// Info returns document metadata.
func (d *Document) Info() *DocumentInfo {
	pinfo := d.reader.GetDocumentInfo()
	return &DocumentInfo{
		PageCount: d.PageCount(),
		Path:      d.path,
		Version:   pinfo.Version,
		Title:     pinfo.Title,
		Author:    pinfo.Author,
		Subject:   pinfo.Subject,
		Keywords:  pinfo.Keywords,
		Creator:   pinfo.Creator,
		Producer:  pinfo.Producer,
		Encrypted: pinfo.Encrypted,
	}
}

// Version returns the PDF version (e.g., "1.7").
func (d *Document) Version() string {
	return d.reader.GetDocumentInfo().Version
}

// Title returns the document title.
func (d *Document) Title() string {
	return d.reader.GetDocumentInfo().Title
}

// Author returns the document author.
func (d *Document) Author() string {
	return d.reader.GetDocumentInfo().Author
}

// Subject returns the document subject.
func (d *Document) Subject() string {
	return d.reader.GetDocumentInfo().Subject
}

// Keywords returns the document keywords.
func (d *Document) Keywords() string {
	return d.reader.GetDocumentInfo().Keywords
}

// Creator returns the application that created the document.
func (d *Document) Creator() string {
	return d.reader.GetDocumentInfo().Creator
}

// Producer returns the PDF producer.
func (d *Document) Producer() string {
	return d.reader.GetDocumentInfo().Producer
}

// IsEncrypted returns true if the document is encrypted.
func (d *Document) IsEncrypted() bool {
	return d.reader.GetDocumentInfo().Encrypted
}

// ExtractTextFromPage extracts text from a specific page (1-based).
func (d *Document) ExtractTextFromPage(pageNum int) (string, error) {
	if pageNum < 1 || pageNum > d.PageCount() {
		return "", fmt.Errorf("page %d out of range (1-%d)", pageNum, d.PageCount())
	}

	// Extract directly to propagate errors
	textExtractor := extractor.NewTextExtractor(d.reader)
	elements, err := textExtractor.ExtractFromPage(pageNum - 1) // Convert to 0-based
	if err != nil {
		return "", fmt.Errorf("failed to extract text from page %d: %w", pageNum, err)
	}

	return extractor.AssembleText(elements), nil
}

// ExtractTextElementsFromPage extracts positioned text elements from a specific page.
//
// pageNum is 1-based. Returns an error if pageNum is out of range.
//
// This is a convenience wrapper around Page.ExtractTextElements.
//
// Example:
//
//	elements, err := doc.ExtractTextElementsFromPage(1)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, e := range elements {
//	    fmt.Printf("%q at (%.1f, %.1f)\n", e.Text, e.X, e.Y)
//	}
func (d *Document) ExtractTextElementsFromPage(pageNum int) ([]TextElement, error) {
	if pageNum < 1 || pageNum > d.PageCount() {
		return nil, fmt.Errorf("gxpdf: page %d out of range (1-%d)", pageNum, d.PageCount())
	}
	page := d.Page(pageNum - 1)
	return page.ExtractTextElements()
}

// ExtractTablesFromPage extracts tables from a specific page (1-based).
//
// Errors are logged via slog. For error handling, use Page.ExtractTablesWithOptions.
func (d *Document) ExtractTablesFromPage(pageNum int) []*Table {
	if pageNum < 1 || pageNum > d.PageCount() {
		logging.Logger().Error("page number out of range",
			"page", pageNum,
			"total_pages", d.PageCount())
		return nil
	}
	page := d.Page(pageNum - 1)
	if page == nil {
		logging.Logger().Error("page not found",
			"page", pageNum)
		return nil
	}
	return page.ExtractTables()
}

// DocumentInfo contains metadata about a PDF document.
type DocumentInfo struct {
	PageCount int
	Path      string
	Version   string
	Title     string
	Author    string
	Subject   string
	Keywords  string
	Creator   string
	Producer  string
	Encrypted bool
}

// FormField represents an interactive form field in the document.
//
// FormField provides read-only access to form field properties.
// Use Document methods to get and set field values.
type FormField struct {
	internal *forms.FieldInfo
}

// Name returns the fully qualified field name.
func (f *FormField) Name() string {
	return f.internal.Name
}

// Type returns the field type.
//   - "Tx" = Text field
//   - "Btn" = Button (checkbox, radio)
//   - "Ch" = Choice (dropdown, list)
//   - "Sig" = Signature
func (f *FormField) Type() string {
	return string(f.internal.Type)
}

// Value returns the current field value.
func (f *FormField) Value() interface{} {
	return f.internal.Value
}

// DefaultValue returns the field's default value.
func (f *FormField) DefaultValue() interface{} {
	return f.internal.DefaultValue
}

// Flags returns the field flags bitmask.
func (f *FormField) Flags() int {
	return f.internal.Flags
}

// Rect returns the field rectangle [x1, y1, x2, y2].
func (f *FormField) Rect() [4]float64 {
	return f.internal.Rect
}

// Options returns the available options for choice fields.
func (f *FormField) Options() []string {
	return f.internal.Options
}

// IsReadOnly returns true if the field is read-only.
func (f *FormField) IsReadOnly() bool {
	return f.internal.Flags&1 != 0
}

// IsRequired returns true if the field is required.
func (f *FormField) IsRequired() bool {
	return f.internal.Flags&2 != 0
}

// IsTextField returns true if this is a text field.
func (f *FormField) IsTextField() bool {
	return f.internal.Type == forms.FieldTypeText
}

// IsButton returns true if this is a button field (checkbox, radio).
func (f *FormField) IsButton() bool {
	return f.internal.Type == forms.FieldTypeButton
}

// IsChoice returns true if this is a choice field (dropdown, list).
func (f *FormField) IsChoice() bool {
	return f.internal.Type == forms.FieldTypeChoice
}

// GetFormFields returns all interactive form fields in the document.
//
// Returns nil if the document has no interactive form.
//
// Example:
//
//	fields, err := doc.GetFormFields()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, f := range fields {
//	    fmt.Printf("%s (%s): %v\n", f.Name(), f.Type(), f.Value())
//	}
func (d *Document) GetFormFields() ([]*FormField, error) {
	reader := forms.NewReader(d.reader)
	internalFields, err := reader.GetFields()
	if err != nil {
		return nil, fmt.Errorf("failed to get form fields: %w", err)
	}

	if internalFields == nil {
		return nil, nil
	}

	fields := make([]*FormField, len(internalFields))
	for i, internal := range internalFields {
		fields[i] = &FormField{internal: internal}
	}

	return fields, nil
}

// GetFieldValue returns the value of a form field by name.
//
// Returns an error if the field is not found.
//
// Example:
//
//	value, err := doc.GetFieldValue("username")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Username: %v\n", value)
func (d *Document) GetFieldValue(name string) (interface{}, error) {
	reader := forms.NewReader(d.reader)
	field, err := reader.GetFieldByName(name)
	if err != nil {
		return nil, err
	}
	return field.Value, nil
}

// HasForm returns true if the document contains an interactive form.
func (d *Document) HasForm() bool {
	acroForm, err := d.reader.GetAcroForm()
	return err == nil && acroForm != nil
}
