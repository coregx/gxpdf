package creator

import (
	"errors"

	"github.com/coregx/gxpdf/internal/document"
	"github.com/coregx/gxpdf/internal/fonts"
)

// Page represents a page in the PDF document being created.
//
// This is a high-level wrapper around the domain Page entity,
// providing a simplified API for adding content.
//
// Example:
//
//	page := creator.NewPage()
//	// Add text, images, etc. to page...
type Page struct {
	// Domain model
	page *document.Page

	// Creator settings
	margins Margins

	// Content operations
	textOps     []TextOperation     // Text drawing operations
	graphicsOps []GraphicsOperation // Graphics drawing operations
}

// SetRotation sets the page rotation.
//
// Valid values are 0, 90, 180, and 270 degrees (clockwise).
//
// Example:
//
//	page.SetRotation(90) // Landscape
func (p *Page) SetRotation(degrees int) error {
	if err := p.page.SetRotation(degrees); err != nil {
		return err
	}
	return nil
}

// Rotate rotates the page by the specified degrees (clockwise).
//
// Valid values are 0, 90, 180, and 270 degrees.
// This method sets the absolute rotation, not cumulative.
//
// This is an alias for SetRotation() for API convenience.
//
// Example:
//
//	page.Rotate(90)  // Landscape
//	page.Rotate(180) // Upside down
//	page.Rotate(270) // Landscape (counter-clockwise)
func (p *Page) Rotate(degrees int) error {
	return p.SetRotation(degrees)
}

// Rotation returns the current page rotation in degrees.
func (p *Page) Rotation() int {
	return p.page.Rotation()
}

// Width returns the page width in points.
//
// If the page is rotated 90 or 270 degrees, width and height are swapped.
func (p *Page) Width() float64 {
	return p.page.Width()
}

// Height returns the page height in points.
//
// If the page is rotated 90 or 270 degrees, width and height are swapped.
func (p *Page) Height() float64 {
	return p.page.Height()
}

// Margins returns the page margins.
func (p *Page) Margins() Margins {
	return p.margins
}

// SetMargins sets page-specific margins.
//
// This overrides the default margins from the Creator.
//
// Example:
//
//	page.SetMargins(36, 36, 36, 36) // 0.5 inch margins
func (p *Page) SetMargins(top, right, bottom, left float64) error {
	if top < 0 || right < 0 || bottom < 0 || left < 0 {
		return ErrInvalidMargins
	}

	p.margins = Margins{
		Top:    top,
		Right:  right,
		Bottom: bottom,
		Left:   left,
	}
	return nil
}

// ContentWidth returns the usable width (page width minus left and right margins).
func (p *Page) ContentWidth() float64 {
	return p.Width() - p.margins.Left - p.margins.Right
}

// ContentHeight returns the usable height (page height minus top and bottom margins).
func (p *Page) ContentHeight() float64 {
	return p.Height() - p.margins.Top - p.margins.Bottom
}

// AddText adds text to the page at the specified position with default black color.
//
// Parameters:
//   - text: The string to display
//   - x: Horizontal position in points (from left edge)
//   - y: Vertical position in points (from bottom edge)
//   - font: Font to use (one of the Standard 14 fonts)
//   - size: Font size in points
//
// Example:
//
//	err := page.AddText("Hello World", 100, 700, creator.Helvetica, 24)
func (p *Page) AddText(text string, x, y float64, font FontName, size float64) error {
	return p.AddTextColor(text, x, y, font, size, Black)
}

