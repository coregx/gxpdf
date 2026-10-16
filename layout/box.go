package layout

// Box is the universal container element. It supports two layout modes:
//   - Vertical (default): children are stacked top-to-bottom (CSS block layout)
//   - Horizontal: children are placed left-to-right (CSS row/inline layout)
//
// Box respects margin, padding, and border from its Style. Background and
// border drawing closures are injected into the Block tree so that the
// renderer can emit them without the layout engine knowing about PDF.
type Box struct {
	// Children are the contained elements laid out according to Direction.
	Children []Element
	// Direction controls whether children stack vertically or horizontally.
	Direction Direction
	// Style contains typographic and box-model properties.
	Style Style
	// Width is the explicit width of the box. Auto means use available width.
	Width Value
	// Height is the explicit height of the box. Auto means content-driven.
	Height Value
}

// PlanLayout implements Element. It resolves box-model spacing, delegates
// to vertical or horizontal layout, and returns a Plan with positioned
// Blocks and an optional Overflow element.
func (b *Box) PlanLayout(area Area) Plan {
	s := b.Style.effective()
	fontSize := s.FontSize

	margin := s.Margin.Resolve(area.Width, area.Height, fontSize)
	padding := s.Padding.Resolve(area.Width, area.Height, fontSize)
	bw := s.Border.widths()

	// Resolve explicit width if set.
	// Percentage widths are NOT re-resolved here because the parent's
	// horizontal layout already resolved them and passed the result as
	// area.Width. Re-resolving would double-apply the percentage
	// (e.g., Pct(16.7) of 82pt = 13.7pt instead of using the 82pt directly).
	availWidth := area.Width - margin.Horizontal()
	if !b.Width.IsAuto() && b.Width.Amount > 0 && b.Width.Unit != UnitPct {
		resolved := b.Width.Resolve(area.Width, fontSize)
		if resolved < availWidth {
			availWidth = resolved
		}
	}

	// Inner content width and height after stripping box-model spacing.
	innerWidth := availWidth - padding.Horizontal() - bw.Horizontal()
	innerHeight := area.Height - margin.Vertical() - padding.Vertical() - bw.Vertical()
	if innerWidth < 0 {
		innerWidth = 0
	}
	if innerHeight < 0 {
		innerHeight = 0
	}

	// Resolve explicit height if set.
	explicitHeight := -1.0
	if !b.Height.IsAuto() && b.Height.Amount > 0 {
		explicitHeight = b.Height.Resolve(area.Height, fontSize)
		explicitContent := explicitHeight - padding.Vertical() - bw.Vertical() - margin.Vertical()
		if explicitContent >= 0 {
			innerHeight = explicitContent
		}
	}

	var childBlocks []Block
	var overflow Element
	var consumed float64

	if b.Direction == Horizontal {
		childBlocks, consumed = b.layoutHorizontal(innerWidth, innerHeight, area.AtTop, &s)
		// Horizontal boxes do not split across pages.
	} else {
		var nothing bool
		childBlocks, consumed, overflow, nothing = b.layoutVertical(innerWidth, innerHeight, area.AtTop, &s)
		if nothing {
			return Plan{Status: Nothing}
		}
	}

	// If KeepTogether is set and content overflowed, return Nothing so the
	// parent can push this box to the next page.
	if s.KeepTogether && overflow != nil {
		return Plan{Status: Nothing}
	}

	// Apply explicit height (pad out consumed if needed).
	if explicitHeight > 0 && consumed < explicitHeight-margin.Vertical()-padding.Vertical()-bw.Vertical() {
		consumed = explicitHeight - margin.Vertical() - padding.Vertical() - bw.Vertical()
	}

	totalConsumed := margin.Top + bw.Top + padding.Top + consumed + padding.Bottom + bw.Bottom + margin.Bottom

	// Offset content blocks by margin+border+padding.
	contentOffX := margin.Left + bw.Left + padding.Left
	contentOffY := margin.Top + bw.Top + padding.Top
	offsetBlocks(childBlocks, contentOffX, contentOffY)

	// Build outer block wrapping child blocks, with background and border drawing.
	outerBlock := Block{
		X:        margin.Left,
		Y:        margin.Top,
		Width:    availWidth - margin.Horizontal(),
		Height:   consumed + padding.Vertical() + bw.Vertical(),
		Children: childBlocks,
	}
	// Draw at (0,0) relative to block origin — renderer adds block.X/Y.
	outerBlock.Draw = buildBoxDraw(&s, 0, 0, outerBlock.Width, outerBlock.Height)

	status := Full
	if overflow != nil {
		status = Partial
	}

	return Plan{
		Status:   status,
		Consumed: totalConsumed,
		Blocks:   []Block{outerBlock},
		Overflow: overflow,
	}
}

