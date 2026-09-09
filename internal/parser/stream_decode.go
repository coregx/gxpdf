package parser

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/ascii85"
	"errors"
	"fmt"
	"image/color"
	"image/jpeg"
	"io"

	pdfencoding "github.com/coregx/gxpdf/internal/encoding"
)

const (
	defaultMaxDecodedStreamBytes int64 = 64 << 20
	defaultMaxStreamFilters            = 8
)

var (
	ErrUnsupportedStreamFilter = errors.New("unsupported filter")
	ErrStreamDecodeLimit       = errors.New("pdf stream decode limit exceeded")
)

// StreamDecodeOptions bounds the work performed while decoding one stream.
type StreamDecodeOptions struct {
	MaxDecodedBytes int64
	MaxFilters      int
}

// DefaultStreamDecodeOptions returns conservative limits suitable for normal
// page, font, object, and appearance streams.
func DefaultStreamDecodeOptions() StreamDecodeOptions {
	return StreamDecodeOptions{
		MaxDecodedBytes: defaultMaxDecodedStreamBytes,
		MaxFilters:      defaultMaxStreamFilters,
	}
}

// Decode decodes the stream using bounded default options.
func (s *Stream) Decode() ([]byte, error) {
	options := DefaultStreamDecodeOptions()
	return s.DecodeWithContext(context.Background(), options)
}

// DecodeWithOptions applies the stream's filter chain in declaration order.
func (s *Stream) DecodeWithOptions(options StreamDecodeOptions) ([]byte, error) {
	return s.DecodeWithContext(context.Background(), options)
}

// DecodeWithContext applies the stream's filter chain in declaration order
// and stops bounded reads and decode loops when ctx is cancelled. Codecs whose
// standard-library API is not context-aware are checked immediately before and
// after the codec call.
func (s *Stream) DecodeWithContext(ctx context.Context, options StreamDecodeOptions) ([]byte, error) {
	decoded, _, err := s.decodeWithContext(ctx, options, nil)
	return decoded, err
}

// DecodePreservingTerminalWithContext applies the stream's filter chain while
// leaving an explicitly allowed terminal filter encoded. This is intended for
// consumers such as image export that must retain a JPEG or JPEG 2000 payload
// but still need every preceding ASCII or compression filter decoded by the
// canonical bounded pipeline. The returned filter name is canonical and does
// not include a leading slash.
func (s *Stream) DecodePreservingTerminalWithContext(
	ctx context.Context,
	options StreamDecodeOptions,
	terminalFilters ...string,
) ([]byte, string, error) {
	preserved := make(map[string]struct{}, len(terminalFilters))
	for _, filter := range terminalFilters {
		preserved[canonicalStreamFilterName(filter)] = struct{}{}
	}
	return s.decodeWithContext(ctx, options, preserved)
}

func (s *Stream) decodeWithContext(
	ctx context.Context,
	options StreamDecodeOptions,
	preservedTerminalFilters map[string]struct{},
) ([]byte, string, error) {
	maxInt := int64(^uint(0) >> 1)
	if ctx == nil || options.MaxDecodedBytes <= 0 || options.MaxDecodedBytes >= maxInt || options.MaxFilters <= 0 {
		return nil, "", fmt.Errorf("invalid stream decode options")
	}
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	content := s.Content()
	if int64(len(content)) > options.MaxDecodedBytes {
		return nil, "", fmt.Errorf("%w: encoded stream is %d bytes", ErrStreamDecodeLimit, len(content))
	}
	filters, err := streamFilterNames(ctx, s.resolveDecodeObject, s.GetFilter(), options.MaxFilters)
	if err != nil {
		return nil, "", err
	}
	parameters, err := streamDecodeParameters(ctx, s.resolveDecodeObject, s.GetDecodeParams(), len(filters))
	if err != nil {
		return nil, "", err
	}

	decoded := append([]byte(nil), content...)
	terminalFilter := ""
	decodeCount := len(filters)
	if decodeCount > 0 {
		terminalFilter = filters[decodeCount-1]
		if _, preserve := preservedTerminalFilters[terminalFilter]; preserve {
			decodeCount--
		}
	}
	for index, filter := range filters[:decodeCount] {
		if err = ctx.Err(); err != nil {
			return nil, "", err
		}
		decoded, err = applyStreamFilter(
			ctx,
			filter,
			decoded,
			parameters[index],
			options.MaxDecodedBytes,
			s.resolveDecodeObject,
		)
		if err != nil {
			return nil, "", fmt.Errorf("decode filter %d (%s): %w", index, filter, err)
		}
	}
	return decoded, terminalFilter, nil
}

