package parser

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/ascii85"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamDecodeSupportedFilters(t *testing.T) {
	raw := []byte("BT /F1 12 Tf 72 700 Td (Filtered text) Tj ET")
	tests := []struct {
		name    string
		filter  string
		encoded func(*testing.T, []byte) []byte
	}{
		{name: "flate", filter: "FlateDecode", encoded: encodeFlateForTest},
		{name: "ascii85", filter: "ASCII85Decode", encoded: encodeASCII85ForTest},
		{name: "ascii hex", filter: "ASCIIHexDecode", encoded: encodeASCIIHexForTest},
		{name: "run length", filter: "RunLengthDecode", encoded: encodeRunLengthForTest},
		{name: "lzw", filter: "LZWDecode", encoded: encodeLiteralLZWForTest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dictionary := NewDictionary()
			dictionary.Set("Filter", NewName(test.filter))
			decoded, err := NewStream(dictionary, test.encoded(t, raw)).Decode()
			require.NoError(t, err)
			assert.Equal(t, raw, decoded)
		})
	}
}

func TestStreamDecodeFilterChain(t *testing.T) {
	raw := []byte("BT (chained filters) Tj ET")
	compressed := encodeFlateForTest(t, raw)
	encoded := encodeASCII85ForTest(t, compressed)
	filters := NewArray()
	filters.Append(NewName("ASCII85Decode"))
	filters.Append(NewName("FlateDecode"))
	parameters := NewArray()
	parameters.Append(NewNull())
	parameters.Append(NewDictionary())
	dictionary := NewDictionary()
	dictionary.Set("Filter", filters)
	dictionary.Set("DecodeParms", parameters)

	decoded, err := NewStream(dictionary, encoded).Decode()
	require.NoError(t, err)
	assert.Equal(t, raw, decoded)
}

func TestStreamDecodePreservesAllowedTerminalFilter(t *testing.T) {
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	filters := NewArray()
	filters.Append(NewName("ASCII85Decode"))
	filters.Append(NewName("DCTDecode"))
	dictionary := NewDictionary()
	dictionary.Set("Filter", filters)

	decoded, terminalFilter, err := NewStream(
		dictionary,
		encodeASCII85ForTest(t, jpegData),
	).DecodePreservingTerminalWithContext(
		context.Background(),
		DefaultStreamDecodeOptions(),
		"DCTDecode",
	)
	require.NoError(t, err)
	assert.Equal(t, jpegData, decoded)
	assert.Equal(t, "DCTDecode", terminalFilter)
}

func TestStreamDecodeNullDictionaryEntriesBehaveAsAbsent(t *testing.T) {
	tests := []struct {
		name       string
		filter     PdfObject
		parameters PdfObject
	}{
		{name: "null filter", filter: NewNull()},
		{name: "null decode parameters", filter: NewName("ASCIIHexDecode"), parameters: NewNull()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := []byte("plain text")
			encoded := raw
			if _, filtered := test.filter.(*Name); filtered {
				encoded = encodeASCIIHexForTest(t, raw)
			}
			dictionary := NewDictionary()
			dictionary.Set("Filter", test.filter)
			if test.parameters != nil {
				dictionary.Set("DecodeParms", test.parameters)
			}

			decoded, err := NewStream(dictionary, encoded).Decode()
			require.NoError(t, err)
			assert.Equal(t, raw, decoded)
		})
	}
}

func TestStreamDecodeFilterAliases(t *testing.T) {
	raw := []byte("alias")
	dictionary := NewDictionary()
	dictionary.Set("Filter", NewName("AHx"))
	decoded, err := NewStream(dictionary, encodeASCIIHexForTest(t, raw)).Decode()
	require.NoError(t, err)
	assert.Equal(t, raw, decoded)
}

