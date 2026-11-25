package terminal

import "testing"

func TestRuneWidth(t *testing.T) {
	cases := []struct {
		r    rune
		want int
	}{
		{'a', 1},
		{' ', 1},
		{'é', 1},
		{'Ω', 1},
		{'ф', 1},
		{'م', 1},
		{'世', 2},
		{'界', 2},
		{'あ', 2},
		{'ア', 2},
		{'한', 2},
		{'Ａ', 2},
		{'😀', 2},
		{'🌍', 2},
		{'←', 1},
		{'─', 1},
		{'█', 1},
		{'\u200c', 0},
		{'\u200d', 0},
		{'\u200b', 0},
		{'\u0301', 0},
		{'\u064b', 0},
		{'\ufe0f', 0},
		{'\u2060', 0},
		{'\ufeff', 0},
		{'\x01', 0},
		{'\x7f', 0},
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
