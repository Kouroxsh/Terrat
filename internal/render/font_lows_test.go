package render

import (
	"sync"
	"testing"
)

func TestZoomLimitsMatchSetFontSizeRange(t *testing.T) {
	fe, err := NewFontEngine(13)
	if err != nil {
		t.Fatal(err)
	}
	defer fe.Close()

	for i := 0; i < 20; i++ {
		if !fe.ZoomOut() {
			break
		}
	}
	if got := fe.FontSize(); got != 7 {
		t.Fatalf("ZoomOut floor = %v, want 7", got)
	}
	if fe.ZoomOut() {
		t.Fatalf("ZoomOut past 7 should return false")
	}

	for i := 0; i < 40; i++ {
		if !fe.ZoomIn() {
			break
		}
	}
	if got := fe.FontSize(); got != 36 {
		t.Fatalf("ZoomIn cap = %v, want 36", got)
	}
	if fe.ZoomIn() {
		t.Fatalf("ZoomIn past 36 should return false")
	}
}

func TestCacheEvictionIsFractional(t *testing.T) {
	fe, err := NewFontEngine(13)
	if err != nil {
		t.Fatal(err)
	}
	defer fe.Close()

	fe.mu.Lock()
	for i := 0; i < 4096; i++ {
		fe.otherCache[uint64(i)] = &GlyphMask{Width: 1, Height: 1, Alpha: []byte{0}}
	}
	fe.mu.Unlock()

	if fe.GetGlyph('€', false) == nil {
		t.Fatal("GetGlyph returned nil")
	}
	fe.mu.RLock()
	n := len(fe.otherCache)
	fe.mu.RUnlock()
	if n < 3000 || n > 3100 {
		t.Fatalf("cache size after eviction = %d, want ~3073 (fractional eviction, not a full clear)", n)
	}
}

func TestDrawGlyphBlitOriginX(t *testing.T) {
	mask := &GlyphMask{
		Width:   10,
		Height:  2,
		OriginX: 2,
		Alpha: []byte{
			255, 0, 0, 0, 255, 0, 0, 0, 0, 0,
			0, 0, 0, 0, 0, 255, 0, 0, 0, 0,
		},
	}
	const stride = 40 * 4
	buf := make([]byte, stride*2)

	for i := range buf {
		buf[i] = 0x77
	}
	fg, bg := uint32(0xffffff), uint32(0x000000)
	DrawGlyphBlit(buf, stride, 20, 0, mask, fg, bg, true, 0)

	if got := buf[18*4]; got != 0xff {
		t.Fatalf("overhang ink at x=18 not painted (byte=%d)", got)
	}

	if got := buf[19*4]; got != 0x77 {
		t.Fatalf("transparent overhang at x=19 painted bg (byte=%d)", got)
	}

	if got := buf[22*4]; got != 0xff {
		t.Fatalf("in-cell ink at x=22 not painted (byte=%d)", got)
	}

	if got := buf[20*4]; got != 0x00 {
		t.Fatalf("in-cell background at x=20 not painted (byte=%d)", got)
	}

	for x := 20; x < 28; x++ {
		if got := buf[stride+x*4]; got != 0xff {
			t.Fatalf("underline at x=%d not painted (byte=%d)", x, got)
		}
	}
	if got := buf[stride+28*4]; got != 0x77 {
		t.Fatalf("underline overran the cell at x=28 (byte=%d)", got)
	}
}

func TestDrawGlyphBlitTransparentOriginX(t *testing.T) {
	mask := &GlyphMask{
		Width:   10,
		Height:  1,
		OriginX: 2,
		Alpha:   []byte{255, 0, 0, 0, 255, 0, 0, 0, 0, 0},
	}
	const stride = 40 * 4
	buf := make([]byte, stride)
	for i := range buf {
		buf[i] = 0x77
	}
	DrawGlyphBlitTransparent(buf, stride, 20, 0, mask, 0xffffff, false, 0)
	if got := buf[18*4]; got != 0xff {
		t.Fatalf("transparent blit overhang at x=18 not painted (byte=%d)", got)
	}
	if got := buf[22*4]; got != 0xff {
		t.Fatalf("transparent blit in-cell ink at x=22 not painted (byte=%d)", got)
	}
	if got := buf[19*4]; got != 0x77 {
		t.Fatalf("transparent blit touched x=19 (byte=%d)", got)
	}
}

func TestConcurrentGlyphAndResize(t *testing.T) {
	fe, err := NewFontEngine(13)
	if err != nil {
		t.Fatal(err)
	}
	defer fe.Close()

	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				fe.GetGlyph(rune('a'+g+i%20), i%2 == 0)
				fe.GetWideGlyph('世', false)
			}
		}(g)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_ = fe.SetFontSize(13 + float64(i%5))
		}
	}()
	wg.Wait()
}
