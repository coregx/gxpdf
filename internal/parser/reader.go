// Package parser implements PDF document reading and parsing.
package parser

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/coregx/gxpdf/internal/encoding"
)

// PDF filter name constants.
const (
	filterFlateDecode = "FlateDecode"
	filterDCTDecode   = "DCTDecode"
)

// Page tree node type constants.
const (
	nodeTypePage  = "Page"
	nodeTypePages = "Pages"
)

// maxXRefChainDepth is the maximum number of /Prev links to follow
// in the cross-reference chain. This prevents infinite loops in
// malformed PDFs with deep or circular /Prev chains.
const maxXRefChainDepth = 100

// Reader reads and parses PDF documents, providing access to document structure.
//
// The Reader ties together all parser components (Lexer, Parser, XRef) to read
// actual PDF files according to PDF 1.7 specification.
//
// PDF File Structure (Section 7.5):
//   - Header: %PDF-X.Y
//   - Body: Indirect objects
//   - Cross-reference table: Object locations
//   - Trailer: Document metadata
//   - startxref: XRef table offset
//   - %%EOF: End of file marker
//
// Thread Safety:
// Reader is thread-safe for concurrent reads using sync.RWMutex for cache
// and sync.Mutex for file access.
// Multiple goroutines can safely call GetObject() simultaneously.
//
// Reference: PDF 1.7 specification, Section 7.5 (File Structure).
type Reader struct {
	file      *os.File
	filename  string
	version   string
	xrefTable *XRefTable
	trailer   *Dictionary
	catalog   *Dictionary
	pages     *Dictionary

	// headerOffset is the number of bytes before the %PDF- marker.
	// Some PDFs have leading whitespace that shifts all internal byte offsets.
	// This offset must be added to all file positions read from the PDF.
	headerOffset int64

	// Object cache for resolved indirect references
	// Key: object number, Value: resolved object
	objectCache map[int]PdfObject
	mu          sync.RWMutex

	// Object Stream cache for compressed objects (PDF 1.5+)
	// Key: ObjStm object number, Value: map of contained objects
	objStmCache map[int]map[int]PdfObject

	// File access mutex (for seek and read operations)
	fileMu sync.Mutex
}

// NewReader creates a new PDF document reader.
//
// The filename is stored but the file is not opened until Open() is called.
// This allows for resource management and lazy loading.
func NewReader(filename string) *Reader {
	return &Reader{
		filename:    filename,
		objectCache: make(map[int]PdfObject),
		objStmCache: make(map[int]map[int]PdfObject),
	}
}

// Open opens the PDF file and parses its structure.
//
// Steps performed:
//  1. Open file
//  2. Read and validate PDF header
//  3. Find startxref offset
//  4. Parse cross-reference table and trailer
//  5. Load document catalog
//  6. Load page tree root
//
// Returns error if file cannot be opened or is not a valid PDF.
//
// Reference: PDF 1.7 specification, Section 7.5 (File Structure).
func (r *Reader) Open() error {
	// Open file
	file, err := os.Open(r.filename)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	r.file = file

	// Read and validate header, get offset of leading whitespace
	version, headerOffset, err := r.readHeader()
	if err != nil {
		_ = r.Close()
		return fmt.Errorf("failed to read PDF header: %w", err)
	}
	r.version = version
	r.headerOffset = headerOffset

	// Find startxref offset
	startxrefOffset, err := r.findStartXRef()
	if err != nil {
		_ = r.Close()
		return fmt.Errorf("failed to find startxref: %w", err)
	}

	// Parse XRef and trailer
	if err := r.parseXRefAndTrailer(startxrefOffset); err != nil {
		_ = r.Close()
		return fmt.Errorf("failed to parse xref table: %w", err)
	}

	// Load catalog
	if err := r.loadCatalog(); err != nil {
		_ = r.Close()
		return fmt.Errorf("failed to load catalog: %w", err)
	}

	return nil
}

// Close closes the PDF file and releases resources.
func (r *Reader) Close() error {
	if r.file != nil {
		err := r.file.Close()
		r.file = nil
		return err
	}
	return nil
}

// adjustOffset adds the header offset to a file position read from the PDF.
// PDF internal offsets assume %PDF- is at byte 0, but some files have leading
// whitespace that shifts all content. This method corrects for that shift.
func (r *Reader) adjustOffset(offset int64) int64 {
	return offset + r.headerOffset
}

// maxHeaderSearchSize is the maximum number of bytes to search for the PDF header.
// PDF 1.7 Appendix H.3 specifies that Acrobat viewers require the header to appear
// somewhere within the first 1024 bytes of the file.
const maxHeaderSearchSize = 1024

