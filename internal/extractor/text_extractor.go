package extractor

import (
	"compress/zlib"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/coregx/gxpdf/internal/parser"
	"github.com/coregx/gxpdf/logging"
)

// filterFlateDecode is the PDF filter name for zlib/deflate compression.
const filterFlateDecode = "FlateDecode"

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
//
// Reference: PDF 1.7 specification, Section 9.4 (Text Objects).
type TextExtractor struct {
	reader        *parser.Reader
	textState     *TextState
	elements      []*TextElement
	fontDecoders  map[string]*FontDecoder // fontName -> FontDecoder
	pageResources *parser.Dictionary      // Current page resources
}

// NewTextExtractor creates a new TextExtractor for the given PDF reader.
func NewTextExtractor(reader *parser.Reader) *TextExtractor {
	return &TextExtractor{
		reader:       reader,
		textState:    NewTextState(),
		elements:     []*TextElement{},
		fontDecoders: make(map[string]*FontDecoder),
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

	return te.elements, nil
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

	// Get current position from text matrix
	x := te.textState.CurrentX
	y := te.textState.CurrentY

	// Estimate width (simple heuristic - will be improved with font metrics in Phase 3)
	// Use decoded text length for more accurate width calculation
	charWidth := te.textState.FontSize * 0.6 * (te.textState.HorizScale / 100.0)
	width := float64(len(decodedText)) * charWidth
	height := te.textState.FontSize

	// Create text element with decoded text
	elem := NewTextElement(decodedText, x, y, width, height, te.textState.FontName, te.textState.FontSize)
	te.elements = append(te.elements, elem)

	// Advance text position
	te.textState.AdvanceX(width)
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
			isType0 = subtypeName.Value() == "Type0"
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
