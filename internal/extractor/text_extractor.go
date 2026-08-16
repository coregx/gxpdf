package extractor

import (
	"compress/zlib"
	"fmt"
	"io"
	"log/slog"
	"math"
	"strings"

	"github.com/coregx/gxpdf/internal/parser"
	"github.com/coregx/gxpdf/logging"
)

// filterFlateDecode is the PDF filter name for zlib/deflate compression.
const filterFlateDecode = "FlateDecode"

// glyphMergeGapFactor is the maximum gap between two adjacent TextElements,
// expressed as a multiple of the font size, that is still treated as
// intra-word spacing rather than a word boundary.
//
// wkhtmltopdf and similar generators emit one Tj per glyph with explicit Td
// kerning moves. The per-glyph advance distances are typically in the range
// 0.3–0.9 × fontSize. We use 1.5 × fontSize as the merge threshold so that
// normal kerning and slight CID tracking are absorbed, while genuine inter-word
// gaps (which are usually ≥ 2 × fontSize for body text) remain as separate
// elements.
const glyphMergeGapFactor = 1.5

// wordSpaceGapFactor is the minimum gap between two adjacent TextElements,
// expressed as a multiple of the font size, above which a space character is
// inserted when assembling the final text string from a slice of TextElements.
//
// This constant is exported for use by the assembleText helper and is kept in
// sync with the merge threshold: a gap that was too large to merge is treated
// as a word boundary and receives a space.
const wordSpaceGapFactor = 1.0

// preciseGlyphGapFactor is used when the PDF provides authoritative font
// widths. Exact widths let us preserve real word gaps without the generous
// tolerance required by the legacy estimated-width path.
const preciseGlyphGapFactor = 0.2

// positionedGlyphFontSizeLimit identifies the Form-XObject pattern emitted by
// Quartz and similar generators: every glyph uses a unit-sized font and a cm
// matrix supplies the actual size and position. Direct-page/larger text keeps
// the established raw-coordinate behavior until gxpdf's graphics and lattice
// coordinate paths can migrate together without changing existing tables.
const positionedGlyphFontSizeLimit = 2.0

// maxXObjectDepth limits Form XObject recursion to prevent infinite loops.
//
// The PDF specification does not forbid cyclic XObject references, so we
// cap the recursion depth. In practice, shipping-label PDFs produced by
// TCPDF use at most 2 levels of nesting.
const maxXObjectDepth = 8

// TextExtractor extracts text with positional information from PDF pages.
//
// The extractor processes PDF content streams and interprets text operators
// to extract text along with its X,Y coordinates. This is critical for
// table extraction, as we need to know where each piece of text is located.
//
// Text Extraction Process:
//  1. Get page's content stream(s)
//  2. Decode stream (handle FlateDecode, etc.)
//  3. Parse content operators
//  4. Track text state (font, position, matrix)
//  5. Extract text with coordinates when text showing operators are encountered
//  6. Decode glyph bytes to Unicode using font CMap/encoding
//  7. Recurse into Form XObjects (Do operator) for nested content
//
// Reference: PDF 1.7 specification, Sections 9.4 (Text Objects), 8.8.1 (Form XObjects).
type TextExtractor struct {
	reader        *parser.Reader
	textState     *TextState
	elements      []*TextElement
	fontDecoders  map[string]*FontDecoder // fontName -> FontDecoder
	fontMetrics   map[string]*fontMetrics // fontName -> glyph widths
	pageResources *parser.Dictionary      // Current page resources
	ctm           Matrix                  // Current transformation matrix

	graphicsStateStack []textExtractorGraphicsState

	// resourceStack holds the saved resource context across Form XObject calls.
	// Each Do operator pushes the current resources; on return they are restored.
	resourceStack []*parser.Dictionary

	// xobjectDepth is the current Form XObject recursion depth.
	// It is incremented on every Do call and decremented on return.
	xobjectDepth int
}

type textExtractorGraphicsState struct {
	ctm Matrix
}

// NewTextExtractor creates a new TextExtractor for the given PDF reader.
func NewTextExtractor(reader *parser.Reader) *TextExtractor {
	return &TextExtractor{
		reader:       reader,
		textState:    NewTextState(),
		elements:     []*TextElement{},
		fontDecoders: make(map[string]*FontDecoder),
		fontMetrics:  make(map[string]*fontMetrics),
		ctm:          Identity(),
	}
}

// ExtractFromPage extracts all text elements from the specified page.
//
// Page numbers are 0-based (first page is 0).
//
// Returns a slice of TextElements with position information, or error if extraction fails.
func (te *TextExtractor) ExtractFromPage(pageNum int) ([]*TextElement, error) {
	// Reset state
	te.elements = []*TextElement{}
	te.textState = NewTextState()
	te.fontDecoders = make(map[string]*FontDecoder)
	te.fontMetrics = make(map[string]*fontMetrics)
	te.ctm = Identity()
	te.graphicsStateStack = nil

	// Get page
	page, err := te.reader.GetPage(pageNum)
	if err != nil {
		return nil, fmt.Errorf("failed to get page %d: %w", pageNum, err)
	}

	// Store page resources for font loading
	te.pageResources = te.getPageResources(page)

	// Get content stream(s)
	contentData, err := te.getPageContent(page)
	if err != nil {
		return nil, fmt.Errorf("failed to get page content: %w", err)
	}

	// If no content, return empty list
	if len(contentData) == 0 {
		return []*TextElement{}, nil
	}

	// Parse content stream operators
	contentParser := NewContentParser(contentData)
	operators, err := contentParser.ParseOperators()
	if err != nil {
		return nil, fmt.Errorf("failed to parse content stream: %w", err)
	}

	// Process operators to extract text
	for _, op := range operators {
		te.processOperator(op)
	}

	// Merge per-glyph TextElements that belong to the same word.
	//
	// Generators such as wkhtmltopdf emit one Tj operator per glyph with
	// explicit Td kerning moves between them. This produces one TextElement
	// per character, which causes callers that concatenate elements with
	// separating spaces to produce "D O M I C I L I O" instead of
	// "DOMICILIO". mergeAdjacentElements collapses runs of same-line,
	// positionally-adjacent elements into single word-level TextElements.
	te.elements = mergeAdjacentElements(te.elements)

	return te.elements, nil
}

