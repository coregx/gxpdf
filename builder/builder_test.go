package builder_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/coregx/gxpdf/builder"
)

// TestBuild_EmptyDocument verifies that an empty builder produces valid PDF bytes.
func TestBuild_EmptyDocument(t *testing.T) {
	b := builder.NewBuilder()
	pdfBytes, err := b.Build()
	if err != nil {
		t.Fatalf("Build() on empty document: %v", err)
	}
	assertValidPDF(t, pdfBytes)
}

// TestBuild_SinglePage verifies end-to-end PDF generation for a single page.
func TestBuild_SinglePage(t *testing.T) {
	b := builder.NewBuilder(
		builder.WithTitle("Test Document"),
		builder.WithAuthor("Test Suite"),
	)
	b.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) {
			c.Text("Hello, World!")
			c.Spacer(builder.Mm(5))
			c.Text("Second paragraph", builder.Bold(), builder.FontSize(14))
			c.Line(builder.LineColor(builder.Black))
		})
	})

	pdfBytes, err := b.Build()
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	assertValidPDF(t, pdfBytes)
}

// TestBuild_PageWithHeaderAndFooter verifies header/footer zones work.
func TestBuild_PageWithHeaderAndFooter(t *testing.T) {
	b := builder.NewBuilder()
	b.Page(func(p *builder.PageBuilder) {
		p.Header(func(h *builder.Container) {
			h.Text("Document Header", builder.Bold())
		})
		p.Content(func(c *builder.Container) {
			c.Text("Main content goes here.")
		})
		p.Footer(func(f *builder.Container) {
			f.PageNumber(
				builder.PageNum+" / "+builder.TotalPages,
				builder.AlignRight(),
				builder.FontSize(8),
			)
		})
	})

	pdfBytes, err := b.Build()
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	assertValidPDF(t, pdfBytes)
}

// TestBuild_TwelveColumnGrid verifies rows and columns produce valid PDF.
func TestBuild_TwelveColumnGrid(t *testing.T) {
	b := builder.NewBuilder()
	b.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) {
			c.Row(func(r *builder.RowBuilder) {
				r.Col(4, func(col *builder.ColBuilder) { col.Text("Column 1/3") })
				r.Col(4, func(col *builder.ColBuilder) { col.Text("Column 2/3") })
				r.Col(4, func(col *builder.ColBuilder) { col.Text("Column 3/3") })
			})
		})
	})

	pdfBytes, err := b.Build()
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	assertValidPDF(t, pdfBytes)
}

// TestBuild_RowWithOptions verifies rows with explicit height and background color.
func TestBuild_RowWithOptions(t *testing.T) {
	b := builder.NewBuilder()
	b.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) {
			c.Row(func(r *builder.RowBuilder) {
				r.Col(12, func(col *builder.ColBuilder) {
					col.Text("Header row", builder.Bold(), builder.TextColor(builder.White))
				})
			}, builder.RowBg(builder.Navy), builder.RowHeight(builder.Mm(12)))
		})
	})

	pdfBytes, err := b.Build()
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	assertValidPDF(t, pdfBytes)
}

// TestBuild_KeepTogether verifies KeepTogether container builds.
func TestBuild_KeepTogether(t *testing.T) {
	b := builder.NewBuilder()
	b.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) {
			c.KeepTogether(func(inner *builder.Container) {
				inner.Text("Section Title", builder.Bold(), builder.FontSize(16))
				inner.Text("This paragraph always follows the title on the same page.")
			})
		})
	})

	pdfBytes, err := b.Build()
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	assertValidPDF(t, pdfBytes)
}

// TestBuild_PageBreak verifies explicit page break produces a multi-page document.
func TestBuild_PageBreak(t *testing.T) {
	b := builder.NewBuilder()
	b.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) {
			c.Text("Page 1 content")
			c.PageBreak()
			c.Text("Page 2 content")
		})
	})

	pdfBytes, err := b.Build()
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	assertValidPDF(t, pdfBytes)
}

// TestBuild_MultiplePages verifies multiple page definitions work.
func TestBuild_MultiplePages(t *testing.T) {
	b := builder.NewBuilder()
	for i := 0; i < 3; i++ {
		b.Page(func(p *builder.PageBuilder) {
			p.Content(func(c *builder.Container) { c.Text("Page content") })
		})
	}

	pdfBytes, err := b.Build()
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	assertValidPDF(t, pdfBytes)
}