// AddTextColor adds colored text to the page at the specified position.
//
// Parameters:
//   - text: The string to display
//   - x: Horizontal position in points (from left edge)
//   - y: Vertical position in points (from bottom edge)
//   - font: Font to use (one of the Standard 14 fonts)
//   - size: Font size in points
//   - color: Text color (RGB, 0.0 to 1.0 range)
//
// Example:
//
//	err := page.AddTextColor("Error!", 100, 700, creator.HelveticaBold, 18, creator.Red)
func (p *Page) AddTextColor(text string, x, y float64, font FontName, size float64, color Color) error {
	// Validate font size
	if size <= 0 {
		return errors.New("font size must be positive")
	}

	// Validate color components
	if color.R < 0 || color.R > 1 || color.G < 0 || color.G > 1 || color.B < 0 || color.B > 1 {
		return errors.New("color components must be in range [0.0, 1.0]")
	}

	// Store text operation
	p.textOps = append(p.textOps, TextOperation{
		Text:  text,
		X:     x,
		Y:     y,
		Font:  font,
		Size:  size,
		Color: color,
	})

	return nil
}

// AddTextColorCMYK adds CMYK-colored text to the page at the specified position.
//
// CMYK (Cyan, Magenta, Yellow, blacK) is a subtractive color model used in
// professional printing. This method should be used when precise color control
// is needed for print production.
//
// Parameters:
//   - text: The string to display
//   - x: Horizontal position in points (from left edge)
//   - y: Vertical position in points (from bottom edge)
//   - font: Font to use (one of the Standard 14 fonts)
//   - size: Font size in points
//   - color: Text color (CMYK, 0.0 to 1.0 range)
//
// Example:
//
//	// Pure cyan for printing
//	cyan := creator.NewColorCMYK(1.0, 0.0, 0.0, 0.0)
//	err := page.AddTextColorCMYK("Print-ready text", 100, 700, creator.Helvetica, 12, cyan)
func (p *Page) AddTextColorCMYK(text string, x, y float64, font FontName, size float64, color ColorCMYK) error {
	// Validate font size
	if size <= 0 {
		return errors.New("font size must be positive")
	}

	// Validate CMYK color components
	if color.C < 0 || color.C > 1 || color.M < 0 || color.M > 1 ||
		color.Y < 0 || color.Y > 1 || color.K < 0 || color.K > 1 {
		return errors.New("CMYK color components must be in range [0.0, 1.0]")
	}

	// Store text operation with CMYK color
	p.textOps = append(p.textOps, TextOperation{
		Text:      text,
		X:         x,
		Y:         y,
		Font:      font,
		Size:      size,
		ColorCMYK: &color,
	})

	return nil
}

// AddTextCustomFont adds text using an embedded TrueType/OpenType font.
//
// This method supports Unicode text including Cyrillic, CJK, Arabic, and symbols.
// The font is automatically subset to include only the glyphs used in the document.
//
// Parameters:
//   - text: The string to display (supports Unicode)
//   - x: Horizontal position in points (from left edge)
//   - y: Vertical position in points (from bottom edge)
//   - font: Custom font loaded via LoadFont()
//   - size: Font size in points
//
// Example:
//
//	font, _ := creator.LoadFont("fonts/OpenSans-Regular.ttf")
//	err := page.AddTextCustomFont("Привет мир! 你好世界!", 100, 700, font, 24)
func (p *Page) AddTextCustomFont(text string, x, y float64, font *CustomFont, size float64) error {
	return p.AddTextCustomFontColor(text, x, y, font, size, Black)
}

// AddTextCustomFontColor adds colored text using an embedded TrueType/OpenType font.
//
// This method supports Unicode text including Cyrillic, CJK, Arabic, and symbols.
// The font is automatically subset to include only the glyphs used in the document.
//
// Parameters:
//   - text: The string to display (supports Unicode)
//   - x: Horizontal position in points (from left edge)
//   - y: Vertical position in points (from bottom edge)
//   - font: Custom font loaded via LoadFont()
//   - size: Font size in points
//   - color: Text color (RGB, 0.0 to 1.0 range)
//
// Example:
//
//	font, _ := creator.LoadFont("fonts/OpenSans-Bold.ttf")
//	err := page.AddTextCustomFontColor("重要!", 100, 700, font, 24, creator.Red)
func (p *Page) AddTextCustomFontColor(text string, x, y float64, font *CustomFont, size float64, color Color) error {
	if font == nil {
		return errors.New("font cannot be nil")
	}
	if size <= 0 {
		return errors.New("font size must be positive")
	}
	if color.R < 0 || color.R > 1 || color.G < 0 || color.G > 1 || color.B < 0 || color.B > 1 {
		return errors.New("color components must be in range [0.0, 1.0]")
	}

	// Mark characters as used for font subsetting.
	font.UseString(text)

	// Store text operation with custom font.
	p.textOps = append(p.textOps, TextOperation{
		Text:       text,
		X:          x,
		Y:          y,
		CustomFont: font,
		Size:       size,
		Color:      color,
	})

	return nil
}