// mergeAdjacentElements collapses per-glyph TextElements into word-level runs.
//
// PDF generators such as wkhtmltopdf (used by Andreani shipping labels) emit
// each glyph as a separate Tj operator preceded by a Td kern move:
//
//	325.9 -63 Td <0001> Tj
//	7.17  0   Td <0002> Tj   ← next glyph, ~7 pts to the right
//	7.58  0   Td <0003> Tj
//
// This creates one TextElement per character. When callers concatenate
// elements with a separating space they produce "D O M I C I L I O"
// instead of "DOMICILIO".
//
// The function groups consecutive elements that:
//   - Share the same font and approximate Y coordinate (same text line), and
//   - Have a gap between the right edge of the previous element and the left
//     edge of the current one that is at most glyphMergeGapFactor × fontSize.
//
// Merged elements receive the concatenated text, the X/Y of the first element,
// and a combined width. Elements separated by a larger gap are kept distinct
// (word boundary) so that word-spacing logic in callers remains correct.
//
// This operation is idempotent: running it on already-merged output is a no-op.
func mergeAdjacentElements(elements []*TextElement) []*TextElement {
	if len(elements) == 0 {
		return elements
	}

	merged := make([]*TextElement, 0, len(elements))
	current := elements[0]

	for _, next := range elements[1:] {
		// Determine whether 'next' should be merged into 'current'.
		if canMerge(current, next) {
			// Extend current element: append text and widen bounding box.
			current = &TextElement{
				Text:         current.Text + next.Text,
				X:            current.X,
				Y:            current.Y,
				Width:        (next.X + next.Width) - current.X,
				Height:       current.Height,
				FontName:     current.FontName,
				FontSize:     current.FontSize,
				preciseWidth: current.preciseWidth && next.preciseWidth,
			}
		} else {
			// Word boundary — commit the current run and start a new one.
			merged = append(merged, current)
			current = next
		}
	}

	// Commit the last run.
	merged = append(merged, current)
	return merged
}

// canMerge reports whether two TextElements should be merged into one run.
//
// Two elements are merge-candidates when:
//  1. They share the same font name.
//  2. Their Y coordinates are within half a font-height of each other
//     (same text line, tolerating minor baseline shifts from Ts).
//  3. The gap from the right edge of 'a' to the left edge of 'b' is no wider
//     than glyphMergeGapFactor × fontSize (intra-word kerning, not a word gap).
//  4. 'b' starts to the right of 'a' (horizontal flow only; vertical moves
//     like T* are excluded by the Y-coordinate check).
func canMerge(a, b *TextElement) bool {
	// Same font is required — different fonts signal a formatting change.
	if a.FontName != b.FontName {
		return false
	}

	// Same approximate font size (allow 0.5 pt tolerance for rounding).
	const fontSizeTol = 0.5
	if a.FontSize-b.FontSize > fontSizeTol || b.FontSize-a.FontSize > fontSizeTol {
		return false
	}

	// Same line: Y coordinates must be within half the font height.
	lineToleranceY := a.Height * 0.5
	if lineToleranceY < 1.0 {
		lineToleranceY = 1.0
	}
	dy := a.Y - b.Y
	if dy < 0 {
		dy = -dy
	}
	if dy > lineToleranceY {
		return false
	}

	// 'b' must start to the right of 'a' (left-to-right flow).
	if b.X < a.X {
		return false
	}

	// Gap between right edge of 'a' and left edge of 'b'.
	gap := b.X - (a.X + a.Width)

	// Negative gap means the elements overlap — that is normal for CID fonts
	// whose width estimate is a rough heuristic. Treat overlap as merge-eligible.
	if gap <= 0 {
		return true
	}

	// Positive gap: merge only if the gap is within the intra-word threshold.
	thresholdFactor := glyphMergeGapFactor
	if a.preciseWidth && b.preciseWidth {
		thresholdFactor = preciseGlyphGapFactor
	}
	threshold := a.FontSize * thresholdFactor
	return gap <= threshold
}

