package extractor

import "github.com/coregx/gxpdf/internal/parser"

// resolveObject follows a single indirect reference using the reader's object
// table. Non-reference objects are returned as-is. It returns nil when the
// referenced object cannot be resolved.
func resolveObject(reader *parser.Reader, object parser.PdfObject) parser.PdfObject {
	reference, ok := object.(*parser.IndirectReference)
	if !ok {
		return object
	}
	resolved, err := reader.GetObject(reference.Number)
	if err != nil {
		return nil
	}
	return resolved
}