// TextOperations returns all text operations for this page.
//
// This is used by the writer infrastructure to generate the content stream.
func (p *Page) TextOperations() []TextOperation {
	return p.textOps
}

// GraphicsOperations returns all graphics operations for this page.
//
// This is used by the writer infrastructure to generate the content stream.
func (p *Page) GraphicsOperations() []GraphicsOperation {
	return p.graphicsOps
}

// DrawLine draws a line from (x1,y1) to (x2,y2).
//
// Parameters:
//   - x1, y1: Starting point coordinates
//   - x2, y2: Ending point coordinates
//   - opts: Line options (color, width, dash pattern)
//
// Example:
//
//	opts := &creator.LineOptions{
//	    Color: creator.Black,
//	    Width: 2.0,
//	}
//	err := page.DrawLine(100, 700, 500, 700, opts)
func (p *Page) DrawLine(x1, y1, x2, y2 float64, opts *LineOptions) error {
	if opts == nil {
		return errors.New("line options cannot be nil")
	}

	// Validate color components.
	if opts.Color.R < 0 || opts.Color.R > 1 || opts.Color.G < 0 || opts.Color.G > 1 || opts.Color.B < 0 || opts.Color.B > 1 {
		return errors.New("color components must be in range [0.0, 1.0]")
	}

	// Validate width.
	if opts.Width < 0 {
		return errors.New("line width must be non-negative")
	}

	// Store graphics operation.
	p.graphicsOps = append(p.graphicsOps, GraphicsOperation{
		Type:     GraphicsOpLine,
		X:        x1,
		Y:        y1,
		X2:       x2,
		Y2:       y2,
		LineOpts: opts,
	})

	return nil
}

// DrawRect draws a rectangle at (x,y) with given width and height.
//
// The rectangle can be stroked, filled, or both, depending on the options.
//
// Parameters:
//   - x, y: Lower-left corner coordinates
//   - width, height: Rectangle dimensions
//   - opts: Rectangle options (stroke color, fill color, width, dash pattern)
//
// Example:
//
//	opts := &creator.RectOptions{
//	    StrokeColor: &creator.Black,
//	    StrokeWidth: 1.0,
//	    FillColor:   &creator.LightGray,
//	}
//	err := page.DrawRect(100, 600, 200, 100, opts)
func (p *Page) DrawRect(x, y, width, height float64, opts *RectOptions) error {
	if opts == nil {
		return errors.New("rectangle options cannot be nil")
	}

	// Validate dimensions.
	if width < 0 || height < 0 {
		return errors.New("rectangle dimensions must be non-negative")
	}

	// Validate options.
	if err := validateRectOptions(opts); err != nil {
		return err
	}

	// Store graphics operation.
	p.graphicsOps = append(p.graphicsOps, GraphicsOperation{
		Type:     GraphicsOpRect,
		X:        x,
		Y:        y,
		Width:    width,
		Height:   height,
		RectOpts: opts,
	})

	return nil
}

// DrawRectFilled draws a filled rectangle (convenience method).
//
// This is a shorthand for DrawRect with only fill color.
//
// Parameters:
//   - x, y: Lower-left corner coordinates
//   - width, height: Rectangle dimensions
//   - fillColor: Fill color
//
// Example:
//
//	err := page.DrawRectFilled(100, 600, 200, 100, creator.LightGray)
func (p *Page) DrawRectFilled(x, y, width, height float64, fillColor Color) error {
	opts := &RectOptions{
		FillColor: &fillColor,
	}
	return p.DrawRect(x, y, width, height, opts)
}

