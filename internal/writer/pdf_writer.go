package writer

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/coregx/gxpdf/internal/document"
)

// PdfWriter writes PDF documents to files.
//
// This is the main infrastructure component for PDF file generation.
// It manages object numbering, cross-reference tables, and file structure.
//
// Example:
//
//	doc := document.NewDocument()
//	doc.AddPage(document.A4)
//
//	writer, err := NewPdfWriter("output.pdf")
//	if err != nil {
//	    return err
//	}
//	defer writer.Close()
//
//	err = writer.Write(doc)
type PdfWriter struct {
	file        *os.File          // Output file (nil for io.Writer mode)
	writer      *bufio.Writer     // Buffered writer
	countWriter *countingWriter   // Tracks bytes written (for io.Writer mode)
	objects     []*IndirectObject // All objects to write
	offsets     map[int]int64     // Byte offsets for each object number
	nextObjNum  int               // Next available object number
	closed      bool              // Whether Close() has been called
}

// countingWriter wraps an io.Writer and tracks bytes written.
type countingWriter struct {
	w io.Writer
	n int64
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	cw.n += int64(n)
	return n, err
}

// NewPdfWriter creates a new PDF writer for the specified file path.
//
// The file will be created or truncated if it already exists.
//
// Returns an error if the file cannot be created.
func NewPdfWriter(path string) (*PdfWriter, error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	return &PdfWriter{
		file:       file,
		writer:     bufio.NewWriter(file),
		objects:    make([]*IndirectObject, 0),
		offsets:    make(map[int]int64),
		nextObjNum: 1, // Object numbering starts at 1
		closed:     false,
	}, nil
}

// NewPdfWriterFromWriter creates a new PDF writer for an io.Writer.
//
// This is useful for writing PDFs to memory buffers, HTTP responses,
// or any other io.Writer implementation. Unlike NewPdfWriter, this
// does not create a file.
//
// Example:
//
//	var buf bytes.Buffer
//	writer := NewPdfWriterFromWriter(&buf)
//	err := writer.Write(doc)
//	writer.Close()
//	pdfBytes := buf.Bytes()
func NewPdfWriterFromWriter(w io.Writer) *PdfWriter {
	cw := &countingWriter{w: w}
	return &PdfWriter{
		file:        nil, // No file
		countWriter: cw,
		writer:      bufio.NewWriter(cw),
		objects:     make([]*IndirectObject, 0),
		offsets:     make(map[int]int64),
		nextObjNum:  1,
		closed:      false,
	}
}

// WriteWithPageContent writes a document with page content operations to the PDF file.
//
// This is similar to Write() but accepts page-level content operations
// (text, graphics, etc.) that will be rendered as PDF content streams.
//
// Parameters:
//   - doc: The document to write
//   - pageContents: Content operations for each page (indexed by page number)
//
// Returns an error if validation or writing fails.
func (w *PdfWriter) WriteWithPageContent(doc *document.Document, pageContents map[int][]TextOp) error {
	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	// Validate document
	if err := doc.Validate(); err != nil {
		return fmt.Errorf("document validation failed: %w", err)
	}

	// Reset state
	w.objects = make([]*IndirectObject, 0)
	w.offsets = make(map[int]int64)
	w.nextObjNum = 1

	// Write PDF header
	if err := w.writeHeader(doc.Version().String()); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Create pages tree with content
	pagesObjs, pagesRootRef, err := w.createPageTreeWithContent(doc, pageContents)
	if err != nil {
		return fmt.Errorf("failed to create page tree: %w", err)
	}

	// Add pages objects to write queue
	w.objects = append(w.objects, pagesObjs...)

	// Create catalog (references pages root)
	catalogObj := w.createCatalog(pagesRootRef, doc)
	w.objects = append([]*IndirectObject{catalogObj}, w.objects...)

	// Write all objects and track their offsets
	for _, obj := range w.objects {
		// Get current offset
		pos, err := w.getCurrentOffset()
		if err != nil {
			return fmt.Errorf("failed to get file position: %w", err)
		}

		w.offsets[obj.Number] = pos

		if _, err := obj.WriteTo(w.writer); err != nil {
			return fmt.Errorf("failed to write object %d: %w", obj.Number, err)
		}
	}

	// Write cross-reference table
	xrefOffset, err := w.writeXRef()
	if err != nil {
		return fmt.Errorf("failed to write xref: %w", err)
	}

	// Write trailer
	catalogRef := catalogObj.Number
	size := w.nextObjNum
	if err := w.writeTrailer(catalogRef, size, xrefOffset, doc); err != nil {
		return fmt.Errorf("failed to write trailer: %w", err)
	}

	// Flush buffer
	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	return nil
}