// AssembleText converts a slice of TextElements into a readable string.
//
// Word boundaries are detected spatially: when the gap between the right
// edge of one element and the left edge of the next exceeds
// wordSpaceGapFactor × fontSize, a space is inserted. Elements on different
// lines (Y change > half font-height) receive a newline.
//
// This function is used by the public API (document.go / page.go) and is
// exported so that table-extraction consumers can apply the same logic.
func AssembleText(elements []*TextElement) string {
	if len(elements) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.Grow(len(elements) * 4) // rough pre-allocation

	sb.WriteString(elements[0].Text)

	for i := 1; i < len(elements); i++ {
		prev := elements[i-1]
		curr := elements[i]

		// Detect line break (different Y).
		lineToleranceY := prev.Height * 0.5
		if lineToleranceY < 1.0 {
			lineToleranceY = 1.0
		}
		dy := prev.Y - curr.Y
		if dy < 0 {
			dy = -dy
		}
		if dy > lineToleranceY {
			sb.WriteByte('\n')
			sb.WriteString(curr.Text)
			continue
		}

		// Same line: decide whether a space is needed.
		gap := curr.X - (prev.X + prev.Width)
		gapFactor := wordSpaceGapFactor
		if prev.preciseWidth && curr.preciseWidth {
			gapFactor = preciseGlyphGapFactor
		}
		if gap > prev.FontSize*gapFactor {
			sb.WriteByte(' ')
		}
		sb.WriteString(curr.Text)
	}

	return sb.String()
}

// getPageContent retrieves and decodes the content stream(s) for a page.
//
// A page can have a single content stream or an array of content streams.
// We concatenate all streams and return the decoded content.
//
//nolint:cyclop // PDF page content handling requires checking multiple cases
func (te *TextExtractor) getPageContent(page *parser.Dictionary) ([]byte, error) {
	contentsObj := page.Get("Contents")
	if contentsObj == nil {
		// No content stream - empty page
		return []byte{}, nil
	}

	// Resolve if it's an indirect reference
	if ref, ok := contentsObj.(*parser.IndirectReference); ok {
		resolved, err := te.reader.GetObject(ref.Number)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve contents reference: %w", err)
		}
		contentsObj = resolved
	}

	var allContent []byte

	// Check if it's a single stream or an array of streams
	switch obj := contentsObj.(type) {
	case *parser.Stream:
		// Single stream
		content, err := te.decodeStream(obj)
		if err != nil {
			return nil, fmt.Errorf("failed to decode content stream: %w", err)
		}
		allContent = content

	case *parser.Array:
		// Array of streams - concatenate them
		for i := 0; i < obj.Len(); i++ {
			streamRef := obj.Get(i)
			if streamRef == nil {
				continue
			}

			// Resolve indirect reference
			if ref, ok := streamRef.(*parser.IndirectReference); ok {
				resolved, err := te.reader.GetObject(ref.Number)
				if err != nil {
					continue
				}
				streamRef = resolved
			}

			// Decode stream
			if stream, ok := streamRef.(*parser.Stream); ok {
				content, err := te.decodeStream(stream)
				if err != nil {
					continue
				}
				allContent = append(allContent, content...)
				// Add space between streams for safety
				allContent = append(allContent, ' ')
			}
		}

	default:
		return nil, fmt.Errorf("unexpected Contents type: %T", obj)
	}

	return allContent, nil
}

// decodeStream decodes a PDF stream based on its filters.
//
// For Phase 2.5, we implement FlateDecode (most common).
// Other filters can be added in future phases.
func (te *TextExtractor) decodeStream(stream *parser.Stream) ([]byte, error) {
	// Get filter
	filterObj := stream.Dictionary().Get("Filter")
	if filterObj == nil {
		// No filter - return raw content
		return stream.Content(), nil
	}

	// Get filter name
	var filterName string
	if name, ok := filterObj.(*parser.Name); ok {
		filterName = name.Value()
	} else if arr, ok := filterObj.(*parser.Array); ok {
		// Array of filters - for now, just handle first one
		if arr.Len() > 0 {
			if name, ok := arr.Get(0).(*parser.Name); ok {
				filterName = name.Value()
			}
		}
	}

	// Apply filter
	switch filterName {
	case filterFlateDecode:
		return te.decodeFlateDecode(stream.Content())

	case "":
		// No filter
		return stream.Content(), nil

	default:
		// Unsupported filter - return raw content and hope for the best
		// In production, we should log this
		return stream.Content(), nil
	}
}

// decodeFlateDecode decodes FlateDecode (zlib) compressed data.
//
// FlateDecode is the most common compression filter in PDFs.
//
// Reference: PDF 1.7 specification, Section 7.4.4 (LZW and Flate Filters).
func (te *TextExtractor) decodeFlateDecode(data []byte) ([]byte, error) {
	// Create a bytes buffer wrapper
	buf := &bytesReaderCloser{data: data, pos: 0}

	// Create zlib reader with actual data
	reader, err := zlib.NewReader(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize zlib reader: %w", err)
	}
	defer func() {
		_ = reader.Close() // Close reader, ignore error
	}()

	// Read all decoded data
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode FlateDecode: %w", err)
	}

	return decoded, nil
}

// bytesReaderCloser wraps a byte slice to implement io.ReadCloser.
type bytesReaderCloser struct {
	data []byte
	pos  int
}

func (b *bytesReaderCloser) Read(p []byte) (int, error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}

func (b *bytesReaderCloser) Close() error {
	return nil
}

