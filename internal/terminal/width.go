package terminal

import "unicode"

// RuneWidth returns the terminal display width of r in cells, following the
// East Asian Width rules used by wcwidth / go-runewidth (non-East-Asian
// context, so ambiguous-width runes count as 1):
//
//   - 0: combining marks, zero-width joiners and variation selectors. These
//     attach to the preceding cell and never occupy a cell of their own.
//     Control characters also report 0; the parser drops them before they
//     reach putChar, so a 0 width here means "do not advance the cursor".
//   - 2: East Asian wide/fullwidth runes (CJK ideographs, Hangul, fullwidth
//     forms) and most emoji.
//   - 1: everything else.
//
// This is a hand-rolled table (no new module dependency): the terminal must
// keep building offline with GOTOOLCHAIN=local.
func RuneWidth(r rune) int {
	if r < 0x20 || (r >= 0x7f && r < 0xa0) {
		return 0
	}
	if isZeroWidth(r) {
		return 0
	}
	if isWide(r) {
		return 2
	}
	return 1
}

// IsZeroWidthAttachable reports whether r is a zero-width rune that should be
// kept as part of the previous cell's grapheme (combining mark, ZWNJ, ZWJ,
// variation selector, ...), so copy/paste round-trips the original text
// (e.g. the Persian ZWNJ in "می‌کنم"). Control characters are zero-width
// but not attachable: they are dropped.
func IsZeroWidthAttachable(r rune) bool {
	if r < 0x20 || (r >= 0x7f && r < 0xa0) {
		return false
	}
	return isZeroWidth(r)
}

func isZeroWidth(r rune) bool {
	switch r {
	case '\u200b', // zero width space
		'\u200c', // zero width non-joiner (Persian نیم‌فاصله)
		'\u200d', // zero width joiner
		'\u2060', // word joiner
		'\ufeff': // zero width no-break space / BOM
		return true
	}
	if r >= 0xfe00 && r <= 0xfe0f {
		return true // variation selectors
	}
	if r >= 0xe0100 && r <= 0xe01ef {
		return true // variation selectors supplement
	}
	// Combining marks (Mn, Me) attach to the previous character.
	return unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r)
}

