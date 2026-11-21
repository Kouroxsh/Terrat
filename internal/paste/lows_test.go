package paste

import (
	"strings"
	"testing"
)

// L34: valid CSI forms the old regex missed (parameters, intermediates,
// final bytes outside [a-zA-Z~]).
func TestSanitizeCSIForms(t *testing.T) {
	cases := []string{
		"a\x1b[<0;10;5Mb", // mouse SGR report
		"a\x1b[>4;2mb",    // modifyOtherKeys
		"a\x1b[1:2mb",     // colon sub-parameters
		"a\x1b[!pb",       // soft reset (intermediate !)
		"a\x1b[@b",        // ICH, final @
		"a\x1b[?25lb",     // DECTCEM
		"a\x1b[38;2;255;0;0mb",
		"a\x1b[1;2 qb", // intermediate space (cursor style)
	}
	for _, in := range cases {
		if got := Sanitize(in).Sanitized; got != "ab" {
			t.Errorf("Sanitize(%q) = %q, want %q", in, got, "ab")
		}
	}
}

// L34: unterminated OSC/DCS/CSI must not leave their payload behind.
func TestSanitizeUnterminatedSequences(t *testing.T) {
	cases := []struct{ in, want string }{
		{"hello\x1b]0;title", "hello"}, // OSC, no terminator
		{"a\x1b]0;t\nls", "a"},         // payload may hold newlines
		{"a\x1b]0;t\x1b[31mb", "ab"},   // OSC runs into the next ESC
		{"a\x1bPpayload", "a"},         // DCS, no ST
		{"a\x1bPz$qm\x1b\\b", "ab"},    // DCS with ST still stripped
		{"a\x1b[31", "a"},              // CSI, no final byte
		{"a\x1b[31;1\nb", "a\nb"},      // unterminated CSI keeps the newline
		{"a\x1b[31mb", "ab"},           // terminated CSI unchanged
		{"a\x1b]0;t\x07b", "ab"},       // BEL-terminated OSC unchanged
	}
	for _, c := range cases {
		if got := Sanitize(c.in).Sanitized; got != c.want {
			t.Errorf("Sanitize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// L35: control characters must warn, not vanish silently.
func TestSanitizeControlCharactersWarn(t *testing.T) {
	res := Sanitize("a\x01b\x7fc")
	if res.Sanitized != "abc" {
		t.Fatalf("Sanitized = %q, want %q", res.Sanitized, "abc")
	}
	found := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "control") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a control-characters warning, got %v", res.Warnings)
	}
	// Clean text warns about nothing.
	if res := Sanitize("plain text\nwith newline\tand tab"); len(res.Warnings) != 0 {
		t.Fatalf("expected no warnings for clean text, got %v", res.Warnings)
	}
}
