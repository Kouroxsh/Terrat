package render

import (
	"testing"
	"time"
)

// TestGetWideGlyphDoubleWidth verifies a wide rune's glyph mask spans two
// cells, so the renderer can paint CJK/emoji across the lead cell and its
// continuation instead of clipping them to one cell.
func TestGetWideGlyphDoubleWidth(t *testing.T) {
	fe, err := NewFontEngine(13.0)
	if err != nil {
		t.Fatalf("failed to initialize font engine: %v", err)
	}
	defer fe.Close()

	// The fallback load is lazy: the first wide glyph only kicks it off,
	// and a notdef rendered while it is still loading is deliberately
	// not cached. Wait for the load to finish before asserting caching.
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
		// The wide mask is cached: same pointer on repeat calls.
		if again := fe.GetWideGlyph(r, false); again != mask {
			t.Fatalf("GetWideGlyph(%U) not cached", r)
		}
	}

	// A normal glyph stays one cell wide.
	if mask := fe.GetGlyph('a', false); mask.Width != fe.CharWidth() {
		t.Fatalf("GetGlyph('a').Width = %d, want %d", mask.Width, fe.CharWidth())
	}
}
