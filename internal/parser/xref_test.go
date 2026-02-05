package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testStartXRef = "startxref"

// ============================================================================
// XRefEntry Tests
// ============================================================================

func TestNewXRefEntry(t *testing.T) {
	tests := []struct {
		name       string
		objectNum  int
		entryType  XRefEntryType
		offset     int64
		generation int
	}{
		{
			name:       "in-use entry",
			objectNum:  1,
			entryType:  XRefEntryInUse,
			offset:     15,
			generation: 0,
		},
		{
			name:       "free entry",
			objectNum:  0,
			entryType:  XRefEntryFree,
			offset:     0,
			generation: 65535,
		},
		{
			name:       "compressed entry",
			objectNum:  5,
			entryType:  XRefEntryCompressed,
			offset:     100,
			generation: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := NewXRefEntry(tt.objectNum, tt.entryType, tt.offset, tt.generation)

			assert.NotNil(t, entry)
			assert.Equal(t, tt.objectNum, entry.ObjectNum)
			assert.Equal(t, tt.entryType, entry.Type)
			assert.Equal(t, tt.offset, entry.Offset)
			assert.Equal(t, tt.generation, entry.Generation)
		})
	}
}

func TestXRefEntry_String(t *testing.T) {
	tests := []struct {
		name     string
		entry    *XRefEntry
		expected string
	}{
		{
			name:     "in-use entry",
			entry:    NewXRefEntry(1, XRefEntryInUse, 15, 0),
			expected: "0000000015 00000 n",
		},
		{
			name:     "free entry",
			entry:    NewXRefEntry(0, XRefEntryFree, 0, 65535),
			expected: "0000000000 65535 f",
		},
		{
			name:     "large offset",
			entry:    NewXRefEntry(10, XRefEntryInUse, 123456789, 0),
			expected: "0123456789 00000 n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.entry.String())
		})
	}
}

func TestXRefEntry_IsFree(t *testing.T) {
	freeEntry := NewXRefEntry(0, XRefEntryFree, 0, 65535)
	inUseEntry := NewXRefEntry(1, XRefEntryInUse, 15, 0)

	assert.True(t, freeEntry.IsFree())
	assert.False(t, inUseEntry.IsFree())
}

func TestXRefEntry_IsInUse(t *testing.T) {
	freeEntry := NewXRefEntry(0, XRefEntryFree, 0, 65535)
	inUseEntry := NewXRefEntry(1, XRefEntryInUse, 15, 0)

	assert.False(t, freeEntry.IsInUse())
	assert.True(t, inUseEntry.IsInUse())
}

func TestXRefEntryType_String(t *testing.T) {
	tests := []struct {
		name      string
		entryType XRefEntryType
		expected  string
	}{
		{"free", XRefEntryFree, "free"},
		{"in-use", XRefEntryInUse, "in-use"},
		{"compressed", XRefEntryCompressed, "compressed"},
		{"unknown", XRefEntryType(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.entryType.String())
		})
	}
}

// ============================================================================
// XRefTable Tests
// ============================================================================

func TestNewXRefTable(t *testing.T) {
	table := NewXRefTable()

	assert.NotNil(t, table)
	assert.NotNil(t, table.Entries)
	assert.NotNil(t, table.Trailer)
	assert.Equal(t, 0, table.Size())
}

func TestXRefTable_AddEntry(t *testing.T) {
	table := NewXRefTable()
	entry := NewXRefEntry(1, XRefEntryInUse, 15, 0)

	table.AddEntry(entry)

	assert.Equal(t, 1, table.Size())
	assert.True(t, table.HasObject(1))
}

func TestXRefTable_AddEntry_Nil(t *testing.T) {
	table := NewXRefTable()
	table.AddEntry(nil)

	assert.Equal(t, 0, table.Size())
}

func TestXRefTable_GetEntry(t *testing.T) {
	table := NewXRefTable()
	entry := NewXRefEntry(1, XRefEntryInUse, 15, 0)
	table.AddEntry(entry)

	retrieved, exists := table.GetEntry(1)
	assert.True(t, exists)
	assert.Equal(t, entry, retrieved)

	_, exists = table.GetEntry(999)
	assert.False(t, exists)
}

func TestXRefTable_HasObject(t *testing.T) {
	table := NewXRefTable()
	entry := NewXRefEntry(1, XRefEntryInUse, 15, 0)
	table.AddEntry(entry)

	assert.True(t, table.HasObject(1))
	assert.False(t, table.HasObject(999))
}

