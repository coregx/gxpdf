// Package parser implements PDF cross-reference table parsing according to
// PDF 1.7 specification, Section 7.5.4-7.5.8 (File Structure).
package parser

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"strconv"
)

// XRefEntryType represents the type of cross-reference entry.
type XRefEntryType int

const (
	// XRefEntryFree represents a free (deleted) object entry.
	// Format: next_free_object_num generation f.
	XRefEntryFree XRefEntryType = iota

	// XRefEntryInUse represents an in-use object entry.
	// Format: byte_offset generation n.
	XRefEntryInUse

	// XRefEntryCompressed represents a compressed object entry (PDF 1.5+).
	// Found in XRef streams, format varies based on stream /W array.
	XRefEntryCompressed
)

// String returns the string representation of the XRefEntryType.
func (t XRefEntryType) String() string {
	switch t {
	case XRefEntryFree:
		return "free"
	case XRefEntryInUse:
		return "in-use"
	case XRefEntryCompressed:
		return "compressed"
	default:
		return "unknown"
	}
}

// XRefEntry represents a single entry in the cross-reference table.
//
// For in-use entries (type 'n'):
//   - Offset: byte offset in file where object starts
//   - Generation: generation number of the object
//
// For free entries (type 'f'):
//   - Offset: object number of next free object (or 0 if last)
//   - Generation: generation number to use when object is reused
//
// Reference: PDF 1.7 specification, Section 7.5.4 (Cross-Reference Table).
type XRefEntry struct {
	Type       XRefEntryType // Entry type (free, in-use, compressed)
	Offset     int64         // Byte offset (in-use) or next free object (free)
	Generation int           // Generation number
	ObjectNum  int           // Object number (for convenience)
}

// NewXRefEntry creates a new cross-reference entry.
func NewXRefEntry(objectNum int, entryType XRefEntryType, offset int64, generation int) *XRefEntry {
	return &XRefEntry{
		Type:       entryType,
		Offset:     offset,
		Generation: generation,
		ObjectNum:  objectNum,
	}
}

// String returns a string representation of the entry.
func (e *XRefEntry) String() string {
	typeChar := "n"
	if e.Type == XRefEntryFree {
		typeChar = "f"
	}
	return fmt.Sprintf("%010d %05d %s", e.Offset, e.Generation, typeChar)
}

// IsFree returns true if this entry represents a free (deleted) object.
func (e *XRefEntry) IsFree() bool {
	return e.Type == XRefEntryFree
}

// IsInUse returns true if this entry represents an in-use object.
func (e *XRefEntry) IsInUse() bool {
	return e.Type == XRefEntryInUse
}

// XRefTable represents a PDF cross-reference table.
//
// The cross-reference table contains information about the location of
// objects in the PDF file. Each object is identified by an object number
// and generation number.
//
// Reference: PDF 1.7 specification, Section 7.5.4 (Cross-Reference Table).
type XRefTable struct {
	Entries map[int]*XRefEntry // Map: object number -> XRef entry
	Trailer *Dictionary        // Trailer dictionary
}

// NewXRefTable creates a new empty cross-reference table.
func NewXRefTable() *XRefTable {
	return &XRefTable{
		Entries: make(map[int]*XRefEntry),
		Trailer: NewDictionary(),
	}
}

// AddEntry adds an entry to the cross-reference table.
func (t *XRefTable) AddEntry(entry *XRefEntry) {
	if entry != nil {
		t.Entries[entry.ObjectNum] = entry
	}
}

// GetEntry retrieves an entry by object number.
// Returns nil if the entry doesn't exist.
func (t *XRefTable) GetEntry(objectNum int) (*XRefEntry, bool) {
	entry, exists := t.Entries[objectNum]
	return entry, exists
}

// Size returns the number of entries in the table.
func (t *XRefTable) Size() int {
	return len(t.Entries)
}

// HasObject returns true if the table contains an entry for the given object number.
func (t *XRefTable) HasObject(objectNum int) bool {
	_, exists := t.Entries[objectNum]
	return exists
}

// GetInUseEntries returns all in-use entries in the table.
func (t *XRefTable) GetInUseEntries() []*XRefEntry {
	var entries []*XRefEntry
	for _, entry := range t.Entries {
		if entry.IsInUse() {
			entries = append(entries, entry)
		}
	}
	return entries
}

