# GxPDF Architecture

Technical architecture overview of the GxPDF PDF library.

**Version**: v0.7.0+
**Last Updated**: 2026-03-23

## Project Structure

```
github.com/coregx/gxpdf
├── gxpdf.go              # Main public API entry point
├── cmd/gxpdf/            # CLI application
│   └── commands/         # CLI command implementations
├── builder/              # Declarative Builder API (high-level, user-facing)
│   └── internal/         # Builder internals (font bridge, PDF renderer)
├── layout/               # Pure computation layout engine (zero PDF dependencies)
├── signature/            # Digital signatures (PAdES B-B/B-T, CMS/PKCS#7, verify)
├── creator/              # PDF creation API (low-level primitives)
│   └── forms/            # Interactive form fields (AcroForm)
├── export/               # Export formats (CSV, JSON, Excel)
└── internal/             # Private implementation
    ├── document/         # Document model, pages, metadata
    ├── encoding/         # Stream codecs (Flate, DCT, ASCII85, LZW)
    ├── extractor/        # Text, image, graphics extraction
    ├── fonts/            # Font handling (Standard 14 + TTF/OTF)
    ├── models/           # Data models
    │   ├── content/      # Content stream models
    │   ├── table/        # Table/Cell rich models
    │   └── types/        # Common types (Image, Rectangle)
    ├── parser/           # PDF file parsing
    ├── reader/           # PDF document reader
    ├── security/         # Encryption (RC4, AES-128/256)
    ├── tabledetect/      # Table detection (4-Pass Hybrid algorithm)
    └── writer/           # PDF file generation
```

## Generation Stack: Three-Layer Architecture

GxPDF provides two complementary generation APIs that share the same underlying rendering engine:

```
Layer 3: builder/    — User-facing declarative API
Layer 2: layout/     — Pure computation layout engine
Layer 1: creator/    — Low-level PDF primitives
```

Each layer has a clear responsibility and strict dependency direction — upper layers depend on lower layers, never the reverse.

### Layer 3: builder/ — Declarative API

The `builder` package is the recommended entry point for document generation. It provides a QuestPDF-inspired API where documents are expressed as nested closures. The engine handles page breaks, header/footer repetition, and two-pass page number resolution automatically.

```go
doc := builder.NewBuilder(
    builder.WithPageSize(builder.A4),
    builder.WithMargins(builder.Mm(20), builder.Mm(15), builder.Mm(20), builder.Mm(15)),
    builder.WithTitle("Annual Report"),
)

doc.Page(func(page *builder.PageBuilder) {
    page.Header(func(h *builder.Container) {
        h.Text("ACME Corporation", builder.Bold(), builder.FontSize(14))
        h.Line()
    })
    page.Content(func(c *builder.Container) {
        c.Row(func(r *builder.RowBuilder) {
            r.Col(8, func(col *builder.ColBuilder) {
                col.Text("Main content area")
            })
            r.Col(4, func(col *builder.ColBuilder) {
                col.Text("Sidebar", builder.TextColor(builder.Gray))
            })
        })
    })
    page.Footer(func(f *builder.Container) {
        f.PageNumber(builder.PageNum+" of "+builder.TotalPages,
            builder.AlignCenter(), builder.FontSize(8))
    })
})

pdfBytes, err := doc.Build()
```

**Key concepts**:
- `Builder` — document entry point, holds document-level config
- `PageBuilder` — defines Header/Content/Footer zones for a logical page
- `Container` — universal content receiver (text, rows, images, lines, spacers)
- `RowBuilder` / `ColBuilder` — 12-column grid with proportional sizing
- `Value` — unit-aware dimension type (`Mm`, `Cm`, `In`, `Pt`, `Pct`, `Fr`)
- `Color` — RGB color type with 13 predefined constants and `Hex()` parser
- All errors are accumulated and returned as a joined error from `Build()`

**Dependency rule**: `builder/` imports `layout/` and `creator/`. It never exposes `layout/` types in its public API — users work exclusively with `builder.Value`, `builder.Color`, `builder.Size`.

### Layer 2: layout/ — Pure Computation Engine

The `layout` package is a zero-dependency layout engine. It takes a tree of `Element` values, computes their positions, and produces positioned `Block` values ready for rendering. It has no knowledge of PDF format, file I/O, or font loading.