func TestStreamDecodeLZWEarlyChangeModes(t *testing.T) {
	raw := bytes.Repeat([]byte("abcdef"), 120)
	for _, earlyChange := range []int{0, 1} {
		t.Run(fmt.Sprintf("EarlyChange_%d", earlyChange), func(t *testing.T) {
			parameters := NewDictionary()
			parameters.Set("EarlyChange", NewInteger(int64(earlyChange)))
			dictionary := NewDictionary()
			dictionary.Set("Filter", NewName("LZWDecode"))
			dictionary.Set("DecodeParms", parameters)
			decoded, err := NewStream(dictionary, encodeLiteralLZW(raw, earlyChange)).Decode()
			require.NoError(t, err)
			assert.Equal(t, raw, decoded)
		})
	}
}

func TestStreamDecodePredictors(t *testing.T) {
	tests := []struct {
		name             string
		predictor        int64
		colors           int64
		bitsPerComponent int64
		columns          int64
		encoded          []byte
		want             []byte
	}{
		{
			name:             "TIFF eight-bit horizontal differencing",
			predictor:        2,
			colors:           1,
			bitsPerComponent: 8,
			columns:          3,
			encoded:          []byte{10, 10, 10, 5, 2, 2},
			want:             []byte{10, 20, 30, 5, 7, 9},
		},
		{
			name:             "TIFF one-bit packed samples",
			predictor:        2,
			colors:           1,
			bitsPerComponent: 1,
			columns:          8,
			encoded:          []byte{0xeb},
			want:             []byte{0xb2},
		},
		{
			name:             "TIFF two-bit packed samples",
			predictor:        2,
			colors:           1,
			bitsPerComponent: 2,
			columns:          4,
			encoded:          []byte{0x55},
			want:             []byte{0x6c},
		},
		{
			name:             "TIFF four-bit packed samples",
			predictor:        2,
			colors:           1,
			bitsPerComponent: 4,
			columns:          2,
			encoded:          []byte{0x16},
			want:             []byte{0x17},
		},
		{
			name:             "TIFF four-bit RGB samples",
			predictor:        2,
			colors:           3,
			bitsPerComponent: 4,
			columns:          2,
			encoded:          []byte{0x12, 0x33, 0x45},
			want:             []byte{0x12, 0x34, 0x68},
		},
		{
			name:             "TIFF sixteen-bit samples",
			predictor:        2,
			colors:           1,
			bitsPerComponent: 16,
			columns:          2,
			encoded:          []byte{0x01, 0x00, 0x02, 0x00},
			want:             []byte{0x01, 0x00, 0x03, 0x00},
		},
		{
			name:             "PNG eight-bit row filters",
			predictor:        15,
			colors:           1,
			bitsPerComponent: 8,
			columns:          3,
			encoded:          []byte{0, 10, 20, 30, 2, 5, 5, 5},
			want:             []byte{10, 20, 30, 15, 25, 35},
		},
		{
			name:             "PNG one-bit packed row",
			predictor:        15,
			colors:           1,
			bitsPerComponent: 1,
			columns:          8,
			encoded:          []byte{0, 0xaa},
			want:             []byte{0xaa},
		},
		{
			name:             "PNG four-bit RGB Sub row",
			predictor:        15,
			colors:           3,
			bitsPerComponent: 4,
			columns:          2,
			encoded:          []byte{1, 0x12, 0x34, 0x44},
			want:             []byte{0x12, 0x34, 0x56},
		},
		{
			name:             "PNG sixteen-bit row",
			predictor:        15,
			colors:           1,
			bitsPerComponent: 16,
			columns:          1,
			encoded:          []byte{0, 0x12, 0x34},
			want:             []byte{0x12, 0x34},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parameters := NewDictionary()
			parameters.Set("Predictor", NewInteger(test.predictor))
			parameters.Set("Colors", NewInteger(test.colors))
			parameters.Set("BitsPerComponent", NewInteger(test.bitsPerComponent))
			parameters.Set("Columns", NewInteger(test.columns))
			dictionary := NewDictionary()
			dictionary.Set("Filter", NewName("FlateDecode"))
			dictionary.Set("DecodeParms", parameters)

			decoded, err := NewStream(dictionary, encodeFlateForTest(t, test.encoded)).Decode()
			require.NoError(t, err)
			assert.Equal(t, test.want, decoded)
		})
	}
}

