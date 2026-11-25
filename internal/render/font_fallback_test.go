package render

import (
	"os"
	"testing"
	"time"
)

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
