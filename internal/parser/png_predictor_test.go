package parser

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// PNG Predictor Filter Tests
// ============================================================================

func TestApplyPNGPredictor(t *testing.T) {
	tests := []struct {
		name     string
		columns  int
		input    []byte // includes filter byte per row
		expected []byte // decoded output (no filter bytes)
	}{
		{
			name:    "None filter (type 0)",
			columns: 3,
			// Row: [filter=0, 1, 2, 3]
			input:    []byte{0, 1, 2, 3},
			expected: []byte{1, 2, 3},
		},
		{
			name:    "None filter multiple rows",
			columns: 2,
			// Row1: [0, 10, 20], Row2: [0, 30, 40]
			input:    []byte{0, 10, 20, 0, 30, 40},
			expected: []byte{10, 20, 30, 40},
		},
		{
			name:    "Sub filter (type 1)",
			columns: 3,
			// Row: [filter=1, 5, 3, 2]
			// decoded[0] = 5 + 0 = 5
			// decoded[1] = 3 + 5 = 8
			// decoded[2] = 2 + 8 = 10
			input:    []byte{1, 5, 3, 2},
			expected: []byte{5, 8, 10},
		},
		{
			name:    "Sub filter multiple rows",
			columns: 3,
			// Row1: [1, 1, 1, 1] -> [1, 2, 3]
			// Row2: [1, 2, 2, 2] -> [2, 4, 6]
			input:    []byte{1, 1, 1, 1, 1, 2, 2, 2},
			expected: []byte{1, 2, 3, 2, 4, 6},
		},
		{
			name:    "Up filter (type 2)",
			columns: 3,
			// Row1: [0, 10, 20, 30] -> [10, 20, 30] (no prev row, so prevRow is zeros)
			// Row2: [2, 5, 5, 5]   -> [10+5, 20+5, 30+5] = [15, 25, 35]
			input:    []byte{0, 10, 20, 30, 2, 5, 5, 5},
			expected: []byte{10, 20, 30, 15, 25, 35},
		},
		{
			name:    "Average filter (type 3)",
			columns: 3,
			// Row1: [0, 10, 20, 30] -> [10, 20, 30]
			// Row2: [3, a, b, c]
			// decoded[0] = a + floor((0 + 10) / 2) = a + 5
			// decoded[1] = b + floor((decoded[0] + 20) / 2)
			// decoded[2] = c + floor((decoded[1] + 30) / 2)
			// Let's use: a=0, b=0, c=0
			// decoded[0] = 0 + 5 = 5
			// decoded[1] = 0 + floor((5 + 20) / 2) = 0 + 12 = 12
			// decoded[2] = 0 + floor((12 + 30) / 2) = 0 + 21 = 21
			input:    []byte{0, 10, 20, 30, 3, 0, 0, 0},
			expected: []byte{10, 20, 30, 5, 12, 21},
		},
		{
			name:    "Paeth filter (type 4)",
			columns: 3,
			// Row1: [0, 10, 20, 30] -> [10, 20, 30]
			// Row2: [4, 0, 0, 0]
			// For first byte: left=0, up=10, upLeft=0
			//   p = 0 + 10 - 0 = 10
			//   pLeft = |10-0| = 10, pUp = |10-10| = 0, pUpLeft = |10-0| = 10
			//   pUp <= pUpLeft, so use up = 10
			//   decoded[0] = 0 + 10 = 10
			// For second byte: left=10, up=20, upLeft=10
			//   p = 10 + 20 - 10 = 20
			//   pLeft = |20-10| = 10, pUp = |20-20| = 0, pUpLeft = |20-10| = 10
			//   pUp <= pUpLeft, so use up = 20
			//   decoded[1] = 0 + 20 = 20
			// For third byte: left=20, up=30, upLeft=20
			//   p = 20 + 30 - 20 = 30
			//   pLeft = |30-20| = 10, pUp = |30-30| = 0, pUpLeft = |30-20| = 10
			//   pUp <= pUpLeft, so use up = 30
			//   decoded[2] = 0 + 30 = 30
			input:    []byte{0, 10, 20, 30, 4, 0, 0, 0},
			expected: []byte{10, 20, 30, 10, 20, 30},
		},
		{
			name:    "Mixed filters across rows",
			columns: 2,
			// Row1: [0, 1, 2]       -> None:    [1, 2]
			// Row2: [1, 3, 4]       -> Sub:     [3, 7]
			// Row3: [2, 1, 1]       -> Up:      [4, 8]
			input:    []byte{0, 1, 2, 1, 3, 4, 2, 1, 1},
			expected: []byte{1, 2, 3, 7, 4, 8},
		},
		{
			name:    "Single column",
			columns: 1,
			// Row1: [0, 100] -> [100]
			// Row2: [2, 50]  -> [100 + 50 = 150]
			input:    []byte{0, 100, 2, 50},
			expected: []byte{100, 150},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := applyPNGPredictor(tt.input, tt.columns)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestApplyPNGPredictor_Errors(t *testing.T) {
	t.Run("invalid filter type", func(t *testing.T) {
		// Filter byte 5 is invalid
		input := []byte{5, 1, 2, 3}
		_, err := applyPNGPredictor(input, 3)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown PNG filter type: 5")
	})

	t.Run("data length not divisible by row size", func(t *testing.T) {
		// columns=3 means rowSize=4, but we have 5 bytes
		input := []byte{0, 1, 2, 3, 4}
		_, err := applyPNGPredictor(input, 3)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not divisible by row size")
	})

	t.Run("columns zero", func(t *testing.T) {
		input := []byte{0, 1, 2, 3}
		_, err := applyPNGPredictor(input, 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "out of valid range")
	})

	t.Run("columns negative", func(t *testing.T) {
		input := []byte{0, 1, 2, 3}
		_, err := applyPNGPredictor(input, -1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "out of valid range")
	})

	t.Run("columns exceeds limit", func(t *testing.T) {
		input := []byte{0, 1, 2, 3}
		_, err := applyPNGPredictor(input, 100_001)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "out of valid range")
	})
}

// ============================================================================
// Paeth Predictor Tests
// ============================================================================

func TestPaethPredictor(t *testing.T) {
	tests := []struct {
		name     string
		left     byte
		up       byte
		upLeft   byte
		expected byte
	}{
		{
			name: "all zeros",
			left: 0, up: 0, upLeft: 0,
			expected: 0, // p=0, all distances equal, returns left
		},
		{
			name: "left closest",
			left: 10, up: 100, upLeft: 50,
			// p = 10 + 100 - 50 = 60
			// pLeft = |60-10| = 50, pUp = |60-100| = 40, pUpLeft = |60-50| = 10
			// pUpLeft is smallest, returns upLeft
			expected: 50,
		},
		{
			name: "up closest",
			left: 10, up: 20, upLeft: 10,
			// p = 10 + 20 - 10 = 20
			// pLeft = |20-10| = 10, pUp = |20-20| = 0, pUpLeft = |20-10| = 10
			// pUp is smallest, returns up
			expected: 20,
		},
		{
			name: "equal distances prefer left",
			left: 10, up: 10, upLeft: 10,
			// p = 10 + 10 - 10 = 10
			// pLeft = 0, pUp = 0, pUpLeft = 0
			// All equal, left wins (pLeft <= pUp && pLeft <= pUpLeft)
			expected: 10,
		},
		{
			name: "upLeft closest",
			left: 0, up: 0, upLeft: 100,
			// p = 0 + 0 - 100 = -100
			// pLeft = |-100-0| = 100, pUp = |-100-0| = 100, pUpLeft = |-100-100| = 200
			// pLeft and pUp are equal and smaller than pUpLeft
			// pLeft <= pUp is true, so returns left
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := paethPredictor(tt.left, tt.up, tt.upLeft)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============================================================================
// DecodeWithPredictor Tests
// ============================================================================

func TestFlateDecoder_DecodeWithPredictor(t *testing.T) {
	t.Run("predictor 1 (none) returns decompressed data", func(t *testing.T) {
		decoder := &flateDecoder{}
		// Create valid zlib-compressed data for "hello"
		// For simplicity, we test with predictor=1 which just decompresses
		// We need actual compressed data, so let's use a minimal valid zlib stream
		compressed := []byte{
			0x78, 0x9c, // zlib header (default compression)
			0xcb, 0x48, 0xcd, 0xc9, 0xc9, 0x07, 0x00, // compressed "hello"
			0x06, 0x2c, 0x02, 0x15, // adler32 checksum
		}
		result, err := decoder.DecodeWithPredictor(compressed, 1, 5)
		require.NoError(t, err)
		assert.Equal(t, []byte("hello"), result)
	})

	t.Run("predictor 0 returns decompressed data", func(t *testing.T) {
		decoder := &flateDecoder{}
		compressed := []byte{
			0x78, 0x9c,
			0xcb, 0x48, 0xcd, 0xc9, 0xc9, 0x07, 0x00,
			0x06, 0x2c, 0x02, 0x15,
		}
		result, err := decoder.DecodeWithPredictor(compressed, 0, 5)
		require.NoError(t, err)
		assert.Equal(t, []byte("hello"), result)
	})

	t.Run("predictor 2 (TIFF) returns error", func(t *testing.T) {
		decoder := &flateDecoder{}
		compressed := []byte{0x78, 0x9c, 0x03, 0x00, 0x00, 0x00, 0x00, 0x01} // empty
		_, err := decoder.DecodeWithPredictor(compressed, 2, 5)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "TIFF predictor not implemented")
	})

	t.Run("unsupported predictor returns error", func(t *testing.T) {
		decoder := &flateDecoder{}
		compressed := []byte{0x78, 0x9c, 0x03, 0x00, 0x00, 0x00, 0x00, 0x01}
		_, err := decoder.DecodeWithPredictor(compressed, 99, 5)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported predictor: 99")
	})

	t.Run("predictor 12 (PNG Up) applies filter", func(t *testing.T) {
		decoder := &flateDecoder{}
		// Create compressed data that when decompressed gives us PNG-filtered rows
		// Row: [filter=0, 0x41, 0x42, 0x43] -> decoded as "ABC"
		// Generated using: zlib.NewWriter + Write([0, 0x41, 0x42, 0x43])
		compressed := []byte{0x78, 0x9c, 0x62, 0x70, 0x74, 0x72, 0x06, 0x04, 0x00, 0x00, 0xff, 0xff, 0x01, 0x8e, 0x00, 0xc7}
		result, err := decoder.DecodeWithPredictor(compressed, 12, 3)
		require.NoError(t, err)
		assert.Equal(t, []byte{0x41, 0x42, 0x43}, result)
	})

	t.Run("predictor 15 (PNG Optimum) with mixed filters", func(t *testing.T) {
		decoder := &flateDecoder{}
		// Two rows with different filters:
		// Row 1: [filter=0, 10, 20] -> None -> [10, 20]
		// Row 2: [filter=2, 5, 5]   -> Up   -> [15, 25]
		// Compressed using zlib.NewWriter
		compressed := []byte{0x78, 0x9c, 0x62, 0xe0, 0x12, 0x61, 0x62, 0x65, 0x05, 0x04, 0x00, 0x00, 0xff, 0xff, 0x00, 0x9d, 0x00, 0x2b}
		result, err := decoder.DecodeWithPredictor(compressed, 15, 2)
		require.NoError(t, err)
		assert.Equal(t, []byte{10, 20, 15, 25}, result)
	})

	t.Run("invalid zlib data returns error", func(t *testing.T) {
		decoder := &flateDecoder{}
		// Invalid zlib data (bad header)
		invalidCompressed := []byte{0x00, 0x00, 0x00, 0x00}
		_, err := decoder.DecodeWithPredictor(invalidCompressed, 12, 3)
		require.Error(t, err)
		// Should fail during decompression
	})
}

// ============================================================================
// Integration Test: XRef Stream with Predictor
// ============================================================================

func TestApplyPNGPredictor_XRefStreamPattern(t *testing.T) {
	// Simulate a typical xref stream pattern with 5-byte entries
	// Type (1 byte) + Field2 (2 bytes) + Field3 (2 bytes) = 5 columns
	// Using Up filter (type 2) which is common for xref streams

	t.Run("xref stream with Up filter", func(t *testing.T) {
		columns := 5
		// Row 1: [filter=0, type=1, offset_hi=0, offset_lo=15, gen_hi=0, gen_lo=0]
		//        -> [1, 0, 15, 0, 0] (in-use at offset 15, gen 0)
		// Row 2: [filter=2, 0, 0, 64, 0, 0] (Up filter, delta from row 1)
		//        -> [1, 0, 79, 0, 0] (in-use at offset 79, gen 0)
		// Row 3: [filter=2, 0, 0, 94, 0, 0]
		//        -> [1, 0, 173, 0, 0] (in-use at offset 173)
		input := []byte{
			0, 1, 0, 15, 0, 0, // Row 1: None filter
			2, 0, 0, 64, 0, 0, // Row 2: Up filter (delta +64)
			2, 0, 0, 94, 0, 0, // Row 3: Up filter (delta +94)
		}
		expected := []byte{
			1, 0, 15, 0, 0, // Entry 1: type=1, offset=15
			1, 0, 79, 0, 0, // Entry 2: type=1, offset=79
			1, 0, 173, 0, 0, // Entry 3: type=1, offset=173
		}

		result, err := applyPNGPredictor(input, columns)
		require.NoError(t, err)
		assert.Equal(t, expected, result)
	})
}

// ============================================================================
// Integration Test: Real PDF with Predictor-Encoded XRef
// ============================================================================

// TestReader_Open_PredictorXRefPDF tests parsing a PDF with predictor-encoded xref.
// The test PDF was generated by testdata/generators/predictor_xref.go
func TestReader_Open_PredictorXRefPDF(t *testing.T) {
	// This PDF has an xref stream with /Predictor 12 (PNG Up filter)
	pdfPath := filepath.Join(testDataDir, "predictor_xref.pdf")

	reader := NewReader(pdfPath)
	require.NotNil(t, reader)

	err := reader.Open()
	require.NoError(t, err, "Should open PDF with predictor-encoded xref stream")
	defer reader.Close()

	// Verify basic structure
	assert.Equal(t, "1.5", reader.Version())

	count, err := reader.GetPageCount()
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify catalog loaded (proves xref was parsed correctly)
	catalog, err := reader.GetCatalog()
	require.NoError(t, err)
	require.NotNil(t, catalog)
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkApplyPNGPredictor_Small(b *testing.B) {
	// 10 rows, 5 columns (typical small xref)
	input := make([]byte, 10*6) // 6 = 5 columns + 1 filter byte
	for i := 0; i < 10; i++ {
		input[i*6] = 2 // Up filter
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = applyPNGPredictor(input, 5)
	}
}

func BenchmarkApplyPNGPredictor_Large(b *testing.B) {
	// 10000 rows, 5 columns (large xref)
	input := make([]byte, 10000*6)
	for i := 0; i < 10000; i++ {
		input[i*6] = 2 // Up filter
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = applyPNGPredictor(input, 5)
	}
}

func BenchmarkPaethPredictor(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = paethPredictor(100, 150, 120)
	}
}