// processOperator processes a single content stream operator.
//
// This is the heart of text extraction - it interprets text operators
// and updates text state or extracts text elements.
//
// Reference: PDF 1.7 specification, Section 9.4 (Text Objects).
//
//nolint:cyclop,funlen,gocognit,gocyclo // Text operator processing inherently requires many cases
func (te *TextExtractor) processOperator(op *Operator) {
	switch op.Name {
	// Graphics state operators (Section 8.4.2). Text coordinates are expressed
	// in the current user space, so page/Form transformations must be applied.
	case "q":
		te.graphicsStateStack = append(te.graphicsStateStack, textExtractorGraphicsState{
			ctm: te.ctm,
		})

	case "Q":
		if n := len(te.graphicsStateStack); n > 0 {
			saved := te.graphicsStateStack[n-1]
			te.graphicsStateStack = te.graphicsStateStack[:n-1]
			te.ctm = saved.ctm
		}

	case "cm":
		if len(op.Operands) >= 6 {
			values := [6]*float64{}
			for i := range values {
				values[i] = getNumber(op.Operands[i])
			}
			if values[0] != nil && values[1] != nil && values[2] != nil &&
				values[3] != nil && values[4] != nil && values[5] != nil {
				operand := NewMatrix(*values[0], *values[1], *values[2], *values[3], *values[4], *values[5])
				te.ctm = te.ctm.Multiply(operand)
			}
		}

	// Text object delimiters (Section 9.4.1)
	case "BT": // Begin text
		te.textState.Reset()

	case "ET": // End text
		// Text object complete - nothing to do

	// Text state operators (Section 9.3)
	case "Tc": // Set character spacing
		if len(op.Operands) >= 1 {
			if num := getNumber(op.Operands[0]); num != nil {
				te.textState.CharSpace = *num
			}
		}

	case "Tw": // Set word spacing
		if len(op.Operands) >= 1 {
			if num := getNumber(op.Operands[0]); num != nil {
				te.textState.WordSpace = *num
			}
		}

	case "Tz": // Set horizontal scaling
		if len(op.Operands) >= 1 {
			if num := getNumber(op.Operands[0]); num != nil {
				te.textState.HorizScale = *num
			}
		}

	case "TL": // Set text leading
		if len(op.Operands) >= 1 {
			if num := getNumber(op.Operands[0]); num != nil {
				te.textState.Leading = *num
			}
		}

	case "Tf": // Set font and size
		if len(op.Operands) >= 2 {
			if name, ok := op.Operands[0].(*parser.Name); ok {
				te.textState.FontName = name.Value()
				// Load font decoder for this font (lazy loading)
				te.loadFontDecoder(name.Value())
			}
			if num := getNumber(op.Operands[1]); num != nil {
				te.textState.FontSize = *num
			}
		}

	case "Tr": // Set text rendering mode
		// Not needed for text extraction (affects appearance only)

	case "Ts": // Set text rise
		if len(op.Operands) >= 1 {
			if num := getNumber(op.Operands[0]); num != nil {
				te.textState.Rise = *num
			}
		}

	// Text positioning operators (Section 9.4.2)
	case "Td": // Move text position
		if len(op.Operands) >= 2 {
			tx := getNumber(op.Operands[0])
			ty := getNumber(op.Operands[1])
			if tx != nil && ty != nil {
				te.textState.Translate(*tx, *ty)
			}
		}

	case "TD": // Move text position and set leading
		if len(op.Operands) >= 2 {
			tx := getNumber(op.Operands[0])
			ty := getNumber(op.Operands[1])
			if tx != nil && ty != nil {
				te.textState.TranslateSetLeading(*tx, *ty)
			}
		}

	case "Tm": // Set text matrix
		if len(op.Operands) >= 6 {
			a := getNumber(op.Operands[0])
			b := getNumber(op.Operands[1])
			c := getNumber(op.Operands[2])
			d := getNumber(op.Operands[3])
			e := getNumber(op.Operands[4])
			f := getNumber(op.Operands[5])
			if a != nil && b != nil && c != nil && d != nil && e != nil && f != nil {
				te.textState.SetTextMatrix(*a, *b, *c, *d, *e, *f)
			}
		}

	case "T*": // Move to start of next line
		te.textState.MoveToNextLine()

	// Text showing operators (Section 9.4.3)
	case "Tj": // Show text string
		if len(op.Operands) >= 1 {
			if str, ok := op.Operands[0].(*parser.String); ok {
				// Use Bytes() to get raw glyph bytes without UTF-8 conversion
				te.addTextBytes(str.Bytes())
			}
		}

	case "TJ": // Show text with individual glyph positioning
		if len(op.Operands) >= 1 {
			if arr, ok := op.Operands[0].(*parser.Array); ok {
				te.processTextArray(arr)
			}
		}

	case "'": // Move to next line and show text
		te.textState.MoveToNextLine()
		if len(op.Operands) >= 1 {
			if str, ok := op.Operands[0].(*parser.String); ok {
				te.addTextBytes(str.Bytes())
			}
		}

	case "\"": // Set word/char spacing, move to next line, show text
		if len(op.Operands) >= 3 {
			if aw := getNumber(op.Operands[0]); aw != nil {
				te.textState.WordSpace = *aw
			}
			if ac := getNumber(op.Operands[1]); ac != nil {
				te.textState.CharSpace = *ac
			}
			te.textState.MoveToNextLine()
			if str, ok := op.Operands[2].(*parser.String); ok {
				te.addTextBytes(str.Bytes())
			}
		}

	case "Do": // Invoke named XObject (Section 8.8)
		// Form XObjects may contain text. We recurse into them so that PDFs
		// that place all page content inside XObjects (e.g. TCPDF shipping
		// labels) are extracted correctly.
		if len(op.Operands) >= 1 {
			if name, ok := op.Operands[0].(*parser.Name); ok {
				te.processFormXObject(name.Value())
			}
		}
	}
}

