package extractor

import (
	"testing"

	"github.com/coregx/gxpdf/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── PathVerb tests ────────────────────────────────────────────────────────────

func TestPathVerb_String(t *testing.T) {
	tests := []struct {
		verb PathVerb
		want string
	}{
		{VerbMoveTo, "MoveTo"},
		{VerbLineTo, "LineTo"},
		{VerbCubicTo, "CubicTo"},
		{VerbQuadTo, "QuadTo"},
		{VerbClose, "Close"},
		{PathVerb(99), "PathVerb(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.verb.String())
		})
	}
}

func TestVerbCoordCount(t *testing.T) {
	tests := []struct {
		verb  PathVerb
		count int
	}{
		{VerbMoveTo, 2},
		{VerbLineTo, 2},
		{VerbCubicTo, 6},
		{VerbQuadTo, 4},
		{VerbClose, 0},
		{PathVerb(99), 0},
	}

	for _, tt := range tests {
		t.Run(tt.verb.String(), func(t *testing.T) {
			assert.Equal(t, tt.count, VerbCoordCount(tt.verb))
		})
	}
}

// ── PaintMode tests ───────────────────────────────────────────────────────────

func TestPaintMode_String(t *testing.T) {
	tests := []struct {
		mode PaintMode
		want string
	}{
		{PaintStroke, "Stroke"},
		{PaintFill, "Fill"},
		{PaintFillStroke, "FillStroke"},
		{PaintMode(99), "PaintMode(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.mode.String())
		})
	}
}

// ── VectorPath tests ──────────────────────────────────────────────────────────

func TestVectorPath_String(t *testing.T) {
	vp := &VectorPath{
		PageNum:    0,
		Verbs:      []PathVerb{VerbMoveTo, VerbLineTo},
		Coords:     []float64{0, 0, 100, 100},
		PaintMode:  PaintStroke,
		LineWidth:  1.5,
		Opacity:    0.8,
		MiterLimit: 10,
	}
	s := vp.String()
	assert.Contains(t, s, "page=0")
	assert.Contains(t, s, "verbs=2")
	assert.Contains(t, s, "Stroke")
	assert.Contains(t, s, "1.50")
	assert.Contains(t, s, "0.80")
}

// ── vectorStateStack tests ────────────────────────────────────────────────────

func TestVectorStateStack_SaveRestore(t *testing.T) {
	var stack vectorStateStack

	gs1 := newVectorGraphicsState()
	gs1.LineWidth = 2.5
	stack.save(gs1)

	gs2 := newVectorGraphicsState()
	gs2.LineWidth = 5.0
	stack.save(gs2)

	restored, ok := stack.restore()
	require.True(t, ok)
	assert.InDelta(t, 5.0, restored.LineWidth, 1e-9)

	restored2, ok := stack.restore()
	require.True(t, ok)
	assert.InDelta(t, 2.5, restored2.LineWidth, 1e-9)

	// Stack empty — should return false.
	_, ok = stack.restore()
	assert.False(t, ok)
}

func TestVectorStateStack_RestoreEmpty(t *testing.T) {
	var stack vectorStateStack
	_, ok := stack.restore()
	assert.False(t, ok)
}

// ── cmykToRGB tests ───────────────────────────────────────────────────────────

func TestCmykToRGB(t *testing.T) {
	tests := []struct {
		name                string
		c, m, y, k          float64
		wantR, wantG, wantB float64
	}{
		{
			name: "pure black (K=1)",
			c:    0, m: 0, y: 0, k: 1,
			wantR: 0, wantG: 0, wantB: 0,
		},
		{
			name: "pure white (all zeros)",
			c:    0, m: 0, y: 0, k: 0,
			wantR: 1, wantG: 1, wantB: 1,
		},
		{
			name: "pure cyan (C=1)",
			c:    1, m: 0, y: 0, k: 0,
			wantR: 0, wantG: 1, wantB: 1,
		},
		{
			name: "pure magenta (M=1)",
			c:    0, m: 1, y: 0, k: 0,
			wantR: 1, wantG: 0, wantB: 1,
		},
		{
			name: "pure yellow (Y=1)",
			c:    0, m: 0, y: 1, k: 0,
			wantR: 1, wantG: 1, wantB: 0,
		},
		{
			// C=0.8, K=0.5 → C+K=1.3 clamps to 1 → R=0
			// M=0,   K=0.5 → M+K=0.5 → G=1-0.5=0.5
			// Y=0,   K=0.5 → Y+K=0.5 → B=1-0.5=0.5
			name: "clamp: C+K > 1",
			c:    0.8, m: 0, y: 0, k: 0.5,
			wantR: 0, wantG: 0.5, wantB: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, g, b := cmykToRGB(tt.c, tt.m, tt.y, tt.k)
			assert.InDelta(t, tt.wantR, r, 1e-9)
			assert.InDelta(t, tt.wantG, g, 1e-9)
			assert.InDelta(t, tt.wantB, b, 1e-9)
		})
	}
}

