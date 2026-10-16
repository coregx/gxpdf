package internal

import (
	"strings"
	"testing"

	"github.com/coregx/gxpdf/creator"
	"github.com/coregx/gxpdf/layout"
)

// ============================================================================
// FontBridge.breakCJK — previously 0% coverage
// ============================================================================

func TestFontBridge_BreakCJK_ShortText(t *testing.T) {
	fb := newTestBridge()
	font := layout.FontRef{Family: "Helvetica"}

	// Short CJK text that fits on one line.
	lines := fb.breakCJK(font, "你好", 12, 1000)
	if len(lines) != 1 {
		t.Errorf("short CJK text in wide area should produce 1 line, got %d: %v", len(lines), lines)
	}
	if lines[0] != "你好" {
		t.Errorf("line = %q, want %q", lines[0], "你好")
	}
}

func TestFontBridge_BreakCJK_ForcesBreak(t *testing.T) {
	fb := newTestBridge()
	font := layout.FontRef{Family: "Helvetica"}

	// Very narrow width forces break at individual CJK characters.
	text := "你好世界"
	lines := fb.breakCJK(font, text, 12, 1) // 1 point: impossible to fit even one char
	// Should produce at least 1 line (each char on its own).
	if len(lines) == 0 {
		t.Fatal("breakCJK should return at least one line")
	}
	// All characters must be preserved across lines.
	joined := strings.Join(lines, "")
	if joined != text {
		t.Errorf("breakCJK lost characters: got %q, want %q", joined, text)
	}
}

func TestFontBridge_BreakCJK_EmptyString(t *testing.T) {
	fb := newTestBridge()
	font := layout.FontRef{Family: "Helvetica"}

	lines := fb.breakCJK(font, "", 12, 100)
	// Empty string produces zero lines (no current to flush).
	if len(lines) != 0 {
		t.Errorf("breakCJK on empty string should return [], got %v", lines)
	}
}

// ============================================================================
// FontBridge.LineBreak with CJK text — exercises CJK code path in LineBreak
// ============================================================================

func TestFontBridge_LineBreak_CJKText(t *testing.T) {
	fb := newTestBridge()
	font := layout.FontRef{Family: "Helvetica"}

	// CJK text with reasonable width.
	lines := fb.LineBreak(font, "你好世界Hello", 12, 1000)
	if len(lines) == 0 {
		t.Fatal("LineBreak should return at least one line")
	}
	// All content should be preserved.
	joined := strings.Join(lines, " ")
	if !strings.Contains(joined, "Hello") {
		t.Errorf("LineBreak lost non-CJK word 'Hello': %v", lines)
	}
}

func TestFontBridge_LineBreak_CJKWithBreaking(t *testing.T) {
	fb := newTestBridge()
	font := layout.FontRef{Family: "Helvetica"}

	// Pure CJK text that will need wrapping.
	text := "一二三四五六七八九十"
	lines := fb.LineBreak(font, text, 12, 20) // very narrow
	if len(lines) == 0 {
		t.Fatal("LineBreak should return at least one line for CJK text")
	}
	// Preserve all runes.
	joined := strings.Join(lines, "")
	if joined != text {
		t.Errorf("LineBreak lost CJK characters: got %q, want %q", joined, text)
	}
}

func TestFontBridge_LineBreak_WhitespaceOnly(t *testing.T) {
	fb := newTestBridge()
	font := layout.FontRef{Family: "Helvetica"}

	// Whitespace-only input returns [""].
	lines := fb.LineBreak(font, "   ", 12, 100)
	if len(lines) != 1 || lines[0] != "" {
		t.Errorf("whitespace-only should return [\"\"], got %v", lines)
	}
}

func TestFontBridge_LineBreak_LongWordThatFits(t *testing.T) {
	fb := newTestBridge()
	font := layout.FontRef{Family: "Helvetica"}

	// Single long word that fits.
	lines := fb.LineBreak(font, "Hello", 12, 1000)
	if len(lines) != 1 || lines[0] != "Hello" {
		t.Errorf("single fitting word should return [Hello], got %v", lines)
	}
}

// ============================================================================
// PDFRenderer — drawJustifiedText path (via WordSpacing > 0 in DrawText)
// ============================================================================

