# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-02-XX "Graphics Revolution"

### Added

#### Skia-like Graphics API (for GoGPU/gg integration)
- **Alpha Channel Support** - Transparency via ExtGState
  - `ColorRGBA` struct with alpha channel (0.0-1.0)
  - `Opacity` parameter on all drawing operations
  - ExtGState caching for efficient PDF output
  - 12 standard PDF blend modes
- **Push/Pop Graphics State** - Skia-like state stack
  - `Surface` type with state management
  - `PushTransform()`, `PushOpacity()`, `PushBlendMode()`
  - `Pop()` to restore previous state
  - `Transform` API: Translate, Scale, Rotate, Skew
- **Fill/Stroke Separation** - Independent fill and stroke
  - `Fill` struct: Paint, Opacity, FillRule (NonZero, EvenOdd)
  - `Stroke` struct: Paint, Width, LineCap, LineJoin, Dash
  - `SetFill()`, `SetStroke()` on Surface
  - LineCap: Butt, Round, Square
  - LineJoin: Miter, Round, Bevel
- **Paint Interface** - Unified color/gradient abstraction
  - `RGB()`, `RGBA()`, `Hex()`, `GrayN()` convenience functions
  - Color, ColorRGBA, ColorCMYK implement Paint
  - Ready for Gradient integration
- **Path Builder API** - Full vector path support
  - `NewPath()` with fluent API
  - `MoveTo()`, `LineTo()`, `CubicTo()`, `QuadraticTo()`, `Close()`
  - Shape helpers: `AddRect()`, `AddRoundedRect()`, `AddCircle()`, `AddEllipse()`, `AddArc()`
  - `DrawPath()`, `FillPath()`, `StrokePath()` on Surface
  - QuadraticTo automatically converts to cubic (PDF spec)

#### Forms API (Interactive PDF Forms)
- **Form Reading** - Read interactive form fields from PDFs
  - `Document.GetFormFields()` - Get all form fields
  - `Document.GetFieldValue(name)` - Get specific field value
  - `Document.HasForm()` - Check if PDF has interactive form
  - `FormField` type with accessors: Name, Type, Value, Options, Flags
  - Support for Text, Button, Choice, Signature field types
- **Form Writing** - Fill form fields programmatically
  - `Appender.SetFieldValue(name, value)` - Set field value
  - `Appender.GetFieldValue(name)` - Get current/pending value
  - Type validation (string for text, bool/string for checkboxes)
  - Option validation for choice fields
- **Form Flattening** - Convert forms to static content
  - `Appender.FlattenForm()` - Flatten all fields
  - `Appender.FlattenFields(names...)` - Flatten specific fields
  - `Appender.CanFlattenForm()` - Check if flattening is possible
- **WASM/Byte API** - Generate PDFs in memory
  - `Creator.WriteTo(io.Writer)` - Write to any writer
  - `Creator.Bytes()` - Get PDF as byte slice
  - `NewPdfWriterFromWriter(io.Writer)` - Low-level writer

#### Advanced Graphics
- **Linear Gradients** - Axial shading (ShadingType 2)
  - `NewLinearGradient(x1, y1, x2, y2)` constructor
  - `AddColorStop()` for color transitions
  - ExtendStart/ExtendEnd flags
- **Radial Gradients** - Radial shading (ShadingType 3)
  - `NewRadialGradient(x0, y0, r0, x1, y1, r1)` constructor
  - Focal point support (inner/outer circle)
- **ClipPath Support** - Clipping path operations
  - `PushClipPath()` with NonZero and EvenOdd fill rules
  - Convenience methods: `PushClipRect`, `PushClipCircle`, `PushClipEllipse`
  - PDF 1.7 Spec Section 8.5.4 compliant

---

## Planned (v0.3.0+)
- Digital signatures (sign and verify)
- PDF/A compliance
- Object streams (30% file size reduction)

---

## [0.1.1] - 2026-01-30

