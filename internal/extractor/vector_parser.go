package extractor

import (
	"context"
	"fmt"
	"math"

	"github.com/coregx/gxpdf/internal/parser"
)

// vectorGraphicsState is the full mutable PDF graphics state tracked during
// vector path extraction.
//
// This state is pushed/popped by the q/Q operators and updated by cm, w, J,
// j, M, RG, rg, G, g, K, k, and gs operators.
//
// Reference: PDF 1.7 specification, Section 8.4 (Graphics State).
type vectorGraphicsState struct {
	// CTM is the Current Transformation Matrix in page space.
	// Initialized to identity; updated by the cm operator.
	// Format: [a b c d e f] as in PDF spec Section 8.3.3.
	CTM Matrix

	// StrokeColor is the current stroke color in RGBA (0–1, alpha always 1.0
	// unless modified by ExtGState /CA).
	StrokeColor [4]float64

	// FillColor is the current fill color in RGBA (0–1, alpha always 1.0
	// unless modified by ExtGState /ca).
	FillColor [4]float64

	// LineWidth is the current line width in points.
	LineWidth float64

	// LineCap is the PDF line cap style (0=butt, 1=round, 2=square).
	LineCap int

	// LineJoin is the PDF line join style (0=miter, 1=round, 2=bevel).
	LineJoin int

	// MiterLimit is the miter join limit.
	MiterLimit float64

	// StrokeOpacity is the stroke opacity from ExtGState /CA (0–1).
	StrokeOpacity float64

	// FillOpacity is the fill opacity from ExtGState /ca (0–1).
	FillOpacity float64
}

// newVectorGraphicsState returns a graphics state with PDF defaults.
//
// Reference: PDF 1.7 specification, Table 51 (Device-Dependent Graphics State
// Parameters) and Table 53 (Device-Independent Graphics State Parameters).
func newVectorGraphicsState() vectorGraphicsState {
	return vectorGraphicsState{
		CTM:           Identity(),
		StrokeColor:   [4]float64{0, 0, 0, 1}, // black
		FillColor:     [4]float64{0, 0, 0, 1}, // black
		LineWidth:     1.0,
		LineCap:       0,
		LineJoin:      0,
		MiterLimit:    10.0,
		StrokeOpacity: 1.0,
		FillOpacity:   1.0,
	}
}

// vectorStateStack implements the graphics state save/restore mechanism
// driven by the q (save) and Q (restore) PDF operators.
//
// Reference: PDF 1.7 specification, Section 8.4.4 (Graphics State Operators).
type vectorStateStack struct {
	states []vectorGraphicsState
}

// save pushes a copy of gs onto the stack.
func (s *vectorStateStack) save(gs vectorGraphicsState) {
	s.states = append(s.states, gs)
}

// restore pops the top of the stack.
//
// Returns false and leaves gs unchanged when the stack is empty (malformed PDF
// guard — Q without matching q).
func (s *vectorStateStack) restore() (vectorGraphicsState, bool) {
	if len(s.states) == 0 {
		return vectorGraphicsState{}, false
	}
	top := s.states[len(s.states)-1]
	s.states = s.states[:len(s.states)-1]
	return top, true
}

// pathSegment stores a single segment of the path being built.
type pathSegment struct {
	verb   PathVerb
	coords []float64 // already transformed to page space
}

// VectorParser extracts vector graphics paths from PDF content streams.
//
// It is separate from GraphicsParser (which serves table-line detection)
// and adds full CTM tracking, graphics-state stack, curve operators,
// fill capture, opacity, and CMYK conversion.
//
// Usage:
//
//	vp := NewVectorParser(reader)
//	paths, err := vp.ParseFromPage(0)
type VectorParser struct {
	reader   *parser.Reader
	ctx      context.Context
	state    vectorGraphicsState
	stack    vectorStateStack
	curPath  []pathSegment // path being assembled between m..h and S/f/B
	curStart Point         // subpath start (for h/close)
	curPoint Point         // current pen position
	hasCur   bool          // whether curPoint is valid
	paths    []*VectorPath
	pageNum  int
	pageRes  *parser.Dictionary // current page Resources dict (for gs lookup)
}

// NewVectorParser creates a VectorParser for the given PDF reader.
func NewVectorParser(reader *parser.Reader) *VectorParser {
	return NewVectorParserWithContext(reader, context.Background())
}