// readHeader reads and validates the PDF header.
//
// Expected format: %PDF-X.Y (e.g., %PDF-1.7)
//
// The header must appear within the first 1024 bytes of the file, after any
// leading whitespace or UTF-8 BOM. Some PDF generators produce files with
// leading whitespace (tabs, spaces, newlines) or a UTF-8 BOM before the header.
// We allow these prefixes and then verify the file contains %PDF-.
//
// Some PDFs may have binary data after the header to prevent
// misinterpretation as text files.
//
// Returns the PDF version string (e.g., "1.7") and the byte offset of the
// %PDF- marker. The offset is used to adjust all internal file positions,
// since PDF byte offsets are calculated from the %PDF- marker, not from
// the actual start of the file.
//
// Reference: PDF 1.7 specification, Section 7.5.1 (File Header) and Appendix H.3.
func (r *Reader) readHeader() (version string, headerOffset int64, err error) {
	// Seek to start of file
	if _, err := r.file.Seek(0, io.SeekStart); err != nil {
		return "", 0, fmt.Errorf("failed to seek to start: %w", err)
	}

	// Read first 1024 bytes (PDF spec Appendix H.3)
	buf := make([]byte, maxHeaderSearchSize)
	n, err := r.file.Read(buf)
	if err != nil && err != io.EOF {
		return "", 0, fmt.Errorf("failed to read header: %w", err)
	}
	if n == 0 {
		return "", 0, fmt.Errorf("empty file")
	}
	buf = buf[:n]

	// Find %PDF- marker and calculate offset
	const pdfMarker = "%PDF-"
	content := string(buf)
	idx := strings.Index(content, pdfMarker)
	if idx < 0 {
		// Show first 20 bytes for debugging
		preview := content
		if len(preview) > 20 {
			preview = preview[:20]
		}
		return "", 0, fmt.Errorf("invalid PDF header: %q (expected %%PDF-X.Y)", preview)
	}

	// Verify only whitespace (and optional UTF-8 BOM) before the marker
	prefix := content[:idx]
	// Strip UTF-8 BOM if present
	prefix = strings.TrimPrefix(prefix, "\xef\xbb\xbf")
	if strings.TrimLeft(prefix, " \t\r\n") != "" {
		preview := content
		if len(preview) > 20 {
			preview = preview[:20]
		}
		return "", 0, fmt.Errorf("invalid PDF header: %q (expected %%PDF-X.Y)", preview)
	}

	headerOffset = int64(idx)

	// Extract header line (up to first newline)
	header := content[idx:]
	if newlineIdx := strings.IndexAny(header, "\r\n"); newlineIdx > 0 {
		header = header[:newlineIdx]
	}

	// Trim any trailing whitespace from header
	header = strings.TrimSpace(header)

	// Extract version (e.g., "1.7" from "%PDF-1.7")
	version = strings.TrimPrefix(header, pdfMarker)
	if len(version) < 3 {
		return "", 0, fmt.Errorf("invalid PDF version in header: %q", header)
	}

	return version, headerOffset, nil
}

// findStartXRef finds the byte offset of the cross-reference table.
//
// The startxref keyword and offset are located near the end of the file:
//
//	startxref
//	byte_offset
//	%%EOF
//
// According to the PDF spec, this should be within the last 1024 bytes.
// However, we search the last 2048 bytes to be more tolerant of malformed PDFs.
//
// Returns the byte offset of the xref table.
//
// Reference: PDF 1.7 specification, Section 7.5.5 (File Trailer).
func (r *Reader) findStartXRef() (int64, error) {
	// Get file size
	fileInfo, err := r.file.Stat()
	if err != nil {
		return 0, fmt.Errorf("failed to stat file: %w", err)
	}

	size := fileInfo.Size()
	if size == 0 {
		return 0, fmt.Errorf("file is empty")
	}

	// Search last 2048 bytes (spec says 1024, but be tolerant)
	searchSize := int64(2048)
	if size < searchSize {
		searchSize = size
	}

	// Seek to search region
	offset := size - searchSize
	if _, err := r.file.Seek(offset, io.SeekStart); err != nil {
		return 0, fmt.Errorf("failed to seek to end region: %w", err)
	}

	// Read search region
	buf := make([]byte, searchSize)
	n, err := io.ReadFull(r.file, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return 0, fmt.Errorf("failed to read end region: %w", err)
	}
	buf = buf[:n]

	// Find last occurrence of "startxref"
	content := string(buf)
	idx := strings.LastIndex(content, "startxref")
	if idx == -1 {
		return 0, fmt.Errorf("startxref keyword not found in last %d bytes", searchSize)
	}

	// Parse the offset after "startxref"
	// Format: startxref\n123\n%%EOF
	afterKeyword := content[idx+9:] // Skip "startxref"

	// Find the number (skip whitespace)
	lines := strings.Split(afterKeyword, "\n")
	if len(lines) < 2 {
		return 0, fmt.Errorf("invalid startxref format: expected offset after keyword")
	}

	// The offset should be in the next non-empty line
	offsetStr := ""
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line != "" && line != "%%EOF" {
			offsetStr = line
			break
		}
	}

	if offsetStr == "" {
		return 0, fmt.Errorf("startxref offset not found")
	}

	// Parse offset
	startxrefOffset, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid startxref offset %q: %w", offsetStr, err)
	}

	if startxrefOffset < 0 || startxrefOffset >= size {
		return 0, fmt.Errorf("startxref offset %d out of bounds (file size: %d)", startxrefOffset, size)
	}

	return startxrefOffset, nil
}

