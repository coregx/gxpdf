//go:build ignore

// Generator for testdata/pdfs/stream_filters/*.pdf.
//
// Run from the repository root with:
//
//	go run testdata/generators/stream_filter_fixtures.go
package main

import (
	"bytes"
	"compress/zlib"
	"encoding/ascii85"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

type fixture struct {
	name       string
	filterDict string
	encode     func([]byte) []byte
}

func main() {
	raw := []byte("BT /F1 12 Tf 72 700 Td (Filtered fixture) Tj ET")
	fixtures := []fixture{
		{name: "unfiltered.pdf", encode: clone},
		{name: "flate.pdf", filterDict: "/Filter /FlateDecode", encode: flate},
		{name: "ascii85.pdf", filterDict: "/Filter /ASCII85Decode", encode: ascii85Encode},
		{name: "asciihex.pdf", filterDict: "/Filter /ASCIIHexDecode", encode: asciiHex},
		{name: "runlength.pdf", filterDict: "/Filter /RunLengthDecode", encode: runLength},
		{name: "lzw.pdf", filterDict: "/Filter /LZWDecode", encode: literalLZW},
		{
			name:       "ascii85_flate.pdf",
			filterDict: "/Filter [/ASCII85Decode /FlateDecode] /DecodeParms [null null]",
			encode:     func(data []byte) []byte { return ascii85Encode(flate(data)) },
		},
		{name: "malformed_ascii85.pdf", filterDict: "/Filter /ASCII85Decode", encode: func([]byte) []byte { return []byte("!!!!~x") }},
		{name: "unsupported_filter.pdf", filterDict: "/Filter /CCITTFaxDecode", encode: clone},
	}
	directory := filepath.Join("testdata", "pdfs", "stream_filters")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		panic(err)
	}
	for _, item := range fixtures {
		if err := os.WriteFile(filepath.Join(directory, item.name), buildPDF(item.encode(raw), item.filterDict), 0o644); err != nil {
			panic(err)
		}
	}
}

func clone(data []byte) []byte { return append([]byte(nil), data...) }

func flate(data []byte) []byte {
	var output bytes.Buffer
	writer := zlib.NewWriter(&output)
	if _, err := writer.Write(data); err != nil {
		panic(err)
	}
	if err := writer.Close(); err != nil {
		panic(err)
	}
	return output.Bytes()
}

func ascii85Encode(data []byte) []byte {
	encoded := make([]byte, ascii85.MaxEncodedLen(len(data)))
	written := ascii85.Encode(encoded, data)
	return append(encoded[:written], '~', '>')
}

func asciiHex(data []byte) []byte {
	encoded := make([]byte, hex.EncodedLen(len(data)))
	hex.Encode(encoded, data)
	return append(encoded, '>')
}

func runLength(data []byte) []byte {
	var encoded []byte
	for len(data) > 0 {
		count := min(len(data), 128)
		encoded = append(encoded, byte(count-1))
		encoded = append(encoded, data[:count]...)
		data = data[count:]
	}
	return append(encoded, 128)
}

func literalLZW(data []byte) []byte {
	var encoded []byte
	bitOffset := 0
	writeCode := func(code, width int) {
		for bit := width - 1; bit >= 0; bit-- {
			if bitOffset%8 == 0 {
				encoded = append(encoded, 0)
			}
			if code>>bit&1 != 0 {
				encoded[len(encoded)-1] |= 1 << (7 - bitOffset%8)
			}
			bitOffset++
		}
	}
	codeWidth := 9
	nextCode := 258
	writeCode(256, codeWidth)
	for index, value := range data {
		writeCode(int(value), codeWidth)
		if index == 0 || nextCode >= 4096 {
			continue
		}
		nextCode++
		if codeWidth < 12 && nextCode+1 == 1<<codeWidth {
			codeWidth++
		}
	}
	writeCode(257, codeWidth)
	return encoded
}

func buildPDF(content []byte, filterDictionary string) []byte {
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>"),
		streamObject(content, filterDictionary),
		[]byte("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"),
	}
	var output bytes.Buffer
	output.WriteString("%PDF-1.7\n%\xE2\xE3\xCF\xD3\n")
	offsets := make([]int, len(objects)+1)
	for index, object := range objects {
		offsets[index+1] = output.Len()
		fmt.Fprintf(&output, "%d 0 obj\n", index+1)
		output.Write(object)
		output.WriteString("\nendobj\n")
	}
	xref := output.Len()
	fmt.Fprintf(&output, "xref\n0 %d\n", len(objects)+1)
	output.WriteString("0000000000 65535 f\n")
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&output, "%010d 00000 n\n", offset)
	}
	fmt.Fprintf(&output, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return output.Bytes()
}

func streamObject(content []byte, filterDictionary string) []byte {
	var output bytes.Buffer
	fmt.Fprintf(&output, "<< /Length %d", len(content))
	if filterDictionary != "" {
		fmt.Fprintf(&output, " %s", filterDictionary)
	}
	output.WriteString(" >>\nstream\n")
	output.Write(content)
	output.WriteString("\nendstream")
	return output.Bytes()
}
