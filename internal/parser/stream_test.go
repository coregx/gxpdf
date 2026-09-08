package parser

import (
	"bytes"
	"compress/zlib"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStreamDecoder_NoFilter tests decoding a stream with no filter.
func TestStreamDecoder_NoFilter(t *testing.T) {
	content := []byte("Hello, PDF!")
	dict := NewDictionary()
	stream := NewStream(dict, content)

	reader := NewReader("")
	decoded, err := reader.decodeStream(stream)

	require.NoError(t, err)
	assert.Equal(t, content, decoded)
}

// TestStreamDecoder_FlateDecode tests decoding a stream with FlateDecode filter.
func TestStreamDecoder_FlateDecode(t *testing.T) {
	// Original data
	originalData := []byte("This is test data for FlateDecode compression")

	// Compress with zlib
	var buf bytes.Buffer
	writer := zlib.NewWriter(&buf)
	_, err := writer.Write(originalData)
	require.NoError(t, err)
	err = writer.Close()
	require.NoError(t, err)
	compressedData := buf.Bytes()

	// Create stream with FlateDecode filter
	dict := NewDictionary()
	dict.Set("Filter", NewName("FlateDecode"))
	stream := NewStream(dict, compressedData)

	// Decode
	reader := NewReader("")
	decoded, err := reader.decodeStream(stream)

	require.NoError(t, err)
	assert.Equal(t, originalData, decoded)
}

// TestStreamDecoder_DCTDecode tests decoding a stream with DCTDecode filter.
func TestStreamDecoder_DCTDecode(t *testing.T) {
	// Create a simple test image (10x10 red square)
	width, height := 10, 10
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, red)
		}
	}

	// Encode to JPEG
	var jpegBuf bytes.Buffer
	err := jpeg.Encode(&jpegBuf, img, &jpeg.Options{Quality: 95})
	require.NoError(t, err)
	jpegData := jpegBuf.Bytes()

	// Create stream with DCTDecode filter
	dict := NewDictionary()
	dict.Set("Filter", NewName("DCTDecode"))
	stream := NewStream(dict, jpegData)

	// Decode
	reader := NewReader("")
	decoded, err := reader.decodeStream(stream)

	require.NoError(t, err)
	assert.NotEmpty(t, decoded)

	// Verify decoded data has expected size (10x10 RGB = 300 bytes)
	expectedSize := width * height * 3
	assert.Equal(t, expectedSize, len(decoded))

	// Verify first pixel is approximately red (JPEG is lossy, so allow some variation)
	r := decoded[0]
	g := decoded[1]
	b := decoded[2]
	assert.Greater(t, r, uint8(200), "Red channel should be high")
	assert.Less(t, g, uint8(50), "Green channel should be low")
	assert.Less(t, b, uint8(50), "Blue channel should be low")
}

// TestStreamDecoder_DCTDecode_WithParams tests DCTDecode with decode parameters.
func TestStreamDecoder_DCTDecode_WithParams(t *testing.T) {
	// Create test image
	width, height := 5, 5
	img := image.NewGray(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetGray(x, y, color.Gray{Y: 128})
		}
	}

	// Encode to JPEG
	var jpegBuf bytes.Buffer
	err := jpeg.Encode(&jpegBuf, img, &jpeg.Options{Quality: 90})
	require.NoError(t, err)
	jpegData := jpegBuf.Bytes()

	// Create stream with DCTDecode filter and decode parameters
	dict := NewDictionary()
	dict.Set("Filter", NewName("DCTDecode"))

	// Add ColorTransform parameter
	decodeParams := NewDictionary()
	decodeParams.Set("ColorTransform", NewInteger(0))
	dict.Set("DecodeParms", decodeParams)

	stream := NewStream(dict, jpegData)

	// Decode
	reader := NewReader("")
	decoded, err := reader.decodeStream(stream)

	require.NoError(t, err)
	assert.NotEmpty(t, decoded)
}

// TestStreamDecoder_UnsupportedFilter tests handling of unsupported filters.
func TestStreamDecoder_UnsupportedFilter(t *testing.T) {
	dict := NewDictionary()
	dict.Set("Filter", NewName("CCITTFaxDecode"))
	stream := NewStream(dict, []byte("data"))

	reader := NewReader("")
	_, err := reader.decodeStream(stream)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported filter")
}

// TestStreamDecoder_MultipleFilters tests handling of filter arrays.
func TestStreamDecoder_MultipleFilters(t *testing.T) {
	// Original data
	originalData := []byte("Test data")

	// Compress
	var buf bytes.Buffer
	writer := zlib.NewWriter(&buf)
	_, err := writer.Write(originalData)
	require.NoError(t, err)
	err = writer.Close()
	require.NoError(t, err)
	compressedData := buf.Bytes()

	// Create stream with filter array (only first filter is applied in current implementation)
	dict := NewDictionary()
	filters := NewArray()
	filters.Append(NewName("FlateDecode"))
	dict.Set("Filter", filters)
	stream := NewStream(dict, compressedData)

	// Decode
	reader := NewReader("")
	decoded, err := reader.decodeStream(stream)

	require.NoError(t, err)
	assert.Equal(t, originalData, decoded)
}
