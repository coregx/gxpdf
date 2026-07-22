package signature

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"

	"github.com/coregx/gxpdf/internal/writer"
)

const (
	// contentsPlaceholderLen is the number of hex characters in the /Contents placeholder.
	// 32768 hex chars = 16384 raw bytes capacity — sufficient for RSA-4096 + cert chain + TSA token.
	contentsPlaceholderLen = 32768

	// byteRangeWidth is the fixed decimal width for each integer in /ByteRange.
	// Using fixed width ensures the placeholder and real value have identical byte lengths.
	byteRangeWidth = 10
)

// placeholderInfo records the byte offsets of the /ByteRange and /Contents fields
// within the assembled PDF so they can be patched after the CMS is computed.

// signResult is returned by buildSignedPDF and carries everything needed
// to hash the byte ranges and inject the final CMS signature.
type signResult struct {
	// pdf is the assembled PDF bytes including placeholder signature dict.
	pdf []byte

	// byteRange is the [offset0, len0, offset2, len2] byte ranges to hash.
	byteRange [4]int64

	// contentsOffset is the position of the first hex char inside /Contents <...>.
	contentsOffset int

	// contentsLength is the total length of the hex placeholder (contentsPlaceholderLen).
	contentsLength int
}

// buildSignedPDF appends an incremental update to pdfData containing a signature
// dictionary with /ByteRange and /Contents placeholders, then computes the actual
// ByteRange values and patches them in-place.
//
// The returned signResult carries the assembled bytes and the offsets needed to:
//  1. compute SHA-256 over the two byte ranges (everything except /Contents)
//  2. inject the hex-encoded CMS into the /Contents placeholder
func buildSignedPDF(pdfData []byte, cfg *signConfig) (*signResult, error) {
	// Validate that the input is a PDF.
	if !bytes.HasPrefix(pdfData, []byte("%PDF-")) {
		return nil, fmt.Errorf("signature: input does not start with %%PDF-")
	}

	// Parse the existing trailer to determine next object number and root ref.
	rootRef, lastXrefOffset, trailerSize, err := parseTrailerBasic(pdfData)
	if err != nil {
		return nil, fmt.Errorf("signature: parse trailer: %w", err)
	}

	// Assign object numbers for the new objects.
	sigDictObjNum := trailerSize
	sigFieldObjNum := trailerSize + 1
	nextSize := trailerSize + 2

	// Format signing time as PDF date string: D:YYYYMMDDHHmmSSOHH'mm'
	signTimeStr := cfg.signTime.UTC().Format("20060102150405+00'00'")

	var appendBuf bytes.Buffer
	appendBuf.WriteByte('\n')

	// --- Signature dictionary object ---
	sigObjOffset := len(pdfData) + appendBuf.Len()
	fmt.Fprintf(&appendBuf, "%d 0 obj\n", sigDictObjNum)
	appendBuf.WriteString("<< /Type /Sig")
	appendBuf.WriteString(" /Filter /Adobe.PPKLite")
	appendBuf.WriteString(" /SubFilter /adbe.pkcs7.detached")
	fmt.Fprintf(&appendBuf, " /M (D:%s)", signTimeStr)

	if cfg.reason != "" {
		fmt.Fprintf(&appendBuf, " /Reason %s", writer.EncodeTextString(cfg.reason))
	}
	if cfg.location != "" {
		fmt.Fprintf(&appendBuf, " /Location %s", writer.EncodeTextString(cfg.location))
	}
	if cfg.contactInfo != "" {
		fmt.Fprintf(&appendBuf, " /ContactInfo %s", writer.EncodeTextString(cfg.contactInfo))
	}

	// /ByteRange placeholder — fixed-width so patching does not shift offsets.
	byteRangePlaceholder := fmt.Sprintf("[%0*d %0*d %0*d %0*d]",
		byteRangeWidth, 0,
		byteRangeWidth, 0,
		byteRangeWidth, 0,
		byteRangeWidth, 0,
	)
	appendBuf.WriteString(" /ByteRange ")
	byteRangeOffset := len(pdfData) + appendBuf.Len()
	appendBuf.WriteString(byteRangePlaceholder)

	// /Contents placeholder — hex string of zeros.
	appendBuf.WriteString(" /Contents ")
	contentsOffset := len(pdfData) + appendBuf.Len() // points to '<'
	appendBuf.WriteByte('<')
	appendBuf.WriteString(strings.Repeat("0", contentsPlaceholderLen))
	appendBuf.WriteByte('>')

	appendBuf.WriteString(" >>\nendobj\n")

	// --- Signature field (widget annotation) ---
	sigFieldObjOffset := len(pdfData) + appendBuf.Len()
	fmt.Fprintf(&appendBuf, "%d 0 obj\n", sigFieldObjNum)
	fmt.Fprintf(&appendBuf, "<< /Type /Annot /Subtype /Widget /FT /Sig /T (Signature1) /V %d 0 R /F 132 /Rect [0 0 0 0] >>\n", sigDictObjNum)
	appendBuf.WriteString("endobj\n")

	// --- New cross-reference table ---
	xrefOffset := len(pdfData) + appendBuf.Len()
	appendBuf.WriteString("xref\n")
	fmt.Fprintf(&appendBuf, "%d 2\n", sigDictObjNum)
	fmt.Fprintf(&appendBuf, "%010d 00000 n \r\n", sigObjOffset)
	fmt.Fprintf(&appendBuf, "%010d 00000 n \r\n", sigFieldObjOffset)

	// --- New trailer ---
	appendBuf.WriteString("trailer\n")
	fmt.Fprintf(&appendBuf, "<< /Size %d /Root %d 0 R /Prev %d >>\n", nextSize, rootRef, lastXrefOffset)
	fmt.Fprintf(&appendBuf, "startxref\n%d\n%%%%EOF\n", xrefOffset)

	// Assemble: original PDF + incremental update.
	total := len(pdfData) + appendBuf.Len()
	result := make([]byte, total)
	copy(result, pdfData)
	copy(result[len(pdfData):], appendBuf.Bytes())

	// Compute actual ByteRange values.
	// The signed regions are everything EXCEPT the /Contents hex string (between < and >).
	// contentsOffset points to '<', so the first signed region ends just before it.
	contentsHexStart := contentsOffset + 1                        // first hex char after '<'
	contentsHexEnd := contentsOffset + 1 + contentsPlaceholderLen // position of '>'

	br := [4]int64{
		0,
		int64(contentsOffset),                  // length of first region: PDF header up to '<'
		int64(contentsHexEnd + 1),              // start of second region: after '>'
		int64(total) - int64(contentsHexEnd+1), // length of second region: rest of file
	}

	// Patch /ByteRange in-place with real values.
	brStr := fmt.Sprintf("[%0*d %0*d %0*d %0*d]",
		byteRangeWidth, br[0],
		byteRangeWidth, br[1],
		byteRangeWidth, br[2],
		byteRangeWidth, br[3],
	)
	copy(result[byteRangeOffset:], brStr)

	return &signResult{
		pdf:            result,
		byteRange:      br,
		contentsOffset: contentsHexStart,
		contentsLength: contentsPlaceholderLen,
	}, nil
}

