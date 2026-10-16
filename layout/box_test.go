package layout

import (
	"testing"
)

// fixedText creates a Text element with known mock font metrics for predictable tests.
func fixedText(content string, fontSize float64) *Text {
	return &Text{
		Content: content,
		Style:   Style{FontSize: fontSize, LineHeight: 1.0},
		Fonts:   &MockFontResolver{},
	}
}

// ------- Vertical Box tests -------

func TestBoxVertical_AllChildrenFit(t *testing.T) {
	// 3 children, each consuming 20pt height (fontSize=20, lineHeight=1.0).
	// Area height = 100, so all should fit.
	box := &Box{
		Children: []Element{
			fixedText("Line1", 20),
			fixedText("Line2", 20),
			fixedText("Line3", 20),
		},
		Direction: Vertical,
	}

	plan := box.PlanLayout(Area{Width: 200, Height: 100})

	if plan.Status != Full {
		t.Errorf("status: got %v, want Full", plan.Status)
	}
	if plan.Overflow != nil {
		t.Errorf("expected no overflow, got %v", plan.Overflow)
	}
	// Consumed = 3 * 20 = 60
	if plan.Consumed != 60 {
		t.Errorf("consumed: got %v, want 60", plan.Consumed)
	}
}

func TestBoxVertical_Overflow(t *testing.T) {
	// 5 children at 20pt each = 100pt total.
	// Area height = 70pt → first 3 fit (60pt), child 4 triggers overflow.
	children := make([]Element, 5)
	for i := range children {
		children[i] = fixedText("item", 20)
	}
	box := &Box{Children: children, Direction: Vertical}

	plan := box.PlanLayout(Area{Width: 200, Height: 70})

	if plan.Status != Partial {
		t.Errorf("status: got %v, want Partial", plan.Status)
	}
	if plan.Overflow == nil {
		t.Fatal("expected overflow, got nil")
	}
	if plan.Consumed > 70 {
		t.Errorf("consumed %v exceeds available height 70", plan.Consumed)
	}
}

func TestBoxVertical_NothingAtPageTop(t *testing.T) {
	// A box with height 200 that does not fit in area height 10.
	// At page top (cursor=0), should force layout.
	big := &Box{
		Children: []Element{fixedText("big", 200)},
	}
	plan := big.PlanLayout(Area{Width: 200, Height: 10, AtTop: true})
	// Forced layout should return Full or Partial, never Nothing.
	if plan.Status == Nothing {
		t.Errorf("should not return Nothing when forced at page top")
	}
}

func TestBoxVertical_NothingMidPageMovesToNextPage(t *testing.T) {
	// The same box below other content on the page: a fresh page may have
	// room, so the box must ask to move instead of running off the page.
	big := &Box{
		Children: []Element{fixedText("big", 200)},
	}
	plan := big.PlanLayout(Area{Width: 200, Height: 10})
	if plan.Status != Nothing {
		t.Errorf("status: got %v, want Nothing (box must move to the next page)", plan.Status)
	}
}

func TestPaginator_BoxNearPageBottomMovesToNextPage(t *testing.T) {
	// Page body 100pt: a 95pt box, then a box whose first line (14.4pt) does
	// not fit in the remaining 5pt. It used to be forced onto page 1 and
	// drawn down to y=109.4, below the page.
	filler := &Box{Height: Pt(95)}
	box := &Box{Children: []Element{&Text{Content: "box text", Fonts: &MockFontResolver{}}}}
	pages := (&Paginator{}).Paginate([]*PageDef{{
		Size:    Size{Width: 200, Height: 100},
		Content: []Element{filler, box},
	}})
	if len(pages) != 2 {
		t.Fatalf("pages: got %d, want 2", len(pages))
	}
	for i, pg := range pages {
		for _, b := range pg.Blocks {
			if b.Y+b.Height > 100+FitTolerance {
				t.Errorf("page %d: block bottom %.1f below the page", i+1, b.Y+b.Height)
			}
		}
	}
}

