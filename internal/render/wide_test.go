package render

import (
	"testing"
	"time"
)

func TestGetWideGlyphDoubleWidth(t *testing.T) {
	fe, err := NewFontEngine(13.0)
	if err != nil {
		t.Fatalf("failed to initialize font engine: %v", err)
	}
	defer fe.Close()

	_ = fe.GetWideGlyph('世', false)
	deadline := time.Now().Add(10 * time.Second)
	for {
		fe.fbMu.RLock()
		loaded := fe.fallbacksLoaded
		fe.fbMu.RUnlock()
		if loaded {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("background fallback load did not complete")
		}
		time.Sleep(5 * time.Millisecond)
	}

	for _, r := range []rune{'世', '界', '😀'} {
		mask := fe.GetWideGlyph(r, false)
		if mask == nil {
			t.Fatalf("GetWideGlyph(%U) returned nil", r)
		}
		if mask.Width != 2*fe.CharWidth() {
			t.Fatalf("GetWideGlyph(%U).Width = %d, want %d", r, mask.Width, 2*fe.CharWidth())
		}
		if mask.Height != fe.CharHeight() {
			t.Fatalf("GetWideGlyph(%U).Height = %d, want %d", r, mask.Height, fe.CharHeight())
		}

		if again := fe.GetWideGlyph(r, false); again != mask {
			t.Fatalf("GetWideGlyph(%U) not cached", r)
		}
	}

	if mask := fe.GetGlyph('a', false); mask.Width != fe.CharWidth() {
		t.Fatalf("GetGlyph('a').Width = %d, want %d", mask.Width, fe.CharWidth())
	}
}