// GetFreeEntries returns all free entries in the table.
func (t *XRefTable) GetFreeEntries() []*XRefEntry {
	var entries []*XRefEntry
	for _, entry := range t.Entries {
		if entry.IsFree() {
			entries = append(entries, entry)
		}
	}
	return entries
}

// SetTrailer sets the trailer dictionary.
func (t *XRefTable) SetTrailer(trailer *Dictionary) {
	if trailer != nil {
		t.Trailer = trailer
	}
}

// GetTrailer returns the trailer dictionary.
func (t *XRefTable) GetTrailer() *Dictionary {
	return t.Trailer
}

// MergeOlder merges entries from an older cross-reference table.
//
// Entries already present in this table (newer) are preserved.
// Only entries missing from this table are added from the older table.
// This implements the PDF incremental update semantics where newer
// xref sections take precedence over older ones.
//
// Reference: PDF 1.7 specification, Section 7.5.6 (Incremental Updates).
func (t *XRefTable) MergeOlder(older *XRefTable) {
	if older == nil {
		return
	}
	for objNum, entry := range older.Entries {
		if _, exists := t.Entries[objNum]; !exists {
			t.Entries[objNum] = entry
		}
	}
}

// String returns a string representation of the XRef table.
func (t *XRefTable) String() string {
	return fmt.Sprintf("XRefTable{entries: %d, trailer: %v}", t.Size(), t.Trailer)
}

// ParseXRef parses a cross-reference table and trailer.
//
// Handles both traditional xref tables (PDF < 1.5) and xref streams (PDF 1.5+):
//
// Traditional format:
//
//	xref
//	startNum count
//	offset1 generation1 type1
//	offset2 generation2 type2
//	...
//	trailer
//	<< trailer dictionary >>
//
// XRef stream format (PDF 1.5+):
//
//	90 0 obj
//	<< /Type /XRef /Size 100 /W [1 3 2] ... >>
//	stream
//	...compressed xref data...
//	endstream
//	endobj
//
// Reference: PDF 1.7 specification, Section 7.5.4 and 7.5.8.
func (p *Parser) ParseXRef() (*XRefTable, error) {
	// Check if this is a traditional xref table or xref stream
	if p.current.Type == TokenInteger {
		// PDF 1.5+ xref stream: starts with object number
		// Example: "90 0 obj" instead of "xref"
		return p.ParseXRefStream()
	}

	// Traditional xref table: expect 'xref' keyword
	if p.current.Type != TokenKeyword || p.current.Value != "xref" {
		return nil, fmt.Errorf("expected 'xref' keyword or object number (for xref stream), got %s(%q) at %d:%d",
			p.current.Type, p.current.Value, p.current.Line, p.current.Column)
	}
	_ = p.advance()

	table := NewXRefTable()

	// Parse subsections (can have multiple subsections)
	if err := p.parseXRefSubsections(table); err != nil {
		return nil, err
	}

	// Parse trailer
	if err := p.parseXRefTrailer(table); err != nil {
		return nil, err
	}

	return table, nil
}

// parseXRefSubsections parses all cross-reference subsections.
// Each subsection starts with: startNum count.
func (p *Parser) parseXRefSubsections(table *XRefTable) error {
	for p.current.Type == TokenInteger {
		// Read start object number
		startNum, err := strconv.Atoi(p.current.Value)
		if err != nil {
			return fmt.Errorf("invalid start object number %q at %d:%d: %w",
				p.current.Value, p.current.Line, p.current.Column, err)
		}
		_ = p.advance()

		// Read count
		if p.current.Type != TokenInteger {
			return fmt.Errorf("expected count after start number, got %s at %d:%d",
				p.current.Type, p.current.Line, p.current.Column)
		}
		count, err := strconv.Atoi(p.current.Value)
		if err != nil {
			return fmt.Errorf("invalid count %q at %d:%d: %w",
				p.current.Value, p.current.Line, p.current.Column, err)
		}
		_ = p.advance()

		// Parse entries for this subsection
		for i := 0; i < count; i++ {
			entry, err := p.parseXRefEntry(startNum + i)
			if err != nil {
				return fmt.Errorf("failed to parse xref entry %d: %w", startNum+i, err)
			}
			table.AddEntry(entry)
		}
	}
	return nil
}