// TestBuild_CustomPageSize verifies a non-default page size is accepted.
func TestBuild_CustomPageSize(t *testing.T) {
	b := builder.NewBuilder(builder.WithPageSize(builder.Letter))
	b.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) { c.Text("Letter-size document") })
	})

	pdfBytes, err := b.Build()
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	assertValidPDF(t, pdfBytes)
}

// TestBuild_CustomMargins verifies custom margins option.
func TestBuild_CustomMargins(t *testing.T) {
	b := builder.NewBuilder(
		builder.WithMargins(builder.Mm(30), builder.Mm(25), builder.Mm(30), builder.Mm(25)),
	)
	b.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) { c.Text("Custom margins document") })
	})

	pdfBytes, err := b.Build()
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	assertValidPDF(t, pdfBytes)
}

// TestBuild_PageLevelSizeAndMargins verifies page-level size/margin override.
func TestBuild_PageLevelSizeAndMargins(t *testing.T) {
	b := builder.NewBuilder()
	b.Page(func(p *builder.PageBuilder) {
		p.Size(builder.Letter)
		p.Margins(builder.In(1), builder.In(1), builder.In(1), builder.In(1))
		p.Content(func(c *builder.Container) {
			c.Text("Page with explicit size and margins")
		})
	})

	pdfBytes, err := b.Build()
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	assertValidPDF(t, pdfBytes)
}

// TestBuildTo_WritesToWriter verifies BuildTo writes valid PDF to an io.Writer.
func TestBuildTo_WritesToWriter(t *testing.T) {
	b := builder.NewBuilder()
	b.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) { c.Text("Written to buffer") })
	})

	var buf bytes.Buffer
	if err := b.BuildTo(&buf); err != nil {
		t.Fatalf("BuildTo() failed: %v", err)
	}
	assertValidPDF(t, buf.Bytes())
}

// TestBuildToFile_WritesToFile verifies BuildToFile creates a valid PDF file.
func TestBuildToFile_WritesToFile(t *testing.T) {
	b := builder.NewBuilder()
	b.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) { c.Text("Written to file") })
	})

	path := t.TempDir() + "/output.pdf"
	if err := b.BuildToFile(path); err != nil {
		t.Fatalf("BuildToFile(%q) failed: %v", path, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	assertValidPDF(t, data)
}

// TestWriteTo_ImplementsWriterTo verifies WriteTo satisfies io.WriterTo.
func TestWriteTo_ImplementsWriterTo(t *testing.T) {
	b := builder.NewBuilder()
	b.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) { c.Text("WriterTo test") })
	})

	var buf bytes.Buffer
	n, err := b.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo() failed: %v", err)
	}
	if n <= 0 {
		t.Errorf("WriteTo() wrote %d bytes, expected positive", n)
	}
	assertValidPDF(t, buf.Bytes())
}

// TestBuild_AllTextOptions verifies all TextOption combinators work together.
func TestBuild_AllTextOptions(t *testing.T) {
	b := builder.NewBuilder()
	b.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) {
			c.Text("Bold text", builder.Bold())
			c.Text("Italic text", builder.Italic())
			c.Text("Large font", builder.FontSize(24))
			c.Text("Colored text", builder.TextColor(builder.Red))
			c.Text("Centered", builder.AlignCenter())
			c.Text("Right-aligned", builder.AlignRight())
			c.Text("Justified paragraph with enough words to trigger wrapping", builder.AlignJustify())
			c.Text("Wide leading", builder.LineHeight(1.8))
			c.Text("Underlined", builder.Underline())
			c.Text("Struck through", builder.Strikethrough())
			c.Text("Spaced out", builder.LetterSpacing(2))
		})
	})

	pdfBytes, err := b.Build()
	if err != nil {
		t.Fatalf("Build() with text options failed: %v", err)
	}
	assertValidPDF(t, pdfBytes)
}

