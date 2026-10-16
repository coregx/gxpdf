package layout

import (
	"fmt"
	"strings"
)

// PageNumberPlaceholder is injected into text content where the current page
// number should appear. It is replaced in a second pass after all pages are
// known.
const PageNumberPlaceholder = "\x00PAGE\x00"

// TotalPagesPlaceholder is injected into text content where the total page
// count should appear.
const TotalPagesPlaceholder = "\x00TOTAL\x00"

// pageNumberTag marks the blocks of PageNumber elements; ResolvePageNumbers
// rewrites them once the page count is known.
const pageNumberTag = "__pagenumber__"

// PageDef describes a page template: its physical size, margins, and the
// content elements to be rendered on each page of the document.
type PageDef struct {
	// Size is the physical page dimensions in PDF points.
	Size Size
	// Margins is the spacing between the page edge and the content area.
	Margins Edges
	// Header contains elements rendered at the top of every page.
	// The header is laid out with unlimited height to measure it, then
	// subtracted from the body area.
	Header []Element
	// Footer contains elements rendered at the bottom of every page.
	Footer []Element
	// Content contains the main body elements to paginate.
	Content []Element
}

// PageLayout holds all positioned blocks for a single rendered page,
// ready to be handed to the renderer.
type PageLayout struct {
	// Size is the physical page dimensions in PDF points.
	Size Size
	// Blocks contains all content blocks for this page, with coordinates
	// measured from the top-left corner of the page.
	Blocks []Block
}

// Paginator runs the layout engine across a sequence of PageDefs and
// produces a slice of PageLayout values — one per physical output page.
// Content that does not fit on a page overflows automatically to the next page.
type Paginator struct {
	// Fonts is the font resolver used by all layout operations.
	// If nil, MockFontResolver is used (suitable for tests).
	Fonts FontResolver
}

// Paginate processes the given page definitions and returns one PageLayout
// per physical output page. The algorithm:
//  1. For each PageDef, resolve margins.
//  2. Layout header and footer at unlimited height to measure their heights.
//  3. Compute bodyHeight = pageHeight - margins - headerH - footerH.
//  4. Iterate content elements, calling PlanLayout on each.
//  5. Full → accumulate blocks, advance cursor.
//  6. Partial → flush page, push Overflow to next page.
//  7. Nothing at page top → force layout with unlimited height (oversized element).
//  8. Nothing mid-page → flush page, retry on fresh page.
//  9. After all pages are generated, resolve page number placeholders.
func (p *Paginator) Paginate(pages []*PageDef) []PageLayout {
	var result []PageLayout

	for _, def := range pages {
		pageSize := def.Size
		if pageSize.Width <= 0 || pageSize.Height <= 0 {
			pageSize = PageA4
		}

		resolvedMargins := def.Margins.Resolve(pageSize.Width, pageSize.Height, 12)

		contentWidth := pageSize.Width - resolvedMargins.Horizontal()
		contentHeight := pageSize.Height - resolvedMargins.Vertical()
		if contentWidth < 0 {
			contentWidth = 0
		}
		if contentHeight < 0 {
			contentHeight = 0
		}

		// Measure header and footer once per page definition.
		headerBlocks, headerHeight := measureSection(def.Header, contentWidth)
		footerBlocks, footerHeight := measureSection(def.Footer, contentWidth)

		bodyHeight := contentHeight - headerHeight - footerHeight
		if bodyHeight < 0 {
			bodyHeight = 0
		}

		// State for paginating the content elements.
		remaining := make([]Element, len(def.Content))
		copy(remaining, def.Content)

		for len(remaining) > 0 {
			pageBlocks, nextRemaining := buildPageBlocks(remaining, contentWidth, bodyHeight)

			allBlocks := assemblePageLayout(
				pageBlocks, headerBlocks, footerBlocks,
				resolvedMargins, headerHeight, footerHeight, contentHeight,
			)

			result = append(result, PageLayout{
				Size:   pageSize,
				Blocks: allBlocks,
			})

			remaining = nextRemaining
		}
	}

	// Two-pass page number resolution.
	ResolvePageNumbers(result)

	return result
}