func TestRenderDocument_JustifiedText(t *testing.T) {
	cr := creator.New()
	r := NewPDFRenderer(cr, nil)

	font := layout.FontRef{Family: "Helvetica"}
	color := layout.Black

	pages := []layout.PageLayout{
		{
			Size: layout.Size{Width: 595, Height: 842},
			Blocks: []layout.Block{
				{
					X: 50, Y: 50, Width: 400, Height: 20,
					Draw: func(renderer layout.Renderer) {
						// WordSpacing > 0 triggers drawJustifiedText path.
						renderer.DrawText("Hello World PDF", 0, 0, font, 12, color, layout.TextDrawOptions{
							WordSpacing: 5.0,
						})
					},
				},
			},
		},
	}
	err := r.RenderDocument(pages, ExportedMetadata("", ""))
	if err != nil {
		t.Fatalf("RenderDocument with justified text failed: %v", err)
	}
}

func TestRenderDocument_JustifiedText_SingleWord(t *testing.T) {
	cr := creator.New()
	r := NewPDFRenderer(cr, nil)

	font := layout.FontRef{Family: "Helvetica"}
	color := layout.Black

	pages := []layout.PageLayout{
		{
			Size: layout.Size{Width: 595, Height: 842},
			Blocks: []layout.Block{
				{
					X: 50, Y: 50, Width: 400, Height: 20,
					Draw: func(renderer layout.Renderer) {
						// Single word with word spacing — drawJustifiedText with 1 word.
						renderer.DrawText("Hello", 0, 0, font, 12, color, layout.TextDrawOptions{
							WordSpacing: 3.0,
						})
					},
				},
			},
		},
	}
	err := r.RenderDocument(pages, ExportedMetadata("", ""))
	if err != nil {
		t.Fatalf("RenderDocument with single-word justified text failed: %v", err)
	}
}

// ============================================================================
// PDFRenderer — empty text in DrawText (early return branch)
// ============================================================================

func TestRenderDocument_EmptyTextEarlyReturn(t *testing.T) {
	cr := creator.New()
	r := NewPDFRenderer(cr, nil)

	font := layout.FontRef{Family: "Helvetica"}
	color := layout.Black

	pages := []layout.PageLayout{
		{
			Size: layout.Size{Width: 595, Height: 842},
			Blocks: []layout.Block{
				{
					X: 50, Y: 50, Width: 400, Height: 20,
					Draw: func(renderer layout.Renderer) {
						// Empty string → early return in DrawText.
						renderer.DrawText("", 0, 0, font, 12, color, layout.TextDrawOptions{})
					},
				},
			},
		},
	}
	err := r.RenderDocument(pages, ExportedMetadata("", ""))
	if err != nil {
		t.Fatalf("RenderDocument with empty text failed: %v", err)
	}
}

// ============================================================================
// PDFRenderer — DrawRect with no fill/no stroke (nil options)
// ============================================================================

func TestRenderDocument_DrawRect_NilFillAndStroke(t *testing.T) {
	cr := creator.New()
	r := NewPDFRenderer(cr, nil)

	pages := []layout.PageLayout{
		{
			Size: layout.Size{Width: 595, Height: 842},
			Blocks: []layout.Block{
				{
					X: 50, Y: 50, Width: 100, Height: 50,
					Draw: func(renderer layout.Renderer) {
						// nil fill and nil stroke.
						renderer.DrawRect(0, 0, 100, 50, nil, nil, 0)
					},
				},
			},
		},
	}
	err := r.RenderDocument(pages, ExportedMetadata("", ""))
	if err != nil {
		t.Fatalf("RenderDocument with nil fill/stroke failed: %v", err)
	}
}

func TestRenderDocument_DrawLine_DefaultWidth(t *testing.T) {
	cr := creator.New()
	r := NewPDFRenderer(cr, nil)
	color := layout.Black

	pages := []layout.PageLayout{
		{
			Size: layout.Size{Width: 595, Height: 842},
			Blocks: []layout.Block{
				{
					X: 50, Y: 100, Width: 400, Height: 1,
					Draw: func(renderer layout.Renderer) {
						// width == 0 → defaults to 1.0 inside DrawLine.
						renderer.DrawLine(0, 0, 100, 0, color, 0)
					},
				},
			},
		},
	}
	err := r.RenderDocument(pages, ExportedMetadata("", ""))
	if err != nil {
		t.Fatalf("RenderDocument with zero-width line failed: %v", err)
	}
}

// ============================================================================
// FontBridge — Descender path with negative custom font descender
// ============================================================================

func TestFontBridge_Descender_Standard14NonNegative(t *testing.T) {
	fb := newTestBridge()

	fonts := []layout.FontRef{
		{Family: "Helvetica"},
		{Family: "Times"},
		{Family: "Courier"},
	}
	for _, font := range fonts {
		d := fb.Descender(font, 12)
		if d < 0 {
			t.Errorf("Descender(%s, 12) = %f, should always be >= 0", font.Family, d)
		}
	}
}
