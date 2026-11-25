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

func TestResolveUnixShellFallback(t *testing.T) {
	t.Setenv("SHELL", "/nonexistent/shell-xyz")
	got := ResolveUnixShell("/nonexistent/configured-sh")
	if len(got) == 0 || got[0] == "/nonexistent/configured-sh" || got[0] == "/nonexistent/shell-xyz" {
		t.Fatalf("expected a fallback shell, got %v", got)
	}
	if got[0] != "/bin/bash" && got[0] != "/bin/sh" {
		t.Fatalf("expected /bin/bash or /bin/sh fallback, got %v", got)
	}
}

func TestResolveUnixShellValid(t *testing.T) {
	t.Setenv("SHELL", "/bin/bash")
	if got := ResolveUnixShell("sh"); len(got) != 1 || got[0] != "sh" {
		t.Fatalf("expected [sh], got %v", got)
	}
	if got := ResolveUnixShell("/bin/bash --login"); len(got) != 2 || got[0] != "/bin/bash" || got[1] != "--login" {
		t.Fatalf("expected [/bin/bash --login], got %v", got)
	}
	if got := ResolveUnixShell("", "echo", "hi"); len(got) != 2 || got[0] != "echo" {
		t.Fatalf("expected custom command to win, got %v", got)
	}
}
