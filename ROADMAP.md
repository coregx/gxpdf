# GxPDF Roadmap

Strategic development plan for the GxPDF PDF library.

**Current Version**: See [CHANGELOG.md](CHANGELOG.md)

## Version History

### v0.3.0 "Parser Hardening"

**Released**: February 2026

Major parser robustness improvements and rendering fixes:

#### New: Logging Package
- slog-based configurable logging (silent by default)
- Error visibility for convenience methods (ExtractText, etc.)

#### Image & Watermark Rendering (Writer)
- Complete image XObject rendering (JPEG + PNG + alpha)
- Watermark rendering with rotation and opacity
- Fixes #36: DrawImage/DrawImageFit now work correctly

#### Error Propagation
- Public API methods properly return/log errors instead of silently failing
- ExtractTextFromPage returns actual errors

#### Parser Hardening (11 community PRs by @mikeschinkel)
- Leading whitespace, CR line endings, trailing garbage after %%EOF
- CMap uint16 infinite loop fix (DoS vulnerability)
- PNG predictor support for xref streams (all 5 filter types)
- Progressive xref stream buffer, /W [0 0 0] support
- Off-by-one xref object recovery with lenient parsing

### v0.2.0 "Graphics Revolution"

**Released**: February 2026

Major graphics and forms capabilities:

#### Skia-like Graphics API (for GoGPU/gg integration)
- Alpha channel support with transparency
- Push/Pop graphics state stack
- Fill/Stroke separation with Paint interface
- Path Builder API (MoveTo, LineTo, CubicTo, etc.)
- Linear and Radial gradients
- ClipPath support

#### Forms API
- Form field reading (GetFormFields, GetFieldValue)
- Form field writing (SetFieldValue with validation)
- Form flattening (FlattenForm, FlattenFields)

#### Platform Support
- WASM API (WriteTo, Bytes for in-memory generation)

### v0.1.1

**Released**: January 2026

Unicode font embedding infrastructure:
- Full Unicode support (Cyrillic, CJK, symbols)
- TrueType font subsetting with ToUnicode CMap
- Type 0 Composite Font for full Unicode range
- Enterprise showcase PDF demonstrating all features
- Fixed PostScriptName parsing for proper font rendering

### v0.1.0

**Released**: January 2026

Full-featured PDF library with:
- PDF creation (Creator API)
- PDF reading and parsing
- Text and table extraction
- Multiple export formats
- DDD architecture

### v0.4.0 "Creator API"

**Released**: February 2026

Page sizes, custom dimensions, landscape orientation, and text rotation:

#### 35+ Built-in Page Sizes
- ISO A/B/C, ANSI, photo, book, JIS, envelopes, slides
- Map-based architecture for maintainability

#### Custom Page Dimensions (#41)
- `NewPageWithDimensions(widthPt, heightPt)` for arbitrary sizes
- Unit conversion helpers: `InchesToPoints`, `MMToPoints`, `CMToPoints` + reverse

#### Landscape Orientation (#41)
- `NewPageWithSize(size, Landscape)` — industry-standard swapped-MediaBox

#### Text Rotation (#42)
- `AddTextRotated` / `AddTextColorRotated` — standard 14 + custom TTF/OTF fonts
- Angle normalization to [0, 360)

### v0.5.0 "Opacity & Bezier"

**Released**: February 2026

#### Shape Opacity Fix (#47)
- Fixed 3-layer pipeline gap: opacity now propagated through writer

#### Quadratic Bezier Curves (#45)
- `DrawQuadBezierCurve` with `QuadBezierSegment` struct
- Exact degree elevation to cubic (not approximation)

#### Text Opacity (#46)
- `AddTextColorAlpha`, `AddTextColorRotatedAlpha` + custom font variants
- ExtGState transparency via `/ca` and `/CA` keys

### v0.6.0 "Encrypted Reading & Gradients"

**Released**: February 2026

#### Encrypted PDF Reading (#34)
- Standard Security Handler: RC4-40, RC4-128, AES-128
- `OpenWithPassword()` / `Open()` with transparent empty-password handling
- `ErrPasswordRequired` sentinel error

#### Full Gradient Rendering (#57)
- PDF Shading Type 2 (axial) + Type 3 (radial), multi-stop
- Clip+Shade technique on all shape types

#### ExtGState Object Creation Fix (#46, #47)
- Shape and text opacity now produce valid PDF output

## Current Development

### v0.8.2

**Released**: May 2026

