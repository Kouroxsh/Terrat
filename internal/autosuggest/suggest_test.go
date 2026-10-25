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

func TestAutosuggestWindowsPathSeparators(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	_ = os.Mkdir(subDir, 0755)
	_ = os.WriteFile(filepath.Join(subDir, "target_file.txt"), []byte("data"), 0644)

	eng := NewEngine()

	suffix := eng.Suggest("cat sub\\tar", tmpDir)
	if suffix != "get_file.txt" {
		t.Fatalf("expected 'get_file.txt', got %q", suffix)
	}

	suffix = eng.Suggest("cat sub/tar", tmpDir)
	if suffix != "get_file.txt" {
		t.Fatalf("expected 'get_file.txt', got %q", suffix)
	}
}

func TestHistoryLoadingIsBoundedAndKeepsRecentUniqueCommands(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("APPDATA", home)
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