func TestXRefTable_GetInUseEntries(t *testing.T) {
	table := NewXRefTable()
	table.AddEntry(NewXRefEntry(0, XRefEntryFree, 0, 65535))
	table.AddEntry(NewXRefEntry(1, XRefEntryInUse, 15, 0))
	table.AddEntry(NewXRefEntry(2, XRefEntryInUse, 79, 0))
	table.AddEntry(NewXRefEntry(3, XRefEntryFree, 0, 0))

	inUseEntries := table.GetInUseEntries()
	assert.Equal(t, 2, len(inUseEntries))
}

func TestXRefTable_GetFreeEntries(t *testing.T) {
	table := NewXRefTable()
	table.AddEntry(NewXRefEntry(0, XRefEntryFree, 0, 65535))
	table.AddEntry(NewXRefEntry(1, XRefEntryInUse, 15, 0))
	table.AddEntry(NewXRefEntry(2, XRefEntryInUse, 79, 0))
	table.AddEntry(NewXRefEntry(3, XRefEntryFree, 0, 0))

	freeEntries := table.GetFreeEntries()
	assert.Equal(t, 2, len(freeEntries))
}

func TestXRefTable_String(t *testing.T) {
	table := NewXRefTable()
	table.AddEntry(NewXRefEntry(1, XRefEntryInUse, 15, 0))

	str := table.String()
	assert.Contains(t, str, "XRefTable")
	assert.Contains(t, str, "entries: 1")
}

// ============================================================================
// MergeOlder Tests
// ============================================================================

func TestXRefTable_MergeOlder_NewerWins(t *testing.T) {
	newer := NewXRefTable()
	newer.AddEntry(NewXRefEntry(1, XRefEntryInUse, 100, 0))
	newer.AddEntry(NewXRefEntry(2, XRefEntryInUse, 200, 0))

	older := NewXRefTable()
	older.AddEntry(NewXRefEntry(1, XRefEntryInUse, 999, 0)) // conflict: should be ignored
	older.AddEntry(NewXRefEntry(2, XRefEntryInUse, 888, 0)) // conflict: should be ignored

	newer.MergeOlder(older)

	assert.Equal(t, 2, newer.Size())

	entry1, _ := newer.GetEntry(1)
	assert.Equal(t, int64(100), entry1.Offset, "newer entry should be preserved")

	entry2, _ := newer.GetEntry(2)
	assert.Equal(t, int64(200), entry2.Offset, "newer entry should be preserved")
}

func TestXRefTable_MergeOlder_GapFill(t *testing.T) {
	newer := NewXRefTable()
	newer.AddEntry(NewXRefEntry(1, XRefEntryInUse, 100, 0))

	older := NewXRefTable()
	older.AddEntry(NewXRefEntry(2, XRefEntryInUse, 200, 0))
	older.AddEntry(NewXRefEntry(3, XRefEntryInUse, 300, 0))

	newer.MergeOlder(older)

	assert.Equal(t, 3, newer.Size())
	assert.True(t, newer.HasObject(1))
	assert.True(t, newer.HasObject(2))
	assert.True(t, newer.HasObject(3))

	entry2, _ := newer.GetEntry(2)
	assert.Equal(t, int64(200), entry2.Offset)
}

func TestXRefTable_MergeOlder_NilSafety(t *testing.T) {
	table := NewXRefTable()
	table.AddEntry(NewXRefEntry(1, XRefEntryInUse, 100, 0))

	// Should not panic
	table.MergeOlder(nil)
	assert.Equal(t, 1, table.Size())
}

func TestXRefTable_MergeOlder_EmptyTables(t *testing.T) {
	t.Run("merge empty into empty", func(t *testing.T) {
		newer := NewXRefTable()
		older := NewXRefTable()
		newer.MergeOlder(older)
		assert.Equal(t, 0, newer.Size())
	})

	t.Run("merge non-empty into empty", func(t *testing.T) {
		newer := NewXRefTable()
		older := NewXRefTable()
		older.AddEntry(NewXRefEntry(1, XRefEntryInUse, 100, 0))
		newer.MergeOlder(older)
		assert.Equal(t, 1, newer.Size())
	})

	t.Run("merge empty into non-empty", func(t *testing.T) {
		newer := NewXRefTable()
		newer.AddEntry(NewXRefEntry(1, XRefEntryInUse, 100, 0))
		older := NewXRefTable()
		newer.MergeOlder(older)
		assert.Equal(t, 1, newer.Size())
	})
}