// BeginClipRect begins a rectangular clipping region.
//
// All subsequent drawing operations (shapes, text, images) will be clipped
// to the specified rectangle. Content outside the rectangle will not be visible.
//
// This is useful for tables where text should not overflow cell boundaries.
//
// You MUST call EndClip() after drawing the clipped content to restore
// the previous graphics state. Clipping regions can be nested.
//
// Parameters:
//   - x, y: Lower-left corner of the clipping rectangle
//   - width, height: Size of the clipping rectangle
//
// Example:
//
//	// Clip text to a cell boundary
//	page.BeginClipRect(100, 500, 200, 30)
//	page.AddText("Very long text that would overflow...", 105, 510, opts)
//	page.EndClip()
func (p *Page) BeginClipRect(x, y, width, height float64) error {
	if width <= 0 || height <= 0 {
		return errors.New("clipping rectangle must have positive width and height")
	}

	p.graphicsOps = append(p.graphicsOps, GraphicsOperation{
		Type:   GraphicsOpBeginClip,
		X:      x,
		Y:      y,
		Width:  width,
		Height: height,
	})

	return nil
}

// EndClip ends a clipping region started by BeginClipRect.
//
// This restores the graphics state to what it was before BeginClipRect was called.
// Every BeginClipRect MUST have a matching EndClip.
func (p *Page) EndClip() error {
	p.graphicsOps = append(p.graphicsOps, GraphicsOperation{
		Type: GraphicsOpEndClip,
	})

	return nil
}

// DrawTextClipped draws text that is clipped to a rectangular region.
//
// This is useful for table cells where text should not overflow the cell boundary.
// Text that extends beyond the clipping rectangle will be cut off (not visible).
//
// Parameters:
//   - text: The text to render
//   - textX, textY: Position of the text baseline
//   - clipX, clipY, clipW, clipH: Clipping rectangle (text outside is hidden)
//   - font: Custom font to use
//   - fontSize: Font size in points
//   - color: Text color
//
// Example:
//
//	// Draw text clipped to a 100pt wide cell
//	page.DrawTextClipped("Very long text...", 55, 510, 50, 500, 100, 30, font, 10, Black)
func (p *Page) DrawTextClipped(text string, textX, textY, clipX, clipY, clipW, clipH float64, font *CustomFont, fontSize float64, color Color) error {
	if font == nil {
		return errors.New("font cannot be nil")
	}
	if fontSize <= 0 {
		return errors.New("font size must be positive")
	}
	if clipW <= 0 || clipH <= 0 {
		return errors.New("clipping rectangle must have positive dimensions")
	}

	// Mark characters as used for font subsetting.
	font.UseString(text)

	// Add BeginClip operation.
	p.graphicsOps = append(p.graphicsOps, GraphicsOperation{
		Type:   GraphicsOpBeginClip,
		X:      clipX,
		Y:      clipY,
		Width:  clipW,
		Height: clipH,
	})

	// Add TextBlock operation (rendered inline with graphics).
	p.graphicsOps = append(p.graphicsOps, GraphicsOperation{
		Type:      GraphicsOpTextBlock,
		X:         textX,
		Y:         textY,
		Text:      text,
		TextFont:  font,
		TextSize:  fontSize,
		TextColor: &color,
	})

	// Add EndClip operation.
	p.graphicsOps = append(p.graphicsOps, GraphicsOperation{
		Type: GraphicsOpEndClip,
	})

	return nil
}

