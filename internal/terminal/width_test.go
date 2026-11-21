package terminal

import "testing"

func TestRuneWidth(t *testing.T) {
	cases := []struct {
		r    rune
		want int
	}{
		{'a', 1},
		{' ', 1},
		{'é', 1},      // precomposed Latin-1
		{'Ω', 1},      // Greek (ambiguous, counts as 1 outside East Asia)
		{'ф', 1},      // Cyrillic
		{'م', 1},      // Arabic letter
		{'世', 2},      // CJK ideograph
		{'界', 2},      // CJK ideograph
		{'あ', 2},      // Hiragana
		{'ア', 2},      // Katakana
		{'한', 2},      // Hangul syllable
		{'Ａ', 2},      // fullwidth Latin
		{'😀', 2},      // emoji
		{'🌍', 2},      // emoji
		{'←', 1},      // arrow (ambiguous)
		{'─', 1},      // box drawing
		{'█', 1},      // full block
		{'\u200c', 0}, // ZWNJ (Persian نیم‌فاصله)
		{'\u200d', 0}, // ZWJ
		{'\u200b', 0}, // zero width space
		{'\u0301', 0}, // combining acute accent
		{'\u064b', 0}, // Arabic combining mark (fathatan)
		{'\ufe0f', 0}, // variation selector 16
		{'\u2060', 0}, // word joiner
		{'\ufeff', 0}, // BOM / zero width no-break space
		{'\x01', 0},   // control character
		{'\x7f', 0},   // DEL
	}
	for _, c := range cases {
		if got := RuneWidth(c.r); got != c.want {
			t.Errorf("RuneWidth(%U) = %d, want %d", c.r, got, c.want)
		}
	}
}

func TestIsZeroWidthAttachable(t *testing.T) {
	attachable := []rune{'\u200c', '\u200d', '\u0301', '\u064b', '\ufe0f', '\u200b'}
	for _, r := range attachable {
		if !IsZeroWidthAttachable(r) {
			t.Errorf("IsZeroWidthAttachable(%U) = false, want true", r)
		}
	}
	notAttachable := []rune{'\x01', '\x7f', 'a', '世'}
	for _, r := range notAttachable {
		if IsZeroWidthAttachable(r) {
			t.Errorf("IsZeroWidthAttachable(%U) = true, want false", r)
		}
	}
}
