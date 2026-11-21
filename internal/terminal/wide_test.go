package terminal

import "testing"

// TestPutCharWideOccupiesTwoCells is the core H2 regression test: a wide
// rune takes two grid columns and the following text lands after it.
func TestPutCharWideOccupiesTwoCells(t *testing.T) {
	term := New(80, 24)
	if _, err := term.Write([]byte("世界ab")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	cx, _, _ := term.Cursor()
	if cx != 6 {
		t.Fatalf("cursor X = %d, want 6 (2+2+1+1)", cx)
	}
	if got := term.GetCell(0, 0); got.Char != '世' || !got.Wide {
		t.Fatalf("cell 0: got %+v, want lead cell of U+4E16", got)
	}
	if got := term.GetCell(1, 0); !got.Continuation {
		t.Fatalf("cell 1: got %+v, want continuation cell", got)
	}
	if got := term.GetCell(2, 0); got.Char != '界' || !got.Wide {
		t.Fatalf("cell 2: got %+v, want lead cell of U+754C", got)
	}
	if got := term.GetCell(3, 0); !got.Continuation {
		t.Fatalf("cell 3: got %+v, want continuation cell", got)
	}
	if got := term.GetCell(4, 0).Char; got != 'a' {
		t.Fatalf("cell 4: got %U, want 'a'", got)
	}
	if got := term.GetCell(5, 0).Char; got != 'b' {
		t.Fatalf("cell 5: got %U, want 'b'", got)
	}
}

// TestPutCharZWNJDoesNotAdvance verifies the Persian ZWNJ (U+200C) takes
// no cell: "می‌کنم" is 5 columns, and the ZWNJ survives in the extracted text.
func TestPutCharZWNJDoesNotAdvance(t *testing.T) {
	term := New(80, 24)
	if _, err := term.Write([]byte("می‌کنم")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	cx, _, _ := term.Cursor()
	if cx != 5 {
		t.Fatalf("cursor X = %d, want 5 (ZWNJ takes no cell)", cx)
	}
	if got := term.GetAllText(); got != "می‌کنم" {
		t.Fatalf("GetAllText() = %q, want %q (ZWNJ must round-trip)", got, "می‌کنم")
	}
	// The ZWNJ is stored on the preceding cell's grapheme.
	cell := term.GetCell(1, 0)
	if len(cell.Extra) != 1 || cell.Extra[0] != '\u200c' {
		t.Fatalf("cell 1 Extra = %U, want [U+200C]", cell.Extra)
	}
}

// TestPutCharCombiningMarkAttaches verifies a combining mark does not
// create a cell of its own and is preserved in the extracted text.
func TestPutCharCombiningMarkAttaches(t *testing.T) {
	term := New(80, 24)
	if _, err := term.Write([]byte("e\u0301x")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	cx, _, _ := term.Cursor()
	if cx != 2 {
		t.Fatalf("cursor X = %d, want 2 (combining mark takes no cell)", cx)
	}
	if got := term.GetAllText(); got != "e\u0301x" {
		t.Fatalf("GetAllText() = %q, want %q", got, "e\u0301x")
	}
}

// TestPutCharWideWrapsAtEdge: a wide rune that does not fit in the last
// column wraps to the next line instead of being split.
func TestPutCharWideWrapsAtEdge(t *testing.T) {
	term := New(4, 2)
	if _, err := term.Write([]byte("abc世")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if got := term.GetRowString(0); got[:3] != "abc" {
		t.Fatalf("row 0 = %q, want it to start with \"abc\"", got)
	}
	if got := term.GetCell(0, 1); got.Char != '世' || !got.Wide {
		t.Fatalf("row 1 cell 0: got %+v, want lead cell of U+4E16", got)
	}
	if got := term.GetCell(1, 1); !got.Continuation {
		t.Fatalf("row 1 cell 1: got %+v, want continuation cell", got)
	}
}

// TestPutCharWideExactFit: a wide rune that exactly fills the last two
// columns stays on the line; the next character wraps.
func TestPutCharWideExactFit(t *testing.T) {
	term := New(4, 2)
	if _, err := term.Write([]byte("ab世x")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if got := term.GetCell(2, 0); got.Char != '世' || !got.Wide {
		t.Fatalf("row 0 cell 2: got %+v, want lead cell of U+4E16", got)
	}
	if got := term.GetCell(0, 1).Char; got != 'x' {
		t.Fatalf("row 1 cell 0: got %U, want 'x'", got)
	}
}

// TestEmojiOccupiesTwoCells verifies emoji advance the cursor by two.
func TestEmojiOccupiesTwoCells(t *testing.T) {
	term := New(80, 24)
	if _, err := term.Write([]byte("a😀b")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	cx, _, _ := term.Cursor()
	if cx != 4 {
		t.Fatalf("cursor X = %d, want 4 (1+2+1)", cx)
	}
	if got := term.GetAllText(); got != "a😀b" {
		t.Fatalf("GetAllText() = %q, want %q", got, "a😀b")
	}
}

// TestRowTextWideSearch: searching a wide word finds it at grid columns.
func TestRowTextWideSearch(t *testing.T) {
	term := New(80, 24)
	if _, err := term.Write([]byte("世界ab世界")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	text, colMap := term.RowText(0)
	if text != "世界ab世界" {
		t.Fatalf("RowText() = %q, want %q", text, "世界ab世界")
	}
	matches := FindRowMatchesCols(text, colMap, "世界")
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %+v", matches)
	}
	if matches[0].StartCol != 0 || matches[0].EndCol != 3 {
		t.Fatalf("unexpected first match: %+v", matches[0])
	}
	if matches[1].StartCol != 6 || matches[1].EndCol != 9 {
		t.Fatalf("unexpected second match: %+v", matches[1])
	}
	// A query spanning the wide/narrow boundary still maps correctly.
	matches = FindRowMatchesCols(text, colMap, "界ab")
	if len(matches) != 1 || matches[0].StartCol != 2 || matches[0].EndCol != 5 {
		t.Fatalf("unexpected boundary match: %+v", matches)
	}
}

// TestSelectedTextWide verifies copy of a selection covering wide runes
// returns each wide rune once, not once per cell.
func TestSelectedTextWide(t *testing.T) {
	term := New(80, 24)
	if _, err := term.Write([]byte("世界ab")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	term.StartSelection(0, 0)
	term.UpdateSelection(5, 0)
	if got := term.GetSelectedText(); got != "世界ab" {
		t.Fatalf("GetSelectedText() = %q, want %q", got, "世界ab")
	}
	// Selecting only the two cells of the first wide rune still copies it once.
	term.StartSelection(0, 0)
	term.UpdateSelection(1, 0)
	if got := term.GetSelectedText(); got != "世" {
		t.Fatalf("GetSelectedText() = %q, want %q", got, "世")
	}
}

// TestEraseCutsWideRune: erasing half of a wide rune must not leave a
// dangling lead or continuation behind.
func TestEraseCutsWideRune(t *testing.T) {
	term := New(80, 24)
	if _, err := term.Write([]byte("a世b")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	// Erase from the continuation cell (col 2) to the end of the line:
	// the lead at col 1 must lose its Wide flag.
	if _, err := term.Write([]byte("\x1b[1;3H\x1b[0K")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if got := term.GetCell(1, 0); got.Wide {
		t.Fatalf("cell 1: got %+v, want the dangling lead repaired", got)
	}
	if got := term.GetCell(2, 0); got.Continuation {
		t.Fatalf("cell 2: got %+v, want the erased cell back to a space", got)
	}
}

// TestWordBoundsWide: double-clicking the second half of a wide rune
// selects the whole rune.
func TestWordBoundsWide(t *testing.T) {
	term := New(80, 24)
	if _, err := term.Write([]byte("a世b")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	// Simulate a word double-click on the continuation cell (col 2).
	term.SelectWord(2, 0)
	if got := term.GetSelectedText(); got != "a世b" {
		t.Fatalf("GetSelectedText() = %q, want %q", got, "a世b")
	}
}
