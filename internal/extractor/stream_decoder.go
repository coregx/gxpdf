package extractor

import (
	"context"

	"github.com/coregx/gxpdf/internal/parser"
)

// decodeStreamData is the FontExtractor adapter for the canonical bounded
// decoder owned by parser.Stream.
func decodeStreamData(stream *parser.Stream) ([]byte, error) {
	return decodeStreamDataWithContext(context.Background(), stream)
}

func decodeStreamDataWithContext(ctx context.Context, stream *parser.Stream) ([]byte, error) {
	return stream.DecodeWithContext(contextOrBackground(ctx), parser.DefaultStreamDecodeOptions())
}

func contextOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
