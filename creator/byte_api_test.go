package creator

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestWriteTo(t *testing.T) {
	c := New()
	c.SetTitle("Test Document")

	page, err := c.NewPage()
	if err != nil {
		t.Fatalf("NewPage() failed: %v", err)
	}
	page.AddText("Hello, World!", 100, 700, Helvetica, 12)

	var buf bytes.Buffer
	n, err := c.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo() failed: %v", err)
	}

	if n == 0 {
		t.Error("WriteTo() returned 0 bytes")
	}

	if buf.Len() == 0 {
		t.Error("Buffer is empty after WriteTo()")
	}

	if int64(buf.Len()) != n {
		t.Errorf("WriteTo() returned %d bytes, but buffer has %d bytes", n, buf.Len())
	}

	// Check PDF header
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Error("Output does not start with PDF header")
	}
}

func TestBytes(t *testing.T) {
	c := New()
	c.SetTitle("Test Document")

	page, err := c.NewPage()
	if err != nil {
		t.Fatalf("NewPage() failed: %v", err)
	}
	page.AddText("Hello, World!", 100, 700, Helvetica, 12)

	pdfBytes, err := c.Bytes()
	if err != nil {
		t.Fatalf("Bytes() failed: %v", err)
	}

	if len(pdfBytes) == 0 {
		t.Error("Bytes() returned empty slice")
	}

	// Check PDF header
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		t.Error("Output does not start with PDF header")
	}

	// Check PDF trailer
	if !bytes.Contains(pdfBytes, []byte("%%EOF")) {
		t.Error("Output does not contain PDF trailer")
	}
}

func TestWriteToContext(t *testing.T) {
	c := New()
	page, err := c.NewPage()
	if err != nil {
		t.Fatalf("NewPage() failed: %v", err)
	}
	page.AddText("Test", 100, 700, Helvetica, 12)

	// Test with normal context
	ctx := context.Background()
	var buf bytes.Buffer
	_, err = c.WriteToContext(ctx, &buf)
	if err != nil {
		t.Fatalf("WriteToContext() failed: %v", err)
	}

	// Test with canceled context
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	var buf2 bytes.Buffer
	_, err = c.WriteToContext(canceledCtx, &buf2)
	if err == nil {
		t.Error("WriteToContext() should fail with canceled context")
	}
}

func TestWriteToConsistency(t *testing.T) {
	// Verify that WriteTo and Bytes produce identical output
	c := New()
	c.SetTitle("Consistency Test")
	c.SetAuthor("Test Author")

	page, err := c.NewPage()
	if err != nil {
		t.Fatalf("NewPage() failed: %v", err)
	}
	page.AddText("Test content", 100, 700, Helvetica, 12)

	// Get bytes via Bytes()
	bytes1, err := c.Bytes()
	if err != nil {
		t.Fatalf("Bytes() failed: %v", err)
	}

	// Get bytes via WriteTo()
	var buf bytes.Buffer
	_, err = c.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo() failed: %v", err)
	}
	bytes2 := buf.Bytes()

	// Compare
	if !bytes.Equal(bytes1, bytes2) {
		t.Error("Bytes() and WriteTo() produce different output")
	}
}

func TestWriteToMultiplePages(t *testing.T) {
	c := New()

	// Create multiple pages
	for i := 0; i < 3; i++ {
		page, err := c.NewPage()
		if err != nil {
			t.Fatalf("NewPage() failed: %v", err)
		}
		page.AddText("Page content", 100, 700, Helvetica, 12)
	}

	pdfBytes, err := c.Bytes()
	if err != nil {
		t.Fatalf("Bytes() failed: %v", err)
	}

	if len(pdfBytes) == 0 {
		t.Error("Multi-page PDF is empty")
	}

	// Verify PDF structure
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		t.Error("Multi-page PDF has invalid header")
	}
}

func TestWriteToEmptyDocument(t *testing.T) {
	c := New()

	// Empty document (no pages) should fail validation
	_, err := c.Bytes()
	if err == nil {
		t.Error("Bytes() should fail for empty document (no pages)")
	}
}

func TestWriteToWithTimeout(t *testing.T) {
	c := New()
	page, err := c.NewPage()
	if err != nil {
		t.Fatalf("NewPage() failed: %v", err)
	}
	page.AddText("Test", 100, 700, Helvetica, 12)

	// Test with a reasonable timeout (should succeed)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var buf bytes.Buffer
	_, err = c.WriteToContext(ctx, &buf)
	if err != nil {
		t.Fatalf("WriteToContext() failed with timeout: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("Buffer is empty after WriteToContext with timeout")
	}
}
