# GxPDF - Enterprise-Grade PDF Library for Go

![GxPDF Cover](assets/gh_cover.png)

[![GitHub Release](https://img.shields.io/github/v/release/coregx/gxpdf)](https://github.com/coregx/gxpdf/releases/latest)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![CI](https://github.com/coregx/gxpdf/actions/workflows/test.yml/badge.svg)](https://github.com/coregx/gxpdf/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/coregx/gxpdf/branch/main/graph/badge.svg)](https://codecov.io/gh/coregx/gxpdf)
[![GoDoc](https://pkg.go.dev/badge/github.com/coregx/gxpdf)](https://pkg.go.dev/github.com/coregx/gxpdf)
[![Go Report Card](https://goreportcard.com/badge/github.com/coregx/gxpdf)](https://goreportcard.com/report/github.com/coregx/gxpdf)
[![GitHub Stars](https://img.shields.io/github/stars/coregx/gxpdf)](https://github.com/coregx/gxpdf/stargazers)

**GxPDF** is a modern, high-performance PDF library for Go, built with clean architecture and Go 1.25+ best practices.

**[View Example PDF](assets/gxpdf_enterprise_brochure.pdf)** - See what GxPDF can create!

## Key Features

### Declarative Builder API (NEW)

QuestPDF-inspired declarative API with 12-column grid, automatic pagination, and composable components. Define documents as nested closures — the engine handles page breaks, header/footer repetition, and two-pass page number resolution automatically.

```go
import "github.com/coregx/gxpdf/builder"

doc := builder.NewBuilder(
    builder.WithPageSize(builder.A4),
    builder.WithMargins(builder.Mm(20), builder.Mm(15), builder.Mm(20), builder.Mm(15)),
    builder.WithTitle("Invoice"),
    builder.WithDefaultFontSize(11),
)

doc.Page(func(page *builder.PageBuilder) {
    page.Header(func(h *builder.Container) {
        h.Row(func(r *builder.RowBuilder) {
            r.Col(8, func(c *builder.ColBuilder) {
                c.Text("ACME Corporation", builder.Bold(), builder.FontSize(16))
            })
            r.Col(4, func(c *builder.ColBuilder) {
                c.Text("INVOICE", builder.Bold(), builder.FontSize(20),
                    builder.AlignRight(), builder.TextColor(builder.Navy))
            })
        })
        h.Line()
    })

    page.Content(func(content *builder.Container) {
        content.Spacer(builder.Mm(10))
        content.Row(func(r *builder.RowBuilder) {
            r.Col(6, func(c *builder.ColBuilder) {
                c.Text("Bill To:", builder.Bold())
                c.Text("John Doe")
                c.Text("123 Main Street")
            })
            r.Col(6, func(c *builder.ColBuilder) {
                c.Text("Invoice #: INV-2026-001", builder.AlignRight())
                c.Text("Date: March 23, 2026", builder.AlignRight())
            })
        })
    })

    page.Footer(func(f *builder.Container) {
        f.PageNumber(builder.PageNum+" of "+builder.TotalPages,
            builder.AlignCenter(), builder.FontSize(8), builder.TextColor(builder.Gray))
    })
})

pdfBytes, err := doc.Build()
```

### Digital Signatures (NEW)

Sign and verify PDF documents with zero external dependencies:

```go
import "github.com/coregx/gxpdf/signature"

// Generate or load your certificate
key, cert, _ := signature.GenerateTestCertificate()
signer, _ := signature.NewLocalSigner(key, []*x509.Certificate{cert})

// Sign a PDF
signed, err := signature.SignDocument(pdfBytes, signer,
    signature.WithReason("Approved"),
    signature.WithLocation("Moscow"),
)

// Verify signatures
infos, err := signature.Verify(signed)
fmt.Println(infos[0].SignedBy, infos[0].Valid) // "CN=Test" true
```

- **PAdES B-B** — CMS/PKCS#7 with ESS signing-certificate-v2
- **PAdES B-T** — RFC 3161 timestamping via `WithTimestamp(tsaURL)`
- **RSA + ECDSA** — SHA-256 by default, SHA-384/512 configurable
- **Verification** — ByteRange hash + CMS cryptographic verification
- **Incremental update** — preserves existing content and prior signatures

### PDF Creation (Creator API)
- **Text & Typography** - Rich text with multiple fonts, styles, and colors
- **Graphics** - Lines, rectangles, circles, polygons, ellipses, arcs (wedge/chord), cubic and quadratic Bezier curves
- **Gradients** - Linear and radial gradient fills with full PDF Shading (multi-stop support)
- **Color Spaces** - RGB and CMYK support
- **Tables** - Complex tables with merged cells, borders, backgrounds
- **Images** - JPEG and PNG with transparency support
- **Fonts** - Standard 14 PDF fonts + TTF/OTF embedding with full Unicode support (Cyrillic, CJK, symbols)
- **Document Structure** - Chapters, auto-generated Table of Contents
- **Unicode Metadata** - Full Unicode support in PDF Info dictionary, annotations, form fields, and signatures (Cyrillic, CJK, Arabic, emoji)
- **Annotations** - Sticky notes, highlights, underlines, stamps
- **Interactive Forms** - Text fields, checkboxes, radio buttons, dropdowns
- **Security** - RC4 (40/128-bit) and AES (128/256-bit) encryption
- **Watermarks** - Text watermarks with rotation and opacity
- **Page Sizes** - 35+ built-in sizes (ISO A/B/C, ANSI, photo, book, JIS, envelopes, slides)
- **Custom Dimensions** - Arbitrary page sizes with unit conversion helpers (inches, mm, cm)
- **Landscape/Portrait** - True landscape via `NewPageWithSize(A4, Landscape)` — industry-standard swapped-MediaBox
- **Text Rotation** - Rotated text at any angle via `Tm` operator (standard 14 + custom TTF fonts)
- **Opacity/Transparency** - Text and shape opacity via ExtGState (`AddTextColorAlpha`, shape `Opacity` option)
- **Page Operations** - Merge, split, rotate, append

### PDF Reading & Extraction
- **Encrypted PDF Reading** - Open password-protected PDFs (RC4-40/128, AES-128)
- **Table Extraction** - Industry-leading accuracy (100% on bank statements)
  - Merged cell detection via Grid Gap Analysis (Lattice mode) — `CellAt()` returns `RowSpan`/`ColSpan` metadata
- **Text Extraction** - Full text with positions and Unicode support
- **Image Extraction** - Extract embedded images
- **Export Formats** - CSV, JSON, Excel

## Installation

```bash
go get github.com/coregx/gxpdf
```

**Requirements**: Go 1.25 or later

## Quick Start

### Creating a PDF with the Builder API

```go
package main

import (
    "log"
    "github.com/coregx/gxpdf/builder"
)

func main() {
    doc := builder.NewBuilder(
        builder.WithPageSize(builder.A4),
        builder.WithTitle("Hello World"),
    )

    doc.Page(func(page *builder.PageBuilder) {
        page.Content(func(c *builder.Container) {
            c.Text("Hello, GxPDF!", builder.Bold(), builder.FontSize(24))
            c.Spacer(builder.Mm(5))
            c.Text("Professional PDF creation in Go.")
        })
    })

    if err := doc.BuildToFile("output.pdf"); err != nil {
        log.Fatal(err)
    }
}
```

### Creating a PDF Document (Creator API)

```go
package main

import (
    "log"
    "github.com/coregx/gxpdf/creator"
)

func main() {
    c := creator.New()
    c.SetTitle("My Document")
    c.SetAuthor("GxPDF")

    page, _ := c.NewPage()

    // Add text
    page.AddText("Hello, GxPDF!", 100, 750, creator.HelveticaBold, 24)
    page.AddText("Professional PDF creation in Go", 100, 720, creator.Helvetica, 12)

    // Draw shapes
    page.DrawRectangle(100, 600, 200, 100, &creator.RectangleOptions{
        FillColor:   &creator.Blue,
        StrokeColor: &creator.Black,
        StrokeWidth: 2,
    })

    if err := c.WriteToFile("output.pdf"); err != nil {
        log.Fatal(err)
    }
}
```

### Unicode Text with Custom Fonts

```go
c := creator.New()
page, _ := c.NewPage()

// Load custom font with Unicode support
font, _ := c.LoadFont("/path/to/arial.ttf")

// Cyrillic text
page.AddTextCustomFont("Привет, мир!", 100, 700, font, 18)

// CJK text (requires appropriate font like Malgun Gothic)
cjkFont, _ := c.LoadFont("/path/to/malgun.ttf")
page.AddTextCustomFont("你好世界 • 안녕하세요", 100, 670, cjkFont, 16)

c.WriteToFile("unicode.pdf")
```

### Creating Encrypted PDFs

```go
c := creator.New()

c.SetEncryption(creator.EncryptionOptions{
    UserPassword:  "user123",
    OwnerPassword: "owner456",
    Permissions:   creator.PermissionPrint | creator.PermissionCopy,
    Algorithm:     creator.EncryptionAES256,
})

page, _ := c.NewPage()
page.AddText("This document is encrypted!", 100, 750, creator.Helvetica, 14)
c.WriteToFile("encrypted.pdf")
```

### Creating Documents with Chapters and TOC

```go
c := creator.New()
c.EnableTOC()

ch1 := creator.NewChapter("Introduction")
ch1.Add(creator.NewParagraph("Introduction content..."))

ch1_1 := ch1.NewSubChapter("Background")
ch1_1.Add(creator.NewParagraph("Background information..."))

ch2 := creator.NewChapter("Methods")
ch2.Add(creator.NewParagraph("Methods description..."))

c.AddChapter(ch1)
c.AddChapter(ch2)

c.WriteToFile("document_with_toc.pdf")
```

### Interactive Forms (AcroForm)

```go
import "github.com/coregx/gxpdf/creator/forms"

c := creator.New()
page, _ := c.NewPage()

// Text field
nameField := forms.NewTextField("name", 100, 700, 200, 20)
nameField.SetLabel("Full Name:")
nameField.SetRequired(true)
page.AddField(nameField)

// Checkbox
agreeBox := forms.NewCheckbox("agree", 100, 660, 15, 15)
agreeBox.SetLabel("I agree to the terms")
page.AddField(agreeBox)

// Dropdown
countryDropdown := forms.NewDropdown("country", 100, 620, 150, 20)
countryDropdown.AddOption("us", "United States")
countryDropdown.AddOption("uk", "United Kingdom")
page.AddField(countryDropdown)

c.WriteToFile("form.pdf")
```

### Reading and Filling Forms

```go
// Read form fields from existing PDF
doc, _ := gxpdf.Open("form.pdf")
defer doc.Close()

// Check if document has a form
if doc.HasForm() {
    fields, _ := doc.GetFormFields()
    for _, f := range fields {
        fmt.Printf("%s (%s): %v\n", f.Name(), f.Type(), f.Value())
    }
}

// Fill form fields
app, _ := creator.NewAppender("form.pdf")
defer app.Close()

app.SetFieldValue("name", "John Doe")
app.SetFieldValue("email", "john@example.com")
app.SetFieldValue("agree", true)  // Checkbox
app.SetFieldValue("country", "USA")  // Dropdown

app.WriteToFile("filled_form.pdf")
```

### Flattening Forms

```go
// Convert form fields to static content (non-editable)
app, _ := creator.NewAppender("filled_form.pdf")
defer app.Close()

app.FlattenForm()  // Flatten all fields
// Or: app.FlattenFields("signature", "date")  // Specific fields

app.WriteToFile("flattened.pdf")
```

### Extracting Text from PDFs

```go
doc, _ := gxpdf.Open("document.pdf")
defer doc.Close()

// Extract text from a specific page (1-based)
text, err := doc.ExtractTextFromPage(1)
if err != nil {
    log.Fatal(err)
}
fmt.Println(text)

// Or iterate all pages
for _, page := range doc.Pages() {
    fmt.Println(page.ExtractText())
}
```

### Positioned Text Extraction (NEW)

```go
doc, _ := gxpdf.Open("document.pdf")
defer doc.Close()

// Get text with positions, sizes, and font info
elements, _ := doc.ExtractTextElementsFromPage(1)
for _, e := range elements {
    fmt.Printf("%q at (%.1f, %.1f) font=%s size=%.1f\n",
        e.Text, e.X, e.Y, e.FontName, e.FontSize)
}
```

### Opening PDFs from Memory (NEW)

```go
// Read PDF from HTTP request, database, or any byte source
data, _ := io.ReadAll(httpResponse.Body)

doc, err := gxpdf.OpenFromBytes(data)
if err != nil {
    log.Fatal(err)
}
defer doc.Close()

// Works with encrypted PDFs too
doc, _ = gxpdf.OpenFromBytesWithPassword(data, "secret")
```

### Extracting Embedded Fonts (NEW)

```go
doc, _ := gxpdf.Open("document.pdf")
defer doc.Close()

// Extract all embedded fonts (TrueType, CIDFontType2)
fonts, _ := doc.GetEmbeddedFonts()
for _, f := range fonts {
    fmt.Printf("Font: %s (%s), %d bytes\n", f.Name, f.Subtype, len(f.Data))
}
```

### Extracting Vector Graphics (NEW)

```go
doc, _ := gxpdf.Open("diagram.pdf")
defer doc.Close()

// Extract all vector paths with colors, opacity, and CTM-transformed coordinates
paths, _ := doc.GetVectorGraphicsForPage(1)
for _, p := range paths {
    fmt.Printf("Path: %d verbs, mode=%v, stroke=%v, fill=%v\n",
        len(p.Verbs), p.PaintMode, p.StrokeColor, p.FillColor)
}
```

### Extracting Tables from PDFs

```go
doc, _ := gxpdf.Open("bank_statement.pdf")
defer doc.Close()

tables := doc.ExtractTables()
for _, table := range tables {
    fmt.Printf("Table: %d rows x %d cols\n", table.RowCount(), table.ColumnCount())

    // Access cell text (simple API, backward compatible)
    val := table.Cell(0, 0)
    fmt.Println("Header:", val)

    // Export to CSV
    csv, _ := table.ToCSV()
    fmt.Println(csv)
}
```

### Merged Cell Detection

Lattice tables with merged cells (exam schedules, timetables, financial reports) are detected automatically. Use `CellAt()` to inspect span metadata:

```go
doc, _ := gxpdf.Open("timetable.pdf")
defer doc.Close()

tables := doc.ExtractTables()
for _, table := range tables {
    for r := 0; r < table.RowCount(); r++ {
        for c := 0; c < table.ColumnCount(); c++ {
            cell := table.CellAt(r, c)
            if cell == nil {
                continue
            }
            if cell.IsMerged() {
                fmt.Printf("[%d,%d] %q spans %dx%d\n",
                    r, c, cell.Text, cell.RowSpan, cell.ColSpan)
            }
        }
    }
}
```

### Reading Encrypted PDFs

```go
// PDFs with empty user password (permissions-only) open transparently
doc, _ := gxpdf.Open("bank_statement_encrypted.pdf")
defer doc.Close()

// PDFs requiring a password
doc, err := gxpdf.OpenWithPassword("protected.pdf", "secret")
if errors.Is(err, gxpdf.ErrPasswordRequired) {
    log.Fatal("Wrong password")
}
defer doc.Close()

fmt.Printf("Pages: %d\n", doc.PageCount())
```

## Package Structure

```
github.com/coregx/gxpdf
├── gxpdf.go          # Main entry point (Open, OpenWithPassword)
├── builder/          # Declarative Builder API (12-col grid, tables, rich text)
├── layout/           # Pure computation layout engine (zero PDF deps)
├── signature/        # Digital signatures (PAdES B-B/B-T, verify)
├── export/           # Export formats (CSV, JSON, Excel)
├── creator/          # Low-level PDF creation API
│   └── forms/        # Interactive form fields
├── logging/          # Configurable debug logging
└── internal/         # Private implementation
    ├── application/  # Use cases (extraction, reading)
    └── infrastructure/ # PDF parsing, encoding, writing
```

## Documentation

- **[API Reference](https://pkg.go.dev/github.com/coregx/gxpdf)** - Full API documentation
- **[Examples](examples/)** - Code examples for all features
- **[Enterprise Brochure (PDF)](assets/gxpdf_enterprise_brochure.pdf)** - Sample PDF created with GxPDF
- **[Architecture](docs/ARCHITECTURE.md)** - DDD architecture overview
- **[Contributing](CONTRIBUTING.md)** - Contribution guidelines
- **[Security](SECURITY.md)** - Security policy

## Testing

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run with coverage
go test -cover ./...
```

### Debug Logging

Debug output is disabled by default. To enable it, configure a logger via the `logging` package:

```go
import (
    "log/slog"
    "os"
    "github.com/coregx/gxpdf/logging"
)

logging.SetLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
    Level: slog.LevelWarn,
})))
```

Convenience methods like `ExtractText()`, `ExtractTables()`, and `GetImages()` log errors via slog instead of returning them. Enable logging to see why these methods return empty results.

## Roadmap

See [ROADMAP.md](ROADMAP.md) for the full development plan and version history.

## License

GxPDF is released under the **MIT License**. See [LICENSE](LICENSE) for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/coregx/gxpdf/issues)
- **Discussions**: [GitHub Discussions](https://github.com/coregx/gxpdf/discussions)

---

**Built with Go 1.25+ and Domain-Driven Design**