### Added
- **Full Unicode Font Embedding** - Complete TrueType/OpenType infrastructure
  - Cyrillic, CJK (Chinese, Japanese, Korean), and special symbols support
  - TTF parser extensions: `post`, `OS/2`, `name` table parsing
  - FontDescriptor generator with all PDF metrics
  - ToUnicode CMap generation for text extraction
  - Font subsetting with deterministic naming (XXXXXX+FontName)
  - Type 0 Composite Font support for full Unicode range
- **Text Clipping** - Clip text to table cell boundaries
- **Enterprise Showcase** - Professional 7-page PDF brochure demonstrating all features

### Fixed
- **hhea Table Parsing** - Corrected numOfLongHorMetrics offset for proper glyph widths
- **Glyph Width Calculation** - Fixed empty GlyphWidths map issue
- **PostScriptName Parsing** - Fixed UTF-16BE decoding in `name` table (was causing garbled font names and rendering issues in PDF viewers)

### Planned
- Form filling (fill existing PDF forms)
- Form flattening (convert forms to static content)
- Digital signatures (sign and verify)
- PDF/A compliance (archival format)
- SVG import

---

## [0.1.0] - 2026-01-07

Initial public release of GxPDF - a modern, enterprise-grade PDF library for Go.

### Added

#### PDF Creation (Creator API)
- **Document Creation** - Create PDF documents from scratch
- **Text Rendering** - Add text with multiple fonts, sizes, and colors
- **Graphics** - Draw lines, rectangles, circles, polygons, ellipses, Bezier curves
- **Gradients** - Linear and radial gradient fills
- **Color Spaces** - RGB and CMYK color support
- **Tables** - Create tables with borders, backgrounds, and merged cells
- **Images** - Embed JPEG and PNG images with transparency support
- **Fonts** - Standard 14 PDF fonts + TTF/OTF font embedding
- **Chapters & TOC** - Document structure with auto-generated Table of Contents
- **Annotations** - Sticky notes, highlights, underlines, stamps
- **Interactive Forms (AcroForm)** - Text fields, checkboxes, radio buttons, dropdowns, list boxes
- **Encryption** - RC4 (40/128-bit) and AES (128/256-bit) encryption
- **Watermarks** - Text watermarks with rotation, opacity, and positioning
- **Bookmarks** - PDF outline/navigation structure
- **Page Operations** - Merge, split, rotate, append pages

#### PDF Reading & Extraction
- **PDF Parser** - Read PDF 1.0-2.0 files
  - Cross-reference table parsing (traditional and stream-based)
  - Object and stream parsing with caching
  - Indirect reference resolution
- **Text Extraction** - Extract text with X,Y positions
  - Unicode support (including Cyrillic)
  - Font decoding (CMap, Identity-H)
  - Content stream parsing
- **Table Extraction** - Industry-leading accuracy
  - 4-Pass Hybrid Detection Algorithm
  - Lattice mode (ruling lines) + Stream mode (whitespace analysis)
  - Multi-line cell support
  - 100% accuracy on real-world bank statements
- **Image Extraction** - Extract embedded images
- **Export Formats** - CSV, JSON, Excel

#### Infrastructure
- **Stream Decoders** - FlateDecode, ASCII85Decode, ASCIIHexDecode
- **Thread Safety** - Object cache with sync.RWMutex
- **DDD Architecture** - Domain-Driven Design with Rich Domain Model

### Architecture
- **Domain Layer** - Pure business logic with no external dependencies
- **Application Layer** - Use cases and service orchestration
- **Infrastructure Layer** - PDF parsing, encoding, file I/O
- **Public API** - Clean, intuitive API with functional options pattern

### Testing
- Comprehensive unit tests
- Integration tests with real PDF files
- Race detector clean
- golangci-lint with 15+ linters: 0 issues

### Documentation
- Full API documentation (godoc)
- Code examples for all features
- Architecture documentation
- Contributing guidelines
- Security policy

---

## Project Information

**Repository**: https://github.com/coregx/gxpdf

**License**: MIT

**Go Version**: 1.25+

---

[Unreleased]: https://github.com/coregx/gxpdf/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/coregx/gxpdf/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/coregx/gxpdf/releases/tag/v0.1.0
