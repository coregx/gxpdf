// Package main demonstrates GxPDF enterprise capabilities with Unicode font support.
//
// This showcase creates a professional PDF document demonstrating:
// - Embedded TrueType fonts with full Unicode support
// - Cyrillic, CJK (Chinese, Japanese, Korean), and special symbols
// - Professional tables with borders
// - Graphics, diagrams, charts, and colors
package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"runtime"

	"github.com/coregx/gxpdf/creator"
)

// Values repeated across the feature, spec and platform tables.
const (
	featureTableExtraction = "Table Extraction"
	labelDependencies      = "Dependencies"
	depsZero               = "Zero"
	unicodeFullBMP         = "Full BMP"
	supportFull            = "Full"
	supportYes             = "Yes"
)

// Corporate color palette - Professional enterprise design.
var (
	// Primary colors.
	NavyDark   = creator.Color{R: 0.09, G: 0.12, B: 0.22} // #171E38 - Dark navy for headers
	NavyMedium = creator.Color{R: 0.15, G: 0.20, B: 0.32} // Background accent
	AccentGold = creator.Color{R: 0.80, G: 0.62, B: 0.25} // #CC9E40 - Gold accent
	AccentBlue = creator.Color{R: 0.25, G: 0.52, B: 0.77} // #4085C5 - Blue for icons
	AccentTeal = creator.Color{R: 0.20, G: 0.60, B: 0.60} // Teal accent

	// Chart colors.
	ChartBlue   = creator.Color{R: 0.30, G: 0.55, B: 0.85}
	ChartGreen  = creator.Color{R: 0.30, G: 0.70, B: 0.45}
	ChartOrange = creator.Color{R: 0.95, G: 0.55, B: 0.20}
	ChartPurple = creator.Color{R: 0.55, G: 0.35, B: 0.75}
	ChartRed    = creator.Color{R: 0.85, G: 0.30, B: 0.30}
	ChartCyan   = creator.Color{R: 0.20, G: 0.70, B: 0.80}

	// Text colors.
	White     = creator.Color{R: 1.00, G: 1.00, B: 1.00}
	TextDark  = creator.Color{R: 0.20, G: 0.20, B: 0.20}
	TextGray  = creator.Color{R: 0.45, G: 0.45, B: 0.45}
	TextMuted = creator.Color{R: 0.60, G: 0.60, B: 0.60}
	TextLight = creator.Color{R: 0.75, G: 0.75, B: 0.75}

	// Table colors.
	TableHeader = creator.Color{R: 0.94, G: 0.96, B: 0.98}
	TableBorder = creator.Color{R: 0.85, G: 0.85, B: 0.85}
	TableStripe = creator.Color{R: 0.98, G: 0.98, B: 0.98}
	LightGray   = creator.Color{R: 0.95, G: 0.95, B: 0.95}

	// Status colors.
	SuccessGreen  = creator.Color{R: 0.18, G: 0.62, B: 0.35}
	WarningOrange = creator.Color{R: 0.90, G: 0.55, B: 0.15}
)

// Fonts structure to hold all loaded fonts.
type Fonts struct {
	Regular *creator.CustomFont
	Bold    *creator.CustomFont
	CJK     *creator.CustomFont
}

func main() {
	fonts, err := loadFonts()
	if err != nil {
		log.Fatalf("Failed to load fonts: %v", err)
	}

	c := creator.New()
	c.SetTitle("GxPDF - Enterprise PDF Library for Go")
	c.SetAuthor("CoreGX Technologies")
	c.SetSubject("Professional PDF Generation with Full Unicode Support")

	// Page 1: Hero/Title page.
	if err := createHeroPage(c, fonts); err != nil {
		log.Fatalf("Failed to create hero page: %v", err)
	}

	// Page 2: Core Features.
	if err := createFeaturesPage(c, fonts); err != nil {
		log.Fatalf("Failed to create features page: %v", err)
	}

	// Page 3: Performance Dashboard with Charts.
	if err := createDashboardPage(c, fonts); err != nil {
		log.Fatalf("Failed to create dashboard page: %v", err)
	}

	// Page 4: Unicode Support.
	if err := createUnicodePage(c, fonts); err != nil {
		log.Fatalf("Failed to create unicode page: %v", err)
	}

	// Page 5: Technical Specifications.
	if err := createSpecsPage(c, fonts); err != nil {
		log.Fatalf("Failed to create specs page: %v", err)
	}

	// Page 6: Architecture Overview.
	if err := createArchitecturePage(c, fonts); err != nil {
		log.Fatalf("Failed to create architecture page: %v", err)
	}

	// Page 7: API Examples.
	if err := createAPIPage(c, fonts); err != nil {
		log.Fatalf("Failed to create API page: %v", err)
	}

	// Write PDF.
	outputPath := "assets/gxpdf_enterprise_brochure.pdf"
	if err := c.WriteToFile(outputPath); err != nil {
		log.Fatalf("Failed to write PDF: %v", err)
	}

	fmt.Printf("Created: %s\n", outputPath)
}