type streamDecodeResolver func(context.Context, PdfObject) (PdfObject, error)

func (s *Stream) resolveDecodeObject(ctx context.Context, object PdfObject) (PdfObject, error) {
	const maxReferenceDepth = 32
	seen := make(map[int]struct{})
	for depth := 0; depth < maxReferenceDepth; depth++ {
		reference, ok := object.(*IndirectReference)
		if !ok {
			return object, nil
		}
		if s.objectResolver == nil {
			return nil, fmt.Errorf("stream decode object %d is indirect but no resolver is available", reference.Number)
		}
		if _, duplicate := seen[reference.Number]; duplicate {
			return nil, fmt.Errorf("stream decode object contains an indirect-reference cycle at object %d", reference.Number)
		}
		seen[reference.Number] = struct{}{}
		resolved, err := s.objectResolver(ctx, reference.Number)
		if err != nil {
			return nil, fmt.Errorf("resolve stream decode object %d: %w", reference.Number, err)
		}
		object = resolved
	}
	return nil, fmt.Errorf("stream decode object exceeds %d indirect references", maxReferenceDepth)
}

func streamFilterNames(
	ctx context.Context,
	resolve streamDecodeResolver,
	filterObject PdfObject,
	maxFilters int,
) ([]string, error) {
	var err error
	filterObject, err = resolve(ctx, filterObject)
	if err != nil {
		return nil, err
	}
	if filterObject == nil {
		return nil, nil
	}
	switch value := filterObject.(type) {
	case *Null:
		return nil, nil
	case *Name:
		return []string{canonicalStreamFilterName(value.Value())}, nil
	case *Array:
		if value.Len() > maxFilters {
			return nil, fmt.Errorf("%w: filter count %d exceeds %d", ErrStreamDecodeLimit, value.Len(), maxFilters)
		}
		filters := make([]string, value.Len())
		for index := 0; index < value.Len(); index++ {
			filter, resolveErr := resolve(ctx, value.Get(index))
			if resolveErr != nil {
				return nil, fmt.Errorf("resolve stream filter %d: %w", index, resolveErr)
			}
			name, ok := filter.(*Name)
			if !ok {
				return nil, fmt.Errorf("stream filter %d is %T, want Name", index, filter)
			}
			filters[index] = canonicalStreamFilterName(name.Value())
		}
		return filters, nil
	default:
		return nil, fmt.Errorf("stream Filter is %T, want Name or Array", filterObject)
	}
}

func canonicalStreamFilterName(name string) string {
	switch name {
	case "Fl":
		return "FlateDecode"
	case "A85":
		return "ASCII85Decode"
	case "AHx":
		return "ASCIIHexDecode"
	case "RL":
		return "RunLengthDecode"
	case "LZW":
		return "LZWDecode"
	case "DCT":
		return "DCTDecode"
	default:
		return name
	}
}

