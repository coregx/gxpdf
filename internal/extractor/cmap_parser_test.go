package extractor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCMapTable_Basic(t *testing.T) {
	t.Run("Create empty table", func(t *testing.T) {
		table := NewCMapTable("TestCMap")
		assert.Equal(t, "TestCMap", table.Name())
		assert.Equal(t, 0, table.Size())
	})

	t.Run("Add single mapping", func(t *testing.T) {
		table := NewCMapTable("TestCMap")
		table.AddMapping(0x0001, 'А')

		unicode, ok := table.GetUnicode(0x0001)
		assert.True(t, ok)
		assert.Equal(t, 'А', unicode)
	})

	t.Run("Get nonexistent mapping", func(t *testing.T) {
		table := NewCMapTable("TestCMap")
		unicode, ok := table.GetUnicode(0xFFFF)
		assert.False(t, ok)
		assert.Equal(t, rune(0), unicode)
	})
}

func TestCMapTable_RangeMapping(t *testing.T) {
	t.Run("Add range mapping", func(t *testing.T) {
		table := NewCMapTable("TestCMap")
		// Map glyphs 0x10-0x20 to Unicode U+0430-0x0440 (Cyrillic lowercase)
		table.AddRangeMapping(0x10, 0x20, 0x0430)

		// Check first glyph in range
		unicode, ok := table.GetUnicode(0x10)
		assert.True(t, ok)
		assert.Equal(t, rune(0x0430), unicode) // 'а'

		// Check middle glyph in range
		unicode, ok = table.GetUnicode(0x15)
		assert.True(t, ok)
		assert.Equal(t, rune(0x0435), unicode) // 'е'

		// Check last glyph in range
		unicode, ok = table.GetUnicode(0x20)
		assert.True(t, ok)
		assert.Equal(t, rune(0x0440), unicode) // 'р'
	})

	t.Run("Range with single glyph", func(t *testing.T) {
		table := NewCMapTable("TestCMap")
		table.AddRangeMapping(0x05, 0x05, 'X')

		unicode, ok := table.GetUnicode(0x05)
		assert.True(t, ok)
		assert.Equal(t, 'X', unicode)
	})

	t.Run("Full range 0x0000-0xFFFF does not infinite loop", func(t *testing.T) {
		// Regression test: uint16 wraparound when endGlyphID is 0xFFFF
		// caused infinite loop (65535 + 1 wraps to 0, never exceeds 65535)
		table := NewCMapTable("TestCMap")
		table.AddRangeMapping(0x0000, 0xFFFF, 0x0000)

		// Verify range boundaries
		unicode, ok := table.GetUnicode(0x0000)
		assert.True(t, ok)
		assert.Equal(t, rune(0x0000), unicode)

		unicode, ok = table.GetUnicode(0xFFFF)
		assert.True(t, ok)
		assert.Equal(t, rune(0xFFFF), unicode)

		// Verify full range was mapped
		assert.Equal(t, 65536, table.Size())
	})
}

func TestCMapParser_Bfchar(t *testing.T) {
	t.Run("Parse single bfchar mapping", func(t *testing.T) {
		cmapData := `
/CIDInit /ProcSet findresource begin
begincmap
/CMapName /Test-UCS def
1 beginbfchar
<0001> <0412>
endbfchar
endcmap
`
		parser := NewCMapParser([]byte(cmapData))
		table, err := parser.Parse()
		require.NoError(t, err)
		assert.Equal(t, "Test-UCS", table.Name())

		unicode, ok := table.GetUnicode(0x0001)
		assert.True(t, ok)
		assert.Equal(t, rune(0x0412), unicode) // Cyrillic 'В'
	})

	t.Run("Parse multiple bfchar mappings", func(t *testing.T) {
		cmapData := `
begincmap
5 beginbfchar
<0001> <0412>
<0002> <044B>
<0003> <043F>
<0004> <0438>
<0005> <0441>
endbfchar
endcmap
`
		table, err := ParseCMapStream([]byte(cmapData))
		require.NoError(t, err)
		assert.Equal(t, 5, table.Size())

		// Check all mappings
		expected := map[uint16]rune{
			0x0001: 0x0412, // В
			0x0002: 0x044B, // ы
			0x0003: 0x043F, // п
			0x0004: 0x0438, // и
			0x0005: 0x0441, // с
		}

		for glyphID, expectedUnicode := range expected {
			unicode, ok := table.GetUnicode(glyphID)
			assert.True(t, ok, "Glyph 0x%04X should be mapped", glyphID)
			assert.Equal(t, expectedUnicode, unicode)
		}
	})

	t.Run("Parse 4-byte hex strings", func(t *testing.T) {
		cmapData := `
beginbfchar
<0001> <FEFF>
endbfchar
`
		table, err := ParseCMapStream([]byte(cmapData))
		require.NoError(t, err)

		unicode, ok := table.GetUnicode(0x0001)
		assert.True(t, ok)
		assert.Equal(t, rune(0xFEFF), unicode) // BOM
	})
}