// parseXRefAndTrailer parses the cross-reference chain following /Prev links.
//
// PDF files with incremental updates have multiple xref sections linked via
// /Prev entries in their trailers. Hybrid-reference PDFs (e.g., MS Word)
// also use /XRefStm to point to a supplementary xref stream.
//
// This method follows the entire chain:
//  1. Parse xref section at startxref offset (newest)
//  2. If trailer has /XRefStm, parse supplementary xref stream and merge
//  3. If trailer has /Prev, follow to older xref section and repeat
//  4. Newer entries always take precedence over older ones
//
// The first (newest) trailer provides /Root, /Info, /ID etc.
//
// Reference: PDF 1.7 specification, Section 7.5.4, 7.5.5, 7.5.6, and 7.5.8.
func (r *Reader) parseXRefAndTrailer(offset int64) error {
	masterXRef := NewXRefTable()
	var masterTrailer *Dictionary

	visitedOffsets := make(map[int64]bool)
	currentOffset := offset

	for depth := 0; currentOffset >= 0; depth++ {
		// Depth limit check
		if depth >= maxXRefChainDepth {
			return fmt.Errorf("xref chain exceeds maximum depth of %d (possible corruption)", maxXRefChainDepth)
		}

		// Cycle detection
		if visitedOffsets[currentOffset] {
			return fmt.Errorf("xref chain cycle detected at offset %d", currentOffset)
		}
		visitedOffsets[currentOffset] = true

		// Parse single xref section
		localXRef, localTrailer, err := r.parseSingleXRef(currentOffset)
		if err != nil {
			return fmt.Errorf("failed to parse xref at offset %d: %w", currentOffset, err)
		}

		// Merge: newer (already in masterXRef) wins over older (localXRef)
		masterXRef.MergeOlder(localXRef)

		// Save first trailer as master (newest trailer has /Root, /Info, etc.)
		if masterTrailer == nil {
			masterTrailer = localTrailer
		}

		// Handle /XRefStm (hybrid-reference PDF)
		if xrefStmOffset := localTrailer.GetInteger("XRefStm"); xrefStmOffset > 0 {
			if !visitedOffsets[xrefStmOffset] {
				visitedOffsets[xrefStmOffset] = true
				stmXRef, _, err := r.parseSingleXRef(xrefStmOffset)
				if err != nil {
					return fmt.Errorf("failed to parse /XRefStm at offset %d: %w", xrefStmOffset, err)
				}
				// XRefStm supplements the same revision — merge as older
				masterXRef.MergeOlder(stmXRef)
			}
		}

		// Follow /Prev to older xref section
		if prevOffset := localTrailer.GetInteger("Prev"); prevOffset > 0 {
			currentOffset = prevOffset
		} else {
			currentOffset = -1 // No more /Prev, end of chain
		}
	}

	r.xrefTable = masterXRef
	r.trailer = masterTrailer

	return nil
}

// parseSingleXRef parses a single cross-reference section (table or stream)
// at the given file offset and returns the xref table and trailer dictionary.
func (r *Reader) parseSingleXRef(offset int64) (*XRefTable, *Dictionary, error) {
	// Adjust offset for any leading whitespace before %PDF- header
	adjustedOffset := r.adjustOffset(offset)

	// Seek to XRef offset
	if _, err := r.file.Seek(adjustedOffset, io.SeekStart); err != nil {
		return nil, nil, fmt.Errorf("failed to seek to xref at offset %d: %w", offset, err)
	}

	// Peek at first few bytes to determine xref type
	peekBuf := make([]byte, 10)
	n, err := r.file.Read(peekBuf)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to peek at xref: %w", err)
	}

	// Seek back to start of xref
	if _, err := r.file.Seek(adjustedOffset, io.SeekStart); err != nil {
		return nil, nil, fmt.Errorf("failed to seek back to xref: %w", err)
	}

	// Check if it's a traditional xref table or xref stream
	isXRefStream := false
	if n >= 4 {
		if peekBuf[0] >= '0' && peekBuf[0] <= '9' {
			isXRefStream = true
		}
	}

	if isXRefStream {
		xrefTable, err := r.parseXRefStream(adjustedOffset)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse xref stream: %w", err)
		}
		return xrefTable, xrefTable.Trailer, nil
	}

	// Parse traditional xref table
	parser := NewParser(r.file)
	xrefTable, err := parser.ParseXRef()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse xref table: %w", err)
	}
	return xrefTable, xrefTable.Trailer, nil
}

