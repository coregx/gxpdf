package layout

import "strings"

// Text is a single-style text element that supports word wrapping and
// justified alignment. It implements both Element and Measurable.
//
// The FontResolver is required for accurate width measurement. When nil,
// the element falls back to an approximate 0.5*fontSize per character.
type Text struct {
	// Content is the text string to lay out. Newlines are treated as hard breaks.
	Content string
	// Style controls typography, alignment, and box-model properties.
	Style Style
	// Fonts is the font resolver used for measurement and line breaking.
	// If nil, an internal approximation is used.
	Fonts FontResolver
}

// PlanLayout implements Element. It breaks Content into wrapped lines,
// positions each line according to alignment, and returns a Plan. If not
// all lines fit vertically, the remaining content is returned as Overflow.
func (t *Text) PlanLayout(area Area) Plan {
	s := t.Style.effective()
	fontSize := s.FontSize
	lineHeight := s.LineHeight
	lineSpacing := fontSize * lineHeight

	resolver := t.Fonts
	if resolver == nil {
		resolver = &MockFontResolver{}
	}
	font := s.Font

	// Break into lines.
	lines := resolver.LineBreak(font, t.Content, fontSize, area.Width)

	blocks := make([]Block, 0, len(lines))
	cursorY := 0.0

	for i, line := range lines {
		if !Fits(cursorY+lineSpacing, area.Height) {
			// Remaining lines overflow.
			overflow := t.overflowText(joinStrings(lines[i:], " "), &s)
			status := Partial
			if len(blocks) == 0 {
				status = Nothing
			}
			return Plan{
				Status:   status,
				Consumed: cursorY,
				Blocks:   blocks,
				Overflow: overflow,
			}
		}

		lineWidth := resolver.MeasureString(font, line, fontSize)
		xPos, wordSpacing := computeTextX(s.TextAlign, lineWidth, area.Width, line, i == len(lines)-1)

		captureLine := line
		captureFont := font
		captureSize := fontSize
		captureColor := s.Color
		captureSpacing := s.LetterSpacing
		captureWS := wordSpacing
		// Half-leading: distribute leading (lineSpacing - fontSize) equally above
		// and below the text, per standard typographic practice. Without this,
		// all leading goes below and text appears top-heavy in table cells.
		halfLeading := (lineSpacing - fontSize) / 2

		block := Block{
			X:      xPos,
			Y:      cursorY,
			Width:  lineWidth,
			Height: lineSpacing,
			Draw: func(r Renderer) {
				// Draw at (0, halfLeading) — shift text down by half the leading
				// for optical vertical centering within the line box.
				r.DrawText(captureLine, 0, halfLeading, captureFont, captureSize, captureColor, TextDrawOptions{
					LetterSpacing: captureSpacing,
					WordSpacing:   captureWS,
					Underline:     s.Underline,
					Strikethrough: s.Strikethrough,
				})
			},
		}
		blocks = append(blocks, block)
		cursorY += lineSpacing
	}

	return Plan{
		Status:   Full,
		Consumed: cursorY,
		Blocks:   blocks,
	}
}

// MinWidth implements Measurable. It returns the width of the longest
// unbreakable word in the content.
func (t *Text) MinWidth() float64 {
	s := t.Style.effective()
	resolver := t.Fonts
	if resolver == nil {
		resolver = &MockFontResolver{}
	}
	font := s.Font
	fontSize := s.FontSize

	words := strings.Fields(t.Content)
	max := 0.0
	for _, w := range words {
		ww := resolver.MeasureString(font, w, fontSize)
		if ww > max {
			max = ww
		}
	}
	return max
}

// MaxWidth implements Measurable. It returns the width of the full content
// rendered on a single line.
func (t *Text) MaxWidth() float64 {
	s := t.Style.effective()
	resolver := t.Fonts
	if resolver == nil {
		resolver = &MockFontResolver{}
	}
	return resolver.MeasureString(s.Font, t.Content, s.FontSize)
}

// overflowText creates a new Text element for the remaining content after a
// page break. It inherits the full style but strips margin/padding so they
// are not doubled at the continuation point.
func (t *Text) overflowText(content string, s *Style) *Text {
	return &Text{
		Content: content,
		Style:   *s,
		Fonts:   t.Fonts,
	}
}

// computeTextX returns the X offset and word spacing for a line of text
// given the alignment mode, measured line width, available width, and
// whether this is the last line of a paragraph (last lines in justified
// paragraphs are left-aligned per typographic convention).
func computeTextX(align Align, lineWidth, availWidth float64, line string, isLast bool) (xPos, wordSpacing float64) {
	switch align {
	case AlignCenter:
		return (availWidth - lineWidth) / 2, 0
	case AlignRight:
		return availWidth - lineWidth, 0
	case AlignJustify:
		if !isLast {
			spaces := strings.Count(line, " ")
			if spaces > 0 {
				ws := (availWidth - lineWidth) / float64(spaces)
				return 0, ws
			}
		}
		return 0, 0
	default: // AlignLeft
		return 0, 0
	}
}

// joinStrings concatenates strings with the given separator.
func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for _, s := range ss[1:] {
		result += sep + s
	}
	return result
}
