package render

import (
	"sync"
)

// AtlasGlyph contains spatial and UV coordinates for a single glyph inside the texture atlas.
type AtlasGlyph struct {
	Rune     rune
	Bold     bool
	PixelX   int
	PixelY   int
	Width    int
	Height   int
	U1, V1   float32
	U2, V2   float32
}

// GlyphAtlas manages a single-channel alpha texture buffer (VRAM texture sheet representation)
// and handles packing glyph masks onto shelves.
type GlyphAtlas struct {
	mu          sync.RWMutex
	width       int
	height      int
	invWidth    float32
	invHeight   float32
	pixels      []byte

	currentX    int
	currentY    int
	shelfHeight int

	asciiCache  [96][2]*AtlasGlyph
	otherCache  map[uint64]*AtlasGlyph

	dirty       bool
	dirtyMinX   int
	dirtyMinY   int
	dirtyMaxX   int
	dirtyMaxY   int
}

// NewGlyphAtlas initializes a new texture atlas with the given dimensions.
// Dimensions are typically power-of-two (e.g. 1024x1024 or 2048x2048).
func NewGlyphAtlas(width, height int) *GlyphAtlas {
	if width <= 0 {
		width = 1024
	}
	if height <= 0 {
		height = 1024
	}
	a := &GlyphAtlas{
		width:      width,
		height:     height,
		invWidth:   1.0 / float32(width),
		invHeight:  1.0 / float32(height),
		pixels:     make([]byte, width*height),
		otherCache: make(map[uint64]*AtlasGlyph),
	}
	a.Reset()
	return a
}

// Reset clears all cached glyphs and resets shelf allocation positions.
func (a *GlyphAtlas) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i := range a.pixels {
		a.pixels[i] = 0
	}
	a.currentX = 0
	a.currentY = 0
	a.shelfHeight = 0
	a.asciiCache = [96][2]*AtlasGlyph{}
	a.otherCache = make(map[uint64]*AtlasGlyph)

	a.dirty = true
	a.dirtyMinX = 0
	a.dirtyMinY = 0
	a.dirtyMaxX = a.width
	a.dirtyMaxY = a.height
}

// GetOrInsert retrieves a glyph from the atlas or rasterizes and packs it if not present.
func (a *GlyphAtlas) GetOrInsert(fe *FontEngine, r rune, bold bool) *AtlasGlyph {
	boldIdx := 0
	if bold {
		boldIdx = 1
	}

	if r >= 32 && r <= 127 {
		a.mu.RLock()
		g := a.asciiCache[r-32][boldIdx]
		a.mu.RUnlock()
		if g != nil {
			return g
		}
	} else {
		key := (uint64(r) << 1) | uint64(boldIdx)
		a.mu.RLock()
		g := a.otherCache[key]
		a.mu.RUnlock()
		if g != nil {
			return g
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if r >= 32 && r <= 127 {
		if g := a.asciiCache[r-32][boldIdx]; g != nil {
			return g
		}
	} else {
		key := (uint64(r) << 1) | uint64(boldIdx)
		if g := a.otherCache[key]; g != nil {
			return g
		}
	}

	mask := fe.GetGlyph(r, bold)
	gw := fe.CharWidth()
	gh := fe.CharHeight()
	var alpha []byte

	if mask != nil && mask.Width > 0 && mask.Height > 0 {
		gw = mask.Width
		gh = mask.Height
		alpha = mask.Alpha
	}

	allocW := gw + 2
	allocH := gh + 2

	if a.currentX+allocW > a.width {
		a.currentY += a.shelfHeight
		a.currentX = 0
		a.shelfHeight = 0
	}

	if a.currentY+allocH > a.height {
		for i := range a.pixels {
			a.pixels[i] = 0
		}
		a.currentX = 0
		a.currentY = 0
		a.shelfHeight = 0
		a.asciiCache = [96][2]*AtlasGlyph{}
		a.otherCache = make(map[uint64]*AtlasGlyph)
	}

	if allocH > a.shelfHeight {
		a.shelfHeight = allocH
	}

	gx := a.currentX + 1
	gy := a.currentY + 1
	a.currentX += allocW

	if len(alpha) > 0 {
		for row := 0; row < gh; row++ {
			srcStart := row * gw
			srcEnd := srcStart + gw
			if srcEnd > len(alpha) {
				srcEnd = len(alpha)
			}
			dstStart := (gy+row)*a.width + gx
			dstEnd := dstStart + (srcEnd - srcStart)
			if dstEnd <= len(a.pixels) {
				copy(a.pixels[dstStart:dstEnd], alpha[srcStart:srcEnd])
			}
		}
	}

	u1 := float32(gx) * a.invWidth
	v1 := float32(gy) * a.invHeight
	u2 := float32(gx+gw) * a.invWidth
	v2 := float32(gy+gh) * a.invHeight

	glyph := &AtlasGlyph{
		Rune:   r,
		Bold:   bold,
		PixelX: gx,
		PixelY: gy,
		Width:  gw,
		Height: gh,
		U1:     u1,
		V1:     v1,
		U2:     u2,
		V2:     v2,
	}

	if r >= 32 && r <= 127 {
		a.asciiCache[r-32][boldIdx] = glyph
	} else {
		key := (uint64(r) << 1) | uint64(boldIdx)
		a.otherCache[key] = glyph
	}

	if !a.dirty {
		a.dirty = true
		a.dirtyMinX = gx
		a.dirtyMinY = gy
		a.dirtyMaxX = gx + gw
		a.dirtyMaxY = gy + gh
	} else {
		if gx < a.dirtyMinX {
			a.dirtyMinX = gx
		}
		if gy < a.dirtyMinY {
			a.dirtyMinY = gy
		}
		if gx+gw > a.dirtyMaxX {
			a.dirtyMaxX = gx + gw
		}
		if gy+gh > a.dirtyMaxY {
			a.dirtyMaxY = gy + gh
		}
	}

	return glyph
}

// PreloadASCII pre-populates all printable ASCII characters (32 to 126) for both normal and bold styles.
func (a *GlyphAtlas) PreloadASCII(fe *FontEngine) {
	for r := rune(32); r <= 126; r++ {
		a.GetOrInsert(fe, r, false)
		a.GetOrInsert(fe, r, true)
	}
}

// Pixels returns the underlying alpha buffer for GPU upload (e.g. via glTexSubImage2D).
func (a *GlyphAtlas) Pixels() []byte {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.pixels
}

// Dimensions returns the width and height of the texture atlas.
func (a *GlyphAtlas) Dimensions() (int, int) {
	return a.width, a.height
}

// IsDirty reports whether the atlas has updated pixels that need to be uploaded to GPU VRAM.
func (a *GlyphAtlas) IsDirty() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.dirty
}

// DirtyBounds returns the bounding rectangle of modified pixels since the last MarkClean call.
func (a *GlyphAtlas) DirtyBounds() (minX, minY, maxX, maxY int) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.dirtyMinX, a.dirtyMinY, a.dirtyMaxX, a.dirtyMaxY
}

// MarkClean resets the dirty region after GPU texture sync.
func (a *GlyphAtlas) MarkClean() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.dirty = false
	a.dirtyMinX = 0
	a.dirtyMinY = 0
	a.dirtyMaxX = 0
	a.dirtyMaxY = 0
}