func TestCMapParser_Bfrange(t *testing.T) {
	t.Run("Parse single bfrange", func(t *testing.T) {
		cmapData := `
begincmap
1 beginbfrange
<0010> <0020> <0430>
endbfrange
endcmap
`
		table, err := ParseCMapStream([]byte(cmapData))
		require.NoError(t, err)

		// Check range: 0x10-0x20 → U+0430-0x0440
		unicode, ok := table.GetUnicode(0x10)
		assert.True(t, ok)
		assert.Equal(t, rune(0x0430), unicode) // 'а'

		unicode, ok = table.GetUnicode(0x20)
		assert.True(t, ok)
		assert.Equal(t, rune(0x0440), unicode) // 'р'
	})

	t.Run("Parse multiple bfrange", func(t *testing.T) {
		cmapData := `
begincmap
2 beginbfrange
<0020> <007E> <0020>
<00A0> <00FF> <00A0>
endbfrange
endcmap
`
		table, err := ParseCMapStream([]byte(cmapData))
		require.NoError(t, err)

		// ASCII range (0x20-0x7E maps to itself)
		unicode, ok := table.GetUnicode(0x0041) // 'A'
		assert.True(t, ok)
		assert.Equal(t, rune(0x0041), unicode)

		// Latin-1 supplement (0xA0-0xFF maps to itself)
		unicode, ok = table.GetUnicode(0x00E9) // 'é'
		assert.True(t, ok)
		assert.Equal(t, rune(0x00E9), unicode)
	})
}

func TestCMapParser_Mixed(t *testing.T) {
	t.Run("Parse bfchar and bfrange together", func(t *testing.T) {
		cmapData := `
/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CMapName /Adobe-Identity-UCS def
/CMapType 2 def

3 beginbfchar
<0001> <0412>
<0002> <044B>
<0003> <043F>
endbfchar

2 beginbfrange
<0010> <001F> <0430>
<0020> <002F> <0440>
endbfrange

endcmap
CMapName currentdict /CMap defineresource pop
end
end
`
		table, err := ParseCMapStream([]byte(cmapData))
		require.NoError(t, err)
		assert.Equal(t, "Adobe-Identity-UCS", table.Name())

		// Check bfchar mappings
		unicode, ok := table.GetUnicode(0x0001)
		assert.True(t, ok)
		assert.Equal(t, rune(0x0412), unicode) // В

		// Check bfrange mappings
		unicode, ok = table.GetUnicode(0x0010)
		assert.True(t, ok)
		assert.Equal(t, rune(0x0430), unicode) // а

		unicode, ok = table.GetUnicode(0x0020)
		assert.True(t, ok)
		assert.Equal(t, rune(0x0440), unicode) // р
	})
}

func TestCMapParser_EdgeCases(t *testing.T) {
	t.Run("Empty CMap", func(t *testing.T) {
		cmapData := `
begincmap
endcmap
`
		table, err := ParseCMapStream([]byte(cmapData))
		require.NoError(t, err)
		assert.Equal(t, 0, table.Size())
	})

	t.Run("No begincmap keyword", func(t *testing.T) {
		cmapData := `some random data`
		table, err := ParseCMapStream([]byte(cmapData))
		require.NoError(t, err)
		assert.Equal(t, 0, table.Size())
	})

	t.Run("Malformed hex string", func(t *testing.T) {
		cmapData := `
beginbfchar
<ZZZZ> <0001>
endbfchar
`
		table, err := ParseCMapStream([]byte(cmapData))
		require.NoError(t, err)
		// Malformed mappings should be skipped
		assert.Equal(t, 0, table.Size())
	})

	t.Run("Incomplete bfchar", func(t *testing.T) {
		cmapData := `
beginbfchar
<0001>
endbfchar
`
		table, err := ParseCMapStream([]byte(cmapData))
		// Should not crash, may return error or empty table
		if err == nil {
			assert.GreaterOrEqual(t, table.Size(), 0)
		}
	})

	t.Run("Incomplete bfrange", func(t *testing.T) {
		cmapData := `
beginbfrange
<0001> <0010>
endbfrange
`
		table, err := ParseCMapStream([]byte(cmapData))
		// Should not crash
		if err == nil {
			assert.GreaterOrEqual(t, table.Size(), 0)
		}
	})
}

