// Package gxpdf provides a modern, enterprise-grade PDF library for Go.
//
// GxPDF is designed to be the reference PDF library for Go applications,
// offering simple API for common tasks while providing full power for advanced use cases.
//
// # Quick Start
//
// Open a PDF and extract tables:
//
//	doc, err := gxpdf.Open("invoice.pdf")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer doc.Close()
//
//	tables := doc.ExtractTables()
//	for _, table := range tables {
//	    fmt.Println(table.Rows())
//	}
//
// # Architecture
//
// The library follows modern Go best practices (2025+):
//   - Root package for core API (gxpdf.Open, gxpdf.Document, gxpdf.Table)
//   - Subpackages for specialized functionality (export/, creator/)
//   - Internal packages for implementation details
//
// # Features
//
//   - PDF reading and parsing
//   - Table extraction with 4-Pass Hybrid detection (100% accuracy on bank statements)
//   - Text extraction with position information
//   - Export to CSV, JSON, Excel
//   - PDF creation (coming soon)
//
// # Thread Safety
//
// Document instances are safe for concurrent read operations.
// Write operations should be synchronized by the caller.
// For PDF creation, use the creator package - each Creator instance
// should be used from a single goroutine.
package gxpdf

import (
	"context"
	"fmt"

	"github.com/coregx/gxpdf/internal/buildinfo"
	"github.com/coregx/gxpdf/internal/parser"
	"github.com/coregx/gxpdf/internal/security"
)

// Version is the gxpdf module version the running binary was built with, as
// recorded by the Go toolchain: the release tag (e.g. "v0.9.5") when gxpdf is
// a dependency, a pseudo-version for builds from an untagged checkout, or
// "(devel)" when no version was recorded (go run, go test, local replace).
var Version = buildinfo.Read().Version

// pathFromBytes is the path sentinel reported by Documents opened from in-memory bytes.
const pathFromBytes = "<bytes>"

// ErrPasswordRequired is returned when a password is needed to open an encrypted PDF.
// Use OpenWithPassword to provide a password.
var ErrPasswordRequired = security.ErrPasswordRequired

// Open opens a PDF file and returns a Document for reading.
//
// This is the main entry point for reading PDF files.
// The returned Document must be closed after use.
//
// Example:
//
//	doc, err := gxpdf.Open("document.pdf")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer doc.Close()
//
//	fmt.Printf("Pages: %d\n", doc.PageCount())
func Open(path string) (*Document, error) {
	return OpenWithContext(context.Background(), path)
}

// OpenWithContext opens a PDF file with a custom context.
//
// The context can be used for cancellation and timeouts.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	doc, err := gxpdf.OpenWithContext(ctx, "large-document.pdf")
func OpenWithContext(ctx context.Context, path string) (*Document, error) {
	reader, err := parser.OpenPDF(path)
	if err != nil {
		return nil, fmt.Errorf("gxpdf: failed to open %s: %w", path, err)
	}

	return &Document{
		reader: reader,
		ctx:    ctx,
		path:   path,
	}, nil
}

// MustOpen opens a PDF file and panics on error.
//
// This is useful for initialization in tests or when the file is known to exist.
//
// Example:
//
//	doc := gxpdf.MustOpen("known-good.pdf")
//	defer doc.Close()
func MustOpen(path string) *Document {
	doc, err := Open(path)
	if err != nil {
		panic(err)
	}
	return doc
}

// OpenWithPassword opens a password-protected PDF file.
//
// Use this for encrypted PDFs that require a non-empty password.
// For PDFs with empty user password (permissions-only encryption),
// Open() handles them transparently.
//
// Example:
//
//	doc, err := gxpdf.OpenWithPassword("encrypted.pdf", "secret")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer doc.Close()
func OpenWithPassword(path, password string) (*Document, error) {
	return OpenWithPasswordAndContext(context.Background(), path, password)
}

// OpenWithPasswordAndContext opens a password-protected PDF with a custom context.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	doc, err := gxpdf.OpenWithPasswordAndContext(ctx, "encrypted.pdf", "secret")
func OpenWithPasswordAndContext(ctx context.Context, path, password string) (*Document, error) {
	reader, err := parser.OpenPDFWithPassword(path, password)
	if err != nil {
		return nil, fmt.Errorf("gxpdf: failed to open %s: %w", path, err)
	}

	return &Document{
		reader: reader,
		ctx:    ctx,
		path:   path,
	}, nil
}

// OpenFromBytes opens a PDF document from an in-memory byte slice.
//
// This is equivalent to Open but reads from memory instead of the filesystem.
// It is useful when the PDF data has been received over the network, read from
// a database, or produced in-process without writing to disk.
//
// The returned Document must be closed after use (Close is a no-op for
// in-memory documents but should still be called for future compatibility).
//
// Example:
//
//	data, err := os.ReadFile("document.pdf")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	doc, err := gxpdf.OpenFromBytes(data)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer doc.Close()
//
//	fmt.Printf("Pages: %d\n", doc.PageCount())
func OpenFromBytes(data []byte) (*Document, error) {
	return OpenFromBytesWithContext(context.Background(), data)
}

// OpenFromBytesWithContext opens a PDF document from an in-memory byte slice
// with a custom context.
//
// The context can be used for cancellation and timeouts during table or text
// extraction operations.
func OpenFromBytesWithContext(ctx context.Context, data []byte) (*Document, error) {
	reader, err := parser.OpenPDFFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("gxpdf: failed to open PDF from bytes: %w", err)
	}

	return &Document{
		reader: reader,
		ctx:    ctx,
		path:   pathFromBytes,
	}, nil
}

// OpenFromBytesWithPassword opens a password-protected PDF from an in-memory byte slice.
//
// For PDFs with an empty user password (permissions-only encryption),
// OpenFromBytes handles them transparently.
//
// Example:
//
//	data, _ := os.ReadFile("encrypted.pdf")
//	doc, err := gxpdf.OpenFromBytesWithPassword(data, "secret")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer doc.Close()
func OpenFromBytesWithPassword(data []byte, password string) (*Document, error) {
	return OpenFromBytesWithPasswordAndContext(context.Background(), data, password)
}

// OpenFromBytesWithPasswordAndContext opens a password-protected in-memory PDF
// with a custom context.
func OpenFromBytesWithPasswordAndContext(ctx context.Context, data []byte, password string) (*Document, error) {
	reader, err := parser.OpenPDFFromBytesWithPassword(data, password)
	if err != nil {
		return nil, fmt.Errorf("gxpdf: failed to open encrypted PDF from bytes: %w", err)
	}

	return &Document{
		reader: reader,
		ctx:    ctx,
		path:   pathFromBytes,
	}, nil
}