// ── clamp01 tests ─────────────────────────────────────────────────────────────

func TestClamp01(t *testing.T) {
	assert.InDelta(t, 0.0, clamp01(-1.0), 1e-9)
	assert.InDelta(t, 0.5, clamp01(0.5), 1e-9)
	assert.InDelta(t, 1.0, clamp01(1.5), 1e-9)
	assert.InDelta(t, 1.0, clamp01(1.0), 1e-9)
	assert.InDelta(t, 0.0, clamp01(0.0), 1e-9)
}

// ── VectorParser unit tests (operator processing) ─────────────────────────────

// makeVectorParser creates a VectorParser with a pre-set state for unit testing
// without a real PDF reader.
func makeVectorParser() *VectorParser {
	vp := &VectorParser{
		state: newVectorGraphicsState(),
	}
	return vp
}

func TestVectorParser_GraphicsStateStack(t *testing.T) {
	vp := makeVectorParser()
	vp.state.LineWidth = 1.0

	// q: save state.
	vp.processVectorOperator(makeOp("q"))
	vp.state.LineWidth = 3.0

	// Q: restore state.
	vp.processVectorOperator(makeOp("Q"))
	assert.InDelta(t, 1.0, vp.state.LineWidth, 1e-9, "line width should be restored after Q")
}

func TestVectorParser_Q_EmptyStack(t *testing.T) {
	// Q on empty stack — should not panic.
	vp := makeVectorParser()
	original := vp.state.LineWidth
	vp.processVectorOperator(makeOp("Q"))
	assert.InDelta(t, original, vp.state.LineWidth, 1e-9, "state unchanged when Q on empty stack")
}

func TestVectorParser_LineWidth(t *testing.T) {
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpF("w", 3.5))
	assert.InDelta(t, 3.5, vp.state.LineWidth, 1e-9)
}

func TestVectorParser_LineCap(t *testing.T) {
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpF("J", 1))
	assert.Equal(t, 1, vp.state.LineCap)
}

func TestVectorParser_LineJoin(t *testing.T) {
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpF("j", 2))
	assert.Equal(t, 2, vp.state.LineJoin)
}

func TestVectorParser_MiterLimit(t *testing.T) {
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpF("M", 4.0))
	assert.InDelta(t, 4.0, vp.state.MiterLimit, 1e-9)
}

func TestVectorParser_RGStrokeColor(t *testing.T) {
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpN("RG", 0.2, 0.4, 0.6))
	assert.InDelta(t, 0.2, vp.state.StrokeColor[0], 1e-9)
	assert.InDelta(t, 0.4, vp.state.StrokeColor[1], 1e-9)
	assert.InDelta(t, 0.6, vp.state.StrokeColor[2], 1e-9)
}

func TestVectorParser_rgFillColor(t *testing.T) {
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpN("rg", 0.1, 0.2, 0.3))
	assert.InDelta(t, 0.1, vp.state.FillColor[0], 1e-9)
	assert.InDelta(t, 0.2, vp.state.FillColor[1], 1e-9)
	assert.InDelta(t, 0.3, vp.state.FillColor[2], 1e-9)
}

func TestVectorParser_GGrayscaleStroke(t *testing.T) {
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpF("G", 0.5))
	assert.InDelta(t, 0.5, vp.state.StrokeColor[0], 1e-9)
	assert.InDelta(t, 0.5, vp.state.StrokeColor[1], 1e-9)
	assert.InDelta(t, 0.5, vp.state.StrokeColor[2], 1e-9)
}

