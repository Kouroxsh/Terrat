package render

import (
	"image"
	"os"
	"sync"
	"unicode"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

type GlyphMask struct {
	Width   int
	Height  int
	OriginX int
	Alpha   []byte
}

type fallbackFont struct {
	face font.Face
	sfnt *sfnt.Font
	has  func(rune) bool
}

type FontEngine struct {
	charWidth  int
	charHeight int
	baseline   int
	face       font.Face
	ttf        *truetype.Font
	fontSize   float64

	mu sync.RWMutex

	fallbacks       []fallbackFont
	fallbacksLoaded bool
	fbStarted       bool

	fbMu sync.RWMutex

	asciiCache [96][2]*GlyphMask
	otherCache map[uint64]*GlyphMask

	wideCache map[uint64]*GlyphMask
}

var candidateTTFFonts = []string{
	"/usr/share/fonts/truetype/hack/Hack-Regular.ttf",
	"/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf",
	"/usr/share/fonts/truetype/liberation/LiberationMono-Regular.ttf",
	"/usr/share/fonts/truetype/ubuntu/UbuntuMono-R.ttf",
	"/usr/share/fonts/truetype/freefont/FreeMono.ttf",
}

var candidateFallbackFonts = []string{
	"/usr/share/fonts/truetype/noto/NotoSansSymbols-Regular.ttf",
	"/usr/share/fonts/truetype/noto/NotoSansSymbols2-Regular.ttf",
	"/usr/share/fonts/truetype/noto/NotoSansMath-Regular.ttf",
	"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf",
	"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
	"/usr/share/fonts/truetype/freefont/FreeSans.ttf",
}

func NewFontEngine(fontSize float64) (*FontEngine, error) {
	if fontSize < 7.0 {
		fontSize = 7.0
	}
	if fontSize > 36.0 {
		fontSize = 36.0
	}
	fe := &FontEngine{
		otherCache: make(map[uint64]*GlyphMask),
		wideCache:  make(map[uint64]*GlyphMask),
		fontSize:   fontSize,
	}

	var loadedFace font.Face
	var loadedTTF *truetype.Font
	for _, path := range candidateTTFFonts {
		if data, err := os.ReadFile(path); err == nil {
			if ttf, err := truetype.Parse(data); err == nil {
				face := truetype.NewFace(ttf, &truetype.Options{
					Size:    fontSize,
					DPI:     72,
					Hinting: font.HintingFull,
				})
				loadedFace = face
				loadedTTF = ttf
				break
			}
		}
	}

	if loadedFace != nil {
		fe.face = loadedFace
		fe.ttf = loadedTTF
		metrics := loadedFace.Metrics()

		fe.charHeight = metrics.Ascent.Ceil() + metrics.Descent.Ceil()
		fe.baseline = metrics.Ascent.Ceil()

		adv, ok := loadedFace.GlyphAdvance('M')
		if ok && adv > 0 {
			fe.charWidth = adv.Ceil()
		} else {
			fe.charWidth = 8
		}
	} else {
		fe.face = basicfont.Face7x13
		fe.charWidth = 7
		fe.charHeight = 13
		fe.baseline = 11
	}

	if fe.charWidth < 6 {
		fe.charWidth = 6
	}
	if fe.charHeight < 10 {
		fe.charHeight = 10
	}

	fe.precacheASCII()

	return fe, nil
}

func (fe *FontEngine) loadFallbacks(fontSize float64, paths []string) {
	fe.fbMu.Lock()
	if fe.fbStarted {
		fe.fbMu.Unlock()
		return
	}
	fe.fbStarted = true
	fe.fbMu.Unlock()
	defer func() {
		fe.fbMu.Lock()
		fe.fallbacksLoaded = true
		fe.fbMu.Unlock()
	}()

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		col, err := opentype.ParseCollection(data)
		if err != nil {
			continue
		}

		sfntFont, err := col.Font(0)
		if err != nil {
			continue
		}
		face, err := opentype.NewFace(sfntFont, &opentype.FaceOptions{
			Size:    fontSize,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err != nil {
			continue
		}
		fb := fallbackFont{
			face: face,
			sfnt: sfntFont,
			has: func(r rune) bool {
				idx, err := sfntFont.GlyphIndex(&sfnt.Buffer{}, r)
				return err == nil && idx != 0
			},
		}
		fe.fbMu.Lock()
		fe.fallbacks = append(fe.fallbacks, fb)
		fe.fbMu.Unlock()
	}
}

func (fe *FontEngine) fallbackSnapshot() []fallbackFont {
	fe.fbMu.RLock()
	defer fe.fbMu.RUnlock()
	out := make([]fallbackFont, len(fe.fallbacks))
	copy(out, fe.fallbacks)
	return out
}

func (fe *FontEngine) fallbacksReady() bool {
	fe.fbMu.RLock()
	defer fe.fbMu.RUnlock()
	return fe.fallbacksLoaded
}

func (fe *FontEngine) startFallbackLoad() {
	paths := append([]string(nil), candidateFallbackFonts...)
	go fe.loadFallbacks(fe.fontSize, paths)
}

func (fe *FontEngine) precacheASCII() {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	for r := rune(32); r < 127; r++ {
		idx := int(r - 32)
		mask, _ := fe.rasterizeRune(r, false, fe.charWidth)
		fe.asciiCache[idx][0] = mask
		mask, _ = fe.rasterizeRune(r, true, fe.charWidth)
		fe.asciiCache[idx][1] = mask
	}
}

func (fe *FontEngine) CharWidth() int {
	fe.mu.RLock()
	defer fe.mu.RUnlock()
	return fe.charWidth
}

func (fe *FontEngine) CharHeight() int {
	fe.mu.RLock()
	defer fe.mu.RUnlock()
	return fe.charHeight
}

func (fe *FontEngine) Baseline() int {
	fe.mu.RLock()
	defer fe.mu.RUnlock()
	return fe.baseline
}

func (fe *FontEngine) FontSize() float64 {
	fe.mu.RLock()
	defer fe.mu.RUnlock()
	return fe.fontSize
}

func (fe *FontEngine) SetFontSize(size float64) error {
	if size < 7.0 {
		size = 7.0
	}
	if size > 36.0 {
		size = 36.0
	}
	if fe.FontSize() == size {
		return nil
	}

	fe.mu.Lock()
	fe.fontSize = size
	if fe.ttf != nil {
		face := truetype.NewFace(fe.ttf, &truetype.Options{
			Size:    size,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if fe.face != nil && fe.face != basicfont.Face7x13 {
			_ = fe.face.Close()
		}
		fe.face = face

		metrics := face.Metrics()
		fe.charHeight = metrics.Ascent.Ceil() + metrics.Descent.Ceil()
		fe.baseline = metrics.Ascent.Ceil()

		adv, ok := face.GlyphAdvance('M')
		if ok && adv > 0 {
			fe.charWidth = adv.Ceil()
		} else {
			fe.charWidth = 8
		}
	} else {
		fe.charWidth = 7
		fe.charHeight = 13
		fe.baseline = 11
	}

	if fe.charWidth < 6 {
		fe.charWidth = 6
	}
	if fe.charHeight < 10 {
		fe.charHeight = 10
	}

	fe.otherCache = make(map[uint64]*GlyphMask)
	fe.wideCache = make(map[uint64]*GlyphMask)
	fe.mu.Unlock()

	fe.fbMu.Lock()
	for i := range fe.fallbacks {
		_ = fe.fallbacks[i].face.Close()
		if face, err := opentype.NewFace(fe.fallbacks[i].sfnt, &opentype.FaceOptions{
			Size: size, DPI: 72, Hinting: font.HintingFull,
		}); err == nil {
			fe.fallbacks[i].face = face
		}
	}
	fe.fbMu.Unlock()
	fe.precacheASCII()
	return nil
}

func (fe *FontEngine) ZoomIn() bool {
	if fe.FontSize() >= 36.0 {
		return false
	}
	_ = fe.SetFontSize(fe.FontSize() + 1.0)
	return true
}

func (fe *FontEngine) ZoomOut() bool {
	if fe.FontSize() <= 7.0 {
		return false
	}
	_ = fe.SetFontSize(fe.FontSize() - 1.0)
	return true
}

func (fe *FontEngine) ZoomReset() bool {
	if fe.FontSize() == 13.0 {
		return false
	}
	_ = fe.SetFontSize(13.0)
	return true
}

func (fe *FontEngine) GetGlyph(r rune, bold bool) *GlyphMask {
	if unicode.Is(unicode.Arabic, r) {
		r = unicode.ReplacementChar
	}
	boldIdx := 0
	if bold {
		boldIdx = 1
	}

	fe.mu.Lock()
	defer fe.mu.Unlock()

	if r >= 32 && r < 127 {
		idx := int(r - 32)
		mask := fe.asciiCache[idx][boldIdx]
		if mask != nil {
			return mask
		}
	}

	key := (uint64(r) << 1) | uint64(boldIdx)
	if mask, ok := fe.otherCache[key]; ok {
		return mask
	}

	mask, cacheable := fe.rasterizeRune(r, bold, fe.charWidth)
	if cacheable {
		if len(fe.otherCache) >= 4096 {
			evictQuarter(fe.otherCache)
		}
		fe.otherCache[key] = mask
	}
	return mask
}

func evictQuarter(m map[uint64]*GlyphMask) {
	n := 0
	for k := range m {
		if n%4 == 0 {
			delete(m, k)
		}
		n++
	}
}

func (fe *FontEngine) GetWideGlyph(r rune, bold bool) *GlyphMask {
	if unicode.Is(unicode.Arabic, r) {
		r = unicode.ReplacementChar
	}
	boldIdx := 0
	if bold {
		boldIdx = 1
	}
	key := (uint64(r) << 1) | uint64(boldIdx)
	fe.mu.Lock()
	defer fe.mu.Unlock()
	if mask, ok := fe.wideCache[key]; ok {
		return mask
	}
	mask, cacheable := fe.rasterizeRune(r, bold, 2*fe.charWidth)
	if cacheable {
		if len(fe.wideCache) >= 4096 {
			evictQuarter(fe.wideCache)
		}
		fe.wideCache[key] = mask
	}
	return mask
}

func (fe *FontEngine) rasterizeRune(r rune, bold bool, width int) (*GlyphMask, bool) {
	if box := renderBoxOrBlock(r, width, fe.charHeight); box != nil {
		return box, true
	}

	useFace := fe.face
	found := true

	if r >= 128 && (fe.ttf == nil || fe.ttf.Index(r) == 0) {
		fe.startFallbackLoad()
		found = false
		for _, fb := range fe.fallbackSnapshot() {
			if fb.has(r) {
				useFace = fb.face
				found = true
				break
			}
		}
	}

	if !found {
		return renderNotdef(width, fe.charHeight), fe.fallbacksReady()
	}

	pad := 0
	if b, _, ok := useFace.GlyphBounds(r); ok && b.Min.X < 0 {
		pad = int((-b.Min.X + 63) >> 6)
	}

	dst := image.NewAlpha(image.Rect(0, 0, width+pad, fe.charHeight))

	dot := fixed.P(pad, fe.baseline)
	d := &font.Drawer{
		Dst:  dst,
		Src:  image.Opaque,
		Face: useFace,
		Dot:  dot,
	}
	d.DrawString(string(r))

	if bold {
		d.Dot = fixed.P(pad+1, fe.baseline)
		d.DrawString(string(r))
	}

	mask := &GlyphMask{
		Width:   width + pad,
		Height:  fe.charHeight,
		OriginX: pad,
		Alpha:   make([]byte, (width+pad)*fe.charHeight),
	}

	for y := 0; y < fe.charHeight; y++ {
		for x := 0; x < width+pad; x++ {
			c := dst.AlphaAt(x, y)
			mask.Alpha[y*(width+pad)+x] = c.A
		}
	}

	return mask, true
}

func (fe *FontEngine) Close() {
	fe.mu.Lock()
	face := fe.face
	fe.mu.Unlock()
	if face != nil && face != basicfont.Face7x13 {
		_ = face.Close()
	}
	for _, fb := range fe.fallbackSnapshot() {
		if fb.face != nil && fb.face != basicfont.Face7x13 {
			_ = fb.face.Close()
		}
	}
}

func renderBoxOrBlock(r rune, w, h int) *GlyphMask {
	if w <= 0 || h <= 0 {
		return nil
	}

	mask := &GlyphMask{
		Width:  w,
		Height: h,
		Alpha:  make([]byte, w*h),
	}

	setPixel := func(x, y int, alpha byte) {
		if x >= 0 && x < w && y >= 0 && y < h {
			mask.Alpha[y*w+x] = alpha
		}
	}
	fillRect := func(x0, y0, x1, y1 int, alpha byte) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				setPixel(x, y, alpha)
			}
		}
	}

	if r >= 0x2580 && r <= 0x259f {
		switch r {
		case 0x2588:
			fillRect(0, 0, w, h, 0xff)
			return mask
		case 0x2580:
			fillRect(0, 0, w, h/2, 0xff)
			return mask
		case 0x2584:
			fillRect(0, h/2, w, h, 0xff)
			return mask
		case 0x258c:
			fillRect(0, 0, w/2, h, 0xff)
			return mask
		case 0x2590:
			fillRect(w/2, 0, w, h, 0xff)
			return mask
		case 0x2594:
			fillRect(0, 0, w, (h+7)/8, 0xff)
			return mask
		case 0x2595:
			fillRect(w-(w+7)/8, 0, w, h, 0xff)
			return mask
		case 0x2591:
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					if (x+y)%4 == 0 {
						setPixel(x, y, 0xff)
					}
				}
			}
			return mask
		case 0x2592:
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					if (x+y)%2 == 0 {
						setPixel(x, y, 0xff)
					}
				}
			}
			return mask
		case 0x2593:
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					if (x+y)%4 != 0 {
						setPixel(x, y, 0xff)
					}
				}
			}
			return mask
		}
		if r >= 0x2581 && r <= 0x2587 {
			eighths := int(r - 0x2580)
			blockH := (h * eighths) / 8
			if blockH < 1 {
				blockH = 1
			}
			fillRect(0, h-blockH, w, h, 0xff)
			return mask
		}
		if r >= 0x2589 && r <= 0x258f {
			eighths := 8 - int(r-0x2588)
			blockW := (w * eighths) / 8
			if blockW < 1 {
				blockW = 1
			}
			fillRect(0, 0, blockW, h, 0xff)
			return mask
		}
	}

	if r >= 0x2500 && r <= 0x257f {
		var up, down, left, right int
		switch r {
		case 0x2500:
			left, right = 1, 1
		case 0x2501:
			left, right = 2, 2
		case 0x2502:
			up, down = 1, 1
		case 0x2503:
			up, down = 2, 2
		case 0x2508:
			left, right = 1, 1
		case 0x2509:
			left, right = 2, 2
		case 0x250A:
			up, down = 1, 1
		case 0x250B:
			up, down = 2, 2
		case 0x250C:
			down, right = 1, 1
		case 0x250D:
			down, right = 1, 2
		case 0x250E:
			down, right = 2, 1
		case 0x250F:
			down, right = 2, 2
		case 0x2510:
			down, left = 1, 1
		case 0x2511:
			down, left = 1, 2
		case 0x2512:
			down, left = 2, 1
		case 0x2513:
			down, left = 2, 2
		case 0x2514:
			up, right = 1, 1
		case 0x2515:
			up, right = 1, 2
		case 0x2516:
			up, right = 2, 1
		case 0x2517:
			up, right = 2, 2
		case 0x2518:
			up, left = 1, 1
		case 0x2519:
			up, left = 1, 2
		case 0x251A:
			up, left = 2, 1
		case 0x251B:
			up, left = 2, 2
		case 0x251C:
			up, down, right = 1, 1, 1
		case 0x251D:
			up, down, right = 1, 1, 2
		case 0x251E:
			up, down, right = 2, 1, 1
		case 0x251F:
			up, down, right = 1, 2, 1
		case 0x2520:
			up, down, right = 2, 2, 1
		case 0x2523:
			up, down, right = 2, 2, 2
		case 0x2524:
			up, down, left = 1, 1, 1
		case 0x2525:
			up, down, left = 1, 1, 2
		case 0x2528:
			up, down, left = 2, 2, 1
		case 0x252B:
			up, down, left = 2, 2, 2
		case 0x252C:
			down, left, right = 1, 1, 1
		case 0x252F:
			down, left, right = 1, 2, 2
		case 0x2530:
			down, left, right = 2, 1, 1
		case 0x2533:
			down, left, right = 2, 2, 2
		case 0x2534:
			up, left, right = 1, 1, 1
		case 0x2537:
			up, left, right = 1, 2, 2
		case 0x2538:
			up, left, right = 2, 1, 1
		case 0x253B:
			up, left, right = 2, 2, 2
		case 0x253C:
			up, down, left, right = 1, 1, 1, 1
		case 0x253F:
			up, down, left, right = 1, 1, 2, 2
		case 0x2542:
			up, down, left, right = 2, 2, 1, 1
		case 0x254B:
			up, down, left, right = 2, 2, 2, 2
		case 0x254C:
			left, right = 1, 1
		case 0x254D:
			left, right = 2, 2
		case 0x254E:
			up, down = 1, 1
		case 0x254F:
			up, down = 2, 2
		case 0x2550:
			left, right = 3, 3
		case 0x2551:
			up, down = 3, 3
		case 0x2552:
			down, right = 1, 3
		case 0x2553:
			down, right = 3, 1
		case 0x2554:
			down, right = 3, 3
		case 0x2555:
			down, left = 1, 3
		case 0x2556:
			down, left = 3, 1
		case 0x2557:
			down, left = 3, 3
		case 0x2558:
			up, right = 1, 3
		case 0x2559:
			up, right = 3, 1
		case 0x255A:
			up, right = 3, 3
		case 0x255B:
			up, left = 1, 3
		case 0x255C:
			up, left = 3, 1
		case 0x255D:
			up, left = 3, 3
		case 0x255E:
			up, down, right = 1, 1, 3
		case 0x255F:
			up, down, right = 3, 3, 1
		case 0x2560:
			up, down, right = 3, 3, 3
		case 0x2561:
			up, down, left = 1, 1, 3
		case 0x2562:
			up, down, left = 3, 3, 1
		case 0x2563:
			up, down, left = 3, 3, 3
		case 0x2564:
			down, left, right = 1, 3, 3
		case 0x2565:
			down, left, right = 3, 1, 1
		case 0x2566:
			down, left, right = 3, 3, 3
		case 0x2567:
			up, left, right = 1, 3, 3
		case 0x2568:
			up, left, right = 3, 1, 1
		case 0x2569:
			up, left, right = 3, 3, 3
		case 0x256A:
			up, down, left, right = 1, 1, 3, 3
		case 0x256B:
			up, down, left, right = 3, 3, 1, 1
		case 0x256C:
			up, down, left, right = 3, 3, 3, 3
		case 0x256D:
			down, right = 1, 1
		case 0x256E:
			down, left = 1, 1
		case 0x256F:
			up, left = 1, 1
		case 0x2570:
			up, right = 1, 1
		case 0x2574:
			left = 1
		case 0x2575:
			up = 1
		case 0x2576:
			right = 1
		case 0x2577:
			down = 1
		case 0x2578:
			left = 2
		case 0x2579:
			up = 2
		case 0x257A:
			right = 2
		case 0x257B:
			down = 2
		default:
			return nil
		}

		midX := w / 2
		midY := h / 2

		drawLine := func(mode int, x0, y0, x1, y1 int, isHoriz bool) {
			switch mode {
			case 0:
				return
			case 1:
				if isHoriz {
					fillRect(x0, midY, x1, midY+1, 0xff)
				} else {
					fillRect(midX, y0, midX+1, y1, 0xff)
				}
			case 2:
				if isHoriz {
					fillRect(x0, midY-1, x1, midY+1, 0xff)
				} else {
					fillRect(midX-1, y0, midX+1, y1, 0xff)
				}
			case 3:
				if isHoriz {
					fillRect(x0, midY-2, x1, midY-1, 0xff)
					fillRect(x0, midY+1, x1, midY+2, 0xff)
				} else {
					fillRect(midX-2, y0, midX-1, y1, 0xff)
					fillRect(midX+1, y0, midX+2, y1, 0xff)
				}
			}
		}

		drawLine(up, midX, 0, midX+1, midY+1, false)
		drawLine(down, midX, midY, midX+1, h, false)
		drawLine(left, 0, midY, midX+1, midY+1, true)
		drawLine(right, midX, midY, w, midY+1, true)
		return mask
	}

	return nil
}

