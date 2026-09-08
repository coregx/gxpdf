package extractor

import "github.com/coregx/gxpdf/internal/parser"

// decodeStreamData is the FontExtractor adapter for the canonical bounded
// decoder owned by parser.Stream.
func decodeStreamData(stream *parser.Stream) ([]byte, error) {
	return stream.Decode()
}
