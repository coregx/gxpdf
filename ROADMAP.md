# GxPDF Roadmap

Strategic development plan for the GxPDF PDF library.

**Current Version**: v0.2.0 (in development)

## Version History

### v0.2.0 "Graphics Revolution" (In Development)

**Target**: February 2026

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

## Planned Features

### v0.3.0 - Digital Signatures

- **Sign PDFs** - Apply digital signatures
- **Verify Signatures** - Validate existing signatures
- **Certificate Support** - PKCS#12, X.509
- **Timestamp Support** - TSA integration

### v0.4.0 - PDF/A Compliance

- **PDF/A-1b** - Basic archival compliance
- **PDF/A-2b** - Extended archival compliance
- **Validation** - Check compliance
- **Conversion** - Convert existing PDFs to PDF/A

### v0.5.0 - Advanced Features

- **SVG Import** - Convert SVG to PDF graphics
- **Barcode Generation** - QR codes, Code128, etc.
- **Advanced Fonts** - Font subsetting optimization
- **Linearization** - Fast web view support

### v1.0.0 - Stable Release

- API stability guarantee
- Performance optimization
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
| Digital Signatures | Planned | v0.3.0 |
| PDF/A Compliance | Planned | v0.4.0 |

## Architecture

GxPDF uses Domain-Driven Design (DDD):

```
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