// parseXRefStream parses a PDF 1.5+ cross-reference stream.
//
// This method handles xref streams by:
//  1. Parsing the stream object header and dictionary using Parser
//  2. Seeking directly to stream data in the file (avoiding lexer buffer issues)
//  3. Reading and decoding the compressed stream data
//  4. Parsing binary xref entries
//
// Reference: PDF 1.7 specification, Section 7.5.8.
func (r *Reader) parseXRefStream(xrefOffset int64) (*XRefTable, error) {
	// Create a parser to read the object header and dictionary
	parser := NewParser(r.file)

	// Call the parser's ParseXRefStream, but we'll need to handle stream reading ourselves
	// For now, let's parse just the object structure
	xrefTable, err := parser.ParseXRefStreamWithFileAccess(r.file, xrefOffset)
	if err != nil {
		return nil, err
	}

	return xrefTable, nil
}

// loadCatalog loads the document catalog (root object).
//
// The catalog is the root of the PDF's object hierarchy and contains
// references to all major document structures:
//   - /Pages: Page tree root
//   - /Outlines: Document outline (bookmarks)
//   - /Names: Named destinations
//   - /Metadata: Document metadata
//
// Reference: PDF 1.7 specification, Section 7.7.2 (Document Catalog).
func (r *Reader) loadCatalog() error {
	// Get /Root from trailer
	rootRef := r.trailer.Get("Root")
	if rootRef == nil {
		return fmt.Errorf("trailer missing /Root entry")
	}

	// Resolve catalog
	catalog, err := r.resolveDictionary(rootRef)
	if err != nil {
		return fmt.Errorf("failed to resolve catalog: %w", err)
	}

	// Verify it's a Catalog
	typeObj := catalog.GetName("Type")
	if typeObj != nil && typeObj.Value() != "Catalog" {
		return fmt.Errorf("root object has wrong /Type: %q (expected 'Catalog')", typeObj.Value())
	}

	r.catalog = catalog

	// Load Pages tree root
	pagesRef := catalog.Get("Pages")
	if pagesRef == nil {
		return fmt.Errorf("catalog missing /Pages entry")
	}

	pages, err := r.resolveDictionary(pagesRef)
	if err != nil {
		return fmt.Errorf("failed to resolve pages tree: %w", err)
	}

	// Verify it's a Pages object
	typeObj = pages.GetName("Type")
	if typeObj != nil && typeObj.Value() != "Pages" {
		return fmt.Errorf("pages object has wrong /Type: %q (expected 'Pages')", typeObj.Value())
	}

	r.pages = pages

	return nil
}

// GetObject retrieves and resolves an indirect object by number.
//
// The object is looked up in the cross-reference table, loaded from
// the file at the specified offset, and cached for future access.
//
// For PDF 1.5+ compressed objects (stored in Object Streams), the
// method automatically loads and parses the containing ObjStm.
//
// Nested indirect references are automatically resolved.
//
// Thread-safe: Multiple goroutines can call this method concurrently.
//
// Returns error if object is not found or cannot be parsed.
func (r *Reader) GetObject(objectNum int) (PdfObject, error) {
	// Check cache first (read lock)
	r.mu.RLock()
	if obj, ok := r.objectCache[objectNum]; ok {
		r.mu.RUnlock()
		return obj, nil
	}
	r.mu.RUnlock()

	// Get XRef entry
	entry, ok := r.xrefTable.GetEntry(objectNum)
	if !ok {
		return nil, fmt.Errorf("object %d not found in xref table", objectNum)
	}

	// Handle different entry types
	switch entry.Type {
	case XRefEntryInUse:
		// Traditional in-use object
		return r.getInUseObject(objectNum, entry)

	case XRefEntryCompressed:
		// PDF 1.5+ compressed object (in Object Stream)
		return r.getCompressedObject(objectNum, entry)

	case XRefEntryFree:
		return nil, fmt.Errorf("object %d is free (deleted)", objectNum)

	default:
		return nil, fmt.Errorf("object %d has unknown entry type: %s", objectNum, entry.Type)
	}
}

// getInUseObject retrieves a traditional in-use object from the file.
func (r *Reader) getInUseObject(objectNum int, entry *XRefEntry) (PdfObject, error) {
	// Seek and parse (lock file access)
	r.fileMu.Lock()

	// Seek to object offset (adjust for any leading whitespace before %PDF- header)
	adjustedOffset := r.adjustOffset(entry.Offset)
	if _, err := r.file.Seek(adjustedOffset, io.SeekStart); err != nil {
		r.fileMu.Unlock()
		return nil, fmt.Errorf("failed to seek to object %d at offset %d: %w",
			objectNum, entry.Offset, err)
	}

	// Parse indirect object
	parser := NewParser(r.file)
	indirectObj, err := parser.ParseIndirectObject()
	r.fileMu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to parse object %d: %w", objectNum, err)
	}

	// Verify object number matches
	if indirectObj.Number != objectNum {
		return nil, fmt.Errorf("object number mismatch: expected %d, got %d",
			objectNum, indirectObj.Number)
	}

	// Verify generation number matches
	if indirectObj.Generation != entry.Generation {
		return nil, fmt.Errorf("object %d generation mismatch: expected %d, got %d",
			objectNum, entry.Generation, indirectObj.Generation)
	}

	// Get the object (do NOT auto-resolve references to avoid circular refs)
	obj := indirectObj.Object

	// Cache the object (write lock)
	r.mu.Lock()
	r.objectCache[objectNum] = obj
	r.mu.Unlock()

	return obj, nil
}