// NewVectorParserWithContext creates a VectorParser whose stream decoding
// observes ctx.
func NewVectorParserWithContext(reader *parser.Reader, ctx context.Context) *VectorParser {
	return &VectorParser{
		reader: reader,
		ctx:    contextOrBackground(ctx),
		state:  newVectorGraphicsState(),
	}
}

// ParseFromPage extracts all vector paths from the specified page.
//
// Page numbers are 0-based (first page is 0).
//
// Returns a slice of VectorPath values, or an error if extraction fails.
// An empty page returns an empty (non-nil) slice.
func (vp *VectorParser) ParseFromPage(pageNum int) ([]*VectorPath, error) {
	if err := contextOrBackground(vp.ctx).Err(); err != nil {
		return nil, err
	}
	// Reset per-page state.
	vp.state = newVectorGraphicsState()
	vp.stack = vectorStateStack{}
	vp.curPath = nil
	vp.hasCur = false
	vp.paths = nil
	vp.pageNum = pageNum

	page, err := vp.reader.GetPage(pageNum)
	if err != nil {
		return nil, fmt.Errorf("failed to get page %d: %w", pageNum, err)
	}

	// Load page resources so gs operator can look up ExtGState entries.
	vp.pageRes = vp.getPageResources(page)

	contentData, err := vp.getPageContent(page)
	if err != nil {
		return nil, fmt.Errorf("failed to get page content: %w", err)
	}

	if len(contentData) == 0 {
		return []*VectorPath{}, nil
	}

	contentParser := NewContentParser(contentData)
	operators, err := contentParser.ParseOperators()
	if err != nil {
		return nil, fmt.Errorf("failed to parse content stream: %w", err)
	}

	for _, op := range operators {
		vp.processVectorOperator(op)
	}

	return vp.paths, nil
}

// getPageResources retrieves the Resources dictionary from a page dictionary.
//
// Resources can be inherited from parent page-tree nodes, but for the
// ExtGState lookup we only need to inspect the page's own resources.
// Inherited resources would require a recursive tree walk which we
// deliberately omit to keep Phase 1 simple.
func (vp *VectorParser) getPageResources(page *parser.Dictionary) *parser.Dictionary {
	resObj := page.Get("Resources")
	if resObj == nil {
		return parser.NewDictionary()
	}
	if ref, ok := resObj.(*parser.IndirectReference); ok {
		resolved, err := vp.reader.GetObject(ref.Number)
		if err == nil {
			if d, ok := resolved.(*parser.Dictionary); ok {
				return d
			}
		}
	}
	if d, ok := resObj.(*parser.Dictionary); ok {
		return d
	}
	return parser.NewDictionary()
}

// getPageContent retrieves and concatenates content stream bytes for a page.
//
//nolint:cyclop,dupl // Similar to GraphicsParser.getPageContent, refactoring later.
func (vp *VectorParser) getPageContent(page *parser.Dictionary) ([]byte, error) {
	contentsObj := page.Get("Contents")
	if contentsObj == nil {
		return []byte{}, nil
	}

	if ref, ok := contentsObj.(*parser.IndirectReference); ok {
		resolved, err := vp.reader.GetObject(ref.Number)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve contents reference: %w", err)
		}
		contentsObj = resolved
	}

	switch obj := contentsObj.(type) {
	case *parser.Stream:
		return vp.decodeStream(obj)

	case *parser.Array:
		var all []byte
		for i := 0; i < obj.Len(); i++ {
			streamRef := obj.Get(i)
			if streamRef == nil {
				return nil, fmt.Errorf("content stream %d is null", i)
			}
			if ref, ok := streamRef.(*parser.IndirectReference); ok {
				resolved, err := vp.reader.GetObject(ref.Number)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve content stream %d: %w", i, err)
				}
				streamRef = resolved
			}
			stream, ok := streamRef.(*parser.Stream)
			if !ok {
				return nil, fmt.Errorf("content stream %d is %T, want Stream", i, streamRef)
			}
			content, err := vp.decodeStream(stream)
			if err != nil {
				return nil, fmt.Errorf("failed to decode content stream %d: %w", i, err)
			}
			all = append(all, content...)
			all = append(all, ' ')
		}
		return all, nil

	default:
		return nil, fmt.Errorf("unexpected Contents type: %T", obj)
	}
}

