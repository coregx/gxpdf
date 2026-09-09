package gxpdf

import (
	"fmt"

	"github.com/coregx/gxpdf/internal/extractor"
	"github.com/coregx/gxpdf/logging"
)

// PathVerb identifies the kind of step in a vector path.
//
// The verb+coords model is compatible with the gogpu/gg PathVerb pattern.
// Each verb describes one path operation and the number of coordinate values
// that follow it in the VectorPath.Coords slice.
//
//   - PathVerbMoveTo  — 2 coords: x, y
//   - PathVerbLineTo  — 2 coords: x, y
//   - PathVerbCubicTo — 6 coords: c1x, c1y, c2x, c2y, x, y
//   - PathVerbQuadTo  — 4 coords: cx, cy, x, y
//   - PathVerbClose   — 0 coords
//
// Reference: PDF 1.7 specification, Section 8.5.2 (Path Construction Operators).
type PathVerb int

const (
	// PathVerbMoveTo starts a new subpath at (x, y). Consumes 2 coords.
	PathVerbMoveTo PathVerb = PathVerb(extractor.VerbMoveTo)

	// PathVerbLineTo adds a straight-line segment to (x, y). Consumes 2 coords.
	PathVerbLineTo PathVerb = PathVerb(extractor.VerbLineTo)

	// PathVerbCubicTo adds a cubic Bézier segment. Consumes 6 coords:
	// c1x, c1y, c2x, c2y, x, y.
	PathVerbCubicTo PathVerb = PathVerb(extractor.VerbCubicTo)

	// PathVerbQuadTo adds a quadratic Bézier segment. Consumes 4 coords:
	// cx, cy, x, y.
	PathVerbQuadTo PathVerb = PathVerb(extractor.VerbQuadTo)

	// PathVerbClose closes the current subpath. Consumes 0 coords.
	PathVerbClose PathVerb = PathVerb(extractor.VerbClose)
)

// String returns a human-readable label for the verb.
func (v PathVerb) String() string {
	return extractor.PathVerb(v).String()
}

// VerbCoordCount returns the number of coordinate values consumed by this verb.
func VerbCoordCount(v PathVerb) int {
	return extractor.VerbCoordCount(extractor.PathVerb(v))
}

// PaintMode describes how a vector path is painted.
//
// Reference: PDF 1.7 specification, Section 8.5.3 (Path Painting Operators).
type PaintMode int

const (
	// PaintModeStroke strokes the path outline.
	PaintModeStroke PaintMode = PaintMode(extractor.PaintStroke)

	// PaintModeFill fills the path interior.
	PaintModeFill PaintMode = PaintMode(extractor.PaintFill)

	// PaintModeFillStroke fills and strokes the path.
	PaintModeFillStroke PaintMode = PaintMode(extractor.PaintFillStroke)
)

// String returns a human-readable label for the paint mode.
func (pm PaintMode) String() string {
	return extractor.PaintMode(pm).String()
}

// VectorPath represents a fully resolved vector graphics path extracted from a
// PDF page, with all coordinates transformed into page user space.
//
// Coordinate system: points (1/72 inch), origin at bottom-left of page.
// All coordinates are post-CTM (Current Transformation Matrix) — they are
// already in absolute page space and require no further transformation.
//
// Iterating verbs and coords:
//
//	idx := 0
//	for _, verb := range path.Verbs {
//	    n := gxpdf.VerbCoordCount(verb)
//	    coords := path.Coords[idx : idx+n]
//	    idx += n
//	    switch verb {
//	    case gxpdf.PathVerbMoveTo:
//	        // coords[0]=x, coords[1]=y
//	    case gxpdf.PathVerbLineTo:
//	        // coords[0]=x, coords[1]=y
//	    case gxpdf.PathVerbCubicTo:
//	        // coords[0..1]=c1, coords[2..3]=c2, coords[4..5]=endpoint
//	    case gxpdf.PathVerbClose:
//	        // no coords
//	    }
//	}
type VectorPath struct {
	// PageNum is the 0-based page index this path was extracted from.
	PageNum int

	// Verbs is the sequence of path construction operations.
	// Use VerbCoordCount(verb) to determine how many Coords each verb consumes.
	Verbs []PathVerb

	// Coords stores the flattened coordinates for all verbs in page space.
	Coords []float64

	// StrokeColor is the stroke color in RGBA (components 0–1).
	// The alpha channel reflects the /CA ExtGState opacity (1.0 if absent).
	StrokeColor [4]float64

	// FillColor is the fill color in RGBA (components 0–1).
	// The alpha channel reflects the /ca ExtGState opacity (1.0 if absent).
	FillColor [4]float64

	// LineWidth is the stroke width in points.
	LineWidth float64

	// LineCap is the PDF line cap style: 0=butt, 1=round, 2=projecting square.
	LineCap int

	// LineJoin is the PDF line join style: 0=miter, 1=round, 2=bevel.
	LineJoin int

	// MiterLimit is the miter join limit (PDF default: 10.0).
	MiterLimit float64

	// Opacity is the effective opacity (0=transparent, 1=opaque).
	// Derived from ExtGState parameters; defaults to 1.0.
	Opacity float64

	// PaintMode indicates how the path is rendered.
	PaintMode PaintMode
}

