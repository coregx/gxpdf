package parser

import (
	"strings"
	"testing"
)

// A stream's /Length is untrusted. Before the fix it sized the content buffer
// directly (make([]byte, length)), so a stream claiming a huge length but
// carrying only a few bytes forced a multi-gigabyte allocation and OOM. The
// parser must instead read the bytes actually present and report a length
// mismatch, without a large allocation.
func TestParseStreamHugeLengthNoHugeAlloc(t *testing.T) {
	// A dictionary with a ~9 GB declared Length but a tiny body.
	input := "1 0 obj\n<< /Length 9000000000 >>\nstream\nhello world\nendstream\nendobj"
	p := NewParser(strings.NewReader(input))
	_, err := p.ParseIndirectObject()
	if err == nil {
		t.Fatal("expected a length-mismatch error for the oversized stream, got nil")
	}
	if !strings.Contains(err.Error(), "expected 9000000000 bytes") {
		t.Fatalf("expected a byte-count mismatch error, got: %v", err)
	}
}

// A well-formed stream with a correct /Length still parses.
func TestParseStreamValidLength(t *testing.T) {
	body := "hello world"
	input := "1 0 obj\n<< /Length 11 >>\nstream\n" + body + "\nendstream\nendobj"
	p := NewParser(strings.NewReader(input))
	ind, err := p.ParseIndirectObject()
	if err != nil {
		t.Fatalf("valid stream failed to parse: %v", err)
	}
	stream, ok := ind.Object.(*Stream)
	if !ok {
		t.Fatalf("expected *Stream, got %T", ind.Object)
	}
	if string(stream.Content()) != body {
		t.Fatalf("stream content mismatch: got %q", stream.Content())
	}
}
