// Package main demonstrates images in the Builder API: a report whose logo
// repeats in the header of every page, a photo next to text in a grid row,
// and the sizing options.
//
// It shows:
//   - JPEG and PNG (with transparency) embedded as image XObjects
//   - one logo in every page header stored once in the PDF
//   - FitWidth, FitHeight and both (the image fits inside the box, keeping
//     its aspect ratio); without options an image takes its pixel size,
//     1 pixel = 1 point, but never more than the available width
//   - an image taller than the page scaled down to the page
//
// The images are generated in code, so the example needs no files.
//
// Usage:
//
//	go run ./examples/builder-images [output.pdf]
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log"
	"math"
	"os"

	"github.com/coregx/gxpdf/builder"
)

func main() {
	out := "builder-images.pdf"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}

	logo := logoPNG(240, 80)
	photo := photoJPEG(400, 240)
	tall := photoJPEG(100, 1600)

	b := builder.NewBuilder(
		builder.WithPageSize(builder.A4),
		builder.WithTitle("Builder images"),
	)
	b.Page(func(p *builder.PageBuilder) {
		p.Header(func(c *builder.Container) {
			c.Row(func(r *builder.RowBuilder) {
				r.Col(3, func(col *builder.ColBuilder) {
					col.Image(logo, builder.FitHeight(builder.Mm(10)))
				})
				r.Col(9, func(col *builder.ColBuilder) {
					col.Text("Quarterly report — the logo is a PNG with transparency, stored once", builder.FontSize(9))
				})
			})
			c.Line()
		})
		p.Content(func(c *builder.Container) {
			c.Text("Photo next to text", builder.Bold(), builder.FontSize(14))
			c.Row(func(r *builder.RowBuilder) {
				r.Col(5, func(col *builder.ColBuilder) {
					col.Image(photo, builder.FitWidth(builder.Mm(70)))
				})
				r.Col(7, func(col *builder.ColBuilder) {
					col.Text("A 400x240 JPEG at FitWidth(70mm): the height follows from its 5:3 aspect ratio. JPEG data is embedded as it is (DCTDecode).")
				})
			})
			c.Spacer(builder.Mm(6))

			c.Text("Sizing options for the same 400x240 photo", builder.Bold(), builder.FontSize(14))
			c.Text("FitHeight(15mm):")
			c.Image(photo, builder.FitHeight(builder.Mm(15)))
			c.Text("FitWidth(40mm) and FitHeight(15mm) — fits inside the 40x15mm box:")
			c.Image(photo, builder.FitWidth(builder.Mm(40)), builder.FitHeight(builder.Mm(15)))
			c.Text("No options — 400x240 points:")
			c.Image(photo)

			c.Text("A 100x1600 image is taller than a page: it is scaled down to fit the page.", builder.Bold())
			c.Image(tall)

			for i := 1; i <= 40; i++ {
				c.Text(fmt.Sprintf("Body paragraph %d — more pages, each with the logo in its header.", i))
			}
		})
	})

	if err := b.BuildToFile(out); err != nil {
		log.Fatalf("build: %v", err)
	}
	fmt.Println("wrote", out)
}

// logoPNG draws a blue disc and an orange bar on a transparent background.
func logoPNG(w, h int) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	r := float64(h) / 2.2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := float64(x)-float64(h)/2, float64(y)-float64(h)/2
			switch {
			case math.Hypot(dx, dy) < r:
				img.SetNRGBA(x, y, color.NRGBA{20, 70, 160, 255})
			case x > h && x < w-10 && y > h/3 && y < 2*h/3:
				img.SetNRGBA(x, y, color.NRGBA{240, 130, 20, 220})
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		log.Fatal(err)
	}
	return buf.Bytes()
}

// photoJPEG draws a colour gradient with a few circles.
func photoJPEG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{uint8(255 * x / w), uint8(255 * y / h), 160, 255}
			if (x/40+y/40)%2 == 0 {
				c.B = 200
			}
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		log.Fatal(err)
	}
	return buf.Bytes()
}
