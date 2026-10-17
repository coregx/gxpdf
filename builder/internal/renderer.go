package internal

import (
	"errors"
	"fmt"
	"strings"

	"github.com/coregx/gxpdf/creator"
	"github.com/coregx/gxpdf/layout"
)

// PDFRenderer walks a slice of layout.PageLayout values and emits PDF content
// by calling creator methods. It bridges the layout coordinate system
// (top-down, Y=0 at top) to the PDF coordinate system (bottom-up, Y=0 at bottom).
//
// Coordinate conversion:
//
//	pdfY = pageHeight - layoutY
//
// The renderer is stateless across pages; each page creates a new creator.Page.
type PDFRenderer struct {
	cr *creator.Creator
	// fonts resolves font families, shared with the layout's FontBridge.
	fonts *FontSet
	// images holds decoded images, shared with the layout's image elements.
	images *ImageSet
}

// NewPDFRenderer creates a PDFRenderer using the given Creator, resolving
// font families through fonts (nil: Standard 14 only) and image bytes through
// images (nil: a fresh set).
func NewPDFRenderer(cr *creator.Creator, fonts *FontSet, images *ImageSet) *PDFRenderer {
	if fonts == nil {
		fonts = NewFontSet(nil, "")
	}
	if images == nil {
		images = NewImageSet()
	}
	return &PDFRenderer{cr: cr, fonts: fonts, images: images}
}

// RenderDocument walks all PageLayouts, creating one creator.Page per layout
// page and rendering every Block tree onto it. If pages is empty (e.g. no
// content was added), a single blank A4 page is created so that the creator
// validation (which requires at least one page) does not fail.
func (r *PDFRenderer) RenderDocument(pages []layout.PageLayout, meta metadata) error {
	if meta.Title != "" {
		r.cr.SetTitle(meta.Title)
	}
	if meta.Author != "" {
		r.cr.SetAuthor(meta.Author)
	}

	// Ensure at least one page exists.
	if len(pages) == 0 {
		_, err := r.cr.NewPageWithDimensions(595.276, 841.890) // A4
		return err
	}

	var errs []error
	for n, pl := range pages {
		page, err := r.cr.NewPageWithDimensions(pl.Size.Width, pl.Size.Height)
		if err != nil {
			return fmt.Errorf("create page (%.0fx%.0f): %w", pl.Size.Width, pl.Size.Height, err)
		}
		pr := &pageRenderer{
			page:       page,
			pageHeight: pl.Size.Height,
			fonts:      r.fonts,
			images:     r.images,
		}
		for i := range pl.Blocks {
			pr.renderBlock(&pl.Blocks[i])
		}
		for _, err := range pr.errs {
			errs = append(errs, fmt.Errorf("page %d: %w", n+1, err))
		}
	}
	return errors.Join(errs...)
}

// metadata holds document-level metadata for PDF generation.
type metadata struct {
	Title  string
	Author string
}

// ExportedMetadata constructs a metadata value from the builder config.
// This is the only way for the builder package to create an internal metadata
// struct while keeping the type unexported within the package.
func ExportedMetadata(title, author string) metadata {
	return metadata{Title: title, Author: author}
}

// pageRenderer renders one physical page by translating layout coordinates to
// PDF coordinates and calling creator.Page methods.
type pageRenderer struct {
	page       *creator.Page
	pageHeight float64
	fonts      *FontSet
	images     *ImageSet
	// errs collects content that could not be drawn; Build() reports them
	// instead of returning a PDF that silently lacks the content.
	errs []error
}

// renderBlock recursively renders a Block and its children. The baseX and baseY
// parameters are the accumulated absolute offset for nested blocks. Layout
// coordinates are top-left origin (Y increases downward); PDF coordinates are
// bottom-left origin (Y increases upward).
func (pr *pageRenderer) renderBlock(b *layout.Block) {
	// If this block has a Draw closure, invoke it with an adaptor Renderer
	// that translates top-left coordinates to PDF bottom-left coordinates.
	if b.Draw != nil {
		adaptor := &blockAdaptor{
			pr:    pr,
			baseX: b.X,
			baseY: b.Y,
		}
		b.Draw(adaptor)
	}
	// Recurse into children.
	for i := range b.Children {
		child := b.Children[i]
		// Accumulate the parent's offset for the child.
		child.X += b.X
		child.Y += b.Y
		pr.renderBlock(&child)
	}
}

// blockAdaptor implements layout.Renderer for a specific block, translating
// coordinates relative to the block's top-left corner into absolute PDF
// coordinates (bottom-left origin).
type blockAdaptor struct {
	pr    *pageRenderer
	baseX float64
	baseY float64
}

// toPDFCoords converts a layout point (top-down, relative to block origin) to
// an absolute PDF point (bottom-up, absolute page coordinates).
func (a *blockAdaptor) toPDFCoords(x, y float64) (float64, float64) {
	absX := a.baseX + x
	absY := a.pr.pageHeight - (a.baseY + y)
	return absX, absY
}