**Core types**:
- `Element` — interface with `PlanLayout(Area) Plan`
- `Plan` — result of layout: status (Full/Partial/Nothing), consumed height, list of Blocks
- `Block` — a positioned rectangle with a `Draw(Renderer)` callback
- `Renderer` — interface for drawing primitives (text, lines, images, rectangles)
- `Paginator` — walks `PageDef` elements, splits content across physical pages
- `FontResolver` — interface for font metric queries (standard 14 + TTF)
- `Style` — typography (font family, size, weight, color, alignment, line height)

**Pagination algorithm**:
1. For each `PageDef`, render header and footer into fixed zones.
2. Feed content elements to `PlanLayout` with the available body area.
3. When an element returns `Nothing` (does not fit), flush the current page and continue on a new one.
4. `KeepTogether` boxes are pushed whole to the next page if they do not fit.
5. After all pages are resolved, a second pass replaces `PageNum`/`TotalPages` placeholders with actual values.

### Layer 1: creator/ — PDF Primitives

The `creator` package writes PDF objects, content streams, and cross-reference tables. It is the rendering backend for both the Builder API and direct low-level use.

```go
c := creator.New()
c.SetTitle("Annual Report")

page, _ := c.NewPage()
page.AddText("Hello, World!", 100, 700, creator.Helvetica, 12)
page.DrawRectangle(100, 600, 200, 50, &creator.RectangleOptions{
    FillColor: &creator.Blue,
})

c.WriteToFile("output.pdf")
```

**Features**:
- Text rendering with Standard 14 fonts
- Graphics (lines, rectangles, circles, polygons, arcs, Bezier curves)
- Linear and radial gradients (PDF Shading Type 2/3 with multi-stop support)
- JPEG/PNG image embedding
- Tables with merged cells
- Chapters with auto-generated TOC
- Headers and footers
- Annotations (sticky notes, highlights, stamps)
- Interactive forms (text fields, checkboxes, dropdowns)
- Unicode metadata (Info dictionary, annotations, form fields, signatures — UTF-16BE per ISO 32000-1 §7.9.2)
- RC4/AES encryption
- Watermarks

## Component Overview

### Public API Layer

#### Main Entry Point (`gxpdf.go`)

```go
// Open and extract from existing PDFs
doc, _ := gxpdf.Open("document.pdf")
tables := doc.ExtractTables()
text := doc.Page(0).Text()
```

#### Export API (`export/`)

```go
exporter := export.NewCSVExporter()
exporter.Export(table, writer)

jsonExp := export.NewJSONExporter()
jsonExp.Export(table, writer)

excelExp := export.NewExcelExporter()
excelExp.Export(tables, "output.xlsx")
```

### Internal Layer

#### Document Model (`internal/document/`)

Core document structure:

- `Document` - PDF document with pages, metadata
- `Page` - Single page with dimensions, resources
- `PageSize` - Predefined sizes (A4, Letter, etc.)

#### Parser (`internal/parser/`)

PDF file parsing infrastructure:

- **Lexer** - Tokenizes PDF byte stream
- **Object Parser** - Parses PDF objects (arrays, dicts, streams)
- **XRef Parser** - Cross-reference table handling
- **Stream Parser** - Content stream parsing

#### Encoding (`internal/encoding/`)

Stream compression/decompression:

| Codec | Description | Status |
|-------|-------------|--------|
| FlateDecode | zlib compression (most common) | Implemented |
| DCTDecode | JPEG image data | Implemented |
| ASCII85Decode | ASCII encoding | Planned |
| ASCIIHexDecode | Hexadecimal encoding | Planned |
| LZWDecode | LZW compression (legacy) | Planned |

**Note**: FlateDecode and DCTDecode cover 95%+ of PDF files. Legacy codecs planned for v0.2.0.

#### Fonts (`internal/fonts/`)

Comprehensive font support:

**Standard 14 Fonts** (built-in, no embedding required):
- Helvetica family (Regular, Bold, Oblique, BoldOblique)
- Times family (Roman, Bold, Italic, BoldItalic)
- Courier family (Regular, Bold, Oblique, BoldOblique)
- Symbol, ZapfDingbats