func TestXRefTable_MergeOlder_FreeVsInUse(t *testing.T) {
	// Newer says obj 1 is free; older says in-use. Newer wins.
	newer := NewXRefTable()
	newer.AddEntry(NewXRefEntry(1, XRefEntryFree, 0, 1))

	older := NewXRefTable()
	older.AddEntry(NewXRefEntry(1, XRefEntryInUse, 100, 0))

	newer.MergeOlder(older)

	entry, _ := newer.GetEntry(1)
	assert.Equal(t, XRefEntryFree, entry.Type, "newer free entry should win over older in-use")
}

func TestXRefTable_MergeOlder_CompressedEntries(t *testing.T) {
	newer := NewXRefTable()
	newer.AddEntry(NewXRefEntry(1, XRefEntryCompressed, 42, 0)) // obj stream 42, index 0

	older := NewXRefTable()
	older.AddEntry(NewXRefEntry(1, XRefEntryInUse, 500, 0)) // traditional entry
	older.AddEntry(NewXRefEntry(5, XRefEntryCompressed, 42, 2))

	newer.MergeOlder(older)

	assert.Equal(t, 2, newer.Size())

	entry1, _ := newer.GetEntry(1)
	assert.Equal(t, XRefEntryCompressed, entry1.Type, "newer compressed entry should be preserved")

	entry5, _ := newer.GetEntry(5)
	assert.Equal(t, XRefEntryCompressed, entry5.Type, "older compressed entry should be added")
}

// ============================================================================
// ParseXRef Tests
// ============================================================================

func TestParser_ParseXRef_Simple(t *testing.T) {
	input := `xref
0 6
0000000000 65535 f
0000000015 00000 n
0000000079 00000 n
0000000173 00000 n
0000000301 00000 n
0000000380 00000 n
trailer
<< /Size 6 /Root 1 0 R >>`

	p := NewParser(strings.NewReader(input))
	table, err := p.ParseXRef()

	require.NoError(t, err)
	require.NotNil(t, table)

	// Check size
	assert.Equal(t, 6, table.Size())

	// Check first entry (free)
	entry0, exists := table.GetEntry(0)
	require.True(t, exists)
	assert.True(t, entry0.IsFree())
	assert.Equal(t, int64(0), entry0.Offset)
	assert.Equal(t, 65535, entry0.Generation)

	// Check second entry (in-use)
	entry1, exists := table.GetEntry(1)
	require.True(t, exists)
	assert.True(t, entry1.IsInUse())
	assert.Equal(t, int64(15), entry1.Offset)
	assert.Equal(t, 0, entry1.Generation)

	// Check last entry
	entry5, exists := table.GetEntry(5)
	require.True(t, exists)
	assert.True(t, entry5.IsInUse())
	assert.Equal(t, int64(380), entry5.Offset)

	// Check trailer
	trailer := table.GetTrailer()
	require.NotNil(t, trailer)
	assert.Equal(t, int64(6), trailer.GetInteger("Size"))
}

func TestParser_ParseXRef_MultipleSubsections(t *testing.T) {
	input := `xref
0 1
0000000000 65535 f
3 2
0000000015 00000 n
0000000079 00000 n
trailer
<< /Size 5 >>`

	p := NewParser(strings.NewReader(input))
	table, err := p.ParseXRef()

	require.NoError(t, err)
	require.NotNil(t, table)

	// Should have 3 entries total
	assert.Equal(t, 3, table.Size())

	// Check entries exist in correct locations
	assert.True(t, table.HasObject(0))
	assert.False(t, table.HasObject(1))
	assert.False(t, table.HasObject(2))
	assert.True(t, table.HasObject(3))
	assert.True(t, table.HasObject(4))

	// Check entry 3
	entry3, _ := table.GetEntry(3)
	assert.Equal(t, int64(15), entry3.Offset)

	// Check entry 4
	entry4, _ := table.GetEntry(4)
	assert.Equal(t, int64(79), entry4.Offset)
}

func TestParser_ParseXRef_LargeObjectNumbers(t *testing.T) {
	input := `xref
1000 2
0000000100 00000 n
0000000200 00000 n
trailer
<< /Size 1002 >>`

	p := NewParser(strings.NewReader(input))
	table, err := p.ParseXRef()

	require.NoError(t, err)
	assert.Equal(t, 2, table.Size())
	assert.True(t, table.HasObject(1000))
	assert.True(t, table.HasObject(1001))
}

