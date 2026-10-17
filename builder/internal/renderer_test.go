package internal

import (
	"testing"

	"github.com/coregx/gxpdf/creator"
	"github.com/coregx/gxpdf/layout"
)

// TestNewPDFRenderer_NilFonts verifies NewPDFRenderer does not panic with nil fonts.
func TestNewPDFRenderer_NilFonts(t *testing.T) {
	cr := creator.New()
	r := NewPDFRenderer(cr, nil, nil)
	if r == nil {
		t.Fatal("NewPDFRenderer returned nil")
	}
}

// TestRenderDocument_EmptyPages verifies that empty page slice creates one blank page.
func TestRenderDocument_EmptyPages(t *testing.T) {
	cr := creator.New()
	r := NewPDFRenderer(cr, nil, nil)
	err := r.RenderDocument(nil, ExportedMetadata("", ""))
	if err != nil {
		t.Fatalf("RenderDocument(nil) failed: %v", err)
	}
	if cr.PageCount() != 1 {
		t.Errorf("expected 1 blank page, got %d", cr.PageCount())
	}
}

// TestRenderDocument_SinglePage verifies a single page layout is rendered.
func TestRenderDocument_SinglePage(t *testing.T) {
	cr := creator.New()
	r := NewPDFRenderer(cr, nil, nil)

	pages := []layout.PageLayout{
		{
			Size:   layout.Size{Width: 595, Height: 842},
			Blocks: nil, // empty page
		},
	}
	err := r.RenderDocument(pages, ExportedMetadata("Title", "Author"))
	if err != nil {
		t.Fatalf("RenderDocument failed: %v", err)
	}
	if cr.PageCount() != 1 {
		t.Errorf("expected 1 page, got %d", cr.PageCount())
	}
}

// TestRenderDocument_MultiplePages verifies multiple pages are created.
func TestRenderDocument_MultiplePages(t *testing.T) {
	cr := creator.New()
	r := NewPDFRenderer(cr, nil, nil)

	pages := []layout.PageLayout{
		{Size: layout.Size{Width: 595, Height: 842}},
		{Size: layout.Size{Width: 595, Height: 842}},
		{Size: layout.Size{Width: 595, Height: 842}},
	}
	err := r.RenderDocument(pages, ExportedMetadata("", ""))
	if err != nil {
		t.Fatalf("RenderDocument failed: %v", err)
	}
	if cr.PageCount() != 3 {
		t.Errorf("expected 3 pages, got %d", cr.PageCount())
	}
}

// TestRenderDocument_PageWithTextBlock verifies a page with a text Draw block renders.
func TestRenderDocument_PageWithTextBlock(t *testing.T) {
	cr := creator.New()
	r := NewPDFRenderer(cr, nil, nil)

	font := layout.FontRef{Family: "Helvetica"}
	color := layout.Black

	pages := []layout.PageLayout{
		{
			Size: layout.Size{Width: 595, Height: 842},
			Blocks: []layout.Block{
				{
					X: 50, Y: 50, Width: 400, Height: 14,
					Draw: func(renderer layout.Renderer) {
						renderer.DrawText("Hello PDF", 0, 0, font, 12, color, layout.TextDrawOptions{})
					},
				},
			},
		},
	}
	err := r.RenderDocument(pages, ExportedMetadata("", ""))
	if err != nil {
		t.Fatalf("RenderDocument with text block failed: %v", err)
	}
}

// TestRenderDocument_PageWithRectBlock verifies rectangle drawing does not error.
func TestRenderDocument_PageWithRectBlock(t *testing.T) {
	cr := creator.New()
	r := NewPDFRenderer(cr, nil, nil)

	fill := layout.Color{R: 0.9, G: 0.9, B: 0.9}
	stroke := layout.Black

	pages := []layout.PageLayout{
		{
			Size: layout.Size{Width: 595, Height: 842},
			Blocks: []layout.Block{
				{
					X: 10, Y: 10, Width: 100, Height: 50,
					Draw: func(renderer layout.Renderer) {
						renderer.DrawRect(0, 0, 100, 50, &fill, &stroke, 1.0)
					},
				},
			},
		},
	}
	err := r.RenderDocument(pages, ExportedMetadata("", ""))
	if err != nil {
		t.Fatalf("RenderDocument with rect block failed: %v", err)
	}
}

