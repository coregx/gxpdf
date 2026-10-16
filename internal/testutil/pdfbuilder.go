// Package testutil provides helpers shared by tests across internal packages.
//
// It is not imported by production code.
package testutil

import (
	"bytes"
	"fmt"
)

// BuildPDF assembles a minimal, structurally valid PDF from object bodies.
//
// objects[i] becomes indirect object i+1 ("i+1 0 obj … endobj"); object 1
// must be the document catalog. Byte offsets in the cross-reference table are
// computed, so tests can describe hostile or unusual object graphs (cycles,
// shared nodes, inconsistent /Count …) without hand-maintaining offsets.
func BuildPDF(objects ...string) []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.7\n")
	offsets := make([]int, len(objects))
	for i, body := range objects {
		offsets[i] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return buf.Bytes()
}

// Stream formats a stream object body with a correct /Length.
// extraDict is inserted into the stream dictionary verbatim (may be empty).
func Stream(extraDict, content string) string {
	return fmt.Sprintf("<< %s /Length %d >>\nstream\n%s\nendstream", extraDict, len(content), content)
}