// decodeStream delegates to the parser's canonical bounded filter pipeline.
func (vp *VectorParser) decodeStream(stream *parser.Stream) ([]byte, error) {
	return stream.DecodeWithContext(contextOrBackground(vp.ctx), parser.DefaultStreamDecodeOptions())
}

// processVectorOperator dispatches a single content-stream operator.
//
//nolint:cyclop,funlen // Operator dispatch requires many cases by design.
func (vp *VectorParser) processVectorOperator(op *Operator) {
	switch op.Name {

	// ── Graphics state: save / restore / CTM ──────────────────────────────

	case "q": // Save graphics state → push stack.
		vp.stack.save(vp.state)

	case "Q": // Restore graphics state → pop stack.
		if gs, ok := vp.stack.restore(); ok {
			vp.state = gs
		}

	case "cm": // Modify CTM: CTM = operand × CTM.
		if len(op.Operands) >= 6 {
			a := getNumber(op.Operands[0])
			b := getNumber(op.Operands[1])
			c := getNumber(op.Operands[2])
			d := getNumber(op.Operands[3])
			e := getNumber(op.Operands[4])
			f := getNumber(op.Operands[5])
			if a != nil && b != nil && c != nil && d != nil && e != nil && f != nil {
				operand := NewMatrix(*a, *b, *c, *d, *e, *f)
				// PDF spec: new CTM = operand × old CTM.
				vp.state.CTM = operand.Multiply(vp.state.CTM)
			}
		}

	// ── Graphics state parameters ──────────────────────────────────────────

	case "w": // Set line width.
		if len(op.Operands) >= 1 {
			if v := getNumber(op.Operands[0]); v != nil {
				vp.state.LineWidth = *v
			}
		}

	case "J": // Set line cap style (0, 1, or 2).
		if len(op.Operands) >= 1 {
			if v := getNumber(op.Operands[0]); v != nil {
				vp.state.LineCap = int(*v)
			}
		}

	case "j": // Set line join style (0, 1, or 2).
		if len(op.Operands) >= 1 {
			if v := getNumber(op.Operands[0]); v != nil {
				vp.state.LineJoin = int(*v)
			}
		}

	case "M": // Set miter limit.
		if len(op.Operands) >= 1 {
			if v := getNumber(op.Operands[0]); v != nil {
				vp.state.MiterLimit = *v
			}
		}

	case "d": // Set dash pattern — stored implicitly; we don't expose it yet.
		// No-op for Phase 1; the operator is consumed to keep state consistent.

	case "gs": // Apply named ExtGState.
		if len(op.Operands) >= 1 {
			if name, ok := op.Operands[0].(*parser.Name); ok {
				vp.applyExtGState(name.Value())
			}
		}

	// ── Color operators ────────────────────────────────────────────────────

	case "RG": // Set RGB stroke color.
		if len(op.Operands) >= 3 {
			r, g, b := getNumber(op.Operands[0]), getNumber(op.Operands[1]), getNumber(op.Operands[2])
			if r != nil && g != nil && b != nil {
				vp.state.StrokeColor = [4]float64{clamp01(*r), clamp01(*g), clamp01(*b), vp.state.StrokeColor[3]}
			}
		}

	case "rg": // Set RGB fill color.
		if len(op.Operands) >= 3 {
			r, g, b := getNumber(op.Operands[0]), getNumber(op.Operands[1]), getNumber(op.Operands[2])
			if r != nil && g != nil && b != nil {
				vp.state.FillColor = [4]float64{clamp01(*r), clamp01(*g), clamp01(*b), vp.state.FillColor[3]}
			}
		}

	case "G": // Set grayscale stroke color.
		if len(op.Operands) >= 1 {
			if gray := getNumber(op.Operands[0]); gray != nil {
				g := clamp01(*gray)
				vp.state.StrokeColor = [4]float64{g, g, g, vp.state.StrokeColor[3]}
			}
		}

	case "g": // Set grayscale fill color.
		if len(op.Operands) >= 1 {
			if gray := getNumber(op.Operands[0]); gray != nil {
				g := clamp01(*gray)
				vp.state.FillColor = [4]float64{g, g, g, vp.state.FillColor[3]}
			}
		}

	case "K": // Set CMYK stroke color.
		if len(op.Operands) >= 4 {
			c, m, y, k := getNumber(op.Operands[0]), getNumber(op.Operands[1]),
				getNumber(op.Operands[2]), getNumber(op.Operands[3])
			if c != nil && m != nil && y != nil && k != nil {
				r, g, b := cmykToRGB(*c, *m, *y, *k)
				vp.state.StrokeColor = [4]float64{r, g, b, vp.state.StrokeColor[3]}
			}
		}

	case "k": // Set CMYK fill color.
		if len(op.Operands) >= 4 {
			c, m, y, k := getNumber(op.Operands[0]), getNumber(op.Operands[1]),
				getNumber(op.Operands[2]), getNumber(op.Operands[3])
			if c != nil && m != nil && y != nil && k != nil {
				r, g, b := cmykToRGB(*c, *m, *y, *k)
				vp.state.FillColor = [4]float64{r, g, b, vp.state.FillColor[3]}
			}
		}

	// ── Path construction ──────────────────────────────────────────────────

	case "m": // moveto.
		if len(op.Operands) >= 2 {
			x, y := getNumber(op.Operands[0]), getNumber(op.Operands[1])
			if x != nil && y != nil {
				ux, uy := vp.state.CTM.Transform(*x, *y)
				vp.curPath = append(vp.curPath, pathSegment{VerbMoveTo, []float64{ux, uy}})
				vp.curPoint = NewPoint(ux, uy)
				vp.curStart = vp.curPoint
				vp.hasCur = true
			}
		}

	case "l": // lineto.
		if len(op.Operands) >= 2 {
			x, y := getNumber(op.Operands[0]), getNumber(op.Operands[1])
			if x != nil && y != nil {
				ux, uy := vp.state.CTM.Transform(*x, *y)
				vp.curPath = append(vp.curPath, pathSegment{VerbLineTo, []float64{ux, uy}})
				vp.curPoint = NewPoint(ux, uy)
				vp.hasCur = true
			}
		}

	case "c": // cubic Bézier: c1x c1y c2x c2y x y.
		if len(op.Operands) >= 6 {
			vp.appendCubic(
				op.Operands[0], op.Operands[1],
				op.Operands[2], op.Operands[3],
				op.Operands[4], op.Operands[5],
			)
		}

	case "v": // cubic Bézier shorthand: c1 = current point.
		// Operands: c2x c2y x y.
		if len(op.Operands) >= 4 && vp.hasCur {
			// c1 is the current point (already in page space).
			c1x, c1y := vp.curPoint.X, vp.curPoint.Y
			c2x, c2y := getNumber(op.Operands[0]), getNumber(op.Operands[1])
			ex, ey := getNumber(op.Operands[2]), getNumber(op.Operands[3])
			if c2x != nil && c2y != nil && ex != nil && ey != nil {
				tc2x, tc2y := vp.state.CTM.Transform(*c2x, *c2y)
				tex, tey := vp.state.CTM.Transform(*ex, *ey)
				vp.curPath = append(vp.curPath, pathSegment{
					VerbCubicTo,
					[]float64{c1x, c1y, tc2x, tc2y, tex, tey},
				})
				vp.curPoint = NewPoint(tex, tey)
				vp.hasCur = true
			}
		}

	case "y": // cubic Bézier shorthand: c2 = endpoint.
		// Operands: c1x c1y x y.
		if len(op.Operands) >= 4 {
			c1x, c1y := getNumber(op.Operands[0]), getNumber(op.Operands[1])
			ex, ey := getNumber(op.Operands[2]), getNumber(op.Operands[3])
			if c1x != nil && c1y != nil && ex != nil && ey != nil {
				tc1x, tc1y := vp.state.CTM.Transform(*c1x, *c1y)
				tex, tey := vp.state.CTM.Transform(*ex, *ey)
				// c2 = endpoint (already in page space).
				vp.curPath = append(vp.curPath, pathSegment{
					VerbCubicTo,
					[]float64{tc1x, tc1y, tex, tey, tex, tey},
				})
				vp.curPoint = NewPoint(tex, tey)
				vp.hasCur = true
			}
		}

	case "re": // rectangle: x y w h.
		if len(op.Operands) >= 4 {
			vp.appendRectangle(
				op.Operands[0], op.Operands[1],
				op.Operands[2], op.Operands[3],
			)
		}

	case "h": // close subpath.
		if vp.hasCur {
			vp.curPath = append(vp.curPath, pathSegment{VerbClose, nil})
			vp.curPoint = vp.curStart
		}

	// ── Path painting ──────────────────────────────────────────────────────

	case "S": // Stroke.
		vp.emitPath(PaintStroke)

	case "s": // Close + stroke.
		if vp.hasCur {
			vp.curPath = append(vp.curPath, pathSegment{VerbClose, nil})
		}
		vp.emitPath(PaintStroke)

	case "f", "F": // Fill (non-zero winding).
		vp.emitPath(PaintFill)

	case "f*": // Fill (even-odd).
		vp.emitPath(PaintFill)

	case "B": // Fill + stroke (non-zero winding).
		vp.emitPath(PaintFillStroke)

	case "B*": // Fill + stroke (even-odd).
		vp.emitPath(PaintFillStroke)

	case "b": // Close + fill + stroke (non-zero).
		if vp.hasCur {
			vp.curPath = append(vp.curPath, pathSegment{VerbClose, nil})
		}
		vp.emitPath(PaintFillStroke)

	case "b*": // Close + fill + stroke (even-odd).
		if vp.hasCur {
			vp.curPath = append(vp.curPath, pathSegment{VerbClose, nil})
		}
		vp.emitPath(PaintFillStroke)

	case "n": // End path without painting (clip-only, discard).
		vp.clearCurPath()

	// ── Clipping (consume to maintain correct state) ───────────────────────

	case "W", "W*": // Set clip path — we discard the path but handle the operator.
		vp.clearCurPath()
	}
}