// TestRenderDocument_PageWithLineBlock verifies line drawing does not error.
func TestRenderDocument_PageWithLineBlock(t *testing.T) {
	cr := creator.New()
	r := NewPDFRenderer(cr, nil, nil)

	color := layout.Black

	pages := []layout.PageLayout{
		{
			Size: layout.Size{Width: 595, Height: 842},
			Blocks: []layout.Block{
				{
					X: 50, Y: 100, Width: 400, Height: 1,
					Draw: func(renderer layout.Renderer) {
						renderer.DrawLine(0, 0, 400, 0, color, 1.0)
					},
				},
			},
		},
	}
	err := r.RenderDocument(pages, ExportedMetadata("", ""))
	if err != nil {
		t.Fatalf("RenderDocument with line block failed: %v", err)
	}
}

// TestRenderDocument_PageWithImageBlock verifies DrawImage stub does not error.
func TestRenderDocument_PageWithImageBlock(t *testing.T) {
	cr := creator.New()
	r := NewPDFRenderer(cr, nil, nil)

	pages := []layout.PageLayout{
		{
			Size: layout.Size{Width: 595, Height: 842},
			Blocks: []layout.Block{
				{
					X: 50, Y: 100, Width: 200, Height: 150,
					Draw: func(renderer layout.Renderer) {
						renderer.DrawImage(pngBytes(t, 4, 3), 0, 0, 200, 150)
					},
				},
			},
		},
	}
	err := r.RenderDocument(pages, ExportedMetadata("", ""))
	if err != nil {
		t.Fatalf("RenderDocument with image block failed: %v", err)
	}
}

// TestRenderDocument_BlockWithChildren verifies recursive child block rendering.
func TestRenderDocument_BlockWithChildren(t *testing.T) {
	cr := creator.New()
	r := NewPDFRenderer(cr, nil, nil)

	color := layout.Black
	font := layout.FontRef{Family: "Helvetica"}

	pages := []layout.PageLayout{
		{
			Size: layout.Size{Width: 595, Height: 842},
			Blocks: []layout.Block{
				{
					X: 50, Y: 50, Width: 400, Height: 100,
					// Parent has no Draw, just children.
					Children: []layout.Block{
						{
							X: 10, Y: 10, Width: 380, Height: 14,
							Draw: func(renderer layout.Renderer) {
								renderer.DrawText("Child block", 0, 0, font, 12, color, layout.TextDrawOptions{})
							},
						},
					},
				},
			},
		},
	}
	err := r.RenderDocument(pages, ExportedMetadata("", ""))
	if err != nil {
		t.Fatalf("RenderDocument with child blocks failed: %v", err)
	}
}

// TestRenderDocument_TextWithDecoration verifies underline and strikethrough
// options do not panic.
func TestRenderDocument_TextWithDecoration(t *testing.T) {
	cr := creator.New()
	r := NewPDFRenderer(cr, nil, nil)

	font := layout.FontRef{Family: "Helvetica"}
	color := layout.Black

	pages := []layout.PageLayout{
		{
			Size: layout.Size{Width: 595, Height: 842},
			Blocks: []layout.Block{
				{
					X: 50, Y: 50, Width: 400, Height: 30,
					Draw: func(renderer layout.Renderer) {
						renderer.DrawText("Underlined", 0, 0, font, 12, color, layout.TextDrawOptions{
							Underline: true,
						})
						renderer.DrawText("Strikethrough", 0, 14, font, 12, color, layout.TextDrawOptions{
							Strikethrough: true,
						})
					},
				},
			},
		},
	}
	err := r.RenderDocument(pages, ExportedMetadata("", ""))
	if err != nil {
		t.Fatalf("RenderDocument with decoration failed: %v", err)
	}
}

// TestRenderDocument_SetClipRect verifies SetClipRect/PopState do not panic.
func TestRenderDocument_SetClipRect(t *testing.T) {
	cr := creator.New()
	r := NewPDFRenderer(cr, nil, nil)

	pages := []layout.PageLayout{
		{
			Size: layout.Size{Width: 595, Height: 842},
			Blocks: []layout.Block{
				{
					X: 50, Y: 50, Width: 200, Height: 100,
					Draw: func(renderer layout.Renderer) {
						renderer.SetClipRect(0, 0, 200, 100)
						renderer.DrawRect(0, 0, 200, 100, nil, nil, 0)
						renderer.PopState()
					},
				},
			},
		},
	}
	err := r.RenderDocument(pages, ExportedMetadata("", ""))
	if err != nil {
		t.Fatalf("RenderDocument with clip rect failed: %v", err)
	}
}

