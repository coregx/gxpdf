package parser

import (
	"bytes"
	"strings"
	"testing"
)

// The XRef-stream parsing paths in xref.go read stream bytes sized by the
// untrusted /Length dictionary entry. Before the fix they allocated
// make([]byte, length) up front, so a cross-reference stream (present in most
// modern PDF 1.5+ files) claiming a huge /Length but carrying a tiny body
// forced a multi-gigabyte allocation and OOM during Open() itself, before any
// page content was touched. Both paths must instead read only the bytes
// present and report a length mismatch, without a large allocation.

// ParseXRefStreamWithFileAccess (the file-access path used when reading a real
// document) must not honor an oversized /Length.
func TestParseXRefStreamWithFileAccessHugeLengthNoHugeAlloc(t *testing.T) {
	obj := "90 0 obj\n<< /Type /XRef /Length 9000000000 /W [1 2 1] /Size 1 >>\nstream\ntiny body\nendstream\nendobj\n"
	data := []byte(obj)
	p := NewParser(bytes.NewReader(data))
	var err error
	assertSmallAlloc(t, func() { _, err = p.ParseXRefStreamWithFileAccess(bytes.NewReader(data), 0) })
	if err == nil {
		t.Fatal("expected a length-mismatch error for the oversized xref stream, got nil")
	}
	if !strings.Contains(err.Error(), "expected 9000000000 bytes") {
		t.Fatalf("expected a byte-count mismatch error, got: %v", err)
	}
}

// parseStreamData (the token-stream path, reached when reading XRef streams via
// the lexer) must not honor an oversized /Length either. Driven directly so
// the buffered lexer's lookahead does not interfere with the newline handling.
func TestParseStreamDataHugeLengthNoHugeAlloc(t *testing.T) {
	// NewParser primes current='stream'; the lexer consumes the '\r', leaving
	// the reader at the '\n' that parseStreamData skips before reading content.
	p := NewParser(strings.NewReader("stream\r\ntiny body\nendstream"))
	dict := NewDictionary()
	dict.Set("Length", NewInteger(9000000000))
	var err error
	assertSmallAlloc(t, func() { _, err = p.parseStreamData(dict) })
	if err == nil {
		t.Fatal("expected a length-mismatch error for the oversized xref stream, got nil")
	}
	if !strings.Contains(err.Error(), "expected 9000000000 bytes") {
		t.Fatalf("expected a byte-count mismatch error, got: %v", err)
	}
}