// WriteWithAllContent writes a document with text and graphics content operations.
//
// This is similar to WriteWithPageContent() but accepts both text and graphics operations.
//
// Parameters:
//   - doc: The document to write
//   - textContents: Text operations for each page (indexed by page number)
//   - graphicsContents: Graphics operations for each page (indexed by page number)
//
// Returns an error if validation or writing fails.
func (w *PdfWriter) WriteWithAllContent(
	doc *document.Document,
	textContents map[int][]TextOp,
	graphicsContents map[int][]GraphicsOp,
) error {
	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	// Validate document
	if err := doc.Validate(); err != nil {
		return fmt.Errorf("document validation failed: %w", err)
	}

	// Reset state
	w.objects = make([]*IndirectObject, 0)
	w.offsets = make(map[int]int64)
	w.nextObjNum = 1

	// Write PDF header
	if err := w.writeHeader(doc.Version().String()); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Create pages tree with all content (text + graphics)
	pagesObjs, pagesRootRef, err := w.createPageTreeWithAllContent(doc, textContents, graphicsContents)
	if err != nil {
		return fmt.Errorf("failed to create page tree: %w", err)
	}

	// Add pages objects to write queue
	w.objects = append(w.objects, pagesObjs...)

	// Create catalog (references pages root)
	catalogObj := w.createCatalog(pagesRootRef, doc)
	w.objects = append([]*IndirectObject{catalogObj}, w.objects...)

	// Write all objects and track their offsets
	for _, obj := range w.objects {
		// Get current offset
		pos, err := w.getCurrentOffset()
		if err != nil {
			return fmt.Errorf("failed to get file position: %w", err)
		}

		w.offsets[obj.Number] = pos

		if _, err := obj.WriteTo(w.writer); err != nil {
			return fmt.Errorf("failed to write object %d: %w", obj.Number, err)
		}
	}

	// Write cross-reference table
	xrefOffset, err := w.writeXRef()
	if err != nil {
		return fmt.Errorf("failed to write xref: %w", err)
	}

	// Write trailer
	catalogRef := catalogObj.Number
	size := w.nextObjNum
	if err := w.writeTrailer(catalogRef, size, xrefOffset, doc); err != nil {
		return fmt.Errorf("failed to write trailer: %w", err)
	}

	// Flush buffer
	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	return nil
}

// Write writes a document to the PDF file.
//
// This performs the following steps:
// 1. Write PDF header with version
// 2. Write catalog object
// 3. Write pages object tree
// 4. Write cross-reference table
// 5. Write trailer
//
// Returns an error if:
// - Document validation fails.
// - Document has no pages.
// - Any write operation fails.
func (w *PdfWriter) Write(doc *document.Document) error {
	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	// Validate document
	if err := doc.Validate(); err != nil {
		return fmt.Errorf("document validation failed: %w", err)
	}

	// Reset state (in case Write is called multiple times)
	w.objects = make([]*IndirectObject, 0)
	w.offsets = make(map[int]int64)
	w.nextObjNum = 1

	// Write PDF header
	if err := w.writeHeader(doc.Version().String()); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Create pages tree first (to get page references)
	pagesObjs, pagesRootRef, err := w.createPageTree(doc)
	if err != nil {
		return fmt.Errorf("failed to create page tree: %w", err)
	}

	// Add pages objects to write queue
	w.objects = append(w.objects, pagesObjs...)

	// Create catalog (references pages root)
	catalogObj := w.createCatalog(pagesRootRef, doc)
	w.objects = append([]*IndirectObject{catalogObj}, w.objects...)

	// Write all objects and track their offsets
	for _, obj := range w.objects {
		// Get current offset
		pos, err := w.getCurrentOffset()
		if err != nil {
			return fmt.Errorf("failed to get file position: %w", err)
		}

		w.offsets[obj.Number] = pos

		if _, err := obj.WriteTo(w.writer); err != nil {
			return fmt.Errorf("failed to write object %d: %w", obj.Number, err)
		}
	}

	// Write cross-reference table
	xrefOffset, err := w.writeXRef()
	if err != nil {
		return fmt.Errorf("failed to write xref: %w", err)
	}

	// Write trailer
	catalogRef := catalogObj.Number
	size := w.nextObjNum // Total number of objects + 1 (includes object 0)
	if err := w.writeTrailer(catalogRef, size, xrefOffset, doc); err != nil {
		return fmt.Errorf("failed to write trailer: %w", err)
	}

	// Flush buffer
	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	return nil
}