// layoutVertical stacks children top-to-bottom within the given inner area.
// It returns the placed blocks, consumed height, and an overflow element if
// not all children fit. nothing reports that not even the first child fits
// and the box is not at the top of the page, so the whole box should move to
// the next page. At the top of the page a first child that does not fit is
// placed anyway (forced), since a fresh page cannot offer more room.
func (b *Box) layoutVertical(innerWidth, innerHeight float64, atTop bool, s *Style) (blocks []Block, consumed float64, overflow Element, nothing bool) {
	cursorY := 0.0

	for i, child := range b.Children {
		remaining := innerHeight - cursorY
		if remaining <= 0 && (i > 0 || !atTop) {
			if i == 0 {
				return nil, 0, nil, true
			}
			// No space left — everything from here overflows.
			overflow := b.overflowBox(b.Children[i:], s)
			return blocks, cursorY, overflow, false
		}

		childPlan := child.PlanLayout(Area{Width: innerWidth, Height: remaining, AtTop: atTop && cursorY == 0})

		switch childPlan.Status {
		case Full:
			// Offset child blocks by current cursor and add to list.
			placed := cloneBlocks(childPlan.Blocks)
			offsetBlocks(placed, 0, cursorY)
			blocks = append(blocks, placed...)
			cursorY += childPlan.Consumed

		case Partial:
			// Place what fits, carry overflow.
			placed := cloneBlocks(childPlan.Blocks)
			offsetBlocks(placed, 0, cursorY)
			blocks = append(blocks, placed...)
			cursorY += childPlan.Consumed

			remaining := make([]Element, 0, 1+len(b.Children[i+1:]))
			remaining = append(remaining, childPlan.Overflow)
			remaining = append(remaining, b.Children[i+1:]...)
			overflow := b.overflowBox(remaining, s)
			return blocks, cursorY, overflow, false

		case Nothing:
			if i > 0 || cursorY > 0 {
				// Something is placed already — overflow everything from here.
				overflow := b.overflowBox(b.Children[i:], s)
				return blocks, cursorY, overflow, false
			}
			if !atTop {
				// The first child does not fit here; a fresh page may help.
				return nil, 0, nil, true
			}
			// Nothing fits even at the page top and the child ignored AtTop:
			// force it with unlimited height.
			forced := child.PlanLayout(Area{Width: innerWidth, Height: 1e9, AtTop: true})
			placed := cloneBlocks(forced.Blocks)
			offsetBlocks(placed, 0, cursorY)
			blocks = append(blocks, placed...)
			cursorY += forced.Consumed

			if forced.Overflow != nil {
				remaining := make([]Element, 0, 1+len(b.Children[i+1:]))
				remaining = append(remaining, forced.Overflow)
				remaining = append(remaining, b.Children[i+1:]...)
				overflow := b.overflowBox(remaining, s)
				return blocks, cursorY, overflow, false
			}
		}
	}

	return blocks, cursorY, nil, false
}

