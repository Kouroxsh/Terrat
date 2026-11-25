package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Kouroxsh/Terrat/internal/terminal"
)

type Config struct {
	Shell          string  `json:"shell"`
	DefaultShell   string  `json:"default_shell,omitempty"`
	Theme          string  `json:"theme"`
	FontSize       float64 `json:"font_size"`
	Opacity        float64 `json:"opacity"`
	GhostText      bool    `json:"ghost_text"`
	Diagnostics    bool    `json:"diagnostics"`
	SanitizePaste  bool    `json:"sanitize_paste"`
	BracketedPaste bool    `json:"bracketed_paste"`

	path string
}

func DefaultConfig() *Config {
	return &Config{
		Shell:          "",
		Theme:          "auto",
		FontSize:       13.0,
		Opacity:        0.95,
		GhostText:      true,
		Diagnostics:    true,
		SanitizePaste:  true,
		BracketedPaste: true,
	}
}

func ConfigPath() (string, error) {
	exe, _ := os.Executable()
	configDir, _ := os.UserConfigDir()
	return configPath(exe, os.Getenv("XDG_CONFIG_HOME"), configDir)
}

func configPath(executable, xdgConfigHome, userConfigDir string) (string, error) {
	if executable != "" {
		portableCfg := filepath.Join(filepath.Dir(executable), "config.json")
		if fi, err := os.Stat(portableCfg); err == nil && !fi.IsDir() {
			if f, err := os.Open(portableCfg); err == nil {
				_ = f.Close()
				return portableCfg, nil
			}
		}
	}
	if xdgConfigHome != "" {
		if filepath.IsAbs(xdgConfigHome) {
			return filepath.Join(xdgConfigHome, "terrat", "config.json"), nil
		}
		fmt.Fprintf(os.Stderr, "terrat: ignoring relative XDG_CONFIG_HOME %q (must be absolute)\n", xdgConfigHome)
	}
	if userConfigDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		userConfigDir = filepath.Join(home, ".config")
	}
	return filepath.Join(userConfigDir, "terrat", "config.json"), nil
}

func backupCorruptConfig(path string) {
	bak := path + ".bak"
	if err := os.Rename(path, bak); err != nil {
		fmt.Fprintf(os.Stderr, "terrat: config %s is corrupt and could not be backed up: %v\n", path, err)
		return
	}
	fmt.Fprintf(os.Stderr, "terrat: config %s is corrupt; backed up to %s, using defaults\n", path, bak)
}

func Load() *Config {
	path, err := ConfigPath()
	if err != nil {
		return DefaultConfig()
	}

	cfg := DefaultConfig()
	cfg.path = path

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "terrat: cannot read config %s: %v; using defaults\n", path, err)
		}
		return cfg
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		backupCorruptConfig(path)
		return cfg
	}
	var str string
	var num float64
	var boolean bool
	if v, ok := raw["shell"]; ok {
		if json.Unmarshal(v, &str) == nil {
			cfg.Shell = str
		}
	}
	if v, ok := raw["default_shell"]; ok {
		if json.Unmarshal(v, &str) == nil {
			cfg.DefaultShell = str
		}
	}
	if v, ok := raw["theme"]; ok {
		if json.Unmarshal(v, &str) == nil {
			cfg.Theme = str
		}
	}
	if v, ok := raw["font_size"]; ok {
		if json.Unmarshal(v, &num) == nil {
			cfg.FontSize = num
		}
	}
	if v, ok := raw["opacity"]; ok {
		if json.Unmarshal(v, &num) == nil {
			cfg.Opacity = num
		}
	}
	if v, ok := raw["ghost_text"]; ok {
		if json.Unmarshal(v, &boolean) == nil {
			cfg.GhostText = boolean
		}
	}
	if v, ok := raw["diagnostics"]; ok {
		if json.Unmarshal(v, &boolean) == nil {
			cfg.Diagnostics = boolean
		}
	}
	if v, ok := raw["sanitize_paste"]; ok {
		if json.Unmarshal(v, &boolean) == nil {
			cfg.SanitizePaste = boolean
		}
	}
	if v, ok := raw["bracketed_paste"]; ok {
		if json.Unmarshal(v, &boolean) == nil {
			cfg.BracketedPaste = boolean
		}
	}

	if cfg.Shell == "" && cfg.DefaultShell != "" {
		cfg.Shell = cfg.DefaultShell
	}
	cfg.DefaultShell = ""

	if cfg.Theme == "" {
		cfg.Theme = "auto"
	}

	if !terminal.IsValidThemePreference(cfg.Theme) {
		fmt.Fprintf(os.Stderr, "terrat: unknown theme %q, falling back to \"auto\"\n", cfg.Theme)
		cfg.Theme = "auto"
	}
	if cfg.FontSize < 7.0 || cfg.FontSize > 36.0 {
		cfg.FontSize = 13.0
	}
	if cfg.Opacity < 0.2 || cfg.Opacity > 1.0 {
		cfg.Opacity = 0.95
	}

	return cfg
}

func Save(cfg *Config) error {
	path := cfg.path
	if path == "" {
		var err error
		path, err = ConfigPath()
		if err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}
