# Security Policy

## Supported Versions

GxPDF is pre-1.0: security fixes go into the latest minor version, on `main`, and are
released as a patch. Older minors get no backports.

| Version | Supported          |
| ------- | ------------------ |
| 0.9.x   | :white_check_mark: |
| < 0.9   | :x:                |

## Reporting a Vulnerability

**Do not open a public issue for a security problem.** A public report leaves every
user exposed before a fix exists.

### How to report

**Preferred — GitHub private vulnerability reporting:**
https://github.com/coregx/gxpdf/security/advisories/new

The report stays private until an advisory is published. Use the draft advisory to send
the description and the reproducer, and to review the fix and the advisory text before
disclosure.

**Alternative — email:** `a.kolkov@gmail.com`, subject `gxpdf security`. Good if GitHub
is not an option for you; say so in the first message if you want an encrypted channel.

### What to include

- What the problem is and its impact (crash, memory exhaustion, infinite loop, code
  execution, information disclosure)
- A reproducer: the malformed PDF, or a short program that builds it
- Affected versions (`go list -m github.com/coregx/gxpdf`)
- A suggested fix, if you have one

### Response timeline

- **Initial reply**: within 72 hours
- **Assessment** (confirmed, severity, affected versions): within 7 days
- **Fix and disclosure**: a patch release and a GitHub security advisory, coordinated
  with you. We ask for 90 days before public disclosure, and we publish earlier if a fix
  ships and you agree, or if the issue is already public.

You are credited in the advisory unless you ask us not to be.

## Scope

**In scope** — anything that processes untrusted input:

- the parser and readers: `internal/parser`, `internal/extractor`, `pkg/`
- the writer and the creator API (`creator/`)
- the CLI (`cmd/`)
- crashes and resource exhaustion caused by a malicious PDF: unbounded memory, unbounded
  stack, quadratic or infinite loops. Availability problems on untrusted input are
  security issues here, not just bugs.

**Out of scope:**

- the `reference/` directory — third-party code, not part of the module or the release
- problems that need a trusted or already-validated file
- bugs in the caller's own code, and resource limits the caller is expected to set
  (see below)

## Security Considerations for Users

GxPDF parses files you do not control. Treat every PDF as hostile.

### Malicious PDF files

Crafted files can attack the parser: deep object graphs, reference cycles, absurd
`/Length` values, streams that expand without bound.

Mitigations in the library:

- recursion depth limits and cycle detection in object references
- a cap on each decode step (64 MiB) and on decoded image data (256 MiB)
- the whole `/Filter` chain is validated before decoding; unsupported filters are an
  error, not a silent pass-through
- fuzzing of the parser and the stream decoder

### Stream decompression (decompression bombs)

A stream can expand far beyond its file size. Each decode step is bounded; the bound
surfaces as an error, including through `GetImagesWithError`.

### Integer overflow

Object numbers, lengths and offsets come from the file. They are bounds-checked before
use; see the regression tests for `/Length` handling.

### Recommendations

```go
// Bound the work, not just the call: a hostile file may be slow rather than huge.
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

doc, err := pdf.NewReader("untrusted.pdf").ReadWithContext(ctx)
if err != nil {
    return fmt.Errorf("parsing PDF: %w", err)
}
```

```go
// Reject oversized input before parsing.
fi, err := os.Stat(path)
if err != nil {
    return err
}
if fi.Size() > maxPDFSize {
    return errors.New("PDF too large")
}
```

Always check errors — a returned error is the only limit signal you get.

## Security Testing

- table-driven tests for malformed input, with the malformed bytes checked in
- fuzz targets: `FuzzOpenFromBytes`, `FuzzStreamDecodeNeverExceedsLimit`,
  `FuzzLoadImageFromReader`, `FuzzTableRowSpans`, `FuzzSplitWord`
- the race detector in CI (Linux, macOS, Windows)
- `golangci-lint` on every push

## Dependencies

Production code depends on `spf13/cobra` (CLI), `xuri/excelize/v2` (XLSX input for
conversion) and `stretchr/testify` (tests). Everything else in `go.mod` is indirect.
Dependency versions are reviewed by hand when they change; there is no automated
dependency bot on this repository.

## Thanks

Thank you for helping keep GxPDF and its users safe.