func TestVectorParser_gGrayscaleFill(t *testing.T) {
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpF("g", 0.75))
	assert.InDelta(t, 0.75, vp.state.FillColor[0], 1e-9)
	assert.InDelta(t, 0.75, vp.state.FillColor[1], 1e-9)
	assert.InDelta(t, 0.75, vp.state.FillColor[2], 1e-9)
}

func TestVectorParser_KCMYKStroke(t *testing.T) {
	vp := makeVectorParser()
	// CMYK (0,0,0,1) → RGB (0,0,0) black.
	vp.processVectorOperator(makeOpN("K", 0, 0, 0, 1))
	assert.InDelta(t, 0.0, vp.state.StrokeColor[0], 1e-9)
	assert.InDelta(t, 0.0, vp.state.StrokeColor[1], 1e-9)
	assert.InDelta(t, 0.0, vp.state.StrokeColor[2], 1e-9)
}

func TestVectorParser_kCMYKFill(t *testing.T) {
	vp := makeVectorParser()
	// CMYK (0,0,0,0) → RGB (1,1,1) white.
	vp.processVectorOperator(makeOpN("k", 0, 0, 0, 0))
	assert.InDelta(t, 1.0, vp.state.FillColor[0], 1e-9)
	assert.InDelta(t, 1.0, vp.state.FillColor[1], 1e-9)
	assert.InDelta(t, 1.0, vp.state.FillColor[2], 1e-9)
}

func TestVectorParser_MoveTo(t *testing.T) {
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpN("m", 10, 20))
	require.Len(t, vp.curPath, 1)
	assert.Equal(t, VerbMoveTo, vp.curPath[0].verb)
	assert.InDelta(t, 10.0, vp.curPath[0].coords[0], 1e-9)
	assert.InDelta(t, 20.0, vp.curPath[0].coords[1], 1e-9)
	assert.True(t, vp.hasCur)
}

func TestVectorParser_LineTo(t *testing.T) {
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpN("m", 0, 0))
	vp.processVectorOperator(makeOpN("l", 100, 50))
	require.Len(t, vp.curPath, 2)
	assert.Equal(t, VerbLineTo, vp.curPath[1].verb)
	assert.InDelta(t, 100.0, vp.curPath[1].coords[0], 1e-9)
}

func TestVectorParser_CubicBezier_c(t *testing.T) {
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpN("m", 0, 0))
	// c: c1x c1y c2x c2y x y
	vp.processVectorOperator(makeOpN("c", 10, 20, 30, 40, 50, 60))
	require.Len(t, vp.curPath, 2)
	seg := vp.curPath[1]
	assert.Equal(t, VerbCubicTo, seg.verb)
	assert.Len(t, seg.coords, 6)
	assert.InDelta(t, 50.0, seg.coords[4], 1e-9)
	assert.InDelta(t, 60.0, seg.coords[5], 1e-9)
}

func TestVectorParser_CubicBezier_v(t *testing.T) {
	// v: c1 = current point.
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpN("m", 10, 20)) // current = (10,20)
	vp.processVectorOperator(makeOpN("v", 30, 40, 50, 60))
	require.Len(t, vp.curPath, 2)
	seg := vp.curPath[1]
	assert.Equal(t, VerbCubicTo, seg.verb)
	// c1 should be (10,20) — the current point.
	assert.InDelta(t, 10.0, seg.coords[0], 1e-9)
	assert.InDelta(t, 20.0, seg.coords[1], 1e-9)
}

func TestVectorParser_CubicBezier_y(t *testing.T) {
	// y: c2 = endpoint.
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpN("m", 0, 0))
	vp.processVectorOperator(makeOpN("y", 10, 20, 50, 60))
	require.Len(t, vp.curPath, 2)
	seg := vp.curPath[1]
	assert.Equal(t, VerbCubicTo, seg.verb)
	// c2 should equal endpoint (50,60).
	assert.InDelta(t, 50.0, seg.coords[2], 1e-9)
	assert.InDelta(t, 60.0, seg.coords[3], 1e-9)
	assert.InDelta(t, 50.0, seg.coords[4], 1e-9)
	assert.InDelta(t, 60.0, seg.coords[5], 1e-9)
}

func TestVectorParser_CloseSubpath(t *testing.T) {
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpN("m", 0, 0))
	vp.processVectorOperator(makeOpN("l", 100, 0))
	vp.processVectorOperator(makeOp("h"))
	require.Len(t, vp.curPath, 3)
	assert.Equal(t, VerbClose, vp.curPath[2].verb)
}