// DrawCircle draws a circle at center (cx,cy) with given radius.
//
// The circle can be stroked, filled, or both, depending on the options.
// The circle is approximated using 4 cubic Bézier curves.
//
// Parameters:
//   - cx, cy: Center coordinates
//   - radius: Circle radius
//   - opts: Circle options (stroke color, fill color, stroke width)
//
// Example:
//
//	opts := &creator.CircleOptions{
//	    StrokeColor: &creator.Red,
//	    StrokeWidth: 2.0,
//	    FillColor:   &creator.Yellow,
//	}
//	err := page.DrawCircle(300, 400, 50, opts)
func (p *Page) DrawCircle(cx, cy, radius float64, opts *CircleOptions) error {
	if opts == nil {
		return errors.New("circle options cannot be nil")
	}

	// Validate radius.
	if radius < 0 {
		return errors.New("circle radius must be non-negative")
	}

	// Validate options.
	if err := validateCircleOptions(opts); err != nil {
		return err
	}

	// Store graphics operation.
	p.graphicsOps = append(p.graphicsOps, GraphicsOperation{
		Type:       GraphicsOpCircle,
		X:          cx,
		Y:          cy,
		Radius:     radius,
		CircleOpts: opts,
	})

	return nil
}

// validateColor validates that all color components are in range [0, 1].
func validateColor(c Color) error {
	if c.R < 0 || c.R > 1 || c.G < 0 || c.G > 1 || c.B < 0 || c.B > 1 {
		return errors.New("color components must be in range [0.0, 1.0]")
	}
	return nil
}

// validateRectOptions validates rectangle drawing options.
func validateRectOptions(opts *RectOptions) error {
	// Validate stroke color if provided.
	if opts.StrokeColor != nil {
		if err := validateColor(*opts.StrokeColor); err != nil {
			return errors.New("stroke " + err.Error())
		}
	}

	// Validate fill color if provided.
	if opts.FillColor != nil {
		if err := validateColor(*opts.FillColor); err != nil {
			return errors.New("fill " + err.Error())
		}
	}

	// Validate stroke width.
	if opts.StrokeWidth < 0 {
		return errors.New("stroke width must be non-negative")
	}

	// At least one of stroke or fill must be set.
	if opts.StrokeColor == nil && opts.FillColor == nil && opts.FillGradient == nil {
		return errors.New("rectangle must have at least stroke, fill color, or gradient")
	}

	// FillColor and FillGradient are mutually exclusive
	if opts.FillColor != nil && opts.FillGradient != nil {
		return errors.New("cannot use both fill color and fill gradient")
	}

	// Validate gradient if provided
	if opts.FillGradient != nil {
		if err := opts.FillGradient.Validate(); err != nil {
			return errors.New("fill gradient: " + err.Error())
		}
	}

	return nil
}

// validateCircleOptions validates circle drawing options.
func validateCircleOptions(opts *CircleOptions) error {
	// Validate stroke color if provided.
	if opts.StrokeColor != nil {
		if err := validateColor(*opts.StrokeColor); err != nil {
			return errors.New("stroke " + err.Error())
		}
	}

	// Validate fill color if provided.
	if opts.FillColor != nil {
		if err := validateColor(*opts.FillColor); err != nil {
			return errors.New("fill " + err.Error())
		}
	}

	// Validate stroke width.
	if opts.StrokeWidth < 0 {
		return errors.New("stroke width must be non-negative")
	}

	// At least one of stroke or fill must be set.
	if opts.StrokeColor == nil && opts.FillColor == nil && opts.FillGradient == nil {
		return errors.New("circle must have at least stroke, fill color, or gradient")
	}

	// FillColor and FillGradient are mutually exclusive
	if opts.FillColor != nil && opts.FillGradient != nil {
		return errors.New("cannot use both fill color and fill gradient")
	}

	// Validate gradient if provided
	if opts.FillGradient != nil {
		if err := opts.FillGradient.Validate(); err != nil {
			return errors.New("fill gradient: " + err.Error())
		}
	}

	return nil
}

