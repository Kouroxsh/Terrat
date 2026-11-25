package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDefault(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Shell != "" {
		t.Fatalf("expected default Shell to be empty, got %s", cfg.Shell)
	}
	if cfg.Theme != "auto" {
		t.Fatalf("expected default theme to be 'auto', got %s", cfg.Theme)
	}
	if cfg.FontSize != 13.0 {
		t.Fatalf("expected default font size 13.0, got %f", cfg.FontSize)
	}
	if cfg.Opacity != 0.95 {
		t.Fatalf("expected default opacity 0.95, got %f", cfg.Opacity)
	}
	if !cfg.GhostText {
		t.Fatalf("expected default GhostText to be true")
	}
	if !cfg.SanitizePaste {
		t.Fatalf("expected default SanitizePaste to be true")
	}
	if !cfg.BracketedPaste {
		t.Fatalf("expected default BracketedPaste to be true")
	}
}

func TestConfigSaveAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "terrat-config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	origConfigHome := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", origConfigHome)
	os.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfgPath, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(cfgPath) != filepath.Join(tmpDir, "terrat") {
		t.Fatalf("unexpected config path: %s", cfgPath)
	}

	loaded := Load()
	if loaded.Theme != "auto" || loaded.FontSize != 13.0 || loaded.Opacity != 0.95 || !loaded.GhostText || !loaded.Diagnostics {
		t.Fatalf("expected loaded default values, got theme=%s, size=%f, opacity=%f, ghost_text=%v, diagnostics=%v", loaded.Theme, loaded.FontSize, loaded.Opacity, loaded.GhostText, loaded.Diagnostics)
	}

	loaded.Theme = "catppuccin-mocha"
	loaded.FontSize = 15.5
	loaded.Opacity = 0.90
	loaded.GhostText = false
	loaded.Diagnostics = false
	if err := Save(loaded); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	reloaded := Load()
	if reloaded.Theme != "catppuccin-mocha" {
		t.Fatalf("expected reloaded theme to be 'catppuccin-mocha', got %s", reloaded.Theme)
	}
	if reloaded.FontSize != 15.5 {
		t.Fatalf("expected reloaded font size to be 15.5, got %f", reloaded.FontSize)
	}
	if reloaded.Opacity != 0.90 {
		t.Fatalf("expected reloaded opacity to be 0.90, got %f", reloaded.Opacity)
	}
	if reloaded.GhostText != false {
		t.Fatalf("expected reloaded GhostText to be false, got %v", reloaded.GhostText)
	}
	if reloaded.Diagnostics != false {
		t.Fatalf("expected reloaded Diagnostics to be false, got %v", reloaded.Diagnostics)
	}
}

func TestConfigShellAndDefaultShell(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "terrat-config-shell-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	origConfigHome := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", origConfigHome)
	os.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfgPath, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		t.Fatal(err)
	}

	shellData := []byte(`{"shell": "/usr/bin/bash"}`)
	if err := os.WriteFile(cfgPath, shellData, 0644); err != nil {
		t.Fatal(err)
	}
	cfg := Load()
	if cfg.Shell != "/usr/bin/bash" {
		t.Fatalf("expected Shell to be '/usr/bin/bash', got %q", cfg.Shell)
	}

	defaultShellData := []byte(`{"default_shell": "bash"}`)
	if err := os.WriteFile(cfgPath, defaultShellData, 0644); err != nil {
		t.Fatal(err)
	}
	cfg2 := Load()
	if cfg2.Shell != "bash" {
		t.Fatalf("expected Shell to be 'bash' when default_shell is provided, got %q", cfg2.Shell)
	}
}

func TestPortableConfigTakesPrecedenceOverXDG(t *testing.T) {
	tmpDir := t.TempDir()
	executable := filepath.Join(tmpDir, "terrat")
	portablePath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(portablePath, []byte(`{"theme":"tokyo-night"}`), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := configPath(executable, filepath.Join(tmpDir, "xdg"), filepath.Join(tmpDir, "user-config"))
	if err != nil {
		t.Fatal(err)
	}
	if got != portablePath {
		t.Fatalf("expected portable config %q to take precedence, got %q", portablePath, got)
	}
}

func TestLoadKeepsValidFieldsOnBadField(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "terrat-config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	origConfigHome := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", origConfigHome)
	os.Setenv("XDG_CONFIG_HOME", tmpDir)

	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	bad := `{"theme": "catppuccin-mocha", "font_size": "13", "opacity": 0.9}`
	if err := os.WriteFile(path, []byte(bad), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := Load()
	if cfg.Theme != "catppuccin-mocha" {
		t.Fatalf("expected valid theme kept, got %q", cfg.Theme)
	}
	if cfg.FontSize != 13.0 {
		t.Fatalf("expected broken font_size to fall back to 13.0, got %f", cfg.FontSize)
	}
	if cfg.Opacity != 0.9 {
		t.Fatalf("expected valid opacity kept, got %f", cfg.Opacity)
	}
}

func TestLoadCorruptConfigBackedUp(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "terrat-config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	origConfigHome := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", origConfigHome)
	os.Setenv("XDG_CONFIG_HOME", tmpDir)

	path, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json at all"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := Load()
	if cfg.Theme != "auto" {
		t.Fatalf("expected defaults for corrupt config, got theme %q", cfg.Theme)
	}
	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("expected corrupt config backed up to %s.bak: %v", path, err)
	}
	if string(bak) != "{not json at all" {
		t.Fatalf("unexpected backup content: %q", string(bak))
	}
}

func TestSaveAtomicNoTempLeftBehind(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "terrat-config-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	origConfigHome := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", origConfigHome)
	os.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := Load()
	cfg.Theme = "minecraft"
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cfg.path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("temp file left behind after Save")
	}
	data, err := os.ReadFile(cfg.path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("saved config is not valid JSON: %v", err)
	}
	if decoded["theme"] != "minecraft" {
		t.Fatalf("expected saved theme minecraft, got %v", decoded["theme"])
	}
}
