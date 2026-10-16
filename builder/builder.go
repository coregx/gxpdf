// Package builder provides a declarative, enterprise-grade API for generating
// PDF documents with GxPDF.
//
// The builder uses a QuestPDF-inspired lambda composition pattern adapted for Go,
// with a 12-column grid layout system and automatic pagination.
//
// # Quick Start
//
//	doc := builder.NewBuilder(
//	    builder.WithPageSize(builder.A4),
//	    builder.WithMargins(builder.Mm(20), builder.Mm(15), builder.Mm(20), builder.Mm(15)),
//	    builder.WithTitle("My Document"),
//	)
//
//	doc.Page(func(page *builder.PageBuilder) {
//	    page.Header(func(h *builder.Container) {
//	        h.Text("Company Name", builder.Bold(), builder.FontSize(14))
//	        h.Line()
//	    })
//
//	    page.Content(func(c *builder.Container) {
//	        c.Row(func(r *builder.RowBuilder) {
//	            r.Col(8, func(col *builder.ColBuilder) {
//	                col.Text("Hello World", builder.Bold(), builder.FontSize(18))
//	            })
//	            r.Col(4, func(col *builder.ColBuilder) {
//	                col.Text("Right column", builder.AlignRight())
//	            })
//	        })
//	        c.Spacer(builder.Mm(5))
//	        c.Text("This is a paragraph of text.")
//	    })
//
//	    page.Footer(func(f *builder.Container) {
//	        f.Text(builder.PageNum+" / "+builder.TotalPages,
//	            builder.AlignCenter(), builder.FontSize(8))
//	    })
//	})
//
//	pdfBytes, err := doc.Build()
//
// # Architecture
//
// The builder package connects three layers:
//
//   - builder/ — user-facing API (this package)
//   - layout/  — pure computation layout engine (no PDF dependencies)
//   - creator/ — low-level PDF rendering backend
//
// Users only import builder/ — all types (Value, Color, Size) are defined here.
// The layout/ package is an internal computation engine not intended for direct use.
//
// # Error Handling
//
// All errors are accumulated and returned as a joined error from [Builder.Build].
// Layout definitions can be built without checking errors at each step.
package builder

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	builderinternal "github.com/coregx/gxpdf/builder/internal"
	"github.com/coregx/gxpdf/creator"
	"github.com/coregx/gxpdf/layout"
)

// Builder is the document builder entry point. It accumulates page definitions
// and document-level configuration, then generates a PDF on Build().
//
// Builder instances are not safe for concurrent use. Use one Builder per
// goroutine.
type Builder struct {
	cfg   config
	pages []*pageDef
	errs  []error
	// fonts resolves font families for both layout and rendering.
	fonts *builderinternal.FontSet
}

// NewBuilder creates a new Builder with the given document-level options.
// Sensible defaults are applied for any options not provided:
//   - Page size: A4
//   - Margins: 20mm top/bottom, 15mm left/right
//   - Default style: Helvetica 12pt, black text, 1.2 line height
//
// Example:
//
//	b := builder.NewBuilder(
//	    builder.WithPageSize(layout.PageLetter),
//	    builder.WithFont("Inter", interTTF),
//	)
func NewBuilder(opts ...Option) *Builder {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	fonts := builderinternal.NewFontSet(cfg.fonts, cfg.fontFallback)
	errs := cfg.errs
	if err := fonts.CheckFallback(); err != nil {
		errs = append(errs, err)
	}
	return &Builder{cfg: cfg, errs: errs, fonts: fonts}
}

// Page adds a page definition to the document. The fn callback receives a
// PageBuilder for defining header, content, and footer zones.
//
// A single Page definition may produce multiple physical PDF pages if the
// content overflows — the paginator handles splitting automatically.
//
// Example:
//
//	b.Page(func(p *builder.PageBuilder) {
//	    p.Header(func(h *builder.Container) { h.Text("Header") })
//	    p.Content(func(c *builder.Container) { c.Text("Body text") })
//	    p.Footer(func(f *builder.Container) {
//	        f.PageNumber(layout.PageNumberPlaceholder + " / " + layout.TotalPagesPlaceholder)
//	    })
//	})
func (b *Builder) Page(fn func(*PageBuilder)) {
	pb := &PageBuilder{b: b}
	fn(pb)
	b.pages = append(b.pages, &pb.def)
}