func TestVectorParser_Rectangle(t *testing.T) {
	vp := makeVectorParser()
	// re: x y w h
	vp.processVectorOperator(makeOpN("re", 10, 20, 100, 50))
	// re expands to: m + 3×l + h = 5 segments.
	require.Len(t, vp.curPath, 5)
	assert.Equal(t, VerbMoveTo, vp.curPath[0].verb)
	assert.Equal(t, VerbLineTo, vp.curPath[1].verb)
	assert.Equal(t, VerbLineTo, vp.curPath[2].verb)
	assert.Equal(t, VerbLineTo, vp.curPath[3].verb)
	assert.Equal(t, VerbClose, vp.curPath[4].verb)
}

func TestVectorParser_StrokePath(t *testing.T) {
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpN("m", 0, 0))
	vp.processVectorOperator(makeOpN("l", 100, 0))
	vp.processVectorOperator(makeOp("S"))

	require.Len(t, vp.paths, 1)
	assert.Equal(t, PaintStroke, vp.paths[0].PaintMode)
	assert.Empty(t, vp.curPath, "curPath should be cleared after S")
}

func TestVectorParser_FillPath_f(t *testing.T) {
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpN("re", 10, 10, 50, 50))
	vp.processVectorOperator(makeOp("f"))

	require.Len(t, vp.paths, 1)
	assert.Equal(t, PaintFill, vp.paths[0].PaintMode)
}

func TestVectorParser_FillPath_F(t *testing.T) {
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpN("re", 0, 0, 100, 100))
	vp.processVectorOperator(makeOp("F"))

	require.Len(t, vp.paths, 1)
	assert.Equal(t, PaintFill, vp.paths[0].PaintMode)
}

func TestVectorParser_FillStrokePath_B(t *testing.T) {
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpN("re", 0, 0, 100, 100))
	vp.processVectorOperator(makeOp("B"))

	require.Len(t, vp.paths, 1)
	assert.Equal(t, PaintFillStroke, vp.paths[0].PaintMode)
}

func TestVectorParser_FillStrokePath_BStar(t *testing.T) {
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpN("re", 0, 0, 100, 100))
	vp.processVectorOperator(makeOp("B*"))
	require.Len(t, vp.paths, 1)
	assert.Equal(t, PaintFillStroke, vp.paths[0].PaintMode)
}

func TestVectorParser_CloseAndStroke_s(t *testing.T) {
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpN("m", 0, 0))
	vp.processVectorOperator(makeOpN("l", 50, 50))
	vp.processVectorOperator(makeOp("s"))

	require.Len(t, vp.paths, 1)
	assert.Equal(t, PaintStroke, vp.paths[0].PaintMode)
	// The path should include the implicit close verb.
	assert.Contains(t, vp.paths[0].Verbs, VerbClose)
}

func TestVectorParser_CloseAndFillStroke_b(t *testing.T) {
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpN("m", 0, 0))
	vp.processVectorOperator(makeOpN("l", 50, 0))
	vp.processVectorOperator(makeOp("b"))

	require.Len(t, vp.paths, 1)
	assert.Equal(t, PaintFillStroke, vp.paths[0].PaintMode)
	assert.Contains(t, vp.paths[0].Verbs, VerbClose)
}

func TestVectorParser_FillStarAndBStar(t *testing.T) {
	// f* and b* are even-odd variants — same PaintMode output.
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpN("re", 0, 0, 10, 10))
	vp.processVectorOperator(makeOp("f*"))
	require.Len(t, vp.paths, 1)
	assert.Equal(t, PaintFill, vp.paths[0].PaintMode)

	vp2 := makeVectorParser()
	vp2.processVectorOperator(makeOpN("m", 0, 0))
	vp2.processVectorOperator(makeOpN("l", 10, 0))
	vp2.processVectorOperator(makeOp("b*"))
	require.Len(t, vp2.paths, 1)
	assert.Equal(t, PaintFillStroke, vp2.paths[0].PaintMode)
}

func TestVectorParser_NPath_Discards(t *testing.T) {
	// n: end path without painting — must not produce a VectorPath.
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpN("m", 0, 0))
	vp.processVectorOperator(makeOpN("l", 100, 0))
	vp.processVectorOperator(makeOp("n"))

	assert.Empty(t, vp.paths)
	assert.Empty(t, vp.curPath)
}

