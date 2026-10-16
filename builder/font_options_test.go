package builder

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testTTF is a TrueType font from the reference corpus; tests that need a
// valid font skip when it is not checked out.
const testTTF = "../reference/krilla/assets/fonts/DejaVuSansMono.ttf"

func textDoc(opts []Option, families ...string) *Builder {
	doc := NewBuilder(append([]Option{WithPageSize(A4)}, opts...)...)
	doc.Page(func(p *PageBuilder) {
		p.Content(func(c *Container) {
			for _, f := range families {
				c.Text(f+" family", FontFamily(f))
			}
		})
	})
	return doc
}

// TestBuild_FontLoadErrors is the #102 repro: fonts that fail to load make
// Build fail with an error naming the family and the cause, instead of a PDF
// in a fallback typeface.
func TestBuild_FontLoadErrors(t *testing.T) {
	doc := textDoc([]Option{
		WithFont("Broken", []byte("this is not a TrueType font")),
		WithFontFile("Missing", "/nonexistent/font.ttf"),
	}, "Broken", "Missing")

	out, err := doc.Build()
	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), `font "Broken": `)
	assert.Contains(t, err.Error(), `font "Missing": `)
	assert.True(t, errors.Is(err, fs.ErrNotExist), "the missing file's cause is kept: %v", err)
}

func TestBuild_FontEmptyData(t *testing.T) {
	_, err := textDoc([]Option{WithFont("Empty", nil)}).Build()
	require.Error(t, err)
	assert.Equal(t, `font "Empty": empty font data`, err.Error())
}

// TestBuild_FontFromBytes verifies a valid font passed as bytes is registered
// and embedded (it used to go through a temporary file).
func TestBuild_FontFromBytes(t *testing.T) {
	data, err := os.ReadFile(testTTF)
	if err != nil {
		t.Skipf("test font not available: %v", err)
	}
	out, err := textDoc([]Option{WithFont("Mono", data)}, "Mono").Build()
	require.NoError(t, err)
	assert.True(t, bytes.Contains(out, []byte("DejaVuSansMono")), "embedded font name in the PDF")
	assert.False(t, strings.Contains(string(out), "/BaseFont /Helvetica"), "no fallback font")
}