// String returns a compact human-readable representation of the vector path.
func (vp *VectorPath) String() string {
	return fmt.Sprintf(
		"VectorPath{page=%d verbs=%d paint=%s lw=%.2f opacity=%.2f}",
		vp.PageNum,
		len(vp.Verbs),
		vp.PaintMode.String(),
		vp.LineWidth,
		vp.Opacity,
	)
}

// GetVectorGraphics extracts all vector graphics paths from all pages.
//
// This is the simplest way to extract vector graphics. It processes all pages
// and returns all paths found across the entire document.
//
// Errors during extraction are logged via slog.
// For error handling, use GetVectorGraphicsWithError.
//
// Example:
//
//	doc, err := gxpdf.Open("diagram.pdf")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer doc.Close()
//
//	paths := doc.GetVectorGraphics()
//	for _, p := range paths {
//	    fmt.Printf("Page %d: %s\n", p.PageNum, p.PaintMode)
//	}
func (d *Document) GetVectorGraphics() []*VectorPath {
	paths, err := d.GetVectorGraphicsWithError()
	if err != nil {
		logging.Logger().Error("failed to extract vector graphics",
			"path", d.path,
			"error", err)
	}
	return paths
}

// GetVectorGraphicsWithError extracts all vector graphics paths from all pages,
// returning any errors encountered.
//
// Use this when you need structured error handling.
func (d *Document) GetVectorGraphicsWithError() ([]*VectorPath, error) {
	count := d.PageCount()
	var all []*VectorPath
	for i := 0; i < count; i++ {
		pagePaths, err := d.GetVectorGraphicsForPage(i)
		if err != nil {
			return nil, fmt.Errorf("gxpdf: failed to extract vector graphics from page %d: %w", i, err)
		}
		all = append(all, pagePaths...)
	}
	return all, nil
}

// GetVectorGraphicsForPage extracts vector graphics paths from a single page.
//
// pageNum is 0-based (first page = 0).
//
// Example:
//
//	paths, err := doc.GetVectorGraphicsForPage(0)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Page 0 has %d vector paths\n", len(paths))
func (d *Document) GetVectorGraphicsForPage(pageNum int) ([]*VectorPath, error) {
	if pageNum < 0 || pageNum >= d.PageCount() {
		return nil, fmt.Errorf("gxpdf: page %d out of range (0-%d)", pageNum, d.PageCount()-1)
	}

	vparser := extractor.NewVectorParserWithContext(d.reader, d.ctx)
	internalPaths, err := vparser.ParseFromPage(pageNum)
	if err != nil {
		return nil, fmt.Errorf("gxpdf: failed to extract vector graphics from page %d: %w", pageNum, err)
	}

	// Convert internal paths to public API paths.
	result := make([]*VectorPath, len(internalPaths))
	for i, ip := range internalPaths {
		result[i] = internalToPublicVectorPath(ip)
	}
	return result, nil
}

// internalToPublicVectorPath converts an internal VectorPath to the public type.
func internalToPublicVectorPath(ip *extractor.VectorPath) *VectorPath {
	verbs := make([]PathVerb, len(ip.Verbs))
	for i, v := range ip.Verbs {
		verbs[i] = PathVerb(v)
	}

	// Copy coords slice to avoid shared backing array.
	coords := make([]float64, len(ip.Coords))
	copy(coords, ip.Coords)

	return &VectorPath{
		PageNum:     ip.PageNum,
		Verbs:       verbs,
		Coords:      coords,
		StrokeColor: ip.StrokeColor,
		FillColor:   ip.FillColor,
		LineWidth:   ip.LineWidth,
		LineCap:     ip.LineCap,
		LineJoin:    ip.LineJoin,
		MiterLimit:  ip.MiterLimit,
		Opacity:     ip.Opacity,
		PaintMode:   PaintMode(ip.PaintMode),
	}
}
