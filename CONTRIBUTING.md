# Contributing to GxPDF

Thank you for considering contributing to GxPDF! We welcome contributions of all kinds.

## Quick Start

```bash
# 1. Fork on GitHub, then clone
git clone https://github.com/YOUR_USERNAME/gxpdf.git
cd gxpdf

# 2. Create branch
git checkout -b feat/my-feature

# 3. Make changes, then run checks
go fmt ./...
go test ./...
golangci-lint run

# 4. Commit and push
git add .
git commit -m "feat: add my feature"
git push origin feat/my-feature

# 5. Open Pull Request on GitHub
```

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [How to Contribute](#how-to-contribute)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)
- [Coding Standards](#coding-standards)
- [Architecture Guidelines](#architecture-guidelines)

## Code of Conduct

This project follows a professional Code of Conduct. By participating, you agree to:

- Be respectful and professional
- Provide constructive feedback
- Focus on the code, not the person

## How to Contribute

### Found a Bug?

1. Check if it's already reported in [Issues](https://github.com/coregx/gxpdf/issues)
2. If not, [open a new issue](https://github.com/coregx/gxpdf/issues/new/choose)
3. Include: Go version, GxPDF version, OS, and minimal reproduction code

### Have an Idea?

1. Check [Discussions](https://github.com/coregx/gxpdf/discussions) and [Issues](https://github.com/coregx/gxpdf/issues)
2. For small changes — open an Issue
3. For large features — start a Discussion first

### Want to Code?

1. Look for issues labeled [`good first issue`](https://github.com/coregx/gxpdf/labels/good%20first%20issue) or [`help wanted`](https://github.com/coregx/gxpdf/labels/help%20wanted)
2. Comment "I'd like to work on this" to avoid duplicate work
3. Fork, code, test, submit PR

## Development Setup

### Prerequisites

- **Go 1.26+** (required)
- **Git**
- **golangci-lint** (recommended)

### Install Tools

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Clone and setup
git clone https://github.com/YOUR_USERNAME/gxpdf.git
cd gxpdf
git remote add upstream https://github.com/coregx/gxpdf.git

# Download dependencies
go mod download

# Verify setup
go test ./...
golangci-lint run
```

## Making Changes

### 1. Sync with Upstream

```bash
git fetch upstream
git checkout main
git merge upstream/main
```

### 2. Create a Branch

```bash
git checkout -b feat/your-feature-name
# or
git checkout -b fix/your-bug-fix
```

**Branch naming**:
- `feat/` — New features
- `fix/` — Bug fixes
- `docs/` — Documentation
- `refactor/` — Code refactoring
- `test/` — Adding tests

### 3. Make Your Changes

- Follow [Coding Standards](#coding-standards)
- Follow [Architecture Guidelines](#architecture-guidelines)
- Write tests for new functionality
- Update documentation as needed

### 4. Run Checks (Required!)

```bash
go fmt ./...           # Format code
go vet ./...           # Static analysis
go test ./...          # Run tests
go test -race ./...    # Race detector
golangci-lint run      # Linter
```

**All checks must pass before submitting PR.**

## Testing

### Writing Tests

Use **table-driven tests** (Go best practice):

```go
func TestRectangle_Dimensions(t *testing.T) {
    tests := []struct {
        name  string
        rect  Rectangle
        wantW float64
        wantH float64
    }{
        {
            name:  "A4 size",
            rect:  MustRectangle(0, 0, 595, 842),
            wantW: 595,
            wantH: 842,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := tt.rect.Width(); got != tt.wantW {
                t.Errorf("Width() = %v, want %v", got, tt.wantW)
            }
        })
    }
}
```

### Running Tests

```bash
go test ./...                              # All tests
go test -cover ./...                       # With coverage
go test -race ./...                        # Race detector
go test ./internal/infrastructure/parser/  # Specific package
```

## Submitting Changes

### 1. Commit Your Changes

Follow **Conventional Commits**:

```
<type>: <subject>

<body>
```

**Types**:
| Type | Description |
|------|-------------|
| `feat:` | New feature |
| `fix:` | Bug fix |
| `docs:` | Documentation |
| `refactor:` | Code refactoring |
| `test:` | Adding tests |
| `chore:` | Maintenance |

**Examples**:
```bash
git commit -m "feat: add PDF/A validation support"
git commit -m "fix: handle empty pages in merger"
git commit -m "docs: update API examples"
```

### 2. Push and Create PR

```bash
git push origin feat/your-feature-name
```

Then [create a Pull Request](https://github.com/coregx/gxpdf/compare) with:
- Clear description of changes
- Link to related issue: `Fixes #123`
- Confirmation that tests pass

### 3. Code Review

- Maintainers will review your PR
- Address feedback in new commits
- Once approved, maintainer will merge

## Coding Standards

### Go Style

Follow [Effective Go](https://go.dev/doc/effective_go) and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).

### Naming Conventions

```go
// Types — PascalCase
type Document struct { ... }
type PdfReader struct { ... }

// Interfaces — -er suffix when possible
type Parser interface { ... }
type Encoder interface { ... }

// Private — camelCase
type Document struct {
    id      DocumentID
    version Version
}

// Constants — PascalCase for exported
const MaxPageSize = 14400
```

### Error Handling

```go
// GOOD: Wrap errors with context
if err := parser.Parse(); err != nil {
    return fmt.Errorf("parse PDF at offset %d: %w", offset, err)
}

// BAD: Lose context
if err := parser.Parse(); err != nil {
    return err
}
```

### Comments

```go
// Parse parses a PDF file from the given reader.
// It returns a Document or an error if parsing fails.
func Parse(r io.Reader) (*Document, error) { ... }
```

## Architecture Guidelines

GxPDF follows **Domain-Driven Design (DDD)** principles.

### Layer Structure

```
internal/
├── domain/              # Pure business logic (NO external deps)
├── application/         # Use cases (orchestrates domain)
└── infrastructure/      # Technical implementation (parser, writer)

pkg/                     # Public API
creator/                 # High-level creation API
```

### Dependency Rules

```
domain/         → NO dependencies (pure Go only)
application/    → depends on domain/
infrastructure/ → depends on domain/
pkg/            → depends on application/ + domain/
```

### Rich Domain Model

**Prefer behavior over data**:

```go
// BAD: Anemic model
type Page struct {
    Width  float64
    Height float64
}

// GOOD: Rich model with behavior
type Page struct {
    dimensions Rectangle
    content    ContentStream
}

func (p *Page) AddText(text string, pos Position, font *Font) error {
    return p.content.AppendText(text, pos, font)
}
```

## What to Contribute

### Good First Issues

Look for [`good first issue`](https://github.com/coregx/gxpdf/labels/good%20first%20issue) label:
- Documentation improvements
- Adding tests
- Small bug fixes
- Adding examples

### Areas That Need Help

- Digital signatures (v0.6.0)
- AES-256 encrypted PDF reading (V=5/R=6)
- PDF/A compliance
- Performance optimization
- Documentation and examples
- Test coverage

## Questions?

- **Questions**: [GitHub Discussions](https://github.com/coregx/gxpdf/discussions)
- **Bugs/Features**: [GitHub Issues](https://github.com/coregx/gxpdf/issues)

## License

By contributing to GxPDF, you agree that your contributions will be licensed under the MIT License.

---

**Thank you for contributing to GxPDF!**
