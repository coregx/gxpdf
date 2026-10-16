package internal

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/coregx/gxpdf/creator"
	"github.com/coregx/gxpdf/layout"
)

// FontSet decides which font a family name means. Text is measured during
// layout (FontBridge) and drawn by the renderer; both resolve through one
// FontSet, so text is always drawn in the font it was measured with.
//
// A family resolves to, in order:
//  1. a font registered under exactly that name (WithFont, WithFontFile);
//  2. a Standard 14 family or one of its aliases, matched case-insensitively
//     (see standardFamilies);
//  3. otherwise it is unknown: it is recorded, and the fallback family is
//     used if one is set, else Helvetica so that layout can complete.
//
// Err reports the unknown families when no fallback is set. A FontSet is not
// safe for concurrent use, like the Builder that owns it.
type FontSet struct {
	custom   map[string]*creator.CustomFont
	fallback string
	unknown  map[string]struct{}
}

// Font is a resolved font: a custom font, or a Standard 14 font when Custom
// is nil.
type Font struct {
	Custom   *creator.CustomFont
	Standard creator.FontName
}

// MeasureString returns the width of text in points at size.
func (f Font) MeasureString(text string, size float64) float64 {
	if f.Custom != nil {
		return f.Custom.MeasureString(text, size)
	}
	return creator.MeasureText(f.Standard, text, size)
}

// Ascender returns the ascender height in points at size.
func (f Font) Ascender(size float64) float64 {
	if f.Custom != nil {
		return f.Custom.Ascender(size)
	}
	return creator.FontAscender(f.Standard, size)
}

// NewFontSet returns a FontSet over the registered custom fonts. fallback,
// if not empty, is the family used for unknown families.
func NewFontSet(custom map[string]*creator.CustomFont, fallback string) *FontSet {
	if custom == nil {
		custom = make(map[string]*creator.CustomFont)
	}
	return &FontSet{custom: custom, fallback: fallback, unknown: make(map[string]struct{})}
}

// Resolve returns the font for ref.
func (fs *FontSet) Resolve(ref layout.FontRef) Font {
	if f, ok := fs.lookup(ref); ok {
		return f
	}
	fs.unknown[ref.Family] = struct{}{}
	if fs.fallback != "" {
		if f, ok := fs.lookup(layout.FontRef{Family: fs.fallback, Weight: ref.Weight, Style: ref.Style}); ok {
			return f
		}
	}
	return Font{Standard: standardFont(familyHelvetica, ref)}
}

// lookup resolves registered and Standard 14 families.
func (fs *FontSet) lookup(ref layout.FontRef) (Font, bool) {
	if cf, ok := fs.custom[ref.Family]; ok {
		return Font{Custom: cf}, true
	}
	if fam, ok := standardFamily(ref.Family); ok {
		return Font{Standard: standardFont(fam, ref)}, true
	}
	return Font{}, false
}

// CheckFallback reports a fallback family that is itself unknown.
func (fs *FontSet) CheckFallback() error {
	if fs.fallback == "" {
		return nil
	}
	if _, ok := fs.lookup(layout.FontRef{Family: fs.fallback}); !ok {
		return fmt.Errorf("font fallback %q is not registered and is not a Standard 14 family", fs.fallback)
	}
	return nil
}

// Reset forgets the unknown families recorded so far.
func (fs *FontSet) Reset() {
	clear(fs.unknown)
}

// Err returns an error naming every unknown family resolved since the last
// Reset, or nil. With a fallback set, unknown families are not errors.
func (fs *FontSet) Err() error {
	if fs.fallback != "" || len(fs.unknown) == 0 {
		return nil
	}
	names := make([]string, 0, len(fs.unknown))
	for name := range fs.unknown {
		names = append(names, name)
	}
	sort.Strings(names)
	errs := make([]error, len(names))
	for i, name := range names {
		if similar := fs.registeredIgnoringCase(name); similar != "" {
			errs[i] = fmt.Errorf("font family %q is not registered (did you mean %q? registered names are case-sensitive)", name, similar)
			continue
		}
		errs[i] = fmt.Errorf("font family %q is not registered: register it with WithFont or WithFontFile, "+
			"use a Standard 14 family, or set WithFontFallback", name)
	}
	return errors.Join(errs...)
}

// registeredIgnoringCase returns a registered family that equals name up to
// letter case, or "".
func (fs *FontSet) registeredIgnoringCase(name string) string {
	for registered := range fs.custom {
		if strings.EqualFold(registered, name) {
			return registered
		}
	}
	return ""
}

// standard14Family identifies a Standard 14 font family.
type standard14Family int

const (
	familyHelvetica standard14Family = iota
	familyTimes
	familyCourier
	familySymbol
	familyZapfDingbats
)

// standardFamilies maps lower-cased family names to Standard 14 families.
// "Arial", "Times New Roman" and "Courier New" are the metric-compatible
// families that PDF viewers substitute for the same Standard 14 fonts.
var standardFamilies = map[string]standard14Family{
	"":                familyHelvetica, // the default style
	"helvetica":       familyHelvetica,
	"arial":           familyHelvetica,
	"times":           familyTimes,
	"times-roman":     familyTimes,
	"times roman":     familyTimes,
	"times new roman": familyTimes,
	"courier":         familyCourier,
	"courier new":     familyCourier,
	"symbol":          familySymbol,
	"zapfdingbats":    familyZapfDingbats,
	"zapf dingbats":   familyZapfDingbats,
}

// standardFamily returns the Standard 14 family a name refers to.
func standardFamily(name string) (standard14Family, bool) {
	fam, ok := standardFamilies[strings.ToLower(strings.TrimSpace(name))]
	return fam, ok
}

// standardFont returns the Standard 14 font of fam for ref's weight and style.
func standardFont(fam standard14Family, ref layout.FontRef) creator.FontName {
	bold := ref.Weight == layout.WeightBold
	italic := ref.Style == layout.StyleItalic
	switch fam {
	case familyTimes:
		return pick(bold, italic, creator.TimesRoman, creator.TimesBold, creator.TimesItalic, creator.TimesBoldItalic)
	case familyCourier:
		return pick(bold, italic, creator.Courier, creator.CourierBold, creator.CourierOblique, creator.CourierBoldOblique)
	case familySymbol:
		return creator.Symbol
	case familyZapfDingbats:
		return creator.ZapfDingbats
	default:
		return pick(bold, italic, creator.Helvetica, creator.HelveticaBold, creator.HelveticaOblique, creator.HelveticaBoldOblique)
	}
}

func pick(bold, italic bool, regular, b, i, bi creator.FontName) creator.FontName {
	switch {
	case bold && italic:
		return bi
	case bold:
		return b
	case italic:
		return i
	default:
		return regular
	}
}