func TestVectorParser_ClipPaths_Discarded(t *testing.T) {
	// W and W* must not produce VectorPaths.
	for _, clipOp := range []string{"W", "W*"} {
		t.Run(clipOp, func(t *testing.T) {
			vp := makeVectorParser()
			vp.processVectorOperator(makeOpN("re", 0, 0, 100, 100))
			vp.processVectorOperator(makeOp(clipOp))
			assert.Empty(t, vp.paths, "clip path operator %s should not produce VectorPath", clipOp)
		})
	}
}

func TestVectorParser_EmptyPath_NotEmitted(t *testing.T) {
	// Calling S with no path should not produce output.
	vp := makeVectorParser()
	vp.processVectorOperator(makeOp("S"))
	assert.Empty(t, vp.paths)
}

func TestVectorParser_CTM_cm(t *testing.T) {
	vp := makeVectorParser()
	// Apply scaling matrix: scale x2.
	// cm: a b c d e f (identity scaled by 2 = [2 0 0 2 0 0]).
	vp.processVectorOperator(makeOpN("cm", 2, 0, 0, 2, 0, 0))

	// Now m at (10, 20) should land at (20, 40) in page space.
	vp.processVectorOperator(makeOpN("m", 10, 20))

	require.Len(t, vp.curPath, 1)
	assert.InDelta(t, 20.0, vp.curPath[0].coords[0], 1e-9)
	assert.InDelta(t, 40.0, vp.curPath[0].coords[1], 1e-9)
}

func TestVectorParser_CTM_SaveRestore(t *testing.T) {
	// Ensure cm is saved and restored by q/Q.
	vp := makeVectorParser()
	vp.processVectorOperator(makeOp("q"))
	vp.processVectorOperator(makeOpN("cm", 3, 0, 0, 3, 0, 0))

	// Point (1,1) with scale=3 should be (3,3).
	vp.processVectorOperator(makeOpN("m", 1, 1))
	assert.InDelta(t, 3.0, vp.curPath[0].coords[0], 1e-9)

	// Restore.
	vp.processVectorOperator(makeOp("Q"))
	vp.curPath = nil
	vp.hasCur = false

	// Point (1,1) with identity CTM should still be (1,1).
	vp.processVectorOperator(makeOpN("m", 1, 1))
	assert.InDelta(t, 1.0, vp.curPath[0].coords[0], 1e-9)
}

func TestVectorParser_Opacity_Default(t *testing.T) {
	// Without any ExtGState, opacity should be 1.0.
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpN("re", 0, 0, 100, 100))
	vp.processVectorOperator(makeOp("f"))

	require.Len(t, vp.paths, 1)
	assert.InDelta(t, 1.0, vp.paths[0].Opacity, 1e-9)
}

func TestVectorParser_Colors_Captured(t *testing.T) {
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpN("RG", 0.1, 0.2, 0.3)) // stroke
	vp.processVectorOperator(makeOpN("rg", 0.4, 0.5, 0.6)) // fill
	vp.processVectorOperator(makeOpN("re", 0, 0, 50, 50))
	vp.processVectorOperator(makeOp("B"))

	require.Len(t, vp.paths, 1)
	p := vp.paths[0]
	assert.InDelta(t, 0.1, p.StrokeColor[0], 1e-9)
	assert.InDelta(t, 0.2, p.StrokeColor[1], 1e-9)
	assert.InDelta(t, 0.3, p.StrokeColor[2], 1e-9)
	assert.InDelta(t, 0.4, p.FillColor[0], 1e-9)
	assert.InDelta(t, 0.5, p.FillColor[1], 1e-9)
	assert.InDelta(t, 0.6, p.FillColor[2], 1e-9)
}

func TestVectorParser_VerbCoordsConsistency(t *testing.T) {
	// All coord counts must match verb definitions.
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpN("m", 0, 0))
	vp.processVectorOperator(makeOpN("l", 50, 0))
	vp.processVectorOperator(makeOpN("c", 60, 10, 70, 10, 80, 0))
	vp.processVectorOperator(makeOp("h"))
	vp.processVectorOperator(makeOp("S"))

	require.Len(t, vp.paths, 1)
	path := vp.paths[0]

	coordIdx := 0
	for _, verb := range path.Verbs {
		n := VerbCoordCount(verb)
		assert.GreaterOrEqual(t, len(path.Coords)-coordIdx, n,
			"not enough coords for verb %s at index %d", verb, coordIdx)
		coordIdx += n
	}
	assert.Equal(t, len(path.Coords), coordIdx, "all coords consumed")
}

