package platform

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDetectSystemColorScheme(t *testing.T) {
	isDark, known := DetectSystemColorScheme()
	t.Logf("Detected system dark mode: %v (known=%v)", isDark, known)
}

// M30: hanging probes (e.g. a broken D-Bus) must not hang startup — the
// whole detection is bounded by probeTimeout.
func TestDetectSystemColorSchemeTimeout(t *testing.T) {
	binDir := t.TempDir()
	// exec replaces the shell with sleep so the context kill hits sleep
	// directly (a bare `sleep` would also need PATH to resolve it).
	hang := "#!/bin/sh\nexec /usr/bin/sleep 30\n"
	for _, name := range []string{"dbus-send", "gsettings", "kreadconfig6", "kreadconfig5"} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(hang), 0755); err != nil {
			t.Fatal(err)
		}
	}
	home := t.TempDir() // no kdeglobals here
	// Fake probes first in PATH so they shadow the real ones.
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	t.Setenv("HOME", home)

	start := time.Now()
	isDark, known := DetectSystemColorScheme()
	elapsed := time.Since(start)
	if elapsed > 5*time.Second {
		t.Fatalf("detection took %v with hanging probes, expected under ~%v", elapsed, probeTimeout)
	}
	if known {
		t.Fatal("expected scheme unknown when every probe hangs")
	}
	if !isDark {
		t.Fatal("expected dark fallback when the scheme is unknown")
	}
}
