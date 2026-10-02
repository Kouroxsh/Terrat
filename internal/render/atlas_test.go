package render

import (
	"sync"
	"testing"
)

func TestAtlasAllocationAndUV(t *testing.T) {
	fe, err := NewFontEngine(13.0)
	if err != nil {
		t.Fatalf("failed to create FontEngine: %v", err)
	}
	defer fe.Close()

	atlas := NewGlyphAtlas(512, 512)
	w, h := atlas.Dimensions()
	if w != 512 || h != 512 {
		t.Fatalf("unexpected dimensions: %dx%d", w, h)
	}

	g1 := atlas.GetOrInsert(fe, 'A', false)
	if g1 == nil {
		t.Fatalf("expected non-nil AtlasGlyph for 'A'")
	}
	if g1.U1 < 0 || g1.U2 > 1.0 || g1.V1 < 0 || g1.V2 > 1.0 {
		t.Fatalf("invalid UV coordinates: (%f,%f)-(%f,%f)", g1.U1, g1.V1, g1.U2, g1.V2)
	}
	if g1.U1 >= g1.U2 || g1.V1 >= g1.V2 {
		t.Fatalf("expected U1 < U2 and V1 < V2, got (%f,%f)-(%f,%f)", g1.U1, g1.V1, g1.U2, g1.V2)
	}

	if !atlas.IsDirty() {
		t.Fatalf("expected atlas to be dirty after insertion")
	}
	minX, minY, maxX, maxY := atlas.DirtyBounds()
	if minX < 0 || minY < 0 || maxX > 512 || maxY > 512 || minX >= maxX || minY >= maxY {
		t.Fatalf("invalid dirty bounds: %d,%d -> %d,%d", minX, minY, maxX, maxY)
	}

	atlas.MarkClean()
	if atlas.IsDirty() {
		t.Fatalf("expected atlas to be clean after MarkClean")
	}

	// Repeated lookup returns identical pointer
	g2 := atlas.GetOrInsert(fe, 'A', false)
	if g1 != g2 {
		t.Fatalf("expected cached glyph pointer to match")
	}
}

func TestAtlasPreloadASCII(t *testing.T) {
	fe, err := NewFontEngine(13.0)
	if err != nil {
		t.Fatalf("failed to create FontEngine: %v", err)
	}
	defer fe.Close()

	atlas := NewGlyphAtlas(1024, 1024)
	atlas.PreloadASCII(fe)

	for r := rune(32); r <= 126; r++ {
		reg := atlas.GetOrInsert(fe, r, false)
		if reg == nil {
			t.Fatalf("missing preloaded regular glyph for rune '%c' (%d)", r, r)
		}
		bold := atlas.GetOrInsert(fe, r, true)
		if bold == nil {
			t.Fatalf("missing preloaded bold glyph for rune '%c' (%d)", r, r)
		}
	}
}

func TestAtlasShelfWrapping(t *testing.T) {
	fe, err := NewFontEngine(13.0)
	if err != nil {
		t.Fatalf("failed to create FontEngine: %v", err)
	}
	defer fe.Close()

	// Small atlas to force shelf wrapping and reset
	atlas := NewGlyphAtlas(128, 128)
	for r := rune(32); r <= 126; r++ {
		g := atlas.GetOrInsert(fe, r, false)
		if g == nil {
			t.Fatalf("expected non-nil glyph for rune '%c'", r)
		}
	}
}

func TestAtlasConcurrentAccess(t *testing.T) {
	fe, err := NewFontEngine(13.0)
	if err != nil {
		t.Fatalf("failed to create FontEngine: %v", err)
	}
	defer fe.Close()

	atlas := NewGlyphAtlas(1024, 1024)
	atlas.PreloadASCII(fe)

	var wg sync.WaitGroup
	workers := 10
	iterations := 500

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				r := rune(32 + (j % 95))
				bold := (j % 2) == 0
				g := atlas.GetOrInsert(fe, r, bold)
				if g == nil {
					t.Errorf("worker %d got nil glyph", workerID)
					return
				}
				// Also occasionally request non-ASCII glyph
				unicodeRune := rune(0x0600 + (j % 50))
				ug := atlas.GetOrInsert(fe, unicodeRune, bold)
				if ug == nil {
					t.Errorf("worker %d got nil unicode glyph", workerID)
					return
				}
			}
		}(i)
	}
	wg.Wait()
}

func BenchmarkAtlasLookupHotPath(b *testing.B) {
	fe, err := NewFontEngine(13.0)
	if err != nil {
		b.Fatalf("failed to create FontEngine: %v", err)
	}
	defer fe.Close()

	atlas := NewGlyphAtlas(1024, 1024)
	atlas.PreloadASCII(fe)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = atlas.GetOrInsert(fe, 'A', false)
	}
}
