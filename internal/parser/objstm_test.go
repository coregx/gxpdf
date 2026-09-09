package parser

import (
	"bytes"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

// TestParseObjectStream tests the ParseObjectStream method with various scenarios.
func TestParseObjectStream(t *testing.T) {
	tests := []struct {
		name        string
		decodedData string
		numObjects  int
		firstOffset int
		wantObjects map[int]string // object number -> expected type/value
		wantErr     bool
		errContains string
	}{
		{
			name: "simple object stream with single integer",
			decodedData: "10 0 " + // Header: obj 10 at offset 0
				"42", // Object 10: integer 42
			numObjects:  1,
			firstOffset: 5, // "10 0 " = 5 bytes
			wantObjects: map[int]string{
				10: "integer:42",
			},
			wantErr: false,
		},
		{
			name: "object stream with names",
			decodedData: "5 0 6 5 " + // Header
				"/Type " + // Object 5
				"/Page", // Object 6
			numObjects:  2,
			firstOffset: 8,
			wantObjects: map[int]string{
				5: "name:Type",
				6: "name:Page",
			},
			wantErr: false,
		},
		{
			name: "object stream with booleans",
			decodedData: "1 0 2 5 " + // Header
				"true " + // Object 1
				"false", // Object 2
			numObjects:  2,
			firstOffset: 8,
			wantObjects: map[int]string{
				1: "boolean:true",
				2: "boolean:false",
			},
			wantErr: false,
		},
		{
			name:        "invalid numObjects (zero)",
			decodedData: "1 0 ",
			numObjects:  0,
			firstOffset: 4,
			wantErr:     true,
			errContains: "invalid number of objects",
		},
		{
			name:        "invalid numObjects (negative)",
			decodedData: "1 0 ",
			numObjects:  -1,
			firstOffset: 4,
			wantErr:     true,
			errContains: "invalid number of objects",
		},
		{
			name:        "invalid firstOffset (negative)",
			decodedData: "1 0 42",
			numObjects:  1,
			firstOffset: -1,
			wantErr:     true,
			errContains: "invalid first offset",
		},
		{
			name:        "firstOffset beyond data length",
			decodedData: "1 0 42",
			numObjects:  1,
			firstOffset: 100,
			wantErr:     true,
			errContains: "invalid first offset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(io.NopCloser(bytes.NewReader([]byte(tt.decodedData))))
			_ = p // Parser created for context but ParseObjectStream is standalone

			objects, err := p.ParseObjectStream([]byte(tt.decodedData), tt.numObjects, tt.firstOffset)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseObjectStream() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("ParseObjectStream() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseObjectStream() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(objects) != len(tt.wantObjects) {
				t.Errorf("ParseObjectStream() got %d objects, want %d", len(objects), len(tt.wantObjects))
			}

			for objNum, expectedType := range tt.wantObjects {
				obj, ok := objects[objNum]
				if !ok {
					t.Errorf("ParseObjectStream() missing object %d", objNum)
					continue
				}

				gotType := getObjectTypeString(obj)
				if gotType != expectedType {
					t.Errorf("ParseObjectStream() object %d = %s, want %s", objNum, gotType, expectedType)
				}
			}
		})
	}
}

// TestParseObjectStream_Dictionary tests parsing dictionaries in object streams.
func TestParseObjectStream_Dictionary(t *testing.T) {
	// Object stream with a dictionary
	decodedData := "15 0 " + // Header: obj15 at offset 0
		"<< /Type /Page /Count 10 >>" // Object 15: dictionary

	p := NewParser(io.NopCloser(bytes.NewReader([]byte(decodedData))))
	objects, err := p.ParseObjectStream([]byte(decodedData), 1, 5)

	if err != nil {
		t.Fatalf("ParseObjectStream() error = %v", err)
	}

	obj, ok := objects[15]
	if !ok {
		t.Fatal("ParseObjectStream() missing object 15")
	}

	dict, ok := obj.(*Dictionary)
	if !ok {
		t.Fatalf("ParseObjectStream() object 15 is %T, want *Dictionary", obj)
	}

	// Verify dictionary contents
	typeObj := dict.GetName("Type")
	if typeObj == nil || typeObj.Value() != "Page" {
		t.Errorf("Dictionary /Type = %v, want 'Page'", typeObj)
	}

	count := dict.GetInteger("Count")
	if count != 10 {
		t.Errorf("Dictionary /Count = %d, want 10", count)
	}
}

