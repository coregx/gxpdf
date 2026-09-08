package extractor

import (
	"bytes"
	"compress/zlib"
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