func TestParser_ParseXRef_WithGenerations(t *testing.T) {
	input := `xref
0 3
0000000000 65535 f
0000000015 00001 n
0000000079 00002 n
trailer
<< /Size 3 >>`

	p := NewParser(strings.NewReader(input))
	table, err := p.ParseXRef()

	require.NoError(t, err)

	entry1, _ := table.GetEntry(1)
	assert.Equal(t, 1, entry1.Generation)

	entry2, _ := table.GetEntry(2)
	assert.Equal(t, 2, entry2.Generation)
}

func TestParser_ParseXRef_ComplexTrailer(t *testing.T) {
	input := `xref
0 1
0000000000 65535 f
trailer
<< /Size 1 /Root 1 0 R /Info 2 0 R /ID [(abc)(def)] >>`

	p := NewParser(strings.NewReader(input))
	table, err := p.ParseXRef()

	require.NoError(t, err)

	trailer := table.GetTrailer()
	assert.Equal(t, int64(1), trailer.GetInteger("Size"))
	assert.NotNil(t, trailer.Get("Root"))
	assert.NotNil(t, trailer.Get("Info"))
	assert.NotNil(t, trailer.Get("ID"))
}

// ============================================================================
// ParseXRef Error Tests
// ============================================================================

func TestParser_ParseXRef_MissingXrefKeyword(t *testing.T) {
	input := `notxref
0 1
0000000000 65535 f
trailer
<< /Size 1 >>`

	p := NewParser(strings.NewReader(input))
	_, err := p.ParseXRef()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected 'xref'")
}

func TestParser_ParseXRef_MissingTrailer(t *testing.T) {
	input := `xref
0 1
0000000000 65535 f`

	p := NewParser(strings.NewReader(input))
	_, err := p.ParseXRef()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected 'trailer'")
}

func TestParser_ParseXRef_InvalidStartNumber(t *testing.T) {
	input := `xref
abc 1
0000000000 65535 f
trailer
<< /Size 1 >>`

	p := NewParser(strings.NewReader(input))
	_, err := p.ParseXRef()

	assert.Error(t, err)
}

func TestParser_ParseXRef_InvalidCount(t *testing.T) {
	input := `xref
0 abc
0000000000 65535 f
trailer
<< /Size 1 >>`

	p := NewParser(strings.NewReader(input))
	_, err := p.ParseXRef()

	assert.Error(t, err)
}

func TestParser_ParseXRef_InvalidEntryOffset(t *testing.T) {
	// When we have something that's not an integer where offset is expected,
	// we should get an error. Using a name token here.
	input := `xref
0 1
/NotAnInteger 65535 f
trailer
<< /Size 1 >>`

	p := NewParser(strings.NewReader(input))
	_, err := p.ParseXRef()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected offset/next")
}

func TestParser_ParseXRef_InvalidEntryGeneration(t *testing.T) {
	// When we have something that's not an integer where generation is expected
	input := `xref
0 1
0000000000 /NotAnInteger f
trailer
<< /Size 1 >>`

	p := NewParser(strings.NewReader(input))
	_, err := p.ParseXRef()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected generation")
}

func TestParser_ParseXRef_InvalidEntryType(t *testing.T) {
	// 'x' will be parsed as a keyword since it's a regular char
	input := `xref
0 1
0000000000 65535 x
trailer
<< /Size 1 >>`

	p := NewParser(strings.NewReader(input))
	_, err := p.ParseXRef()

	require.Error(t, err)
	// The error message will include "expected entry type"
	assert.Contains(t, err.Error(), "entry type")
}

func TestParser_ParseXRef_MissingTrailerDictionary(t *testing.T) {
	input := `xref
0 1
0000000000 65535 f
trailer
123`

	p := NewParser(strings.NewReader(input))
	_, err := p.ParseXRef()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse trailer dictionary")
}

// ============================================================================
// ParseStartXRef Tests
// ============================================================================

func TestParser_ParseStartXRef(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			name:     "simple startxref",
			input:    testStartXRef + "\n492",
			expected: 492,
		},
		{
			name:     "large offset",
			input:    testStartXRef + "\n1234567890",
			expected: 1234567890,
		},
		{
			name:     "zero offset",
			input:    testStartXRef + "\n0",
			expected: 0,
		},
		{
			name:     "with EOF marker",
			input:    testStartXRef + "\n492\n%%EOF",
			expected: 492,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(strings.NewReader(tt.input))
			offset, err := p.ParseStartXRef()

			require.NoError(t, err)
			assert.Equal(t, tt.expected, offset)
		})
	}
}

