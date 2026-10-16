package layout

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPaginate_PageNumberOnEveryPage verifies page numbers in the page header
// and footer show each page's own number. Header and footer are laid out once
// per page definition and their blocks are copied shallowly to every page, so
// the nested page-number blocks were shared: page 1 resolved them and every
// later page showed "1".
func TestPaginate_PageNumberOnEveryPage(t *testing.T) {
	fonts := &MockFontResolver{}
	format := PageNumberPlaceholder + "/" + TotalPagesPlaceholder
	pn := func() Element { return &PageNumber{Format: format, Fonts: fonts} }

	// Each content element fills most of the body, so there are three pages.
	page := Size{Width: 200, Height: 200}
	fill := elementFunc(func(a Area) Plan {
		if !Fits(150, a.Height) {
			return Plan{Status: Nothing}
		}
		return Plan{Status: Full, Consumed: 150}
	})
	content := []Element{fill, fill, fill}
	pages := (&Paginator{Fonts: fonts}).Paginate([]*PageDef{{
		Size:    page,
		Header:  []Element{pn()},
		Footer:  []Element{&Box{Children: []Element{&Box{Children: []Element{pn()}}}}}, // nested
		Content: content,
	}})
	require.Len(t, pages, 3)

	for i, pg := range pages {
		var texts []string
		for _, d := range capture(pg.Blocks) {
			if d.kind == "text" {
				texts = append(texts, d.text)
			}
		}
		want := []string{
			string(rune('1'+i)) + "/3", // header
			string(rune('1'+i)) + "/3", // footer
		}
		assert.Equal(t, want, texts, "page %d", i+1)
	}
}