func TestStreamDecodeRejectsInvalidInputs(t *testing.T) {
	jpegData := func() []byte {
		var output bytes.Buffer
		require.NoError(t, jpeg.Encode(&output, image.NewGray(image.Rect(0, 0, 100, 100)), nil))
		return output.Bytes()
	}()
	tests := []struct {
		name       string
		stream     *Stream
		options    StreamDecodeOptions
		wantIs     error
		wantDetail string
	}{
		{
			name: "unsupported filter",
			stream: func() *Stream {
				dictionary := NewDictionary()
				dictionary.Set("Filter", NewName("CCITTFaxDecode"))
				return NewStream(dictionary, []byte("encoded"))
			}(),
			options: DefaultStreamDecodeOptions(), wantIs: ErrUnsupportedStreamFilter,
		},
		{
			name: "decoded output limit",
			stream: func() *Stream {
				dictionary := NewDictionary()
				dictionary.Set("Filter", NewName("RunLengthDecode"))
				return NewStream(dictionary, []byte{129, 'x', 128})
			}(),
			options: StreamDecodeOptions{MaxDecodedBytes: 8, MaxFilters: 1}, wantIs: ErrStreamDecodeLimit,
		},
		{
			name: "DCT decoded output limit",
			stream: func() *Stream {
				dictionary := NewDictionary()
				dictionary.Set("Filter", NewName("DCTDecode"))
				return NewStream(dictionary, jpegData)
			}(),
			options: StreamDecodeOptions{MaxDecodedBytes: 1_000, MaxFilters: 1}, wantIs: ErrStreamDecodeLimit,
		},
		{
			name: "filter chain limit",
			stream: func() *Stream {
				filters := NewArray()
				filters.Append(NewName("ASCIIHexDecode"))
				filters.Append(NewName("ASCIIHexDecode"))
				dictionary := NewDictionary()
				dictionary.Set("Filter", filters)
				return NewStream(dictionary, []byte("41>"))
			}(),
			options: StreamDecodeOptions{MaxDecodedBytes: 100, MaxFilters: 1}, wantIs: ErrStreamDecodeLimit,
		},
		{
			name:    "overflowing byte limit",
			stream:  NewStream(NewDictionary(), nil),
			options: StreamDecodeOptions{MaxDecodedBytes: math.MaxInt64, MaxFilters: 1}, wantDetail: "invalid stream decode options",
		},
		{
			name: "decode parameter count",
			stream: func() *Stream {
				filters := NewArray()
				filters.Append(NewName("ASCII85Decode"))
				filters.Append(NewName("FlateDecode"))
				dictionary := NewDictionary()
				dictionary.Set("Filter", filters)
				dictionary.Set("DecodeParms", NewDictionary())
				return NewStream(dictionary, nil)
			}(),
			options: DefaultStreamDecodeOptions(), wantDetail: "requires exactly one filter",
		},
		{
			name: "decode parameter type",
			stream: func() *Stream {
				parameters := NewDictionary()
				parameters.Set("Predictor", NewName("invalid"))
				dictionary := NewDictionary()
				dictionary.Set("Filter", NewName("FlateDecode"))
				dictionary.Set("DecodeParms", parameters)
				return NewStream(dictionary, encodeFlateForTest(t, []byte("content")))
			}(),
			options: DefaultStreamDecodeOptions(), wantDetail: "Predictor is *parser.Name, want Integer",
		},
		{
			name: "invalid predictor component depth",
			stream: func() *Stream {
				parameters := NewDictionary()
				parameters.Set("Predictor", NewInteger(2))
				parameters.Set("BitsPerComponent", NewInteger(3))
				dictionary := NewDictionary()
				dictionary.Set("Filter", NewName("FlateDecode"))
				dictionary.Set("DecodeParms", parameters)
				return NewStream(dictionary, encodeFlateForTest(t, []byte("content")))
			}(),
			options: DefaultStreamDecodeOptions(), wantDetail: "want 1, 2, 4, 8, or 16",
		},
		{
			name: "truncated run length",
			stream: func() *Stream {
				dictionary := NewDictionary()
				dictionary.Set("Filter", NewName("RunLengthDecode"))
				return NewStream(dictionary, []byte{2, 'a'})
			}(),
			options: DefaultStreamDecodeOptions(), wantDetail: "unexpected EOF",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.stream.DecodeWithOptions(test.options)
			require.Error(t, err)
			if test.wantIs != nil {
				assert.True(t, errors.Is(err, test.wantIs), "error = %v", err)
			}
			if test.wantDetail != "" {
				assert.ErrorContains(t, err, test.wantDetail)
			}
		})
	}
}