func renderNotdef(w, h int) *GlyphMask {
	if w <= 0 || h <= 0 {
		return nil
	}
	mask := &GlyphMask{
		Width:  w,
		Height: h,
		Alpha:  make([]byte, w*h),
	}
	set := func(x, y int) {
		if x >= 0 && x < w && y >= 0 && y < h {
			mask.Alpha[y*w+x] = 0xff
		}
	}

	for x := 1; x < w-1; x++ {
		set(x, 1)
		set(x, h-2)
	}
	for y := 1; y < h-1; y++ {
		set(1, y)
		set(w-2, y)
	}
	return mask
}

func DrawGlyphBlit(buf []byte, stride int, cellX, cellY int, mask *GlyphMask, fgPixel, bgPixel uint32, underline bool, baseline int) {
	if mask == nil || stride <= 0 || mask.Width <= 0 || mask.Height <= 0 || len(mask.Alpha) < mask.Width*mask.Height {
		return
	}
	width := mask.Width
	height := mask.Height

	originX := cellX - mask.OriginX
	cellW := mask.Width - mask.OriginX

	fgR := byte(fgPixel >> 16)
	fgG := byte(fgPixel >> 8)
	fgB := byte(fgPixel)

	bgR := byte(bgPixel >> 16)
	bgG := byte(bgPixel >> 8)
	bgB := byte(bgPixel)

	for y := 0; y < height; y++ {
		py := cellY + y
		if py < 0 || py >= len(buf)/stride {
			continue
		}
		startX := max(0, -originX)
		endX := min(width, stride/4-originX)
		if startX >= endX {
			continue
		}
		bufRowOffset := py * stride
		maskRowOffset := y * width

		for x := startX; x < endX; x++ {
			pixelOffset := bufRowOffset + (originX+x)*4

			alpha := int(mask.Alpha[maskRowOffset+x])
			if alpha == 0 {
				if x < mask.OriginX {
					continue
				}
				buf[pixelOffset+0] = bgB
				buf[pixelOffset+1] = bgG
				buf[pixelOffset+2] = bgR
				buf[pixelOffset+3] = 0xff
			} else if alpha >= 255 {
				buf[pixelOffset+0] = fgB
				buf[pixelOffset+1] = fgG
				buf[pixelOffset+2] = fgR
				buf[pixelOffset+3] = 0xff
			} else {
				inv := 255 - alpha
				outB := byte((int(fgB)*alpha + int(bgB)*inv) / 255)
				outG := byte((int(fgG)*alpha + int(bgG)*inv) / 255)
				outR := byte((int(fgR)*alpha + int(bgR)*inv) / 255)

				buf[pixelOffset+0] = outB
				buf[pixelOffset+1] = outG
				buf[pixelOffset+2] = outR
				buf[pixelOffset+3] = 0xff
			}
		}
	}

	if underline && baseline+1 >= 0 && baseline+1 < height && cellY+baseline+1 >= 0 && cellY+baseline+1 < len(buf)/stride {
		py := cellY + baseline + 1
		bufRowOffset := py * stride
		startX := max(0, -cellX)
		endX := min(cellW, stride/4-cellX)
		for x := startX; x < endX; x++ {
			pixelOffset := bufRowOffset + (cellX+x)*4
			buf[pixelOffset+0] = fgB
			buf[pixelOffset+1] = fgG
			buf[pixelOffset+2] = fgR
			buf[pixelOffset+3] = 0xff
		}
	}
}

