package autosuggest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTildeCompletion(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	_ = os.Mkdir(filepath.Join(home, "Documents"), 0o755)
	_ = os.WriteFile(filepath.Join(home, "notes.txt"), []byte("x"), 0o644)
	t.Setenv("HOME", home)

	for _, input := range []string{"~", "cd ~", "cd \"~"} {
		if got := suggestPath(input, cwd); got == "" {
			t.Fatalf("expected a completion for %q, got empty", input)
		}
	}

	if got := suggestPath("~/", cwd); got != "Documents/" {
		t.Fatalf("expected 'Documents/' for '~/', got %q", got)
	}
	if got := suggestPath("cd ~/D", cwd); got != "ocuments/" {
		t.Fatalf("expected 'ocuments/' for 'cd ~/D', got %q", got)
	}

	if got := suggestPath("~root", cwd); got != "/" {
		t.Fatalf("expected '/' for '~root', got %q", got)
	}
	if got := suggestPath("~root/.b", cwd); got != "ashrc" {
		t.Fatalf("expected 'ashrc' for '~root/.b', got %q", got)
	}

	if got := suggestPath("~nosuchuserxyz/", cwd); got != "" {
		t.Fatalf("expected empty for '~nosuchuserxyz/', got %q", got)
	}
	if got := expandTilde("~nosuchuserxyz"); got != "~nosuchuserxyz" {
		t.Fatalf("expected unknown user to stay unexpanded, got %q", got)
	}
}

func TestHistoryTailCapAndLongLines(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var sb strings.Builder
	sb.WriteString("oldest marker command\n")
	for i := 0; i < 200000; i++ {
		sb.WriteString("pad command\n")
	}
	sb.WriteString("recent marker command\n")
	if err := os.WriteFile(filepath.Join(home, ".bash_history"), []byte(sb.String()), 0o600); err != nil {
		t.Fatal(err)
	}

	e := &Engine{historyM: make(map[string]struct{}), maxItems: 2000}
	e.loadShellHistory()
	if _, ok := e.historyM["recent marker command"]; !ok {
		t.Fatalf("recent command missing from history: %v", e.history)
	}
	if _, ok := e.historyM["oldest marker command"]; ok {
		t.Fatalf("oldest command should have been cut off by the tail cap")
	}

	longCmd := "echo " + strings.Repeat("x", 100*1024)
	if err := os.WriteFile(filepath.Join(home, ".bash_history"), []byte(longCmd+"\nafter long line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	e2 := &Engine{historyM: make(map[string]struct{}), maxItems: 2000}
	e2.loadShellHistory()
	if _, ok := e2.historyM[longCmd]; !ok {
		t.Fatalf("100KB history line was not loaded")
	}
	if _, ok := e2.historyM["after long line"]; !ok {
		t.Fatalf("line after the long line was not loaded")
	}
}

func TestZshExtendedHistory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	data := ": foo bar\n" +
		": 1700000000:0;git stash\n" +
		": 1700000001:0;for i in 1 2 3; do\n" +
		" echo $i\n" +
		"done\n" +
		": 1700000002:0;\n" +
		": 1700000003:0;echo done\n"
	if err := os.WriteFile(filepath.Join(home, ".zsh_history"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	e := &Engine{historyM: make(map[string]struct{}), maxItems: 100}
	e.loadShellHistory()

	want := []string{
		"echo done",
		"for i in 1 2 3; do\n echo $i\ndone",
		"git stash",
		": foo bar",
	}
	if len(e.history) != len(want) {
		t.Fatalf("history = %q, want %q", e.history, want)
	}
	for i, cmd := range want {
		if e.history[i] != cmd {
			t.Fatalf("history[%d] = %q, want %q", i, e.history[i], cmd)
		}
	}
	for _, fragment := range []string{"echo $i", "done", ": 1700000002:0;"} {
		if _, ok := e.historyM[fragment]; ok {
			t.Fatalf("fragment %q must not be a history entry", fragment)
		}
	}
	if cmd, ok := parseZshExtended(": 1700000002:0;"); !ok || cmd != "" {
		t.Fatalf("parseZshExtended(empty entry) = %q, %v, want \"\", true", cmd, ok)
	}
	if _, ok := parseZshExtended(": foo bar"); ok {
		t.Fatalf("parseZshExtended must reject the ':' builtin command")
	}
	if cmd, ok := parseZshExtended(": 1700000000:0;echo a;b"); !ok || cmd != "echo a;b" {
		t.Fatalf("parseZshExtended = %q, %v, want 'echo a;b', true", cmd, ok)
	}
}

func TestFoldPrefixLen(t *testing.T) {
	cases := []struct {
		s, prefix string
		want      int
	}{
		{"İstanbul", "i", 2},
		{"İstanbul", "İ", 2},
		{"İstanbul", "İS", 3},
		{"abc", "ab", 2},
		{"abc", "x", -1},
		{"abc", "abcd", -1},
		{"", "", 0},
	}
	for _, c := range cases {
		if got := foldPrefixLen(c.s, c.prefix); got != c.want {
			t.Errorf("foldPrefixLen(%q, %q) = %d, want %d", c.s, c.prefix, got, c.want)
		}
	}

	eng := &Engine{historyM: make(map[string]struct{}), maxItems: 100}
	eng.Add("İstanbul")
	if got := eng.Suggest("i"); got != "stanbul" {
		t.Fatalf("expected suffix 'stanbul', got %q", got)
	}
	if got := eng.Suggest("İ"); got != "stanbul" {
		t.Fatalf("expected suffix 'stanbul', got %q", got)
	}
}

func TestDirCacheTTL(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := cachedReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("first read = %v, %v", entries, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err = cachedReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("cached read should still see 1 entry, got %v, %v", entries, err)
	}
	time.Sleep(dirCacheTTL + 200*time.Millisecond)
	entries, err = cachedReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("expired read should see 2 entries, got %v, %v", entries, err)
	}
}
