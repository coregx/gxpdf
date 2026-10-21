package testutil

import (
	"fmt"
	"strings"
)

type transformedPDFObject []byte

type transformedPDFBuilder struct {
	objects []transformedPDFObject
}

func (b *transformedPDFBuilder) reserve() int {
	b.objects = append(b.objects, nil)
	return len(b.objects)
}

func (b *transformedPDFBuilder) add(body string) int {
	number := b.reserve()
	b.set(number, transformedPDFObject(body))
	return number
}

func (b *transformedPDFBuilder) addStream(data, dictionary string) int {
	number := b.reserve()
	b.set(number, transformedPDFObject(Stream(dictionary, data)))
	return number
}

func (b *transformedPDFBuilder) set(number int, body transformedPDFObject) {
	b.objects[number-1] = body
}

func (b *transformedPDFBuilder) bytes(root int) []byte {
	objects := make([]string, len(b.objects))
	for index, object := range b.objects {
		if object == nil {
			panic(fmt.Sprintf("object %d was not populated", index+1))
		}
		objects[index] = string(object)
	}
	return BuildPDFWithOptions(PDFBuildOptions{
		RootObject: root, BinaryComment: true, CompactXRef: true,
	}, objects...)
}

func transformedFontDictionary() string {
	return fmt.Sprintf(
		"<< /Type /Font /Subtype /TrueType /BaseFont /Synthetic /FirstChar 32 /LastChar 126 /Widths [%s] >>",
		strings.TrimSpace(strings.Repeat("600 ", 95)),
	)
}

func transformedFormDictionary(font int, matrixValue, extraResources string) string {
	resources := fmt.Sprintf("/Font << /F1 %d 0 R /F2 %d 0 R >> %s", font, font, extraResources)
	return fmt.Sprintf(
		"/Type /XObject /Subtype /Form /BBox [-200 -200 1000 1000] /Matrix %s /Resources << %s >>",
		matrixValue, resources,
	)
}