// TestParseObjectStream_Array tests parsing arrays in object streams.
func TestParseObjectStream_Array(t *testing.T) {
	// Object stream with an array
	decodedData := "20 0 " + // Header
		"[ 1 2 3 4 5 ]" // Object 20: array

	p := NewParser(io.NopCloser(bytes.NewReader([]byte(decodedData))))
	objects, err := p.ParseObjectStream([]byte(decodedData), 1, 5)

	if err != nil {
		t.Fatalf("ParseObjectStream() error = %v", err)
	}

	obj, ok := objects[20]
	if !ok {
		t.Fatal("ParseObjectStream() missing object 20")
	}

	arr, ok := obj.(*Array)
	if !ok {
		t.Fatalf("ParseObjectStream() object 20 is %T, want *Array", obj)
	}

	if arr.Len() != 5 {
		t.Errorf("Array length = %d, want 5", arr.Len())
	}
}

// TestParseObjectStream_MultipleObjects tests parsing multiple objects with various types.
func TestParseObjectStream_MultipleObjects(t *testing.T) {
	// Create a realistic object stream
	// Offsets are calculated from firstOffset position
	obj30 := "42 "               // Integer (3 bytes)
	obj31 := "/FontName "        // Name (10 bytes)
	obj32 := "[ 1 2 3 ] "        // Array (10 bytes)
	obj33 := "<< /Key /Value >>" // Dictionary (18 bytes)

	// Calculate offsets relative to start of object data
	offset30 := 0
	offset31 := len(obj30)                 // 3
	offset32 := len(obj30 + obj31)         // 13
	offset33 := len(obj30 + obj31 + obj32) // 23

	// Build header with proper formatting
	header := fmt.Sprintf("30 %d 31 %d 32 %d 33 %d ", offset30, offset31, offset32, offset33)

	decodedData := header + obj30 + obj31 + obj32 + obj33
	firstOffset := len(header)

	p := NewParser(io.NopCloser(bytes.NewReader([]byte(decodedData))))
	objects, err := p.ParseObjectStream([]byte(decodedData), 4, firstOffset)

	if err != nil {
		t.Fatalf("ParseObjectStream() error = %v", err)
	}

	if len(objects) != 4 {
		t.Fatalf("ParseObjectStream() got %d objects, want 4", len(objects))
	}

	// Verify object 30 (integer)
	if obj, ok := objects[30]; !ok {
		t.Error("Missing object 30")
	} else if _, ok := obj.(*Integer); !ok {
		t.Errorf("Object 30 is %T, want *Integer", obj)
	}

	// Verify object 31 (name)
	if obj, ok := objects[31]; !ok {
		t.Error("Missing object 31")
	} else if _, ok := obj.(*Name); !ok {
		t.Errorf("Object 31 is %T, want *Name", obj)
	}

	// Verify object 32 (array)
	if obj, ok := objects[32]; !ok {
		t.Error("Missing object 32")
	} else if _, ok := obj.(*Array); !ok {
		t.Errorf("Object 32 is %T, want *Array", obj)
	}

	// Verify object 33 (dictionary)
	if obj, ok := objects[33]; !ok {
		t.Error("Missing object 33")
	} else if _, ok := obj.(*Dictionary); !ok {
		t.Errorf("Object 33 is %T, want *Dictionary", obj)
	}
}

// TestDecodeStream tests the decodeStream helper method indirectly through a mock.
func TestDecodeStream_FlateDecode(t *testing.T) {
	// Create test data
	originalData := []byte("This is test data for compression")

	// Compress with zlib
	var buf bytes.Buffer
	writer := zlib.NewWriter(&buf)
	_, err := writer.Write(originalData)
	if err != nil {
		t.Fatalf("Failed to compress test data: %v", err)
	}
	writer.Close()
	compressedData := buf.Bytes()

	// Create a stream with FlateDecode filter
	dict := NewDictionary()
	dict.Set("Filter", NewName("FlateDecode"))
	dict.SetInteger("Length", int64(len(compressedData)))
	stream := NewStream(dict, compressedData)

	// Create a minimal reader to test decodeStream
	reader := &Reader{
		objStmCache: make(map[int]map[int]PdfObject),
	}

	// Decode the stream
	decoded, err := reader.decodeStream(stream)
	if err != nil {
		t.Fatalf("decodeStream() error = %v", err)
	}

	if !bytes.Equal(decoded, originalData) {
		t.Errorf("decodeStream() = %q, want %q", decoded, originalData)
	}
}