// parseXRefTrailer parses the trailer dictionary.
func (p *Parser) parseXRefTrailer(table *XRefTable) error {
	// Expect 'trailer' keyword
	if p.current.Type != TokenKeyword || p.current.Value != "trailer" {
		return fmt.Errorf("expected 'trailer' keyword, got %s(%q) at %d:%d",
			p.current.Type, p.current.Value, p.current.Line, p.current.Column)
	}
	_ = p.advance()

	// Parse trailer dictionary
	trailer, err := p.parseDictionary()
	if err != nil {
		return fmt.Errorf("failed to parse trailer dictionary: %w", err)
	}
	table.SetTrailer(trailer)

	return nil
}

// parseXRefEntry parses a single cross-reference table entry.
//
// Expected format (fixed-width fields):
//
//	nnnnnnnnnn ggggg f
//	nnnnnnnnnn ggggg n
//
// Where:
//   - nnnnnnnnnn: 10-digit offset (in-use) or next free object number (free)
//   - ggggg: 5-digit generation number
//   - f/n: entry type ('f' = free, 'n' = in-use)
//
// Reference: PDF 1.7 specification, Section 7.5.4 (Cross-Reference Table).
func (p *Parser) parseXRefEntry(objectNum int) (*XRefEntry, error) {
	// Expect integer (offset or next free object number)
	if p.current.Type != TokenInteger {
		return nil, fmt.Errorf("expected offset/next for xref entry, got %s at %d:%d",
			p.current.Type, p.current.Line, p.current.Column)
	}
	offset, err := strconv.ParseInt(p.current.Value, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid offset %q at %d:%d: %w",
			p.current.Value, p.current.Line, p.current.Column, err)
	}
	_ = p.advance()

	// Expect generation number
	if p.current.Type != TokenInteger {
		return nil, fmt.Errorf("expected generation for xref entry, got %s at %d:%d",
			p.current.Type, p.current.Line, p.current.Column)
	}
	generation, err := strconv.Atoi(p.current.Value)
	if err != nil {
		return nil, fmt.Errorf("invalid generation %q at %d:%d: %w",
			p.current.Value, p.current.Line, p.current.Column, err)
	}
	_ = p.advance()

	// Expect type character ('n' or 'f')
	// The lexer will parse this as a keyword since it's a regular character
	if p.current.Type != TokenKeyword {
		return nil, fmt.Errorf("expected entry type ('n' or 'f'), got %s at %d:%d",
			p.current.Type, p.current.Line, p.current.Column)
	}

	var entryType XRefEntryType
	switch p.current.Value {
	case "n":
		entryType = XRefEntryInUse
	case "f":
		entryType = XRefEntryFree
	default:
		return nil, fmt.Errorf("invalid xref entry type %q at %d:%d (expected 'n' or 'f')",
			p.current.Value, p.current.Line, p.current.Column)
	}
	_ = p.advance()

	return NewXRefEntry(objectNum, entryType, offset, generation), nil
}

// ParseStartXRef parses the startxref section at the end of a PDF file.
//
// Expected format:
//
//	startxref
//	byte_offset
//	%%EOF
//
// Returns the byte offset of the cross-reference table.
//
// Reference: PDF 1.7 specification, Section 7.5.5 (File Trailer).
func (p *Parser) ParseStartXRef() (int64, error) {
	// Expect 'startxref' keyword
	if p.current.Type != TokenKeyword || p.current.Value != "startxref" {
		return 0, fmt.Errorf("expected 'startxref' keyword, got %s(%q) at %d:%d",
			p.current.Type, p.current.Value, p.current.Line, p.current.Column)
	}
	_ = p.advance()

	// Expect integer offset
	if p.current.Type != TokenInteger {
		return 0, fmt.Errorf("expected integer offset after 'startxref', got %s at %d:%d",
			p.current.Type, p.current.Line, p.current.Column)
	}

	offset, err := strconv.ParseInt(p.current.Value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid startxref offset %q at %d:%d: %w",
			p.current.Value, p.current.Line, p.current.Column, err)
	}
	_ = p.advance()

	return offset, nil
}