func streamDecodeParameters(
	ctx context.Context,
	resolve streamDecodeResolver,
	parameterObject PdfObject,
	filterCount int,
) ([]*Dictionary, error) {
	parameters := make([]*Dictionary, filterCount)
	var err error
	parameterObject, err = resolve(ctx, parameterObject)
	if err != nil {
		return nil, err
	}
	if parameterObject == nil || filterCount == 0 {
		return parameters, nil
	}
	if _, ok := parameterObject.(*Null); ok {
		return parameters, nil
	}
	if dictionary, ok := parameterObject.(*Dictionary); ok {
		if filterCount != 1 {
			return nil, fmt.Errorf("stream DecodeParms dictionary requires exactly one filter")
		}
		parameters[0] = dictionary
		return parameters, nil
	}
	array, ok := parameterObject.(*Array)
	if !ok {
		return nil, fmt.Errorf("stream DecodeParms is %T, want Dictionary or Array", parameterObject)
	}
	if array.Len() != filterCount {
		return nil, fmt.Errorf("stream DecodeParms count %d does not match filter count %d", array.Len(), filterCount)
	}
	for index := 0; index < array.Len(); index++ {
		value, resolveErr := resolve(ctx, array.Get(index))
		if resolveErr != nil {
			return nil, fmt.Errorf("resolve stream DecodeParms %d: %w", index, resolveErr)
		}
		switch value := value.(type) {
		case *Dictionary:
			parameters[index] = value
		case *Null:
			// A null entry means default parameters for this filter.
		default:
			return nil, fmt.Errorf("stream DecodeParms %d is %T, want Dictionary or Null", index, value)
		}
	}
	return parameters, nil
}

func applyStreamFilter(
	ctx context.Context,
	name string,
	data []byte,
	parameters *Dictionary,
	limit int64,
	resolve streamDecodeResolver,
) ([]byte, error) {
	var (
		decoded       []byte
		err           error
		usesPredictor bool
	)
	switch name {
	case "FlateDecode":
		usesPredictor = true
		var reader io.ReadCloser
		reader, err = zlib.NewReader(bytes.NewReader(data))
		if err == nil {
			defer func() { _ = reader.Close() }()
			decoded, err = readLimitedDecoded(ctx, reader, limit)
		}
	case "ASCII85Decode":
		var payload []byte
		payload, err = stripASCII85Framing(data)
		if err == nil {
			decoded, err = readLimitedDecoded(ctx, ascii85.NewDecoder(bytes.NewReader(payload)), limit)
		}
	case "ASCIIHexDecode":
		decoded, err = decodeASCIIHex(ctx, data, limit)
	case "RunLengthDecode":
		decoded, err = decodeRunLength(ctx, data, limit)
	case "LZWDecode":
		usesPredictor = true
		var earlyChange int
		earlyChange, err = decodeParameter(ctx, resolve, parameters, "EarlyChange", 1)
		if err == nil {
			decoded, err = decodePDFLZW(ctx, data, earlyChange, limit)
		}
	case "DCTDecode":
		// Parser.Stream preserves the Reader's established decoded-pixel
		// contract. ImageExtractor intentionally takes a separate path and keeps
		// the original JPEG payload for image export.
		if err = validateDCTOutputLimit(data, limit); err != nil {
			break
		}
		var colorTransform int
		colorTransform, err = decodeParameter(ctx, resolve, parameters, "ColorTransform", 1)
		if err == nil {
			decoder := pdfencoding.NewDCTDecoderWithParams(colorTransform)
			decoded, err = decoder.Decode(data)
		}
		if err == nil {
			err = ctx.Err()
		}
		if err == nil && int64(len(decoded)) > limit {
			err = fmt.Errorf("%w: DCTDecode produced more than %d bytes", ErrStreamDecodeLimit, limit)
		}
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedStreamFilter, name)
	}
	if err != nil {
		return nil, err
	}
	if usesPredictor {
		return applyStreamPredictor(ctx, decoded, parameters, resolve)
	}
	return decoded, nil
}

func validateDCTOutputLimit(data []byte, limit int64) error {
	config, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("DCTDecode: inspect dimensions: %w", err)
	}
	components := int64(3)
	if config.ColorModel == color.GrayModel {
		components = 1
	}
	width, height := int64(config.Width), int64(config.Height)
	if width <= 0 || height <= 0 || width > limit/components || height > limit/(width*components) {
		return fmt.Errorf("%w: DCTDecode dimensions %dx%d exceed %d decoded bytes", ErrStreamDecodeLimit, config.Width, config.Height, limit)
	}
	return nil
}