// getCompressedObject retrieves a compressed object from an Object Stream (PDF 1.5+).
//
// Compressed objects are stored in special stream objects (Type /ObjStm) along
// with other objects for space efficiency.
//
// Reference: PDF 1.7 specification, Section 7.5.7 (Object Streams).
func (r *Reader) getCompressedObject(objectNum int, entry *XRefEntry) (PdfObject, error) {
	// entry.Offset contains the ObjStm object number
	// entry.Generation contains the index within that ObjStm
	objStmNum := int(entry.Offset)
	objIndex := entry.Generation

	// Check if we've already parsed this ObjStm (read lock)
	r.mu.RLock()
	if objStmObjects, ok := r.objStmCache[objStmNum]; ok {
		if obj, ok := objStmObjects[objectNum]; ok {
			r.mu.RUnlock()
			return obj, nil
		}
		r.mu.RUnlock()
		return nil, fmt.Errorf("object %d not found in ObjStm %d at index %d", objectNum, objStmNum, objIndex)
	}
	r.mu.RUnlock()

	// Need to load and parse the ObjStm (write lock for cache)
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine might have loaded it)
	if objStmObjects, ok := r.objStmCache[objStmNum]; ok {
		if obj, ok := objStmObjects[objectNum]; ok {
			return obj, nil
		}
		return nil, fmt.Errorf("object %d not found in ObjStm %d at index %d", objectNum, objStmNum, objIndex)
	}

	// Load the ObjStm object (it must be in-use, not compressed itself)
	objStmEntry, ok := r.xrefTable.GetEntry(objStmNum)
	if !ok {
		return nil, fmt.Errorf("ObjStm %d not found in xref table", objStmNum)
	}
	if objStmEntry.Type != XRefEntryInUse {
		return nil, fmt.Errorf("ObjStm %d is not in-use (type: %s)", objStmNum, objStmEntry.Type)
	}

	// Seek to ObjStm (adjust for any leading whitespace before %PDF- header)
	r.fileMu.Lock()
	adjustedOffset := r.adjustOffset(objStmEntry.Offset)
	if _, err := r.file.Seek(adjustedOffset, io.SeekStart); err != nil {
		r.fileMu.Unlock()
		return nil, fmt.Errorf("failed to seek to ObjStm %d: %w", objStmNum, err)
	}

	// Parse ObjStm indirect object
	parser := NewParser(r.file)
	indirectObj, err := parser.ParseIndirectObject()
	r.fileMu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to parse ObjStm %d: %w", objStmNum, err)
	}

	// Verify it's a stream
	stream, ok := indirectObj.Object.(*Stream)
	if !ok {
		return nil, fmt.Errorf("ObjStm %d is not a stream (got %T)", objStmNum, indirectObj.Object)
	}

	// Verify it's an Object Stream
	dict := stream.Dictionary()
	typeObj := dict.GetName("Type")
	if typeObj == nil || typeObj.Value() != "ObjStm" {
		return nil, fmt.Errorf("stream %d is not an ObjStm (Type: %v)", objStmNum, typeObj)
	}

	// Get /N (number of objects) and /First (offset to first object)
	numObjects := int(dict.GetInteger("N"))
	firstOffset := int(dict.GetInteger("First"))

	if numObjects <= 0 {
		return nil, fmt.Errorf("ObjStm %d has invalid /N: %d", objStmNum, numObjects)
	}
	if firstOffset < 0 {
		return nil, fmt.Errorf("ObjStm %d has invalid /First: %d", objStmNum, firstOffset)
	}

	// Decode the stream
	decodedData, err := r.decodeStream(stream)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ObjStm %d: %w", objStmNum, err)
	}

	// Parse the Object Stream
	objStmObjects, err := parser.ParseObjectStream(decodedData, numObjects, firstOffset)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ObjStm %d: %w", objStmNum, err)
	}

	// Cache the parsed objects
	r.objStmCache[objStmNum] = objStmObjects

	// Also cache each individual object in objectCache
	for objNum, obj := range objStmObjects {
		r.objectCache[objNum] = obj
	}

	// Return the requested object
	obj, ok := objStmObjects[objectNum]
	if !ok {
		return nil, fmt.Errorf("object %d not found in ObjStm %d (contains %d objects)", objectNum, objStmNum, len(objStmObjects))
	}

	return obj, nil
}