// addTextBytes adds text from raw glyph bytes to the extracted elements.
//
// This creates a TextElement with the current position from the text matrix.
// The text is decoded from glyph bytes to Unicode using the current font's CMap/encoding.
func (te *TextExtractor) addTextBytes(glyphBytes []byte) {
	if len(glyphBytes) == 0 {
		return
	}

	// Decode glyph bytes to Unicode text
	decodedText := te.decodeTextBytes(glyphBytes)

	advance, width, precise := te.measureGlyphBytes(glyphBytes)
	positionedFormGeometry := te.usesPositionedFormGeometry()
	if !positionedFormGeometry {
		width = float64(len(decodedText)) * te.textState.FontSize * 0.6 * (te.textState.HorizScale / 100.0)
		advance, precise = width, false
	}
	x, y := te.textState.CurrentX, te.textState.CurrentY
	height, effectiveFontSize := te.textState.FontSize, te.textState.FontSize
	if positionedFormGeometry {
		x, y, width, height, effectiveFontSize = te.transformedTextBounds(width)
	}

	// Create text element with decoded text
	elem := NewTextElement(decodedText, x, y, width, height, te.textState.FontName, effectiveFontSize)
	elem.preciseWidth = precise
	te.elements = append(te.elements, elem)

	// Advance text position
	te.textState.AdvanceX(advance)
}

// transformedTextBounds maps the text-space bounding box through the text
// matrix and current transformation matrix and returns its axis-aligned bounds.
func (te *TextExtractor) transformedTextBounds(width float64) (float64, float64, float64, float64, float64) {
	// Existing direct-page extraction intentionally remains in raw text space
	// for compatibility with the lattice coordinate normalizer. Form XObjects,
	// however, cannot be positioned without inheriting the caller CTM and their
	// own /Matrix. The caller invokes this function only for that positioned
	// Form path, so the accumulated CTM is always authoritative here.
	coordinateMatrix := te.ctm
	bottom := te.textState.Rise
	top := bottom + te.textState.FontSize
	points := [4][2]float64{{0, bottom}, {width, bottom}, {0, top}, {width, top}}

	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, point := range points {
		tx, ty := te.textState.Tm.Transform(point[0], point[1])
		x, y := coordinateMatrix.Transform(tx, ty)
		minX = math.Min(minX, x)
		minY = math.Min(minY, y)
		maxX = math.Max(maxX, x)
		maxY = math.Max(maxY, y)
	}

	baseX, baseY := te.textState.Tm.Transform(0, bottom)
	topX, topY := te.textState.Tm.Transform(0, top)
	baseX, baseY = coordinateMatrix.Transform(baseX, baseY)
	topX, topY = coordinateMatrix.Transform(topX, topY)
	effectiveFontSize := math.Hypot(topX-baseX, topY-baseY)

	return minX, minY, maxX - minX, maxY - minY, effectiveFontSize
}

func (te *TextExtractor) usesPositionedFormGeometry() bool {
	if te.xobjectDepth == 0 || te.textState.FontSize <= 0 {
		return false
	}
	baseX, baseY := te.textState.Tm.Transform(0, te.textState.Rise)
	topX, topY := te.textState.Tm.Transform(0, te.textState.Rise+te.textState.FontSize)
	rawTextSize := math.Hypot(topX-baseX, topY-baseY)
	return rawTextSize > 0 && rawTextSize <= positionedGlyphFontSizeLimit
}

// processTextArray processes a TJ array with positioning adjustments.
//
// The TJ operator takes an array that can contain:
//   - Strings: Text to show
//   - Numbers: Position adjustments (negative values move text forward)
//
// Example: [(Hello) -250 (World)] shows "Hello", moves forward 250 units, shows "World"
//
// Reference: PDF 1.7 specification, Section 9.4.3 (Text Showing Operators).
func (te *TextExtractor) processTextArray(arr *parser.Array) {
	for i := 0; i < arr.Len(); i++ {
		item := arr.Get(i)
		if item == nil {
			continue
		}

		switch obj := item.(type) {
		case *parser.String:
			// Text string - add it
			te.addTextBytes(obj.Bytes())

		case *parser.Integer, *parser.Real:
			// Position adjustment
			if num := getNumber(obj); num != nil {
				// Negative values move forward, positive values move backward
				// The unit is 1/1000 of a text space unit
				adjustment := -*num / 1000.0 * te.textState.FontSize
				te.textState.AdvanceX(adjustment)
			}
		}
	}
}