// GetLayoutContext creates a LayoutContext for this page.
//
// The context is initialized with the cursor at the top-left of the content area
// (inside margins).
//
// Example:
//
//	ctx := page.GetLayoutContext()
//	paragraph := NewParagraph("Hello World")
//	paragraph.Draw(ctx, page)
func (p *Page) GetLayoutContext() *LayoutContext {
	return &LayoutContext{
		PageWidth:  p.Width(),
		PageHeight: p.Height(),
		Margins:    p.margins,
		CursorX:    p.margins.Left,
		CursorY:    0, // Top of content area
	}
}

// Draw renders a Drawable element on the page.
//
// This uses the page's layout context and automatically positions
// the element. The cursor advances after drawing.
//
// Example:
//
//	p := NewParagraph("Hello World")
//	page.Draw(p)
func (p *Page) Draw(d Drawable) error {
	ctx := p.GetLayoutContext()
	return d.Draw(ctx, p)
}

// DrawAt renders a Drawable element at a specific position.
//
// x is measured from the left edge of the page.
// y is measured from the top of the content area (below top margin).
//
// Example:
//
//	p := NewParagraph("Hello World")
//	page.DrawAt(p, 100, 50)  // 100 points from left, 50 from top
func (p *Page) DrawAt(d Drawable, x, y float64) error {
	ctx := p.GetLayoutContext()
	ctx.SetCursor(x, y)
	return d.Draw(ctx, p)
}

// MoveCursor moves the page's layout cursor to the specified position.
//
// This affects subsequent Draw() calls that use the default layout context.
// Note: This creates a new context each time, so for multiple sequential
// draws, use GetLayoutContext() and pass it to Draw() on the Drawable directly.
//
// x is measured from the left edge of the page.
// y is measured from the top of the content area (below top margin).
func (p *Page) MoveCursor(x, y float64) {
	// Note: Since we create a new context each time in Draw(),
	// this method primarily exists for API consistency.
	// For efficient multi-draw operations, use GetLayoutContext() directly.
	_ = x
	_ = y
}

// Surface creates a new drawing surface for this page.
//
// Surface provides Skia-like Push/Pop semantics for graphics state management.
// This allows composable transformations, opacity, blend modes, and clipping.
//
// Example:
//
//	surface := page.Surface()
//	surface.PushTransform(Rotate(45))
//	surface.PushOpacity(0.5)
//	// ... draw operations ...
//	surface.Pop()
//	surface.Pop()
func (p *Page) Surface() *Surface {
	return NewSurface(p)
}

// AddLink adds a clickable URL link with default styling (blue, underlined).
//
// The text is rendered at the specified position and made clickable.
// A clickable annotation is created that covers the text area.
//
// Parameters:
//   - text: The link text to display
//   - url: The target URL (e.g., "https://example.com")
//   - x: Horizontal position in points (from left edge)
//   - y: Vertical position in points (from bottom edge)
//   - font: Font to use for the link text
//   - size: Font size in points
//
// Example:
//
//	page.AddLink("Visit Google", "https://google.com", 100, 700, creator.Helvetica, 12)
func (p *Page) AddLink(text, url string, x, y float64, font FontName, size float64) error {
	style := DefaultLinkStyle()
	style.Font = font
	style.Size = size
	return p.AddLinkStyled(text, url, x, y, style)
}

// AddLinkStyled adds a clickable URL link with custom styling.
//
// This gives full control over the link appearance (font, size, color, underline).
//
// Parameters:
//   - text: The link text to display
//   - url: The target URL (e.g., "https://example.com")
//   - x: Horizontal position in points (from left edge)
//   - y: Vertical position in points (from bottom edge)
//   - style: Visual style for the link
//
// Example:
//
//	style := creator.LinkStyle{
//	    Font:      creator.HelveticaBold,
//	    Size:      14,
//	    Color:     creator.Red,
//	    Underline: false,
//	}
//	page.AddLinkStyled("Click here", "https://example.com", 100, 700, style)
func (p *Page) AddLinkStyled(text, url string, x, y float64, style LinkStyle) error {
	return p.addLinkWithStyle(text, url, -1, false, x, y, style)
}

