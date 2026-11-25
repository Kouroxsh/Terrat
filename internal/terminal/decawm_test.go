package terminal

import (
	"testing"
)

func TestDECAWMAutoWrap(t *testing.T) {
	term := New(10, 5)
	term.Write([]byte("1234567890AB"))
	if r := term.GetCell(9, 0).Char; r != '0' {
		t.Fatalf("expected '0' at (9, 0), got %c", r)
	}
	if r := term.GetCell(0, 1).Char; r != 'A' {
		t.Fatalf("expected 'A' at (0, 1), got %c", r)
	}
	if r := term.GetCell(1, 1).Char; r != 'B' {
		t.Fatalf("expected 'B' at (1, 1), got %c", r)
	}

	term = New(10, 5)
	term.Write([]byte("\x1b[?7l1234567890ABCDEF"))

	if r := term.GetCell(0, 1).Char; r != 0 && r != ' ' {
		t.Fatalf("expected empty cell at (0, 1) when DECAWM disabled, got %c", r)
	}
	x, y, _ := term.Cursor()
	if y != 0 || x != 9 {
		t.Fatalf("expected cursor at (9, 0) with DECAWM disabled, got (%d, %d)", x, y)
	}

	if r := term.GetCell(9, 0).Char; r != 'F' {
		t.Fatalf("expected 'F' at (9, 0), got %c", r)
	}

	term.Write([]byte("\x1b[?7hZW"))
	if r := term.GetCell(9, 0).Char; r != 'Z' {
		t.Fatalf("expected 'Z' at (9, 0), got %c", r)
	}
	if r := term.GetCell(0, 1).Char; r != 'W' {
		t.Fatalf("expected 'W' at (0, 1) after re-enabling DECAWM, got %c", r)
	}
}