// processFormXObject processes a Form XObject referenced by a Do operator.
//
// TCPDF and similar PDF generators place all content (text, graphics) inside
// Form XObjects rather than directly in the page content stream. The Do operator
// invokes such objects by name. We recurse into them to extract their text.
//
// Execution model:
//  1. Look up the named XObject in the current resource context (pageResources).
//  2. Verify it is a Form XObject (/Subtype /Form).
//  3. Push the current resources onto the stack and replace them with the
//     XObject's own /Resources (if any), so font lookups use the correct context.
//  4. Parse and process the XObject's content stream operators.
//  5. Pop the saved resources to restore the caller's context.
//
// Infinite-recursion guard: recursion is capped at maxXObjectDepth levels.
//
// Reference: PDF 1.7 specification, Section 8.8.1 (Form XObjects).
//
//nolint:cyclop // XObject resolution requires several type-assertion branches
func (te *TextExtractor) processFormXObject(xobjName string) {
	// Guard against infinite recursion
	if te.xobjectDepth >= maxXObjectDepth {
		return
	}

	// Resolve the XObject dictionary from the current resource context
	xobjectStream := te.resolveXObject(xobjName)
	if xobjectStream == nil {
		return
	}

	// Only handle Form XObjects (/Subtype /Form)
	subtypeObj := xobjectStream.Dictionary().Get("Subtype")
	if subtypeName, ok := subtypeObj.(*parser.Name); !ok || subtypeName.Value() != "Form" {
		return
	}

	// Decode the XObject content stream
	contentData, err := te.decodeStream(xobjectStream)
	if err != nil || len(contentData) == 0 {
		return
	}

	// Push current resources and switch to XObject's resources (if present)
	savedResources := te.pageResources
	savedCTM := te.ctm
	savedGraphicsDepth := len(te.graphicsStateStack)
	te.resourceStack = append(te.resourceStack, savedResources)
	te.xobjectDepth++

	xobjResources := te.getXObjectResources(xobjectStream)
	if xobjResources != nil {
		te.pageResources = xobjResources
	}
	if formMatrix, ok := te.getFormMatrix(xobjectStream); ok {
		te.ctm = te.ctm.Multiply(formMatrix)
	}

	// Parse and process the XObject's content stream
	contentParser := NewContentParser(contentData)
	operators, err := contentParser.ParseOperators()
	if err == nil {
		for _, op := range operators {
			te.processOperator(op)
		}
	}

	// Restore saved resources and depth counter
	te.xobjectDepth--
	te.ctm = savedCTM
	te.graphicsStateStack = te.graphicsStateStack[:savedGraphicsDepth]
	if len(te.resourceStack) > 0 {
		te.pageResources = te.resourceStack[len(te.resourceStack)-1]
		te.resourceStack = te.resourceStack[:len(te.resourceStack)-1]
	} else {
		te.pageResources = savedResources
	}
}

// getFormMatrix resolves a Form XObject's optional /Matrix entry.
func (te *TextExtractor) getFormMatrix(stream *parser.Stream) (Matrix, bool) {
	matrixObj := stream.Dictionary().Get("Matrix")
	if ref, ok := matrixObj.(*parser.IndirectReference); ok {
		resolved, err := te.reader.GetObject(ref.Number)
		if err != nil {
			return Matrix{}, false
		}
		matrixObj = resolved
	}
	arr, ok := matrixObj.(*parser.Array)
	if !ok || arr.Len() != 6 {
		return Matrix{}, false
	}
	values := [6]float64{}
	for i := range values {
		num := getNumber(arr.Get(i))
		if num == nil {
			return Matrix{}, false
		}
		values[i] = *num
	}
	return NewMatrix(values[0], values[1], values[2], values[3], values[4], values[5]), true
}

// resolveXObject looks up a named XObject from the current pageResources and
// returns the resolved Stream, or nil if not found or not a stream.
func (te *TextExtractor) resolveXObject(name string) *parser.Stream {
	if te.pageResources == nil {
		return nil
	}

	xobjDictObj := te.pageResources.Get("XObject")
	if xobjDictObj == nil {
		return nil
	}

	// Resolve indirect reference
	if ref, ok := xobjDictObj.(*parser.IndirectReference); ok {
		resolved, err := te.reader.GetObject(ref.Number)
		if err != nil {
			return nil
		}
		xobjDictObj = resolved
	}

	xobjDict, ok := xobjDictObj.(*parser.Dictionary)
	if !ok {
		return nil
	}

	// Look up the specific XObject by name
	xobj := xobjDict.Get(name)
	if xobj == nil {
		return nil
	}

	// Resolve indirect reference to the XObject itself
	if ref, ok := xobj.(*parser.IndirectReference); ok {
		resolved, err := te.reader.GetObject(ref.Number)
		if err != nil {
			return nil
		}
		xobj = resolved
	}

	stream, _ := xobj.(*parser.Stream)
	return stream
}

// getXObjectResources extracts the /Resources dictionary from a Form XObject stream.
//
// Form XObjects may declare their own font and other resource dictionaries.
// When they do, those resources take precedence for the duration of the XObject.
// Returns nil when the XObject has no /Resources entry.
func (te *TextExtractor) getXObjectResources(stream *parser.Stream) *parser.Dictionary {
	resourcesObj := stream.Dictionary().Get("Resources")
	if resourcesObj == nil {
		return nil
	}

	if ref, ok := resourcesObj.(*parser.IndirectReference); ok {
		resolved, err := te.reader.GetObject(ref.Number)
		if err != nil {
			return nil
		}
		resourcesObj = resolved
	}

	dict, _ := resourcesObj.(*parser.Dictionary)
	return dict
}

// getNumber extracts a numeric value from a PDF object.
//
// Returns nil if the object is not a number.
func getNumber(obj parser.PdfObject) *float64 {
	switch v := obj.(type) {
	case *parser.Integer:
		val := float64(v.Value())
		return &val
	case *parser.Real:
		val := v.Value()
		return &val
	default:
		return nil
	}
}

// getPageResources retrieves the Resources dictionary from a page.
//
// Resources can be inherited from parent nodes in the page tree,
// so we need to traverse up the tree if not found on the page itself.
//
// Reference: PDF 1.7 specification, Section 7.7.3.4 (Page Objects).
func (te *TextExtractor) getPageResources(page *parser.Dictionary) *parser.Dictionary {
	// Try to get Resources from page
	resourcesObj := page.Get("Resources")
	if resourcesObj != nil {
		// Resolve if it's an indirect reference
		if ref, ok := resourcesObj.(*parser.IndirectReference); ok {
			resolved, err := te.reader.GetObject(ref.Number)
			if err == nil {
				if dict, ok := resolved.(*parser.Dictionary); ok {
					return dict
				}
			}
		}
		// Direct dictionary
		if dict, ok := resourcesObj.(*parser.Dictionary); ok {
			return dict
		}
	}

	// Resources not found or not a dictionary - return empty dictionary
	return parser.NewDictionary()
}

