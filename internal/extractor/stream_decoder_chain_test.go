package extractor

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/ascii85"
	"testing"

	"github.com/coregx/gxpdf/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractorConsumersUseCompleteFilterChain(t *testing.T) {
	raw := []byte("q 0 0 m 10 10 l S Q")
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	_, err := writer.Write(raw)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	armored := make([]byte, ascii85.MaxEncodedLen(compressed.Len()))
	written := ascii85.Encode(armored, compressed.Bytes())
	armored = append(armored[:written], '~', '>')

	filters := parser.NewArray()
	filters.Append(parser.NewName("ASCII85Decode"))
	filters.Append(parser.NewName("FlateDecode"))
	parameters := parser.NewArray()
	parameters.Append(parser.NewNull())
	parameters.Append(parser.NewNull())
	dictionary := parser.NewDictionary()
	dictionary.Set("Filter", filters)
	dictionary.Set("DecodeParms", parameters)
	stream := parser.NewStream(dictionary, armored)

	consumers := []struct {
		name   string
		decode func(*parser.Stream) ([]byte, error)
	}{
		{name: "text", decode: (&TextExtractor{}).decodeStream},
		{name: "graphics", decode: (&GraphicsParser{}).decodeStream},
		{name: "vector", decode: (&VectorParser{}).decodeStream},
		{name: "font", decode: decodeStreamData},
	}
	for _, consumer := range consumers {
		t.Run(consumer.name, func(t *testing.T) {
			decoded, err := consumer.decode(stream)
			require.NoError(t, err)
			assert.Equal(t, raw, decoded)
		})
	}
}

func TestExtractorConsumersHonorCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stream := parser.NewStream(parser.NewDictionary(), []byte("content"))
	consumers := []struct {
		name   string
		decode func(*parser.Stream) error
	}{
		{name: "text", decode: func(stream *parser.Stream) error {
			_, err := (&TextExtractor{ctx: ctx}).decodeStream(stream)
			return err
		}},
		{name: "graphics", decode: func(stream *parser.Stream) error {
			_, err := (&GraphicsParser{ctx: ctx}).decodeStream(stream)
			return err
		}},
		{name: "vector", decode: func(stream *parser.Stream) error {
			_, err := (&VectorParser{ctx: ctx}).decodeStream(stream)
			return err
		}},
		{name: "font", decode: func(stream *parser.Stream) error {
			_, err := decodeStreamDataWithContext(ctx, stream)
			return err
		}},
		{name: "image", decode: func(stream *parser.Stream) error {
			_, _, err := (&ImageExtractor{ctx: ctx}).decodeImageData(stream)
			return err
		}},
	}

	for _, consumer := range consumers {
		t.Run(consumer.name, func(t *testing.T) {
			require.ErrorIs(t, consumer.decode(stream), context.Canceled)
		})
	}
}
