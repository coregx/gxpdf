package extractor

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/ascii85"
	"fmt"
	"testing"

	"github.com/coregx/gxpdf/internal/parser"
	"github.com/coregx/gxpdf/internal/testutil"
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

func TestImageExtractorRejectsUnsupportedJPX(t *testing.T) {
	dictionary := parser.NewDictionary()
	dictionary.Set("Filter", parser.NewName("JPXDecode"))
	stream := parser.NewStream(dictionary, []byte("encoded JPEG 2000 payload"))

	_, _, err := (&ImageExtractor{}).decodeImageData(stream)

	require.ErrorIs(t, err, parser.ErrUnsupportedStreamFilter)
}

func TestImageExtractorPreservesTerminalDCT(t *testing.T) {
	dictionary := parser.NewDictionary()
	dictionary.Set("Filter", parser.NewName("DCTDecode"))
	payload := []byte("encoded JPEG payload")
	stream := parser.NewStream(dictionary, payload)

	data, filter, err := (&ImageExtractor{}).decodeImageData(stream)

	require.NoError(t, err)
	assert.Equal(t, payload, data)
	assert.Equal(t, "/DCTDecode", filter)
}

func TestImageExtractorDecodesPackedPredictorSamples(t *testing.T) {
	tests := []struct {
		name             string
		predictor        int64
		bitsPerComponent int64
		columns          int64
		encoded          []byte
		want             []byte
	}{
		{
			name:             "one-bit TIFF rows with padding",
			predictor:        2,
			bitsPerComponent: 1,
			columns:          3,
			encoded:          []byte{0xe0, 0x40},
			want:             []byte{0xa0, 0x60},
		},
		{
			name:             "sixteen-bit PNG Sub row",
			predictor:        15,
			bitsPerComponent: 16,
			columns:          2,
			encoded:          []byte{1, 0x01, 0x00, 0x01, 0x00},
			want:             []byte{0x01, 0x00, 0x02, 0x00},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var compressed bytes.Buffer
			writer := zlib.NewWriter(&compressed)
			_, err := writer.Write(test.encoded)
			require.NoError(t, err)
			require.NoError(t, writer.Close())

			parameters := parser.NewDictionary()
			parameters.Set("Predictor", parser.NewInteger(test.predictor))
			parameters.Set("Colors", parser.NewInteger(1))
			parameters.Set("BitsPerComponent", parser.NewInteger(test.bitsPerComponent))
			parameters.Set("Columns", parser.NewInteger(test.columns))
			dictionary := parser.NewDictionary()
			dictionary.Set("Filter", parser.NewName("FlateDecode"))
			dictionary.Set("DecodeParms", parameters)

			data, terminalFilter, decodeErr := (&ImageExtractor{}).decodeImageData(
				parser.NewStream(dictionary, compressed.Bytes()),
			)
			require.NoError(t, decodeErr)
			assert.Equal(t, "/FlateDecode", terminalFilter)
			assert.Equal(t, test.want, data)
		})
	}
}

func TestTextExtractorReadsIdentityCryptContent(t *testing.T) {
	pdf := testutil.BuildPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		testutil.Stream("/Filter /Crypt", "BT /F1 12 Tf 10 100 Td (Identity crypt filter) Tj ET"),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	)
	reader, err := parser.OpenPDFFromBytes(pdf)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reader.Close() })

	elements, err := NewTextExtractor(reader).ExtractFromPage(0)
	require.NoError(t, err)
	assert.Equal(t, "Identity crypt filter", AssembleText(elements))
}

func imagePagePDF(imageDictionary string, payload []byte) []byte {
	return testutil.BuildPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Resources << /XObject << /Im1 5 0 R >> >> /Contents 4 0 R >>",
		testutil.Stream("", "q 100 0 0 100 0 0 cm /Im1 Do Q"),
		testutil.Stream("/Type /XObject /Subtype /Image "+imageDictionary, string(payload)),
	)
}