func TestStreamDecodeIndirectControlsFailClosed(t *testing.T) {
	tests := []struct {
		name       string
		configure  func(*Stream)
		wantDetail string
	}{
		{
			name:       "missing resolver",
			configure:  func(*Stream) {},
			wantDetail: "no resolver is available",
		},
		{
			name: "reference cycle",
			configure: func(stream *Stream) {
				stream.setObjectResolver(func(_ context.Context, objectNumber int) (PdfObject, error) {
					if objectNumber == 6 {
						return NewIndirectReference(7, 0), nil
					}
					return NewIndirectReference(6, 0), nil
				})
			},
			wantDetail: "indirect-reference cycle",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dictionary := NewDictionary()
			dictionary.Set("Filter", NewIndirectReference(6, 0))
			stream := NewStream(dictionary, []byte("encoded"))
			test.configure(stream)

			_, err := stream.Decode()
			require.Error(t, err)
			assert.ErrorContains(t, err, test.wantDetail)
		})
	}
}

func TestStreamDecodeDCTWithinLimit(t *testing.T) {
	var encoded bytes.Buffer
	pixels := image.NewGray(image.Rect(0, 0, 2, 2))
	pixels.SetGray(0, 0, color.Gray{Y: 20})
	require.NoError(t, jpeg.Encode(&encoded, pixels, nil))
	dictionary := NewDictionary()
	dictionary.Set("Filter", NewName("DCTDecode"))

	decoded, err := NewStream(dictionary, encoded.Bytes()).DecodeWithOptions(StreamDecodeOptions{
		MaxDecodedBytes: 512,
		MaxFilters:      1,
	})
	require.NoError(t, err)
	assert.Len(t, decoded, 4)
}

func encodeFlateForTest(t *testing.T, data []byte) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := zlib.NewWriter(&output)
	_, err := writer.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return output.Bytes()
}

func encodeASCII85ForTest(t *testing.T, data []byte) []byte {
	t.Helper()
	encoded := make([]byte, ascii85.MaxEncodedLen(len(data)))
	written := ascii85.Encode(encoded, data)
	return append(encoded[:written], '~', '>')
}

func encodeASCIIHexForTest(t *testing.T, data []byte) []byte {
	t.Helper()
	encoded := make([]byte, hex.EncodedLen(len(data)))
	hex.Encode(encoded, data)
	return append(encoded, '>')
}

func encodeRunLengthForTest(t *testing.T, data []byte) []byte {
	t.Helper()
	var encoded []byte
	for len(data) > 0 {
		count := min(len(data), 128)
		encoded = append(encoded, byte(count-1))
		encoded = append(encoded, data[:count]...)
		data = data[count:]
	}
	return append(encoded, 128)
}

func encodeLiteralLZWForTest(t *testing.T, data []byte) []byte {
	t.Helper()
	return encodeLiteralLZW(data, 1)
}

func encodeLiteralLZW(data []byte, earlyChange int) []byte {
	var encoded []byte
	bitOffset := 0
	writeCode := func(code, width int) {
		for bit := width - 1; bit >= 0; bit-- {
			if bitOffset%8 == 0 {
				encoded = append(encoded, 0)
			}
			if code>>bit&1 != 0 {
				encoded[len(encoded)-1] |= 1 << (7 - bitOffset%8)
			}
			bitOffset++
		}
	}
	codeWidth := 9
	nextCode := 258
	writeCode(256, codeWidth)
	for index, value := range data {
		writeCode(int(value), codeWidth)
		if index == 0 || nextCode >= 4096 {
			continue
		}
		nextCode++
		if codeWidth < 12 && nextCode+earlyChange == 1<<codeWidth {
			codeWidth++
		}
	}
	writeCode(257, codeWidth)
	return encoded
}

