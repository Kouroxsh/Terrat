package terminal

import (
	"strings"
	"testing"
)

func writeTerminal(t *testing.T, term *Terminal, s string) {
	t.Helper()
	if _, err := term.Write([]byte(s)); err != nil {
		t.Fatalf("write failed: %v", err)
	}
}

func rowString(t *testing.T, term *Terminal, y int) string {
	t.Helper()
	text, _ := term.RowText(y)
	return text
}

// TestOSCDiscardOverflow: an OSC string longer than the 4096-rune buffer
// must be swallowed up to its terminator, never printed as typed text.
func TestOSCDiscardOverflow(t *testing.T) {
	term := New(80, 24)
	writeTerminal(t, term, "\x1b]0;"+strings.Repeat("x", 5000)+"\aOK")
	if got := rowString(t, term, 0); got != "OK" {
		t.Fatalf("over-long OSC leaked onto the screen: row 0 = %q", got)
	}

	// The truncated OSC must not have been executed as a title either.
	select {
	case title := <-term.TitleChan:
		t.Fatalf("truncated OSC was executed as title %q", title)
	default:
	}

	// A normal OSC afterwards still works.
	writeTerminal(t, term, "\x1b]2;hello\a")
	select {
	case title := <-term.TitleChan:
		if title != "hello" {
			t.Fatalf("expected title %q, got %q", "hello", title)
		}
	default:
		t.Fatalf("valid OSC after a discarded one produced no title")
	}
}

func TestOSCDiscardOverflowSTTerminator(t *testing.T) {
	term := New(80, 24)
	writeTerminal(t, term, "\x1b]0;"+strings.Repeat("y", 5000)+"\x1b\\OK")
	if got := rowString(t, term, 0); got != "OK" {
		t.Fatalf("over-long OSC with ST terminator leaked: row 0 = %q", got)
	}
}

func TestOSCDiscardOverflowCANTerminator(t *testing.T) {
	term := New(80, 24)
	writeTerminal(t, term, "\x1b]0;"+strings.Repeat("z", 5000)+"\x18OK")
	if got := rowString(t, term, 0); got != "OK" {
		t.Fatalf("over-long OSC with CAN terminator leaked: row 0 = %q", got)
	}
}

func TestOSCAtLimitStillWorks(t *testing.T) {
	term := New(80, 24)
	// Just under the limit: a normal OSC must still execute.
	writeTerminal(t, term, "\x1b]2;"+strings.Repeat("a", 100)+"\a")
	select {
	case title := <-term.TitleChan:
		if title != strings.Repeat("a", 100) {
			t.Fatalf("unexpected title length %d", len(title))
		}
	default:
		t.Fatalf("OSC under the limit produced no title")
	}
}