// BuildTransformedTextGeometryPDF deterministically builds the fixture used to
// verify that direct and Form text share one transformed coordinate model.
func BuildTransformedTextGeometryPDF() []byte {
	var b transformedPDFBuilder
	catalog := b.reserve()
	pages := b.reserve()
	font := b.add(transformedFontDictionary())

	sizes := []string{"1", "2", "2.01", "12"}
	sizeForms := make([]int, len(sizes))
	for index, size := range sizes {
		content := fmt.Sprintf("BT /F2 %s Tf 1 0 0 1 10 %d Tm (F%s) Tj ET", size, 20+index*30, size)
		sizeForms[index] = b.addStream(content, transformedFormDictionary(font, "[1 0 0 1 0 0]", ""))
	}

	equivalent := b.addStream(
		"BT /F2 12 Tf 1 0 0 1 3 4 Tm (FM) Tj ET",
		transformedFormDictionary(font, "[0 1 -1 0 20 30]", ""),
	)
	inner := b.addStream(
		"BT /F2 2.01 Tf 1 0 0 1 1 2 Tm (Nested) Tj ET",
		transformedFormDictionary(font, "[1 0.25 -0.5 1 7 11]", ""),
	)
	outer := b.addStream(
		"/Inner Do",
		transformedFormDictionary(font, "[2 0 0 3 13 17]", fmt.Sprintf("/XObject << /Inner %d 0 R >>", inner)),
	)
	spaceForm := b.addStream(
		"BT /F2 12 Tf 4 Tw 1 0 0 1 10 80 Tm (A ) Tj /F2 8 Tf 150 Tz (B) Tj 100 Tz /F2 10 Tf 1 0 0 1 10 95 Tm (C) Tj ( D) Tj 1 0 0 1 10 110 Tm (E) Tj (F) Tj ET",
		transformedFormDictionary(font, "[1 0 0 1 0 0]", ""),
	)

	pageNumbers := make([]int, 0, 8)
	addPage := func(content, mediaBox, rotate string) {
		contentNumber := b.addStream(content, "")
		page := b.reserve()
		pageNumbers = append(pageNumbers, page)
		rotation := ""
		if rotate != "" {
			rotation = " /Rotate " + rotate
		}
		formEntries := make([]string, 0, len(sizeForms)+3)
		for index, form := range sizeForms {
			formEntries = append(formEntries, fmt.Sprintf("/S%d %d 0 R", index, form))
		}
		formEntries = append(formEntries,
			fmt.Sprintf("/Eq %d 0 R", equivalent),
			fmt.Sprintf("/Outer %d 0 R", outer),
			fmt.Sprintf("/Space %d 0 R", spaceForm),
		)
		b.set(page, transformedPDFObject(fmt.Sprintf(
			"<< /Type /Page /Parent %d 0 R /MediaBox %s%s /Resources << /Font << /F1 %d 0 R >> /XObject << %s >> >> /Contents %d 0 R >>",
			pages, mediaBox, rotation, font, strings.Join(formEntries, " "), contentNumber,
		)))
	}

	var thresholdPage strings.Builder
	thresholdPage.WriteString("q 2 0.5 0.25 3 100 50 cm\n")
	for index, size := range sizes {
		fmt.Fprintf(&thresholdPage, "BT /F1 %s Tf 1 0 0 1 10 %d Tm (D%s) Tj ET /S%d Do\n", size, 20+index*30, size, index)
	}
	thresholdPage.WriteString("Q")
	addPage(thresholdPage.String(), "[0 0 612 792]", "")

	addPage("q 0 1 -1 0 20 30 cm BT /F1 12 Tf 1 0 0 1 3 4 Tm (CM) Tj ET Q\n/Eq Do", "[0 0 612 792]", "")
	addPage("q 0.5 0 0 0.5 100 200 cm /Outer Do Q q 0.5 0 0 0.5 200 300 cm /Outer Do Q", "[0 0 612 792]", "")

	for _, rotation := range []string{"0", "90", "180", "270"} {
		addPage("BT /F1 12 Tf 1 0.2 -0.1 1 30 40 Tm (Rotate) Tj ET /S3 Do", "[0 0 612 792]", rotation)
	}
	addPage("BT /F1 12 Tf 1 0 0 1 -75 850 Tm (Origin) Tj ET", "[-50 100 550 900]", "")
	addPage("BT /F1 12 Tf 4 Tw 1 0 0 1 10 20 Tm (A ) Tj /F1 8 Tf 150 Tz (B) Tj 100 Tz /F1 10 Tf 1 0 0 1 10 35 Tm (C) Tj ( D) Tj 1 0 0 1 10 50 Tm (E) Tj (F) Tj ET\n/Space Do", "[0 0 612 792]", "")

	kids := make([]string, len(pageNumbers))
	for index, page := range pageNumbers {
		kids[index] = fmt.Sprintf("%d 0 R", page)
	}
	b.set(pages, transformedPDFObject(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(kids))))
	b.set(catalog, transformedPDFObject(fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R >>", pages)))
	return b.bytes(catalog)
}

// BuildTransformedTextLatticePDF deterministically builds the mixed direct and
// nested-Form lattice fixture used by page and document table extraction tests.
func BuildTransformedTextLatticePDF() []byte {
	var b transformedPDFBuilder
	catalog := b.reserve()
	pages := b.reserve()
	font := b.add(transformedFontDictionary())
	body := b.addStream(
		"BT /F2 10 Tf 1 0 0 1 15 5 Tm (Revenue) Tj ET\nBT /F2 10 Tf 1 0 0 1 115 5 Tm (1200) Tj ET",
		transformedFormDictionary(font, "[1 0 0 1 0 0]", ""),
	)
	outer := b.addStream(
		"/Body Do",
		transformedFormDictionary(font, "[1 0 0 1 0 0]", fmt.Sprintf("/XObject << /Body %d 0 R >>", body)),
	)
	spaceBody := b.addStream(
		"BT /F2 10 Tf 4 Tw 1 0 0 1 130 10 Tm (A ) Tj (B) Tj 1 0 0 1 130 25 Tm (C) Tj ( D) Tj 1 0 0 1 130 40 Tm (E) Tj (F) Tj ET",
		transformedFormDictionary(font, "[1 0 0 1 0 0]", ""),
	)
	content := strings.Join([]string{
		"q 1.5 0 0 2 30 40 cm",
		"0 0 m 160 0 l S 0 20 m 160 20 l S 0 40 m 160 40 l S",
		// Extend vertical rules beyond the outer horizontal rules. The existing
		// lattice extractor expands edge-cell bounds for missing-border recovery;
		// full-height rules make this fixture unambiguously unmerged.
		"0 -40 m 0 80 l S 100 -40 m 100 80 l S 160 -40 m 160 80 l S",
		"BT /F1 10 Tf 1 0 0 1 15 25 Tm (Line Item) Tj ET",
		"BT /F1 10 Tf 1 0 0 1 115 25 Tm (2024) Tj ET",
		"/Outer Do",
		"Q",
	}, "\n")
	contents := b.addStream(content, "")
	pageOne := b.reserve()
	b.set(pageOne, transformedPDFObject(fmt.Sprintf(
		"<< /Type /Page /Parent %d 0 R /MediaBox [-20 10 500 700] /Resources << /Font << /F1 %d 0 R >> /XObject << /Outer %d 0 R >> >> /Contents %d 0 R >>",
		pages, font, outer, contents,
	)))
	affineContent := strings.Join([]string{
		"0 0 m 240 0 l S 0 60 m 240 60 l S 0 120 m 240 120 l S",
		"0 -120 m 0 240 l S 120 -120 m 120 240 l S 240 -120 m 240 240 l S",
		"BT /F1 4 Tf 0 1 -1 0 110 65 Tm (Note) Tj ET",
		"BT /F1 10 Tf -1 0 0 1 100 80 Tm (Net) Tj ET",
		"BT /F1 10 Tf -1 0 0 1 55 80 Tm (Income) Tj ET",
		"BT /F1 10 Tf -1 0 0 1 100 100 Tm (Gross) Tj ET",
		"BT /F1 10 Tf -1 0 0 1 65 100 Tm (Profit) Tj ET",
		"BT /F1 5 Tf 0 1 -1 0 200 70 Tm (Second) Tj ET",
		"BT /F1 5 Tf 0 1 -1 0 200 105 Tm (Line) Tj ET",
		"BT /F1 5 Tf 0 1 -1 0 160 70 Tm (Rotated) Tj ET",
		"BT /F1 5 Tf 0 1 -1 0 160 105 Tm (Header) Tj ET",
		"BT /F1 10 Tf 4 Tw 1 0 0 1 10 10 Tm (A ) Tj (B) Tj 1 0 0 1 10 25 Tm (C) Tj ( D) Tj 1 0 0 1 10 40 Tm (E) Tj (F) Tj ET",
		"/SpaceBody Do",
	}, "\n")
	affineContents := b.addStream(affineContent, "")
	pageTwo := b.reserve()
	b.set(pageTwo, transformedPDFObject(fmt.Sprintf(
		"<< /Type /Page /Parent %d 0 R /MediaBox [0 0 300 180] /Resources << /Font << /F1 %d 0 R >> /XObject << /SpaceBody %d 0 R >> >> /Contents %d 0 R >>",
		pages, font, spaceBody, affineContents,
	)))
	b.set(pages, transformedPDFObject(fmt.Sprintf("<< /Type /Pages /Kids [%d 0 R %d 0 R] /Count 2 >>", pageOne, pageTwo)))
	b.set(catalog, transformedPDFObject(fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R >>", pages)))
	return b.bytes(catalog)
}