func TestBoxVertical_KeepTogether(t *testing.T) {
	// A KeepTogether box that does not fit should return Nothing.
	box := &Box{
		Children: []Element{
			fixedText("Line1", 20),
			fixedText("Line2", 20),
			fixedText("Line3", 20),
		},
		Style: Style{KeepTogether: true},
	}
	// 3 * 20 = 60pt needed, only 50 available.
	plan := box.PlanLayout(Area{Width: 200, Height: 50})
	if plan.Status != Nothing {
		t.Errorf("KeepTogether: expected Nothing, got %v", plan.Status)
	}
}

func TestBoxVertical_KeepTogetherFits(t *testing.T) {
	// A KeepTogether box that fits should return Full.
	box := &Box{
		Children: []Element{
			fixedText("Line1", 20),
			fixedText("Line2", 20),
		},
		Style: Style{KeepTogether: true},
	}
	plan := box.PlanLayout(Area{Width: 200, Height: 100})
	if plan.Status != Full {
		t.Errorf("KeepTogether that fits: expected Full, got %v", plan.Status)
	}
}

func TestBoxVertical_EmptyChildren(t *testing.T) {
	box := &Box{Children: nil, Direction: Vertical}
	plan := box.PlanLayout(Area{Width: 200, Height: 200})
	if plan.Status != Full {
		t.Errorf("empty box: expected Full, got %v", plan.Status)
	}
	if plan.Consumed != 0 {
		t.Errorf("empty box consumed: got %v, want 0", plan.Consumed)
	}
}

func TestBoxVertical_WithPadding(t *testing.T) {
	// Box with 10pt top+bottom padding. Content = 1 child at 20pt.
	// Total consumed = 10 + 20 + 10 = 40pt.
	box := &Box{
		Children: []Element{fixedText("text", 20)},
		Style: Style{
			Padding:    UniformEdges(Pt(10)),
			FontSize:   12,
			LineHeight: 1.0,
		},
	}
	plan := box.PlanLayout(Area{Width: 200, Height: 200})
	if plan.Status != Full {
		t.Errorf("status: got %v, want Full", plan.Status)
	}
	wantConsumed := 10.0 + 20.0 + 10.0 // top padding + child + bottom padding
	if plan.Consumed != wantConsumed {
		t.Errorf("consumed: got %v, want %v", plan.Consumed, wantConsumed)
	}
}

func TestBoxVertical_WithMargin(t *testing.T) {
	// Box with 5pt uniform margin and a single 20pt child.
	// Total consumed = 5 + 20 + 5 = 30pt.
	box := &Box{
		Children: []Element{fixedText("text", 20)},
		Style: Style{
			Margin:     UniformEdges(Pt(5)),
			FontSize:   12,
			LineHeight: 1.0,
		},
	}
	plan := box.PlanLayout(Area{Width: 200, Height: 200})
	if plan.Consumed != 30 {
		t.Errorf("consumed: got %v, want 30", plan.Consumed)
	}
}

func TestBoxVertical_OverflowPreservesRemainingChildren(t *testing.T) {
	// 3 children at 30pt each = 90pt. Area = 50pt → first child fits (30pt),
	// second triggers overflow. The overflow box must contain children 2 and 3.
	box := &Box{
		Children: []Element{
			fixedText("a", 30),
			fixedText("b", 30),
			fixedText("c", 30),
		},
	}
	plan := box.PlanLayout(Area{Width: 200, Height: 50})
	if plan.Status != Partial {
		t.Fatalf("expected Partial, got %v", plan.Status)
	}
	if plan.Overflow == nil {
		t.Fatal("expected overflow")
	}
	// Overflow should be a Box containing children b and c.
	overflowBox, ok := plan.Overflow.(*Box)
	if !ok {
		t.Fatalf("expected overflow to be *Box, got %T", plan.Overflow)
	}
	if len(overflowBox.Children) != 2 {
		t.Errorf("overflow children: got %d, want 2", len(overflowBox.Children))
	}
}

// ------- Horizontal Box tests -------

func TestBoxHorizontal_EqualWidthSplit(t *testing.T) {
	// 3 auto-width children in 300pt-wide area → each gets 100pt.
	box := &Box{
		Children: []Element{
			fixedText("A", 12),
			fixedText("B", 12),
			fixedText("C", 12),
		},
		Direction: Horizontal,
	}
	plan := box.PlanLayout(Area{Width: 300, Height: 200})
	if plan.Status != Full {
		t.Errorf("status: got %v, want Full", plan.Status)
	}
	if len(plan.Blocks) == 0 {
		t.Error("expected blocks, got none")
	}
}