// XRefStream represents a compressed cross-reference stream (PDF 1.5+).
//
// Cross-reference streams provide a more compact alternative to traditional
// cross-reference tables by using stream compression.
//
// Note: Full XRef stream support requires stream decoding (compression),
// which will be implemented in a later phase.
//
// Reference: PDF 1.7 specification, Section 7.5.8 (Cross-Reference Streams).
type XRefStream struct {
	Stream  *Stream      // The stream object containing compressed xref data
	Entries []*XRefEntry // Parsed entries (if decoded)
	W       []int        // Field widths from /W array in stream dictionary
	Index   []int        // Object number ranges from /Index array
}

// NewXRefStream creates a new XRef stream structure.
func NewXRefStream(stream *Stream) *XRefStream {
	return &XRefStream{
		Stream:  stream,
		Entries: make([]*XRefEntry, 0),
		W:       make([]int, 0),
		Index:   make([]int, 0),
	}
}

// ParseXRefStreamWithFileAccess parses a cross-reference stream with direct file access.
//
// This version is used when we have access to an io.ReadSeeker (file handle)
// and can seek to the exact stream data position, avoiding lexer buffer issues.
func (p *Parser) ParseXRefStreamWithFileAccess(file io.ReadSeeker, xrefOffset int64) (*XRefTable, error) {
	// Parse object header: "90 0 obj"
	if p.current.Type != TokenInteger {
		return nil, fmt.Errorf("expected object number for xref stream, got %s", p.current.Type)
	}
	_ = p.advance()

	if p.current.Type != TokenInteger {
		return nil, fmt.Errorf("expected generation number for xref stream, got %s", p.current.Type)
	}
	_ = p.advance()

	if p.current.Type != TokenKeyword || p.current.Value != "obj" {
		return nil, fmt.Errorf("expected 'obj' keyword, got %s", p.current.Type)
	}
	_ = p.advance()

	// Parse stream dictionary
	if p.current.Type != TokenDictStart {
		return nil, fmt.Errorf("expected dictionary for xref stream, got %s", p.current.Type)
	}
	dict, err := p.parseDictionary()
	if err != nil {
		return nil, fmt.Errorf("failed to parse xref stream dictionary: %w", err)
	}

	// Verify it's an XRef stream
	typeObj := dict.GetName("Type")
	if typeObj == nil || typeObj.Value() != "XRef" {
		return nil, fmt.Errorf("stream is not an XRef stream")
	}

	// Expect 'stream' keyword
	if p.current.Type != TokenKeyword || p.current.Value != "stream" {
		return nil, fmt.Errorf("expected 'stream' keyword, got %s", p.current.Type)
	}

	// Get stream length
	length := dict.GetInteger("Length")
	if length <= 0 {
		return nil, fmt.Errorf("invalid or missing stream /Length")
	}

	// Find 'stream' keyword in the file by reading from xref offset
	// Seek to xref start
	if _, err := file.Seek(xrefOffset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to xref offset: %w", err)
	}

	// Read enough data to find 'stream' keyword (read up to 1KB)
	searchBuf := make([]byte, 1024)
	n, err := file.Read(searchBuf)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read for stream search: %w", err)
	}

	// Find "stream" in the buffer
	streamIdx := bytes.Index(searchBuf[:n], []byte("stream"))
	if streamIdx == -1 {
		return nil, fmt.Errorf("could not find 'stream' keyword in xref object")
	}

	// Calculate absolute position of stream data
	// streamIdx is relative to xrefOffset
	streamKeywordPos := xrefOffset + int64(streamIdx)
	streamDataStart := streamKeywordPos + 6 // After "stream" (6 bytes)

	// Check for newline after 'stream' and skip it
	if streamIdx+6 < n {
		if searchBuf[streamIdx+6] == '\r' {
			streamDataStart++ // Skip \r
			if streamIdx+7 < n && searchBuf[streamIdx+7] == '\n' {
				streamDataStart++ // Skip \n in \r\n
			}
		} else if searchBuf[streamIdx+6] == '\n' {
			streamDataStart++ // Skip \n
		}
	}

	// Seek to stream data start
	if _, err := file.Seek(streamDataStart, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to stream data: %w", err)
	}

	// Read stream data
	streamData := make([]byte, length)
	if _, err := io.ReadFull(file, streamData); err != nil {
		return nil, fmt.Errorf("failed to read stream data: %w", err)
	}

	// Decode the stream based on filter
	filterObj := dict.Get("Filter")
	var decodedData []byte

	if filterObj != nil {
		var filterName string
		if nameObj, ok := filterObj.(*Name); ok {
			filterName = nameObj.Value()
		}

		if filterName == filterFlateDecode {
			decoder := &flateDecoder{}
			decodedData, err = decoder.Decode(streamData)
			if err != nil {
				return nil, fmt.Errorf("failed to decode %s stream: %w", filterFlateDecode, err)
			}
		} else {
			return nil, fmt.Errorf("unsupported xref stream filter: %s", filterName)
		}
	} else {
		// No filter, use data as-is
		decodedData = streamData
	}

	// Parse binary xref entries (using Parser method)
	table, err := p.parseXRefStreamEntries(dict, decodedData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse xref entries: %w", err)
	}

	// Use dictionary as trailer
	table.SetTrailer(dict)

	return table, nil
}

