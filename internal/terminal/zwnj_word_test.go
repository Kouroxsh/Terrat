package terminal

import "testing"

// M6: double-click word selection must not split a Persian word at ZWNJ.
// After H2, ZWNJ attaches to the previous cell's grapheme and occupies no
// cell of its own, so the word walk sees consecutive letters.
func TestSelectWordPersianZWNJ(t *testing.T) {
	term := New(20, 5)
	if _, err := term.Write([]byte("می‌کنم")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	// Double-click on the middle of the word (column of 'ک').
	term.SelectWord(3, 0)
	got := term.GetSelectedText()
	if got != "می‌کنم" {
		t.Fatalf("SelectWord on می‌کنم = %q, want %q", got, "می‌کنم")
	}
	// Double-click on the first half must select the whole word too.
	term.SelectWord(0, 0)
	if got := term.GetSelectedText(); got != "می‌کنم" {
		t.Fatalf("SelectWord at word start = %q, want %q", got, "می‌کنم")
	}
}