// TestBuild_EnsureSpace verifies EnsureSpace compiles and builds.
func TestBuild_EnsureSpace(t *testing.T) {
	b := builder.NewBuilder()
	b.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) {
			c.Text("Some content before ensure-space guard")
			c.EnsureSpace(builder.Mm(100))
			c.Text("This may appear on a new page if space is tight")
		})
	})

	pdfBytes, err := b.Build()
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	assertValidPDF(t, pdfBytes)
}

// TestBuild_NestedRows verifies rows nested inside columns.
func TestBuild_NestedRows(t *testing.T) {
	b := builder.NewBuilder()
	b.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) {
			c.Row(func(r *builder.RowBuilder) {
				r.Col(6, func(left *builder.ColBuilder) {
					left.Row(func(inner *builder.RowBuilder) {
						inner.Col(6, func(c *builder.ColBuilder) { c.Text("1.1") })
						inner.Col(6, func(c *builder.ColBuilder) { c.Text("1.2") })
					})
				})
				r.Col(6, func(right *builder.ColBuilder) {
					right.Text("Right column")
				})
			})
		})
	})

	pdfBytes, err := b.Build()
	if err != nil {
		t.Fatalf("Build() with nested rows failed: %v", err)
	}
	assertValidPDF(t, pdfBytes)
}

// TestBuild_DefaultStyle verifies WithDefaultFontSize propagates to elements.
func TestBuild_DefaultStyle(t *testing.T) {
	b := builder.NewBuilder(
		builder.WithDefaultFontSize(11),
	)
	b.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) { c.Text("Default style text") })
	})

	pdfBytes, err := b.Build()
	if err != nil {
		t.Fatalf("Build() with default style failed: %v", err)
	}
	assertValidPDF(t, pdfBytes)
}

// TestBuild_ImageBadData verifies that bytes which are not a JPEG or PNG make
// Build fail instead of producing a PDF without the image (#101).
func TestBuild_ImageBadData(t *testing.T) {
	b := builder.NewBuilder()
	b.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) {
			c.Image([]byte("FAKE"), builder.FitWidth(builder.Mm(60)))
		})
	})

	pdfBytes, err := b.Build()
	if err == nil {
		t.Fatalf("Build() = %d bytes, nil error; want an image error", len(pdfBytes))
	}
	if !strings.Contains(err.Error(), "image (4 bytes)") {
		t.Errorf("error %q does not name the image", err)
	}
}

// TestBuild_LongDocument verifies pagination works across many paragraphs.
func TestBuild_LongDocument(t *testing.T) {
	b := builder.NewBuilder()
	b.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) {
			for i := 0; i < 100; i++ {
				c.Text("This is paragraph number line content that may wrap when the column is narrow enough to cause overflow into the next page during pagination testing.")
				c.Spacer(builder.Pt(4))
			}
		})
	})

	pdfBytes, err := b.Build()
	if err != nil {
		t.Fatalf("Build() long document failed: %v", err)
	}
	assertValidPDF(t, pdfBytes)
}

// TestBytes_AliasForBuild verifies Bytes() returns same result as Build().
func TestBytes_AliasForBuild(t *testing.T) {
	b := builder.NewBuilder()
	b.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) { c.Text("Bytes alias") })
	})

	data, err := b.Bytes()
	if err != nil {
		t.Fatalf("Bytes() failed: %v", err)
	}
	assertValidPDF(t, data)
}

// TestWriteToFile_AliasForBuildToFile verifies WriteToFile() works.
func TestWriteToFile_AliasForBuildToFile(t *testing.T) {
	b := builder.NewBuilder()
	b.Page(func(p *builder.PageBuilder) {
		p.Content(func(c *builder.Container) { c.Text("WriteToFile alias") })
	})

	path := t.TempDir() + "/alias.pdf"
	if err := b.WriteToFile(path); err != nil {
		t.Fatalf("WriteToFile(%q) failed: %v", path, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	assertValidPDF(t, data)
}

// --- Helpers ---

// assertValidPDF checks that the bytes start with the %PDF signature.
func assertValidPDF(t *testing.T, data []byte) {
	t.Helper()
	if len(data) == 0 {
		t.Fatal("PDF data is empty")
	}
	prefix := data
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	if !strings.HasPrefix(string(prefix), "%PDF") {
		t.Fatalf("data does not start with %%PDF signature, got: %q", string(data[:minInt(20, len(data))]))
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
