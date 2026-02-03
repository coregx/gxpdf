// Package creator provides a high-level API for creating PDF documents.
//
// This package offers a fluent, developer-friendly interface for PDF creation,
// hiding the complexity of the underlying domain model.
//
// Example:
//
//	c := creator.New()
//	c.SetTitle("My Document")
//	c.SetAuthor("John Doe")
//
//	page := c.NewPage(creator.A4)
//	// Add content to page...
//
//	err := c.WriteToFile("output.pdf")
package creator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/coregx/gxpdf/internal/document"
	"github.com/coregx/gxpdf/internal/fonts"
	"github.com/coregx/gxpdf/internal/writer"
)

// Creator is a high-level API for creating PDF documents.
//
// It provides a fluent interface for document creation with sensible defaults
// and simplified methods. Creator wraps the underlying domain model (Document)
// and provides a more intuitive API for common use cases.
//
// # Thread Safety
//
// Creator is NOT safe for concurrent use. Each goroutine should create its
// own Creator instance. However, multiple Creator instances can safely be
// used concurrently without synchronization.
//
// # Example
//
//	c := creator.New()
//	c.SetPageSize(creator.A4)
//	c.SetMargins(72, 72, 72, 72) // 1 inch on all sides
//
//	page := c.NewPage()
//	// Add content...
//
//	c.WriteToFile("output.pdf")
type Creator struct {
	// Domain model
	doc *document.Document

	// Default settings (applied to new pages)
	defaultPageSize document.PageSize
	defaultMargins  Margins

	// Creator pages (with content operations)
	pages []*Page

	// Header and footer configuration
	headerFunc      HeaderFunc
	footerFunc      FooterFunc
	headerHeight    float64
	footerHeight    float64
	skipHeaderFirst bool
	skipFooterFirst bool

	// Encryption options (set via SetEncryption)
	encryptionOpts *EncryptionOptions

	// Bookmarks (document outline)
	bookmarks []Bookmark

	// Table of Contents (TOC)
	tocEnabled bool
	toc        *TOC

	// Chapters (document structure)
	chapters []*Chapter
}

// Margins represents page margins in points (1 point = 1/72 inch).
type Margins struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
}

// New creates a new Creator with default settings.
//
// Default settings:
// - Page size: A4 (595 × 842 points)
// - Margins: 72 points (1 inch) on all sides
// - PDF version: 1.7
//
// Example:
//
//	c := creator.New()
//	c.SetTitle("My Document")
func New() *Creator {
	return &Creator{
		doc:             document.NewDocument(),
		defaultPageSize: document.A4,
		defaultMargins: Margins{
			Top:    72, // 1 inch
			Right:  72,
			Bottom: 72,
			Left:   72,
		},
		pages:        make([]*Page, 0),
		headerHeight: DefaultHeaderHeight,
		footerHeight: DefaultFooterHeight,
		bookmarks:    make([]Bookmark, 0),
		tocEnabled:   false,
		toc:          NewTOC(),
		chapters:     make([]*Chapter, 0),
	}
}

// NewPage adds a new page with the default page size.
//
// The page uses the default page size set via SetPageSize.
// If no default is set, A4 is used.
//
// Returns the newly created page for method chaining.
//
// Example:
//
//	page := c.NewPage()
//	// Add content to page...
func (c *Creator) NewPage() (*Page, error) {
	domainPage, err := c.doc.AddPage(c.defaultPageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to add page: %w", err)
	}

	// Wrap domain page in creator page
	creatorPage := &Page{
		page:        domainPage,
		margins:     c.defaultMargins,
		textOps:     make([]TextOperation, 0),
		graphicsOps: make([]GraphicsOperation, 0),
	}

	// Track creator page
	c.pages = append(c.pages, creatorPage)

	return creatorPage, nil
}

// NewPageWithSize adds a new page with a specific size.
//
// This overrides the default page size for this specific page.
//
// Example:
//
//	page := c.NewPageWithSize(creator.Letter)
func (c *Creator) NewPageWithSize(size PageSize) (*Page, error) {
	domainSize := size.toDomainSize()
	domainPage, err := c.doc.AddPage(domainSize)
	if err != nil {
		return nil, fmt.Errorf("failed to add page: %w", err)
	}

	creatorPage := &Page{
		page:        domainPage,
		margins:     c.defaultMargins,
		textOps:     make([]TextOperation, 0),
		graphicsOps: make([]GraphicsOperation, 0),
	}

	// Track creator page
	c.pages = append(c.pages, creatorPage)

	return creatorPage, nil
}

// SetPageSize sets the default page size for new pages.
//
// This affects all pages added after calling this method.
// Existing pages are not affected.
//
// Example:
//
//	c.SetPageSize(creator.Letter) // 8.5 × 11 inches
//	c.NewPage() // Uses Letter size
func (c *Creator) SetPageSize(size PageSize) {
	c.defaultPageSize = size.toDomainSize()
}