// Close closes the writer and the underlying file.
//
// It's safe to call Close multiple times.
func (w *PdfWriter) Close() error {
	if w.closed {
		return nil
	}

	w.closed = true

	// Flush any remaining buffered data
	if err := w.writer.Flush(); err != nil {
		if w.file != nil {
			_ = w.file.Close() // Best effort cleanup
		}
		return fmt.Errorf("failed to flush buffer: %w", err)
	}

	// Close the file if we have one (not needed for io.Writer mode)
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return fmt.Errorf("failed to close file: %w", err)
		}
	}

	return nil
}

// getCurrentOffset returns the current byte offset in the output.
// For file mode, it uses file.Seek. For io.Writer mode, it uses
// the counting writer plus buffered bytes.
func (w *PdfWriter) getCurrentOffset() (int64, error) {
	// Flush buffered data first to get accurate count
	if err := w.writer.Flush(); err != nil {
		return 0, err
	}

	if w.file != nil {
		// File mode: use Seek to get position
		pos, err := w.file.Seek(0, 1)
		if err != nil {
			return 0, err
		}
		return pos, nil
	}

	// io.Writer mode: use counting writer
	if w.countWriter != nil {
		return w.countWriter.n, nil
	}

	return 0, fmt.Errorf("no file or counting writer available")
}

// writeHeader writes the PDF header with version and binary marker.
//
// Format:
//
//	%PDF-1.7
//	%âãÏÓ
//
// The binary marker (4 bytes with values > 128) ensures the file
// is treated as binary by transfer programs.
func (w *PdfWriter) writeHeader(version string) error {
	// PDF header with version
	header := fmt.Sprintf("%%PDF-%s\n", version)
	if _, err := w.writer.WriteString(header); err != nil {
		return fmt.Errorf("failed to write PDF header: %w", err)
	}

	// Binary marker (ensures file is treated as binary)
	// Using bytes > 127 to force binary mode
	binaryMarker := []byte{0x25, 0xE2, 0xE3, 0xCF, 0xD3, 0x0A} // %âãÏÓ\n
	if _, err := w.writer.Write(binaryMarker); err != nil {
		return fmt.Errorf("failed to write binary marker: %w", err)
	}

	return nil
}

// writeXRef writes the cross-reference table.
//
// Format:
//
//	xref
//	0 N
//	0000000000 65535 f
//	0000000015 00000 n
//	...
//
// Returns the byte offset where xref starts.
func (w *PdfWriter) writeXRef() (int64, error) {
	// Get current position (where xref starts)
	xrefOffset, err := w.getCurrentOffset()
	if err != nil {
		return 0, fmt.Errorf("failed to get file position: %w", err)
	}

	// Write xref header
	if _, err := w.writer.WriteString("xref\n"); err != nil {
		return 0, fmt.Errorf("failed to write xref header: %w", err)
	}

	// Write subsection header: "0 N" (N = total number of objects including 0)
	subsectionHeader := fmt.Sprintf("0 %d\n", w.nextObjNum)
	if _, err := w.writer.WriteString(subsectionHeader); err != nil {
		return 0, fmt.Errorf("failed to write subsection header: %w", err)
	}

	// Write entry for object 0 (always free, generation 65535)
	if _, err := w.writer.WriteString("0000000000 65535 f \n"); err != nil {
		return 0, fmt.Errorf("failed to write object 0 entry: %w", err)
	}

	// Write entries for all objects (1 to nextObjNum-1)
	for i := 1; i < w.nextObjNum; i++ {
		offset, exists := w.offsets[i]
		if !exists {
			return 0, fmt.Errorf("missing offset for object %d", i)
		}

		// Format: "0000000015 00000 n \n" (10 digits offset, 5 digits generation, n/f flag)
		entry := fmt.Sprintf("%010d %05d n \n", offset, 0)
		if _, err := w.writer.WriteString(entry); err != nil {
			return 0, fmt.Errorf("failed to write xref entry for object %d: %w", i, err)
		}
	}

	return xrefOffset, nil
}