// loadFonts loads all required fonts.
func loadFonts() (*Fonts, error) {
	fonts := &Fonts{}

	fontPath := findFont([]string{
		"C:/Windows/Fonts/arial.ttf",
		"/Library/Fonts/Arial.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	})
	if fontPath == "" {
		return nil, fmt.Errorf("no Unicode font found")
	}
	fmt.Printf("Main font: %s\n", fontPath)

	var err error
	fonts.Regular, err = creator.LoadFont(fontPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load regular font: %w", err)
	}

	boldPath := findFont([]string{
		"C:/Windows/Fonts/arialbd.ttf",
		"/Library/Fonts/Arial Bold.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
	})
	if boldPath != "" {
		fonts.Bold, err = creator.LoadFont(boldPath)
		if err != nil {
			fonts.Bold = fonts.Regular
		} else {
			fmt.Printf("Bold font: %s\n", boldPath)
		}
	} else {
		fonts.Bold = fonts.Regular
	}

	cjkPath := findFont([]string{
		"C:/Windows/Fonts/malgun.ttf",
		"C:/Windows/Fonts/msyh.ttc",
		"/System/Library/Fonts/PingFang.ttc",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
	})
	if cjkPath != "" {
		fonts.CJK, err = creator.LoadFont(cjkPath)
		if err != nil {
			fonts.CJK = nil
		} else {
			fmt.Printf("CJK font: %s\n", cjkPath)
		}
	}

	return fonts, nil
}

func findFont(paths []string) string {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// =============================================================================
// PAGE 1: HERO PAGE
// =============================================================================

func createHeroPage(c *creator.Creator, fonts *Fonts) error {
	page, err := c.NewPage()
	if err != nil {
		return err
	}

	width := page.Width()
	height := page.Height()

	// --- DARK HEADER SECTION (top 42% of page) ---
	headerHeight := height * 0.42
	if err := page.DrawRect(0, height-headerHeight, width, headerHeight, &creator.RectOptions{
		FillColor: &NavyDark,
	}); err != nil {
		return err
	}

	// --- LOGO ICON (centered blue square with gold inner square) ---
	logoSize := 75.0
	logoX := (width - logoSize) / 2
	logoY := height - 35 // Logo at top of header

	// Blue outer square with subtle border.
	if err := page.DrawRect(logoX, logoY-logoSize, logoSize, logoSize, &creator.RectOptions{
		FillColor: &AccentBlue,
	}); err != nil {
		return err
	}

	// Gold inner square.
	innerSize := 30.0
	innerX := logoX + (logoSize-innerSize)/2
	innerY := logoY - logoSize + (logoSize-innerSize)/2
	if err := page.DrawRect(innerX, innerY, innerSize, innerSize, &creator.RectOptions{
		FillColor: &AccentGold,
	}); err != nil {
		return err
	}

	// --- MAIN TITLE (fixed position, not relative to logo) ---
	titleY := height - 180.0
	if err := drawCenteredText(page, "GxPDF", width, titleY, fonts.Bold, 56, White); err != nil {
		return err
	}

	// --- SUBTITLE ---
	subtitleY := titleY - 45
	if err := drawCenteredText(page, "Enterprise PDF Library for Go", width, subtitleY, fonts.Regular, 20, White); err != nil {
		return err
	}

	// --- TAGLINE (three parts with separators) ---
	taglineY := subtitleY - 50
	tagline := "Pure Go    |    Zero Dependencies    |    Production Ready"
	if err := drawCenteredText(page, tagline, width, taglineY, fonts.Regular, 12, AccentGold); err != nil {
		return err
	}

	// --- GOLD ACCENT LINE ---
	lineY := height - headerHeight - 4
	if err := page.DrawRect(0, lineY, width, 4, &creator.RectOptions{
		FillColor: &AccentGold,
	}); err != nil {
		return err
	}

	// --- WHITE CONTENT SECTION ---
	contentY := lineY - 50

	// Section title.
	if err := page.AddTextCustomFontColor("Why GxPDF?", 60, contentY, fonts.Bold, 20, NavyDark); err != nil {
		return err
	}

	// Feature items with icons.
	contentY -= 45
	featureItems := []struct {
		title string
		desc  string
	}{
		{"100% Accuracy", "Table extraction tested on 740+ bank transactions with perfect accuracy"},
		{"Enterprise Security", "AES-256 & RC4 encryption with full permission control"},
		{"Full Featured", "Create, read, merge, split, encrypt, watermark, and more"},
		{"Unicode Support", "Full support for Latin, Cyrillic, Greek, CJK, and symbols"},
		{"Zero Dependencies", "Pure Go implementation using only standard library"},
		{"High Performance", "Process 500+ pages per second with minimal memory footprint"},
	}

	for _, item := range featureItems {
		contentY = drawFeatureItem(page, fonts, 60, contentY, item.title, item.desc)
		contentY -= 15
	}

	// --- BOTTOM STATS BAR ---
	statsBottom := 55.0 // Above footer (footer line at y=25)
	statsHeight := 50.0
	statsTop := statsBottom + statsHeight // = 90
	if err := page.DrawRect(0, statsBottom, width, statsHeight, &creator.RectOptions{
		FillColor: &LightGray,
	}); err != nil {
		return err
	}

	// Stats - vertically centered in gray bar.
	stats := []struct {
		value string
		label string
	}{
		{"740+", "Test Transactions"},
		{"100%", "Accuracy"},
		{"500+", "Pages/Second"},
		{"0", labelDependencies},
	}

	statWidth := width / float64(len(stats))
	centerY := statsBottom + statsHeight/2
	for i, stat := range stats {
		statX := float64(i)*statWidth + statWidth/2
		_ = drawCenteredText(page, stat.value, statX*2, centerY+5, fonts.Bold, 16, AccentBlue)
		_ = drawCenteredText(page, stat.label, statX*2, centerY-12, fonts.Regular, 8, TextGray)
	}
	_ = statsTop // suppress unused

	drawFooter(page, fonts, 1, 7)
	return nil
}

// drawFeatureItem draws a feature with blue bullet, title, and description.
func drawFeatureItem(page *creator.Page, fonts *Fonts, x, y float64, title, description string) float64 {
	// Blue square bullet - aligned with title text.
	bulletSize := 10.0
	_ = page.DrawRect(x, y-1, bulletSize, bulletSize, &creator.RectOptions{
		FillColor: &AccentBlue,
	})

	// Title (bold).
	_ = page.AddTextCustomFontColor(title, x+20, y, fonts.Bold, 13, TextDark)

	// Description (muted).
	_ = page.AddTextCustomFontColor(description, x+20, y-17, fonts.Regular, 10, TextMuted)

	return y - 20
}

// =============================================================================
// PAGE 2: CORE FEATURES
// =============================================================================

func createFeaturesPage(c *creator.Creator, fonts *Fonts) error {
	page, err := c.NewPage()
	if err != nil {
		return err
	}

	width := page.Width()
	height := page.Height()

	// --- PAGE HEADER ---
	drawPageHeader(page, fonts, "Core Features", "Comprehensive PDF manipulation capabilities")

	// --- FEATURE CARDS (2 columns, 3 rows) ---
	cardWidth := (width - 130) / 2
	cardHeight := 145.0
	startY := height - 120
	gap := 12.0

	// Row 1.
	drawFeatureCard(page, fonts, 50, startY, cardWidth, cardHeight,
		"Document Creation",
		[]string{
			"Create PDFs from scratch",
			"Rich text with custom fonts",
			"Tables with auto-layout",
			"Images with alpha channel",
			"Form fields and annotations",
		})

	drawFeatureCard(page, fonts, 65+cardWidth, startY, cardWidth, cardHeight,
		"Content Extraction",
		[]string{
			"100% accurate table extraction",
			"Text extraction with positioning",
			"Image extraction (JPEG, PNG)",
			"Metadata and XMP parsing",
			"Font information extraction",
		})

	// Row 2.
	startY -= cardHeight + gap
	drawFeatureCard(page, fonts, 50, startY, cardWidth, cardHeight,
		"Document Processing",
		[]string{
			"Merge multiple PDFs",
			"Split by page ranges",
			"Page rotation & reordering",
			"Watermarks and stamps",
			"Flatten form fields",
		})

	drawFeatureCard(page, fonts, 65+cardWidth, startY, cardWidth, cardHeight,
		"Security Features",
		[]string{
			"AES-256 encryption",
			"RC4 40/128-bit encryption",
			"Password protection",
			"Permission controls",
			"Digital signatures",
		})

	// Row 3.
	startY -= cardHeight + gap
	drawFeatureCard(page, fonts, 50, startY, cardWidth, cardHeight,
		"Font Support",
		[]string{
			"Standard 14 PDF fonts",
			"TrueType embedding",
			"OpenType embedding",
			"Font subsetting",
			"Full Unicode (BMP)",
		})

	drawFeatureCard(page, fonts, 65+cardWidth, startY, cardWidth, cardHeight,
		"Image Support",
		[]string{
			"JPEG with quality control",
			"PNG with alpha channel",
			"Image scaling & positioning",
			"Color space handling",
			"Inline and XObject images",
		})

	// --- BOTTOM COMPARISON TABLE ---
	startY -= cardHeight + 30
	_ = page.AddTextCustomFontColor("Comparison with Alternatives", 50, startY, fonts.Bold, 14, NavyDark)

	startY -= 20
	comparison := [][]string{
		{"Feature", "GxPDF", "UniPDF", "pdfcpu"},
		{featureTableExtraction, "Yes (100%)", supportYes, "No"},
		{"Unicode Support", unicodeFullBMP, unicodeFullBMP, "Limited"},
		{"License", "MIT", "Commercial", "Apache 2.0"},
		{labelDependencies, depsZero, "Multiple", depsZero},
	}
	_, _ = drawTable(page, fonts, 50, startY, width-100, comparison)

	drawFooter(page, fonts, 2, 7)
	return nil
}

// drawFeatureCard draws a feature card with title and bullet points.
//
//nolint:unparam // h is parameterized for reusability
func drawFeatureCard(page *creator.Page, fonts *Fonts, x, y, w, h float64, title string, items []string) {
	// Card background with subtle shadow effect.
	_ = page.DrawRect(x+2, y-h-2, w, h, &creator.RectOptions{
		FillColor: &TableBorder,
	})
	_ = page.DrawRect(x, y-h, w, h, &creator.RectOptions{
		FillColor:   &White,
		StrokeColor: &TableBorder,
		StrokeWidth: 1,
	})

	// Title bar.
	titleBarHeight := 32.0
	_ = page.DrawRect(x, y-titleBarHeight, w, titleBarHeight, &creator.RectOptions{
		FillColor: &NavyDark,
	})

	// Title text.
	_ = page.AddTextCustomFontColor(title, x+12, y-22, fonts.Bold, 12, White)

	// Bullet items.
	itemY := y - titleBarHeight - 18
	for _, item := range items {
		// Small gold bullet - aligned with text.
		_ = page.DrawRect(x+12, itemY+1, 5, 5, &creator.RectOptions{
			FillColor: &AccentGold,
		})
		_ = page.AddTextCustomFontColor(item, x+24, itemY, fonts.Regular, 9, TextDark)
		itemY -= 18
	}
}

// =============================================================================
// PAGE 3: PERFORMANCE DASHBOARD
// =============================================================================

func createDashboardPage(c *creator.Creator, fonts *Fonts) error {
	page, err := c.NewPage()
	if err != nil {
		return err
	}

	width := page.Width()
	height := page.Height()

	drawPageHeader(page, fonts, "Performance Dashboard", "Benchmarks and metrics")

	// === LEFT COLUMN (x: 50 to width/2 - 20) ===
	leftColWidth := width/2 - 70

	// --- BAR CHART: Processing Speed ---
	y := height - 150.0
	_ = page.AddTextCustomFontColor("Processing Speed (pages/second)", 50, y, fonts.Bold, 12, NavyDark)

	y -= 12
	speedData := []struct {
		label string
		value float64
		color creator.Color
	}{
		{"Text Extraction", 1000, ChartBlue},
		{featureTableExtraction, 500, ChartGreen},
		{"PDF Generation", 100, ChartOrange},
		{"Font Embedding", 50, ChartPurple},
	}

	barHeight := 18.0
	maxBarWidth := leftColWidth - 100
	maxValue := 1000.0
	labelWidth := 95.0

	for _, item := range speedData {
		y -= barHeight + 6
		_ = page.AddTextCustomFontColor(item.label, 50, y+4, fonts.Regular, 8, TextDark)
		barWidth := (item.value / maxValue) * maxBarWidth
		_ = page.DrawRect(50+labelWidth, y, barWidth, barHeight, &creator.RectOptions{
			FillColor: &item.color,
		})
		valueText := fmt.Sprintf("%.0f", item.value)
		_ = page.AddTextCustomFontColor(valueText, 55+labelWidth+barWidth, y+4, fonts.Bold, 8, item.color)
	}

	// --- MEMORY BAR CHART ---
	y -= 30
	_ = page.AddTextCustomFontColor("Memory Footprint (MB per 100 pages)", 50, y, fonts.Bold, 12, NavyDark)

	y -= 12
	memData := []struct {
		label string
		value float64
		color creator.Color
	}{
		{"Text Extraction", 5, ChartBlue},
		{featureTableExtraction, 10, ChartGreen},
		{"PDF Generation", 20, ChartOrange},
		{"Font Embedding", 50, ChartPurple},
	}

	maxMem := 50.0
	for _, item := range memData {
		y -= barHeight + 6
		_ = page.AddTextCustomFontColor(item.label, 50, y+4, fonts.Regular, 8, TextDark)
		barWidth := (item.value / maxMem) * maxBarWidth
		_ = page.DrawRect(50+labelWidth, y, barWidth, barHeight, &creator.RectOptions{
			FillColor: &item.color,
		})
		valueText := fmt.Sprintf("%.0f MB", item.value)
		_ = page.AddTextCustomFontColor(valueText, 55+labelWidth+barWidth, y+4, fonts.Bold, 8, item.color)
	}

	// --- PIE CHART with legend below ---
	y -= 35
	_ = page.AddTextCustomFontColor("Module Distribution", 50, y, fonts.Bold, 12, NavyDark)

	pieX := 130.0
	pieY := y - 75
	pieR := 55.0

	pieData := []struct {
		label   string
		percent float64
		color   creator.Color
	}{
		{"Document", 30, ChartBlue},
		{"Content", 25, ChartGreen},
		{"Security", 20, ChartOrange},
		{"Resources", 15, ChartPurple},
		{"Utility", 10, ChartCyan},
	}

	drawPieChart(page, pieX, pieY, pieR, pieData)

	// Legend below pie chart (horizontal layout).
	legendY := pieY - pieR - 20
	legendX := 50.0
	for i, item := range pieData {
		if i == 3 { // Second row.
			legendY -= 16
			legendX = 50.0
		}
		_ = page.DrawRect(legendX, legendY-3, 10, 10, &creator.RectOptions{
			FillColor: &item.color,
		})
		legendText := fmt.Sprintf("%s %.0f%%", item.label, item.percent)
		_ = page.AddTextCustomFontColor(legendText, legendX+14, legendY, fonts.Regular, 8, TextDark)
		legendX += 85
	}

	// === RIGHT COLUMN (x: width/2 + 10) ===
	cardX := width/2 + 10
	cardY := height - 150.0
	cardW := width/2 - 60
	cardH := 65.0

	_ = page.AddTextCustomFontColor("Key Metrics", cardX, cardY, fonts.Bold, 12, NavyDark)
	cardY -= 20

	metricsCards := []struct {
		title string
		value string
		desc  string
		color creator.Color
	}{
		{"Test Coverage", "85%", "Unit & integration tests", ChartGreen},
		{"Memory Usage", "<10 MB", "Per 100 pages processed", ChartBlue},
		{"Accuracy Rate", "100%", "Table extraction (740 tx)", AccentGold},
		{"Code Quality", "A+", "Zero linter warnings", ChartPurple},
	}

	for _, card := range metricsCards {
		drawMetricCard(page, fonts, cardX, cardY, cardW, cardH, card.title, card.value, card.desc, card.color)
		cardY -= cardH + 12
	}

	// --- Additional stats table on right side ---
	cardY -= 20
	_ = page.AddTextCustomFontColor("Benchmark Results", cardX, cardY, fonts.Bold, 12, NavyDark)

	cardY -= 15
	benchData := [][]string{
		{"Operation", "Ops/sec"},
		{"Parse PDF", "~2,000"},
		{"Extract text", "~1,000"},
		{"Extract tables", "~500"},
		{"Generate PDF", "~100"},
	}
	_, _ = drawTable(page, fonts, cardX, cardY, cardW, benchData)

	drawFooter(page, fonts, 3, 7)
	return nil
}

// drawPieChart draws a simple pie chart using filled rectangles as segment indicators.
func drawPieChart(page *creator.Page, cx, cy, r float64, data []struct {
	label   string
	percent float64
	color   creator.Color
}) {
	// Draw circle outline.
	segments := 36
	angleStep := 2 * math.Pi / float64(segments)

	// Fill pie segments using small wedges.
	startAngle := 0.0
	for _, item := range data {
		endAngle := startAngle + (item.percent/100)*2*math.Pi

		// Draw filled segment as a series of triangles (approximation).
		for angle := startAngle; angle < endAngle; angle += angleStep / 2 {
			nextAngle := angle + angleStep/2
			if nextAngle > endAngle {
				nextAngle = endAngle
			}

			// Triangle from center to two edge points.
			x1 := cx + r*math.Cos(angle)
			y1 := cy + r*math.Sin(angle)
			x2 := cx + r*math.Cos(nextAngle)
			y2 := cy + r*math.Sin(nextAngle)

			// Draw as a small rectangle approximation.
			_ = page.DrawLine(cx, cy, x1, y1, &creator.LineOptions{Color: item.color, Width: 2})
			_ = page.DrawLine(x1, y1, x2, y2, &creator.LineOptions{Color: item.color, Width: 2})
		}

		startAngle = endAngle
	}

	// Draw circle outline.
	for i := 0; i < segments; i++ {
		angle1 := float64(i) * angleStep
		angle2 := float64(i+1) * angleStep
		x1 := cx + r*math.Cos(angle1)
		y1 := cy + r*math.Sin(angle1)
		x2 := cx + r*math.Cos(angle2)
		y2 := cy + r*math.Sin(angle2)
		_ = page.DrawLine(x1, y1, x2, y2, &creator.LineOptions{Color: NavyDark, Width: 1})
	}
}

// drawMetricCard draws a metric card with value and description.
func drawMetricCard(page *creator.Page, fonts *Fonts, x, y, w, h float64, title, value, desc string, accentColor creator.Color) {
	// Card background.
	_ = page.DrawRect(x, y-h, w, h, &creator.RectOptions{
		FillColor:   &White,
		StrokeColor: &TableBorder,
		StrokeWidth: 1,
	})

	// Left accent bar.
	_ = page.DrawRect(x, y-h, 4, h, &creator.RectOptions{
		FillColor: &accentColor,
	})

	// Title.
	_ = page.AddTextCustomFontColor(title, x+15, y-15, fonts.Regular, 9, TextGray)

	// Value.
	_ = page.AddTextCustomFontColor(value, x+15, y-38, fonts.Bold, 22, accentColor)

	// Description.
	_ = page.AddTextCustomFontColor(desc, x+15, y-55, fonts.Regular, 8, TextMuted)
}

// =============================================================================
// PAGE 4: UNICODE SUPPORT
// =============================================================================

func createUnicodePage(c *creator.Creator, fonts *Fonts) error {
	page, err := c.NewPage()
	if err != nil {
		return err
	}

	width := page.Width()
	height := page.Height()

	drawPageHeader(page, fonts, "Unicode Support", "Full international text rendering capabilities")

	// --- EUROPEAN LANGUAGES TABLE ---
	y := height - 120.0
	_ = page.AddTextCustomFontColor("European Languages", 50, y, fonts.Bold, 14, NavyDark)

	y -= 20
	euroLangs := [][]string{
		{"Language", "Sample Text"},
		{"English", "Hello, World! Professional PDF generation."},
		{"Russian", "Привет, мир! Профессиональная генерация PDF."},
		{"Ukrainian", "Привіт, світ! Професійна генерація PDF."},
		{"Greek", "Γειά σου κόσμε! Επαγγελματική δημιουργία."},
		{"German", "Hallo, Welt! Professionelle PDF-Erstellung."},
		{"French", "Bonjour, le monde! Generation PDF professionnelle."},
		{"Spanish", "Hola, mundo! Generacion de PDF profesional."},
		{"Polish", "Czesc, swiecie! Profesjonalne generowanie PDF."},
		{"Czech", "Ahoj, svete! Profesionalni generovani PDF."},
	}

	y, _ = drawTable(page, fonts, 50, y, width-100, euroLangs)

	// --- CJK SECTION ---
	y -= 35
	if fonts.CJK != nil {
		_ = page.AddTextCustomFontColor("CJK Languages (East Asian)", 50, y, fonts.Bold, 14, NavyDark)
		y -= 20

		cjkExamples := []struct {
			lang string
			text string
		}{
			{"Korean", "안녕하세요! 전문적인 PDF 생성."},
			{"Chinese (Simplified)", "你好，世界！专业的PDF生成。"},
			{"Chinese (Traditional)", "你好，世界！專業的PDF生成。"},
			{"Japanese", "こんにちは！プロフェッショナルなPDF。"},
		}

		rowHeight := 26.0
		tableWidth := width - 100
		col1Width := 140.0

		// Header.
		_ = page.DrawRect(50, y-rowHeight, tableWidth, rowHeight, &creator.RectOptions{
			FillColor:   &TableHeader,
			StrokeColor: &TableBorder,
			StrokeWidth: 0.5,
		})
		_ = page.AddTextCustomFontColor("Language", 58, y-17, fonts.Bold, 10, TextDark)
		_ = page.AddTextCustomFontColor("Sample Text", 58+col1Width, y-17, fonts.Bold, 10, TextDark)
		y -= rowHeight

		for i, ex := range cjkExamples {
			fillColor := &White
			if i%2 == 0 {
				fillColor = &TableStripe
			}
			_ = page.DrawRect(50, y-rowHeight, tableWidth, rowHeight, &creator.RectOptions{
				FillColor:   fillColor,
				StrokeColor: &TableBorder,
				StrokeWidth: 0.5,
			})
			_ = page.AddTextCustomFontColor(ex.lang, 58, y-17, fonts.Regular, 10, TextDark)
			_ = page.AddTextCustomFontColor(ex.text, 58+col1Width, y-17, fonts.CJK, 10, TextDark)
			y -= rowHeight
		}
	}

	// --- SYMBOLS SECTION ---
	y -= 35
	_ = page.AddTextCustomFontColor("Special Symbols & Characters", 50, y, fonts.Bold, 14, NavyDark)

	symbolGroups := []struct {
		category string
		symbols  string
	}{
		{"Mathematical", "+ - = < > x / % ( ) [ ] { }"},
		{"Currency", "$ (USD)  EUR  GBP  JPY  RUB"},
		{"Punctuation", "! ? @ # & * : ; \" ' , . ..."},
		{"Brackets", "( ) [ ] { } < >"},
		{"Accents", "a e i o u A E I O U n N"},
	}

	y -= 20
	for _, group := range symbolGroups {
		_ = page.DrawRect(50, y-4, 8, 8, &creator.RectOptions{
			FillColor: &AccentBlue,
		})
		_ = page.AddTextCustomFontColor(group.category+":", 65, y, fonts.Bold, 10, TextGray)
		_ = page.AddTextCustomFontColor(group.symbols, 160, y, fonts.Regular, 10, TextDark)
		y -= 20
	}

	// --- ENCODING INFO (ensure it stays above footer at y=25) ---
	boxTop := 95.0 // Fixed position above footer
	boxHeight := 55.0
	boxCenter := boxTop - boxHeight/2
	_ = page.DrawRect(50, boxTop-boxHeight, width-100, boxHeight, &creator.RectOptions{
		FillColor: &LightGray,
	})
	_ = page.AddTextCustomFontColor("Encoding Details", 60, boxCenter+12, fonts.Bold, 11, NavyDark)
	_ = page.AddTextCustomFontColor("Identity-H CMap with CIDToGIDMap for TrueType fonts", 60, boxCenter-3, fonts.Regular, 9, TextDark)
	_ = page.AddTextCustomFontColor("Full BMP support: U+0000 to U+FFFF (65,536 code points)", 60, boxCenter-17, fonts.Regular, 9, TextDark)

	drawFooter(page, fonts, 4, 7)
	return nil
}

// =============================================================================
// PAGE 5: TECHNICAL SPECIFICATIONS
// =============================================================================

func createSpecsPage(c *creator.Creator, fonts *Fonts) error {
	page, err := c.NewPage()
	if err != nil {
		return err
	}

	width := page.Width()
	height := page.Height()

	drawPageHeader(page, fonts, "Technical Specifications", "Enterprise-grade implementation details")

	// --- TWO COLUMN LAYOUT ---
	colWidth := (width - 120) / 2

	// LEFT COLUMN: Core Specifications.
	y := height - 150.0
	_ = page.AddTextCustomFontColor("Core Specifications", 50, y, fonts.Bold, 13, NavyDark)

	y -= 20
	coreSpecs := [][]string{
		{"Property", "Value"},
		{"PDF Version", "1.7 (ISO 32000-1)"},
		{"Go Version", "1.21+"},
		{labelDependencies, depsZero},
		{"Font Support", "Standard 14 + TTF/OTF"},
		{"Unicode", unicodeFullBMP},
		{"License", "MIT"},
	}
	y, _ = drawTable(page, fonts, 50, y, colWidth, coreSpecs)

	// RIGHT COLUMN: Performance (start at same Y as left column).
	rightX := 60 + colWidth
	y2 := height - 150.0
	_ = page.AddTextCustomFontColor("Performance", rightX, y2, fonts.Bold, 13, NavyDark)

	y2 -= 20
	perfSpecs := [][]string{
		{"Operation", "Speed"},
		{"Text extraction", "~1000 pg/s"},
		{"Table extraction", "~500 pg/s"},
		{"PDF generation", "~100 pg/s"},
		{"Merge", "~200 pg/s"},
		{"Encrypt", "~150 pg/s"},
	}
	y2, _ = drawTable(page, fonts, rightX, y2, colWidth, perfSpecs)

	// Security and Compression at same level (use lower of y and y2).
	secY := y - 30
	if y2-30 < secY {
		secY = y2 - 30
	}

	// Security specs (left).
	_ = page.AddTextCustomFontColor("Security", 50, secY, fonts.Bold, 13, NavyDark)
	secSpecs := [][]string{
		{"Feature", "Support"},
		{"RC4 40-bit", supportYes},
		{"RC4 128-bit", supportYes},
		{"AES 128-bit", supportYes},
		{"AES 256-bit", supportYes},
		{"Permissions", supportFull},
	}
	_, _ = drawTable(page, fonts, 50, secY-20, colWidth, secSpecs)

	// Compression (right) - same Y level.
	_ = page.AddTextCustomFontColor("Compression", rightX, secY, fonts.Bold, 13, NavyDark)
	compSpecs := [][]string{
		{"Filter", "Status"},
		{"FlateDecode", supportFull},
		{"LZWDecode", "Read"},
		{"ASCII85", supportFull},
		{"ASCIIHex", supportFull},
		{"RunLength", "Read"},
	}
	_, _ = drawTable(page, fonts, rightX, secY-20, colWidth, compSpecs)

	// --- PLATFORM COMPATIBILITY (bottom) ---
	y = 230.0
	_ = page.AddTextCustomFontColor("Platform Compatibility", 50, y, fonts.Bold, 13, NavyDark)

	platforms := []struct {
		name   string
		status string
	}{
		{"Windows", supportFull},
		{"macOS", supportFull},
		{"Linux", supportFull},
		{"FreeBSD", supportFull},
		{"ARM64", supportFull},
		{"WebAssembly", "Experimental"},
	}

	y -= 25
	for i, p := range platforms {
		col := float64(i % 3)
		row := float64(i / 3)
		px := 50 + col*170
		py := y - row*25

		color := SuccessGreen
		if p.status == "Experimental" {
			color = WarningOrange
		}

		_ = page.DrawRect(px, py-4, 10, 10, &creator.RectOptions{
			FillColor: &color,
		})
		_ = page.AddTextCustomFontColor(p.name, px+18, py, fonts.Regular, 10, TextDark)
	}

	drawFooter(page, fonts, 5, 7)
	return nil
}

// =============================================================================
// PAGE 6: ARCHITECTURE OVERVIEW
// =============================================================================

func createArchitecturePage(c *creator.Creator, fonts *Fonts) error {
	page, err := c.NewPage()
	if err != nil {
		return err
	}

	width := page.Width()
	height := page.Height()

	drawPageHeader(page, fonts, "Architecture", "Clean, modular design following DDD principles")

	// --- ARCHITECTURE DIAGRAM ---
	y := height - 170.0
	centerX := width / 2

	// Layer 1: Public API.
	boxW := 400.0
	boxH := 40.0
	drawArchBox(page, fonts, centerX-boxW/2, y, boxW, boxH, "Public API  (pkg/pdf, pkg/creator, pkg/extractor)", AccentBlue, White)

	// Arrow down.
	y -= boxH + 10
	drawArrow(page, centerX, y+7, centerX, y-3, TextLight)

	// Layer 2: Application.
	y -= 10
	drawArchBox(page, fonts, centerX-boxW/2, y, boxW, boxH, "Application Layer  (internal/application/)", AccentTeal, White)

	// Arrow down.
	y -= boxH + 10
	drawArrow(page, centerX, y+7, centerX, y-3, TextLight)

	// Domain label (above boxes).
	y -= 12
	_ = page.AddTextCustomFontColor("Domain Model (internal/domain/)", centerX-80, y, fonts.Regular, 9, TextGray)

	// Layer 3: Domain (split boxes) - below label.
	y -= 15
	subBoxW := (boxW - 20) / 3
	drawArchBox(page, fonts, centerX-boxW/2, y, subBoxW, boxH, "Document", NavyDark, White)
	drawArchBox(page, fonts, centerX-subBoxW/2, y, subBoxW, boxH, "Content", NavyDark, White)
	drawArchBox(page, fonts, centerX+boxW/2-subBoxW, y, subBoxW, boxH, "Resources", NavyDark, White)

	// Arrow down.
	y -= boxH + 10
	drawArrow(page, centerX, y+7, centerX, y-3, TextLight)

	// Layer 4: Infrastructure.
	y -= 10
	drawArchBox(page, fonts, centerX-boxW/2, y, boxW, boxH, "Infrastructure  (internal/infrastructure/)", WarningOrange, White)

	// --- PRINCIPLES ---
	y -= 80
	_ = page.AddTextCustomFontColor("Design Principles", 50, y, fonts.Bold, 13, NavyDark)

	principles := []struct {
		title string
		desc  string
	}{
		{"Domain-Driven Design", "Rich domain model with behavior, not just data structures"},
		{"Clean Architecture", "Dependencies point inward; domain has no external dependencies"},
		{"SOLID Principles", "Single responsibility, open/closed, Liskov, interface segregation, DI"},
		{"Zero Dependencies", "Only Go standard library; no external packages required"},
	}

	y -= 20
	for _, p := range principles {
		_ = page.DrawRect(50, y-1, 8, 8, &creator.RectOptions{
			FillColor: &AccentGold,
		})
		_ = page.AddTextCustomFontColor(p.title, 68, y, fonts.Bold, 10, TextDark)
		_ = page.AddTextCustomFontColor(p.desc, 68, y-15, fonts.Regular, 9, TextMuted)
		y -= 35
	}

	// --- PACKAGE OVERVIEW ---
	y -= 20
	_ = page.AddTextCustomFontColor("Package Structure", 50, y, fonts.Bold, 13, NavyDark)

	packages := []struct {
		pkg  string
		desc string
	}{
		{"pkg/pdf", "Main entry point for PDF operations"},
		{"pkg/creator", "High-level document creation API"},
		{"pkg/extractor", "Text and table extraction"},
		{"internal/domain", "Core business logic and entities"},
		{"internal/application", "Use cases and orchestration"},
		{"internal/infrastructure", "PDF parsing, encoding, I/O"},
	}

	y -= 15
	for _, pkg := range packages {
		_ = page.AddTextCustomFontColor(pkg.pkg, 60, y, fonts.Bold, 9, AccentBlue)
		_ = page.AddTextCustomFontColor(pkg.desc, 200, y, fonts.Regular, 9, TextGray)
		y -= 16
	}

	drawFooter(page, fonts, 6, 7)
	return nil
}

// drawArchBox draws an architecture diagram box.
//
//nolint:unparam // parameters are for reusability
func drawArchBox(page *creator.Page, fonts *Fonts, x, y, w, h float64, text string, bgColor, textColor creator.Color) {
	_ = page.DrawRect(x, y-h, w, h, &creator.RectOptions{
		FillColor: &bgColor,
	})

	// Center text in box.
	textWidth := fonts.Bold.MeasureString(text, 10)
	textX := x + (w-textWidth)/2
	textY := y - h/2 - 4

	_ = page.AddTextCustomFontColor(text, textX, textY, fonts.Bold, 10, textColor)
}

// drawArrow draws a simple down arrow.
func drawArrow(page *creator.Page, x1, y1, x2, y2 float64, color creator.Color) {
	_ = page.DrawLine(x1, y1, x2, y2, &creator.LineOptions{Color: color, Width: 1.5})
	_ = page.DrawLine(x2-4, y2+6, x2, y2, &creator.LineOptions{Color: color, Width: 1.5})
	_ = page.DrawLine(x2+4, y2+6, x2, y2, &creator.LineOptions{Color: color, Width: 1.5})
}

// =============================================================================
// PAGE 7: API EXAMPLES
// =============================================================================

func createAPIPage(c *creator.Creator, fonts *Fonts) error {
	page, err := c.NewPage()
	if err != nil {
		return err
	}

	width := page.Width()
	height := page.Height()

	drawPageHeader(page, fonts, "API Examples", "Simple, intuitive interface for common operations")

	y := height - 150.0

	// Example 1: Create PDF.
	y = drawCodeExample(page, fonts, 50, y, width-100,
		"Create a PDF Document",
		[]string{
			"c := creator.New()",
			"c.SetTitle(\"My Document\")",
			"page, _ := c.NewPage()",
			"page.AddText(\"Hello, World!\", 50, 700)",
			"c.WriteToFile(\"output.pdf\")",
		})

	// Example 2: Extract Text.
	y -= 25
	y = drawCodeExample(page, fonts, 50, y, width-100,
		"Extract Text from PDF",
		[]string{
			"doc, _ := pdf.Open(\"input.pdf\")",
			"for i := 0; i < doc.PageCount(); i++ {",
			"    text, _ := doc.ExtractText(i)",
			"    fmt.Println(text)",
			"}",
		})

	// Example 3: Extract Tables.
	y -= 25
	y = drawCodeExample(page, fonts, 50, y, width-100,
		"Extract Tables from PDF",
		[]string{
			"doc, _ := pdf.Open(\"statement.pdf\")",
			"tables, _ := extractor.ExtractTables(doc, 0)",
			"for _, table := range tables {",
			"    for _, row := range table.Rows {",
			"        fmt.Println(row.Cells)",
			"    }",
			"}",
		})

	// Example 4: Add Security.
	y -= 25
	_ = drawCodeExample(page, fonts, 50, y, width-100,
		"Encrypt PDF with Password",
		[]string{
			"c := creator.New()",
			"c.SetEncryption(creator.EncryptionOptions{",
			"    UserPassword:  \"user123\",",
			"    OwnerPassword: \"owner456\",",
			"    Algorithm:     creator.AES256,",
			"})",
			"c.WriteToFile(\"secure.pdf\")",
		})

	// --- RESOURCES BOX ---
	_ = page.DrawRect(50, 70, width-100, 55, &creator.RectOptions{
		FillColor: &NavyDark,
	})
	_ = page.AddTextCustomFontColor("Get Started Today", 60, 108, fonts.Bold, 14, White)
	_ = page.AddTextCustomFontColor("GitHub: github.com/coregx/gxpdf", 60, 88, fonts.Regular, 10, AccentGold)
	_ = page.AddTextCustomFontColor("Docs: pkg.go.dev/github.com/coregx/gxpdf", 300, 88, fonts.Regular, 10, AccentGold)

	drawFooter(page, fonts, 7, 7)
	return nil
}

// drawCodeExample draws a code example block.
//
//nolint:unparam // x, w are parameterized for reusability
func drawCodeExample(page *creator.Page, fonts *Fonts, x, y, w float64, title string, code []string) float64 {
	// Title.
	_ = page.AddTextCustomFontColor(title, x, y, fonts.Bold, 12, NavyDark)

	// Code block background.
	lineHeight := 14.0
	codeHeight := float64(len(code))*lineHeight + 16
	y -= 18

	_ = page.DrawRect(x, y-codeHeight, w, codeHeight, &creator.RectOptions{
		FillColor:   &NavyDark,
		StrokeColor: &AccentBlue,
		StrokeWidth: 1,
	})

	// Code lines.
	codeY := y - 12
	for _, line := range code {
		_ = page.AddTextCustomFontColor(line, x+10, codeY, fonts.Regular, 9, AccentGold)
		codeY -= lineHeight
	}

	return y - codeHeight
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// drawPageHeader draws a consistent page header.
func drawPageHeader(page *creator.Page, fonts *Fonts, title, subtitle string) {
	width := page.Width()
	height := page.Height()

	// Header bar.
	_ = page.DrawRect(0, height-70, width, 70, &creator.RectOptions{
		FillColor: &NavyDark,
	})

	// Title.
	_ = page.AddTextCustomFontColor(title, 50, height-35, fonts.Bold, 26, White)

	// Subtitle.
	_ = page.AddTextCustomFontColor(subtitle, 50, height-55, fonts.Regular, 11, AccentGold)

	// Gold accent line.
	_ = page.DrawRect(0, height-72, width, 2, &creator.RectOptions{
		FillColor: &AccentGold,
	})
}

// drawFooter draws the page footer.
//
//nolint:unparam // totalPages is parameterized for reusability
func drawFooter(page *creator.Page, fonts *Fonts, pageNum, totalPages int) {
	width := page.Width()

	// Footer line (very bottom of page).
	_ = page.DrawLine(50, 25, width-50, 25, &creator.LineOptions{
		Color: TableBorder,
		Width: 0.5,
	})

	// Page number.
	pageText := fmt.Sprintf("Page %d of %d", pageNum, totalPages)
	_ = drawCenteredText(page, pageText, width, 10, fonts.Regular, 9, TextGray)

	// Company.
	_ = page.AddTextCustomFontColor("CoreGX Technologies", 50, 10, fonts.Regular, 9, TextGray)

	// Year.
	_ = page.AddTextCustomFontColor("2026", width-80, 10, fonts.Regular, 9, TextGray)
}

// drawCenteredText draws text centered horizontally.
func drawCenteredText(page *creator.Page, text string, pageWidth, y float64, font *creator.CustomFont, size float64, color creator.Color) error {
	textWidth := font.MeasureString(text, size)
	x := (pageWidth - textWidth) / 2
	return page.AddTextCustomFontColor(text, x, y, font, size, color)
}

// drawTable draws a professional table with zebra stripes and column dividers.
//
//nolint:unparam // error return is for future extensibility
func drawTable(page *creator.Page, fonts *Fonts, x, y, width float64, data [][]string) (float64, error) {
	if len(data) == 0 {
		return y, nil
	}

	rowHeight := 24.0
	numCols := len(data[0])

	// First column is narrower (30% for 2 cols, 25% for more).
	colWidths := make([]float64, numCols)
	if numCols == 2 {
		colWidths[0] = width * 0.30
		colWidths[1] = width * 0.70
	} else {
		for i := range colWidths {
			colWidths[i] = width / float64(numCols)
		}
	}

	for rowIdx, row := range data {
		isHeader := rowIdx == 0
		isStripe := rowIdx%2 == 0

		fillColor := &White
		if isHeader {
			fillColor = &TableHeader
		} else if isStripe {
			fillColor = &TableStripe
		}

		// Row background.
		_ = page.DrawRect(x, y-rowHeight, width, rowHeight, &creator.RectOptions{
			FillColor:   fillColor,
			StrokeColor: &TableBorder,
			StrokeWidth: 0.5,
		})

		// Cell contents and vertical dividers.
		cellX := x
		for colIdx, cell := range row {
			colW := colWidths[colIdx]

			font := fonts.Regular
			color := TextDark
			if isHeader {
				font = fonts.Bold
			}

			textX := cellX + 6
			textY := y - rowHeight + 7
			clipX := cellX + 2
			clipY := y - rowHeight + 2
			clipW := colW - 4
			clipH := rowHeight - 4

			_ = page.DrawTextClipped(cell, textX, textY, clipX, clipY, clipW, clipH, font, 9, color)

			// Vertical divider (except after last column).
			if colIdx < numCols-1 {
				_ = page.DrawLine(cellX+colW, y, cellX+colW, y-rowHeight, &creator.LineOptions{
					Color: TableBorder,
					Width: 0.5,
				})
			}

			cellX += colW
		}

		y -= rowHeight
	}

	return y, nil
}

// Unused but kept for potential future use on other platforms.
var _ = runtime.GOOS
