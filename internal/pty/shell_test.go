package pty

import "testing"

func TestExtractShellName(t *testing.T) {
	tests := []struct{ input, expected string }{
		{"/bin/bash", "bash"},
		{"/usr/bin/zsh", "zsh"},
		{"", "bash"},
	}
	for _, tc := range tests {
		if got := ExtractShellName(tc.input); got != tc.expected {
			t.Errorf("ExtractShellName(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestSetAndGetDefaultShell(t *testing.T) {
	orig := GetDefaultShell()
	defer SetDefaultShell(orig)
	SetDefaultShell("/bin/zsh")
	if got := GetDefaultShell(); got != "/bin/zsh" {
		t.Fatalf("got %q", got)
	}
}
