package parser

import (
	"bytes"
	"context"
	"fmt"
	"io"
)

type streamObjectResolver func(context.Context, int) (PdfObject, error)

// Stream represents a PDF stream object.
// A stream consists of a dictionary followed by zero or more bytes bracketed
// between the keywords stream (followed by newline) and endstream.
//
// Reference: PDF 1.7 specification, Section 7.3.8 (Stream Objects).
type Stream struct {
	dict           *Dictionary // Stream dictionary
	content        []byte      // Raw or decoded stream data
	objectResolver streamObjectResolver
}

// NewStream creates a new Stream with the given dictionary and content.
func NewStream(dict *Dictionary, content []byte) *Stream {
	if dict == nil {
		dict = NewDictionary()
	}
	return &Stream{
		dict:    dict,
		content: content,
	}
}

// Dictionary returns the stream's dictionary.
func (s *Stream) Dictionary() *Dictionary {
	return s.dict
}

// Content returns the raw stream content.
func (s *Stream) Content() []byte {
	return s.content
}

// SetContent sets the stream content and updates the Length entry in the dictionary.
func (s *Stream) SetContent(content []byte) {
	s.content = content
	s.dict.SetInteger("Length", int64(len(content)))
}

// Length returns the length of the stream content.
func (s *Stream) Length() int64 {
	return int64(len(s.content))
}

// String returns a string representation of the stream.
// Only shows the dictionary and length, not the full content.
func (s *Stream) String() string {
	return fmt.Sprintf("stream[dict=%s, length=%d]", s.dict.String(), len(s.content))
}

// WriteTo writes the PDF representation of the stream to w.
// Format: dictionary\nstream\ncontent\nendstream.
func (s *Stream) WriteTo(w io.Writer) (int64, error) {
	var total int64

	// Ensure Length is set correctly
	s.dict.SetInteger("Length", int64(len(s.content)))

	// Write dictionary
	written, err := s.dict.WriteTo(w)
	total += written
	if err != nil {
		return total, err
	}

	// Write newline before stream keyword
	n, err := w.Write([]byte("\n"))
	total += int64(n)
	if err != nil {
		return total, err
	}

	// Write stream keyword
	n, err = w.Write([]byte("stream\n"))
	total += int64(n)
	if err != nil {
		return total, err
	}

	// Write content
	n, err = w.Write(s.content)
	total += int64(n)
	if err != nil {
		return total, err
	}

	// Write newline before endstream (if content doesn't end with one)
	if len(s.content) > 0 && s.content[len(s.content)-1] != '\n' {
		n, err = w.Write([]byte("\n"))
		total += int64(n)
		if err != nil {
			return total, err
		}
	}

	// Write endstream keyword
	n, err = w.Write([]byte("endstream"))
	total += int64(n)
	return total, err
}

// Clone creates a deep copy of the stream.
func (s *Stream) Clone() *Stream {
	// Clone dictionary
	clonedDict := s.dict.Clone()

	// Clone content
	clonedContent := make([]byte, len(s.content))
	copy(clonedContent, s.content)

	return &Stream{
		dict:           clonedDict,
		content:        clonedContent,
		objectResolver: s.objectResolver,
	}
}

func (s *Stream) setObjectResolver(resolver streamObjectResolver) {
	s.objectResolver = resolver
}

// Encode encodes the stream content with the specified filters.
// This is a placeholder for Phase 3 (Stream Processing).
func (s *Stream) Encode(_ []string) error {
	// TODO: Phase 3 - implement filter encoding
	return nil
}

// GetFilter returns the filter(s) applied to this stream.
// Returns nil if no filters are applied.
func (s *Stream) GetFilter() PdfObject {
	return s.dict.Get("Filter")
}

// GetDecodeParams returns the decode parameters for the filters.
// Returns nil if no decode parameters are specified.
func (s *Stream) GetDecodeParams() PdfObject {
	if parameters := s.dict.Get("DecodeParms"); parameters != nil {
		return parameters
	}
	// Acrobat accepts DP as a compatibility abbreviation in stream
	// dictionaries, not only in inline images.
	return s.dict.Get("DP")
}

// Bytes returns the raw stream content as a byte slice.
// Alias for Content() for convenience.
func (s *Stream) Bytes() []byte {
	return s.content
}

// Reader returns an io.Reader for the stream content.
func (s *Stream) Reader() io.Reader {
	return bytes.NewReader(s.content)
}
