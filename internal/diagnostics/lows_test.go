package diagnostics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNeedsRootBareCommands(t *testing.T) {
	for _, cmd := range []string{"apt", "apt-get", "pacman", "dnf", "yum", "ufw", "fdisk", "mkfs", "iptables", "useradd", "userdel", "groupadd"} {
		if needsRoot(cmd, []string{cmd}) {
			t.Errorf("needsRoot(%q, bare) = true, want false (prints usage)", cmd)
		}
	}

	for _, parts := range [][]string{
		{"apt", "install", "x"},
		{"ufw", "enable"},
		{"fdisk", "/dev/sda"},
		{"apt", "--version"},
	} {
		want := parts[1] != "--version"
		if got := needsRoot(parts[0], parts); got != want {
			t.Errorf("needsRoot(%q) = %v, want %v", parts, got, want)
		}
	}

	for _, cmd := range []string{"reboot", "poweroff", "shutdown"} {
		if !needsRoot(cmd, []string{cmd}) {
			t.Errorf("needsRoot(%q, bare) = false, want true (acts with no args)", cmd)
		}
	}
}

func TestShortSubcommandTypos(t *testing.T) {
	cases := []struct{ input, want string }{
		{"git plu", "Did you mean 'pull'?"},
		{"docker rn", "Did you mean 'run'?"},
		{"go tst", "Did you mean 'test'?"},
		{"git psu", "Did you mean 'push'?"},
	}
	for _, c := range cases {
		d := Analyze(c.input)
		if d == nil || d.Suggestion != c.want {
			t.Fatalf("Analyze(%q) = %+v, want suggestion %q", c.input, d, c.want)
		}
	}

	if d := Analyze("git a"); d != nil {
		t.Fatalf("expected nil for 'git a', got %+v", d)
	}
}

func TestWindowsOnlyCommandsNotSuppressed(t *testing.T) {
	for _, cmd := range []string{"winget", "ipconfig", "cmd", "cls", "dir"} {
		if commonBaseSet[cmd] {
			t.Errorf("commonBaseSet still contains %q", cmd)
		}
	}
	if d := Analyze("cmd"); d == nil || d.Suggestion != "Did you mean 'cd'?" {
		t.Fatalf("expected cd suggestion for 'cmd', got %+v", d)
	}
	if d := Analyze("winget"); d == nil {
		t.Fatalf("expected a diagnostic for 'winget', got nil")
	}
}

func TestQuiDoesNotSuggestQuit(t *testing.T) {
	if fix, ok := commonTypos["qui"]; ok {
		t.Fatalf("commonTypos still maps 'qui' to %q", fix)
	}
	if d := Analyze("qui"); d != nil && strings.Contains(d.Suggestion, "quit") {
		t.Fatalf("Analyze('qui') still suggests quit: %+v", d)
	}
}

func TestEmptyQuotedStringIsAToken(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{`"" gti`, []string{"", "gti"}},
		{`gti ""`, []string{"gti", ""}},
		{`""`, []string{""}},
		{`'' x`, []string{"", "x"}},
		{`a""b`, []string{"ab"}},
		{`"" ""`, []string{"", ""}},
	}
	for _, c := range cases {
		got := LexCommandLine(c.input).Tokens
		if len(got) != len(c.want) {
			t.Fatalf("LexCommandLine(%q).Tokens = %q, want %q", c.input, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("LexCommandLine(%q).Tokens = %q, want %q", c.input, got, c.want)
			}
		}
	}

	if d := Analyze(`"" gti`); d != nil {
		t.Fatalf("expected nil for '\"\" gti', got %+v", d)
	}
}

func TestSeparatorsInsideSubstitution(t *testing.T) {
	for _, input := range []string{
		"echo $(foo; gti)",
		"echo $(foo | gti)",
		"echo $(foo && gti)",
		"echo ${x:-a;b}",
		"echo `foo; gti`",
		"echo $(echo $(a; b))",
		"echo ${x:-$(y;z)}",
	} {
		lex := LexCommandLine(input)
		if lex.ActiveSegment != input {
			t.Fatalf("LexCommandLine(%q).ActiveSegment = %q, want the whole line", input, lex.ActiveSegment)
		}
	}

	if lex := LexCommandLine("echo $(foo; bar); gti"); lex.ActiveSegment != "gti" {
		t.Fatalf("expected active segment %q, got %q", "gti", lex.ActiveSegment)
	}

	if d := Analyze("echo $(foo; gti)"); d != nil {
		t.Fatalf("expected nil for 'echo $(foo; gti)', got %+v", d)
	}
	if d := Analyze("echo ${x:-a;b}"); d != nil {
		t.Fatalf("expected nil for 'echo ${x:-a;b}', got %+v", d)
	}
}

func TestAliasLineTabSeparator(t *testing.T) {
	RegisterAliasFromLine("alias\tlltab='ls -la'")
	if !IsAlias("lltab") {
		t.Fatalf("expected tab-separated alias to be registered")
	}

	RegisterAliasFromLine("aliasfoo=bar")
	if IsAlias("aliasfoo") {
		t.Fatalf("expected 'aliasfoo=bar' not to register an alias")
	}
}

func TestLoadAliasesFromMoreFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	longVal := strings.Repeat("x", 100*1024)
	if err := os.WriteFile(filepath.Join(dir, ".bash_profile"), []byte("alias bp='echo bp'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".profile"), []byte("alias prof='echo prof'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".zsh_aliases"), []byte("alias za='echo za'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".aliases"), []byte("alias al='echo al'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".config", "fish", "conf.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".config", "fish", "conf.d", "mine.fish"), []byte("alias ff='echo ff'\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, ".bashrc"), []byte("alias big='"+longVal+"'\nalias afterbig='echo after'\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	loadAliasesFromFiles()

	for _, name := range []string{"bp", "prof", "za", "al", "ff", "big", "afterbig"} {
		if _, ok := shellAliases.Load(name); !ok {
			t.Fatalf("expected alias %q to be loaded from the new files", name)
		}
	}
}

func TestDamerauLevenshteinDistance(t *testing.T) {
	cases := []struct {
		s, tc string
		want  int
	}{
		{"cta", "cat", 1},
		{"plu", "pull", 2},
		{"plu", "push", 3},
		{"rn", "run", 1},
		{"rn", "rm", 1},
		{"tst", "test", 1},
		{"git", "git", 0},
		{"abc", "def", 3},
	}
	for _, c := range cases {
		if got := DamerauLevenshteinDistance(c.s, c.tc); got != c.want {
			t.Errorf("DamerauLevenshteinDistance(%q, %q) = %d, want %d", c.s, c.tc, got, c.want)
		}
	}

	if got := FindClosestMatch("rn", []string{"rm", "run"}, 1); got != "run" {
		t.Fatalf("FindClosestMatch(rn, [rm run]) = %q, want %q", got, "run")
	}
}
