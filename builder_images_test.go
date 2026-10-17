package gxpdf

import (
	"bytes"
	"compress/zlib"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/coregx/gxpdf/builder"
	"github.com/coregx/gxpdf/internal/parser"
)

// solidPNG returns a w x h PNG filled with c.
func solidPNG(t *testing.T, w, h int, c color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// pdfImage is an image XObject as written in the PDF.
type pdfImage struct {
	objNum             int
	width, height      int
	colorSpace, filter string
	hasSMask           bool
	stream             *parser.Stream
}

// pageImages parses pdf and returns the image XObjects in the resources of
// each page. (It reads the objects directly: the public GetImages does not
// return filtered images yet.)
func pageImages(t *testing.T, pdf []byte) [][]pdfImage {
	t.Helper()
	r, err := parser.OpenPDFFromBytes(pdf)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	n, err := r.GetPageCount()
	if err != nil {
		t.Fatal(err)
	}
	pages := make([][]pdfImage, n)
	for i := 0; i < n; i++ {
		page, err := r.GetPage(i)
		if err != nil {
			t.Fatal(err)
		}
		res, _ := resolveDict(t, r, page.Get("Resources"))
		if res == nil {
			continue
		}
		xobjs, _ := resolveDict(t, r, res.Get("XObject"))
		if xobjs == nil {
			continue
		}
		for _, name := range xobjs.Keys() {
			ref, ok := xobjs.Get(name).(*parser.IndirectReference)
			if !ok {
				t.Fatalf("page %d: XObject %s is not a reference", i+1, name)
			}
			obj, err := r.GetObject(ref.Number)
			if err != nil {
				t.Fatal(err)
			}
			s, ok := obj.(*parser.Stream)
			if !ok {
				t.Fatalf("XObject %d is %T", ref.Number, obj)
			}
			d := s.Dictionary()
			img := pdfImage{
				objNum: ref.Number, stream: s,
				width: int(d.GetInteger("Width")), height: int(d.GetInteger("Height")),
				hasSMask: d.Get("SMask") != nil,
			}
			if v := d.GetName("ColorSpace"); v != nil {
				img.colorSpace = v.Value()
			}
			if v := d.GetName("Filter"); v != nil {
				img.filter = v.Value()
			}
			pages[i] = append(pages[i], img)
		}
	}
	return pages
}

func resolveDict(t *testing.T, r *parser.Reader, obj parser.PdfObject) (*parser.Dictionary, bool) {
	t.Helper()
	if ref, ok := obj.(*parser.IndirectReference); ok {
		var err error
		if obj, err = r.GetObject(ref.Number); err != nil {
			t.Fatal(err)
		}
	}
	d, ok := obj.(*parser.Dictionary)
	return d, ok
}

func inflate(t *testing.T, data []byte) []byte {
	t.Helper()
	zr, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// imagePlacement is one "w 0 0 h x y cm /ImN Do" in a content stream.
type imagePlacement struct{ w, h, x, y float64 }

var placementRe = regexp.MustCompile(`([-\d.]+) 0(?:\.0+)? 0(?:\.0+)? ([-\d.]+) ([-\d.]+) ([-\d.]+) cm\s+/Im\d+ Do`)

// imagePlacements returns the image placements of each page, read from the
// page content streams (Flate, as our writer produces them).
func imagePlacements(t *testing.T, pdf []byte) [][]imagePlacement {
	t.Helper()
	r, err := parser.OpenPDFFromBytes(pdf)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	n, err := r.GetPageCount()
	if err != nil {
		t.Fatal(err)
	}
	out := make([][]imagePlacement, n)
	for i := 0; i < n; i++ {
		page, err := r.GetPage(i)
		if err != nil {
			t.Fatal(err)
		}
		var refs []parser.PdfObject
		switch c := page.Get("Contents").(type) {
		case *parser.Array:
			for k := 0; k < c.Len(); k++ {
				refs = append(refs, c.Get(k))
			}
		default:
			refs = append(refs, c)
		}
		var content []byte
		for _, ref := range refs {
			ir, ok := ref.(*parser.IndirectReference)
			if !ok {
				t.Fatalf("page %d: Contents entry %T", i+1, ref)
			}
			obj, err := r.GetObject(ir.Number)
			if err != nil {
				t.Fatal(err)
			}
			content = append(content, inflate(t, obj.(*parser.Stream).Content())...)
		}
		for _, m := range placementRe.FindAllStringSubmatch(string(content), -1) {
			var v [4]float64
			for k := range v {
				v[k], _ = strconv.ParseFloat(m[k+1], 64)
			}
			out[i] = append(out[i], imagePlacement{v[0], v[1], v[2], v[3]})
		}
	}
	return out
}

func buildDoc(t *testing.T, doc *builder.Builder) []byte {
	t.Helper()
	data, err := doc.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return data
}

// TestBuilderImage_Issue101 is the #101 repro: a 200x100 red PNG at
// FitHeight(20mm) was replaced by a grey rectangle and Build returned nil.
func TestBuilderImage_Issue101(t *testing.T) {
	red := solidPNG(t, 200, 100, color.NRGBA{255, 0, 0, 255})
	doc := builder.NewBuilder(builder.WithPageSize(builder.A4))
	doc.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) {
			c.Text("A red 200x100 PNG should appear below:")
			c.Image(red, builder.FitHeight(builder.Mm(20)))
		})
	})
	pdf := buildDoc(t, doc)

	pages := pageImages(t, pdf)
	if len(pages) != 1 || len(pages[0]) != 1 {
		t.Fatalf("image XObjects per page: %v, want one on the only page", pages)
	}
	img := pages[0][0]
	if img.width != 200 || img.height != 100 || img.colorSpace != "DeviceRGB" || img.filter != "FlateDecode" {
		t.Errorf("image %dx%d %s %s, want 200x100 DeviceRGB FlateDecode", img.width, img.height, img.colorSpace, img.filter)
	}
	px := inflate(t, img.stream.Content())
	if len(px) != 200*100*3 {
		t.Fatalf("%d pixel bytes, want %d", len(px), 200*100*3)
	}
	if mid := (50*200 + 100) * 3; px[mid] != 255 || px[mid+1] != 0 || px[mid+2] != 0 {
		t.Errorf("pixel (100,50) = %v, want red", px[mid:mid+3])
	}

	pl := imagePlacements(t, pdf)[0]
	if len(pl) != 1 {
		t.Fatalf("%d placements, want 1", len(pl))
	}
	wantH := 20 * 72 / 25.4
	if math.Abs(pl[0].h-wantH) > 0.01 || math.Abs(pl[0].w-2*wantH) > 0.01 {
		t.Errorf("drawn %.3f x %.3f pt, want %.3f x %.3f (20 mm tall, 2:1)", pl[0].w, pl[0].h, 2*wantH, wantH)
	}
	if bytes.Contains(pdf, []byte("0.90 0.90 0.90 rg")) {
		t.Error("placeholder rectangle still drawn")
	}
}