- **Per-glyph text merging** (#74) — wkhtmltopdf single-Tj-per-glyph output now merged into words via spatial gap detection
- **Form XObject extraction** (#74) — TCPDF content inside `Do` Form XObjects now extracted with resource stack and recursion guard
- Real-world CID TrueType test fixtures from @iv7dev

### v0.8.1

**Released**: May 2026

- **CID TrueType text extraction fix** (#74, @iv7dev) — `ExtractTextFromPage()` now correctly decodes CID TrueType fonts with Identity-H encoding (wkhtmltopdf, TCPDF, CJK generators)
- Fixed: `beginbfrange` array format, `begincodespacerange` parsing, UTF-16BE surrogate pairs, composite font 2-byte detection

### v0.8.0 "Extraction & Access"

**Released**: May 2026

Community-driven release — all three extraction features requested by @joa23:

#### Positioned Text Extraction (#68)
- `Page.ExtractTextElements()` — text runs with X, Y, Width, Height, FontName, FontSize
- `Document.ExtractTextElementsFromPage()` — 1-based convenience method
- Essential for layout analysis, content reflow, and accessibility tooling

#### In-Memory PDF Opening (#68)
- `OpenFromBytes()`, `OpenFromBytesWithPassword()` — no filesystem I/O
- Context-aware variants for server-side workflows
- Internal Reader refactored from `*os.File` to `io.ReadSeeker`

#### Embedded Font Extraction (#67)
- `Document.GetEmbeddedFonts()` — extract TTF/OTF font binaries
- TrueType + Type0/CIDFontType2 support
- `fonts.LoadTTFFromBytes()` — round-trip back into Creator API

#### Vector Graphics Extraction (#66)
- `Document.GetVectorGraphics()` — paths, bezier curves, colors, opacity
- Path verb + coordinates model compatible with [gogpu/gg](https://github.com/gogpu/gg)
- Graphics state stack (`q`/`Q`), CTM tracking, CMYK→RGB
- Stroke, fill, fill-stroke paint modes with separate colors

#### Bug Fix
- uint16 overflow in `FontSubset.MeasureString` (#69, @yurikilian)

### v0.7.0 "Builder & Signatures"

**Released**: March 2026

- **Declarative Builder API** — QuestPDF-inspired layout with 12-col grid, auto-pagination
- **Enterprise Tables** — colspan, rowspan, header repeat, page split
- **Rich Text** — mixed-style inline text with baseline alignment, justify
- **Digital Signatures** — PAdES B-B + B-T, CMS/PKCS#7, RFC 3161, zero deps
- **Arc Drawing** (#59) — elliptical/circular arcs with wedge/chord fill modes
- **Text Measurement API** — exported font metrics (Standard 14 + TTF)

### Planned

#### v0.9.0 - "Scientific & Generation"

- **Scientific Paper Text Extraction** — two-column layout detection, reading order (ADR-001)
- **Scientific Metadata Extraction** — title, authors, abstract, DOI via font heuristics (ADR-002)
- **QR Code + Barcode** — QR, Code128, EAN-13 generation
- **PDF/A Compliance** — archival format (A-1b, A-2b)

#### v1.0.0 - "Full Platform"

- **HTML to PDF** — render HTML/CSS into PDF via Builder API
- **PDF/UA Accessibility** — tagged PDF for screen readers
- **Ready Components** — invoice, report, letter templates

#### v1.0.0 - Stable Release

- API stability guarantee
- Test coverage 80%+
- Comprehensive documentation
- Security audit

## Feature Status

| Feature | Status | Version |
|---------|--------|---------|
| PDF Creation | Done | v0.1.0 |
| Text Rendering | Done | v0.1.0 |
| Graphics (shapes, curves) | Done | v0.1.0 |
| Tables | Done | v0.1.0 |
| Images (JPEG, PNG) | Done | v0.1.0 |
| Fonts (Standard 14 + TTF) | Done | v0.1.0 |
| Unicode Font Embedding | Done | v0.1.1 |
| Chapters & TOC | Done | v0.1.0 |
| Annotations | Done | v0.1.0 |
| Interactive Forms | Done | v0.1.0 |
| Encryption (RC4, AES) | Done | v0.1.0 |
| Watermarks | Done | v0.1.0 |
| PDF Reading | Done | v0.1.0 |
| Text Extraction | Done | v0.1.0 |
| Table Extraction | Done | v0.1.0 |
| Export (CSV, JSON, Excel) | Done | v0.1.0 |
| Skia-like Graphics API | Done | v0.2.0 |
| Linear/Radial Gradients | Done | v0.2.0 |
| ClipPath Support | Done | v0.2.0 |
| Form Reading | Done | v0.2.0 |
| Form Filling | Done | v0.2.0 |
| Form Flattening | Done | v0.2.0 |
| WASM API | Done | v0.2.0 |
| Logging (slog) | Done | v0.3.0 |
| Image XObject Rendering | Done | v0.3.0 |
| Watermark Rendering | Done | v0.3.0 |
| Error Propagation | Done | v0.3.0 |
| Parser Hardening | Done | v0.3.0 |
| 35+ Page Sizes | Done | v0.4.0 |
| Custom Page Dimensions | Done | v0.4.0 |
| Landscape/Portrait | Done | v0.4.0 |
| Text Rotation | Done | v0.4.0 |
| Shape Opacity | Done | v0.5.0 |
| Quadratic Bezier Curves | Done | v0.5.0 |
| Text Opacity | Done | v0.5.0 |
| Gradient Rendering (PDF Shading) | Done | v0.6.0 |
| Encrypted PDF Reading | Done | v0.6.0 |
| Arc Drawing (elliptical/circular) | Done | v0.7.0 |
| Declarative Builder API | Done | v0.7.0 |
| Tables with ColSpan/RowSpan | Done | v0.7.0 |
| Rich Text (mixed inline styles) | Done | v0.7.0 |
| Digital Signatures (PAdES) | Done | v0.7.0 |
| Positioned Text Extraction | Done | v0.8.0 |
| In-Memory PDF Opening | Done | v0.8.0 |
| Embedded Font Extraction | Done | v0.8.0 |
| Vector Graphics Extraction | Done | v0.8.0 |
| Scientific Paper Text Extraction | Planned | v0.9.0 |
| HTML to PDF | Planned | v1.0.0 |
| PDF/A Compliance | Planned | v0.9.0 |
| PDF Render to Image | Planned | v1.0.0 |
| SVG Import | Planned | v1.0.0 |
| Barcode / QR Code | Planned | v0.9.0 |

## Backlog

| ID | Feature | Priority | Status | Description |
|----|---------|----------|--------|-------------|
| feat-067 | Custom Page Dimensions | **P1** | **Done** | Arbitrary page sizes (#41) |
| feat-068 | Text Rotation | **P1** | **Done** | Rotated text via Tm operator (#42) |
| feat-069 | Paper Sizes Expansion | **P1** | **Done** | 35+ built-in page sizes |
| fix-006 | Shape Opacity | **P1** | **Done** | Pipeline gap fix (#47) |
| feat-074 | Quadratic Bezier | **P2** | **Done** | Degree elevation to cubic (#45) |
| feat-075 | Text Opacity | **P2** | **Done** | ExtGState transparency (#46) |
| feat-078 | Gradient Rendering | **P1** | **Done** | Full PDF Shading (Type 2/3) for gradients (#57) |
| feat-042 | Encrypted PDF Reading | **P1** | **Done** | RC4/AES-128 with password support |
| feat-079 | Arc Drawing | **P2** | **Done** | Elliptical/circular arcs with wedge/chord (#59) |
| feat-076 | Declarative Builder API | **P1** | **Done** | QuestPDF-inspired layout, 12-col grid, auto-pagination |
| feat-077 | Tables ColSpan/RowSpan | **P1** | Backlog | Enterprise-grade table layout in builder |
| feat-081 | Rich Text | **P1** | Backlog | Mixed-style inline text elements |
| feat-037 | Digital Signatures | **P1** | Backlog | Sign and verify PDFs (CMS/PKCS#7 + PAdES) |
| feat-080 | HTML to PDF | **P2** | Backlog | Render WYSIWYG HTML via GxPDF |
| feat-062 | Fluent Text API | P3 | Backlog | Chainable text methods |
| feat-063 | Paragraph | P3 | Backlog | Multi-line text container |
| feat-064 | Y-Cursor | P3 | Backlog | Auto vertical positioning |
| feat-065 | Simple Table API | P3 | Backlog | Easy table creation |
| feat-066 | Shape Builders | P3 | Backlog | Fluent shape construction |
| feat-036 | SVG Import | P3 | Backlog | Vector graphics import |
| feat-039 | Invoice Template | P3 | Backlog | Pre-built invoice |
| feat-040 | Chart Integration | P3 | Backlog | Embed charts |
| feat-041 | PDF Render | P3 | Backlog | Render to images |

## Architecture

GxPDF uses Domain-Driven Design (DDD) with a three-layer generation stack:

```
builder/        — User-facing declarative API
layout/         — Pure computation layout engine (zero PDF deps)
creator/        — Low-level PDF primitives

internal/
├── domain/         # Pure business logic
├── application/    # Use cases
└── infrastructure/ # PDF parsing, encoding
```

See [ARCHITECTURE.md](docs/ARCHITECTURE.md) for details.

## Contributing

We welcome contributions! Priority areas:

- **Documentation** - Examples, tutorials
- **Tests** - Increase coverage
- **Performance** - Optimization
- **Features** - See planned features above

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Timeline

No fixed timelines. Features are released when ready and tested.

Priorities are based on:
1. User demand (GitHub issues)
2. Technical dependencies
3. Maintainer availability

## Feedback

Feature requests and feedback welcome:

- **GitHub Issues**: https://github.com/coregx/gxpdf/issues
- **Discussions**: https://github.com/coregx/gxpdf/discussions

---

*This roadmap is updated as priorities evolve.*