// ParseXRefStream parses a cross-reference stream object (PDF 1.5+).
//
// When a PDF uses xref streams, the startxref pointer points to an
// indirect object (e.g., "90 0 obj") instead of the "xref" keyword.
//
// The stream dictionary contains:
//   - /Type /XRef
//   - /Size: number of entries
//   - /W [w1 w2 w3]: field widths for parsing binary data
//   - /Index: optional array of [start count ...] pairs (default [0 Size])
//   - Trailer entries: /Root, /Info, /ID, etc.
//
// Reference: PDF 1.7 specification, Section 7.5.8 (Cross-Reference Streams).
func (p *Parser) ParseXRefStream() (*XRefTable, error) {
	// We're positioned right after detecting an INTEGER instead of 'xref'
	// The current token is the object number

	// Parse object number
	if p.current.Type != TokenInteger {
		return nil, fmt.Errorf("expected object number for xref stream, got %s at %d:%d",
			p.current.Type, p.current.Line, p.current.Column)
	}
	_, err := strconv.Atoi(p.current.Value) // objectNum (not needed for xref parsing)
	if err != nil {
		return nil, fmt.Errorf("invalid object number %q: %w", p.current.Value, err)
	}
	_ = p.advance()

	// Parse generation number
	if p.current.Type != TokenInteger {
		return nil, fmt.Errorf("expected generation number for xref stream, got %s at %d:%d",
			p.current.Type, p.current.Line, p.current.Column)
	}
	_, err = strconv.Atoi(p.current.Value) // generation (not needed for xref parsing)
	if err != nil {
		return nil, fmt.Errorf("invalid generation number %q: %w", p.current.Value, err)
	}
	_ = p.advance()

	// Expect 'obj' keyword
	if p.current.Type != TokenKeyword || p.current.Value != "obj" {
		return nil, fmt.Errorf("expected 'obj' keyword, got %s(%q) at %d:%d",
			p.current.Type, p.current.Value, p.current.Line, p.current.Column)
	}
	_ = p.advance()

	// Parse stream dictionary
	if p.current.Type != TokenDictStart {
		return nil, fmt.Errorf("expected dictionary for xref stream, got %s at %d:%d",
			p.current.Type, p.current.Line, p.current.Column)
	}
	dict, err := p.parseDictionary()
	if err != nil {
		return nil, fmt.Errorf("failed to parse xref stream dictionary: %w", err)
	}

	// Verify it's an XRef stream
	typeObj := dict.GetName("Type")
	if typeObj == nil || typeObj.Value() != "XRef" {
		return nil, fmt.Errorf("stream is not an XRef stream (Type: %v)", typeObj)
	}

	// Parse stream data
	stream, err := p.parseStreamData(dict)
	if err != nil {
		return nil, fmt.Errorf("failed to parse xref stream data: %w", err)
	}

	// Decode the stream (typically FlateDecode)
	decodedData, err := p.decodeXRefStream(dict, stream)
	if err != nil {
		return nil, fmt.Errorf("failed to decode xref stream: %w", err)
	}

	// Parse binary xref entries
	table, err := p.parseXRefStreamEntries(dict, decodedData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse xref stream entries: %w", err)
	}

	// Use the stream dictionary as trailer (it contains /Root, /Info, /Size, etc.)
	table.SetTrailer(dict)

	return table, nil
}