func TestBoxHorizontal_ExplicitWidthChildren(t *testing.T) {
	// Two children: first has explicit 100pt width, second is auto (takes remaining 200pt).
	first := &Box{
		Children: []Element{fixedText("fixed", 12)},
		Width:    Pt(100),
	}
	second := &Box{
		Children: []Element{fixedText("auto", 12)},
	}
	box := &Box{
		Children:  []Element{first, second},
		Direction: Horizontal,
	}
	plan := box.PlanLayout(Area{Width: 300, Height: 200})
	if plan.Status != Full {
		t.Errorf("status: got %v, want Full", plan.Status)
	}
}

func TestBoxHorizontal_NoChildren(t *testing.T) {
	box := &Box{Direction: Horizontal}
	plan := box.PlanLayout(Area{Width: 200, Height: 200})
	if plan.Status != Full {
		t.Errorf("expected Full, got %v", plan.Status)
	}
}

// ------- Block coordinate tests -------

func TestBoxVertical_BlocksOffset(t *testing.T) {
	// Children should be positioned with increasing Y offsets.
	box := &Box{
		Children: []Element{
			fixedText("first", 20),
			fixedText("second", 20),
		},
	}
	plan := box.PlanLayout(Area{Width: 200, Height: 200})
	if plan.Status != Full {
		t.Fatalf("expected Full, got %v", plan.Status)
	}
	// Should have at least 1 outer block.
	if len(plan.Blocks) == 0 {
		t.Fatal("expected blocks")
	}
}

// ------- Background/border draw tests -------

func TestBoxBackgroundDrawClosure(t *testing.T) {
	bg := Color{R: 1, G: 0, B: 0}
	box := &Box{
		Children: []Element{fixedText("text", 12)},
		Style:    Style{Background: &bg},
	}
	plan := box.PlanLayout(Area{Width: 200, Height: 200})
	if len(plan.Blocks) == 0 {
		t.Fatal("expected blocks")
	}
	// The outer block should have a non-nil Draw closure.
	if plan.Blocks[0].Draw == nil {
		t.Error("expected Draw closure for box with background")
	}
}

func TestBoxNilDrawClosure_NoStyle(t *testing.T) {
	box := &Box{
		Children: []Element{fixedText("text", 12)},
	}
	plan := box.PlanLayout(Area{Width: 200, Height: 200})
	if len(plan.Blocks) == 0 {
		t.Fatal("expected blocks")
	}
	// No background or border → Draw should be nil.
	if plan.Blocks[0].Draw != nil {
		t.Error("expected nil Draw closure for plain box")
	}
}

// ------- ExplicitHeight tests -------

func TestBoxExplicitHeight(t *testing.T) {
	// Box with explicit height 100pt, single 20pt child.
	// Consumed should be at least 100pt.
	box := &Box{
		Children: []Element{fixedText("text", 20)},
		Height:   Pt(100),
	}
	plan := box.PlanLayout(Area{Width: 200, Height: 200})
	if plan.Consumed < 100 {
		t.Errorf("explicit height: consumed %v should be >= 100", plan.Consumed)
	}
}

// ------- resolveChildWidths unit tests -------

func TestResolveChildWidths_AllAuto(t *testing.T) {
	children := []Element{
		&Box{},
		&Box{},
		&Box{},
	}
	widths := resolveChildWidths(children, 300, 12)
	for i, w := range widths {
		if w != 100 {
			t.Errorf("widths[%d] = %v, want 100", i, w)
		}
	}
}

func TestResolveChildWidths_MixedExplicitAuto(t *testing.T) {
	children := []Element{
		&Box{Width: Pt(120)}, // explicit 120
		&Box{},               // auto
		&Box{},               // auto
	}
	widths := resolveChildWidths(children, 300, 12)
	if widths[0] != 120 {
		t.Errorf("explicit child: got %v, want 120", widths[0])
	}
	// Remaining 180pt split between 2 auto children = 90 each.
	if widths[1] != 90 || widths[2] != 90 {
		t.Errorf("auto children: got %v and %v, want 90 each", widths[1], widths[2])
	}
}