func readLimitedDecoded(ctx context.Context, reader io.Reader, limit int64) ([]byte, error) {
	decoded, err := io.ReadAll(io.LimitReader(&contextReader{ctx: ctx, reader: reader}, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(decoded)) > limit {
		return nil, fmt.Errorf("%w: decoded output exceeds %d bytes", ErrStreamDecodeLimit, limit)
	}
	return decoded, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader *contextReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(buffer)
}

func stripASCII85Framing(data []byte) ([]byte, error) {
	data = bytes.TrimSpace(data)
	if bytes.HasPrefix(data, []byte("<~")) {
		data = data[2:]
	}
	if end := bytes.Index(data, []byte("~>")); end >= 0 {
		if len(bytes.TrimSpace(data[end+2:])) != 0 {
			return nil, fmt.Errorf("ASCII85Decode: non-whitespace data after end marker")
		}
		data = data[:end]
	}
	return data, nil
}

func decodeASCIIHex(ctx context.Context, data []byte, limit int64) ([]byte, error) {
	decoded := make([]byte, 0, len(data)/2)
	highNibble := -1
	ended := false
	for index, value := range data {
		if index&4095 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		if isPDFWhitespace(value) {
			continue
		}
		if ended {
			return nil, fmt.Errorf("ASCIIHexDecode: non-whitespace data after end marker")
		}
		if value == '>' {
			ended = true
			continue
		}
		nibble, ok := hexNibble(value)
		if !ok {
			return nil, fmt.Errorf("ASCIIHexDecode: invalid byte 0x%02x", value)
		}
		if highNibble < 0 {
			highNibble = nibble
			continue
		}
		decoded = append(decoded, byte(highNibble<<4|nibble))
		highNibble = -1
		if int64(len(decoded)) > limit {
			return nil, fmt.Errorf("%w: ASCIIHexDecode output exceeds %d bytes", ErrStreamDecodeLimit, limit)
		}
	}
	if highNibble >= 0 {
		decoded = append(decoded, byte(highNibble<<4))
	}
	return decoded, nil
}

func hexNibble(value byte) (int, bool) {
	switch {
	case value >= '0' && value <= '9':
		return int(value - '0'), true
	case value >= 'a' && value <= 'f':
		return int(value-'a') + 10, true
	case value >= 'A' && value <= 'F':
		return int(value-'A') + 10, true
	default:
		return 0, false
	}
}

func isPDFWhitespace(value byte) bool {
	switch value {
	case 0, '\t', '\n', '\f', '\r', ' ':
		return true
	default:
		return false
	}
}

func decodeRunLength(ctx context.Context, data []byte, limit int64) ([]byte, error) {
	decoded := make([]byte, 0, len(data))
	for index := 0; index < len(data); {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		length := int(data[index])
		index++
		switch {
		case length == 128:
			return decoded, nil
		case length <= 127:
			count := length + 1
			if index+count > len(data) {
				return nil, io.ErrUnexpectedEOF
			}
			if int64(len(decoded)+count) > limit {
				return nil, fmt.Errorf("%w: RunLengthDecode output exceeds %d bytes", ErrStreamDecodeLimit, limit)
			}
			decoded = append(decoded, data[index:index+count]...)
			index += count
		default:
			if index >= len(data) {
				return nil, io.ErrUnexpectedEOF
			}
			count := 257 - length
			if int64(len(decoded)+count) > limit {
				return nil, fmt.Errorf("%w: RunLengthDecode output exceeds %d bytes", ErrStreamDecodeLimit, limit)
			}
			for range count {
				decoded = append(decoded, data[index])
			}
			index++
		}
	}
	return nil, fmt.Errorf("RunLengthDecode: missing end marker")
}

type msbCodeReader struct {
	data      []byte
	bitOffset int
}

func (reader *msbCodeReader) read(width int) (int, error) {
	if width <= 0 || reader.bitOffset+width > len(reader.data)*8 {
		return 0, io.ErrUnexpectedEOF
	}
	value := 0
	for range width {
		byteIndex := reader.bitOffset / 8
		bitIndex := 7 - reader.bitOffset%8
		value = value<<1 | int(reader.data[byteIndex]>>bitIndex&1)
		reader.bitOffset++
	}
	return value, nil
}

func decodePDFLZW(ctx context.Context, data []byte, earlyChange int, limit int64) ([]byte, error) {
	if earlyChange != 0 && earlyChange != 1 {
		return nil, fmt.Errorf("LZWDecode: EarlyChange must be 0 or 1, got %d", earlyChange)
	}
	reader := &msbCodeReader{data: data}
	dictionary := make([][]byte, 4096)
	reset := func() {
		clear(dictionary)
		for value := 0; value < 256; value++ {
			dictionary[value] = []byte{byte(value)}
		}
	}
	reset()
	codeWidth := 9
	nextCode := 258
	var previous []byte
	capacity := len(data)
	if capacity <= int(limit)/2 {
		capacity *= 2
	} else {
		capacity = int(limit)
	}
	decoded := make([]byte, 0, capacity)

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		code, err := reader.read(codeWidth)
		if err != nil {
			return nil, fmt.Errorf("LZWDecode: %w", err)
		}
		switch code {
		case 256:
			reset()
			codeWidth = 9
			nextCode = 258
			previous = nil
			continue
		case 257:
			return decoded, nil
		}

		var entry []byte
		if code < nextCode && dictionary[code] != nil {
			entry = dictionary[code]
		} else if code == nextCode && len(previous) > 0 {
			entry = append(append([]byte(nil), previous...), previous[0])
		} else {
			return nil, fmt.Errorf("LZWDecode: invalid code %d", code)
		}
		if int64(len(decoded)+len(entry)) > limit {
			return nil, fmt.Errorf("%w: LZWDecode output exceeds %d bytes", ErrStreamDecodeLimit, limit)
		}
		decoded = append(decoded, entry...)

		if len(previous) > 0 && nextCode < len(dictionary) {
			candidate := make([]byte, len(previous)+1)
			copy(candidate, previous)
			candidate[len(previous)] = entry[0]
			dictionary[nextCode] = candidate
			nextCode++
			if codeWidth < 12 && nextCode+earlyChange == 1<<codeWidth {
				codeWidth++
			}
		}
		previous = append(previous[:0], entry...)
	}
}