// parseStreamData parses the stream content between 'stream' and 'endstream'.
func (p *Parser) parseStreamData(dict *Dictionary) ([]byte, error) {
	// Expect 'stream' keyword
	if p.current.Type != TokenKeyword || p.current.Value != "stream" {
		return nil, fmt.Errorf("expected 'stream' keyword, got %s(%q) at %d:%d",
			p.current.Type, p.current.Value, p.current.Line, p.current.Column)
	}

	// Get stream length from dictionary
	lengthObj := dict.Get("Length")
	if lengthObj == nil {
		return nil, fmt.Errorf("stream dictionary missing /Length entry")
	}

	var length int64
	switch obj := lengthObj.(type) {
	case *Integer:
		length = obj.Value()
	case *IndirectReference:
		// TODO: Resolve indirect length reference
		return nil, fmt.Errorf("indirect /Length references not yet supported")
	default:
		return nil, fmt.Errorf("invalid /Length type: %T", lengthObj)
	}

	if length < 0 {
		return nil, fmt.Errorf("invalid stream length: %d", length)
	}

	// After 'stream' keyword, we need to skip exactly the EOL marker
	// PDF spec allows: \n, \r\n, or \r
	// The lexer has already consumed 'stream', now we need to skip the newline(s)

	// Read one byte (should be \r or \n)
	buf := make([]byte, 1)
	n, err := p.lexer.reader.Read(buf)
	if err != nil || n == 0 {
		return nil, fmt.Errorf("failed to read after 'stream' keyword: %w", err)
	}

	// If \r, check if next is \n (for \r\n)
	if buf[0] == '\r' {
		// Peek at next byte
		n, err = p.lexer.reader.Read(buf)
		if err == nil && n > 0 && buf[0] == '\n' {
			// Found \r\n, both consumed
		} else {
			// Just \r, which is okay (put back the byte we read if it's not \n)
			// For simplicity in this implementation, we'll accept it
			// In a production parser, we'd use a buffered reader with UnreadByte
		}
	} else if buf[0] == '\n' {
		// Found \n, consumed
	} else {
		return nil, fmt.Errorf("expected newline after 'stream', got byte %02x", buf[0])
	}

	// Read stream data
	data := make([]byte, length)
	n, err = io.ReadFull(p.lexer.reader, data)
	if err != nil {
		return nil, fmt.Errorf("failed to read stream data (length=%d): %w", length, err)
	}
	if int64(n) != length {
		return nil, fmt.Errorf("stream length mismatch: expected %d bytes, got %d", length, n)
	}

	// Now we should skip forward to find 'endstream'
	// Read until we find 'endstream' keyword
	// For now, we'll advance the parser
	_ = p.advance()

	// Try to find endstream
	for i := 0; i < 10; i++ { // Try a few times
		if p.current.Type == TokenKeyword && p.current.Value == "endstream" {
			_ = p.advance()
			return data, nil
		}
		_ = p.advance()
	}

	return data, nil // Return data even if we didn't find endstream (tolerant parsing)
}

// decodeXRefStream decodes a compressed xref stream.
func (p *Parser) decodeXRefStream(dict *Dictionary, data []byte) ([]byte, error) {
	// Check for /Filter entry
	filterObj := dict.Get("Filter")
	if filterObj == nil {
		// No filter, data is uncompressed
		return data, nil
	}

	// Get filter name
	var filterName string
	switch obj := filterObj.(type) {
	case *Name:
		filterName = obj.Value()
	case *Array:
		// Multiple filters (apply in order)
		// For now, handle single filter case
		if obj.Len() > 0 {
			if nameObj, ok := obj.Get(0).(*Name); ok {
				filterName = nameObj.Value()
			}
		}
	}

	// Decode based on filter type
	switch filterName {
	case "FlateDecode":
		// Use embedded decoder to avoid import cycles
		decoder := &flateDecoder{}
		decoded, err := decoder.Decode(data)
		if err != nil {
			return nil, fmt.Errorf("failed to decode FlateDecode stream: %w", err)
		}
		return decoded, nil

	case "":
		// No filter
		return data, nil

	default:
		return nil, fmt.Errorf("unsupported xref stream filter: %s", filterName)
	}
}