// TestBuilderImage_LogoSharedAcrossPagesAndPaths places one logo in the
// header of a multi-page body, in a Row/Col cell and in a second page
// definition: every placement draws, and all refer to one image XObject.
func TestBuilderImage_LogoSharedAcrossPagesAndPaths(t *testing.T) {
	logo := solidPNG(t, 120, 40, color.NRGBA{0, 0, 200, 255})
	doc := builder.NewBuilder(builder.WithPageSize(builder.A4))
	header := func(p *builder.PageBuilder) {
		p.Header(func(c *builder.Container) { c.Image(logo, builder.FitHeight(builder.Mm(8))) })
	}
	doc.Page(func(p *builder.PageBuilder) {
		header(p)
		p.Content(func(c *builder.Container) {
			c.Row(func(r *builder.RowBuilder) {
				r.Col(6, func(col *builder.ColBuilder) { col.Image(logo, builder.FitWidth(builder.Mm(30))) })
				r.Col(6, func(col *builder.ColBuilder) { col.Text("text next to the logo") })
			})
			for i := 0; i < 90; i++ {
				c.Text("Body line long enough to need several pages of content.")
			}
		})
	})
	doc.Page(func(p *builder.PageBuilder) { // a second definition, same bytes in a new slice
		header(p)
		p.Content(func(c *builder.Container) { c.Image(append([]byte(nil), logo...)) })
	})
	pdf := buildDoc(t, doc)

	pages := pageImages(t, pdf)
	if len(pages) < 3 {
		t.Fatalf("%d pages, want the first definition to span at least 2", len(pages))
	}
	objs := map[int]bool{}
	for i, imgs := range pages {
		if len(imgs) == 0 {
			t.Errorf("page %d has no image resource", i+1)
		}
		for _, img := range imgs {
			objs[img.objNum] = true
		}
	}
	if len(objs) != 1 {
		t.Errorf("image objects %v, want one shared by all pages", objs)
	}
	// Every page has its header logo; the first page also the Row/Col logo,
	// the last page (second definition) also its body logo.
	placements := imagePlacements(t, pdf)
	last := len(placements) - 1
	for i, pl := range placements {
		want := 1
		if i == 0 || i == last {
			want = 2
		}
		if len(pl) != want {
			t.Errorf("page %d: %d image placements, want %d", i+1, len(pl), want)
		}
		if len(pl) > 0 && math.Abs(pl[0].h-8*72/25.4) > 0.01 {
			t.Errorf("page %d: header logo %.3f pt tall, want 8 mm", i+1, pl[0].h)
		}
	}
}

func TestBuilderImage_JPEGAndTransparentPNG(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 64, 32))
	for i := range src.Pix {
		src.Pix[i] = 255
	}
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, src, nil); err != nil {
		t.Fatal(err)
	}
	translucent := solidPNG(t, 10, 10, color.NRGBA{0, 128, 0, 100})

	doc := builder.NewBuilder()
	doc.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) {
			c.Image(jpg.Bytes(), builder.FitWidth(builder.Mm(40)))
			c.Image(translucent, builder.FitWidth(builder.Mm(10)))
		})
	})
	pdf := buildDoc(t, doc)

	var got []string
	for _, img := range pageImages(t, pdf)[0] {
		got = append(got, strconv.Itoa(img.width)+"x"+strconv.Itoa(img.height)+" "+img.filter+" smask="+strconv.FormatBool(img.hasSMask))
		if img.filter == "DCTDecode" && !bytes.Equal(img.stream.Content(), jpg.Bytes()) {
			t.Error("JPEG bytes not embedded as they are")
		}
	}
	sort.Strings(got)
	want := []string{"10x10 FlateDecode smask=true", "64x32 DCTDecode smask=false"}
	if strings.Join(got, "; ") != strings.Join(want, "; ") {
		t.Errorf("images %q, want %q", got, want)
	}
}

func TestBuilderImage_BadDataFailsBuild(t *testing.T) {
	doc := builder.NewBuilder()
	doc.Page(func(p *builder.PageBuilder) {
		p.Header(func(c *builder.Container) { c.Image([]byte("not an image")) })
		p.Content(func(c *builder.Container) { c.Text("body") })
	})
	if _, err := doc.Build(); err == nil || !strings.Contains(err.Error(), "image (12 bytes)") {
		t.Fatalf("Build error = %v, want an image error", err)
	}
}
