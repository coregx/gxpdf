package builder

import "github.com/coregx/gxpdf/layout"

// TextOption configures text styling for elements like [Container.Text] and [Container.PageNumber].
// Multiple options are composed left-to-right; later options override earlier ones.
//
// Use constructor functions like [Bold], [FontSize], [TextColor], [AlignCenter] to create options.
type TextOption struct {
	apply func(*layout.Style)
}

// Bold applies bold weight to the font.
func Bold() TextOption {
	return TextOption{apply: func(s *layout.Style) {
		s.Bold = true
		s.Font.Weight = layout.WeightBold
	}}
}

// Italic applies italic style to the font.
func Italic() TextOption {
	return TextOption{apply: func(s *layout.Style) {
		s.Italic = true
		s.Font.Style = layout.StyleItalic
	}}
}

// FontSize sets the font size in PDF points.
func FontSize(size float64) TextOption {
	return TextOption{apply: func(s *layout.Style) {
		s.FontSize = size
	}}
}

// FontFamily sets the font family by name (e.g. "Helvetica", "Inter").
//
// The family must be registered via WithFont or WithFontFile (exact name), or
// be a Standard 14 family, matched in any letter case: Helvetica (alias
// Arial), Times (Times-Roman, Times Roman, Times New Roman), Courier (Courier
// New), Symbol, ZapfDingbats. Any other family makes Build return an error
// naming it, unless WithFontFallback is set.
func FontFamily(family string) TextOption {
	return TextOption{apply: func(s *layout.Style) {
		s.Font.Family = family
	}}
}

// TextColor sets the foreground text color.
func TextColor(c Color) TextOption {
	lc := c.toLayout()
	return TextOption{apply: func(s *layout.Style) {
		s.Color = lc
	}}
}

// BgColor sets the background fill color of the text element's bounding box.
func BgColor(c Color) TextOption {
	lc := c.toLayout()
	return TextOption{apply: func(s *layout.Style) {
		s.Background = &lc
	}}
}

// AlignLeft sets left text alignment (default).
func AlignLeft() TextOption {
	return TextOption{apply: func(s *layout.Style) {
		s.TextAlign = layout.AlignLeft
	}}
}

// AlignCenter sets centered text alignment.
func AlignCenter() TextOption {
	return TextOption{apply: func(s *layout.Style) {
		s.TextAlign = layout.AlignCenter
	}}
}

// AlignRight sets right text alignment.
func AlignRight() TextOption {
	return TextOption{apply: func(s *layout.Style) {
		s.TextAlign = layout.AlignRight
	}}
}

// AlignJustify sets justified text alignment.
// The last line of a paragraph is left-aligned per typographic convention.
func AlignJustify() TextOption {
	return TextOption{apply: func(s *layout.Style) {
		s.TextAlign = layout.AlignJustify
	}}
}

// LineHeight sets the line height multiplier (relative to font size).
// A value of 1.2 means 20% leading — the default. 1.5 gives more open spacing.
func LineHeight(multiplier float64) TextOption {
	return TextOption{apply: func(s *layout.Style) {
		s.LineHeight = multiplier
	}}
}

// Underline adds an underline decoration to the text.
func Underline() TextOption {
	return TextOption{apply: func(s *layout.Style) {
		s.Underline = true
	}}
}

// Strikethrough adds a strikethrough decoration to the text.
func Strikethrough() TextOption {
	return TextOption{apply: func(s *layout.Style) {
		s.Strikethrough = true
	}}
}

// LetterSpacing adds extra spacing between characters in PDF points.
func LetterSpacing(pts float64) TextOption {
	return TextOption{apply: func(s *layout.Style) {
		s.LetterSpacing = pts
	}}
}

// applyTextOptions applies a slice of TextOption values to a base Style and
// returns the resulting Style. The base style is not mutated.
func applyTextOptions(base *layout.Style, opts []TextOption) layout.Style {
	s := *base
	for _, opt := range opts {
		opt.apply(&s)
	}
	return s
}

// RowOption is a functional option that modifies row configuration.
type RowOption func(*rowConfig)

// rowConfig holds per-row configuration derived from RowOption values.
type rowConfig struct {
	height  *layout.Value
	bgColor *layout.Color
	padding *layout.Value // vertical padding (top and bottom)
}

// RowHeight sets an explicit height for a row.
func RowHeight(h Value) RowOption {
	lh := h.toLayout()
	return func(c *rowConfig) {
		c.height = &lh
	}
}

// RowBg sets the background color for the entire row.
func RowBg(c Color) RowOption {
	lc := c.toLayout()
	return func(cfg *rowConfig) {
		cfg.bgColor = &lc
	}
}

// RowPadding sets uniform padding (all 4 sides) for a row.
// This adds space around the row content within the background area.
func RowPadding(v Value) RowOption {
	lv := v.toLayout()
	return func(cfg *rowConfig) {
		cfg.padding = &lv
	}
}

// applyRowOptions applies a slice of RowOption values and returns the resulting rowConfig.
func applyRowOptions(opts []RowOption) rowConfig {
	cfg := rowConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// ImageOption is a functional option for image elements.
type ImageOption func(*imageConfig)

// imageConfig holds per-image layout configuration.
type imageConfig struct {
	width  *layout.Value
	height *layout.Value
}

// FitWidth constrains the image width to the given value, preserving aspect ratio.
func FitWidth(w Value) ImageOption {
	lw := w.toLayout()
	return func(c *imageConfig) {
		c.width = &lw
	}
}

// FitHeight constrains the image height to the given value, preserving aspect ratio.
func FitHeight(h Value) ImageOption {
	lh := h.toLayout()
	return func(c *imageConfig) {
		c.height = &lh
	}
}

// applyImageOptions applies a slice of ImageOption values and returns the resulting imageConfig.
func applyImageOptions(opts []ImageOption) imageConfig {
	cfg := imageConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// LineOption is a functional option for horizontal rule elements.
type LineOption func(*lineConfig)

// lineConfig holds per-line configuration.
type lineConfig struct {
	color *layout.Color
	width float64
}

// LineColor sets the color of a horizontal rule.
func LineColor(c Color) LineOption {
	lc := c.toLayout()
	return func(cfg *lineConfig) {
		cfg.color = &lc
	}
}

// LineWidth sets the stroke width of a horizontal rule in PDF points.
func LineWidth(w float64) LineOption {
	return func(cfg *lineConfig) {
		cfg.width = w
	}
}

// applyLineOptions applies a slice of LineOption values and returns the resulting lineConfig.
func applyLineOptions(opts []LineOption) lineConfig {
	cfg := lineConfig{width: 1.0}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}