func TestGetCompressedObjectWithContext(t *testing.T) {
	objectStream := []byte("5 0 obj\n<< /Type /ObjStm /N 1 /First 5 /Length 7 >>\nstream\n10 0 42\nendstream\nendobj")
	targetEntry := NewXRefEntry(10, XRefEntryCompressed, 5, 0)

	newReader := func() *Reader {
		xref := NewXRefTable()
		xref.AddEntry(NewXRefEntry(5, XRefEntryInUse, 0, 0))
		xref.AddEntry(targetEntry)
		return &Reader{
			src:         bytes.NewReader(objectStream),
			fileSize:    int64(len(objectStream)),
			xrefTable:   xref,
			objectCache: make(map[int]PdfObject),
			objStmCache: make(map[int]map[int]PdfObject),
		}
	}

	t.Run("decodes with active context", func(t *testing.T) {
		object, err := newReader().GetObjectWithContext(context.Background(), 10)
		if err != nil {
			t.Fatal(err)
		}
		integer, ok := object.(*Integer)
		if !ok || integer.Value() != 42 {
			t.Fatalf("object = %#v, want integer 42", object)
		}
	})

	t.Run("cancellation reaches object stream decode", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := newReader().getCompressedObjectWithContext(ctx, 10, targetEntry)
		if err == nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
	})

	t.Run("resolves indirect decode parameters without holding cache lock", func(t *testing.T) {
		decodedObjectStream := []byte("10 0 42")
		var compressed bytes.Buffer
		writer := zlib.NewWriter(&compressed)
		if _, err := writer.Write(decodedObjectStream); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}

		objectStream := []byte(fmt.Sprintf(
			"5 0 obj\n<< /Type /ObjStm /N 1 /First 5 /Length %d /Filter /FlateDecode /DecodeParms 6 0 R >>\nstream\n%s\nendstream\nendobj\n",
			compressed.Len(),
			compressed.Bytes(),
		))
		parametersOffset := int64(len(objectStream))
		source := append(objectStream, []byte("6 0 obj\n<< /Predictor 1 >>\nendobj")...)

		xref := NewXRefTable()
		xref.AddEntry(NewXRefEntry(5, XRefEntryInUse, 0, 0))
		xref.AddEntry(NewXRefEntry(6, XRefEntryInUse, parametersOffset, 0))
		xref.AddEntry(targetEntry)
		reader := &Reader{
			src:         bytes.NewReader(source),
			fileSize:    int64(len(source)),
			xrefTable:   xref,
			objectCache: make(map[int]PdfObject),
			objStmCache: make(map[int]map[int]PdfObject),
		}

		object, err := reader.GetObjectWithContext(context.Background(), 10)
		if err != nil {
			t.Fatal(err)
		}
		integer, ok := object.(*Integer)
		if !ok || integer.Value() != 42 {
			t.Fatalf("object = %#v, want integer 42", object)
		}
	})

	t.Run("rejects circular decode parameters in the same object stream", func(t *testing.T) {
		objectStream := []byte(
			"5 0 obj\n<< /Type /ObjStm /N 1 /First 5 /Length 7 /Filter /FlateDecode /DecodeParms 10 0 R >>\nstream\ninvalid\nendstream\nendobj",
		)
		xref := NewXRefTable()
		xref.AddEntry(NewXRefEntry(5, XRefEntryInUse, 0, 0))
		xref.AddEntry(targetEntry)
		reader := &Reader{
			src:         bytes.NewReader(objectStream),
			fileSize:    int64(len(objectStream)),
			xrefTable:   xref,
			objectCache: make(map[int]PdfObject),
			objStmCache: make(map[int]map[int]PdfObject),
		}

		_, err := reader.GetObjectWithContext(context.Background(), 10)
		if err == nil {
			t.Fatal("GetObjectWithContext() error = nil, want circular dependency error")
		}
		if !strings.Contains(err.Error(), "circular decode dependency through ObjStm 5") {
			t.Fatalf("GetObjectWithContext() error = %v, want circular dependency detail", err)
		}
	})
}

// Helper function to check if a string contains a substring.
func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}

// Helper function to get a string representation of object type and value.
func getObjectTypeString(obj PdfObject) string {
	switch o := obj.(type) {
	case *Integer:
		return "integer:" + o.String()
	case *Name:
		return "name:" + o.Value()
	case *Boolean:
		if o.Value() {
			return "boolean:true"
		}
		return "boolean:false"
	case *Dictionary:
		return "dictionary"
	case *Array:
		return "array"
	default:
		return "unknown"
	}
}