// Build runs the full PDF generation pipeline and returns the resulting PDF
// bytes. All accumulated errors (from font loading, invalid options, etc.) are
// returned as a joined error.
//
// The pipeline:
//  1. Check accumulated errors.
//  2. Convert page definitions to layout.PageDef values.
//  3. Run the paginator to produce positioned blocks.
//  4. Walk the block tree with the renderer to emit PDF content via creator.
//  5. Serialize to bytes and return.
func (b *Builder) Build() ([]byte, error) {
	if err := b.accumulatedError(); err != nil {
		return nil, err
	}

	cr := creator.New()
	if err := b.renderTo(cr); err != nil {
		return nil, err
	}

	pdfBytes, err := cr.Bytes()
	if err != nil {
		return nil, fmt.Errorf("serialize PDF: %w", err)
	}
	return pdfBytes, nil
}

// BuildTo writes the generated PDF to the given io.Writer.
// Returns an error if PDF generation or writing fails.
func (b *Builder) BuildTo(w io.Writer) error {
	pdfBytes, err := b.Build()
	if err != nil {
		return err
	}
	_, err = w.Write(pdfBytes)
	return err
}

// BuildToFile writes the generated PDF to the file at path, creating or
// truncating the file as needed.
func (b *Builder) BuildToFile(path string) error {
	pdfBytes, err := b.Build()
	if err != nil {
		return err
	}
	return os.WriteFile(path, pdfBytes, 0644)
}

// --- Internal helpers ---

// defaultStyle returns the document-level default style. Consumers (Container,
// PageBuilder) call this to initialize per-element styles.
func (b *Builder) defaultStyle() layout.Style {
	return b.cfg.defaultStyle
}

// fontResolver returns a layout.FontResolver backed by the registered custom
// fonts and the Standard 14 font metrics, sharing the renderer's FontSet.
func (b *Builder) fontResolver() layout.FontResolver {
	return builderinternal.NewFontBridge(b.fonts)
}

// addError appends an error to the accumulated error list.

// accumulatedError returns all accumulated errors joined into a single error,
// or nil if there are none.
func (b *Builder) accumulatedError() error {
	if len(b.errs) == 0 {
		return nil
	}
	return errors.Join(b.errs...)
}

// layoutPages runs the layout pipeline and returns the positioned pages.
func (b *Builder) layoutPages() []layout.PageLayout {
	layoutPages := make([]*layout.PageDef, 0, len(b.pages))
	for _, pd := range b.pages {
		layoutPages = append(layoutPages, pd.toLayoutPageDef(b.cfg.pageSize, b.cfg.margins))
	}

	paginator := &layout.Paginator{
		Fonts: b.fontResolver(),
	}
	return paginator.Paginate(layoutPages)
}

// renderTo runs the layout pipeline and emits PDF content into cr.
func (b *Builder) renderTo(cr *creator.Creator) error {
	// Families are recorded as layout and rendering resolve them.
	b.fonts.Reset()
	pageLayouts := b.layoutPages()

	// Render page layouts via the PDF renderer.
	renderer := builderinternal.NewPDFRenderer(cr, b.fonts)
	meta := builderinternal.ExportedMetadata(b.cfg.meta.title, b.cfg.meta.author)
	if err := renderer.RenderDocument(pageLayouts, meta); err != nil {
		return fmt.Errorf("render: %w", err)
	}

	// Text in a family that is neither registered nor Standard 14 would have
	// been drawn in a substitute font; without WithFontFallback that is an
	// error, not a silently different document.
	return b.fonts.Err()
}

// Bytes is an alias for Build() for callers that prefer the creator-style API.
func (b *Builder) Bytes() ([]byte, error) {
	return b.Build()
}

// WriteToFile is an alias for BuildToFile() for callers that prefer the
// creator-style API.
func (b *Builder) WriteToFile(path string) error {
	return b.BuildToFile(path)
}

// WriteTo implements io.WriterTo.
func (b *Builder) WriteTo(w io.Writer) (int64, error) {
	pdfBytes, err := b.Build()
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(w, bytes.NewReader(pdfBytes))
	return n, err
}