func TestParser_ParseStartXRef_MissingKeyword(t *testing.T) {
	input := "notstartxref\n492"
	p := NewParser(strings.NewReader(input))
	_, err := p.ParseStartXRef()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected '"+testStartXRef+"'")
}

func TestParser_ParseStartXRef_MissingOffset(t *testing.T) {
	input := testStartXRef
	p := NewParser(strings.NewReader(input))
	_, err := p.ParseStartXRef()

	assert.Error(t, err)
}

func TestParser_ParseStartXRef_InvalidOffset(t *testing.T) {
	input := testStartXRef + "\nabc"
	p := NewParser(strings.NewReader(input))
	_, err := p.ParseStartXRef()

	assert.Error(t, err)
}

// ============================================================================
// XRefStream Tests
// ============================================================================

func TestNewXRefStream(t *testing.T) {
	stream := NewStream(nil, []byte("test"))
	xrefStream := NewXRefStream(stream)

	assert.NotNil(t, xrefStream)
	assert.Equal(t, stream, xrefStream.Stream)
	assert.NotNil(t, xrefStream.Entries)
	assert.NotNil(t, xrefStream.W)
	assert.NotNil(t, xrefStream.Index)
}

// Note: ParseXRefStream is now fully implemented (PDF 1.5+ support)
// See xref_stream_test.go and objstm_test.go for comprehensive tests

// ============================================================================
// Integration Tests
// ============================================================================

func TestParser_ParseXRef_FullExample(t *testing.T) {
	// This is a realistic example from a simple PDF
	input := `xref
0 8
0000000000 65535 f
0000000009 00000 n
0000000074 00000 n
0000000120 00000 n
0000000179 00000 n
0000000322 00000 n
0000000415 00000 n
0000000445 00000 n
trailer
<< /Size 8 /Root 1 0 R /Info 7 0 R >>`

	p := NewParser(strings.NewReader(input))
	table, err := p.ParseXRef()

	require.NoError(t, err)
	require.NotNil(t, table)

	// Verify all entries
	assert.Equal(t, 8, table.Size())

	// Verify specific entries
	entry0, _ := table.GetEntry(0)
	assert.True(t, entry0.IsFree())

	entry1, _ := table.GetEntry(1)
	assert.Equal(t, int64(9), entry1.Offset)

	entry7, _ := table.GetEntry(7)
	assert.Equal(t, int64(445), entry7.Offset)

	// Verify trailer
	trailer := table.GetTrailer()
	assert.Equal(t, int64(8), trailer.GetInteger("Size"))
}

func TestParser_ParseXRef_WithWhitespace(t *testing.T) {
	// Test that extra whitespace is handled correctly
	input := `xref
0   1
0000000000   65535   f
trailer
<<  /Size  1  >>`

	p := NewParser(strings.NewReader(input))
	table, err := p.ParseXRef()

	require.NoError(t, err)
	assert.Equal(t, 1, table.Size())
}

func TestParser_ParseXRef_EmptySubsection(t *testing.T) {
	// Test subsection with count 0 (edge case)
	input := `xref
0 0
trailer
<< /Size 0 >>`

	p := NewParser(strings.NewReader(input))
	table, err := p.ParseXRef()

	require.NoError(t, err)
	assert.Equal(t, 0, table.Size())
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkParseXRef_Small(b *testing.B) {
	input := `xref
0 6
0000000000 65535 f
0000000015 00000 n
0000000079 00000 n
0000000173 00000 n
0000000301 00000 n
0000000380 00000 n
trailer
<< /Size 6 /Root 1 0 R >>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := NewParser(strings.NewReader(input))
		_, _ = p.ParseXRef()
	}
}

func BenchmarkParseXRef_Large(b *testing.B) {
	// Build a large xref table
	var sb strings.Builder
	sb.WriteString("xref\n0 1000\n")
	sb.WriteString("0000000000 65535 f \n")
	for i := 1; i < 1000; i++ {
		sb.WriteString("0000001000 00000 n \n")
	}
	sb.WriteString("trailer\n<< /Size 1000 >>")
	input := sb.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := NewParser(strings.NewReader(input))
		_, _ = p.ParseXRef()
	}
}

func BenchmarkParseStartXRef(b *testing.B) {
	input := "startxref\n492\n%%EOF"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := NewParser(strings.NewReader(input))
		_, _ = p.ParseStartXRef()
	}
}