// SetMargins sets the default margins for new pages.
//
// Margins are specified in points (1 point = 1/72 inch).
//
// Example:
//
//	c.SetMargins(72, 72, 72, 72) // 1 inch on all sides
//	c.SetMargins(36, 36, 36, 36) // 0.5 inch on all sides
func (c *Creator) SetMargins(top, right, bottom, left float64) error {
	if top < 0 || right < 0 || bottom < 0 || left < 0 {
		return ErrInvalidMargins
	}

	c.defaultMargins = Margins{
		Top:    top,
		Right:  right,
		Bottom: bottom,
		Left:   left,
	}
	return nil
}

// SetTitle sets the document title.
//
// Example:
//
//	c.SetTitle("Annual Report 2025")
func (c *Creator) SetTitle(title string) {
	c.doc.SetMetadata(title, "", "")
}

// SetAuthor sets the document author.
//
// Example:
//
//	c.SetAuthor("John Doe")
func (c *Creator) SetAuthor(author string) {
	c.doc.SetMetadata("", author, "")
}

// SetSubject sets the document subject.
//
// Example:
//
//	c.SetSubject("Financial Report")
func (c *Creator) SetSubject(subject string) {
	c.doc.SetMetadata("", "", subject)
}

// SetMetadata sets all document metadata at once.
//
// Example:
//
//	c.SetMetadata("My Document", "John Doe", "Annual Report")
func (c *Creator) SetMetadata(title, author, subject string) {
	c.doc.SetMetadata(title, author, subject)
}

// SetKeywords sets document keywords for search/indexing.
//
// Example:
//
//	c.SetKeywords("report", "2025", "finance", "annual")
func (c *Creator) SetKeywords(keywords ...string) {
	c.doc.SetMetadata("", "", "", keywords...)
}

// SetHeaderFunc sets the function to render headers on each page.
//
// The function is called once for each page during PDF generation.
// It receives page information and a Block to draw header content into.
//
// Example:
//
//	c.SetHeaderFunc(func(args HeaderFunctionArgs) {
//	    p := NewParagraph("Document Title")
//	    p.SetFont(HelveticaBold, 10)
//	    args.Block.Draw(p)
//	})
func (c *Creator) SetHeaderFunc(f HeaderFunc) {
	c.headerFunc = f
}

// SetFooterFunc sets the function to render footers on each page.
//
// The function is called once for each page during PDF generation.
// It receives page information and a Block to draw footer content into.
//
// Example:
//
//	c.SetFooterFunc(func(args FooterFunctionArgs) {
//	    text := fmt.Sprintf("Page %d of %d", args.PageNum, args.TotalPages)
//	    p := NewParagraph(text)
//	    p.SetAlignment(AlignCenter)
//	    args.Block.Draw(p)
//	})
func (c *Creator) SetFooterFunc(f FooterFunc) {
	c.footerFunc = f
}

// SetHeaderHeight sets the height reserved for headers in points.
//
// Default: 50 points.
//
// Example:
//
//	c.SetHeaderHeight(40)  // 40 points for header
func (c *Creator) SetHeaderHeight(h float64) {
	c.headerHeight = h
}

// SetFooterHeight sets the height reserved for footers in points.
//
// Default: 30 points.
//
// Example:
//
//	c.SetFooterHeight(25)  // 25 points for footer
func (c *Creator) SetFooterHeight(h float64) {
	c.footerHeight = h
}

// HeaderHeight returns the current header height in points.
func (c *Creator) HeaderHeight() float64 {
	return c.headerHeight
}

// FooterHeight returns the current footer height in points.
func (c *Creator) FooterHeight() float64 {
	return c.footerHeight
}

// SetSkipHeaderOnFirstPage sets whether to skip the header on the first page.
//
// This is useful for documents with a title page that should not have a header.
//
// Example:
//
//	c.SetSkipHeaderOnFirstPage(true)  // No header on page 1
func (c *Creator) SetSkipHeaderOnFirstPage(skip bool) {
	c.skipHeaderFirst = skip
}

// SetSkipFooterOnFirstPage sets whether to skip the footer on the first page.
//
// This is useful for documents with a title page that should not have a footer.
//
// Example:
//
//	c.SetSkipFooterOnFirstPage(true)  // No footer on page 1
func (c *Creator) SetSkipFooterOnFirstPage(skip bool) {
	c.skipFooterFirst = skip
}

// SkipHeaderOnFirstPage returns whether headers are skipped on the first page.
func (c *Creator) SkipHeaderOnFirstPage() bool {
	return c.skipHeaderFirst
}

// SkipFooterOnFirstPage returns whether footers are skipped on the first page.
func (c *Creator) SkipFooterOnFirstPage() bool {
	return c.skipFooterFirst
}

// PageCount returns the number of pages in the document.
func (c *Creator) PageCount() int {
	return c.doc.PageCount()
}