// AddInternalLink adds a link to another page in the document.
//
// The destPage parameter is 0-based (0 = first page, 1 = second page, etc.).
//
// Parameters:
//   - text: The link text to display
//   - destPage: Target page number (0-based)
//   - x: Horizontal position in points (from left edge)
//   - y: Vertical position in points (from bottom edge)
//   - font: Font to use for the link text
//   - size: Font size in points
//
// Example:
//
//	page.AddInternalLink("See page 3", 2, 100, 600, creator.Helvetica, 12)
func (p *Page) AddInternalLink(text string, destPage int, x, y float64, font FontName, size float64) error {
	style := DefaultLinkStyle()
	style.Font = font
	style.Size = size
	return p.addLinkWithStyle(text, "", destPage, true, x, y, style)
}

// addLinkWithStyle is the internal implementation for adding links.
//
// This method:
// 1. Renders the text at the specified position with the given style.
// 2. Optionally draws an underline below the text.
// 3. Calculates the bounding rectangle for the clickable area.
// 4. Creates a LinkAnnotation and adds it to the domain page.
func (p *Page) addLinkWithStyle(text, url string, destPage int, isInternal bool, x, y float64, style LinkStyle) error {
	// Validate inputs.
	if err := validateLinkInputs(text, url, destPage, isInternal, style.Size); err != nil {
		return err
	}

	// Render the link text with the specified style.
	if err := p.AddTextColor(text, x, y, style.Font, style.Size, style.Color); err != nil {
		return err
	}

	// Measure text width for bounding rect and underline.
	textWidth := measureTextWidth(string(style.Font), text, style.Size)

	// Draw underline if requested.
	if style.Underline {
		if err := p.drawUnderline(x, y, textWidth, style); err != nil {
			return err
		}
	}

	// Calculate bounding rectangle and create annotation.
	rect := calculateLinkRect(x, y, textWidth, style.Size)
	annot := createLinkAnnotation(rect, url, destPage, isInternal)

	// Add annotation to domain page.
	return p.page.AddAnnotation(annot)
}

// validateLinkInputs validates the inputs for adding a link.
func validateLinkInputs(text, url string, destPage int, isInternal bool, fontSize float64) error {
	if text == "" {
		return errors.New("link text cannot be empty")
	}
	if !isInternal && url == "" {
		return errors.New("external link must have a URL")
	}
	if isInternal && destPage < 0 {
		return errors.New("internal link destination page must be >= 0")
	}
	if fontSize <= 0 {
		return errors.New("link font size must be positive")
	}
	return nil
}

// createLinkAnnotation creates a link annotation based on the link type.
func createLinkAnnotation(rect [4]float64, url string, destPage int, isInternal bool) *document.LinkAnnotation {
	if isInternal {
		return document.NewInternalLinkAnnotation(rect, destPage)
	}
	return document.NewLinkAnnotation(rect, url)
}

// drawUnderline draws an underline below the link text.
func (p *Page) drawUnderline(x, y, width float64, style LinkStyle) error {
	// Underline position: slightly below baseline (fontSize * 0.1).
	underlineY := y - style.Size*0.1
	underlineWidth := style.Size * 0.05 // 5% of font size

	lineOpts := &LineOptions{
		Color: style.Color,
		Width: underlineWidth,
	}

	return p.DrawLine(x, underlineY, x+width, underlineY, lineOpts)
}

// calculateLinkRect calculates the bounding rectangle for a link.
//
// The rectangle encompasses the text with some padding above and below.
// Returns [x1, y1, x2, y2] in PDF coordinates.
func calculateLinkRect(x, y, width, fontSize float64) [4]float64 {
	// Vertical padding: 10% above and below the font size.
	padding := fontSize * 0.1

	return [4]float64{
		x,                      // x1 (left)
		y - padding,            // y1 (bottom, with padding below baseline)
		x + width,              // x2 (right)
		y + fontSize + padding, // y2 (top, with padding above cap height)
	}
}