// loadFontDecoder loads the font decoder for the given font name.
//
// This method:
//  1. Looks up the font in the page's Resources/Font dictionary
//  2. Extracts the ToUnicode CMap stream (if present)
//  3. Parses the CMap to build a glyph-to-Unicode mapping table
//  4. Creates a FontDecoder for this font
//  5. Caches the decoder for reuse
//
// If the font cannot be loaded or has no ToUnicode CMap, we create
// a default decoder that will use fallback encoding (Latin-1).
func (te *TextExtractor) loadFontDecoder(fontName string) {
	// Check if already loaded
	if _, exists := te.fontDecoders[fontName]; exists {
		return
	}

	// Get Font dictionary from Resources
	fontsObj := te.pageResources.Get("Font")
	if fontsObj == nil {
		// No fonts in resources - use default decoder
		te.fontDecoders[fontName] = NewFontDecoder(nil, "", false)
		return
	}

	// Resolve Font dictionary
	var fontsDict *parser.Dictionary
	if ref, ok := fontsObj.(*parser.IndirectReference); ok {
		resolved, err := te.reader.GetObject(ref.Number)
		if err == nil {
			fontsDict, _ = resolved.(*parser.Dictionary)
		}
	} else {
		fontsDict, _ = fontsObj.(*parser.Dictionary)
	}

	if fontsDict == nil {
		// Font dictionary not found - use default decoder
		te.fontDecoders[fontName] = NewFontDecoder(nil, "", false)
		return
	}

	// Get the specific font object
	fontObj := fontsDict.Get(fontName)
	if fontObj == nil {
		// Font not found - use default decoder
		te.fontDecoders[fontName] = NewFontDecoder(nil, "", false)
		return
	}

	// Resolve font object
	var fontDict *parser.Dictionary
	if ref, ok := fontObj.(*parser.IndirectReference); ok {
		resolved, err := te.reader.GetObject(ref.Number)
		if err == nil {
			fontDict, _ = resolved.(*parser.Dictionary)
		}
	} else {
		fontDict, _ = fontObj.(*parser.Dictionary)
	}

	if fontDict == nil {
		// Font dictionary not resolved - use default decoder
		te.fontDecoders[fontName] = NewFontDecoder(nil, "", false)
		return
	}

	// Load widths alongside the decoder so positioned glyphs receive accurate
	// bounds and word-gap detection can use a conservative threshold.
	te.fontMetrics[fontName] = te.loadFontMetrics(fontDict)

	// Extract encoding name AND Differences array
	encodingName := ""
	var differences map[uint16]string

	if encodingObj := fontDict.Get("Encoding"); encodingObj != nil {
		// Case 1: Encoding is a simple name (e.g., /WinAnsiEncoding)
		if name, ok := encodingObj.(*parser.Name); ok {
			encodingName = name.Value()
		} else {
			// Case 2: Encoding is a dictionary (custom encoding with Differences)
			// Resolve if its an indirect reference
			if ref, ok := encodingObj.(*parser.IndirectReference); ok {
				resolved, err := te.reader.GetObject(ref.Number)
				if err == nil {
					encodingObj = resolved
				}
			}

			// Now check if its a dictionary
			if encDict, ok := encodingObj.(*parser.Dictionary); ok {
				// Get BaseEncoding (if specified)
				if baseEnc := encDict.Get("BaseEncoding"); baseEnc != nil {
					if name, ok := baseEnc.(*parser.Name); ok {
						encodingName = name.Value()
					}
				}

				// Parse Differences array (custom glyph mappings)
				differences = te.parseDifferencesArray(encDict)
			}
		}
	}

	// Detect whether this is a composite (Type0) font. Composite fonts use
	// 2-byte character codes and must never be downgraded to 1-byte decoding.
	isType0 := false
	if subtypeObj := fontDict.Get("Subtype"); subtypeObj != nil {
		if subtypeName, ok := subtypeObj.(*parser.Name); ok {
			isType0 = subtypeName.Value() == type0FontSubtype
		}
	}

	// Identity-H/V and composite fonts always use 2-byte glyph codes.
	use2ByteGlyphs := strings.Contains(encodingName, "Identity") || isType0

	// Try to get ToUnicode CMap
	toUnicodeObj := fontDict.Get("ToUnicode")
	if toUnicodeObj == nil {
		// No ToUnicode CMap - check if we have Differences array
		if differences != nil && len(differences) > 0 {
			// Create decoder with custom encoding (Differences array).
			// use2ByteGlyphs is false for simple fonts with Differences.
			te.fontDecoders[fontName] = NewFontDecoderWithCustomEncoding(differences, encodingName, false)
		} else {
			// Fallback: create decoder with encoding name only.
			// For Identity-H/V and Type0 fonts, honor 2-byte mode.
			decoder := NewFontDecoder(nil, encodingName, use2ByteGlyphs)
			if isType0 {
				decoder.isCompositeFont = true
			}
			te.fontDecoders[fontName] = decoder
		}
		return
	}

	// Resolve ToUnicode stream
	var toUnicodeStream *parser.Stream
	if ref, ok := toUnicodeObj.(*parser.IndirectReference); ok {
		resolved, err := te.reader.GetObject(ref.Number)
		if err == nil {
			toUnicodeStream, _ = resolved.(*parser.Stream)
		}
	} else {
		toUnicodeStream, _ = toUnicodeObj.(*parser.Stream)
	}

	if toUnicodeStream == nil {
		// ToUnicode is not a stream - create decoder with encoding only
		decoder := NewFontDecoder(nil, encodingName, use2ByteGlyphs)
		if isType0 {
			decoder.isCompositeFont = true
		}
		te.fontDecoders[fontName] = decoder
		return
	}

	// Decode the CMap stream (handle compression)
	cmapData, err := te.decodeStream(toUnicodeStream)
	if err != nil {
		// Failed to decode stream - create decoder with encoding only
		decoder := NewFontDecoder(nil, encodingName, use2ByteGlyphs)
		if isType0 {
			decoder.isCompositeFont = true
		}
		te.fontDecoders[fontName] = decoder
		return
	}

	// Parse CMap
	cmap, err := ParseCMapStream(cmapData)
	if err != nil {
		// Failed to parse CMap - create decoder with encoding only
		decoder := NewFontDecoder(nil, encodingName, use2ByteGlyphs)
		if isType0 {
			decoder.isCompositeFont = true
		}
		te.fontDecoders[fontName] = decoder
		return
	}

	// When begincodespacerange declared 2-byte codes, honor that even if
	// no Identity encoding name was present (e.g. some CJK fonts).
	if cmap.CodeBytes == 2 {
		use2ByteGlyphs = true
	}

	// Create decoder with CMap
	decoder := NewFontDecoder(cmap, encodingName, use2ByteGlyphs)
	if isType0 {
		decoder.isCompositeFont = true
	}

	// Add Differences array if present (for fonts with custom encoding)
	if differences != nil && len(differences) > 0 {
		customEncoding := buildCustomEncoding(differences)
		decoder.customEncoding = customEncoding
	}

	te.fontDecoders[fontName] = decoder
}

