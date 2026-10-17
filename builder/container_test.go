package builder

import (
	"bytes"
	"testing"

	"github.com/coregx/gxpdf/layout"
)

// testBuilder creates a minimal Builder suitable for container tests.
func testBuilder() *Builder {
	return NewBuilder()
}

func TestContainer_Text_AddsElement(t *testing.T) {
	b := testBuilder()
	c := newContainer(b)

	c.Text("Hello World")

	if len(c.elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(c.elements))
	}
	text, ok := c.elements[0].(*layout.Text)
	if !ok {
		t.Fatalf("expected *layout.Text, got %T", c.elements[0])
	}
	if text.Content != "Hello World" {
		t.Errorf("text content = %q, want %q", text.Content, "Hello World")
	}
}

func TestContainer_Text_WithOptions(t *testing.T) {
	b := testBuilder()
	c := newContainer(b)

	c.Text("Bold Red", Bold(), TextColor(Red), FontSize(18))

	text := c.elements[0].(*layout.Text)
	if !text.Style.Bold {
		t.Error("expected Bold = true")
	}
	wantColor := Red.toLayout()
	if text.Style.Color != wantColor {
		t.Errorf("expected color = Red (layout), got %v", text.Style.Color)
	}
	if text.Style.FontSize != 18 {
		t.Errorf("expected FontSize = 18, got %f", text.Style.FontSize)
	}
}

func TestContainer_Text_MultipleElements(t *testing.T) {
	b := testBuilder()
	c := newContainer(b)

	c.Text("First")
	c.Text("Second")
	c.Text("Third")

	if len(c.elements) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(c.elements))
	}
}

func TestContainer_Spacer_AddsElement(t *testing.T) {
	b := testBuilder()
	c := newContainer(b)

	c.Spacer(Mm(10))

	if len(c.elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(c.elements))
	}
	spacer, ok := c.elements[0].(*spacerElement)
	if !ok {
		t.Fatalf("expected *spacerElement, got %T", c.elements[0])
	}
	if spacer.height.Amount != 10 || spacer.height.Unit != layout.UnitMm {
		t.Errorf("spacer height = %v, want 10mm", spacer.height)
	}
}

func TestContainer_Line_AddsElement(t *testing.T) {
	b := testBuilder()
	c := newContainer(b)

	c.Line(LineColor(Navy), LineWidth(2))

	if len(c.elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(c.elements))
	}
	line, ok := c.elements[0].(*lineElement)
	if !ok {
		t.Fatalf("expected *lineElement, got %T", c.elements[0])
	}
	wantColor := Navy.toLayout()
	if line.cfg.color == nil || *line.cfg.color != wantColor {
		t.Errorf("line color = %v, want Navy (layout)", line.cfg.color)
	}
	if line.cfg.width != 2 {
		t.Errorf("line width = %f, want 2", line.cfg.width)
	}
}

func TestContainer_PageBreak_AddsElement(t *testing.T) {
	b := testBuilder()
	c := newContainer(b)

	c.PageBreak()

	if len(c.elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(c.elements))
	}
	_, ok := c.elements[0].(*pageBreakElement)
	if !ok {
		t.Fatalf("expected *pageBreakElement, got %T", c.elements[0])
	}
}

func TestContainer_Row_AddsElement(t *testing.T) {
	b := testBuilder()
	c := newContainer(b)

	c.Row(func(r *RowBuilder) {
		r.Col(6, func(col *ColBuilder) { col.Text("Left") })
		r.Col(6, func(col *ColBuilder) { col.Text("Right") })
	})

	if len(c.elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(c.elements))
	}
	box, ok := c.elements[0].(*layout.Box)
	if !ok {
		t.Fatalf("expected *layout.Box, got %T", c.elements[0])
	}
	if box.Direction != layout.Horizontal {
		t.Error("row box should have Horizontal direction")
	}
	if len(box.Children) != 2 {
		t.Errorf("row box should have 2 children, got %d", len(box.Children))
	}
}

func TestContainer_AutoRow_IsEquivalentToRow(t *testing.T) {
	b := testBuilder()
	c1 := newContainer(b)
	c2 := newContainer(b)

	fn := func(r *RowBuilder) {
		r.Col(12, func(col *ColBuilder) { col.Text("Content") })
	}
	c1.Row(fn)
	c2.AutoRow(fn)

	if len(c1.elements) != 1 || len(c2.elements) != 1 {
		t.Fatal("both should have 1 element")
	}
}

func TestContainer_KeepTogether_AddsBoxWithFlag(t *testing.T) {
	b := testBuilder()
	c := newContainer(b)

	c.KeepTogether(func(inner *Container) {
		inner.Text("Title")
		inner.Text("Subtitle")
	})

	if len(c.elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(c.elements))
	}
	box, ok := c.elements[0].(*layout.Box)
	if !ok {
		t.Fatalf("expected *layout.Box, got %T", c.elements[0])
	}
	if !box.Style.KeepTogether {
		t.Error("KeepTogether box should have Style.KeepTogether = true")
	}
	if len(box.Children) != 2 {
		t.Errorf("KeepTogether box should contain 2 children, got %d", len(box.Children))
	}
}

