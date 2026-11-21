package autosuggest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAutosuggestBasic(t *testing.T) {
	eng := NewEngine()
	eng.Add("git checkout -b feature/test")
	eng.Add("docker ps --format '{{.Names}}'")

	suffix := eng.Suggest("git check")
	expected := "out -b feature/test"
	if suffix != expected {
		t.Fatalf("expected suffix %q, got %q", expected, suffix)
	}

	suffix = eng.Suggest("GIT CHECK")
	if suffix != expected {
		t.Fatalf("expected case-insensitive matching %q, got %q", expected, suffix)
	}

	suffix = eng.Suggest("docker ps")
	expectedDocker := " --format '{{.Names}}'"
	if suffix != expectedDocker {
		t.Fatalf("expected docker suffix %q, got %q", expectedDocker, suffix)
	}
}

func TestAutosuggestAddAndPrioritize(t *testing.T) {
	eng := &Engine{
		historyM: make(map[string]struct{}),
		maxItems: 5,
	}

	eng.Add("make build")
	eng.Add("make test")

	// Most recently added should match first
	suffix := eng.Suggest("make")
	if suffix != " test" {
		t.Fatalf("expected ' test' as most recent match, got %q", suffix)
	}

	// Re-add make build to bump priority
	eng.Add("make build")
	suffix = eng.Suggest("make")
	if suffix != " build" {
		t.Fatalf("expected ' build' after priority bump, got %q", suffix)
	}
}

func TestAutosuggestEmpty(t *testing.T) {
	eng := NewEngine()
	if eng.Suggest("") != "" {
		t.Fatalf("empty input should return empty suggestion")
	}
	if eng.Suggest("completely_unknown_command_xyz_123") != "" {
		t.Fatalf("unknown input should return empty suggestion")
	}
}

func TestAutosuggestPathWithSpaces(t *testing.T) {
	tmpDir := t.TempDir()

	_ = os.Mkdir(filepath.Join(tmpDir, "My Projects Folder"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "script with space.sh"), []byte("echo"), 0755)

	eng := NewEngine()

	// 1. Unquoted prefix
	suffix := eng.Suggest("cd My", tmpDir)
	if suffix != "\\ Projects\\ Folder/" {
		t.Fatalf("expected '\\ Projects\\ Folder/', got %q", suffix)
	}

	// 2. Escaped space prefix
	suffix = eng.Suggest("cd My\\ ", tmpDir)
	if suffix != "Projects\\ Folder/" {
		t.Fatalf("expected 'Projects\\ Folder/', got %q", suffix)
	}

	// 3. Double-quoted prefix
	suffix = eng.Suggest("cd \"My", tmpDir)
	if suffix != " Projects Folder/" {
		t.Fatalf("expected ' Projects Folder/', got %q", suffix)
	}

	// 4. Single unescaped space typed after first word
	suffix = eng.Suggest("cd My ", tmpDir)
	if suffix != "Projects\\ Folder/" {
		t.Fatalf("expected 'Projects\\ Folder/', got %q", suffix)
	}

	// 5. File match
	suffix = eng.Suggest("cat scr", tmpDir)
	if suffix != "ipt\\ with\\ space.sh" {
		t.Fatalf("expected 'ipt\\ with\\ space.sh', got %q", suffix)
	}
}

func TestHistoryLoadingIsBoundedAndKeepsRecentUniqueCommands(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	data := "old command\nbeta command\nalpha command\ngamma command\nalpha command\n"
	if err := os.WriteFile(filepath.Join(home, ".bash_history"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".zsh_history"), []byte("other command\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e := &Engine{historyM: make(map[string]struct{}), maxItems: 3}
	e.loadShellHistory()
	want := []string{"alpha command", "gamma command", "beta command"}
	if len(e.history) != len(want) || len(e.historyM) != len(want) {
		t.Fatalf("history exceeded limit: %d/%d", len(e.history), len(e.historyM))
	}
	for i, cmd := range want {
		if e.history[i] != cmd {
			t.Fatalf("history[%d] = %q, want %q", i, e.history[i], cmd)
		}
	}
	e.Add("new command")
	if _, ok := e.historyM["beta command"]; ok {
		t.Fatal("evicted command retained in map")
	}
	if len(e.history) != 3 || e.history[0] != "new command" {
		t.Fatal("new command did not replace the oldest")
	}
}

// M24: real shell history must win over the built-in defaults.
func TestHistoryBeatsDefaults(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".bash_history"), []byte("git stash\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	eng := NewEngine()
	if got := eng.Suggest("git st"); got != "ash" {
		t.Fatalf("expected history 'git stash' to win (suffix %q), got %q", "ash", got)
	}
}

// M25: a token ending in `/` completes inside that directory.
func TestTrailingSlashListsDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "usr"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "usr", "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "usr", "lib.txt"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	eng := &Engine{historyM: make(map[string]struct{}), maxItems: 100}
	got := eng.Suggest("cd "+filepath.Join(dir, "usr")+"/", dir)
	if got != "bin/" {
		t.Fatalf("expected first entry 'bin/' inside usr/, got %q", got)
	}
	// `cd ../` from a subdirectory lists the parent.
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "aaa.txt"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := eng.Suggest("cd ../", sub); got == "" {
		t.Fatal("expected 'cd ../' to list the parent directory, got empty")
	}
}

// M26: the first token (command word) must not be path-completed.
func TestFirstTokenNotPathCompleted(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	eng := &Engine{historyM: make(map[string]struct{}), maxItems: 100}
	eng.Add("docker ps -a")
	if got := eng.Suggest("do", dir); got != "cker ps -a" {
		t.Fatalf("expected history suggestion 'cker ps -a', got %q", got)
	}
	// ... but a real path as the first token still completes.
	if err := os.Mkdir(filepath.Join(dir, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	if got := eng.Suggest("./b", dir); got != "in/" {
		t.Fatalf("expected path suggestion 'in/' for './b', got %q", got)
	}
}

// M27: bash HISTTIMEFORMAT timestamps must never be suggested.
func TestTimestampLinesNotSuggested(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".bash_history"), []byte("#1600000000\necho hi\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	eng := NewEngine()
	if got := eng.Suggest("#"); got != "" {
		t.Fatalf("expected no suggestion for '#', got %q", got)
	}
	if got := eng.Suggest("echo"); got != " hi" {
		t.Fatalf("expected history suggestion ' hi', got %q", got)
	}
}