// measureTextWidth measures the width of text in points.
func measureTextWidth(fontName, text string, size float64) float64 {
	// Import fonts package for text measurement.
	return fonts.MeasureString(fontName, text, size)
}

// AddTextAnnotation adds a text (sticky note) annotation to the page.
//
// The annotation appears as a small icon at the specified position.
// When clicked, it displays the contents in a pop-up.
//
// Example:
//
//	note := creator.NewTextAnnotation(100, 700, "Review this section")
//	note.SetAuthor("Alice").SetColor(creator.Yellow)
//	page.AddTextAnnotation(note)
func (p *Page) AddTextAnnotation(annotation *TextAnnotation) error {
	domainAnnot := annotation.toDomain()
	return p.page.AddTextAnnotation(domainAnnot)
}

// AddHighlightAnnotation adds a highlight annotation to the page.
//
// The highlight marks text with a colored overlay.
//
// Example:
//
//	highlight := creator.NewHighlightAnnotation(100, 650, 300, 670)
//	highlight.SetColor(creator.Yellow).SetAuthor("Bob")
//	page.AddHighlightAnnotation(highlight)
func (p *Page) AddHighlightAnnotation(annotation *HighlightAnnotation) error {
	domainAnnot := annotation.toDomain()
	return p.page.AddMarkupAnnotation(domainAnnot)
}

// AddUnderlineAnnotation adds an underline annotation to the page.
//
// The underline draws a line under text.
//
// Example:
//
//	underline := creator.NewUnderlineAnnotation(100, 650, 300, 670)
//	underline.SetColor(creator.Blue)
//	page.AddUnderlineAnnotation(underline)
func (p *Page) AddUnderlineAnnotation(annotation *UnderlineAnnotation) error {
	domainAnnot := annotation.toDomain()
	return p.page.AddMarkupAnnotation(domainAnnot)
}

// AddStrikeOutAnnotation adds a strikeout annotation to the page.
//
// The strikeout draws a line through text.
//
// Example:
//
//	strikeout := creator.NewStrikeOutAnnotation(100, 650, 300, 670)
//	strikeout.SetColor(creator.Red)
//	page.AddStrikeOutAnnotation(strikeout)
func (p *Page) AddStrikeOutAnnotation(annotation *StrikeOutAnnotation) error {
	domainAnnot := annotation.toDomain()
	return p.page.AddMarkupAnnotation(domainAnnot)
}

// AddStampAnnotation adds a stamp annotation to the page.
//
// The stamp displays predefined text like "Approved", "Draft", etc.
//
// Example:
//
//	stamp := creator.NewStampAnnotation(300, 700, 100, 50, creator.StampApproved)
//	stamp.SetColor(creator.Green).SetAuthor("Manager")
//	page.AddStampAnnotation(stamp)
func (p *Page) AddStampAnnotation(annotation *StampAnnotation) error {
	domainAnnot := annotation.toDomain()
	return p.page.AddStampAnnotation(domainAnnot)
}

// AddField adds a form field to the page.
//
// Form fields allow user input and interaction in PDF documents.
// This is part of the AcroForm (Interactive Forms) system.
//
// Supported field types:
//   - TextField: Single-line or multi-line text input
//   - (Future: CheckBox, RadioButton, ComboBox, ListBox, PushButton)
//
// Example:
//
//	field := forms.NewTextField("username", 100, 700, 200, 20)
//	field.SetValue("John Doe").SetRequired(true)
//	page.AddField(field)
func (p *Page) AddField(field interface{}) error {
	// Convert creator form field to domain form field
	domainField, err := convertFieldToDomain(field)
	if err != nil {
		return err
	}

	return p.page.AddFormField(domainField)
}

// Errors.
var (
	// ErrContentOutOfBounds is returned when content is positioned outside margins.
	ErrContentOutOfBounds = errors.New("content is outside page margins")

	// ErrUnsupportedFieldType is returned when an unsupported field type is added.
	ErrUnsupportedFieldType = errors.New("unsupported form field type")
)
