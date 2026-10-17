package fonts

// FontMetrics contains metric information for a font.
// All measurements are in font units (1000 units = 1 em).
type FontMetrics struct {
	// Ascender is the maximum height above the baseline.
	Ascender int

	// Descender is the maximum depth below the baseline (negative value).
	Descender int

	// CapHeight is the height of capital letters.
	CapHeight int

	// XHeight is the height of lowercase letters (typically 'x').
	XHeight int

	// DefaultWidth is used for characters without explicit width data.
	DefaultWidth int

	// CharWidths maps runes to their widths in font units.
	// For WinAnsiEncoding fonts, it covers the characters of the encoding;
	// for symbolic fonts, it is indexed by character code.
	CharWidths map[rune]int

	// symbolic marks Symbol and ZapfDingbats, whose text is given as the
	// character codes of their built-in encodings (see code).
	symbolic bool
}

// GetCharWidth returns the width of a character in font units (1000 units = 1 em).
// If the character is not found, returns the default width.
func (m *FontMetrics) GetCharWidth(ch rune) int {
	if w, ok := m.CharWidths[ch]; ok {
		return w
	}
	return m.DefaultWidth
}

// MeasureString returns the width of a string in points at the given font size.
// Formula: width_points = (sum of char widths) * size / 1000.
//
// A character the font's encoding cannot represent is measured as the '?'
// drawn in its place (see Encode).
func (m *FontMetrics) MeasureString(text string, size float64) float64 {
	var totalWidth int
	for _, ch := range text {
		_, drawn := m.code(ch)
		totalWidth += m.GetCharWidth(drawn)
	}
	return float64(totalWidth) * size / 1000.0
}

// GetAscender returns the ascender value in font units.
func (m *FontMetrics) GetAscender() int {
	return m.Ascender
}

// GetDescender returns the descender value in font units (negative).
func (m *FontMetrics) GetDescender() int {
	return m.Descender
}

// GetCapHeight returns the cap height in font units.
func (m *FontMetrics) GetCapHeight() int {
	return m.CapHeight
}

// GetXHeight returns the x-height in font units.
func (m *FontMetrics) GetXHeight() int {
	return m.XHeight
}

// GetMetrics returns the FontMetrics for a Standard14Font.
// Returns nil if the font is not recognized.
func (f *Standard14Font) GetMetrics() *FontMetrics {
	return GetMetrics(f.Name)
}

// fontMetricsRegistry maps font names to their metrics.
// Initialized lazily on first access to avoid init() complexity.
//
//nolint:gochecknoglobals // Font registry is intentionally global
var fontMetricsRegistry map[string]*FontMetrics

// initFontMetricsRegistry initializes the font metrics registry.
func initFontMetricsRegistry() {
	if fontMetricsRegistry != nil {
		return
	}
	fontMetricsRegistry = map[string]*FontMetrics{
		Helvetica.Name:            helveticaMetrics,
		HelveticaBold.Name:        helveticaBoldMetrics,
		HelveticaOblique.Name:     helveticaObliqueMetrics,
		HelveticaBoldOblique.Name: helveticaBoldObliqueMetrics,
		TimesRoman.Name:           timesRomanMetrics,
		TimesBold.Name:            timesBoldMetrics,
		TimesItalic.Name:          timesItalicMetrics,
		TimesBoldItalic.Name:      timesBoldItalicMetrics,
		Courier.Name:              courierMetrics,
		CourierBold.Name:          courierBoldMetrics,
		CourierOblique.Name:       courierObliqueMetrics,
		CourierBoldOblique.Name:   courierBoldObliqueMetrics,
		Symbol.Name:               symbolMetrics,
		ZapfDingbats.Name:         zapfDingbatsMetrics,
	}
}

// GetMetrics returns the FontMetrics for a font by its PostScript name.
// Returns nil if the font is not recognized.
func GetMetrics(fontName string) *FontMetrics {
	initFontMetricsRegistry()
	return fontMetricsRegistry[fontName]
}

// MeasureString is a convenience function to measure a string width in points.
// Uses the specified font's metrics. Returns 0 if the font is not recognized.
func MeasureString(fontName string, text string, size float64) float64 {
	m := GetMetrics(fontName)
	if m == nil {
		return 0
	}
	return m.MeasureString(text, size)
}