// appendCubic transforms and records a full cubic Bézier segment.
func (vp *VectorParser) appendCubic(c1xObj, c1yObj, c2xObj, c2yObj, exObj, eyObj parser.PdfObject) {
	c1x, c1y := getNumber(c1xObj), getNumber(c1yObj)
	c2x, c2y := getNumber(c2xObj), getNumber(c2yObj)
	ex, ey := getNumber(exObj), getNumber(eyObj)
	if c1x == nil || c1y == nil || c2x == nil || c2y == nil || ex == nil || ey == nil {
		return
	}
	tc1x, tc1y := vp.state.CTM.Transform(*c1x, *c1y)
	tc2x, tc2y := vp.state.CTM.Transform(*c2x, *c2y)
	tex, tey := vp.state.CTM.Transform(*ex, *ey)
	vp.curPath = append(vp.curPath, pathSegment{
		VerbCubicTo,
		[]float64{tc1x, tc1y, tc2x, tc2y, tex, tey},
	})
	vp.curPoint = NewPoint(tex, tey)
	vp.hasCur = true
}

// appendRectangle expands a rectangle into path segments.
//
// PDF re operator: x y w h → produces the equivalent of:
//
//	m x y  l x+w y  l x+w y+h  l x y+h  h
//
// All corner points are transformed through the CTM.
func (vp *VectorParser) appendRectangle(xObj, yObj, wObj, hObj parser.PdfObject) {
	x, y := getNumber(xObj), getNumber(yObj)
	w, h := getNumber(wObj), getNumber(hObj)
	if x == nil || y == nil || w == nil || h == nil {
		return
	}

	// Four corners in user (pre-CTM) space, then transformed.
	x0, y0 := *x, *y
	x1, y1 := *x+*w, *y
	x2, y2 := *x+*w, *y+*h
	x3, y3 := *x, *y+*h

	tx0, ty0 := vp.state.CTM.Transform(x0, y0)
	tx1, ty1 := vp.state.CTM.Transform(x1, y1)
	tx2, ty2 := vp.state.CTM.Transform(x2, y2)
	tx3, ty3 := vp.state.CTM.Transform(x3, y3)

	startPt := NewPoint(tx0, ty0)

	vp.curPath = append(vp.curPath,
		pathSegment{VerbMoveTo, []float64{tx0, ty0}},
		pathSegment{VerbLineTo, []float64{tx1, ty1}},
		pathSegment{VerbLineTo, []float64{tx2, ty2}},
		pathSegment{VerbLineTo, []float64{tx3, ty3}},
		pathSegment{VerbClose, nil},
	)
	vp.curPoint = startPt
	vp.curStart = startPt
	vp.hasCur = true
}