// wideRanges lists the [lo, hi] rune ranges that occupy two cells.
// East Asian Wide/Fullwidth blocks plus the emoji blocks that
// go-runewidth treats as wide.
var wideRanges = [][2]rune{
	{0x1100, 0x115f},   // Hangul Jamo
	{0x231a, 0x231b},   // watch, hourglass
	{0x2329, 0x232a},   // angle brackets
	{0x23e9, 0x23ec},   // black squares (emoji presentation)
	{0x23f0, 0x23f0},   // alarm clock
	{0x23f3, 0x23f3},   // hourglass flowing sand
	{0x25fd, 0x25fe},   // small squares
	{0x2614, 0x2615},   // umbrella, hot beverage
	{0x2648, 0x2653},   // zodiac
	{0x267f, 0x267f},   // wheelchair symbol
	{0x2693, 0x2693},   // anchor
	{0x26a1, 0x26a1},   // high voltage sign
	{0x26aa, 0x26ab},   // circles
	{0x26bd, 0x26be},   // soccer ball, baseball
	{0x26c4, 0x26c5},   // snowman, sun behind cloud
	{0x26ce, 0x26ce},   // Ophiuchus
	{0x26d4, 0x26d4},   // no entry
	{0x26ea, 0x26ea},   // church
	{0x26f2, 0x26f3},   // fountain, flag in hole
	{0x26f5, 0x26f5},   // sailboat
	{0x26fa, 0x26fa},   // tent
	{0x26fd, 0x26fd},   // fuel pump
	{0x2705, 0x2705},   // white heavy check mark
	{0x270a, 0x270b},   // raised fist/hand
	{0x2728, 0x2728},   // sparkles
	{0x274c, 0x274c},   // cross mark
	{0x274e, 0x274e},   // cross mark button
	{0x2753, 0x2755},   // question/exclamation marks
	{0x2757, 0x2757},   // heavy exclamation mark
	{0x2795, 0x2797},   // plus/minus/divide
	{0x27b0, 0x27b0},   // curly loop
	{0x27bf, 0x27bf},   // double curly loop
	{0x2b1b, 0x2b1c},   // black/white large squares
	{0x2b50, 0x2b50},   // white medium star
	{0x2b55, 0x2b55},   // heavy large circle
	{0x2e80, 0x2e99},   // CJK radicals supplement
	{0x2e9b, 0x2ef3},   // CJK radicals supplement
	{0x2f00, 0x2fd5},   // Kangxi radicals
	{0x2ff0, 0x2ffb},   // ideographic description characters
	{0x3000, 0x303e},   // CJK symbols and punctuation
	{0x3041, 0x3096},   // Hiragana
	{0x3099, 0x30ff},   // Hiragana / Katakana
	{0x3105, 0x312d},   // Bopomofo
	{0x3131, 0x318e},   // Hangul compatibility Jamo
	{0x3190, 0x31e3},   // Kanbun / CJK strokes
	{0x31f0, 0x321e},   // Katakana phonetic extensions
	{0x3220, 0x3247},   // enclosed CJK letters and months
	{0x3250, 0x4dbf},   // enclosed CJK / CJK ext A
	{0x4e00, 0xa48c},   // CJK unified ideographs
	{0xa490, 0xa4c6},   // Yi syllables
	{0xa960, 0xa97c},   // Hangul Jamo ext A
	{0xac00, 0xd7a3},   // Hangul syllables
	{0xf900, 0xfaff},   // CJK compatibility ideographs
	{0xfe10, 0xfe19},   // vertical forms
	{0xfe30, 0xfe52},   // CJK compatibility forms
	{0xfe54, 0xfe66},   // small form variants
	{0xfe68, 0xfe6b},   // small form variants
	{0xff00, 0xff60},   // fullwidth forms
	{0xffe0, 0xffe6},   // fullwidth signs
	{0x16fe0, 0x16fe4}, // ideographic symbols
	{0x17000, 0x187f7}, // Tangut ideographs
	{0x18800, 0x18cd5}, // Tangut components
	{0x18d00, 0x18d08}, // Tangut ideographs supplement
	{0x1aff0, 0x1aff3}, // Kana extended-B
	{0x1aff5, 0x1affb}, // Kana extended-B
	{0x1affd, 0x1affe}, // Kana extended-B
	{0x1b000, 0x1b122}, // Kana supplement
	{0x1b150, 0x1b152}, // Kana supplement
	{0x1b164, 0x1b167}, // Kana supplement
	{0x1b170, 0x1b2fb}, // Nushu
	{0x1f004, 0x1f004}, // mahjong tile
	{0x1f0cf, 0x1f0cf}, // playing card black joker
	{0x1f18e, 0x1f18e}, // negative squared AB
	{0x1f191, 0x1f19a}, // squared words
	{0x1f200, 0x1f202}, // squared ideographs
	{0x1f210, 0x1f23b}, // squared ideographs
	{0x1f240, 0x1f248}, // squared ideographs
	{0x1f250, 0x1f251}, // circled ideographs
	{0x1f260, 0x1f265}, // circled ideographs
	{0x1f300, 0x1f320}, // weather, maps, etc.
	{0x1f32d, 0x1f335}, // food / plants
	{0x1f337, 0x1f37c}, // plants / food
	{0x1f37e, 0x1f393}, // food / drink / celebration
	{0x1f3a0, 0x1f3ca}, // activities / sports
	{0x1f3cf, 0x1f3d3}, // sports
	{0x1f3e0, 0x1f3f0}, // buildings / places
	{0x1f3f4, 0x1f3f4}, // black flag
	{0x1f3f8, 0x1f43e}, // art / animals
	{0x1f440, 0x1f440}, // eyes
	{0x1f442, 0x1f4fc}, // body / clothing / objects
	{0x1f4ff, 0x1f53d}, // objects / symbols
	{0x1f54b, 0x1f54e}, // religion
	{0x1f550, 0x1f567}, // clocks
	{0x1f57a, 0x1f57a}, // man dancing
	{0x1f595, 0x1f5a4}, // hands
	{0x1f5fb, 0x1f64f}, // maps / faces
	{0x1f680, 0x1f6c5}, // transport / places
	{0x1f6cc, 0x1f6cc}, // sleeping face
	{0x1f6d0, 0x1f6d2}, // place of worship
	{0x1f6d5, 0x1f6d7}, // hammock / etc.
	{0x1f6dc, 0x1f6df}, // wireless / etc.
	{0x1f6eb, 0x1f6ec}, // airplane departure / arrival
	{0x1f6f4, 0x1f6fc}, // transport
	{0x1f7e0, 0x1f7eb}, // geometric shapes (circles)
	{0x1f7f0, 0x1f7f0}, // heavy equals sign
	{0x1f90c, 0x1f93a}, // hands / body
	{0x1f93c, 0x1f945}, // sports
	{0x1f947, 0x1f9ff}, // medals / faces / animals
	{0x1fa70, 0x1fa74}, // food
	{0x1fa78, 0x1fa7c}, // food
	{0x1fa80, 0x1fa88}, // objects
	{0x1fa90, 0x1fabd}, // objects / body
	{0x1fabf, 0x1fac5}, // body
	{0x1face, 0x1fadb}, // faces / body
	{0x1fae0, 0x1fae8}, // faces
	{0x1faf0, 0x1faf8}, // hands
	{0x20000, 0x2fffd}, // CJK ext B-F
	{0x30000, 0x3fffd}, // CJK ext G-H
}

func isWide(r rune) bool {
	// Binary search over the sorted, non-overlapping ranges.
	lo, hi := 0, len(wideRanges)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		if r < wideRanges[mid][0] {
			hi = mid - 1
		} else if r > wideRanges[mid][1] {
			lo = mid + 1
		} else {
			return true
		}
	}
	return false
}
