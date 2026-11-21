package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Kouroxsh/Terrat/internal/terminal"
)

// L36: an unknown theme name must not survive in the config while a
// different theme is applied.
func TestUnknownThemeFallsBackToAuto(t *testing.T) {
	for _, pref := range []string{"auto", "tokyo-night", "catppuccin-mocha", "minecraft", "tokyo-day", "solarized-light"} {
		if !terminal.IsValidThemePreference(pref) {
			t.Fatalf("expected %q to be a valid theme preference", pref)
		}
	}
	if terminal.IsValidThemePreference("dracula") {
		t.Fatalf("expected 'dracula' to be an invalid theme preference")
	}

	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"theme": "dracula"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if cfg := Load(); cfg.Theme != "auto" {
		t.Fatalf("expected unknown theme to fall back to 'auto', got %q", cfg.Theme)
	}

	// A known theme is kept as-is.
	if err := os.WriteFile(path, []byte(`{"theme": "minecraft"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if cfg := Load(); cfg.Theme != "minecraft" {
		t.Fatalf("expected known theme to be kept, got %q", cfg.Theme)
	}
}

// L37: a relative XDG_CONFIG_HOME must be ignored, not resolved
// against the current directory.
func TestRelativeXDGConfigHomeIgnored(t *testing.T) {
	// Absolute: honoured.
	path, err := configPath("", "/abs/xdg", "/home/u/.config")
	if err != nil || path != "/abs/xdg/terrat/config.json" {
		t.Fatalf("absolute XDG_CONFIG_HOME: path = %q, err = %v", path, err)
	}
	// Relative: falls through to the user config dir.
	path, err = configPath("", "relative/xdg", "/home/u/.config")
	if err != nil || path != "/home/u/.config/terrat/config.json" {
		t.Fatalf("relative XDG_CONFIG_HOME: path = %q, err = %v", path, err)
	}
	path, err = configPath("", "./xdg", "/home/u/.config")
	if err != nil || path != "/home/u/.config/terrat/config.json" {
		t.Fatalf("'./xdg' XDG_CONFIG_HOME: path = %q, err = %v", path, err)
	}
}

// L38: Load must not create files as a side effect.
func TestLoadDoesNotCreateConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	Load()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected Load not to create %s", path)
	}

	// Corrupt config: backed up, but not recreated by Load either.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	Load()
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatalf("expected corrupt config to be backed up: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected Load not to recreate %s after backing it up", path)
	}
}