// decodeTextBytes decodes glyph bytes to Unicode text using the current font decoder.
//
// This method looks up the decoder for the current font and uses it to
// convert raw glyph bytes (from PDF text operators) to readable Unicode text.
//
// If no decoder is available for the current font, it treats the bytes as Latin-1.
func (te *TextExtractor) decodeTextBytes(glyphBytes []byte) string {
	// Get decoder for current font
	decoder, exists := te.fontDecoders[te.textState.FontName]
	if !exists {
		// No decoder - treat as Latin-1 (fallback)
		return string(glyphBytes)
	}

	// Decode using font decoder (no conversion needed - already []byte)
	return decoder.DecodeString(glyphBytes)
}

// parseDifferencesArray parses the /Differences array from an Encoding dictionary.
//
// The Differences array specifies custom glyph name mappings that override
// the base encoding. The format is (PDF 1.7 Section 9.6.6.1):
//
//	[code1 /name1 /name2 ... codeN /nameN ...]
//
// Example:
//
//	[1 /zero /one /two /three /four /five /six /seven /eight /nine]
//	→ Glyph 1='zero', 2='one', ..., 10='nine'
//
// This is used when a font has custom glyph IDs that don't match standard encodings.
// For example, a font might map digits to non-standard glyph IDs (like 0x01-0x0A
// instead of 0x30-0x39).
//
// Returns: map[glyphID]glyphName
func (te *TextExtractor) parseDifferencesArray(encodingDict *parser.Dictionary) map[uint16]string {
	logger := logging.Logger().With(slog.String("func", "parseDifferencesArray"))

	differences := make(map[uint16]string)

	diffsObj := encodingDict.Get("Differences")
	if diffsObj == nil {
		logger.Debug("No Differences found in encoding dictionary")
		return differences
	}
	logger.Debug("Differences object found", slog.Any("type", diffsObj))

	// Resolve if indirect reference
	if ref, ok := diffsObj.(*parser.IndirectReference); ok {
		resolved, err := te.reader.GetObject(ref.Number)
		if err == nil {
			diffsObj = resolved
		} else {
			return differences
		}
	}

	diffsArr, ok := diffsObj.(*parser.Array)
	if !ok {
		return differences
	}

	// Parse array: alternating integers (starting codes) and names (glyph names)
	// Format: [code1 name1 name2 name3 code2 name4 name5 ...]
	var currentCode int
	for i := 0; i < diffsArr.Len(); i++ {
		elem := diffsArr.Get(i)
		if elem == nil {
			continue
		}

		// Check if element is an integer (new starting code)
		if intObj, ok := elem.(*parser.Integer); ok {
			currentCode = int(intObj.Value())
		} else if name, ok := elem.(*parser.Name); ok {
			// Element is a glyph name
			glyphName := name.Value()
			// Remove leading '/' if present (PDF names sometimes include it)
			if len(glyphName) > 0 && glyphName[0] == '/' {
				glyphName = glyphName[1:]
			}
			differences[uint16(currentCode)] = glyphName
			currentCode++
			if currentCode <= 11 { // Log first 10 mappings
				logger.Debug("Mapped glyph",
					slog.Int("code", currentCode-1),
					slog.String("name", glyphName),
				)
			}
		}
	}

	logger.Debug("Finished", slog.Int("total_mappings", len(differences)))
	return differences
}
