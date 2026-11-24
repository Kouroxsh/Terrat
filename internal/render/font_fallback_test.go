package render

import (
	"os"
	"testing"
	"time"
)

// M8: the Noto CJK .ttc must actually load as a fallback. Its fonts are
// CFF-flavoured (OTTO), which freetype's truetype.Parse rejects outright,
// so the old loader silently skipped the only CJK fallback font.
func TestCJKTTCLoadsAsFallback(t *testing.T) {
	ttc := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	if _, err := os.Stat(ttc); err != nil {
		t.Skip("no CJK TTC font on this system")
	}
	previous := candidateFallbackFonts
	candidateFallbackFonts = []string{ttc}
	t.Cleanup(func() { candidateFallbackFonts = previous })

	fe, err := NewFontEngine(13)
	if err != nil {
		t.Fatal(err)
	}
	defer fe.Close()

	// Kick off the lazy load with a CJK rune the main face lacks.
	_ = fe.GetWideGlyph('世', false)

	deadline := time.Now().Add(10 * time.Second)
	for {
		fe.fbMu.RLock()
		loaded, n := fe.fallbacksLoaded, len(fe.fallbacks)
		fe.fbMu.RUnlock()
		if loaded {
			if n != 1 {
				t.Fatalf("expected 1 fallback font from the TTC, got %d", n)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("background fallback load did not complete")
		}
		time.Sleep(5 * time.Millisecond)
	}

	fb := fe.fallbackSnapshot()[0]
	if !fb.has('世') {
		t.Fatal("CJK fallback does not cover '世' (U+4E16)")
	}
	if !fb.has('あ') {
		t.Fatal("CJK fallback does not cover 'あ' (U+3042)")
	}
	// And the wide glyph for it must actually paint, at double width.
	mask := fe.GetWideGlyph('世', false)
	if mask.Width != 2*fe.CharWidth() {
		t.Fatalf("wide glyph width = %d, want %d", mask.Width, 2*fe.CharWidth())
	}
	visible := false
	for _, a := range mask.Alpha {
		if a != 0 {
			visible = true
			break
		}
	}
	if !visible {
		t.Fatal("CJK wide glyph rendered blank")
	}
}

// M15: out-of-range font sizes are clamped to 7–36pt at construction.
func TestNewFontEngineClampsSize(t *testing.T) {
	fe, err := NewFontEngine(1000)
	if err != nil {
		t.Fatal(err)
	}
	defer fe.Close()
	if got := fe.FontSize(); got != 36.0 {
		t.Fatalf("expected font size clamped to 36, got %v", got)
	}
	fe2, err := NewFontEngine(1)
	if err != nil {
		t.Fatal(err)
	}
	defer fe2.Close()
	if got := fe2.FontSize(); got != 7.0 {
		t.Fatalf("expected font size clamped to 7, got %v", got)
	}
}