func FuzzStreamDecodeNeverExceedsLimit(f *testing.F) {
	f.Add(byte(0), byte(0), byte(0), []byte("raw"))
	f.Add(byte(1), byte(1), byte(1), []byte("!!!!"))
	f.Add(byte(2), byte(2), byte(2), []byte{128})
	f.Fuzz(func(t *testing.T, selector, filterShape, parameterShape byte, data []byte) {
		filters := []string{"FlateDecode", "ASCII85Decode", "ASCIIHexDecode", "RunLengthDecode", "LZWDecode", "unknown"}
		dictionary := NewDictionary()
		filter := NewName(filters[int(selector)%len(filters)])
		switch filterShape % 3 {
		case 0:
			dictionary.Set("Filter", filter)
		case 1:
			array := NewArray()
			array.Append(filter)
			dictionary.Set("Filter", array)
		case 2:
			array := NewArray()
			array.Append(filter)
			array.Append(NewInteger(int64(selector)))
			dictionary.Set("Filter", array)
		}
		switch parameterShape % 4 {
		case 1:
			dictionary.Set("DecodeParms", NewDictionary())
		case 2:
			array := NewArray()
			array.Append(NewNull())
			dictionary.Set("DecodeParms", array)
		case 3:
			dictionary.Set("DecodeParms", NewName("invalid"))
		}
		decoded, _ := NewStream(dictionary, data).DecodeWithOptions(StreamDecodeOptions{
			MaxDecodedBytes: 1024,
			MaxFilters:      2,
		})
		if len(decoded) > 1024 {
			t.Fatalf("decoded %d bytes, limit 1024", len(decoded))
		}
	})
}

func TestStreamDecodeRejectsLimitsBeforeInspectingBoundedData(t *testing.T) {
	filters := NewArray()
	filters.Append(NewInteger(1))
	filters.Append(NewInteger(2))
	dictionary := NewDictionary()
	dictionary.Set("Filter", filters)

	_, err := NewStream(dictionary, []byte("x")).DecodeWithOptions(StreamDecodeOptions{
		MaxDecodedBytes: 8,
		MaxFilters:      1,
	})
	require.ErrorIs(t, err, ErrStreamDecodeLimit)
	assert.ErrorContains(t, err, "filter count 2 exceeds 1")

	_, err = NewStream(NewDictionary(), []byte("too large")).DecodeWithOptions(StreamDecodeOptions{
		MaxDecodedBytes: 4,
		MaxFilters:      1,
	})
	require.ErrorIs(t, err, ErrStreamDecodeLimit)
	assert.ErrorContains(t, err, "encoded stream is 9 bytes")
}

func TestStreamDecodeAcceptsConfiguredLimitsExactly(t *testing.T) {
	raw := []byte("data")
	decoded, err := NewStream(NewDictionary(), raw).DecodeWithOptions(StreamDecodeOptions{
		MaxDecodedBytes: int64(len(raw)),
		MaxFilters:      1,
	})
	require.NoError(t, err)
	assert.Equal(t, raw, decoded)

	filters := NewArray()
	filters.Append(NewName("ASCIIHexDecode"))
	filters.Append(NewName("ASCIIHexDecode"))
	dictionary := NewDictionary()
	dictionary.Set("Filter", filters)
	twiceEncoded := encodeASCIIHexForTest(t, encodeASCIIHexForTest(t, []byte("A")))
	decoded, err = NewStream(dictionary, twiceEncoded).DecodeWithOptions(StreamDecodeOptions{
		MaxDecodedBytes: int64(len(twiceEncoded)),
		MaxFilters:      filters.Len(),
	})
	require.NoError(t, err)
	assert.Equal(t, []byte("A"), decoded)
}

func TestStreamDecodeHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dictionary := NewDictionary()
	dictionary.Set("Filter", NewName("FlateDecode"))

	_, err := NewStream(dictionary, []byte("content is never inspected")).DecodeWithContext(
		ctx,
		DefaultStreamDecodeOptions(),
	)
	require.ErrorIs(t, err, context.Canceled)
}