// createDCTDecoder creates a DCT decoder with parameters from the stream dictionary.
func (r *Reader) createDCTDecoder(dict *Dictionary) *encoding.DCTDecoder {
	// Check for decode parameters
	decodeParmsObj := dict.Get("DecodeParms")
	if decodeParmsObj == nil {
		// No parameters - use defaults
		return encoding.NewDCTDecoder()
	}

	// Extract ColorTransform parameter
	colorTransform := 1 // Default: YCbCr to RGB
	if parmsDict, ok := decodeParmsObj.(*Dictionary); ok {
		if ctObj := parmsDict.Get("ColorTransform"); ctObj != nil {
			if ctInt, ok := ctObj.(*Integer); ok {
				colorTransform = int(ctInt.Value())
			}
		}
	}

	return encoding.NewDCTDecoderWithParams(colorTransform)
}

// decodeStream decodes a stream object based on its filters.
func (r *Reader) decodeStream(stream *Stream) ([]byte, error) {
	dict := stream.Dictionary()
	filterObj := dict.Get("Filter")

	// No filter - return raw content
	if filterObj == nil {
		return stream.Content(), nil
	}

	// Extract filter name from Filter entry
	filterName := r.extractFilterName(filterObj)
	if filterName == "" {
		return stream.Content(), nil
	}

	// Apply the filter
	return r.applyFilter(filterName, dict, stream.Content())
}

// extractFilterName extracts the filter name from a Filter object.
func (r *Reader) extractFilterName(filterObj PdfObject) string {
	switch obj := filterObj.(type) {
	case *Name:
		return obj.Value()
	case *Array:
		// Multiple filters - for now, handle single filter case
		if obj.Len() > 0 {
			if nameObj, ok := obj.Get(0).(*Name); ok {
				return nameObj.Value()
			}
		}
	}
	return ""
}

// applyFilter applies the specified filter to stream content.
func (r *Reader) applyFilter(filterName string, dict *Dictionary, content []byte) ([]byte, error) {
	switch filterName {
	case filterFlateDecode:
		decoder := encoding.NewFlateDecoder()
		decoded, err := decoder.Decode(content)
		if err != nil {
			return nil, fmt.Errorf("%s failed: %w", filterFlateDecode, err)
		}
		return decoded, nil

	case filterDCTDecode:
		decoder := r.createDCTDecoder(dict)
		decoded, err := decoder.Decode(content)
		if err != nil {
			return nil, fmt.Errorf("DCTDecode failed: %w", err)
		}
		return decoded, nil

	default:
		return nil, fmt.Errorf("unsupported filter: %s", filterName)
	}
}

// resolveReferences recursively resolves indirect references.
//
// PDF objects can contain indirect references (e.g., "1 0 R") that
// point to other objects. This method follows these references and
// replaces them with the actual objects.
//
// For arrays and dictionaries, all nested references are resolved.
//
// Circular references are not currently detected (Phase 2.4).
// This will be addressed in a future phase if needed.
func (r *Reader) resolveReferences(obj PdfObject) PdfObject {
	switch o := obj.(type) {
	case *IndirectReference:
		// Resolve the reference
		resolved, err := r.GetObject(o.Number)
		if err != nil {
			// If resolution fails, return the unresolved reference
			// This allows the caller to handle the error
			return o
		}
		return resolved

	case *Array:
		// Resolve all array elements
		for i := 0; i < o.Len(); i++ {
			elem := o.Get(i)
			if elem != nil {
				resolved := r.resolveReferences(elem)
				_ = o.Set(i, resolved)
			}
		}
		return o

	case *Dictionary:
		// Resolve all dictionary values
		for _, key := range o.Keys() {
			value := o.Get(key)
			if value != nil {
				resolved := r.resolveReferences(value)
				o.Set(key, resolved)
			}
		}
		return o

	default:
		// Direct objects are returned as-is
		return obj
	}
}

// resolveDictionary is a helper that resolves an object and ensures it's a dictionary.
func (r *Reader) resolveDictionary(obj PdfObject) (*Dictionary, error) {
	// If it's an indirect reference, resolve it
	if ref, ok := obj.(*IndirectReference); ok {
		resolved, err := r.GetObject(ref.Number)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve reference %d %d R: %w",
				ref.Number, ref.Generation, err)
		}
		obj = resolved
	}

	// Ensure it's a dictionary
	dict, ok := obj.(*Dictionary)
	if !ok {
		return nil, fmt.Errorf("expected dictionary, got %T", obj)
	}

	return dict, nil
}

// GetCatalog returns the document catalog (root object).
//
// The catalog must be loaded via Open() before calling this method.
//
// Reference: PDF 1.7 specification, Section 7.7.2 (Document Catalog).
func (r *Reader) GetCatalog() (*Dictionary, error) {
	if r.catalog == nil {
		return nil, fmt.Errorf("catalog not loaded (call Open first)")
	}
	return r.catalog, nil
}