func TestImageExtractorDecodesImageLargerThanStreamDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("decodes a 67 MB image")
	}
	const width, height = 8200, 8200 // 67.24 MB of DeviceGray samples
	var compressed bytes.Buffer
	writer, err := zlib.NewWriterLevel(&compressed, zlib.BestSpeed)
	require.NoError(t, err)
	_, err = writer.Write(make([]byte, width*height))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	require.Greater(t, int64(width*height), parser.DefaultStreamDecodeOptions().MaxDecodedBytes)

	reader, err := parser.OpenPDFFromBytes(imagePagePDF(
		"/Width 8200 /Height 8200 /ColorSpace /DeviceGray /BitsPerComponent 8 /Filter /FlateDecode",
		compressed.Bytes(),
	))
	require.NoError(t, err)
	t.Cleanup(func() { _ = reader.Close() })

	images, err := NewImageExtractor(reader).ExtractFromPage(0)
	require.NoError(t, err)
	require.Len(t, images, 1)
	assert.Equal(t, width, images[0].Width())
	assert.Equal(t, height, images[0].Height())
}

func TestImageExtractorReportsImageOverDecodeLimit(t *testing.T) {
	// 20000 x 20000 RGB declares 1.2 GB, above the per-image limit.
	reader, err := parser.OpenPDFFromBytes(imagePagePDF(
		"/Width 20000 /Height 20000 /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode",
		[]byte("x"),
	))
	require.NoError(t, err)
	t.Cleanup(func() { _ = reader.Close() })

	extractor := NewImageExtractor(reader)
	images, err := extractor.ExtractFromPage(0)
	require.ErrorIs(t, err, parser.ErrStreamDecodeLimit)
	assert.Empty(t, images)

	images, err = extractor.ExtractFromDocument()
	require.ErrorIs(t, err, parser.ErrStreamDecodeLimit)
	assert.Empty(t, images)
}

func TestImageDecodeOptionsFollowImageDictionary(t *testing.T) {
	newDictionary := func(width, height, bits int64, colorSpace string) *parser.Dictionary {
		dictionary := parser.NewDictionary()
		dictionary.Set("Width", parser.NewInteger(width))
		dictionary.Set("Height", parser.NewInteger(height))
		dictionary.Set("BitsPerComponent", parser.NewInteger(bits))
		dictionary.Set("ColorSpace", parser.NewName(colorSpace))
		return dictionary
	}
	extractor := &ImageExtractor{}
	defaultLimit := parser.DefaultStreamDecodeOptions().MaxDecodedBytes

	small, err := extractor.imageDecodeOptions(newDictionary(10, 10, 8, "DeviceRGB"))
	require.NoError(t, err)
	assert.Equal(t, defaultLimit, small.MaxDecodedBytes)

	rgb, err := extractor.imageDecodeOptions(newDictionary(5000, 5000, 8, "DeviceRGB"))
	require.NoError(t, err)
	assert.Equal(t, int64(5000*5000*3), rgb.MaxDecodedBytes)

	cmyk, err := extractor.imageDecodeOptions(newDictionary(5000, 5000, 8, "DeviceCMYK"))
	require.NoError(t, err)
	assert.Equal(t, int64(5000*5000*4), cmyk.MaxDecodedBytes)

	predicted := newDictionary(5000, 5000, 8, "DeviceRGB")
	parameters := parser.NewDictionary()
	parameters.Set("Predictor", parser.NewInteger(15))
	predicted.Set("DecodeParms", parameters)
	withTags, err := extractor.imageDecodeOptions(predicted)
	require.NoError(t, err)
	assert.Equal(t, int64(5000*5000*3+5000), withTags.MaxDecodedBytes)

	_, err = extractor.imageDecodeOptions(newDictionary(20000, 20000, 8, "DeviceRGB"))
	require.ErrorIs(t, err, parser.ErrStreamDecodeLimit)
}