// emitPath converts the accumulated curPath into a VectorPath and resets state.
func (vp *VectorParser) emitPath(mode PaintMode) {
	if len(vp.curPath) == 0 {
		return
	}

	path := &VectorPath{
		PageNum:     vp.pageNum,
		LineWidth:   vp.state.LineWidth,
		LineCap:     vp.state.LineCap,
		LineJoin:    vp.state.LineJoin,
		MiterLimit:  vp.state.MiterLimit,
		PaintMode:   mode,
		StrokeColor: vp.state.StrokeColor,
		FillColor:   vp.state.FillColor,
	}

	// Apply opacity: use the minimum of stroke and fill opacities so that
	// a FillStroke path carries the most restrictive value.
	opacity := vp.state.StrokeOpacity
	if vp.state.FillOpacity < opacity {
		opacity = vp.state.FillOpacity
	}
	path.Opacity = opacity

	// Set alpha channel on the exported colors.
	path.StrokeColor[3] = vp.state.StrokeOpacity
	path.FillColor[3] = vp.state.FillOpacity

	// Flatten verbs and coords.
	for _, seg := range vp.curPath {
		path.Verbs = append(path.Verbs, seg.verb)
		path.Coords = append(path.Coords, seg.coords...)
	}

	vp.paths = append(vp.paths, path)
	vp.clearCurPath()
}

