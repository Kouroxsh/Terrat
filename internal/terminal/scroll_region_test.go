package terminal

import "testing"

// TestLineFeedBelowScrollRegion (M3): lineFeed must only scroll when the
// cursor is exactly on the bottom of the scroll region. Below the region
// (e.g. under DECSTBM, as in nano) it just moves down, clamped.
func TestLineFeedBelowScrollRegion(t *testing.T) {
	term := New(10, 5)
	writeTerminal(t, term, "AAAAA\r\nBBBBB\r\nCCCCC\r\nDDDDD\r\nEEEEE")
	// Scroll region = rows 1..3 (0-based).
	writeTerminal(t, term, "\x1b[2;4r")
	// Move below the region and feed line feeds: nothing may scroll.
	writeTerminal(t, term, "\x1b[5;1H\n\n")
	if got := rowString(t, term, 1); got != "BBBBB" {
		t.Fatalf("region row 1 moved: got %q, want %q", got, "BBBBB")
	}
	if got := rowString(t, term, 3); got != "DDDDD" {
		t.Fatalf("region row 3 moved: got %q, want %q", got, "DDDDD")
	}
	if _, cy, _ := term.Cursor(); cy != 4 {
		t.Fatalf("cursor should be clamped to the last row, got y=%d", cy)
	}
	// A line feed exactly on the region bottom still scrolls.
	writeTerminal(t, term, "\x1b[4;1H\n")
	if got := rowString(t, term, 1); got != "CCCCC" {
		t.Fatalf("expected region to scroll, row 1 = %q", got)
	}
	if got := rowString(t, term, 3); got != "" {
		t.Fatalf("expected blank row at region bottom, got %q", got)
	}
}

// TestReverseIndexAboveScrollRegion (M4): reverseIndex (ESC M) must only
// scroll when the cursor is exactly on the top of the scroll region.
// Above the region it just moves up, clamped.
func TestReverseIndexAboveScrollRegion(t *testing.T) {
	term := New(10, 5)
	writeTerminal(t, term, "AAAAA\r\nBBBBB\r\nCCCCC\r\nDDDDD\r\nEEEEE")
	// Scroll region = rows 1..3 (0-based).
	writeTerminal(t, term, "\x1b[2;4r")
	// Move above the region and reverse-index: nothing may scroll.
	writeTerminal(t, term, "\x1b[1;1H\x1bM")
	if got := rowString(t, term, 1); got != "BBBBB" {
		t.Fatalf("region row 1 moved: got %q, want %q", got, "BBBBB")
	}
	if got := rowString(t, term, 2); got != "CCCCC" {
		t.Fatalf("region row 2 moved: got %q, want %q", got, "CCCCC")
	}
	if _, cy, _ := term.Cursor(); cy != 0 {
		t.Fatalf("cursor should be clamped to the first row, got y=%d", cy)
	}
	// A reverse index exactly on the region top still scrolls down.
	writeTerminal(t, term, "\x1b[2;1H\x1bM")
	if got := rowString(t, term, 1); got != "" {
		t.Fatalf("expected blank row at region top, got %q", got)
	}
	if got := rowString(t, term, 2); got != "BBBBB" {
		t.Fatalf("expected row 2 to hold the old row 1, got %q", got)
	}
	if got := rowString(t, term, 3); got != "CCCCC" {
		t.Fatalf("expected row 3 to hold the old row 2, got %q", got)
	}
}
