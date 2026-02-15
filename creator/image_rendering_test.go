package creator

import (
	"os"
	"testing"
)

// TestImageRendering_Issue36 reproduces the bug from GitHub issue #36.
//
// Bug: creator.DrawImageFit() produces blank PDFs with no errors.
//
// Expected: PDF should contain the rendered image.
// Actual (before fix): PDF is created but contains no visible content.
func TestImageRendering_Issue36(t *testing.T) {
	// Use an existing test image from the reference directory
	imagePath := "../reference/pdfcpu/pkg/testdata/resources/logoSmall.png"
	if _, err := os.Stat(imagePath); err != nil {
		t.Skipf("Test image not found: %s", imagePath)
	}

	// Reproduce the exact code from issue #36
	c := New()
	c.SetTitle("Image Test")

	page, err := c.NewPage()
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	// Load image
	img, err := LoadImage(imagePath)
	if err != nil {
		t.Fatalf("Failed to load image: %v", err)
	}

	// Draw image using DrawImageFit
	if err := page.DrawImageFit(img, 100, 500, 200, 200); err != nil {
		t.Fatalf("Failed to draw image: %v", err)
	}

	// Write PDF
	outputPath := "../tmp/test_image_issue36.pdf"
	os.MkdirAll("../tmp", 0755)
	if err := c.WriteToFile(outputPath); err != nil {
		t.Fatalf("Failed to write PDF: %v", err)
	}

	// Verify PDF was created
	stat, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("PDF file not created: %v", err)
	}

	// PDF should have reasonable size (not empty)
	if stat.Size() < 100 {
		t.Errorf("PDF file is too small (likely empty): %d bytes", stat.Size())
	}

	// TODO: Add content verification by parsing the PDF
	t.Logf("PDF created successfully: %s (%d bytes)", outputPath, stat.Size())
}

// TestDrawImageBasic tests basic image drawing without aspect ratio preservation.
func TestDrawImageBasic(t *testing.T) {
	imagePath := "../reference/pdfcpu/pkg/testdata/resources/mountain.jpg"
	if _, err := os.Stat(imagePath); err != nil {
		t.Skipf("Test image not found: %s", imagePath)
	}

	c := New()
	page, err := c.NewPage()
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	img, err := LoadImage(imagePath)
	if err != nil {
		t.Fatalf("Failed to load image: %v", err)
	}

	// Draw image with explicit dimensions
	if err := page.DrawImage(img, 50, 600, 100, 150); err != nil {
		t.Fatalf("Failed to draw image: %v", err)
	}

	outputPath := "../tmp/test_draw_image.pdf"
	os.MkdirAll("../tmp", 0755)
	if err := c.WriteToFile(outputPath); err != nil {
		t.Fatalf("Failed to write PDF: %v", err)
	}

	t.Logf("PDF created: %s", outputPath)
}