// GetAcroForm returns the interactive form dictionary (AcroForm).
//
// Returns nil if the document has no interactive form.
// The AcroForm dictionary contains form field definitions and settings.
//
// Reference: PDF 1.7 specification, Section 12.7 (Interactive Forms).
func (r *Reader) GetAcroForm() (*Dictionary, error) {
	if r.catalog == nil {
		return nil, fmt.Errorf("catalog not loaded (call Open first)")
	}

	acroFormObj := r.catalog.Get("AcroForm")
	if acroFormObj == nil {
		return nil, nil // No interactive form
	}

	// Resolve if indirect reference
	acroFormObj = r.resolveReferences(acroFormObj)

	dict, ok := acroFormObj.(*Dictionary)
	if !ok {
		return nil, fmt.Errorf("AcroForm is not a dictionary")
	}

	return dict, nil
}

// GetPages returns the page tree root.
//
// The page tree is a hierarchical structure containing all pages.
//
// Reference: PDF 1.7 specification, Section 7.7.3 (Page Tree).
func (r *Reader) GetPages() (*Dictionary, error) {
	if r.pages == nil {
		return nil, fmt.Errorf("pages not loaded (call Open first)")
	}
	return r.pages, nil
}

// GetPageCount returns the total number of pages in the document.
//
// The count is read from the /Count entry in the page tree root.
//
// Reference: PDF 1.7 specification, Section 7.7.3.2 (Page Tree Nodes).
func (r *Reader) GetPageCount() (int, error) {
	if r.pages == nil {
		return 0, fmt.Errorf("pages not loaded (call Open first)")
	}

	count := r.pages.GetInteger("Count")
	if count <= 0 {
		return 0, fmt.Errorf("invalid page count: %d", count)
	}

	return int(count), nil
}

// GetPage returns the page dictionary for the specified page number.
//
// Page numbers are 0-based (first page is 0).
//
// The method traverses the page tree to find the requested page.
// The page tree can have intermediate nodes (/Type /Pages) and
// leaf nodes (/Type /Page).
//
// Reference: PDF 1.7 specification, Section 7.7.3 (Page Tree).
func (r *Reader) GetPage(pageNum int) (*Dictionary, error) {
	if r.pages == nil {
		return nil, fmt.Errorf("pages not loaded (call Open first)")
	}

	if pageNum < 0 {
		return nil, fmt.Errorf("invalid page number: %d (must be >= 0)", pageNum)
	}

	// Traverse page tree
	page, err := r.getPageFromNode(r.pages, &pageNum)
	if err != nil {
		return nil, err
	}

	if page == nil {
		return nil, fmt.Errorf("page %d not found (page count: %d)", pageNum, r.pages.GetInteger("Count"))
	}

	return page, nil
}

// getPageFromNode recursively traverses the page tree to find a page.
//
// The pageNum pointer is decremented as we traverse leaf pages,
// so when it reaches 0, we've found the target page.
//
// Page tree structure:
//   - Intermediate nodes: /Type /Pages, /Kids [array of child nodes], /Count total
//   - Leaf nodes: /Type /Page
//
// Reference: PDF 1.7 specification, Section 7.7.3.2 (Page Tree Nodes).
func (r *Reader) getPageFromNode(node *Dictionary, pageNum *int) (*Dictionary, error) {
	typeObj := node.GetName("Type")
	if typeObj == nil {
		return nil, fmt.Errorf("page tree node missing /Type entry")
	}

	nodeType := typeObj.Value()

	if nodeType == nodeTypePage {
		// Leaf node - this is a page
		if *pageNum == 0 {
			return node, nil
		}
		*pageNum--
		return nil, nil
	}

	if nodeType == nodeTypePages {
		// Intermediate node - traverse kids
		kidsObj := node.Get("Kids")
		if kidsObj == nil {
			return nil, fmt.Errorf("pages node missing /Kids entry")
		}

		// Resolve kids array
		kids, err := r.resolveArray(kidsObj)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve /Kids array: %w", err)
		}

		// Traverse each kid
		for i := 0; i < kids.Len(); i++ {
			kidObj := kids.Get(i)
			if kidObj == nil {
				continue
			}

			// Resolve kid dictionary
			kid, err := r.resolveDictionary(kidObj)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve kid %d: %w", i, err)
			}

			// Recursively search this subtree
			page, err := r.getPageFromNode(kid, pageNum)
			if err != nil {
				return nil, err
			}

			if page != nil {
				return page, nil
			}

			// If pageNum didn't change or became negative, something is wrong
			if *pageNum < 0 {
				return nil, fmt.Errorf("page index exceeded page count")
			}
		}

		// If we've exhausted all kids and haven't found the page, return nil
		// This allows parent node to continue searching in other subtrees
		return nil, nil
	}

	return nil, fmt.Errorf("unknown page tree node type: %s", nodeType)
}