// buildPageBlocks fills one page worth of body blocks from remaining elements.
// It returns the blocks placed on this page and the elements carried over to
// the next page.
func buildPageBlocks(remaining []Element, contentWidth, bodyHeight float64) (pageBlocks []Block, nextRemaining []Element) {
	cursorY := 0.0

	for len(remaining) > 0 {
		elem := remaining[0]
		availH := bodyHeight - cursorY

		plan := elem.PlanLayout(Area{Width: contentWidth, Height: availH, AtTop: cursorY == 0})

		switch plan.Status {
		case Full:
			placed := cloneBlocks(plan.Blocks)
			offsetBlocks(placed, 0, cursorY)
			pageBlocks = append(pageBlocks, placed...)
			cursorY += plan.Consumed
			remaining = remaining[1:]

		case Partial:
			placed := cloneBlocks(plan.Blocks)
			offsetBlocks(placed, 0, cursorY)
			pageBlocks = append(pageBlocks, placed...)
			// Overflow goes to the next page; include remaining elements after it.
			nextRemaining = make([]Element, 0, 1+len(remaining[1:]))
			nextRemaining = append(nextRemaining, plan.Overflow)
			nextRemaining = append(nextRemaining, remaining[1:]...)
			return pageBlocks, nextRemaining

		case Nothing:
			pageBlocks, nextRemaining, remaining = handleNothingStatus(
				elem, remaining, pageBlocks, cursorY, contentWidth,
			)
			if nextRemaining != nil {
				return pageBlocks, nextRemaining
			}
			// remaining was updated in-place; re-read cursorY from what was consumed.
			// Since handleNothingStatus may have advanced cursorY, we recompute it.
			cursorY = recomputeCursorY(pageBlocks)
		}
	}

	return pageBlocks, nil
}

// handleNothingStatus processes a Nothing plan result. It either forces
// placement at page top or flushes the page for mid-page nothing.
// Returns updated pageBlocks, non-nil nextRemaining if page should flush,
// and updated remaining slice.
func handleNothingStatus(
	elem Element,
	remaining []Element,
	pageBlocks []Block,
	cursorY float64,
	contentWidth float64,
) (outBlocks []Block, nextRemaining []Element, outRemaining []Element) {
	if cursorY == 0 {
		// Nothing at page top — force with unlimited height.
		forcedPlan := elem.PlanLayout(Area{Width: contentWidth, Height: 1e9, AtTop: true})
		placed := cloneBlocks(forcedPlan.Blocks)
		offsetBlocks(placed, 0, cursorY)
		pageBlocks = append(pageBlocks, placed...)
		remaining = remaining[1:]
		if forcedPlan.Overflow != nil {
			next := make([]Element, 0, 1+len(remaining))
			next = append(next, forcedPlan.Overflow)
			next = append(next, remaining...)
			return pageBlocks, next, nil
		}
		return pageBlocks, nil, remaining
	}
	// Nothing mid-page — flush page, retry this element on next page.
	return pageBlocks, remaining, nil
}

// recomputeCursorY returns the Y position after the last placed block.
// It is used after handleNothingStatus to restore the cursor correctly.
func recomputeCursorY(blocks []Block) float64 {
	if len(blocks) == 0 {
		return 0
	}
	last := blocks[len(blocks)-1]
	return last.Y + last.Height
}

// assemblePageLayout composes header, body, and footer blocks into a single
// slice offset by margins, ready for the PageLayout.
func assemblePageLayout(
	pageBlocks, headerBlocks, footerBlocks []Block,
	margins ResolvedEdges,
	headerHeight, footerHeight, contentHeight float64,
) []Block {
	mx := margins.Left
	my := margins.Top

	var allBlocks []Block

	// Header at top of content area.
	if len(headerBlocks) > 0 {
		hb := cloneBlocks(headerBlocks)
		offsetBlocks(hb, mx, my)
		allBlocks = append(allBlocks, hb...)
	}

	// Body below header.
	bodyOffY := my + headerHeight
	if len(pageBlocks) > 0 {
		bb := cloneBlocks(pageBlocks)
		offsetBlocks(bb, mx, bodyOffY)
		allBlocks = append(allBlocks, bb...)
	}

	// Footer at bottom of content area.
	if len(footerBlocks) > 0 {
		fb := cloneBlocks(footerBlocks)
		footerY := my + contentHeight - footerHeight
		offsetBlocks(fb, mx, footerY)
		allBlocks = append(allBlocks, fb...)
	}

	return allBlocks
}

// measureSection lays out a slice of elements at unlimited height and returns
// their blocks and total consumed height. This is used to measure header and
// footer sections.
func measureSection(elements []Element, width float64) ([]Block, float64) {
	if len(elements) == 0 {
		return nil, 0
	}
	container := &Box{
		Children: elements,
	}
	plan := container.PlanLayout(Area{Width: width, Height: 1e9})
	return plan.Blocks, plan.Consumed
}

// ResolvePageNumbers performs a second pass over all pages, replacing
// PageNumberPlaceholder and TotalPagesPlaceholder strings inside Draw
// closures with the actual page number and total page count.
//
// Because Draw closures are opaque functions, page number injection is
// implemented by wrapping Draw closures in Text blocks that contain the
// placeholder strings. The paginator substitutes those strings when the
// Text element is created with PageNumberPlaceholder or TotalPagesPlaceholder
// as content.
//
// This function walks all blocks recursively and replaces placeholder content
// in any TextBlock created by the text layout.
func ResolvePageNumbers(pages []PageLayout) {
	total := len(pages)
	for i := range pages {
		resolveBlockPageNumbers(pages[i].Blocks, i+1, total)
	}
}