**TrueType/OpenType Fonts** (embedded):
- TTF/OTF file parsing
- Font metrics extraction (Ascender, Descender, CapHeight, etc.)
- Glyph width calculation
- Font subsetting
- ToUnicode CMap generation for text extraction

**Key Types**:
```go
// Standard 14 font (built-in PDF fonts, no embedding required)
type Standard14Font struct {
    Name       string  // PostScript name (e.g., "Helvetica", "Times-Roman")
    Family     string  // Font family (e.g., "Helvetica", "Times")
    Weight     string  // Weight (e.g., "Regular", "Bold")
    Style      string  // Style (e.g., "Normal", "Oblique", "Italic")
    IsSymbolic bool    // True for Symbol/ZapfDingbats fonts
}

// Parsed TrueType/OpenType font (all fields)
type TTFFont struct {
    FilePath           string              // Path to font file
    PostScriptName     string              // PostScript name from name table
    Tables             map[string]*TTFTable // All parsed tables
    UnitsPerEm         uint16              // Units per em (typically 1000 or 2048)

    // Glyph data
    CharToGlyph        map[rune]uint16     // Unicode → glyph ID
    GlyphWidths        map[uint16]uint16   // Glyph ID → advance width
    FontData           []byte              // Raw data for embedding

    // From head table
    FontBBox           [4]int16            // Bounding box [xMin, yMin, xMax, yMax]

    // From hhea table
    Ascender           int16               // Typographic ascender
    Descender          int16               // Typographic descender (negative)
    LineGap            int16               // Line gap

    // From post table
    ItalicAngle        float64             // Italic angle in degrees
    UnderlinePosition  int16               // Underline position
    UnderlineThickness int16               // Underline thickness
    IsFixedPitch       bool                // True if monospaced

    // From OS/2 table
    CapHeight          int16               // Height of capital letters
    XHeight            int16               // Height of lowercase x
    WeightClass        uint16              // Weight (100-900)
    WidthClass         uint16              // Width class (1-9)
    FSType             uint16              // Embedding licensing rights
    TypoAscender       int16               // OS/2 typographic ascender
    TypoDescender      int16               // OS/2 typographic descender

    // Derived values
    StemV              int16               // Vertical stem width (estimated)
    Flags              uint32              // PDF font flags bitmap
}

// PDF FontDescriptor (metrics for embedded fonts)
type FontDescriptor struct {
    FontName     string   // PostScript name
    Flags        uint32   // Font flags (FixedPitch, Serif, Italic, etc.)
    FontBBox     [4]int   // Bounding box in PDF units (1000/em)
    ItalicAngle  float64  // Italic angle
    Ascent       int      // Ascender in PDF units
    Descent      int      // Descender in PDF units (negative)
    CapHeight    int      // Cap height in PDF units
    StemV        int      // Vertical stem width
    XHeight      int      // X-height in PDF units
    Leading      int      // Line spacing
    FontFile2Ref int      // Object reference to embedded font stream
}
```

#### Writer (`internal/writer/`)

PDF file generation:

- **PdfWriter** - Main PDF file writer
- **ContentStreamWriter** - PDF content stream generation
- **ResourceDictionary** - Font/image resource management
- **TrueTypeFontWriter** - Embedded font object generation
- **Stream Compression** - FlateDecode compression

**TrueType Font Embedding** generates:
1. Font dictionary (`/Type /Font /Subtype /TrueType`)
2. FontDescriptor (`/Type /FontDescriptor`)
3. ToUnicode CMap stream (for text extraction)
4. FontFile2 stream (compressed font data)

#### Security (`internal/security/`)

PDF encryption support:

| Algorithm | Key Length | Status |
|-----------|------------|--------|
| RC4 | 40-bit | Full |
| RC4 | 128-bit | Full |
| AES | 128-bit | Full |
| AES | 256-bit | Full |

#### Table Detection (`internal/tabledetect/`)

**4-Pass Hybrid Algorithm** for universal table extraction:

1. **Pass 1: Gap Detection** - Adaptive vertical gap threshold
2. **Pass 2: Overlap Detection** - Tabula-inspired line grouping
3. **Pass 3: Alignment Detection** - Geometric column clustering
4. **Pass 4: Multi-line Cell Merger** - Amount-based row discrimination

**Merged Cell Detection** (Lattice mode):

