package creator

import (
	"os"
	"testing"

	"github.com/coregx/gxpdf/internal/fonts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFontFromBytes_Invalid(t *testing.T) {
	_, err := LoadFontFromBytes([]byte("not a font"))
	assert.Error(t, err)
	_, err = LoadFontFromBytes(nil)
	assert.Error(t, err)
}

func TestLoadFontFromBytes_MatchesLoadFont(t *testing.T) {
	data, err := os.ReadFile(testFontPath)
	if err != nil {
		t.Skipf("test font not available: %v", err)
	}
	fromBytes, err := LoadFontFromBytes(data)
	require.NoError(t, err)
	fromFile := loadTestFont(t)
	assert.Equal(t, fromFile.PostScriptName(), fromBytes.PostScriptName())
	assert.Equal(t, fromFile.MeasureString("Hello", 12), fromBytes.MeasureString("Hello", 12))
}

// TestCustomFont_IDWithoutNameOrPath verifies fonts loaded from bytes without
// a PostScript name get distinct IDs instead of an empty one.
func TestCustomFont_IDWithoutNameOrPath(t *testing.T) {
	a := &CustomFont{ttfFont: &fonts.TTFFont{}}
	b := &CustomFont{ttfFont: &fonts.TTFFont{}}
	assert.NotEmpty(t, a.ID())
	assert.NotEqual(t, a.ID(), b.ID())
	assert.Equal(t, a.ID(), a.ID(), "stable for one font")
}