// layoutHorizontal places children left-to-right within the inner area.
// Children with an explicit Width are resolved first; remaining children
// share the leftover space equally. Horizontal boxes do not split.
func (b *Box) layoutHorizontal(innerWidth, innerHeight float64, atTop bool, s *Style) ([]Block, float64) {
	if len(b.Children) == 0 {
		return nil, 0
	}

	widths := resolveChildWidths(b.Children, innerWidth, s.FontSize)

	var blocks []Block
	cursorX := 0.0
	maxHeight := 0.0

	for i, child := range b.Children {
		w := widths[i]
		childPlan := child.PlanLayout(Area{Width: w, Height: innerHeight, AtTop: atTop})

		placed := cloneBlocks(childPlan.Blocks)
		offsetBlocks(placed, cursorX, 0)
		blocks = append(blocks, placed...)

		if childPlan.Consumed > maxHeight {
			maxHeight = childPlan.Consumed
		}
		cursorX += w
	}

	return blocks, maxHeight
}

// overflowBox creates a Box carrying the remaining children with the same
// style, to be laid out on the next page.
func (b *Box) overflowBox(children []Element, s *Style) *Box {
	if len(children) == 0 {
		return nil
	}
	return &Box{
		Children:  children,
		Direction: b.Direction,
		Style: Style{
			Padding: s.Padding,
			Border:  s.Border,
			// Margin is intentionally omitted from overflow continuations
			// to avoid double-applying top/bottom margins at page breaks.
		},
	}
}

// resolveChildWidths computes the width for each child in a horizontal box.
// Children with an explicit Width (non-Auto) are resolved first; the
// remaining space is divided equally among Auto-width children.
func resolveChildWidths(children []Element, parentWidth, fontSize float64) []float64 {
	widths := make([]float64, len(children))
	usedWidth := 0.0
	autoCount := 0

	for i, child := range children {
		if box, ok := child.(*Box); ok && !box.Width.IsAuto() && box.Width.Amount > 0 {
			w := box.Width.Resolve(parentWidth, fontSize)
			widths[i] = w
			usedWidth += w
		} else {
			autoCount++
		}
	}

	if autoCount > 0 {
		remaining := parentWidth - usedWidth
		if remaining < 0 {
			remaining = 0
		}
		autoWidth := remaining / float64(autoCount)
		for i, child := range children {
			if box, ok := child.(*Box); ok && !box.Width.IsAuto() && box.Width.Amount > 0 {
				_ = box
				continue
			}
			if widths[i] == 0 {
				_ = child
				widths[i] = autoWidth
			}
		}
	}

	return widths
}

// buildBoxDraw returns a Draw closure that renders the background fill and
// border of a box using the Renderer interface. It captures all required
// data by value so the closure is safe to call later.
func buildBoxDraw(s *Style, x, y, width, height float64) func(Renderer) {
	bg := s.Background
	border := s.Border

	hasBG := bg != nil
	hasBorder := border.Top.Width > 0 || border.Right.Width > 0 ||
		border.Bottom.Width > 0 || border.Left.Width > 0

	if !hasBG && !hasBorder {
		return nil
	}

	return func(r Renderer) {
		if hasBG {
			bgColor := *bg
			r.DrawRect(x, y, width, height, &bgColor, nil, 0)
		}
		if border.Top.Width > 0 {
			c := border.Top.Color
			r.DrawLine(x, y, x+width, y, c, border.Top.Width)
		}
		if border.Right.Width > 0 {
			c := border.Right.Color
			r.DrawLine(x+width, y, x+width, y+height, c, border.Right.Width)
		}
		if border.Bottom.Width > 0 {
			c := border.Bottom.Color
			r.DrawLine(x, y+height, x+width, y+height, c, border.Bottom.Width)
		}
		if border.Left.Width > 0 {
			c := border.Left.Color
			r.DrawLine(x, y, x, y+height, c, border.Left.Width)
		}
	}
}

// offsetBlocks shifts all blocks in the slice by (dx, dy).
func offsetBlocks(blocks []Block, dx, dy float64) {
	for i := range blocks {
		blocks[i].X += dx
		blocks[i].Y += dy
	}
}

// cloneBlocks makes a shallow copy of a block slice so that offsetBlocks
// can be applied without mutating the original plan data.
func cloneBlocks(blocks []Block) []Block {
	if len(blocks) == 0 {
		return nil
	}
	result := make([]Block, len(blocks))
	copy(result, blocks)
	return result
}
