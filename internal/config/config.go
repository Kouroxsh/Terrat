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

	// path is the file this config was loaded from (M40). Load resolves it
	// once and Save reuses it, so the location cannot change between Load
	// and Save (e.g. if a portable config.json appears next to the binary
	// mid-session). Unexported, so it is never serialised. Empty when the
	// config was built by DefaultConfig; Save then resolves the path fresh.
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
	// A config beside the executable marks a portable installation and takes
	// precedence over the per-user location, including XDG_CONFIG_HOME.
	// M40: the portable file must be a regular, readable file — a directory
	// or unreadable file named config.json used to win and make Load silently
	// fall back to defaults without ever trying the XDG location.
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
		// L37: a relative XDG_CONFIG_HOME is invalid per the XDG spec
		// (and Go's os.UserConfigDir already rejects it). Accepting it
		// made Terrat read and write its config relative to whatever
		// directory it was launched from. Ignore it and fall through
		// to the user config dir instead.
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

// backupCorruptConfig moves a broken config file aside so the user's
// settings are not silently destroyed (M21).
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
		// L38: Load used to write a fresh config file here (and swallow
		// the error). A load must not create files: the file appears on
		// the first real settings change, when Save is called. A
		// non-NotExist error is still worth reporting — the old code
		// silently overwrote even an unreadable config with defaults.
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "terrat: cannot read config %s: %v; using defaults\n", path, err)
		}
		return cfg
	}

	// M21: parse leniently, field by field. A single mistyped field (e.g.
	// "font_size": "13" as a string) used to fail the whole unmarshal and
	// silently discard the theme, shell and every toggle. Now the valid
	// fields are kept and only the broken ones fall back to defaults.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		// Not even a JSON object: back the file up and start fresh.
		// (L38: no Save here either — see above.)
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
	// L36: an unknown theme name used to survive in the config while
	// ResolveTheme silently applied Tokyo Night/Day — the preferences
	// modal then showed a theme that was not in effect. Normalise it
	// to "auto" (and say so) so the stored value always matches what
	// is actually applied.
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
	// M40: reuse the path Load resolved, so Save cannot land in a different
	// location than Load read from.
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

	// M20: atomic write — a crash mid-write used to corrupt config.json
	// and silently reset every setting. Write a temp file in the same
	// directory, fsync it, then rename over the real file.
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