// EnableTOC enables automatic Table of Contents generation.
//
// When enabled, the TOC will be inserted at the beginning of the document
// and will include all chapters added via AddChapter.
//
// The TOC includes clickable links to each chapter and section.
//
// Example:
//
//	c := creator.New()
//	c.EnableTOC()
//	ch1 := creator.NewChapter("Introduction")
//	c.AddChapter(ch1)
//	c.WriteToFile("document.pdf")  // TOC is automatically generated
func (c *Creator) EnableTOC() {
	c.tocEnabled = true
}

// DisableTOC disables automatic Table of Contents generation.
func (c *Creator) DisableTOC() {
	c.tocEnabled = false
}

// TOCEnabled returns whether TOC generation is enabled.
func (c *Creator) TOCEnabled() bool {
	return c.tocEnabled
}

// TOC returns the Table of Contents instance for customization.
//
// This allows customizing TOC appearance before rendering.
//
// Example:
//
//	c.EnableTOC()
//	toc := c.TOC()
//	toc.SetTitle("Contents")
//	style := toc.Style()
//	style.TitleSize = 28
//	toc.SetStyle(style)
func (c *Creator) TOC() *TOC {
	return c.toc
}

// AddChapter adds a chapter to the document.
//
// Chapters provide document structure with automatic numbering and
// optional Table of Contents integration.
//
// The chapter will be rendered on a new page, and all sub-chapters
// will be included automatically.
//
// Example:
//
//	c := creator.New()
//	c.EnableTOC()
//
//	ch1 := creator.NewChapter("Introduction")
//	ch1.Add(creator.NewParagraph("This is the introduction..."))
//	c.AddChapter(ch1)
//
//	ch2 := creator.NewChapter("Methods")
//	sec := ch2.NewSubChapter("Background")
//	c.AddChapter(ch2)
func (c *Creator) AddChapter(ch *Chapter) error {
	if ch == nil {
		return errors.New("cannot add nil chapter")
	}

	// Assign chapter number
	ch.assignNumbers([]int{}, len(c.chapters))

	// Add to chapters list
	c.chapters = append(c.chapters, ch)

	return nil
}

// Chapters returns all top-level chapters in the document.
func (c *Creator) Chapters() []*Chapter {
	return c.chapters
}

// Validate checks if the document is valid and ready to be written.
//
// Returns an error if:
// - Document has no pages
// - Any page validation fails
//
// It's recommended to call this before WriteToFile to catch errors early.
func (c *Creator) Validate() error {
	if err := c.doc.Validate(); err != nil {
		return fmt.Errorf("document validation failed: %w", err)
	}
	return nil
}

// WriteToFile writes the PDF document to a file.
//
// This will:
// 1. Validate the document
// 2. Generate the PDF structure
// 3. Write to the specified file
//
// Returns an error if validation or writing fails.
//
// Example:
//
//	err := c.WriteToFile("output.pdf")
//	if err != nil {
//	    log.Fatal(err)
//	}
func (c *Creator) WriteToFile(path string) error {
	ctx := context.Background()
	return c.WriteToFileContext(ctx, path)
}

// WriteToFileContext writes the PDF document to a file with context support.
//
// This allows cancellation and timeout control. The context is checked
// at multiple points during PDF generation:
//   - Before rendering TOC and chapters
//   - Before validation
//   - Before writing to file
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	err := c.WriteToFileContext(ctx, "output.pdf")
func (c *Creator) WriteToFileContext(ctx context.Context, path string) error {
	// Check context before starting.
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled before PDF generation: %w", err)
	}

	// Render TOC and chapters if enabled.
	if err := c.renderTOCAndChapters(); err != nil {
		return fmt.Errorf("failed to render TOC and chapters: %w", err)
	}

	// Check context after rendering chapters.
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled during TOC/chapter rendering: %w", err)
	}

	// Validate before writing.
	if err := c.Validate(); err != nil {
		return err
	}

	// Check context before file operations.
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled before file write: %w", err)
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

	// Write document with page content (text and graphics).
	textContents, graphicsContents := c.collectAllPageContents()
	if err := w.WriteWithAllContent(c.doc, textContents, graphicsContents); err != nil {
		return fmt.Errorf("failed to write PDF: %w", err)
	}

	return nil
}

// WriteTo writes the PDF document to an io.Writer.
//
// This implements io.WriterTo and is useful for:
//   - Writing to HTTP responses
//   - Writing to memory buffers
//   - Writing to network connections
//   - Any other io.Writer implementation
//
// Example:
//
//	var buf bytes.Buffer
//	n, err := c.WriteTo(&buf)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Wrote %d bytes\n", n)
func (c *Creator) WriteTo(w io.Writer) (int64, error) {
	ctx := context.Background()
	return c.WriteToContext(ctx, w)
}

