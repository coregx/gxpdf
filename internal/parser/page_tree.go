package parser

import (
	"fmt"
	"log/slog"

	"github.com/coregx/gxpdf/logging"
)

// maxPageTreeDepth bounds the nesting of /Pages nodes.
//
// ISO 32000-1 §7.7.3 sets no limit, but conforming writers produce shallow,
// balanced trees (a few levels even for very large documents). A hostile or
// corrupted file can chain nodes arbitrarily deep; anything below this depth
// is skipped instead of being followed.
const maxPageTreeDepth = 256

// pageIndex is the flattened, document-ordered list of page (leaf) dictionaries.
//
// It is built once per Reader from the page tree and is the single source of
// truth for page lookup and page count. The tree is untrusted input, so the
// walk is iterative (no recursion) and tolerates:
//   - cycles and repeated nodes (a node reachable twice is visited once),
//   - excessive depth (see maxPageTreeDepth),
//   - unresolvable or malformed kids.
//
// Offending nodes are skipped and logged; the remaining pages stay usable.
// The declared /Count is not trusted: a page count must not be inflated by a
// single integer in the file.
type pageIndex struct {
	pages []*Dictionary
}

// pageTreeWalker carries the state of one page tree traversal.
type pageTreeWalker struct {
	r *Reader

	// Node identity: indirect nodes by object number, direct (inline) nodes by
	// pointer. A node seen twice is either a cycle or a duplicate kid; both are
	// skipped so the walk is bounded by the number of distinct nodes.
	visitedRefs  map[int]struct{}
	visitedDicts map[*Dictionary]struct{}

	pages []*Dictionary
}

// pageTreeFrame is one /Pages node whose /Kids are being visited.
type pageTreeFrame struct {
	kids  *Array
	next  int
	depth int
}

// buildPageIndex walks the page tree rooted at r.pages in document order.
func (r *Reader) buildPageIndex() *pageIndex {
	w := &pageTreeWalker{
		r:            r,
		visitedRefs:  make(map[int]struct{}),
		visitedDicts: make(map[*Dictionary]struct{}),
	}
	if r.pagesRootRef > 0 {
		w.visitedRefs[r.pagesRootRef] = struct{}{}
	}
	w.visitedDicts[r.pages] = struct{}{}

	var stack []pageTreeFrame
	if kids := w.kidsOf(r.pages, r.pagesRootRef); kids != nil {
		stack = append(stack, pageTreeFrame{kids: kids, depth: 1})
	}

	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if top.next >= top.kids.Len() {
			stack = stack[:len(stack)-1]
			continue
		}
		kidObj := top.kids.Get(top.next)
		top.next++
		depth := top.depth

		node, objNum, ok := w.resolveKid(kidObj)
		if !ok {
			continue
		}

		switch classifyPageTreeNode(node) {
		case pageTreeLeaf:
			w.pages = append(w.pages, node)
		case pageTreeIntermediate:
			if depth >= maxPageTreeDepth {
				w.warn("page tree too deep, subtree skipped", objNum, slog.Int("depth", depth))
				continue
			}
			if kids := w.kidsOf(node, objNum); kids != nil {
				stack = append(stack, pageTreeFrame{kids: kids, depth: depth + 1})
			}
		default:
			w.warn("page tree node has unexpected /Type, skipped", objNum)
		}
	}

	return &pageIndex{pages: w.pages}
}

// resolveKid resolves one /Kids entry and enforces node uniqueness.
// It returns the node, its object number (0 for direct nodes) and whether the
// node should be visited.
func (w *pageTreeWalker) resolveKid(kidObj PdfObject) (*Dictionary, int, bool) {
	objNum := 0
	if ref, ok := kidObj.(*IndirectReference); ok {
		objNum = ref.Number
		if _, seen := w.visitedRefs[objNum]; seen {
			w.warn("page tree node referenced more than once (cycle or duplicate), skipped", objNum)
			return nil, objNum, false
		}
		w.visitedRefs[objNum] = struct{}{}
	}

	node, err := w.r.resolveDictionary(kidObj)
	if err != nil {
		w.warn("page tree kid could not be resolved, skipped", objNum, slog.String("error", err.Error()))
		return nil, objNum, false
	}
	if _, seen := w.visitedDicts[node]; seen {
		w.warn("page tree node referenced more than once (cycle or duplicate), skipped", objNum)
		return nil, objNum, false
	}
	w.visitedDicts[node] = struct{}{}
	return node, objNum, true
}

// kidsOf resolves the /Kids array of an intermediate node.
func (w *pageTreeWalker) kidsOf(node *Dictionary, objNum int) *Array {
	kidsObj := node.Get("Kids")
	if kidsObj == nil {
		w.warn("pages node missing /Kids, skipped", objNum)
		return nil
	}
	kids, err := w.r.resolveArray(kidsObj)
	if err != nil {
		w.warn("pages node /Kids could not be resolved, skipped", objNum, slog.String("error", err.Error()))
		return nil
	}
	return kids
}

func (w *pageTreeWalker) warn(msg string, objNum int, attrs ...any) {
	args := append([]any{slog.Int("object", objNum), slog.String("file", w.r.filename)}, attrs...)
	logging.Logger().Warn(msg, args...)
}

// pageTreeNodeKind classifies a page tree node.
type pageTreeNodeKind int

const (
	pageTreeInvalid pageTreeNodeKind = iota
	pageTreeLeaf
	pageTreeIntermediate
)

// classifyPageTreeNode decides whether a node is a page or a /Pages node.
//
// /Type is required by the specification but is missing in some real-world
// files; in that case the presence of /Kids decides. A node with an explicit
// but foreign /Type (e.g. /Font) is invalid.
func classifyPageTreeNode(node *Dictionary) pageTreeNodeKind {
	if typeName := node.GetName("Type"); typeName != nil {
		switch typeName.Value() {
		case nodeTypePage:
			return pageTreeLeaf
		case nodeTypePages:
			return pageTreeIntermediate
		default:
			return pageTreeInvalid
		}
	}
	if node.Get("Kids") != nil {
		return pageTreeIntermediate
	}
	return pageTreeLeaf
}

// pageIndexOrBuild returns the page index, building it on first use.
// Safe for concurrent use.
func (r *Reader) pageIndexOrBuild() (*pageIndex, error) {
	if r.pages == nil {
		return nil, fmt.Errorf("pages not loaded (call Open first)")
	}
	r.pageIndexOnce.Do(func() {
		r.pageIdx = r.buildPageIndex()
		if declared := r.pages.GetInteger("Count"); declared != int64(len(r.pageIdx.pages)) {
			logging.Logger().Warn("page tree /Count does not match the number of pages found",
				slog.Int64("declared", declared),
				slog.Int("found", len(r.pageIdx.pages)),
				slog.String("file", r.filename))
		}
	})
	return r.pageIdx, nil
}
