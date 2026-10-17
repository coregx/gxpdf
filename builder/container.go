package builder

import (
	"fmt"
	"math"

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
// raw JPEG or PNG bytes; the image is embedded as an image XObject, once per
// document however often the same bytes are placed (e.g. a logo in a header).
//
// Size: FitWidth and/or FitHeight keep the aspect ratio (with both, the image
// fits inside that box); without options the image takes its intrinsic size,
// 1 pixel = 1 point. It never gets wider than the available width, and an
// image taller than an empty page is scaled down to the page.
//
// Data that is not a decodable JPEG or PNG makes Build return an error.
//
// Example:
//
//	c.Image(pngData, builder.FitWidth(layout.Mm(60)))
func (c *Container) Image(data []byte, opts ...ImageOption) {
	img, err := c.b.images.Load(data)
	if err != nil {
		c.b.errs = append(c.b.errs, fmt.Errorf("image (%d bytes): %w", len(data), err))
		return
	}
	if img.Width() <= 0 || img.Height() <= 0 {
		c.b.errs = append(c.b.errs, fmt.Errorf("image (%d bytes): no pixels (%dx%d)", len(data), img.Width(), img.Height()))
		return
	}
	cfg := applyImageOptions(opts)
	c.elements = append(c.elements, newImageElement(data, img.Width(), img.Height(), cfg))
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

// imageElement places a decoded JPEG or PNG image, keeping its aspect ratio
// (see displaySize). An image never splits across pages.
type imageElement struct {
	data []byte
	cfg  imageConfig
	// natW and natH are the intrinsic size in points (1 pixel = 1 point,
	// 72 DPI), as in gpdf and folio.
	natW, natH float64
}

func newImageElement(data []byte, pixelsW, pixelsH int, cfg imageConfig) *imageElement {
	return &imageElement{data: data, cfg: cfg, natW: float64(pixelsW), natH: float64(pixelsH)}
}

// fixedLength resolves v when it does not depend on the surrounding area
// (pt, mm, cm, in). Percentages, fr, auto and non-positive values report false.
func fixedLength(v *layout.Value) (float64, bool) {
	if v == nil {
		return 0, false
	}
	switch v.Unit {
	case layout.UnitPct, layout.UnitFr, layout.UnitAuto:
		return 0, false
	}
	n := v.Resolve(0, 12)
	return n, n > 0
}

// displaySize returns the drawn size for an area of areaW x areaH points:
//   - FitWidth only: that width, the height from the aspect ratio;
//   - FitHeight only: that height, the width from the aspect ratio;
//   - both: the largest size with the image's aspect ratio inside that box;
//   - neither: the intrinsic size.
//
// Percentages refer to the area. A result wider than the area is scaled down
// to its width, keeping the aspect ratio.
func (e *imageElement) displaySize(areaW, areaH float64) (w, h float64) {
	ratio := e.natH / e.natW
	var boxW, boxH float64
	if e.cfg.width != nil {
		boxW = e.cfg.width.Resolve(areaW, 12)
	}
	if e.cfg.height != nil {
		boxH = e.cfg.height.Resolve(areaH, 12)
	}
	switch {
	case boxW > 0 && boxH > 0:
		w, h = boxW, boxW*ratio
		if h > boxH {
			w, h = boxH/ratio, boxH
		}
	case boxW > 0:
		w, h = boxW, boxW*ratio
	case boxH > 0:
		w, h = boxH/ratio, boxH
	default:
		w, h = e.natW, e.natH
	}
	if w > areaW {
		w, h = areaW, areaW*ratio
	}
	return w, h
}

// MinWidth implements layout.Measurable. A size given in absolute units cannot
// shrink; an intrinsic or percentage size can, down to nothing, like a CSS
// replaced element whose width is capped by its container.
func (e *imageElement) MinWidth() float64 {
	w, fixed := e.fixedWidth()
	if !fixed {
		return 0
	}
	return w
}

// MaxWidth implements layout.Measurable: the width with no area limit.
func (e *imageElement) MaxWidth() float64 {
	w, _ := e.fixedWidth()
	return w
}

// fixedWidth returns the width the image takes when nothing narrows it, and
// whether that width comes only from absolute FitWidth/FitHeight values.
func (e *imageElement) fixedWidth() (float64, bool) {
	fw, okW := fixedLength(e.cfg.width)
	fh, okH := fixedLength(e.cfg.height)
	byHeight := fh * e.natW / e.natH
	switch {
	case okW && okH:
		return math.Min(fw, byHeight), true
	case okW:
		// A percentage height may still narrow it.
		return fw, e.cfg.height == nil
	case okH:
		return byHeight, e.cfg.width == nil
	default:
		return e.natW, false
	}
}

func (e *imageElement) PlanLayout(area layout.Area) layout.Plan {
	w, h := e.displaySize(area.Width, area.Height)
	if w <= 0 || h <= 0 {
		// A zero-width area (e.g. an Auto table column squeezed to its
		// minimum) leaves no room: nothing to draw, nothing to report.
		return layout.Plan{Status: layout.Full}
	}
	if !layout.Fits(h, area.Height) {
		if !area.AtTop || area.Height <= 0 {
			return layout.Plan{Status: layout.Nothing}
		}
		// Taller than an empty page: scale down to the page instead of
		// running off it (no other page has more room).
		w, h = w*area.Height/h, area.Height
	}

	data := e.data
	block := layout.Block{
		Width:  w,
		Height: h,
		Draw: func(r layout.Renderer) {
			r.DrawImage(data, 0, 0, w, h)
		},
	}
	return layout.Plan{
		Status:   layout.Full,
		Consumed: h,
		Blocks:   []layout.Block{block},
	}
}
