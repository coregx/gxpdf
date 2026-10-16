package internal

import (
	"strings"
	"testing"

	"github.com/coregx/gxpdf/creator"
	"github.com/coregx/gxpdf/layout"
)

// stdName resolves ref with no custom fonts and returns its Standard 14 font.
func stdName(ref layout.FontRef) creator.FontName {
	return NewFontSet(nil, "").Resolve(ref).Standard
}

func TestFontSet_StandardFamiliesAndAliases(t *testing.T) {
	bold := layout.WeightBold
	italic := layout.StyleItalic
	tests := []struct {
		family string
		weight layout.FontWeight
		style  layout.FontStyle
		want   creator.FontName
	}{
		{"", 0, 0, creator.Helvetica},
		{"Helvetica", 0, 0, creator.Helvetica},
		{"helvetica", bold, 0, creator.HelveticaBold},
		{"Arial", 0, italic, creator.HelveticaOblique},
		{"ARIAL", bold, italic, creator.HelveticaBoldOblique},
		{"Times", 0, 0, creator.TimesRoman},
		{"times-roman", bold, 0, creator.TimesBold},
		{"Times Roman", 0, italic, creator.TimesItalic},
		{"Times New Roman", bold, italic, creator.TimesBoldItalic},
		{"courier", 0, 0, creator.Courier},
		{"Courier New", bold, 0, creator.CourierBold},
		{"Courier", 0, italic, creator.CourierOblique},
		{" Courier ", bold, italic, creator.CourierBoldOblique},
		{"Symbol", bold, 0, creator.Symbol},
		{"ZapfDingbats", 0, 0, creator.ZapfDingbats},
		{"Zapf Dingbats", 0, 0, creator.ZapfDingbats},
	}
	for _, tc := range tests {
		fs := NewFontSet(nil, "")
		got := fs.Resolve(layout.FontRef{Family: tc.family, Weight: tc.weight, Style: tc.style})
		if got.Custom != nil || got.Standard != tc.want {
			t.Errorf("Resolve(%q, %v, %v) = %q, want %q", tc.family, tc.weight, tc.style, got.Standard, tc.want)
		}
		if err := fs.Err(); err != nil {
			t.Errorf("%q is a Standard 14 family, got error %v", tc.family, err)
		}
	}
}

func TestFontSet_UnknownFamilyIsReported(t *testing.T) {
	fs := NewFontSet(nil, "")
	for _, fam := range []string{"Inetr", "Courier Prime", "Inetr", "Garamond"} {
		if got := fs.Resolve(layout.FontRef{Family: fam}); got.Standard != creator.Helvetica {
			t.Errorf("unknown %q resolved to %q, want Helvetica so layout can complete", fam, got.Standard)
		}
	}
	err := fs.Err()
	if err == nil {
		t.Fatal("expected an error for unknown families")
	}
	msg := err.Error()
	// Each family once, sorted, so the error is deterministic.
	want := []string{`"Courier Prime"`, `"Garamond"`, `"Inetr"`}
	last := -1
	for _, w := range want {
		i := strings.Index(msg, w)
		if i < 0 || i < last {
			t.Fatalf("error %q: want %s in sorted order", msg, strings.Join(want, ", "))
		}
		last = i
	}
	if strings.Count(msg, `"Inetr"`) != 1 {
		t.Errorf("Inetr reported more than once: %q", msg)
	}

	fs.Reset()
	if err := fs.Err(); err != nil {
		t.Errorf("after Reset: %v", err)
	}
}

func TestFontSet_CustomFamilyIsExact(t *testing.T) {
	cf := &creator.CustomFont{}
	fs := NewFontSet(map[string]*creator.CustomFont{"Inter": cf}, "")
	if got := fs.Resolve(layout.FontRef{Family: "Inter", Weight: layout.WeightBold}); got.Custom != cf {
		t.Errorf("Inter resolved to %+v, want the registered font", got)
	}
	fs.Resolve(layout.FontRef{Family: "inter"})
	err := fs.Err()
	if err == nil || !strings.Contains(err.Error(), `"inter" is not registered (did you mean "Inter"?`) {
		t.Errorf("registered names are case-sensitive, with a hint: got %v", err)
	}
}

func TestFontSet_Fallback(t *testing.T) {
	cf := &creator.CustomFont{}
	fs := NewFontSet(map[string]*creator.CustomFont{"Inter": cf}, "Inter")
	if err := fs.CheckFallback(); err != nil {
		t.Fatalf("CheckFallback: %v", err)
	}
	if got := fs.Resolve(layout.FontRef{Family: "Inetr"}); got.Custom != cf {
		t.Errorf("unknown family with fallback resolved to %+v, want the fallback font", got)
	}
	if err := fs.Err(); err != nil {
		t.Errorf("with a fallback unknown families are not errors: %v", err)
	}

	std := NewFontSet(nil, "Times")
	if got := std.Resolve(layout.FontRef{Family: "Inetr", Weight: layout.WeightBold}); got.Standard != creator.TimesBold {
		t.Errorf("fallback keeps weight: got %q, want %q", got.Standard, creator.TimesBold)
	}

	if err := NewFontSet(nil, "Nope").CheckFallback(); err == nil {
		t.Error("an unknown fallback must be reported")
	}
}

// TestFontBridgeAndRendererAgree verifies measurement and drawing use the same
// font. Before, layout matched "times…"/"courier…" by prefix while the
// renderer matched exact names, so "courier prime" was measured as Courier
// and drawn in Helvetica.
func TestFontBridgeAndRendererAgree(t *testing.T) {
	fs := NewFontSet(nil, "")
	fb := NewFontBridge(fs)
	for _, fam := range []string{"courier", "Times New Roman", "Arial", "Courier Prime"} {
		ref := layout.FontRef{Family: fam}
		want := fs.Resolve(ref).MeasureString("Hello world", 12)
		if got := fb.MeasureString(ref, "Hello world", 12); got != want {
			t.Errorf("%q: layout measured %.3f, renderer font measures %.3f", fam, got, want)
		}
	}
}