// computeByteRangeHash computes the SHA-256 hash over the two signed byte ranges.
// The byte range format is [offset0, len0, offset2, len2] — two non-contiguous
// regions that together cover everything except the /Contents hex value.
func computeByteRangeHash(data []byte, br [4]int64) ([]byte, error) {
	h := sha256.New()

	end1 := br[0] + br[1]
	if end1 > int64(len(data)) {
		return nil, fmt.Errorf("signature: byte range [0..%d] exceeds data length %d", end1, len(data))
	}
	h.Write(data[br[0]:end1])

	end2 := br[2] + br[3]
	if end2 > int64(len(data)) {
		return nil, fmt.Errorf("signature: byte range [%d..%d] exceeds data length %d", br[2], end2, len(data))
	}
	h.Write(data[br[2]:end2])

	return h.Sum(nil), nil
}

// injectSignature hex-encodes the DER CMS signature and writes it into the
// /Contents placeholder at contentsOffset. The placeholder is filled with
// the hex-encoded signature followed by zero padding.
func injectSignature(data []byte, contentsOffset, contentsLength int, sig []byte) ([]byte, error) {
	hexSig := fmt.Sprintf("%X", sig)
	if len(hexSig) > contentsLength {
		return nil, fmt.Errorf("signature: CMS too large: %d hex chars, placeholder is %d", len(hexSig), contentsLength)
	}

	result := make([]byte, len(data))
	copy(result, data)

	// Write hex signature, padded with '0' to fill the placeholder.
	dst := result[contentsOffset : contentsOffset+contentsLength]
	copy(dst, hexSig)
	for i := len(hexSig); i < contentsLength; i++ {
		dst[i] = '0'
	}
	return result, nil
}

// --- Trailer parsing helpers ---

// parseTrailerBasic extracts /Root object number, last xref offset, and /Size
// from the PDF trailer using lightweight string scanning.
// This avoids pulling in the full PDF parser for a simple structural field read.
func parseTrailerBasic(data []byte) (rootRef, lastXrefOffset, trailerSize int, err error) {
	// Find "startxref" from the end (handles linearized PDFs with two startxref entries).
	idx := bytes.LastIndex(data, []byte("startxref"))
	if idx < 0 {
		return 0, 0, 0, fmt.Errorf("signature: startxref not found")
	}
	rest := strings.TrimSpace(string(data[idx+len("startxref"):]))
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return 0, 0, 0, fmt.Errorf("signature: no offset after startxref")
	}
	lastXrefOffset, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("signature: invalid startxref offset: %w", err)
	}

	// Find "trailer" dictionary from the end.
	trailerIdx := bytes.LastIndex(data, []byte("trailer"))
	if trailerIdx < 0 {
		return 0, 0, 0, fmt.Errorf("signature: trailer not found")
	}
	trailerStr := string(data[trailerIdx:])

	rootRef, err = parsePDFRef(trailerStr, "/Root")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("signature: parse /Root: %w", err)
	}

	trailerSize, err = parsePDFInt(trailerStr, "/Size")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("signature: parse /Size: %w", err)
	}

	return rootRef, lastXrefOffset, trailerSize, nil
}

// parsePDFRef extracts an indirect object number from a PDF reference like "/Key N G R".
func parsePDFRef(s, key string) (int, error) {
	idx := strings.Index(s, key)
	if idx < 0 {
		return 0, fmt.Errorf("%s not found", key)
	}
	parts := strings.Fields(strings.TrimSpace(s[idx+len(key):]))
	if len(parts) < 3 || parts[2] != "R" {
		return 0, fmt.Errorf("invalid reference after %s", key)
	}
	return strconv.Atoi(parts[0])
}

// parsePDFInt extracts an integer value from "/Key N" in a PDF dictionary string.
func parsePDFInt(s, key string) (int, error) {
	idx := strings.Index(s, key)
	if idx < 0 {
		return 0, fmt.Errorf("%s not found", key)
	}
	parts := strings.Fields(strings.TrimSpace(s[idx+len(key):]))
	if len(parts) == 0 {
		return 0, fmt.Errorf("no value after %s", key)
	}
	return strconv.Atoi(parts[0])
}