func TestImageDecodeOptionsBoundaryIsExact(t *testing.T) {
	// DeviceGray 8-bit rows of 4096 bytes: maxImageDecodedBytes/4096 rows
	// fit exactly; one more row does not.
	const width = 4096
	maxRows := maxImageDecodedBytes / width
	newDictionary := func(height int64, predictor bool) *parser.Dictionary {
		dictionary := parser.NewDictionary()
		dictionary.Set("Width", parser.NewInteger(width))
		dictionary.Set("Height", parser.NewInteger(height))
		dictionary.Set("BitsPerComponent", parser.NewInteger(8))
		dictionary.Set("ColorSpace", parser.NewName("DeviceGray"))
		if predictor {
			parameters := parser.NewDictionary()
			parameters.Set("Predictor", parser.NewInteger(12))
			parameters.Set("Columns", parser.NewInteger(width))
			dictionary.Set("DecodeParms", parameters)
		}
		return dictionary
	}
	extractor := &ImageExtractor{}

	atLimit, err := extractor.imageDecodeOptions(newDictionary(maxRows, false))
	require.NoError(t, err)
	assert.Equal(t, maxImageDecodedBytes, atLimit.MaxDecodedBytes)

	_, err = extractor.imageDecodeOptions(newDictionary(maxRows+1, false))
	require.ErrorIs(t, err, parser.ErrStreamDecodeLimit)

	// With PNG predictor tag bytes, the largest accepted limit still stays
	// within maxImageDecodedBytes.
	predictedRows := maxImageDecodedBytes / (width + 1)
	underLimit, err := extractor.imageDecodeOptions(newDictionary(predictedRows, true))
	require.NoError(t, err)
	assert.LessOrEqual(t, underLimit.MaxDecodedBytes, maxImageDecodedBytes)
	assert.Equal(t, predictedRows*(width+1), underLimit.MaxDecodedBytes)

	_, err = extractor.imageDecodeOptions(newDictionary(predictedRows+1, true))
	require.ErrorIs(t, err, parser.ErrStreamDecodeLimit)
}

// TestTextExtractorSkipsInlineImage covers a page whose content stream embeds
// an inline image: its flate payload used to reach the operator parser, which
// failed the whole page with `unknown token "x"` (the 0x78 header).
func TestTextExtractorSkipsInlineImage(t *testing.T) {
	content := "q 100 0 0 100 0 0 cm\nBI /W 1 /H 1 /BPC 8 /CS /G /F /Fl\n" +
		"ID \x78\x9c\x01\x02\x03\nEI Q\nBT /F1 12 Tf 10 100 Td (After the image) Tj ET"
	pdf := testutil.BuildPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		testutil.Stream("", content),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	)
	reader, err := parser.OpenPDFFromBytes(pdf)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reader.Close() })

	elements, err := NewTextExtractor(reader).ExtractFromPage(0)
	require.NoError(t, err)
	assert.Equal(t, "After the image", AssembleText(elements))
}

// TestTextExtractorReadsStreamWithIndirectLength covers a compressed stream
// whose /Length is an indirect reference: the parser cannot size the stream and
// scans for endstream instead. The end-of-line markers around the data are
// syntax, not content - leaving the one after `stream` in place made every such
// flate stream fail with "zlib: invalid header" (bug-009).
func TestTextExtractorReadsStreamWithIndirectLength(t *testing.T) {
	text := "BT /F1 12 Tf 10 100 Td (Indirect length) Tj ET"
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	_, err := writer.Write([]byte(text))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	pdf := testutil.BuildPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		// CRLF after the stream keyword, as most producers write it.
		"<< /Length 6 0 R /Filter /FlateDecode >>\nstream\r\n"+compressed.String()+"\nendstream",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("%d", compressed.Len()),
	)
	reader, err := parser.OpenPDFFromBytes(pdf)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reader.Close() })

	elements, err := NewTextExtractor(reader).ExtractFromPage(0)
	require.NoError(t, err)
	assert.Equal(t, "Indirect length", AssembleText(elements))
}