func FillRect(buf []byte, stride int, x, y, w, h int, pixel uint32) {
	if w <= 0 || h <= 0 || stride <= 0 {
		return
	}
	b := byte(pixel)
	g := byte(pixel >> 8)
	r := byte(pixel >> 16)
	startX := max(0, x)
	endX := min(stride/4, x+w)
	if startX >= endX {
		return
	}
	startY := max(0, y)
	endY := min(len(buf)/stride, y+h)
	if startY >= endY {
		return
	}
	first := startY*stride + startX*4
	rowBytes := (endX - startX) * 4
	for p := first; p < first+rowBytes; p += 4 {
		buf[p+0] = b
		buf[p+1] = g
		buf[p+2] = r
		buf[p+3] = 0xff
	}
	for py := startY + 1; py < endY; py++ {
		offset := py*stride + startX*4
		copy(buf[offset:offset+rowBytes], buf[first:first+rowBytes])
	}
}

func (fe *FontEngine) DrawString(buf []byte, stride int, startX, startY int, text string, fgPixel, bgPixel uint32, bold bool) {
	charW := fe.CharWidth()
	curX := startX
	for _, ch := range text {
		mask := fe.GetGlyph(ch, bold)
		DrawGlyphBlit(buf, stride, curX, startY, mask, fgPixel, bgPixel, false, fe.Baseline())
		curX += charW
	}
}