// resolveBlockPageNumbers recursively finds page-number blocks and rebuilds
// their Draw closures with the resolved text.
//
// Children slices may be shared between pages: page header and footer blocks
// are laid out once per PageDef and copied shallowly to every page. A
// Children slice is therefore copied before anything inside it changes, or
// the first page would resolve the shared blocks for all pages.
func resolveBlockPageNumbers(blocks []Block, pageNum, totalPages int) {
	for i := range blocks {
		if blocks[i].Tag == pageNumberTag && blocks[i].AltText != "" {
			resolved := blocks[i].AltText
			resolved = strings.ReplaceAll(resolved, PageNumberPlaceholder, fmt.Sprintf("%d", pageNum))
			resolved = strings.ReplaceAll(resolved, TotalPagesPlaceholder, fmt.Sprintf("%d", totalPages))
			blocks[i].AltText = resolved

			// Replace the Draw closure with one that uses the resolved text.
			// The old closure captured styling info — we rebuild it here.
			if blocks[i].drawData != nil {
				dd := blocks[i].drawData
				// Draw at (0,0) — the block's X/Y handles positioning.
				// Update block.X to the new alignment offset for the resolved text.
				w := dd.fonts.MeasureString(dd.font, resolved, dd.size)
				rx, _ := computeTextX(dd.align, w, dd.areaWidth, resolved, true)
				blocks[i].X = rx
				blocks[i].Width = w
				blocks[i].Draw = func(r Renderer) {
					r.DrawText(resolved, 0, 0, dd.font, dd.size, dd.color, TextDrawOptions{})
				}
			}
		}
		if containsPageNumber(blocks[i].Children) {
			blocks[i].Children = cloneBlocks(blocks[i].Children)
			resolveBlockPageNumbers(blocks[i].Children, pageNum, totalPages)
		}
	}
}

// containsPageNumber reports whether blocks or their descendants include a
// page-number block.
func containsPageNumber(blocks []Block) bool {
	for i := range blocks {
		if blocks[i].Tag == pageNumberTag || containsPageNumber(blocks[i].Children) {
			return true
		}
	}
	return false
}

// PageNumber is a special Element that renders the current page number.
// It uses a two-pass approach: on the first pass it inserts a placeholder
// that the paginator replaces with the actual page number after all pages
// are known.
type PageNumber struct {
	// Format is the format string. Use PageNumberPlaceholder and
	// TotalPagesPlaceholder as substitution markers.
	// Example: PageNumberPlaceholder + " / " + TotalPagesPlaceholder
	Format string
	// Style controls the appearance of the page number text.
	Style Style
	// Fonts is the font resolver.
	Fonts FontResolver
}

// pageNumberDrawData holds the styling data needed to rebuild a page number
// Draw closure after ResolvePageNumbers replaces placeholder text.
type pageNumberDrawData struct {
	fonts     FontResolver
	font      FontRef
	size      float64
	color     Color
	align     Align
	areaWidth float64
}

// PlanLayout implements Element. It creates blocks tagged with "__pagenumber__"
// whose Draw closures are rebuilt by ResolvePageNumbers after pagination.
func (pn *PageNumber) PlanLayout(area Area) Plan {
	s := pn.Style.effective()
	fontSize := s.FontSize
	lineSpacing := fontSize * s.LineHeight

	resolver := pn.Fonts
	if resolver == nil {
		resolver = &MockFontResolver{}
	}
	font := s.Font

	lineWidth := resolver.MeasureString(font, pn.Format, fontSize)
	xPos, _ := computeTextX(s.TextAlign, lineWidth, area.Width, pn.Format, true)

	dd := &pageNumberDrawData{
		fonts:     resolver,
		font:      font,
		size:      fontSize,
		color:     s.Color,
		align:     s.TextAlign,
		areaWidth: area.Width,
	}

	// Initial Draw uses placeholder text; ResolvePageNumbers replaces it.
	capturedFormat := pn.Format
	block := Block{
		X:        xPos,
		Y:        0,
		Width:    lineWidth,
		Height:   lineSpacing,
		Tag:      pageNumberTag,
		AltText:  pn.Format,
		drawData: dd,
		Draw: func(r Renderer) {
			r.DrawText(capturedFormat, 0, 0, dd.font, dd.size, dd.color, TextDrawOptions{})
		},
	}

	return Plan{
		Status:   Full,
		Consumed: lineSpacing,
		Blocks:   []Block{block},
	}
}