func TestVectorParser_LineWidthInPath(t *testing.T) {
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpF("w", 3.0))
	vp.processVectorOperator(makeOpN("m", 0, 0))
	vp.processVectorOperator(makeOpN("l", 100, 0))
	vp.processVectorOperator(makeOp("S"))

	require.Len(t, vp.paths, 1)
	assert.InDelta(t, 3.0, vp.paths[0].LineWidth, 1e-9)
}

func TestVectorParser_LineCapJoinMiterInPath(t *testing.T) {
	vp := makeVectorParser()
	vp.processVectorOperator(makeOpF("J", 1))   // round cap
	vp.processVectorOperator(makeOpF("j", 2))   // bevel join
	vp.processVectorOperator(makeOpF("M", 8.0)) // miter limit
	vp.processVectorOperator(makeOpN("m", 0, 0))
	vp.processVectorOperator(makeOpN("l", 100, 0))
	vp.processVectorOperator(makeOp("S"))

	require.Len(t, vp.paths, 1)
	assert.Equal(t, 1, vp.paths[0].LineCap)
	assert.Equal(t, 2, vp.paths[0].LineJoin)
	assert.InDelta(t, 8.0, vp.paths[0].MiterLimit, 1e-9)
}

func TestVectorParser_CMYK_Stroke(t *testing.T) {
	vp := makeVectorParser()
	// K 0 0 0 1 → black.
	vp.processVectorOperator(makeOpN("K", 0, 0, 0, 1))
	vp.processVectorOperator(makeOpN("m", 0, 0))
	vp.processVectorOperator(makeOpN("l", 100, 0))
	vp.processVectorOperator(makeOp("S"))

	require.Len(t, vp.paths, 1)
	assert.InDelta(t, 0.0, vp.paths[0].StrokeColor[0], 1e-9)
	assert.InDelta(t, 0.0, vp.paths[0].StrokeColor[1], 1e-9)
	assert.InDelta(t, 0.0, vp.paths[0].StrokeColor[2], 1e-9)
}

func TestVectorParser_CMYK_Fill(t *testing.T) {
	vp := makeVectorParser()
	// k 1 0 0 0 → cyan.
	vp.processVectorOperator(makeOpN("k", 1, 0, 0, 0))
	vp.processVectorOperator(makeOpN("re", 0, 0, 50, 50))
	vp.processVectorOperator(makeOp("f"))

	require.Len(t, vp.paths, 1)
	assert.InDelta(t, 0.0, vp.paths[0].FillColor[0], 1e-9)
	assert.InDelta(t, 1.0, vp.paths[0].FillColor[1], 1e-9)
	assert.InDelta(t, 1.0, vp.paths[0].FillColor[2], 1e-9)
}

func TestVectorParser_d_Noop(t *testing.T) {
	// d (dash pattern) should not crash.
	vp := makeVectorParser()
	// The d operator takes an array and a phase, but our content parser will
	// have already parsed them — we just pass two operands.
	op := &Operator{Name: "d", Operands: nil}
	assert.NotPanics(t, func() { vp.processVectorOperator(op) })
}

// ── Test helpers ──────────────────────────────────────────────────────────────

// makeOp creates an operator with no operands.
func makeOp(name string) *Operator {
	return &Operator{Name: name}
}

// makeOpF creates an operator with a single float64 operand.
func makeOpF(name string, v float64) *Operator {
	return &Operator{
		Name:     name,
		Operands: floatOperands(v),
	}
}

// makeOpN creates an operator with multiple float64 operands.
func makeOpN(name string, vals ...float64) *Operator {
	return &Operator{
		Name:     name,
		Operands: floatOperands(vals...),
	}
}

// floatOperands converts a variadic list of float64 values to PdfObject operands.
func floatOperands(vals ...float64) []parser.PdfObject {
	objs := make([]parser.PdfObject, len(vals))
	for i, v := range vals {
		objs[i] = parser.NewReal(v)
	}
	return objs
}
