package writer

import (
	"bytes"
	"testing"

	"github.com/coregx/gxpdf/internal/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- ContentStreamWriter uncovered methods ----------

func TestContentStreamWriter_ShowTextEncoded(t *testing.T) {
	csw := NewContentStreamWriter()
	csw.BeginText()
	csw.ShowTextEncoded("encodedHex")
	csw.EndText()
	content := csw.String()
	assert.Contains(t, content, "BT")
	assert.Contains(t, content, "ET")
}

func TestContentStreamWriter_ClipEvenOdd(t *testing.T) {
	csw := NewContentStreamWriter()
	csw.MoveTo(0, 0)
	csw.LineTo(100, 100)
	csw.ClipEvenOdd()
	content := csw.String()
	assert.Contains(t, content, "W*")
}

func TestContentStreamWriter_SetCompression(t *testing.T) {
	csw := NewContentStreamWriter()
	csw.SetCompression(BestCompression)
	assert.Equal(t, BestCompression, csw.GetCompression())
}

func TestContentStreamWriter_GetCompression_Default(t *testing.T) {
	csw := NewContentStreamWriter()
	level := csw.GetCompression()
	// Default is DefaultCompression (-1) or NoCompression (0)
	_ = level // Just ensure it's callable without panic
}

func TestContentStreamWriter_IsCompressed_False(t *testing.T) {
	csw := NewContentStreamWriter()
	csw.SetCompression(NoCompression)
	assert.False(t, csw.IsCompressed())
}

func TestContentStreamWriter_IsCompressed_True(t *testing.T) {
	csw := NewContentStreamWriter()
	csw.SetCompression(BestCompression)
	assert.True(t, csw.IsCompressed())
}

func TestContentStreamWriter_CompressedBytes_NoCompression(t *testing.T) {
	csw := NewContentStreamWriter()
	csw.SetCompression(NoCompression)
	csw.BeginText()
	csw.ShowText("hello")
	csw.EndText()
	data, err := csw.CompressedBytes()
	require.NoError(t, err)
	assert.Greater(t, len(data), 0)
}

func TestContentStreamWriter_CompressedBytes_WithCompression(t *testing.T) {
	csw := NewContentStreamWriter()
	csw.SetCompression(DefaultCompression)
	csw.BeginText()
	csw.ShowText("hello world test content")
	csw.EndText()
	data, err := csw.CompressedBytes()
	require.NoError(t, err)
	assert.Greater(t, len(data), 0)
}

// ---------- GenerateContentStream tests ----------

func TestGenerateContentStream_Empty(t *testing.T) {
	content, res, err := GenerateContentStream(nil)
	require.NoError(t, err)
	assert.Empty(t, content)
	assert.NotNil(t, res)
}

func TestGenerateContentStream_SimpleText(t *testing.T) {
	ops := []TextOp{
		{
			Text:  "Hello World",
			X:     100,
			Y:     700,
			Font:  "Helvetica",
			Size:  12,
			Color: RGB{R: 0, G: 0, B: 0},
		},
	}
	content, res, err := GenerateContentStream(ops)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
	assert.NotNil(t, res)
}

func TestGenerateContentStream_MultipleOps(t *testing.T) {
	ops := []TextOp{
		{Text: "First", X: 100, Y: 700, Font: "Helvetica", Size: 12},
		{Text: "Second", X: 100, Y: 680, Font: "Times-Roman", Size: 10},
	}
	content, res, err := GenerateContentStream(ops)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
	assert.NotNil(t, res)
}

func TestGenerateContentStream_WithOpacity(t *testing.T) {
	ops := []TextOp{
		{Text: "Faded", X: 100, Y: 700, Font: "Helvetica", Size: 12, Opacity: 0.5},
	}
	content, res, err := GenerateContentStream(ops)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
	assert.NotNil(t, res)
}

func TestGenerateContentStream_WithRotation(t *testing.T) {
	ops := []TextOp{
		{Text: "Rotated", X: 100, Y: 400, Font: "Helvetica", Size: 12, Rotation: 45.0},
	}
	content, _, err := GenerateContentStream(ops)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}

func TestGenerateContentStream_WithCMYKColor(t *testing.T) {
	cmyk := &CMYK{C: 0, M: 0, Y: 0, K: 1}
	ops := []TextOp{
		{Text: "CMYK", X: 100, Y: 700, Font: "Helvetica", Size: 12, ColorCMYK: cmyk},
	}
	content, _, err := GenerateContentStream(ops)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}

// ---------- GenerateContentStreamWithGraphics tests ----------

func TestGenerateContentStreamWithGraphics_Empty(t *testing.T) {
	content, res, err := GenerateContentStreamWithGraphics(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, content)
	assert.NotNil(t, res)
}

func TestGenerateContentStreamWithGraphics_Line(t *testing.T) {
	graphicsOps := []GraphicsOp{
		{
			Type: 0, // line
			X:    10, Y: 10, X2: 200, Y2: 200,
			StrokeColor: &RGB{R: 0, G: 0, B: 0},
			StrokeWidth: 1.0,
		},
	}
	content, _, err := GenerateContentStreamWithGraphics(nil, graphicsOps)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}

func TestGenerateContentStreamWithGraphics_Rect(t *testing.T) {
	graphicsOps := []GraphicsOp{
		{
			Type: 1, // rect
			X:    50, Y: 50, Width: 200, Height: 100,
			StrokeColor: &RGB{R: 0, G: 0, B: 0},
			StrokeWidth: 1.0,
		},
	}
	content, _, err := GenerateContentStreamWithGraphics(nil, graphicsOps)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}

func TestGenerateContentStreamWithGraphics_Circle(t *testing.T) {
	graphicsOps := []GraphicsOp{
		{
			Type: 2, // circle
			X:    100, Y: 100, Radius: 50,
			StrokeColor: &RGB{R: 1, G: 0, B: 0},
			StrokeWidth: 1.0,
		},
	}
	content, _, err := GenerateContentStreamWithGraphics(nil, graphicsOps)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}

func TestGenerateContentStreamWithGraphics_Polygon(t *testing.T) {
	vertices := []Point{{X: 100, Y: 100}, {X: 200, Y: 100}, {X: 150, Y: 200}}
	graphicsOps := []GraphicsOp{
		{Type: 5, Vertices: vertices, StrokeColor: &RGB{R: 0, G: 0, B: 1}},
	}
	content, _, err := GenerateContentStreamWithGraphics(nil, graphicsOps)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}

func TestGenerateContentStreamWithGraphics_Polyline(t *testing.T) {
	vertices := []Point{{X: 10, Y: 10}, {X: 50, Y: 100}, {X: 100, Y: 10}}
	graphicsOps := []GraphicsOp{
		{Type: 6, Vertices: vertices, StrokeColor: &RGB{R: 0, G: 1, B: 0}},
	}
	content, _, err := GenerateContentStreamWithGraphics(nil, graphicsOps)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}

func TestGenerateContentStreamWithGraphics_Ellipse(t *testing.T) {
	graphicsOps := []GraphicsOp{
		{Type: 7, X: 200, Y: 200, RX: 100, RY: 50, StrokeColor: &RGB{R: 0, G: 0, B: 0}},
	}
	content, _, err := GenerateContentStreamWithGraphics(nil, graphicsOps)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}

func TestGenerateContentStreamWithGraphics_Bezier(t *testing.T) {
	segs := []BezierSegment{
		{
			Start: Point{X: 0, Y: 0},
			C1:    Point{X: 100, Y: 200},
			C2:    Point{X: 200, Y: 200},
			End:   Point{X: 300, Y: 0},
		},
	}
	graphicsOps := []GraphicsOp{
		{Type: 8, BezierSegs: segs, StrokeColor: &RGB{R: 0, G: 0, B: 0}},
	}
	content, _, err := GenerateContentStreamWithGraphics(nil, graphicsOps)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}

func TestGenerateContentStreamWithGraphics_Arc(t *testing.T) {
	graphicsOps := []GraphicsOp{
		{
			Type: 9, // arc
			X:    200, Y: 200, RX: 100, RY: 100,
			StartAngle: 0, SweepAngle: 90,
			StrokeColor: &RGB{R: 0, G: 0, B: 0},
		},
	}
	content, _, err := GenerateContentStreamWithGraphics(nil, graphicsOps)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}

func TestGenerateContentStreamWithGraphics_WithOpacity(t *testing.T) {
	graphicsOps := []GraphicsOp{
		{
			Type: 1, // rect
			X:    50, Y: 50, Width: 100, Height: 50,
			StrokeColor: &RGB{R: 0, G: 0, B: 0},
			Opacity:     0.5,
		},
	}
	content, _, err := GenerateContentStreamWithGraphics(nil, graphicsOps)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}

func TestGenerateContentStreamWithGraphics_WithFill(t *testing.T) {
	graphicsOps := []GraphicsOp{
		{
			Type: 1, // rect
			X:    50, Y: 50, Width: 100, Height: 50,
			StrokeColor: &RGB{R: 0, G: 0, B: 0},
			FillColor:   &RGB{R: 0.5, G: 0.5, B: 0.5},
		},
	}
	content, _, err := GenerateContentStreamWithGraphics(nil, graphicsOps)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}

func TestGenerateContentStreamWithGraphics_WithDash(t *testing.T) {
	graphicsOps := []GraphicsOp{
		{
			Type: 0, // line
			X:    10, Y: 10, X2: 200, Y2: 10,
			StrokeColor: &RGB{},
			Dashed:      true,
			DashArray:   []float64{5, 3},
			DashPhase:   0,
		},
	}
	content, _, err := GenerateContentStreamWithGraphics(nil, graphicsOps)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}

func TestGenerateContentStreamWithGraphics_WithCMYKStroke(t *testing.T) {
	graphicsOps := []GraphicsOp{
		{
			Type: 1,
			X:    50, Y: 50, Width: 100, Height: 50,
			StrokeColorCMYK: &CMYK{C: 0, M: 1, Y: 0, K: 0},
		},
	}
	content, _, err := GenerateContentStreamWithGraphics(nil, graphicsOps)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}

func TestGenerateContentStreamWithGraphics_BeginEndClip(t *testing.T) {
	// Type 20 = BeginClipRect, Type 21 = EndClip
	graphicsOps := []GraphicsOp{
		{Type: 20, X: 0, Y: 0, Width: 100, Height: 100},
		{Type: 1, X: 10, Y: 10, Width: 50, Height: 50, StrokeColor: &RGB{}},
		{Type: 21},
	}
	content, _, err := GenerateContentStreamWithGraphics(nil, graphicsOps)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}

func TestGenerateContentStreamWithGraphics_Watermark(t *testing.T) {
	graphicsOps := []GraphicsOp{
		{
			Type: 4, // watermark
			X:    200, Y: 400,
			Text:              "DRAFT",
			TextSize:          72,
			TextColorR:        0.5,
			WatermarkFont:     "Helvetica",
			WatermarkOpacity:  0.3,
			WatermarkRotation: 45,
		},
	}
	content, _, err := GenerateContentStreamWithGraphics(nil, graphicsOps)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}

// ---------- CreateFontObjects tests ----------

func TestCreateFontObjects_Empty(t *testing.T) {
	fonts, err := CreateFontObjects(nil)
	require.NoError(t, err)
	assert.Empty(t, fonts)
}

func TestCreateFontObjects_StandardFont(t *testing.T) {
	ops := []TextOp{
		{Font: "Helvetica"},
		{Font: "Helvetica"}, // duplicate, should not appear twice
		{Font: "Times-Roman"},
	}
	fonts, err := CreateFontObjects(ops)
	require.NoError(t, err)
	assert.Len(t, fonts, 2)
	assert.Contains(t, fonts, "Helvetica")
	assert.Contains(t, fonts, "Times-Roman")
}

func TestCreateFontObjects_UnknownFont(t *testing.T) {
	ops := []TextOp{
		{Font: "NonExistentFont99"},
	}
	_, err := CreateFontObjects(ops)
	assert.Error(t, err)
}

func TestCreateFontObjects_SkipsCustomFont(t *testing.T) {
	// CustomFont != nil → should be skipped by CreateFontObjects
	ops := []TextOp{
		{CustomFont: &EmbeddedFont{ID: "custom_1"}},
	}
	fonts, err := CreateFontObjects(ops)
	require.NoError(t, err)
	assert.Empty(t, fonts)
}

// ---------- CreateFontCollection tests ----------

func TestCreateFontCollection_Empty(t *testing.T) {
	fc, err := CreateFontCollection(nil)
	require.NoError(t, err)
	assert.NotNil(t, fc)
	assert.Empty(t, fc.Standard14)
	assert.Empty(t, fc.Embedded)
}

func TestCreateFontCollection_StandardFont(t *testing.T) {
	ops := []TextOp{
		{Font: "Helvetica"},
	}
	fc, err := CreateFontCollection(ops)
	require.NoError(t, err)
	assert.NotNil(t, fc)
	assert.Contains(t, fc.Standard14, "Helvetica")
}

func TestCreateFontCollection_HasEmbeddedFonts(t *testing.T) {
	fc, err := CreateFontCollection(nil)
	require.NoError(t, err)
	assert.False(t, fc.HasEmbeddedFonts())
}

func TestCreateFontCollection_TotalFontCount(t *testing.T) {
	ops := []TextOp{
		{Font: "Helvetica"},
		{Font: "Times-Roman"},
	}
	fc, err := CreateFontCollection(ops)
	require.NoError(t, err)
	assert.Equal(t, 2, fc.TotalFontCount())
}

// ---------- CreateContentStreamObject tests ----------

func TestCreateContentStreamObject_Uncompressed(t *testing.T) {
	content := []byte("BT /F1 12 Tf 100 700 Td (Hello) Tj ET")
	obj := CreateContentStreamObject(5, content, false)
	assert.NotNil(t, obj)
	assert.Equal(t, 5, obj.Number)
	assert.Contains(t, string(obj.Data), "BT")
}

func TestCreateContentStreamObject_Compressed(t *testing.T) {
	// Large content to ensure compression is beneficial
	content := bytes.Repeat([]byte("BT /F1 12 Tf 100 700 Td (Hello World) Tj ET\n"), 20)
	obj := CreateContentStreamObject(6, content, true)
	assert.NotNil(t, obj)
	// Content should have Length key
	objContent := string(obj.Data)
	assert.Contains(t, objContent, "Length")
}

func TestCreateContentStreamObject_Empty(t *testing.T) {
	obj := CreateContentStreamObject(7, []byte{}, false)
	assert.NotNil(t, obj)
}

// ---------- CreateAcroFormDict tests ----------

func TestCreateAcroFormDict_NoFields(t *testing.T) {
	// Returns empty string when no fields
	result := CreateAcroFormDict(nil, 5)
	assert.Empty(t, result)
}

func TestCreateAcroFormDict_WithFields(t *testing.T) {
	fieldRefs := []int{10, 11, 12}
	result := CreateAcroFormDict(fieldRefs, 5)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Fields")
}

func TestCreateAcroFormDict_WithFieldsNoFont(t *testing.T) {
	fieldRefs := []int{10}
	result := CreateAcroFormDict(fieldRefs, 0) // fontObjNum=0 → no /DR
	assert.NotEmpty(t, result)
	assert.NotContains(t, result, "/DR")
}

// ---------- createFormFieldObject tests ----------

func TestCreateFormFieldObject_TextField(t *testing.T) {
	field := document.NewFormField("Tx", "name", [4]float64{100, 700, 300, 720})
	field.SetValue("John Doe")
	obj := createFormFieldObject(10, field)
	assert.NotNil(t, obj)
	content := string(obj.Data)
	assert.Contains(t, content, "Widget")
}

func TestCreateFormFieldObject_ButtonField(t *testing.T) {
	field := document.NewFormField("Btn", "checkbox", [4]float64{50, 600, 70, 620})
	obj := createFormFieldObject(11, field)
	assert.NotNil(t, obj)
}

func TestCreateFormFieldObject_ChoiceField(t *testing.T) {
	field := document.NewFormField("Ch", "dropdown", [4]float64{100, 500, 200, 520})
	obj := createFormFieldObject(12, field)
	assert.NotNil(t, obj)
}

// ---------- Annotation writer tests (using PdfWriter.WriteAnnotations / WriteAllAnnotations) ----------

func newTestPdfWriter(t *testing.T) *PdfWriter {
	t.Helper()
	var buf bytes.Buffer
	return NewPdfWriterFromWriter(&buf)
}

func TestWriteAnnotations_Empty(t *testing.T) {
	w := newTestPdfWriter(t)
	objs, refs, err := w.WriteAnnotations(nil)
	require.NoError(t, err)
	assert.Nil(t, objs)
	assert.Nil(t, refs)
}

func TestWriteAnnotations_URLLink(t *testing.T) {
	w := newTestPdfWriter(t)
	link := document.NewLinkAnnotation([4]float64{100, 690, 200, 710}, "https://example.com")
	objs, refs, err := w.WriteAnnotations([]*document.LinkAnnotation{link})
	require.NoError(t, err)
	assert.Len(t, objs, 1)
	assert.Len(t, refs, 1)
}

func TestWriteAnnotations_InternalLink(t *testing.T) {
	w := newTestPdfWriter(t)
	link := document.NewInternalLinkAnnotation([4]float64{100, 690, 200, 710}, 2)
	objs, refs, err := w.WriteAnnotations([]*document.LinkAnnotation{link})
	require.NoError(t, err)
	assert.Len(t, objs, 1)
	assert.Len(t, refs, 1)
	content := string(objs[0].Data)
	assert.Contains(t, content, "Annot")
}

func TestWriteAllAnnotations_EmptyPage(t *testing.T) {
	w := newTestPdfWriter(t)
	page := document.NewPage(0, document.A4)
	objs, refs, err := w.WriteAllAnnotations(page)
	require.NoError(t, err)
	assert.Empty(t, objs)
	assert.Empty(t, refs)
}

func TestWriteAllAnnotations_WithTextAnnotation(t *testing.T) {
	w := newTestPdfWriter(t)
	page := document.NewPage(0, document.A4)

	textAnnot := document.NewTextAnnotation([4]float64{100, 700, 120, 720}, "Test note", "Author")
	page.AddTextAnnotation(textAnnot)

	objs, refs, err := w.WriteAllAnnotations(page)
	require.NoError(t, err)
	assert.Len(t, objs, 1)
	assert.Len(t, refs, 1)
}

func TestWriteAllAnnotations_WithMarkupAnnotation(t *testing.T) {
	w := newTestPdfWriter(t)
	page := document.NewPage(0, document.A4)

	quadPoints := [][8]float64{{100, 700, 300, 700, 100, 720, 300, 720}}
	markupAnnot := document.NewMarkupAnnotation(
		document.AnnotationTypeHighlight,
		[4]float64{100, 700, 300, 720},
		quadPoints,
	)
	page.AddMarkupAnnotation(markupAnnot)

	objs, refs, err := w.WriteAllAnnotations(page)
	require.NoError(t, err)
	assert.Len(t, objs, 1)
	assert.Len(t, refs, 1)
}

func TestWriteAllAnnotations_WithStampAnnotation(t *testing.T) {
	w := newTestPdfWriter(t)
	page := document.NewPage(0, document.A4)

	stampAnnot := document.NewStampAnnotation(
		[4]float64{100, 600, 200, 650},
		document.StampApproved,
	)
	page.AddStampAnnotation(stampAnnot)

	objs, refs, err := w.WriteAllAnnotations(page)
	require.NoError(t, err)
	assert.Len(t, objs, 1)
	assert.Len(t, refs, 1)
}

func TestWriteAllAnnotations_AllTypes(t *testing.T) {
	w := newTestPdfWriter(t)
	page := document.NewPage(0, document.A4)

	// Add all annotation types
	link := document.NewLinkAnnotation([4]float64{100, 690, 200, 710}, "https://example.com")
	page.AddLinkAnnotation(link)

	textAnnot := document.NewTextAnnotation([4]float64{100, 700, 120, 720}, "Note", "Author")
	page.AddTextAnnotation(textAnnot)

	objs, refs, err := w.WriteAllAnnotations(page)
	require.NoError(t, err)
	assert.Equal(t, 2, len(objs))
	assert.Equal(t, 2, len(refs))
}

// ---------- ResourceDictionary methods ----------

func TestResourceDictionary_SetFontObjNumByID_Found(t *testing.T) {
	// Use the content stream generator to populate fontIDs
	content, rd2, err := GenerateContentStream([]TextOp{
		{Text: "hello", Font: "Helvetica", Size: 12, X: 100, Y: 700},
	})
	require.NoError(t, err)
	_ = content
	// rd2 has fontID mappings populated during generation
	ok := rd2.SetFontObjNumByID("std:Helvetica", 42)
	assert.True(t, ok)
}

func TestResourceDictionary_SetFontObjNumByID_NotFound(t *testing.T) {
	rd := NewResourceDictionary()
	ok := rd.SetFontObjNumByID("nonexistent", 42)
	assert.False(t, ok)
}

func TestResourceDictionary_GetFontIDMapping_Empty(t *testing.T) {
	rd := NewResourceDictionary()
	m := rd.GetFontIDMapping()
	assert.Empty(t, m)
}

func TestResourceDictionary_GetFontIDMapping_NonEmpty(t *testing.T) {
	_, rd, err := GenerateContentStream([]TextOp{
		{Text: "hello", Font: "Helvetica", Size: 12, X: 100, Y: 700},
	})
	require.NoError(t, err)
	m := rd.GetFontIDMapping()
	assert.NotEmpty(t, m)
}

func TestResourceDictionary_SetImageObjNum_Found(t *testing.T) {
	rd := NewResourceDictionary()
	name := rd.AddImage(0) // placeholder objNum=0
	ok := rd.SetImageObjNum(name, 99)
	assert.True(t, ok)
}

func TestResourceDictionary_SetImageObjNum_NotFound(t *testing.T) {
	rd := NewResourceDictionary()
	ok := rd.SetImageObjNum("Im99", 99)
	assert.False(t, ok)
}

// ---------- encodeTextForEmbeddedFont ----------

func TestEncodeTextForEmbeddedFont_NilFont(t *testing.T) {
	result := encodeTextForEmbeddedFont("hello", nil)
	assert.Equal(t, "<>", result)
}

func TestEncodeTextForEmbeddedFont_NilTTF(t *testing.T) {
	ef := &EmbeddedFont{TTF: nil}
	result := encodeTextForEmbeddedFont("hello", ef)
	assert.Equal(t, "<>", result)
}

// ---------- hasTextGraphicsOps ----------

func TestHasTextGraphicsOps_False(t *testing.T) {
	ops := []GraphicsOp{
		{Type: 0}, // line
		{Type: 1}, // rect
	}
	assert.False(t, hasTextGraphicsOps(ops))
}

func TestHasTextGraphicsOps_True(t *testing.T) {
	ops := []GraphicsOp{
		{Type: 0},
		{Type: 22}, // TextBlock
	}
	assert.True(t, hasTextGraphicsOps(ops))
}

func TestHasTextGraphicsOps_Watermark(t *testing.T) {
	assert.True(t, hasTextGraphicsOps([]GraphicsOp{{Type: 4}}))
}

func TestHasTextGraphicsOps_Empty(t *testing.T) {
	assert.False(t, hasTextGraphicsOps(nil))
}

// ---------- renderImage via GenerateContentStreamWithGraphics ----------

func TestGenerateContentStreamWithGraphics_Image(t *testing.T) {
	imgData := &ImageData{
		Data:             []byte{0xFF, 0xD8, 0xFF, 0xE0}, // fake JPEG header
		Width:            100,
		Height:           100,
		ColorSpace:       "DeviceRGB",
		Format:           "jpeg",
		BitsPerComponent: 8,
	}
	graphicsOps := []GraphicsOp{
		{
			Type: 3, // image
			X:    50, Y: 100, Width: 100, Height: 100,
			Image: imgData,
		},
	}
	content, resources, err := GenerateContentStreamWithGraphics(nil, graphicsOps)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
	_ = resources
}

func TestGenerateContentStreamWithGraphics_ImageNil(t *testing.T) {
	graphicsOps := []GraphicsOp{
		{Type: 3, X: 50, Y: 100, Width: 100, Height: 100, Image: nil},
	}
	_, _, err := GenerateContentStreamWithGraphics(nil, graphicsOps)
	assert.Error(t, err)
}

func TestGenerateContentStreamWithGraphics_ImageZeroDimensions(t *testing.T) {
	imgData := &ImageData{Data: []byte{0xFF}, Width: 0, Height: 0}
	graphicsOps := []GraphicsOp{
		{Type: 3, X: 50, Y: 100, Width: 0, Height: 0, Image: imgData},
	}
	_, _, err := GenerateContentStreamWithGraphics(nil, graphicsOps)
	assert.Error(t, err)
}

// ---------- createImageXObject / createSMaskObject via createAndAssignImageXObjects ----------

func TestCreateAndAssignImageXObjects_NoImages(t *testing.T) {
	w := newTestPdfWriter(t)
	rd := NewResourceDictionary()
	ops := []GraphicsOp{{Type: 0, X: 0, Y: 0, X2: 100, Y2: 100}}
	objs, err := w.createAndAssignImageXObjects(ops, rd)
	require.NoError(t, err)
	assert.Empty(t, objs)
}

func TestCreateAndAssignImageXObjects_JPEG(t *testing.T) {
	w := newTestPdfWriter(t)
	rd := NewResourceDictionary()
	imgData := &ImageData{
		Data:             []byte{0xFF, 0xD8},
		Width:            10,
		Height:           10,
		ColorSpace:       "DeviceRGB",
		Format:           "jpeg",
		BitsPerComponent: 8,
	}
	ops := []GraphicsOp{
		{Type: 3, X: 0, Y: 0, Width: 10, Height: 10, Image: imgData},
	}
	objs, err := w.createAndAssignImageXObjects(ops, rd)
	require.NoError(t, err)
	assert.Len(t, objs, 1)
}

func TestCreateAndAssignImageXObjects_PNGWithAlpha(t *testing.T) {
	w := newTestPdfWriter(t)
	rd := NewResourceDictionary()
	imgData := &ImageData{
		Data:             []byte{0x89, 0x50, 0x4E, 0x47},
		AlphaMask:        []byte{0xFF, 0x80, 0x00},
		Width:            2,
		Height:           2,
		ColorSpace:       "DeviceRGB",
		Format:           "png",
		BitsPerComponent: 8,
	}
	ops := []GraphicsOp{
		{Type: 3, X: 0, Y: 0, Width: 2, Height: 2, Image: imgData},
	}
	objs, err := w.createAndAssignImageXObjects(ops, rd)
	require.NoError(t, err)
	// Should have SMask object + image object = 2
	assert.Len(t, objs, 2)
}

// ---------- writeFormFields ----------

func TestWriteFormFields_Empty(t *testing.T) {
	w := newTestPdfWriter(t)
	objs, refs, err := w.writeFormFields(nil)
	require.NoError(t, err)
	assert.Nil(t, objs)
	assert.Nil(t, refs)
}

func TestWriteFormFields_WithFields(t *testing.T) {
	w := newTestPdfWriter(t)
	fields := []*document.FormField{
		document.NewFormField("Tx", "name", [4]float64{100, 700, 300, 720}),
		document.NewFormField("Btn", "submit", [4]float64{50, 600, 150, 620}),
	}
	objs, refs, err := w.writeFormFields(fields)
	require.NoError(t, err)
	assert.Len(t, objs, 2)
	assert.Len(t, refs, 2)
}

// ---------- WriteWithPageContent / WriteWithAllContent ----------

func newTestDocument(t *testing.T) *document.Document {
	t.Helper()
	doc := document.NewDocument()
	_, err := doc.AddPage(document.A4)
	require.NoError(t, err)
	return doc
}

func TestPdfWriter_WriteWithPageContent_Basic(t *testing.T) {
	var buf bytes.Buffer
	w := NewPdfWriterFromWriter(&buf)

	doc := newTestDocument(t)
	textContents := map[int][]TextOp{
		0: {
			{Text: "Hello", Font: "Helvetica", Size: 12, X: 100, Y: 700},
		},
	}
	err := w.WriteWithPageContent(doc, textContents)
	require.NoError(t, err)
	// Valid PDF starts with %PDF-
	data := buf.Bytes()
	assert.Greater(t, len(data), 100)
	assert.Equal(t, "%PDF-", string(data[:5]))
}

func TestPdfWriter_WriteWithPageContent_EmptyContent(t *testing.T) {
	var buf bytes.Buffer
	w := NewPdfWriterFromWriter(&buf)

	doc := newTestDocument(t)
	err := w.WriteWithPageContent(doc, nil)
	require.NoError(t, err)
	assert.Greater(t, len(buf.Bytes()), 100)
}

func TestPdfWriter_WriteWithPageContent_ClosedWriter(t *testing.T) {
	var buf bytes.Buffer
	w := NewPdfWriterFromWriter(&buf)
	w.closed = true

	doc := newTestDocument(t)
	err := w.WriteWithPageContent(doc, nil)
	assert.Error(t, err)
}

func TestPdfWriter_WriteWithAllContent_Basic(t *testing.T) {
	var buf bytes.Buffer
	w := NewPdfWriterFromWriter(&buf)

	doc := newTestDocument(t)
	textContents := map[int][]TextOp{
		0: {
			{Text: "Hello", Font: "Helvetica", Size: 12, X: 100, Y: 700},
		},
	}
	graphicsContents := map[int][]GraphicsOp{
		0: {
			{Type: 0, X: 10, Y: 10, X2: 200, Y2: 200, StrokeWidth: 1},
		},
	}
	err := w.WriteWithAllContent(doc, textContents, graphicsContents)
	require.NoError(t, err)
	data := buf.Bytes()
	assert.Greater(t, len(data), 100)
}

func TestPdfWriter_WriteWithAllContent_EmptyContent(t *testing.T) {
	var buf bytes.Buffer
	w := NewPdfWriterFromWriter(&buf)

	doc := newTestDocument(t)
	err := w.WriteWithAllContent(doc, nil, nil)
	require.NoError(t, err)
	assert.Greater(t, len(buf.Bytes()), 100)
}

func TestPdfWriter_WriteWithAllContent_ClosedWriter(t *testing.T) {
	var buf bytes.Buffer
	w := NewPdfWriterFromWriter(&buf)
	w.closed = true

	doc := newTestDocument(t)
	err := w.WriteWithAllContent(doc, nil, nil)
	assert.Error(t, err)
}

func TestPdfWriter_WriteWithAllContent_WithImage(t *testing.T) {
	var buf bytes.Buffer
	w := NewPdfWriterFromWriter(&buf)

	doc := newTestDocument(t)
	imgData := &ImageData{
		Data:             []byte{0xFF, 0xD8, 0xFF},
		Width:            10,
		Height:           10,
		ColorSpace:       "DeviceRGB",
		Format:           "jpeg",
		BitsPerComponent: 8,
	}
	graphicsContents := map[int][]GraphicsOp{
		0: {
			{Type: 3, X: 50, Y: 100, Width: 10, Height: 10, Image: imgData},
		},
	}
	err := w.WriteWithAllContent(doc, nil, graphicsContents)
	require.NoError(t, err)
	assert.Greater(t, len(buf.Bytes()), 100)
}

func TestPdfWriter_WriteWithAllContent_TextBlock(t *testing.T) {
	var buf bytes.Buffer
	w := NewPdfWriterFromWriter(&buf)

	doc := newTestDocument(t)
	graphicsContents := map[int][]GraphicsOp{
		0: {
			{Type: 22, X: 100, Y: 500, Text: "Block text", TextSize: 12},
		},
	}
	err := w.WriteWithAllContent(doc, nil, graphicsContents)
	require.NoError(t, err)
	assert.Greater(t, len(buf.Bytes()), 100)
}

func TestPdfWriter_Write_Basic(t *testing.T) {
	var buf bytes.Buffer
	w := NewPdfWriterFromWriter(&buf)

	doc := newTestDocument(t)
	err := w.Write(doc)
	require.NoError(t, err)
	data := buf.Bytes()
	assert.Greater(t, len(data), 100)
	assert.Equal(t, "%PDF-", string(data[:5]))
}

func TestPdfWriter_Write_Closed(t *testing.T) {
	var buf bytes.Buffer
	w := NewPdfWriterFromWriter(&buf)
	w.closed = true

	doc := newTestDocument(t)
	err := w.Write(doc)
	assert.Error(t, err)
}

// ---------- renderGraphicsOp error paths ----------

func TestGenerateContentStreamWithGraphics_InvalidTextBlock(t *testing.T) {
	// TextBlock (type 22) without font - should produce output or error gracefully
	graphicsOps := []GraphicsOp{
		{
			Type: 22,
			X:    100, Y: 500,
			Text:     "Block text",
			TextSize: 12,
		},
	}
	// This may or may not error depending on implementation
	_, _, _ = GenerateContentStreamWithGraphics(nil, graphicsOps)
}

// ---------- setStrokeColor / setFillColor with nil color ----------

func TestGenerateContentStreamWithGraphics_NilStrokeColor(t *testing.T) {
	graphicsOps := []GraphicsOp{
		{
			Type: 0,
			X:    10, Y: 10, X2: 100, Y2: 100,
			StrokeColor: nil, // nil - no stroke color set
			StrokeWidth: 1.0,
		},
	}
	content, _, err := GenerateContentStreamWithGraphics(nil, graphicsOps)
	require.NoError(t, err)
	_ = content
}

func TestGenerateContentStreamWithGraphics_RectWithFillOnly(t *testing.T) {
	graphicsOps := []GraphicsOp{
		{
			Type: 1,
			X:    50, Y: 50, Width: 100, Height: 50,
			FillColor: &RGB{R: 1, G: 0, B: 0},
		},
	}
	content, _, err := GenerateContentStreamWithGraphics(nil, graphicsOps)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}

// ---------- gradient fill ----------

func TestGenerateContentStreamWithGraphics_LinearGradient(t *testing.T) {
	grad := &GradientOp{
		Type: GradientTypeLinear,
		X1:   0, Y1: 0, X2: 100, Y2: 100,
		ColorStops: []ColorStopOp{
			{Position: 0.0, Color: RGB{R: 1, G: 0, B: 0}},
			{Position: 1.0, Color: RGB{R: 0, G: 0, B: 1}},
		},
	}
	graphicsOps := []GraphicsOp{
		{
			Type: 1,
			X:    0, Y: 0, Width: 100, Height: 100,
			FillGradient: grad,
		},
	}
	content, _, err := GenerateContentStreamWithGraphics(nil, graphicsOps)
	require.NoError(t, err)
	_ = content
}

// ---------- ClipPath flag ----------

func TestGenerateContentStreamWithGraphics_ClipPath(t *testing.T) {
	graphicsOps := []GraphicsOp{
		{
			Type: 1,
			X:    0, Y: 0, Width: 100, Height: 100,
			IsClipPath: true,
		},
	}
	content, _, err := GenerateContentStreamWithGraphics(nil, graphicsOps)
	require.NoError(t, err)
	_ = content
}