// TestRenderDocument_PushState verifies PushState is a safe no-op.
func TestRenderDocument_PushState(t *testing.T) {
	cr := creator.New()
	r := NewPDFRenderer(cr, nil, nil)

	pages := []layout.PageLayout{
		{
			Size: layout.Size{Width: 595, Height: 842},
			Blocks: []layout.Block{
				{
					X: 0, Y: 0, Width: 100, Height: 100,
					Draw: func(renderer layout.Renderer) {
						renderer.PushState()
					},
				},
			},
		},
	}
	err := r.RenderDocument(pages, ExportedMetadata("", ""))
	if err != nil {
		t.Fatalf("RenderDocument with PushState failed: %v", err)
	}
}

// TestResolveStandard14Name_HelveticaVariants verifies all Helvetica variants.
func TestResolveStandard14Name_HelveticaVariants(t *testing.T) {
	tests := []struct {
		font layout.FontRef
		want creator.FontName
	}{
		{layout.FontRef{Family: "Helvetica"}, creator.Helvetica},
		{layout.FontRef{Family: "Helvetica", Weight: layout.WeightBold}, creator.HelveticaBold},
		{layout.FontRef{Family: "Helvetica", Style: layout.StyleItalic}, creator.HelveticaOblique},
		{layout.FontRef{Family: "Helvetica", Weight: layout.WeightBold, Style: layout.StyleItalic}, creator.HelveticaBoldOblique},
	}
	for _, tc := range tests {
		got := stdName(tc.font)
		if got != tc.want {
			t.Errorf("stdName(%v) = %q, want %q", tc.font, got, tc.want)
		}
	}
}

// TestResolveStandard14Name_TimesVariants verifies Times family resolution.
func TestResolveStandard14Name_TimesVariants(t *testing.T) {
	tests := []struct {
		font layout.FontRef
		want creator.FontName
	}{
		{layout.FontRef{Family: "Times"}, creator.TimesRoman},
		{layout.FontRef{Family: "Times New Roman"}, creator.TimesRoman},
		{layout.FontRef{Family: "Times", Weight: layout.WeightBold}, creator.TimesBold},
		{layout.FontRef{Family: "Times", Style: layout.StyleItalic}, creator.TimesItalic},
		{layout.FontRef{Family: "Times", Weight: layout.WeightBold, Style: layout.StyleItalic}, creator.TimesBoldItalic},
	}
	for _, tc := range tests {
		got := stdName(tc.font)
		if got != tc.want {
			t.Errorf("stdName(%v) = %q, want %q", tc.font, got, tc.want)
		}
	}
}

// TestResolveStandard14Name_CourierVariants verifies Courier family resolution.
func TestResolveStandard14Name_CourierVariants(t *testing.T) {
	tests := []struct {
		font layout.FontRef
		want creator.FontName
	}{
		{layout.FontRef{Family: "Courier"}, creator.Courier},
		{layout.FontRef{Family: "Courier New"}, creator.Courier},
		{layout.FontRef{Family: "Courier", Weight: layout.WeightBold}, creator.CourierBold},
		{layout.FontRef{Family: "Courier", Style: layout.StyleItalic}, creator.CourierOblique},
		{layout.FontRef{Family: "Courier", Weight: layout.WeightBold, Style: layout.StyleItalic}, creator.CourierBoldOblique},
	}
	for _, tc := range tests {
		got := stdName(tc.font)
		if got != tc.want {
			t.Errorf("stdName(%v) = %q, want %q", tc.font, got, tc.want)
		}
	}
}

// TestResolveStandard14Name_SpecialFonts verifies Symbol and ZapfDingbats.
func TestResolveStandard14Name_SpecialFonts(t *testing.T) {
	if got := stdName(layout.FontRef{Family: "Symbol"}); got != creator.Symbol {
		t.Errorf("Symbol -> %q, want Symbol", got)
	}
	if got := stdName(layout.FontRef{Family: "ZapfDingbats"}); got != creator.ZapfDingbats {
		t.Errorf("ZapfDingbats -> %q, want ZapfDingbats", got)
	}
}

// TestResolveStandard14Name_UnknownFallsToHelvetica verifies unknown families fall back.
func TestResolveStandard14Name_UnknownFallsToHelvetica(t *testing.T) {
	got := stdName(layout.FontRef{Family: "UnknownFont"})
	if got != creator.Helvetica {
		t.Errorf("unknown font -> %q, want Helvetica", got)
	}
}

// TestExportedMetadata verifies ExportedMetadata constructs the correct struct.
func TestExportedMetadata(t *testing.T) {
	m := ExportedMetadata("My Doc", "Jane Doe")
	if m.Title != "My Doc" {
		t.Errorf("Title = %q, want %q", m.Title, "My Doc")
	}
	if m.Author != "Jane Doe" {
		t.Errorf("Author = %q, want %q", m.Author, "Jane Doe")
	}
}
