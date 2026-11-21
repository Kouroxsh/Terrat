package terminal

import "testing"

// TestFindMatchesScrollback (M5): search must cover the scrollback, not
// just the visible grid. Absolute rows count from the oldest scrollback
// row (0) through the grid rows.
func TestFindMatchesScrollback(t *testing.T) {
	term := New(10, 4)
	writeTerminal(t, term, "line0\r\nline1\r\nline2\r\nline3\r\nline4\r\nline5\r\nline6\r\nline7\r\nline8\r\nline9")
	if got := term.ScrollbackLen(); got != 6 {
		t.Fatalf("expected 6 scrollback rows, got %d", got)
	}

	m := term.FindMatches("line1")
	if len(m) != 1 || m[0].AbsRow != 1 || m[0].StartCol != 0 || m[0].EndCol != 4 {
		t.Fatalf("line1 should match scrollback row 1, got %+v", m)
	}

	m = term.FindMatches("line9")
	if len(m) != 1 || m[0].AbsRow != 9 {
		t.Fatalf("line9 should match grid row 3 (absolute row 9), got %+v", m)
	}

	// Case-insensitive, like the viewport search.
	m = term.FindMatches("LINE5")
	if len(m) != 1 || m[0].AbsRow != 5 {
		t.Fatalf("LINE5 should match absolute row 5, got %+v", m)
	}

	if m := term.FindMatches(""); m != nil {
		t.Fatalf("empty query should return nil, got %+v", m)
	}
	if m := term.FindMatches("nope"); len(m) != 0 {
		t.Fatalf("missing query should return no matches, got %+v", m)
	}
}

// TestFindMatchesWrappedLine (M5): a query spanning a wrapped line must be
// found, split into one entry per physical row.
func TestFindMatchesWrappedLine(t *testing.T) {
	term := New(10, 4)
	writeTerminal(t, term, "abcdefghijklmnop")
	// Row 0: "abcdefghij", row 1: "klmnop" (wrapped continuation).
	m := term.FindMatches("jklm")
	if len(m) != 2 {
		t.Fatalf("expected 2 row entries for a wrap-spanning match, got %+v", m)
	}
	if m[0] != (AbsoluteSearchMatch{AbsRow: 0, StartCol: 9, EndCol: 9}) {
		t.Fatalf("unexpected first row entry: %+v", m[0])
	}
	if m[1] != (AbsoluteSearchMatch{AbsRow: 1, StartCol: 0, EndCol: 2}) {
		t.Fatalf("unexpected second row entry: %+v", m[1])
	}

	// A query fully inside the second row still works.
	m = term.FindMatches("klm")
	if len(m) != 1 || m[0] != (AbsoluteSearchMatch{AbsRow: 1, StartCol: 0, EndCol: 2}) {
		t.Fatalf("unexpected match for 'klm': %+v", m)
	}

	// A full row followed by an explicit newline is NOT one logical line:
	// "9abc" must not match across the newline.
	term2 := New(10, 4)
	writeTerminal(t, term2, "0123456789\r\nabcdefghij")
	if m := term2.FindMatches("9abc"); len(m) != 0 {
		t.Fatalf("must not match across an explicit newline, got %+v", m)
	}
}

// TestFindMatchesWrappedLineInScrollback (M5): wrapped flags must survive
// scrolling into the history, so wrap-spanning matches are still found
// after the lines leave the viewport.
func TestFindMatchesWrappedLineInScrollback(t *testing.T) {
	term := New(10, 4)
	writeTerminal(t, term, "abcdefghijklmnop\r\nAAAA\r\nBBBB\r\nCCCC\r\nDDDD\r\nEEEE")
	if got := term.ScrollbackLen(); got != 3 {
		t.Fatalf("expected 3 scrollback rows, got %d", got)
	}
	m := term.FindMatches("jklm")
	if len(m) != 2 {
		t.Fatalf("expected 2 row entries for a wrap-spanning match in scrollback, got %+v", m)
	}
	if m[0] != (AbsoluteSearchMatch{AbsRow: 0, StartCol: 9, EndCol: 9}) {
		t.Fatalf("unexpected first row entry: %+v", m[0])
	}
	if m[1] != (AbsoluteSearchMatch{AbsRow: 1, StartCol: 0, EndCol: 2}) {
		t.Fatalf("unexpected second row entry: %+v", m[1])
	}
}

// TestFindMatchesAltScreen (M5): the alternate screen has no scrollback;
// search covers just the alt grid.
func TestFindMatchesAltScreen(t *testing.T) {
	term := New(10, 4)
	writeTerminal(t, term, "visible\r\nlines\r\n")
	writeTerminal(t, term, "\x1b[?1049h\x1b[H")
	writeTerminal(t, term, "alttext")
	m := term.FindMatches("alttext")
	if len(m) != 1 || m[0].AbsRow != 0 {
		t.Fatalf("alt-screen match should be row 0, got %+v", m)
	}
	if m := term.FindMatches("visible"); len(m) != 0 {
		t.Fatalf("main-screen content must not match in the alt screen, got %+v", m)
	}
}
