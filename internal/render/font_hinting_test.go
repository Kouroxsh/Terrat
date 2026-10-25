package render

import (
	"os"
	"path/filepath"
	"testing"

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

func TestFallbackFontsLoadOnlyWhenNeeded(t *testing.T) {
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
	if fe.fallbacksLoaded || len(fe.fallbacks) != 0 {
		t.Fatal("ASCII startup loaded fallback fonts")
	}
	fe.GetGlyph('\U0010ffff', false)
	if !fe.fallbacksLoaded || len(fe.fallbacks) != 1 {
		t.Fatal("missing glyph did not trigger fallback loading")
	}
	loaded := fe.fallbacks[0].ttf
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := fe.SetFontSize(14); err != nil {
		t.Fatal(err)
	}
	if len(fe.fallbacks) != 1 || fe.fallbacks[0].ttf != loaded {
		t.Fatal("zoom must reuse parsed fallback fonts")
	}
}
