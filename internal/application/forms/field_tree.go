package forms

import (
	"log/slog"

	"github.com/coregx/gxpdf/internal/parser"
	"github.com/coregx/gxpdf/logging"
)

// maxFieldDepth bounds the nesting of the AcroForm field hierarchy.
//
// ISO 32000-1 §12.7.3.1 sets no limit; real forms nest a handful of levels
// (one per component of the fully qualified field name). Anything deeper is
// treated as malformed and not followed.
const maxFieldDepth = 64

// fieldTreeGuard bounds a walk over the field hierarchy (/Fields and /Kids).
//
// The hierarchy is untrusted input: /Kids may reference an ancestor (a cycle),
// list the same field more than once, or nest without bound. Every walker in
// this package goes through enter, which admits each field at most once and
// stops descending at maxFieldDepth, so a walk is bounded by the number of
// objects in the file.
//
// A field is identified both by object number and by dictionary pointer:
// /Fields often arrives already resolved (GetAcroForm resolves one level), so
// the same field can be reached once as a dictionary and later through an
// indirect reference; parser.Reader.GetObject returns the same cached
// dictionary for both.
type fieldTreeGuard struct {
	pdf          *parser.Reader
	visitedRefs  map[int]struct{}
	visitedDicts map[*parser.Dictionary]struct{}
}

func newFieldTreeGuard(pdf *parser.Reader) *fieldTreeGuard {
	return &fieldTreeGuard{
		pdf:          pdf,
		visitedRefs:  make(map[int]struct{}),
		visitedDicts: make(map[*parser.Dictionary]struct{}),
	}
}

// enter resolves one /Fields or /Kids entry to its field dictionary.
//
// depth is the nesting level of the entry (0 for /Fields). It reports false
// when the entry must not be visited: nested too deeply, already visited
// (cycle or duplicate), unresolvable, or not a dictionary. The returned
// dictionary is the document's own object (not a copy).
func (g *fieldTreeGuard) enter(obj parser.PdfObject, depth int) (*parser.Dictionary, bool) {
	objNum := 0
	ref, isRef := obj.(*parser.IndirectReference)
	if isRef {
		objNum = ref.Number
	}
	if depth > maxFieldDepth {
		g.warn("form field nested too deeply, skipped", objNum, slog.Int("depth", depth))
		return nil, false
	}

	if isRef {
		if _, seen := g.visitedRefs[objNum]; seen {
			g.warn("form field referenced more than once (cycle or duplicate), skipped", objNum)
			return nil, false
		}
		g.visitedRefs[objNum] = struct{}{}

		resolved, err := g.pdf.GetObject(objNum)
		if err != nil {
			g.warn("form field could not be resolved, skipped", objNum, slog.String("error", err.Error()))
			return nil, false
		}
		obj = resolved
	}

	dict, ok := obj.(*parser.Dictionary)
	if !ok {
		return nil, false
	}
	if _, seen := g.visitedDicts[dict]; seen {
		g.warn("form field referenced more than once (cycle or duplicate), skipped", objNum)
		return nil, false
	}
	g.visitedDicts[dict] = struct{}{}
	return dict, true
}

func (g *fieldTreeGuard) warn(msg string, objNum int, attrs ...any) {
	logging.Logger().Warn(msg, append([]any{slog.Int("object", objNum)}, attrs...)...)
}