func applyStreamPredictor(
	ctx context.Context,
	data []byte,
	parameters *Dictionary,
	resolve streamDecodeResolver,
) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	predictor, err := decodeParameter(ctx, resolve, parameters, "Predictor", 1)
	if err != nil {
		return nil, err
	}
	if predictor <= 1 {
		return data, nil
	}
	colors, err := decodeParameter(ctx, resolve, parameters, "Colors", 1)
	if err != nil {
		return nil, err
	}
	bitsPerComponent, err := decodeParameter(ctx, resolve, parameters, "BitsPerComponent", 8)
	if err != nil {
		return nil, err
	}
	columns, err := decodeParameter(ctx, resolve, parameters, "Columns", 1)
	if err != nil {
		return nil, err
	}
	if colors <= 0 || columns <= 0 {
		return nil, fmt.Errorf("predictor Colors and Columns must be positive")
	}
	switch bitsPerComponent {
	case 1, 2, 4, 8, 16:
	default:
		return nil, fmt.Errorf("predictor BitsPerComponent %d is invalid; want 1, 2, 4, 8, or 16", bitsPerComponent)
	}
	if colors > 1_000_000 || columns > 1_000_000 || colors > 1_000_000/columns {
		return nil, fmt.Errorf("predictor row exceeds 1000000 samples")
	}
	samplesPerRow := colors * columns
	rowBits := int64(samplesPerRow) * int64(bitsPerComponent)
	rowBytes := int((rowBits + 7) / 8)
	if rowBytes > 1_000_000 {
		return nil, fmt.Errorf("predictor row size %d is invalid", rowBytes)
	}
	bytesPerPixel := max(1, (colors*bitsPerComponent+7)/8)
	switch {
	case predictor == 2:
		return applyTIFFPredictor(ctx, data, rowBytes, colors, columns, bitsPerComponent)
	case predictor >= 10 && predictor <= 15:
		return applyPNGPredictorBytesContext(ctx, data, rowBytes, bytesPerPixel)
	default:
		return nil, fmt.Errorf("unsupported predictor: %d", predictor)
	}
}

