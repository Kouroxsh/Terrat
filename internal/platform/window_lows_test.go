//go:build linux

package platform

import (
	"strings"

	"github.com/jezek/xgb/xproto"
	"testing"
)

func TestToLatin1(t *testing.T) {
	// Pure ASCII passes through unchanged.
	if got := string(toLatin1("bash")); got != "bash" {
		t.Fatalf("toLatin1(bash) = %q", got)
	}
	// Latin-1 runes (é, ü, ÿ) survive as single bytes; anything above
	// U+00FF (em dash) and the C1 controls become '?'. Compare bytes:
	// the expected string is Latin-1, not UTF-8.
	want := []byte{'c', 'a', 'f', 0xE9, ' ', '?', ' ', 0xFC, 'n', 0xEF, 'c', 'o', 'd', 'e', ' ', 0xFF}
	if got := toLatin1("café — ünïcode ÿ"); string(got) != string(want) {
		t.Fatalf("toLatin1 = %q, want %q", got, want)
	}
	if got := toLatin1("a\x85b"); string(got) != "a?b" {
		t.Fatalf("toLatin1(C1) = %q", got)
	}
	// A Persian title degrades to '?' per rune, not raw UTF-8 bytes.
	if got := string(toLatin1("سلام")); got != "????" {
		t.Fatalf("toLatin1(Persian) = %q", got)
	}
}

func TestOpenURLRejectsNonHTTPS(t *testing.T) {
	for _, u := range []string{
		"file:///etc/passwd",
		"ssh://example.com",
		"javascript:alert(1)",
		"ftp://example.com/x",
		"//example.com/no-scheme",
		"not a url",
		"",
	} {
		if err := OpenURL(u); err == nil {
			t.Errorf("OpenURL(%q) accepted a non-http(s) URL", u)
		}
	}
	// An https URL must get past validation (xdg-open itself may fail
	// in this VM — only a validation error is a test failure).
	if err := OpenURL("https://example.com/"); err != nil && strings.Contains(err.Error(), "refusing to open") {
		t.Errorf("OpenURL(https) rejected: %v", err)
	}
}

func TestParseMultiplePairs(t *testing.T) {
	// Two complete pairs decode in order.
	v := []byte{1, 0, 0, 0, 10, 0, 0, 0, 2, 0, 0, 0, 20, 0, 0, 0}
	pairs := parseMultiplePairs(v)
	if len(pairs) != 2 || pairs[0] != [2]xproto.Atom{1, 10} || pairs[1] != [2]xproto.Atom{2, 20} {
		t.Fatalf("parseMultiplePairs = %v", pairs)
	}
	// A truncated tail (5 bytes) is dropped, not misread.
	if got := parseMultiplePairs(append(v, 9, 0, 0, 0, 7)); len(got) != 2 {
		t.Fatalf("truncated tail: got %d pairs, want 2", len(got))
	}
	// Empty input is nil.
	if got := parseMultiplePairs(nil); got != nil {
		t.Fatalf("empty input: got %v, want nil", got)
	}
}

func TestSelectionTargets(t *testing.T) {
	w := &Window{AtomTargets: 1, AtomUtf8String: 2, AtomText: 3, AtomIncr: 4, AtomMultiple: 5, AtomTimestamp: 6}
	got := w.selectionTargets()
	want := []uint32{1, 2, uint32(xproto.AtomString), 3, 4, 5, 6}
	if len(got) != len(want) {
		t.Fatalf("selectionTargets = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("selectionTargets[%d] = %d, want %d", i, got[i], want[i])
		}
	}
	// With the optional atoms unset, only the base targets are listed.
	w2 := &Window{AtomTargets: 1, AtomUtf8String: 2}
	if got := w2.selectionTargets(); len(got) != 3 {
		t.Fatalf("minimal selectionTargets = %v, want 3 entries", got)
	}
}

func TestSelectionTimestamp(t *testing.T) {
	w := &Window{}
	w.clipboardTime = 424242
	// Any non-PRIMARY selection reads the clipboard time.
	if got := w.selectionTimestamp(xproto.Atom(999)); got != 424242 {
		t.Fatalf("selectionTimestamp(CLIPBOARD) = %d, want 424242", got)
	}
	w.primaryTime = 777
	if got := w.selectionTimestamp(xproto.AtomPrimary); got != 777 {
		t.Fatalf("selectionTimestamp(PRIMARY) = %d, want 777", got)
	}
}