// WriteToContext writes the PDF document to an io.Writer with context support.
//
// This allows cancellation and timeout control during PDF generation.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	var buf bytes.Buffer
//	n, err := c.WriteToContext(ctx, &buf)
func (c *Creator) WriteToContext(ctx context.Context, w io.Writer) (int64, error) {
	// Check context before starting.
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("context canceled before PDF generation: %w", err)
	}

	// Render TOC and chapters if enabled.
	if err := c.renderTOCAndChapters(); err != nil {
		return 0, fmt.Errorf("failed to render TOC and chapters: %w", err)
	}

	// Check context after rendering chapters.
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("context canceled during TOC/chapter rendering: %w", err)
	}

	// Validate before writing.
	if err := c.Validate(); err != nil {
		return 0, err
	}

	// Check context before write.
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("context canceled before write: %w", err)
	}

	// Use counting writer to track bytes written.
	cw := &countingWriter{w: w}

	// Create PDF writer for io.Writer.
	pdfWriter := writer.NewPdfWriterFromWriter(cw)
	defer pdfWriter.Close()

	// Write document with page content.
	textContents, graphicsContents := c.collectAllPageContents()
	if err := pdfWriter.WriteWithAllContent(c.doc, textContents, graphicsContents); err != nil {
		return cw.n, fmt.Errorf("failed to write PDF: %w", err)
	}

	return cw.n, nil
}

