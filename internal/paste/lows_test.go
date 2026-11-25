package paste

import (
	"strings"
	"testing"
)

func TestSanitizeCSIForms(t *testing.T) {
	cases := []string{
		"a\x1b[<0;10;5Mb",
		"a\x1b[>4;2mb",
		"a\x1b[1:2mb",
		"a\x1b[!pb",
		"a\x1b[@b",
		"a\x1b[?25lb",
		"a\x1b[38;2;255;0;0mb",
		"a\x1b[1;2 qb",
	}
	for _, in := range cases {
		if got := Sanitize(in).Sanitized; got != "ab" {
			t.Errorf("Sanitize(%q) = %q, want %q", in, got, "ab")
		}
	}
}

func TestSanitizeUnterminatedSequences(t *testing.T) {
	cases := []struct{ in, want string }{
		{"hello\x1b]0;title", "hello"},
		{"a\x1b]0;t\nls", "a"},
		{"a\x1b]0;t\x1b[31mb", "ab"},
		{"a\x1bPpayload", "a"},
		{"a\x1bPz$qm\x1b\\b", "ab"},
		{"a\x1b[31", "a"},
		{"a\x1b[31;1\nb", "a\nb"},
		{"a\x1b[31mb", "ab"},
		{"a\x1b]0;t\x07b", "ab"},
	}
	for _, c := range cases {
		if got := Sanitize(c.in).Sanitized; got != c.want {
			t.Errorf("Sanitize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

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

	if res := Sanitize("plain text\nwith newline\tand tab"); len(res.Warnings) != 0 {
		t.Fatalf("expected no warnings for clean text, got %v", res.Warnings)
	}
}
