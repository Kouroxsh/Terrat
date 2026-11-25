//go:build linux

package platform

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const probeTimeout = 2 * time.Second

func runProbe(ctx context.Context, name string, args ...string) (string, bool) {
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

func DetectSystemColorScheme() (dark bool, known bool) {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()

	if out, ok := runProbe(ctx, "dbus-send", "--session", "--print-reply=literal",
		"--dest=org.freedesktop.portal.Desktop",
		"/org/freedesktop/portal/desktop",
		"org.freedesktop.portal.Settings.Read",
		"string:org.freedesktop.appearance", "string:color-scheme"); ok {
		s := out
		if strings.Contains(s, "uint32 1") {
			return true, true
		}
		if strings.Contains(s, "uint32 2") {
			return false, true
		}
	}

	if out, ok := runProbe(ctx, "gsettings", "get", "org.gnome.desktop.interface", "color-scheme"); ok {
		s := strings.ToLower(out)
		if strings.Contains(s, "dark") {
			return true, true
		}
		if strings.Contains(s, "light") {
			return false, true
		}
	}

	for _, cmd := range []string{"kreadconfig6", "kreadconfig5"} {
		if out, ok := runProbe(ctx, cmd, "--group", "General", "--key", "ColorScheme"); ok {
			s := strings.ToLower(out)
			if strings.Contains(s, "dark") {
				return true, true
			}
			if strings.Contains(s, "light") || strings.Contains(s, "breeze") {
				return false, true
			}
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		kdeGlobalsPath := filepath.Join(home, ".config", "kdeglobals")
		if data, err := os.ReadFile(kdeGlobalsPath); err == nil {
			content := strings.ToLower(string(data))
			for _, line := range strings.Split(content, "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "colorscheme=") {
					if strings.Contains(line, "dark") {
						return true, true
					}
					if strings.Contains(line, "light") {
						return false, true
					}
				}
			}
		}
	}

	return true, false
}
