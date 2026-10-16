package builder

import (
	"github.com/coregx/gxpdf/layout"
)

// Container is the universal content receiver. It collects layout elements that
// will be paginated and rendered. Container is used for page header, footer,
// and body content; it is also embedded in ColBuilder so that column cells
// support the same rich set of content methods.
//
// All methods on Container accumulate layout.Element values into an internal
// slice. The slice is consumed by the Builder when constructing PageDef objects
// for the paginator.
type Container struct {
	// b is a back-reference to the owning Builder, used to access font
	// resolver and default style.
	b *Builder
	// elements is the ordered list of content elements added to this container.
	elements []layout.Element
}

// newContainer creates a Container attached to the given Builder.
func newContainer(b *Builder) *Container {
	return &Container{b: b}
}

// Text adds a single-style text paragraph to the container.
//
// Example:
//
//	c.Text("Hello World")
//	c.Text("Important!", builder.Bold(), builder.FontSize(18), builder.TextColor(builder.Red))
func (c *Container) Text(text string, opts ...TextOption) {
	ds := c.b.defaultStyle()
	style := applyTextOptions(&ds, opts)
	elem := &layout.Text{
		Content: text,
		Style:   style,
		Fonts:   c.b.fontResolver(),
	}
	c.elements = append(c.elements, elem)
}

// RichText adds a mixed-style inline text paragraph to the container. The fn
// callback receives a [RichTextBuilder] which is used to append individual
// spans with their own style overrides via [RichTextBuilder.Span] and
// hyperlinks via [RichTextBuilder.Link].
//
// Optional TextOption values in opts define the base style that all spans
// inherit unless they override it explicitly.
//
// Example:
//
//	c.RichText(func(rt *builder.RichTextBuilder) {
//	    rt.Span("Revenue grew ")
//	    rt.Span("+18% ", builder.Bold(), builder.TextColor(builder.Hex("#1B7B34")))
//	    rt.Span("year-over-year, driven by enterprise contract wins.")
//	}, builder.FontSize(10), builder.LineHeight(1.5), builder.AlignJustify())
func (c *Container) RichText(fn func(*RichTextBuilder), opts ...TextOption) {
	ds := c.b.defaultStyle()
	baseStyle := applyTextOptions(&ds, opts)
	rtb := &RichTextBuilder{
		b:         c.b,
		baseStyle: baseStyle,
	}
	fn(rtb)

	// Extract alignment and line height from the base style so they propagate
	// to the layout element. We read the raw fields; RichText.PlanLayout will
	// apply effective() to each fragment's style independently.
	align := baseStyle.TextAlign
	lineHeight := baseStyle.LineHeight
	if lineHeight <= 0 {
		lineHeight = 1.2 // match layout.DefaultStyle()
	}
	elem := rtb.build(align, lineHeight)
	c.elements = append(c.elements, elem)
}

// PageNumber adds a page number element to the container. The format string
// should contain PageNum and/or TotalPages placeholders for substitution
// during the two-pass page number resolution.
//
// Example:
//
//	c.PageNumber(builder.PageNum + " / " + builder.TotalPages,
//	    builder.AlignRight(), builder.FontSize(8))
func (c *Container) PageNumber(format string, opts ...TextOption) {
	ds := c.b.defaultStyle()
	style := applyTextOptions(&ds, opts)
	elem := &layout.PageNumber{
		Format: format,
		Style:  style,
		Fonts:  c.b.fontResolver(),
	}
	c.elements = append(c.elements, elem)
}

// Row adds a 12-column grid row to the container. The fn callback receives a
// RowBuilder which is used to define columns (each with a span 1-12).
//
// Optional RowOption values configure the row height and background.
//
// Example:
//
//	c.Row(func(r *builder.RowBuilder) {
//	    r.Col(8, func(col *builder.ColBuilder) { col.Text("Left") })
//	    r.Col(4, func(col *builder.ColBuilder) { col.Text("Right") })
//	})
func (c *Container) Row(fn func(*RowBuilder), opts ...RowOption) {
	rb := &RowBuilder{b: c.b}
	fn(rb)
	elem := rb.build(applyRowOptions(opts))
	c.elements = append(c.elements, elem)
}

// AutoRow is a convenience alias for Row with no options (auto height).
func (c *Container) AutoRow(fn func(*RowBuilder)) {
	c.Row(fn)
}

// Image adds an image element to the container. The data parameter must be
// raw JPEG or PNG bytes.
//
// Example:
//
//	c.Image(pngData, builder.FitWidth(layout.Mm(60)))
func (c *Container) Image(data []byte, opts ...ImageOption) {
	cfg := applyImageOptions(opts)
	elem := newImageElement(data, cfg)
	c.elements = append(c.elements, elem)
}

// Line adds a horizontal rule (separator line) to the container.
//
// Example:
//
//	c.Line(builder.LineColor(builder.Navy), builder.LineWidth(1))
func (c *Container) Line(opts ...LineOption) {
	cfg := applyLineOptions(opts)
	elem := newLineElement(cfg)
	c.elements = append(c.elements, elem)
}

// Spacer adds a fixed-height vertical gap to the container.
//
// Example:
//
//	c.Spacer(builder.Mm(10))  // 10mm gap
func (c *Container) Spacer(height Value) {
	elem := newSpacerElement(height.toLayout())
	c.elements = append(c.elements, elem)
}