func DrawGlyphBlitTransparent(buf []byte, stride int, cellX, cellY int, mask *GlyphMask, fgPixel uint32, underline bool, baseline int) {
	if mask == nil || stride <= 0 || mask.Width <= 0 || mask.Height <= 0 || len(mask.Alpha) < mask.Width*mask.Height {
		return
	}
	width := mask.Width
	height := mask.Height

	originX := cellX - mask.OriginX
	cellW := mask.Width - mask.OriginX

	fgR := byte(fgPixel >> 16)
	fgG := byte(fgPixel >> 8)
	fgB := byte(fgPixel)

	for y := 0; y < height; y++ {
		py := cellY + y
		if py < 0 || py >= len(buf)/stride {
			continue
		}
		startX := max(0, -originX)
		endX := min(width, stride/4-originX)
		if startX >= endX {
			continue
		}
		bufRowOffset := py * stride
		maskRowOffset := y * width

		for x := startX; x < endX; x++ {
			alpha := int(mask.Alpha[maskRowOffset+x])
			if alpha == 0 {
				continue
			}
			pixelOffset := bufRowOffset + (originX+x)*4

			if alpha >= 255 {
				buf[pixelOffset+0] = fgB
				buf[pixelOffset+1] = fgG
				buf[pixelOffset+2] = fgR
				buf[pixelOffset+3] = 0xff
			} else {
				inv := 255 - alpha
				buf[pixelOffset+0] = byte((int(fgB)*alpha + int(buf[pixelOffset+0])*inv) / 255)
				buf[pixelOffset+1] = byte((int(fgG)*alpha + int(buf[pixelOffset+1])*inv) / 255)
				buf[pixelOffset+2] = byte((int(fgR)*alpha + int(buf[pixelOffset+2])*inv) / 255)
				buf[pixelOffset+3] = 0xff
			}
		}
	}

	if underline && baseline+1 >= 0 && baseline+1 < height && cellY+baseline+1 >= 0 && cellY+baseline+1 < len(buf)/stride {
		py := cellY + baseline + 1
		bufRowOffset := py * stride
		startX := max(0, -cellX)
		endX := min(cellW, stride/4-cellX)
		for x := startX; x < endX; x++ {
			pixelOffset := bufRowOffset + (cellX+x)*4
			buf[pixelOffset+0] = fgB
			buf[pixelOffset+1] = fgG
			buf[pixelOffset+2] = fgR
			buf[pixelOffset+3] = 0xff
		}
	}
}

