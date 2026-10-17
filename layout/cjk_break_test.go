package layout

import (
	"reflect"
	"strings"
	"testing"
)

// textCapture records the strings drawn, in order.
type textCapture struct{ texts []string }

func (r *textCapture) DrawText(s string, _, _ float64, _ FontRef, _ float64, _ Color, _ TextDrawOptions) {
	r.texts = append(r.texts, s)
}
func (r *textCapture) DrawRect(_, _, _, _ float64, _ *Color, _ *Color, _ float64) {}
func (r *textCapture) DrawLine(_, _, _, _ float64, _ Color, _ float64)            {}
func (r *textCapture) DrawImage(_ []byte, _, _, _, _ float64)                     {}
func (r *textCapture) PushState()                                                 {}
func (r *textCapture) PopState()                                                  {}
func (r *textCapture) SetClipRect(_, _, _, _ float64)                             {}

// drawnLines returns the text of each block of plan, one string per line.
func drawnLines(plan Plan) []string {
	lines := make([]string, 0, len(plan.Blocks))
	for _, b := range plan.Blocks {
		var c textCapture
		b.Draw(&c)
		lines = append(lines, strings.Join(c.texts, ""))
	}
	return lines
}

// runeBreaker is a FontResolver that breaks text after every character, as a
// resolver breaking CJK text does.
type runeBreaker struct{ MockFontResolver }

func (*runeBreaker) LineBreak(_ FontRef, text string, _, _ float64) []string {
	var lines []string
	for _, r := range strings.Join(strings.Fields(text), "") {
		lines = append(lines, string(r))
	}
	return lines
}

func TestRemainingContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		lines   []string
		i       int
		want    string
	}{
		{"space break", "one two  three", []string{"one two", "three"}, 1, "three"},
		{"collapsed whitespace", "one\n\ttwo three", []string{"one two", "three"}, 1, "three"},
		{"break inside CJK word", "ab 一二三四", []string{"ab 一二", "三四"}, 1, "三四"},
		{"oversized word split", "superlongword end", []string{"super", "long", "word end"}, 1, "longword end"},
		{"nothing shown", "  text", []string{"text"}, 0, "text"},
		// Lines that do not match the content fall back to joining them.
		{"hyphenating resolver", "hyphen ok", []string{"hy-", "phen ok"}, 1, "phen ok"},
		{"hyphenating resolver, more lines", "hyphenation", []string{"hy-", "phen-", "ation"}, 1, "phen- ation"},
	}
	for _, tt := range tests {
		if got := remainingContent(tt.content, tt.lines, tt.i); got != tt.want {
			t.Errorf("%s: remainingContent = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// TestText_OverflowKeepsCJKWithoutSpaces checks that text continued on the
// next page is the rest of the content: lines broken inside a word used to be
// joined with spaces, adding a space at every CJK line break.
func TestText_OverflowKeepsCJKWithoutSpaces(t *testing.T) {
	text := &Text{Content: "中文排版", Style: Style{FontSize: 10, LineHeight: 1}, Fonts: &runeBreaker{}}
	plan := text.PlanLayout(Area{Width: 100, Height: 20})
	if plan.Status != Partial {
		t.Fatalf("status = %v, want Partial", plan.Status)
	}
	if got := plan.Overflow.(*Text).Content; got != "排版" {
		t.Errorf("overflow content = %q, want %q", got, "排版")
	}
}

// TestText_MinWidthCJK checks that CJK text can be as narrow as its widest
// unbreakable piece, not its whole length.
func TestText_MinWidthCJK(t *testing.T) {
	text := &Text{Content: "日本語の文章。", Style: Style{FontSize: 10}}
	// MockFontResolver: 5pt per character; "章。" stays together.
	if got := text.MinWidth(); got != 10 {
		t.Errorf("MinWidth = %v, want 10", got)
	}
}

// TestRichText_WrapsCJK checks that a CJK word wraps between characters
// instead of running past the line.
func TestRichText_WrapsCJK(t *testing.T) {
	rt := &RichText{
		Fragments: []RichTextFragment{
			{Text: "GxPDF ", Style: Style{FontSize: 10}},
			{Text: "一二三四五。", Style: Style{FontSize: 10, Bold: true}},
		},
		LineHeight: 1,
	}
	// MockFontResolver: 5pt per character, so 6 characters per line.
	plan := rt.PlanLayout(Area{Width: 30, Height: 100})
	want := []string{"GxPDF", "一二三四五。"}
	if got := drawnLines(plan); !reflect.DeepEqual(got, want) {
		t.Errorf("lines = %q, want %q", got, want)
	}

	plan = rt.PlanLayout(Area{Width: 15, Height: 100})
	// A Latin word is not split; it runs past the narrow line.
	want = []string{"GxPDF", "一二三", "四五。"}
	if got := drawnLines(plan); !reflect.DeepEqual(got, want) {
		t.Errorf("narrow: lines = %q, want %q", got, want)
	}

	if got := rt.MinWidth(); got != 25 {
		t.Errorf("MinWidth = %v, want 25 (the word GxPDF)", got)
	}
}

// TestRichText_OverflowKeepsCJKWithoutSpaces checks that the content continued
// on the next page has no space where a CJK line broke.
func TestRichText_OverflowKeepsCJKWithoutSpaces(t *testing.T) {
	rt := &RichText{
		Fragments:  []RichTextFragment{{Text: "一二三四五六 七八", Style: Style{FontSize: 10}}},
		LineHeight: 1,
	}
	plan := rt.PlanLayout(Area{Width: 15, Height: 10})
	if plan.Status != Partial {
		t.Fatalf("status = %v, want Partial", plan.Status)
	}
	var parts []string
	for _, f := range plan.Overflow.(*RichText).Fragments {
		parts = append(parts, f.Text)
	}
	if got, want := strings.Join(parts, ""), "四五六 七八"; got != want {
		t.Errorf("overflow text = %q, want %q", got, want)
	}
}