// PageBreak inserts an explicit page break. Content after this point begins
// on a new page.
func (c *Container) PageBreak() {
	elem := &pageBreakElement{}
	c.elements = append(c.elements, elem)
}

// KeepTogether groups child elements so that they are never split across pages.
// If the group does not fit on the current page, it is pushed to the next page
// as a whole. If it does not fit on a fresh page, it is placed anyway.
//
// Example:
//
//	c.KeepTogether(func(inner *builder.Container) {
//	    inner.Text("Section Title", builder.Bold(), builder.FontSize(16))
//	    inner.Text("This paragraph always follows the title.")
//	})
func (c *Container) KeepTogether(fn func(*Container)) {
	inner := newContainer(c.b)
	fn(inner)
	box := &layout.Box{
		Children: inner.elements,
		Style: layout.Style{
			KeepTogether: true,
		},
	}
	c.elements = append(c.elements, box)
}

// EnsureSpace inserts a sentinel that ensures at least the given vertical space
// remains on the page. If less space is available, a page break is forced.
//
// Example:
//
//	c.EnsureSpace(builder.Mm(50))  // push to next page if < 50mm remain
func (c *Container) EnsureSpace(height Value) {
	elem := newEnsureSpaceElement(height.toLayout())
	c.elements = append(c.elements, elem)
}

// --- Inline element helpers ---

// pageBreakElement is an Element that always returns Nothing, forcing the
// paginator to flush the current page.
type pageBreakElement struct{}

func (e *pageBreakElement) PlanLayout(_ layout.Area) layout.Plan {
	return layout.Plan{Status: layout.Nothing}
}

// spacerElement is a fixed-height vertical spacer.
type spacerElement struct {
	height layout.Value
}

func newSpacerElement(h layout.Value) *spacerElement {
	return &spacerElement{height: h}
}

func (e *spacerElement) PlanLayout(area layout.Area) layout.Plan {
	h := e.height.Resolve(area.Height, 12)
	if h <= 0 {
		h = 0
	}
	if !layout.Fits(h, area.Height) {
		return layout.Plan{Status: layout.Nothing}
	}
	return layout.Plan{
		Status:   layout.Full,
		Consumed: h,
		Blocks:   nil,
	}
}

// ensureSpaceElement forces a page break when remaining height is insufficient.
type ensureSpaceElement struct {
	minHeight layout.Value
}

func newEnsureSpaceElement(h layout.Value) *ensureSpaceElement {
	return &ensureSpaceElement{minHeight: h}
}

func (e *ensureSpaceElement) PlanLayout(area layout.Area) layout.Plan {
	required := e.minHeight.Resolve(area.Width, 12)
	if !layout.Fits(required, area.Height) {
		return layout.Plan{Status: layout.Nothing}
	}
	// Consume zero height — just a guard.
	return layout.Plan{Status: layout.Full, Consumed: 0}
}

// lineElement is a full-width horizontal rule.
type lineElement struct {
	cfg lineConfig
}

func newLineElement(cfg lineConfig) *lineElement {
	return &lineElement{cfg: cfg}
}

func (e *lineElement) PlanLayout(area layout.Area) layout.Plan {
	strokeWidth := e.cfg.width
	if strokeWidth <= 0 {
		strokeWidth = 1.0
	}
	color := layout.Black
	if e.cfg.color != nil {
		color = *e.cfg.color
	}

	capColor := color
	capWidth := area.Width
	capStroke := strokeWidth

	block := layout.Block{
		X:      0,
		Y:      0,
		Width:  area.Width,
		Height: strokeWidth,
		Draw: func(r layout.Renderer) {
			r.DrawLine(0, capStroke/2, capWidth, capStroke/2, capColor, capStroke)
		},
	}
	return layout.Plan{
		Status:   layout.Full,
		Consumed: strokeWidth,
		Blocks:   []layout.Block{block},
	}
}

// imageElement is a placeholder image element. Full image support requires
// integration with creator.Image loading; this stub reserves space and emits
// a DrawImage call using the raw bytes.
//
// TODO(Phase 4): Load image bytes via creator.LoadImageFromBytes and resolve
// actual pixel dimensions for correct aspect-ratio sizing.
type imageElement struct {
	data []byte
	cfg  imageConfig
}

func newImageElement(data []byte, cfg imageConfig) *imageElement {
	return &imageElement{data: data, cfg: cfg}
}

func (e *imageElement) PlanLayout(area layout.Area) layout.Plan {
	// Determine display width.
	displayW := area.Width
	if e.cfg.width != nil {
		displayW = e.cfg.width.Resolve(area.Width, 12)
	}

	// Default height: 50% of display width (approximate square-ish aspect).
	// Phase 4 will decode image headers for accurate aspect ratios.
	displayH := displayW * 0.5
	if e.cfg.height != nil {
		displayH = e.cfg.height.Resolve(area.Height, 12)
		if e.cfg.width == nil {
			// If only height is constrained, match width to height.
			displayW = displayH * 2.0
		}
	}

	if !layout.Fits(displayH, area.Height) {
		return layout.Plan{Status: layout.Nothing}
	}

	captureData := e.data
	captureW := displayW
	captureH := displayH

	block := layout.Block{
		X:      0,
		Y:      0,
		Width:  captureW,
		Height: captureH,
		Draw: func(r layout.Renderer) {
			r.DrawImage(captureData, 0, 0, captureW, captureH)
		},
	}
	return layout.Plan{
		Status:   layout.Full,
		Consumed: displayH,
		Blocks:   []layout.Block{block},
	}
}