// clearCurPath resets the current path construction state.
func (vp *VectorParser) clearCurPath() {
	vp.curPath = nil
	vp.hasCur = false
}

// applyExtGState looks up a named graphics state resource and applies
// the /ca (fill opacity) and /CA (stroke opacity) entries to the current state.
//
// Only these two keys are relevant for Phase 1. All other ExtGState parameters
// (blend mode, overprint, etc.) are deferred.
//
// Reference: PDF 1.7 specification, Section 8.4.5 (Graphics State Parameter
// Dictionaries) and Table 57 (Entries in a Graphics State Parameter Dictionary).
func (vp *VectorParser) applyExtGState(name string) {
	if vp.pageRes == nil {
		return
	}

	// Navigate Resources → ExtGState → <name>.
	extGObj := vp.pageRes.Get("ExtGState")
	if extGObj == nil {
		return
	}

	// Resolve indirect reference if needed.
	if ref, ok := extGObj.(*parser.IndirectReference); ok {
		resolved, err := vp.reader.GetObject(ref.Number)
		if err != nil {
			return
		}
		extGObj = resolved
	}

	extGDict, ok := extGObj.(*parser.Dictionary)
	if !ok {
		return
	}

	gsObj := extGDict.Get(name)
	if gsObj == nil {
		return
	}

	// Resolve indirect reference for the graphics state dict itself.
	if ref, ok := gsObj.(*parser.IndirectReference); ok {
		resolved, err := vp.reader.GetObject(ref.Number)
		if err != nil {
			return
		}
		gsObj = resolved
	}

	gsDict, ok := gsObj.(*parser.Dictionary)
	if !ok {
		return
	}

	// /CA — stroke opacity (capital CA per PDF spec).
	if caObj := gsDict.Get("CA"); caObj != nil {
		if v := getNumber(caObj); v != nil {
			vp.state.StrokeOpacity = clamp01(*v)
		}
	}

	// /ca — fill opacity (lowercase ca per PDF spec).
	if caObj := gsDict.Get("ca"); caObj != nil {
		if v := getNumber(caObj); v != nil {
			vp.state.FillOpacity = clamp01(*v)
		}
	}
}

// ── Color helpers ─────────────────────────────────────────────────────────────

// cmykToRGB converts CMYK (0–1) to RGB (0–1).
//
// Formula: R = 1 − min(1, C+K), G = 1 − min(1, M+K), B = 1 − min(1, Y+K).
//
// Reference: PDF 1.7 specification, Section 10.3.5 (Conversion between Device
// Color Spaces).
func cmykToRGB(c, m, y, k float64) (r, g, b float64) {
	r = 1 - math.Min(1, c+k)
	g = 1 - math.Min(1, m+k)
	b = 1 - math.Min(1, y+k)
	return r, g, b
}

// clamp01 restricts v to [0, 1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