// Bytes returns the PDF document as a byte slice.
//
// This is a convenience method that writes to an in-memory buffer.
// For large documents, consider using WriteTo with a streaming writer.
//
// Example:
//
//	pdfBytes, err := c.Bytes()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Use pdfBytes...
func (c *Creator) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	_, err := c.WriteTo(&buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// countingWriter wraps an io.Writer and counts bytes written.
type countingWriter struct {
	w io.Writer
	n int64
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	cw.n += int64(n)
	return n, err
}

// collectAllPageContents converts creator operations to writer operations.
func (c *Creator) collectAllPageContents() (map[int][]writer.TextOp, map[int][]writer.GraphicsOp) {
	textContents := make(map[int][]writer.TextOp)
	graphicsContents := make(map[int][]writer.GraphicsOp)
	totalPages := len(c.pages)

	for i, creatorPage := range c.pages {
		pageNum := i + 1 // 1-based page number

		// Collect page text/graphics operations.
		var pageTextOps []TextOperation
		var pageGraphicsOps []GraphicsOperation

		// Add header content.
		if c.headerFunc != nil && !c.shouldSkipHeader(pageNum) {
			headerOps := c.renderHeader(creatorPage, pageNum, totalPages)
			pageTextOps = append(pageTextOps, headerOps...)
		}

		// Add main page content.
		pageTextOps = append(pageTextOps, creatorPage.textOps...)
		pageGraphicsOps = append(pageGraphicsOps, creatorPage.graphicsOps...)

		// Add footer content.
		if c.footerFunc != nil && !c.shouldSkipFooter(pageNum) {
			footerOps := c.renderFooter(creatorPage, pageNum, totalPages)
			pageTextOps = append(pageTextOps, footerOps...)
		}

		// Convert to writer operations.
		if len(pageTextOps) > 0 {
			textContents[i] = convertTextOps(pageTextOps)
		}
		if len(pageGraphicsOps) > 0 {
			graphicsContents[i] = convertGraphicsOps(pageGraphicsOps)
		}
	}

	return textContents, graphicsContents
}

// shouldSkipHeader returns true if header should be skipped for the given page.
func (c *Creator) shouldSkipHeader(pageNum int) bool {
	return c.skipHeaderFirst && pageNum == 1
}

// shouldSkipFooter returns true if footer should be skipped for the given page.
func (c *Creator) shouldSkipFooter(pageNum int) bool {
	return c.skipFooterFirst && pageNum == 1
}

// renderHeader renders header content for a page and returns text operations.
func (c *Creator) renderHeader(page *Page, pageNum, totalPages int) []TextOperation {
	// Create header block.
	headerWidth := page.Width() - page.margins.Left - page.margins.Right
	block := NewBlock(headerWidth, c.headerHeight)

	// Call the header function.
	args := HeaderFunctionArgs{
		PageNum:    pageNum,
		TotalPages: totalPages,
		PageWidth:  page.Width(),
		PageHeight: page.Height(),
		Block:      block,
	}
	c.headerFunc(args)

	// Convert block drawables to text operations.
	return c.convertBlockToTextOps(block, page.margins.Left, page.Height()-page.margins.Top)
}

// renderFooter renders footer content for a page and returns text operations.
func (c *Creator) renderFooter(page *Page, pageNum, totalPages int) []TextOperation {
	// Create footer block.
	footerWidth := page.Width() - page.margins.Left - page.margins.Right
	block := NewBlock(footerWidth, c.footerHeight)

	// Call the footer function.
	args := FooterFunctionArgs{
		PageNum:    pageNum,
		TotalPages: totalPages,
		PageWidth:  page.Width(),
		PageHeight: page.Height(),
		Block:      block,
	}
	c.footerFunc(args)

	// Convert block drawables to text operations.
	// Footer is positioned at bottom margin.
	return c.convertBlockToTextOps(block, page.margins.Left, page.margins.Bottom+c.footerHeight)
}

// convertBlockToTextOps converts block drawables to text operations.
func (c *Creator) convertBlockToTextOps(block *Block, offsetX, offsetY float64) []TextOperation {
	drawables := block.GetDrawables()
	ops := make([]TextOperation, 0, len(drawables))

	for _, dp := range drawables {
		// Render drawable to get text operations.
		blockOps := c.renderDrawableToTextOps(dp, block, offsetX, offsetY)
		ops = append(ops, blockOps...)
	}

	return ops
}

// renderDrawableToTextOps renders a drawable positioned in a block to text operations.
func (c *Creator) renderDrawableToTextOps(dp DrawablePosition, block *Block, offsetX, offsetY float64) []TextOperation {
	// Create a temporary page-like context for the drawable.
	ctx := block.GetLayoutContext()
	ctx.CursorX = dp.X
	ctx.CursorY = dp.Y

	// For paragraphs, we can extract the text operations directly.
	if para, ok := dp.Drawable.(*Paragraph); ok {
		return c.paragraphToTextOps(para, ctx, offsetX, offsetY)
	}

	// For other drawables, return empty (they may need graphics ops).
	return nil
}

// paragraphToTextOps converts a paragraph to text operations at the given offset.
func (c *Creator) paragraphToTextOps(p *Paragraph, ctx *LayoutContext, offsetX, offsetY float64) []TextOperation {
	lines := p.WrapTextLines(ctx.AvailableWidth())
	lineHeight := p.FontSize() * p.LineSpacing()

	ops := make([]TextOperation, 0, len(lines))
	for i, line := range lines {
		x := calculateParaLineX(p, ctx, line) + offsetX
		// PDF Y coordinate: offsetY is the top of the block, we go down.
		y := offsetY - ctx.CursorY - float64(i+1)*lineHeight

		ops = append(ops, TextOperation{
			Text:  line,
			X:     x,
			Y:     y,
			Font:  p.Font(),
			Size:  p.FontSize(),
			Color: p.Color(),
		})
	}

	return ops
}

// calculateParaLineX calculates the X position for a paragraph line based on alignment.
func calculateParaLineX(p *Paragraph, ctx *LayoutContext, line string) float64 {
	// Use the paragraph's internal method logic.
	switch p.Alignment() {
	case AlignCenter:
		lineWidth := measureLineWidth(p, line)
		return (ctx.AvailableWidth() - lineWidth) / 2
	case AlignRight:
		lineWidth := measureLineWidth(p, line)
		return ctx.AvailableWidth() - lineWidth
	default:
		return 0
	}
}

// measureLineWidth measures the width of a line of text.
func measureLineWidth(p *Paragraph, line string) float64 {
	// Import fonts to measure.
	return fonts.MeasureString(string(p.Font()), line, p.FontSize())
}

// convertTextOps converts creator text operations to writer text operations.
func convertTextOps(ops []TextOperation) []writer.TextOp {
	textOps := make([]writer.TextOp, 0, len(ops))
	for _, op := range ops {
		textOp := writer.TextOp{
			Text:  op.Text,
			X:     op.X,
			Y:     op.Y,
			Font:  string(op.Font),
			Size:  op.Size,
			Color: writer.RGB{R: op.Color.R, G: op.Color.G, B: op.Color.B},
		}

		// Handle custom embedded font.
		if op.CustomFont != nil {
			textOp.CustomFont = &writer.EmbeddedFont{
				TTF:    op.CustomFont.GetTTF(),
				Subset: op.CustomFont.GetSubset(),
				ID:     op.CustomFont.ID(),
			}
			textOp.Font = "" // Clear standard font when using custom.
		}

		// Convert CMYK color if present (takes precedence over RGB)
		if op.ColorCMYK != nil {
			textOp.ColorCMYK = &writer.CMYK{
				C: op.ColorCMYK.C,
				M: op.ColorCMYK.M,
				Y: op.ColorCMYK.Y,
				K: op.ColorCMYK.K,
			}
		}

		textOps = append(textOps, textOp)
	}
	return textOps
}

// convertGraphicsOps converts creator graphics operations to writer graphics operations.
func convertGraphicsOps(ops []GraphicsOperation) []writer.GraphicsOp {
	graphicsOps := make([]writer.GraphicsOp, 0, len(ops))
	for _, op := range ops {
		gop := writer.GraphicsOp{
			Type:   int(op.Type),
			X:      op.X,
			Y:      op.Y,
			X2:     op.X2,
			Y2:     op.Y2,
			Width:  op.Width,
			Height: op.Height,
			Radius: op.Radius,
			RX:     op.RX,
			RY:     op.RY,
		}

		// Convert vertices (polygon/polyline)
		if len(op.Vertices) > 0 {
			gop.Vertices = make([]writer.Point, len(op.Vertices))
			for i, v := range op.Vertices {
				gop.Vertices[i] = writer.Point{X: v.X, Y: v.Y}
			}
		}

		// Convert bezier segments
		if len(op.BezierSegs) > 0 {
			gop.BezierSegs = make([]writer.BezierSegment, len(op.BezierSegs))
			for i, seg := range op.BezierSegs {
				gop.BezierSegs[i] = writer.BezierSegment{
					Start: writer.Point{X: seg.Start.X, Y: seg.Start.Y},
					C1:    writer.Point{X: seg.C1.X, Y: seg.C1.Y},
					C2:    writer.Point{X: seg.C2.X, Y: seg.C2.Y},
					End:   writer.Point{X: seg.End.X, Y: seg.End.Y},
				}
			}
		}

		// Convert TextBlock fields
		if op.Type == GraphicsOpTextBlock && op.TextFont != nil {
			gop.Text = op.Text
			gop.TextFont = &writer.EmbeddedFont{
				TTF:    op.TextFont.GetTTF(),
				Subset: op.TextFont.GetSubset(),
				ID:     op.TextFont.ID(),
			}
			gop.TextSize = op.TextSize
			if op.TextColor != nil {
				gop.TextColorR = op.TextColor.R
				gop.TextColorG = op.TextColor.G
				gop.TextColorB = op.TextColor.B
			}
		}

		convertGraphicsOptions(&gop, &op)
		graphicsOps = append(graphicsOps, gop)
	}
	return graphicsOps
}

// convertGraphicsOptions converts creator graphics options to writer options.
func convertGraphicsOptions(gop *writer.GraphicsOp, op *GraphicsOperation) {
	// Line options
	if op.LineOpts != nil {
		gop.StrokeColor = &writer.RGB{R: op.LineOpts.Color.R, G: op.LineOpts.Color.G, B: op.LineOpts.Color.B}
		if op.LineOpts.ColorCMYK != nil {
			gop.StrokeColorCMYK = &writer.CMYK{C: op.LineOpts.ColorCMYK.C, M: op.LineOpts.ColorCMYK.M, Y: op.LineOpts.ColorCMYK.Y, K: op.LineOpts.ColorCMYK.K}
		}
		gop.StrokeWidth = op.LineOpts.Width
		gop.Dashed = op.LineOpts.Dashed
		gop.DashArray = op.LineOpts.DashArray
		gop.DashPhase = op.LineOpts.DashPhase
	}

	// Rectangle options
	if op.RectOpts != nil {
		convertRectOptions(gop, op.RectOpts)
	}

	// Circle options
	if op.CircleOpts != nil {
		convertCircleOptions(gop, op.CircleOpts)
	}

	// Polygon options
	if op.PolygonOpts != nil {
		convertPolygonOptions(gop, op.PolygonOpts)
	}

	// Polyline options
	if op.PolylineOpts != nil {
		convertPolylineOptions(gop, op.PolylineOpts)
	}

	// Ellipse options
	if op.EllipseOpts != nil {
		convertEllipseOptions(gop, op.EllipseOpts)
	}

	// Bezier options
	if op.BezierOpts != nil {
		convertBezierOptions(gop, op.BezierOpts)
	}
}

// convertRectOptions converts rectangle options.
func convertRectOptions(gop *writer.GraphicsOp, opts *RectOptions) {
	if opts.StrokeColor != nil {
		gop.StrokeColor = &writer.RGB{R: opts.StrokeColor.R, G: opts.StrokeColor.G, B: opts.StrokeColor.B}
	}
	if opts.StrokeColorCMYK != nil {
		gop.StrokeColorCMYK = &writer.CMYK{C: opts.StrokeColorCMYK.C, M: opts.StrokeColorCMYK.M, Y: opts.StrokeColorCMYK.Y, K: opts.StrokeColorCMYK.K}
	}
	if opts.FillColor != nil {
		gop.FillColor = &writer.RGB{R: opts.FillColor.R, G: opts.FillColor.G, B: opts.FillColor.B}
	}
	if opts.FillColorCMYK != nil {
		gop.FillColorCMYK = &writer.CMYK{C: opts.FillColorCMYK.C, M: opts.FillColorCMYK.M, Y: opts.FillColorCMYK.Y, K: opts.FillColorCMYK.K}
	}
	if opts.FillGradient != nil {
		gop.FillGradient = convertGradient(opts.FillGradient)
	}
	gop.StrokeWidth = opts.StrokeWidth
	gop.Dashed = opts.Dashed
	gop.DashArray = opts.DashArray
	gop.DashPhase = opts.DashPhase
}

// convertCircleOptions converts circle options.
func convertCircleOptions(gop *writer.GraphicsOp, opts *CircleOptions) {
	if opts.StrokeColor != nil {
		gop.StrokeColor = &writer.RGB{R: opts.StrokeColor.R, G: opts.StrokeColor.G, B: opts.StrokeColor.B}
	}
	if opts.StrokeColorCMYK != nil {
		gop.StrokeColorCMYK = &writer.CMYK{C: opts.StrokeColorCMYK.C, M: opts.StrokeColorCMYK.M, Y: opts.StrokeColorCMYK.Y, K: opts.StrokeColorCMYK.K}
	}
	if opts.FillColor != nil {
		gop.FillColor = &writer.RGB{R: opts.FillColor.R, G: opts.FillColor.G, B: opts.FillColor.B}
	}
	if opts.FillColorCMYK != nil {
		gop.FillColorCMYK = &writer.CMYK{C: opts.FillColorCMYK.C, M: opts.FillColorCMYK.M, Y: opts.FillColorCMYK.Y, K: opts.FillColorCMYK.K}
	}
	if opts.FillGradient != nil {
		gop.FillGradient = convertGradient(opts.FillGradient)
	}
	gop.StrokeWidth = opts.StrokeWidth
}

// convertGradient converts a creator gradient to writer gradient.
func convertGradient(g *Gradient) *writer.GradientOp {
	if g == nil {
		return nil
	}

	grad := &writer.GradientOp{
		Type:        writer.GradientType(g.Type),
		X1:          g.X1,
		Y1:          g.Y1,
		X2:          g.X2,
		Y2:          g.Y2,
		X0:          g.X0,
		Y0:          g.Y0,
		R0:          g.R0,
		R1:          g.R1,
		ExtendStart: g.ExtendStart,
		ExtendEnd:   g.ExtendEnd,
		ColorStops:  make([]writer.ColorStopOp, len(g.ColorStops)),
	}

	for i, stop := range g.ColorStops {
		grad.ColorStops[i] = writer.ColorStopOp{
			Position: stop.Position,
			Color:    writer.RGB{R: stop.Color.R, G: stop.Color.G, B: stop.Color.B},
		}
	}

	return grad
}

// convertPolygonOptions converts polygon options.
func convertPolygonOptions(gop *writer.GraphicsOp, opts *PolygonOptions) {
	if opts.StrokeColor != nil {
		gop.StrokeColor = &writer.RGB{R: opts.StrokeColor.R, G: opts.StrokeColor.G, B: opts.StrokeColor.B}
	}
	if opts.StrokeColorCMYK != nil {
		gop.StrokeColorCMYK = &writer.CMYK{C: opts.StrokeColorCMYK.C, M: opts.StrokeColorCMYK.M, Y: opts.StrokeColorCMYK.Y, K: opts.StrokeColorCMYK.K}
	}
	if opts.FillColor != nil {
		gop.FillColor = &writer.RGB{R: opts.FillColor.R, G: opts.FillColor.G, B: opts.FillColor.B}
	}
	if opts.FillColorCMYK != nil {
		gop.FillColorCMYK = &writer.CMYK{C: opts.FillColorCMYK.C, M: opts.FillColorCMYK.M, Y: opts.FillColorCMYK.Y, K: opts.FillColorCMYK.K}
	}
	if opts.FillGradient != nil {
		gop.FillGradient = convertGradient(opts.FillGradient)
	}
	gop.StrokeWidth = opts.StrokeWidth
	gop.Dashed = opts.Dashed
	gop.DashArray = opts.DashArray
	gop.DashPhase = opts.DashPhase
}

// convertPolylineOptions converts polyline options.
func convertPolylineOptions(gop *writer.GraphicsOp, opts *PolylineOptions) {
	gop.StrokeColor = &writer.RGB{R: opts.Color.R, G: opts.Color.G, B: opts.Color.B}
	if opts.ColorCMYK != nil {
		gop.StrokeColorCMYK = &writer.CMYK{C: opts.ColorCMYK.C, M: opts.ColorCMYK.M, Y: opts.ColorCMYK.Y, K: opts.ColorCMYK.K}
	}
	gop.StrokeWidth = opts.Width
	gop.Dashed = opts.Dashed
	gop.DashArray = opts.DashArray
	gop.DashPhase = opts.DashPhase
}

// convertEllipseOptions converts ellipse options.
func convertEllipseOptions(gop *writer.GraphicsOp, opts *EllipseOptions) {
	if opts.StrokeColor != nil {
		gop.StrokeColor = &writer.RGB{R: opts.StrokeColor.R, G: opts.StrokeColor.G, B: opts.StrokeColor.B}
	}
	if opts.StrokeColorCMYK != nil {
		gop.StrokeColorCMYK = &writer.CMYK{C: opts.StrokeColorCMYK.C, M: opts.StrokeColorCMYK.M, Y: opts.StrokeColorCMYK.Y, K: opts.StrokeColorCMYK.K}
	}
	if opts.FillColor != nil {
		gop.FillColor = &writer.RGB{R: opts.FillColor.R, G: opts.FillColor.G, B: opts.FillColor.B}
	}
	if opts.FillColorCMYK != nil {
		gop.FillColorCMYK = &writer.CMYK{C: opts.FillColorCMYK.C, M: opts.FillColorCMYK.M, Y: opts.FillColorCMYK.Y, K: opts.FillColorCMYK.K}
	}
	if opts.FillGradient != nil {
		gop.FillGradient = convertGradient(opts.FillGradient)
	}
	gop.StrokeWidth = opts.StrokeWidth
}

// convertBezierOptions converts bezier options.
func convertBezierOptions(gop *writer.GraphicsOp, opts *BezierOptions) {
	gop.StrokeColor = &writer.RGB{R: opts.Color.R, G: opts.Color.G, B: opts.Color.B}
	if opts.ColorCMYK != nil {
		gop.StrokeColorCMYK = &writer.CMYK{C: opts.ColorCMYK.C, M: opts.ColorCMYK.M, Y: opts.ColorCMYK.Y, K: opts.ColorCMYK.K}
	}
	gop.StrokeWidth = opts.Width
	gop.Dashed = opts.Dashed
	gop.DashArray = opts.DashArray
	gop.DashPhase = opts.DashPhase
	gop.Closed = opts.Closed
	if opts.FillColor != nil {
		gop.FillColor = &writer.RGB{R: opts.FillColor.R, G: opts.FillColor.G, B: opts.FillColor.B}
	}
	if opts.FillGradient != nil {
		gop.FillGradient = convertGradient(opts.FillGradient)
	}
}

// renderTOCAndChapters renders the Table of Contents and all chapters.
//
// This is called automatically before writing the PDF.
// It performs a two-pass rendering:
// 1. First pass: Render all chapters and record page indices
// 2. Second pass: Render TOC with correct page numbers
func (c *Creator) renderTOCAndChapters() error {
	// Nothing to do if no chapters
	if len(c.chapters) == 0 {
		return nil
	}

	// First pass: Render all chapters and record page indices
	chapterPages := make([]*Page, 0)
	for _, ch := range c.chapters {
		pages, err := c.renderChapter(ch)
		if err != nil {
			return fmt.Errorf("failed to render chapter: %w", err)
		}
		chapterPages = append(chapterPages, pages...)
	}

	// Second pass: Render TOC if enabled
	if c.tocEnabled {
		tocPages, err := c.renderTOC()
		if err != nil {
			return fmt.Errorf("failed to render TOC: %w", err)
		}

		// Insert TOC pages at the beginning
		// Update chapter page indices to account for TOC pages
		tocPageCount := len(tocPages)
		for _, ch := range c.chapters {
			c.updateChapterPageIndices(ch, tocPageCount)
		}

		// Prepend TOC pages to the pages list
		c.pages = append(tocPages, c.pages...)
	}

	return nil
}

// renderChapter renders a chapter and all its sub-chapters.
func (c *Creator) renderChapter(ch *Chapter) ([]*Page, error) {
	// Create new page for chapter
	page, err := c.NewPage()
	if err != nil {
		return nil, fmt.Errorf("failed to create page for chapter: %w", err)
	}

	// Record page index for this chapter
	ch.setPageIndex(len(c.pages) - 1)

	// Get layout context
	ctx := page.GetLayoutContext()

	// Draw chapter content
	if err := ch.Draw(ctx, page); err != nil {
		return nil, fmt.Errorf("failed to draw chapter: %w", err)
	}

	// Return the page
	return []*Page{page}, nil
}

// renderTOC renders the Table of Contents.
func (c *Creator) renderTOC() ([]*Page, error) {
	// Set chapters in TOC
	c.toc.setChapters(c.chapters)

	// Create new page for TOC
	page, err := c.NewPage()
	if err != nil {
		return nil, fmt.Errorf("failed to create page for TOC: %w", err)
	}

	// Get layout context
	ctx := page.GetLayoutContext()

	// Draw TOC
	if err := c.toc.Draw(ctx, page); err != nil {
		return nil, fmt.Errorf("failed to draw TOC: %w", err)
	}

	return []*Page{page}, nil
}

// updateChapterPageIndices updates page indices for chapter and sub-chapters.
func (c *Creator) updateChapterPageIndices(ch *Chapter, offset int) {
	if ch.PageIndex() >= 0 {
		ch.setPageIndex(ch.PageIndex() + offset)
	}
	for _, sub := range ch.SubChapters() {
		c.updateChapterPageIndices(sub, offset)
	}
}

// Document returns the underlying domain document.
//
// This is provided for advanced use cases where you need direct access
// to the domain model. Most users should use the Creator API instead.
//
// Example:
//
//	doc := c.Document()
//	// Direct domain operations...
func (c *Creator) Document() *document.Document {
	return c.doc
}

// Errors.
var (
	// ErrInvalidMargins is returned when margins are negative.
	ErrInvalidMargins = errors.New("margins must be non-negative")

	// ErrWriterNotImplemented is returned when PDF writer is not yet implemented.
	ErrWriterNotImplemented = errors.New("PDF writer not yet implemented (Phase 3 TODO)")
)