func decodeParameter(
	ctx context.Context,
	resolve streamDecodeResolver,
	parameters *Dictionary,
	key string,
	fallback int,
) (int, error) {
	if parameters == nil {
		return fallback, nil
	}
	parameter := parameters.Get(key)
	if parameter == nil {
		return fallback, nil
	}
	var err error
	parameter, err = resolve(ctx, parameter)
	if err != nil {
		return 0, fmt.Errorf("resolve stream DecodeParms %s: %w", key, err)
	}
	value, ok := parameter.(*Integer)
	if !ok {
		return 0, fmt.Errorf("stream DecodeParms %s is %T, want Integer", key, parameter)
	}
	converted := int(value.Value())
	if int64(converted) != value.Value() {
		return 0, fmt.Errorf("stream DecodeParms %s is outside the platform integer range", key)
	}
	return converted, nil
}

func applyTIFFPredictor(
	ctx context.Context,
	data []byte,
	rowBytes int,
	colors int,
	columns int,
	bitsPerComponent int,
) ([]byte, error) {
	if len(data)%rowBytes != 0 {
		return nil, fmt.Errorf("TIFF predictor data length %d is not divisible by row size %d", len(data), rowBytes)
	}
	decoded := append([]byte(nil), data...)
	for row := 0; row < len(decoded); row += rowBytes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rowData := decoded[row : row+rowBytes]
		for sample := colors; sample < colors*columns; sample++ {
			if sample&4095 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			current := readPackedSample(rowData, sample, bitsPerComponent)
			previous := readPackedSample(rowData, sample-colors, bitsPerComponent)
			mask := uint32(1<<bitsPerComponent) - 1
			writePackedSample(rowData, sample, bitsPerComponent, (current+previous)&mask)
		}
	}
	return decoded, nil
}

func readPackedSample(data []byte, sample, bitsPerComponent int) uint32 {
	bitOffset := sample * bitsPerComponent
	var value uint32
	for bit := 0; bit < bitsPerComponent; bit++ {
		absoluteBit := bitOffset + bit
		value = value<<1 | uint32(data[absoluteBit/8]>>(7-absoluteBit%8)&1)
	}
	return value
}

func writePackedSample(data []byte, sample, bitsPerComponent int, value uint32) {
	bitOffset := sample * bitsPerComponent
	for bit := 0; bit < bitsPerComponent; bit++ {
		absoluteBit := bitOffset + bit
		mask := byte(1 << (7 - absoluteBit%8))
		if value>>uint(bitsPerComponent-1-bit)&1 == 1 {
			data[absoluteBit/8] |= mask
		} else {
			data[absoluteBit/8] &^= mask
		}
	}
}

func applyPNGPredictorBytes(data []byte, rowBytes, bytesPerPixel int) ([]byte, error) {
	return applyPNGPredictorBytesContext(context.Background(), data, rowBytes, bytesPerPixel)
}

func applyPNGPredictorBytesContext(ctx context.Context, data []byte, rowBytes, bytesPerPixel int) ([]byte, error) {
	encodedRowBytes := rowBytes + 1
	if len(data)%encodedRowBytes != 0 {
		return nil, fmt.Errorf("PNG predictor data length %d is not divisible by row size %d", len(data), encodedRowBytes)
	}
	decoded := make([]byte, 0, len(data))
	previous := make([]byte, rowBytes)
	for offset := 0; offset < len(data); offset += encodedRowBytes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		filter := data[offset]
		encoded := data[offset+1 : offset+encodedRowBytes]
		row := make([]byte, rowBytes)
		for index, value := range encoded {
			var left, up, upperLeft byte
			if index >= bytesPerPixel {
				left = row[index-bytesPerPixel]
				upperLeft = previous[index-bytesPerPixel]
			}
			up = previous[index]
			switch filter {
			case 0:
				row[index] = value
			case 1:
				row[index] = value + left
			case 2:
				row[index] = value + up
			case 3:
				row[index] = value + byte((int(left)+int(up))/2)
			case 4:
				row[index] = value + paethPredictor(left, up, upperLeft)
			default:
				return nil, fmt.Errorf("PNG predictor filter %d is invalid", filter)
			}
		}
		decoded = append(decoded, row...)
		previous = row
	}
	return decoded, nil
}