func MinecraftShadow(pixel uint32) uint32 {
	r := ((pixel >> 16) & 0xff) / 4
	g := ((pixel >> 8) & 0xff) / 4
	b := (pixel & 0xff) / 4
	return (r << 16) | (g << 8) | b
}

func (fe *FontEngine) DrawStringShadow(buf []byte, stride int, startX, startY int, text string, fgPixel, shadowPixel uint32, bold bool) {
	charW := fe.CharWidth()
	baseline := fe.Baseline()
	curX := startX + 1
	curY := startY + 1
	for _, ch := range text {
		mask := fe.GetGlyph(ch, bold)
		DrawGlyphBlitTransparent(buf, stride, curX, curY, mask, shadowPixel, false, baseline)
		curX += charW
	}
	curX = startX
	for _, ch := range text {
		mask := fe.GetGlyph(ch, bold)
		DrawGlyphBlitTransparent(buf, stride, curX, startY, mask, fgPixel, false, baseline)
		curX += charW
	}
}

func DrawHLine(buf []byte, stride int, x, y, width int, pixel uint32) {
	if stride <= 0 || y < 0 || y >= len(buf)/stride || width <= 0 {
		return
	}
	b := byte(pixel)
	g := byte(pixel >> 8)
	r := byte(pixel >> 16)
	rowOffset := y * stride
	for px := max(0, x); px < min(stride/4, x+width); px++ {
		p := rowOffset + px*4
		buf[p+0] = b
		buf[p+1] = g
		buf[p+2] = r
		buf[p+3] = 0xff
	}
}

func DrawVLine(buf []byte, stride int, x, y, height int, pixel uint32) {
	if stride <= 0 || x < 0 || x >= stride/4 || height <= 0 {
		return
	}
	b := byte(pixel)
	g := byte(pixel >> 8)
	r := byte(pixel >> 16)
	for py := max(0, y); py < min(len(buf)/stride, y+height); py++ {
		p := py*stride + x*4
		buf[p+0] = b
		buf[p+1] = g
		buf[p+2] = r
		buf[p+3] = 0xff
	}
}

func DrawRectBorder(buf []byte, stride int, x, y, w, h int, pixel uint32) {
	DrawHLine(buf, stride, x, y, w, pixel)
	DrawHLine(buf, stride, x, y+h-1, w, pixel)
	DrawVLine(buf, stride, x, y, h, pixel)
	DrawVLine(buf, stride, x+w-1, y, h, pixel)
}