func TestCMapParser_RealWorld(t *testing.T) {
	t.Run("Cyrillic CMap (realistic)", func(t *testing.T) {
		// Realistic Cyrillic CMap structure (simplified from actual PDFs)
		cmapData := `
begincmap
/CMapName /Adobe-Identity-UCS def
16 beginbfchar
<0001> <0412>
<0002> <044B>
<0003> <043F>
<0004> <0438>
<0005> <0441>
<0006> <043A>
<0007> <0430>
<0008> <043F>
<0009> <043E>
<000A> <0020>
<000B> <0441>
<000C> <0447>
<000D> <0451>
<000E> <0442>
<000F> <0443>
<0010> <0020>
endbfchar
endcmap
`
		table, err := ParseCMapStream([]byte(cmapData))
		require.NoError(t, err)
		assert.Equal(t, 16, table.Size(), "Table should have 16 mappings")

		// Check Cyrillic mappings
		expected := map[uint16]string{
			0x0001: "В", // Cyrillic capital letter VE
			0x0002: "ы", // Cyrillic small letter YERU
			0x0003: "п", // Cyrillic small letter PE
			0x0004: "и", // Cyrillic small letter I
			0x0005: "с", // Cyrillic small letter ES
			0x0006: "к", // Cyrillic small letter KA
			0x0007: "а", // Cyrillic small letter A
		}

		for glyphID, expectedChar := range expected {
			unicode, ok := table.GetUnicode(glyphID)
			assert.True(t, ok, "Glyph 0x%04X should be mapped", glyphID)
			assert.Equal(t, []rune(expectedChar)[0], unicode, "Glyph 0x%04X should map to %s", glyphID, expectedChar)
		}
	})
}

func TestParseHexString(t *testing.T) {
	tests := []struct {
		name     string
		hexStr   string
		expected int
		wantErr  bool
	}{
		{"Single byte", "<01>", 0x01, false},
		{"Two bytes", "<0001>", 0x0001, false},
		{"Four bytes", "<00000001>", 0x00000001, false},
		{"Cyrillic В", "<0412>", 0x0412, false},
		{"Max uint16", "<FFFF>", 0xFFFF, false},
		{"Without brackets", "0001", 0x0001, false},
		{"Empty", "<>", 0, true},
		{"Invalid hex", "<ZZZZ>", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseHexString(tt.hexStr)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestCMapParser_Tokenization(t *testing.T) {
	t.Run("Parse various tokens", func(t *testing.T) {
		data := []byte("/Name <0001> [array] 123 beginbfchar")
		parser := NewCMapParser(data)

		tokens := []string{}
		for {
			token := parser.nextToken()
			if token == "" {
				break
			}
			tokens = append(tokens, token)
		}

		expected := []string{"/Name", "<0001>", "[array]", "123", "beginbfchar"}
		assert.Equal(t, expected, tokens)
	})

	t.Run("Handle whitespace correctly", func(t *testing.T) {
		data := []byte("  \t\n  token1  \r\n  token2  ")
		parser := NewCMapParser(data)

		token1 := parser.nextToken()
		token2 := parser.nextToken()
		token3 := parser.nextToken()

		assert.Equal(t, "token1", token1)
		assert.Equal(t, "token2", token2)
		assert.Equal(t, "", token3)
	})
}

func BenchmarkCMapParser_Parse(b *testing.B) {
	cmapData := `
begincmap
100 beginbfchar
<0001> <0412>
<0002> <044B>
<0003> <043F>
<0004> <0438>
<0005> <0441>
<0006> <043A>
<0007> <0430>
<0008> <043F>
<0009> <043E>
<000A> <0020>
endbfchar
endcmap
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser := NewCMapParser([]byte(cmapData))
		_, _ = parser.Parse()
	}
}

func BenchmarkCMapTable_GetUnicode(b *testing.B) {
	table := NewCMapTable("TestCMap")
	for i := uint16(0); i < 1000; i++ {
		table.AddMapping(i, rune(0x0430+i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = table.GetUnicode(uint16(i % 1000))
	}
}