Grid Gap Analysis algorithm detects cells spanning multiple rows or columns by scanning for absent interior ruling lines within each column/row range. `HLineIndex`/`VLineIndex` spatial indexes provide O(1) line lookup. Owner cells receive correct `RowSpan`/`ColSpan`; covered cells remain as empty placeholders (backward compatible). Public API: `Table.CellAt(row, col)` returns `*CellInfo` with span metadata.

**Key Innovation**: Amount-based discrimination
```go
// Works universally across all bank formats
isTransactionRow := hasAmount(row)   // Has monetary amount = transaction
isContinuation := !hasAmount(row)    // No amount = continuation text
```

**Results**: 100% accuracy on bank statements (740/740 transactions)

## Design Principles

### 1. Simple Public API

Hide complexity behind intuitive interfaces:

```go
// User sees simple API
doc, _ := gxpdf.Open("file.pdf")
tables := doc.ExtractTables()

// Complex 4-pass algorithm hidden inside
```

### 2. Internal Privacy

`internal/` enforces API boundaries:

- External code cannot import `internal/`
- Free to refactor without breaking users
- Clear separation of public and private

### 3. Rich Domain Model

Objects with behavior, not just data containers:

```go
// Rich model example
type Page struct {
    dimensions Rectangle
    content    ContentStream
    resources  *Resources
}

func (p *Page) AddText(text string, pos Position, font *Font) error {
    // Validation and business logic in domain entity
    if err := p.validatePosition(pos); err != nil {
        return err
    }
    return p.content.AppendText(text, pos, font)
}
```

### 4. Functional Options

Configuration through options pattern:

```go
c := creator.New()
c.SetEncryption(creator.EncryptionOptions{
    UserPassword:  "user123",
    OwnerPassword: "owner456",
    Algorithm:     creator.EncryptionAES256,
    Permissions:   creator.PermissionPrint,
})
```

The builder API applies the same pattern at the document level:

```go
doc := builder.NewBuilder(
    builder.WithPageSize(builder.Letter),
    builder.WithDefaultFontSize(11),
    builder.WithAuthor("ACME Corp"),
)
```

### 5. Error Context

Errors with full context for debugging:

```go
if err != nil {
    return fmt.Errorf("parse xref at offset %d: %w", offset, err)
}
```

The builder accumulates errors and surfaces them all at once from `Build()`, so callers do not need to check errors at every content-adding step.

## Testing Strategy

### Test Types

```bash
# Unit tests
go test ./...

# Race detector
go test -race ./...

# Coverage
go test -cover ./...

# Benchmarks
go test -bench=. -benchmem ./...
```

### Coverage Targets

| Component | Target | Current |
|-----------|--------|---------|
| Parser | 80% | Done |
| Fonts | 80% | Done |
| Writer | 75% | Done |
| Creator | 70% | Done |
| layout/ | 80% | 81% |
| builder/ | 80% | 81% |

## Dependencies

**Production**: Standard library only (zero external dependencies)

**Testing**:
- `github.com/stretchr/testify` - Assertions

**Build**:
- Go 1.25+
- golangci-lint

## Performance Considerations

### Memory Efficiency

- Streaming PDF parsing (not loading entire file)
- Font subsetting (embed only used glyphs)
- Stream compression (FlateDecode)

### CPU Efficiency

- Table detection optimized for large documents
- Lazy evaluation where possible
- Efficient text encoding for embedded fonts

## Future Roadmap

### v0.7.0 "Builder & Signatures" (Done, unreleased)

- [x] Declarative Builder API (layout/ + builder/)
- [x] Enterprise Tables (ColSpan, RowSpan, header repeat, page split)
- [x] Rich Text (mixed-style inline text with baseline alignment)
- [x] Digital Signatures (PAdES B-B/B-T, CMS/PKCS#7, RSA + ECDSA, zero deps)
- [x] Text Measurement API (Standard 14 + TTF font metrics)
- [x] Coverage 80%+ on all new packages (layout 85.7%, builder 80.6%, signature 80.7%)

### v0.8.0 "Generation Platform" (Planned)

- [ ] HTML to PDF (separate module, uses Builder API)
- [ ] QR Code + Barcode (Code128, EAN-13)
- [ ] PDF/A compliance (A-1b, A-2b)
- [ ] PDF/UA accessibility (tagged PDF)
- [ ] Ready components (invoice, report, letter templates)