// DrawText implements layout.Renderer. It renders a text string at the given
// layout position. The Y coordinate is adjusted so that text is placed at the
// baseline, using the ascender to offset from the top of the line box.
func (a *blockAdaptor) DrawText(text string, x, y float64, font layout.FontRef, size float64, color layout.Color, options layout.TextDrawOptions) {
	if text == "" {
		return
	}

	f := a.pr.fonts.Resolve(font)
	pdfX, pdfYTop := a.toPDFCoords(x, y)
	pdfY := pdfYTop - f.Ascender(size)

	creatorColor := creator.Color{R: color.R, G: color.G, B: color.B}

	// If word spacing is set (justified text), render each word separately
	// with computed X positions since creator doesn't support PDF Tw operator.
	if options.WordSpacing > 0 {
		a.drawJustifiedText(text, pdfX, pdfY, f, size, creatorColor, options.WordSpacing)
		return
	}

	a.addText(text, pdfX, pdfY, f, size, creatorColor)

	if options.Underline {
		a.drawUnderline(text, pdfX, pdfY, f, size, creatorColor)
	}
	if options.Strikethrough {
		a.drawStrikethrough(text, pdfX, pdfY, f, size, creatorColor)
	}
}

// addText draws text in the resolved font.
func (a *blockAdaptor) addText(text string, pdfX, pdfY float64, f Font, size float64, color creator.Color) {
	if f.Custom != nil {
		_ = a.pr.page.AddTextCustomFontColor(text, pdfX, pdfY, f.Custom, size, color)
		return
	}
	_ = a.pr.page.AddTextColor(text, pdfX, pdfY, f.Standard, size, color)
}

// drawJustifiedText renders text word-by-word with extra spacing between words.
func (a *blockAdaptor) drawJustifiedText(text string, pdfX, pdfY float64, f Font, size float64, color creator.Color, wordSpacing float64) {
	words := strings.Fields(text)
	curX := pdfX
	for i, word := range words {
		a.addText(word, curX, pdfY, f, size, color)
		curX += f.MeasureString(word, size)
		if i < len(words)-1 {
			// Add normal space width + extra word spacing
			curX += f.MeasureString(" ", size) + wordSpacing
		}
	}
}

func (a *blockAdaptor) drawUnderline(text string, pdfX, pdfY float64, f Font, size float64, color creator.Color) {
	underlineY := pdfY - size*0.1
	w := f.MeasureString(text, size)
	_ = a.pr.page.DrawLine(pdfX, underlineY, pdfX+w, underlineY, &creator.LineOptions{Color: color, Width: size * 0.05})
}

func (a *blockAdaptor) drawStrikethrough(text string, pdfX, pdfY float64, f Font, size float64, color creator.Color) {
	midY := pdfY + size*0.3
	w := f.MeasureString(text, size)
	_ = a.pr.page.DrawLine(pdfX, midY, pdfX+w, midY, &creator.LineOptions{Color: color, Width: size * 0.05})
}

// DrawRect implements layout.Renderer.
func (a *blockAdaptor) DrawRect(x, y, width, height float64, fill *layout.Color, stroke *layout.Color, strokeWidth float64) {
	pdfX, pdfYTop := a.toPDFCoords(x, y)
	// PDF rect origin is lower-left; layout rect origin is upper-left.
	// pdfY lower-left = pdfYTop - height
	pdfY := pdfYTop - height

	opts := &creator.RectOptions{}
	if fill != nil {
		c := creator.Color{R: fill.R, G: fill.G, B: fill.B}
		opts.FillColor = &c
	}
	if stroke != nil {
		c := creator.Color{R: stroke.R, G: stroke.G, B: stroke.B}
		opts.StrokeColor = &c
		opts.StrokeWidth = strokeWidth
		if opts.StrokeWidth <= 0 {
			opts.StrokeWidth = 1.0
		}
	}
	_ = a.pr.page.DrawRect(pdfX, pdfY, width, height, opts)
}

// DrawLine implements layout.Renderer.
func (a *blockAdaptor) DrawLine(x1, y1, x2, y2 float64, color layout.Color, width float64) {
	pdfX1, pdfY1 := a.toPDFCoords(x1, y1)
	pdfX2, pdfY2 := a.toPDFCoords(x2, y2)
	c := creator.Color{R: color.R, G: color.G, B: color.B}
	w := width
	if w <= 0 {
		w = 1.0
	}
	_ = a.pr.page.DrawLine(pdfX1, pdfY1, pdfX2, pdfY2, &creator.LineOptions{
		Color: c,
		Width: w,
	})
}

// DrawImage implements layout.Renderer. It embeds data (JPEG or PNG) as an
// image XObject scaled to width x height. Decoding goes through the shared
// ImageSet, so the layout's earlier decode of the same bytes is reused and
// repeated placements share one XObject.
func (a *blockAdaptor) DrawImage(data []byte, x, y, width, height float64) {
	img, err := a.pr.images.Load(data)
	if err != nil {
		a.pr.errs = append(a.pr.errs, fmt.Errorf("image: %w", err))
		return
	}
	pdfX, pdfYTop := a.toPDFCoords(x, y)
	if err := a.pr.page.DrawImage(img, pdfX, pdfYTop-height, width, height); err != nil {
		a.pr.errs = append(a.pr.errs, fmt.Errorf("image: %w", err))
	}
}

// PushState implements layout.Renderer. Currently a no-op stub; clip state
// management is handled at the BeginClipRect/EndClip level.
func (a *blockAdaptor) PushState() {
	// Clip state is saved implicitly by BeginClipRect in creator.
}

// PopState implements layout.Renderer.
func (a *blockAdaptor) PopState() {
	_ = a.pr.page.EndClip()
}

// SetClipRect implements layout.Renderer. Sets a rectangular clipping region.
// Layout uses top-left origin; creator.BeginClipRect uses bottom-left origin.
func (a *blockAdaptor) SetClipRect(x, y, width, height float64) {
	pdfX, pdfYTop := a.toPDFCoords(x, y)
	pdfY := pdfYTop - height
	_ = a.pr.page.BeginClipRect(pdfX, pdfY, width, height)
}
