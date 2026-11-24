package render

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/image/font/gofont/gomono"
)

func TestFontEngineCellBoundsAfterZoom(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mono.ttf")
	if err := os.WriteFile(path, gomono.TTF, 0600); err != nil {
		t.Fatal(err)
	}
	previous := candidateTTFFonts
	candidateTTFFonts = []string{path}
	t.Cleanup(func() { candidateTTFFonts = previous })
	fe, err := NewFontEngine(13)
	if err != nil {
		t.Fatal(err)
	}
	defer fe.Close()
	for _, size := range []float64{7, 8, 13, 14, 18, 32, 36, 13} {
		if err := fe.SetFontSize(size); err != nil {
			t.Fatal(err)
		}
		if fe.ttf == nil {
			t.Fatal("expected TrueType font")
		}
		for _, r := range "Hgjpqy_" {
			bounds, _, ok := fe.face.GlyphBounds(r)
			if !ok {
				t.Fatalf("missing glyph %q", r)
			}
			if bounds.Min.Y.Floor()+fe.baseline < 0 || bounds.Max.Y.Ceil()+fe.baseline > fe.charHeight {
				t.Fatalf("size %.0f: glyph %q clipped: %v, baseline %d, height %d", size, r, bounds, fe.baseline, fe.charHeight)
			}
			mask := fe.GetGlyph(r, false)
			if mask.Width != fe.CharWidth() || mask.Height != fe.CharHeight() {
				t.Fatalf("size %.0f: stale glyph dimensions for %q", size, r)
			}
			visible := false
			for _, alpha := range mask.Alpha {
				if alpha > 0 {
					visible = true
					break
				}
			}
			if !visible {
				t.Fatalf("size %.0f: empty glyph %q", size, r)
			}
		}
	}
}

// M28: fallback fonts load once in the background, never lazily under
// the terminal read-lock — triggered by the first non-ASCII glyph the
// main face lacks, so engine creation itself stays cheap. M29: a rune
// no font covers renders as a visible .notdef box, not a blank.
func TestFallbackFontsLoadInBackground(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mono.ttf")
	if err := os.WriteFile(path, gomono.TTF, 0600); err != nil {
		t.Fatal(err)
	}
	primary, fallbacks := candidateTTFFonts, candidateFallbackFonts
	candidateTTFFonts = []string{path}
	candidateFallbackFonts = []string{path}
	t.Cleanup(func() { candidateTTFFonts = primary; candidateFallbackFonts = fallbacks })
	fe, err := NewFontEngine(13)
	if err != nil {
		t.Fatal(err)
	}
	defer fe.Close()
	// Kick off the lazy load with a rune no font covers; the load runs
	// in the background, so this call returns immediately.
	_ = fe.GetGlyph('\U0010ffff', false)
	// Wait for the background load to finish (it reads one tiny test font).
	deadline := time.Now().Add(5 * time.Second)
	for {
		fe.fbMu.RLock()
		loaded, n := fe.fallbacksLoaded, len(fe.fallbacks)
		fe.fbMu.RUnlock()
		if loaded && n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("background fallback load did not complete")
		}
		time.Sleep(5 * time.Millisecond)
	}
	// A rune no loaded font covers must render as a visible box, not blank.
	mask := fe.GetGlyph('\U0010ffff', false)
	visible := false
	for _, a := range mask.Alpha {
		if a != 0 {
			visible = true
			break
		}
	}
	if !visible {
		t.Fatal("missing glyph rendered blank instead of a .notdef box")
	}
	loaded := fe.fallbacks[0].sfnt
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := fe.SetFontSize(14); err != nil {
		t.Fatal(err)
	}
	if len(fe.fallbacks) != 1 || fe.fallbacks[0].sfnt != loaded {
		t.Fatal("zoom must reuse parsed fallback fonts")
	}
}
