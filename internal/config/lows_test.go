package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Kouroxsh/Terrat/internal/terminal"
)

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

	if err := os.WriteFile(path, []byte(`{"theme": "minecraft"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if cfg := Load(); cfg.Theme != "minecraft" {
		t.Fatalf("expected known theme to be kept, got %q", cfg.Theme)
	}
}

func TestRelativeXDGConfigHomeIgnored(t *testing.T) {
	path, err := configPath("", "/abs/xdg", "/home/u/.config")
	if err != nil || path != "/abs/xdg/terrat/config.json" {
		t.Fatalf("absolute XDG_CONFIG_HOME: path = %q, err = %v", path, err)
	}

	path, err = configPath("", "relative/xdg", "/home/u/.config")
	if err != nil || path != "/home/u/.config/terrat/config.json" {
		t.Fatalf("relative XDG_CONFIG_HOME: path = %q, err = %v", path, err)
	}
	path, err = configPath("", "./xdg", "/home/u/.config")
	if err != nil || path != "/home/u/.config/terrat/config.json" {
		t.Fatalf("'./xdg' XDG_CONFIG_HOME: path = %q, err = %v", path, err)
	}
}

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
