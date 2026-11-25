package terminal

import "testing"

func TestSelectWordPersianZWNJ(t *testing.T) {
	term := New(20, 5)
	if _, err := term.Write([]byte("می‌کنم")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	term.SelectWord(3, 0)
	got := term.GetSelectedText()
	if got != "می‌کنم" {
		t.Fatalf("SelectWord on می‌کنم = %q, want %q", got, "می‌کنم")
	}

	term.SelectWord(0, 0)
	if got := term.GetSelectedText(); got != "می‌کنم" {
		t.Fatalf("SelectWord at word start = %q, want %q", got, "می‌کنم")
	}
}