// resolveArray is a helper that resolves an object and ensures it's an array.
func (r *Reader) resolveArray(obj PdfObject) (*Array, error) {
	// If it's an indirect reference, resolve it
	if ref, ok := obj.(*IndirectReference); ok {
		resolved, err := r.GetObject(ref.Number)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve reference %d %d R: %w",
				ref.Number, ref.Generation, err)
		}
		obj = resolved
	}

	// Ensure it's an array
	arr, ok := obj.(*Array)
	if !ok {
		return nil, fmt.Errorf("expected array, got %T", obj)
	}

	return arr, nil
}

// ResolveArray resolves an object and ensures it's an array.
// This is the exported version of resolveArray.
func (r *Reader) ResolveArray(obj PdfObject) (*Array, error) {
	return r.resolveArray(obj)
}

// ResolveReferences recursively resolves indirect references in an object.
// This is the exported version of resolveReferences.
func (r *Reader) ResolveReferences(obj PdfObject) PdfObject {
	return r.resolveReferences(obj)
}

// Version returns the PDF version string from the file header.
//
// Returns empty string if Open() has not been called.
//
// Reference: PDF 1.7 specification, Section 7.5.1 (File Header).
func (r *Reader) Version() string {
	return r.version
}

// Trailer returns the trailer dictionary.
//
// The trailer contains document-level metadata like:
//   - /Size: Number of entries in xref table
//   - /Root: Reference to catalog
//   - /Info: Document information dictionary
//   - /ID: File identifier array
//
// Reference: PDF 1.7 specification, Section 7.5.5 (File Trailer).
func (r *Reader) Trailer() *Dictionary {
	return r.trailer
}

// XRefTable returns the cross-reference table.
//
// The xref table maps object numbers to byte offsets in the file.
//
// Reference: PDF 1.7 specification, Section 7.5.4 (Cross-Reference Table).
func (r *Reader) XRefTable() *XRefTable {
	return r.xrefTable
}

// DocInfo contains document metadata from the Info dictionary.
type DocInfo struct {
	Version   string
	Title     string
	Author    string
	Subject   string
	Keywords  string
	Creator   string
	Producer  string
	Encrypted bool
}

// GetDocumentInfo returns document metadata from the Info dictionary.
//
// Reference: PDF 1.7 specification, Section 14.3.3 (Document Information Dictionary).
func (r *Reader) GetDocumentInfo() DocInfo {
	info := DocInfo{
		Version: r.version,
	}

	// Check if document is encrypted
	if r.trailer != nil {
		if r.trailer.Get("Encrypt") != nil {
			info.Encrypted = true
		}
	}

	// Get Info dictionary from trailer
	if r.trailer == nil {
		return info
	}

	infoRef := r.trailer.Get("Info")
	if infoRef == nil {
		return info
	}

	// Resolve indirect reference
	infoDict := r.resolveReferences(infoRef)
	dict, ok := infoDict.(*Dictionary)
	if !ok {
		return info
	}

	// Extract string fields using GetString helper
	info.Title = dict.GetString("Title")
	info.Author = dict.GetString("Author")
	info.Subject = dict.GetString("Subject")
	info.Keywords = dict.GetString("Keywords")
	info.Creator = dict.GetString("Creator")
	info.Producer = dict.GetString("Producer")

	return info
}

// OpenPDF is a convenience function that creates a Reader and opens the PDF.
//
// This is equivalent to:
//
//	reader := NewReader(filename)
//	err := reader.Open()
//
// Remember to call Close() when done:
//
//	defer reader.Close()
func OpenPDF(filename string) (*Reader, error) {
	reader := NewReader(filename)
	if err := reader.Open(); err != nil {
		return nil, err
	}
	return reader, nil
}

// ReadPDFInfo is a convenience function that reads basic PDF information
// without loading the entire document structure.
//
// Returns: version, page count, error.
//
// This is useful for quickly checking PDF properties without
// loading all objects into memory.
func ReadPDFInfo(filename string) (version string, pageCount int, err error) {
	reader := NewReader(filename)
	if err := reader.Open(); err != nil {
		return "", 0, err
	}
	defer func() { _ = reader.Close() }()

	count, err := reader.GetPageCount()
	if err != nil {
		return reader.Version(), 0, err
	}

	return reader.Version(), count, nil
}

// String returns a string representation of the reader's state.
func (r *Reader) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "PDFReader{")
	fmt.Fprintf(&buf, "file=%q, ", r.filename)
	fmt.Fprintf(&buf, "version=%q, ", r.version)

	if r.xrefTable != nil {
		fmt.Fprintf(&buf, "objects=%d, ", r.xrefTable.Size())
	}

	if r.pages != nil {
		count, _ := r.GetPageCount()
		fmt.Fprintf(&buf, "pages=%d, ", count)
	}

	fmt.Fprintf(&buf, "cached=%d", len(r.objectCache))
	fmt.Fprintf(&buf, "}")

	return buf.String()
}