// flateDecoder is a simple Flate decoder embedded here to avoid import cycles.
// This uses standard library compress/zlib.
type flateDecoder struct{}

func (d *flateDecoder) Decode(data []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create zlib reader: %w", err)
	}
	defer func() { _ = reader.Close() }()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}

	return buf.Bytes(), nil
}

// parseXRefStreamEntries parses binary xref entries from decoded stream data.
//
// The /W array specifies field widths: [type_bytes field2_bytes field3_bytes]
//   - type (field 1): 0 = free, 1 = in-use, 2 = compressed
//   - field 2: For type 1: byte offset. For type 2: object stream number.
//   - field 3: For type 1: generation. For type 2: index in object stream.
//
// The /Index array specifies object ranges: [start1 count1 start2 count2 ...]
// Default: [0 Size]
//
// Reference: PDF 1.7 specification, Section 7.5.8.2 and 7.5.8.3.
func (p *Parser) parseXRefStreamEntries(dict *Dictionary, data []byte) (*XRefTable, error) {
	// Get /W array (field widths)
	wObj := dict.Get("W")
	if wObj == nil {
		return nil, fmt.Errorf("xref stream missing /W array")
	}
	wArray, ok := wObj.(*Array)
	if !ok || wArray.Len() != 3 {
		return nil, fmt.Errorf("invalid /W array: must have 3 elements")
	}

	w1 := int(wArray.Get(0).(*Integer).Value())
	w2 := int(wArray.Get(1).(*Integer).Value())
	w3 := int(wArray.Get(2).(*Integer).Value())
	entrySize := w1 + w2 + w3

	if entrySize == 0 {
		return nil, fmt.Errorf("invalid /W array: entry size is 0")
	}

	// Get /Index array (object ranges) - default is [0 Size]
	var index []int
	indexObj := dict.Get("Index")
	if indexObj != nil {
		indexArray, ok := indexObj.(*Array)
		if ok {
			for i := 0; i < indexArray.Len(); i++ {
				index = append(index, int(indexArray.Get(i).(*Integer).Value()))
			}
		}
	} else {
		// Default: [0 Size]
		size := dict.GetInteger("Size")
		if size <= 0 {
			return nil, fmt.Errorf("xref stream missing or invalid /Size")
		}
		index = []int{0, int(size)}
	}

	// Parse entries
	table := NewXRefTable()
	offset := 0

	for i := 0; i < len(index); i += 2 {
		startNum := index[i]
		count := index[i+1]

		for j := 0; j < count; j++ {
			objectNum := startNum + j

			if offset+entrySize > len(data) {
				return nil, fmt.Errorf("xref stream data truncated at object %d", objectNum)
			}

			// Read entry fields
			entryData := data[offset : offset+entrySize]
			offset += entrySize

			// Parse fields according to widths
			var type_, field2, field3 int64

			pos := 0
			if w1 > 0 {
				type_ = readBigEndianInt(entryData[pos : pos+w1])
				pos += w1
			} else {
				type_ = 1 // Default type is 1 (in-use)
			}

			if w2 > 0 {
				field2 = readBigEndianInt(entryData[pos : pos+w2])
				pos += w2
			}

			if w3 > 0 {
				field3 = readBigEndianInt(entryData[pos : pos+w3])
			}

			// Create xref entry based on type
			var entry *XRefEntry
			switch type_ {
			case 0:
				// Free entry
				entry = NewXRefEntry(objectNum, XRefEntryFree, field2, int(field3))

			case 1:
				// In-use entry: field2 = byte offset, field3 = generation
				entry = NewXRefEntry(objectNum, XRefEntryInUse, field2, int(field3))

			case 2:
				// Compressed entry: field2 = object stream num, field3 = index
				entry = NewXRefEntry(objectNum, XRefEntryCompressed, field2, int(field3))

			default:
				return nil, fmt.Errorf("invalid xref entry type %d for object %d", type_, objectNum)
			}

			table.AddEntry(entry)
		}
	}

	return table, nil
}

// readBigEndianInt reads a big-endian integer from bytes.
func readBigEndianInt(data []byte) int64 {
	var result int64
	for _, b := range data {
		result = (result << 8) | int64(b)
	}
	return result
}
