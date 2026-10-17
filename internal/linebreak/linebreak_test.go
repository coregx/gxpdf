package linebreak

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitWord(t *testing.T) {
	tests := []struct {
		word string
		want []string
	}{
		{"", nil},
		{"hello", []string{"hello"}},
		{"中文", []string{"中", "文"}},
		{"日本語です", []string{"日", "本", "語", "で", "す"}},
		{"한국어", []string{"한", "국", "어"}},
		// Latin next to CJK breaks; Latin runs stay whole.
		{"Go言語", []string{"Go", "言", "語"}},
		{"中文abc", []string{"中", "文", "abc"}},
		// Closing punctuation stays with the text before it.
		{"中文。", []string{"中", "文。"}},
		{"はい、そうです。", []string{"は", "い、", "そ", "う", "で", "す。"}},
		{"中文.", []string{"中", "文."}},
		{"中文!?", []string{"中", "文!?"}},
		{"々", []string{"々"}},
		{"時々", []string{"時々"}},
		// Opening punctuation stays with the text after it.
		{"「中文」", []string{"「中", "文」"}},
		{"(中文)", []string{"(中", "文)"}},
		{"（注）", []string{"（注）"}},
		// Small kana and ー may start a line (line-break: normal).
		{"ちょっと", []string{"ち", "ょ", "っ", "と"}},
		{"コーヒー", []string{"コ", "ー", "ヒ", "ー"}},
		// Fullwidth forms count as CJK.
		{"ＡＢ", []string{"Ａ", "Ｂ"}},
		// Non-CJK scripts keep words whole.
		{"Привет", []string{"Привет"}},
		{"naïve", []string{"naïve"}},
	}
	for _, tt := range tests {
		got := SplitWord(tt.word)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("SplitWord(%q) = %q, want %q", tt.word, got, tt.want)
		}
		if strings.Join(got, "") != tt.word {
			t.Errorf("SplitWord(%q) pieces %q do not join back", tt.word, got)
		}
	}
}

func TestIsCJK(t *testing.T) {
	for _, r := range "中あア가、。「」（）ＡｱⅪ" {
		want := r != 'Ⅺ'
		if got := IsCJK(r); got != want {
			t.Errorf("IsCJK(%q U+%04X) = %v, want %v", r, r, got, want)
		}
	}
	for _, r := range "Az1 é—Ж" {
		if IsCJK(r) {
			t.Errorf("IsCJK(%q U+%04X) = true", r, r)
		}
	}
	// Ideographs outside the basic block: Extension A and B.
	for _, r := range []rune{0x3400, 0x20000} {
		if !IsCJK(r) {
			t.Errorf("IsCJK(U+%04X) = false", r)
		}
	}
}

func FuzzSplitWord(f *testing.F) {
	for _, s := range []string{"中文", "「中文」。", "Go言語abc", "ちょっと", "hello"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, word string) {
		pieces := SplitWord(word)
		if strings.Join(pieces, "") != word {
			t.Fatalf("SplitWord(%q) = %q, does not join back", word, pieces)
		}
		for _, p := range pieces {
			if p == "" {
				t.Fatalf("SplitWord(%q) has an empty piece: %q", word, pieces)
			}
		}
	})
}