func TestContainer_EnsureSpace_AddsElement(t *testing.T) {
	b := testBuilder()
	c := newContainer(b)

	c.EnsureSpace(Mm(50))

	if len(c.elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(c.elements))
	}
	ens, ok := c.elements[0].(*ensureSpaceElement)
	if !ok {
		t.Fatalf("expected *ensureSpaceElement, got %T", c.elements[0])
	}
	if ens.minHeight.Amount != 50 || ens.minHeight.Unit != layout.UnitMm {
		t.Errorf("ensureSpace.minHeight = %v, want 50mm", ens.minHeight)
	}
}

func TestContainer_PageNumber_AddsElement(t *testing.T) {
	b := testBuilder()
	c := newContainer(b)

	format := layout.PageNumberPlaceholder + " / " + layout.TotalPagesPlaceholder
	c.PageNumber(format, AlignRight(), FontSize(8))

	if len(c.elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(c.elements))
	}
	pn, ok := c.elements[0].(*layout.PageNumber)
	if !ok {
		t.Fatalf("expected *layout.PageNumber, got %T", c.elements[0])
	}
	if pn.Format != format {
		t.Errorf("PageNumber.Format = %q, want %q", pn.Format, format)
	}
	if pn.Style.TextAlign != layout.AlignRight {
		t.Error("PageNumber should have AlignRight applied")
	}
}

func TestContainer_Image_AddsElement(t *testing.T) {
	b := testBuilder()
	c := newContainer(b)

	data := testPNG(t, 200, 100)
	c.Image(data, FitWidth(Mm(60)))

	if len(c.elements) != 1 {
		t.Fatalf("expected 1 element, got %d (errs %v)", len(c.elements), b.errs)
	}
	img, ok := c.elements[0].(*imageElement)
	if !ok {
		t.Fatalf("expected *imageElement, got %T", c.elements[0])
	}
	if !bytes.Equal(img.data, data) {
		t.Error("image data not preserved")
	}
	if img.natW != 200 || img.natH != 100 {
		t.Errorf("intrinsic size %vx%v, want 200x100 from the PNG", img.natW, img.natH)
	}
	if img.cfg.width == nil || img.cfg.width.Amount != 60 {
		t.Errorf("image FitWidth not applied: %v", img.cfg.width)
	}
}

// --- Layout element plan tests ---

func TestPageBreakElement_ReturnsNothing(t *testing.T) {
	e := &pageBreakElement{}
	plan := e.PlanLayout(layout.Area{Width: 500, Height: 800})
	if plan.Status != layout.Nothing {
		t.Errorf("pageBreakElement should return Nothing, got %d", plan.Status)
	}
}

func TestSpacerElement_FitsWithinArea(t *testing.T) {
	e := newSpacerElement(layout.Mm(10))
	area := layout.Area{Width: 400, Height: 200}
	plan := e.PlanLayout(area)

	if plan.Status != layout.Full {
		t.Errorf("spacer that fits should return Full, got %d", plan.Status)
	}
	wantH := layout.Mm(10).Resolve(200, 12)
	if plan.Consumed < wantH-0.1 || plan.Consumed > wantH+0.1 {
		t.Errorf("spacer Consumed = %f, want ~%f", plan.Consumed, wantH)
	}
}

func TestSpacerElement_DoesNotFit(t *testing.T) {
	e := newSpacerElement(layout.Mm(100))
	area := layout.Area{Width: 400, Height: 10}
	plan := e.PlanLayout(area)

	if plan.Status != layout.Nothing {
		t.Errorf("oversized spacer should return Nothing, got %d", plan.Status)
	}
}

func TestEnsureSpaceElement_SufficientSpace(t *testing.T) {
	e := newEnsureSpaceElement(layout.Mm(50))
	area := layout.Area{Width: 400, Height: 200}
	plan := e.PlanLayout(area)

	if plan.Status != layout.Full {
		t.Errorf("EnsureSpace with sufficient height should return Full, got %d", plan.Status)
	}
	if plan.Consumed != 0 {
		t.Errorf("EnsureSpace should consume 0, got %f", plan.Consumed)
	}
}

func TestEnsureSpaceElement_InsufficientSpace(t *testing.T) {
	e := newEnsureSpaceElement(layout.Mm(50))
	area := layout.Area{Width: 400, Height: 10}
	plan := e.PlanLayout(area)

	if plan.Status != layout.Nothing {
		t.Errorf("EnsureSpace with insufficient height should return Nothing, got %d", plan.Status)
	}
}

func TestLineElement_PlanLayout(t *testing.T) {
	cfg := applyLineOptions([]LineOption{LineColor(Black), LineWidth(1)})
	e := newLineElement(cfg)
	area := layout.Area{Width: 400, Height: 200}
	plan := e.PlanLayout(area)

	if plan.Status != layout.Full {
		t.Errorf("line should return Full, got %d", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Error("line should consume some vertical space")
	}
	if len(plan.Blocks) != 1 {
		t.Errorf("line should produce 1 block, got %d", len(plan.Blocks))
	}
	if plan.Blocks[0].Draw == nil {
		t.Error("line block should have a Draw closure")
	}
}