// writeTrailer writes the PDF trailer.
//
// Format:
//
//	trailer
//	<< /Size N /Root 1 0 R >>
//	startxref
//	<xref_offset>
//	%%EOF
func (w *PdfWriter) writeTrailer(catalogRef int, size int, xrefOffset int64, doc *document.Document) error {
	// Write trailer keyword
	if _, err := w.writer.WriteString("trailer\n"); err != nil {
		return fmt.Errorf("failed to write trailer keyword: %w", err)
	}

	// Build trailer dictionary
	var trailerDict bytes.Buffer
	trailerDict.WriteString("<<")
	trailerDict.WriteString(fmt.Sprintf(" /Size %d", size))
	trailerDict.WriteString(fmt.Sprintf(" /Root %d 0 R", catalogRef))

	// Add Info dictionary if metadata exists
	if doc.Title() != "" || doc.Author() != "" || doc.Subject() != "" {
		infoRef := w.allocateObjNum()
		trailerDict.WriteString(fmt.Sprintf(" /Info %d 0 R", infoRef))

		// Create Info object
		infoObj := w.createInfo(infoRef, doc)
		w.objects = append(w.objects, infoObj)

		// Write Info object immediately (before startxref)
		offset := xrefOffset // Info comes after xref, so we track it
		w.offsets[infoRef] = offset

		// We need to write it to a temp buffer to calculate size,
		// but for simplicity, we'll skip Info in this iteration
		// TODO: Implement Info object writing in next iteration
		_ = infoObj // Prevent unused variable error
	}

	trailerDict.WriteString(" >>")

	// Write trailer dictionary
	if _, err := w.writer.WriteString(trailerDict.String()); err != nil {
		return fmt.Errorf("failed to write trailer dictionary: %w", err)
	}
	if _, err := w.writer.WriteString("\n"); err != nil {
		return err
	}

	// Write startxref
	if _, err := w.writer.WriteString("startxref\n"); err != nil {
		return fmt.Errorf("failed to write startxref: %w", err)
	}

	// Write xref offset
	xrefOffsetStr := fmt.Sprintf("%d\n", xrefOffset)
	if _, err := w.writer.WriteString(xrefOffsetStr); err != nil {
		return fmt.Errorf("failed to write xref offset: %w", err)
	}

	// Write EOF marker
	if _, err := w.writer.WriteString("%%EOF\n"); err != nil {
		return fmt.Errorf("failed to write EOF marker: %w", err)
	}

	return nil
}

// allocateObjNum allocates a new object number and returns it.
func (w *PdfWriter) allocateObjNum() int {
	num := w.nextObjNum
	w.nextObjNum++
	return num
}

// createInfo creates an Info dictionary object with document metadata.
func (w *PdfWriter) createInfo(objNum int, doc *document.Document) *IndirectObject {
	var info bytes.Buffer
	info.WriteString("<<")

	if doc.Title() != "" {
		info.WriteString(fmt.Sprintf(" /Title (%s)", escapePDFString(doc.Title())))
	}
	if doc.Author() != "" {
		info.WriteString(fmt.Sprintf(" /Author (%s)", escapePDFString(doc.Author())))
	}
	if doc.Subject() != "" {
		info.WriteString(fmt.Sprintf(" /Subject (%s)", escapePDFString(doc.Subject())))
	}
	if doc.Creator() != "" {
		info.WriteString(fmt.Sprintf(" /Creator (%s)", escapePDFString(doc.Creator())))
	}
	if doc.Producer() != "" {
		info.WriteString(fmt.Sprintf(" /Producer (%s)", escapePDFString(doc.Producer())))
	}

	// Creation date
	info.WriteString(fmt.Sprintf(" /CreationDate (%s)", formatPDFDate(doc.CreationDate())))

	// Modification date
	info.WriteString(fmt.Sprintf(" /ModDate (%s)", formatPDFDate(doc.ModificationDate())))

	info.WriteString(" >>")

	return NewIndirectObject(objNum, 0, info.Bytes())
}

// escapePDFString escapes special characters in PDF strings.
func escapePDFString(s string) string {
	// For now, simple implementation - just handle basic escaping
	// TODO: Implement full PDF string escaping (parentheses, backslash, etc.)
	return s
}

// formatPDFDate formats a time.Time as a PDF date string.
//
// Format: D:YYYYMMDDHHmmSSOHH'mm'.
// Example: D:20250127123045+03'00'.
func formatPDFDate(t time.Time) string {
	// Format: D:YYYYMMDDHHmmSS+HH'mm'
	_, offset := t.Zone()
	offsetHours := offset / 3600
	offsetMinutes := (offset % 3600) / 60

	sign := "+"
	if offset < 0 {
		sign = "-"
		offsetHours = -offsetHours
		offsetMinutes = -offsetMinutes
	}

	return fmt.Sprintf("D:%04d%02d%02d%02d%02d%02d%s%02d'%02d'",
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second(),
		sign, offsetHours, offsetMinutes)
}
