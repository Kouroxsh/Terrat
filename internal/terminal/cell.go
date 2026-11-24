package terminal

// ColorKind records how a cell's colour was specified, so a theme change
// can remap palette colours without touching explicit truecolour values
// (L6): palette entries are re-resolved against the new theme, RGB values
// are left exactly as the program set them.
//
// int8: there are only four kinds, and keeping this (and the palette
// indices below) narrow keeps Cell small — Cell is copied on every
// putChar and cleared on every scroll, so its size is on the terminal's
// hot path (see the 2026-11-24 benchmark work: 80-byte cells cost ~2x
// the write throughput of 32-byte cells).
type ColorKind int8

const (
	ColorKindDefault ColorKind = iota // terminal default FG/BG (resolves via the theme)
	ColorKindANSI16                   // one of the 16 theme palette entries
	ColorKindANSI256                  // a 256-colour palette entry
	ColorKindRGB                      // an explicit truecolour value
)

// Cell is one grid cell. A wide rune (CJK, emoji) occupies two cells: the
// lead cell carries the rune with Wide set, and the following cell is a
// continuation (Continuation set, Char 0) that the renderer and text
// extraction skip. Zero-width runes (combining marks, ZWNJ/ZWJ, variation
// selectors) never occupy a cell: they are stored in Extra on the preceding
// cell so copy/paste and search still see the original text.
type Cell struct {
	Char rune
	FG   Color
	BG   Color
	// FGKind/BGKind + FGIdx/BGIdx record how FG/BG were specified (see
	// ColorKind); FGIdx/BGIdx hold the palette index for the ANSI kinds
	// (0-255, 0 for default/RGB), so int16 is ample.
	FGKind       ColorKind
	BGKind       ColorKind
	FGIdx        int16
	BGIdx        int16
	Bold         bool
	Underline    bool
	Inverse      bool
	Wide         bool
	Continuation bool
	// Extra holds zero-width runes attached to this cell (combining
	// marks, ZWNJ/ZWJ, variation selectors). It is a pointer so the
	// common case (no attached runes) costs 8 bytes instead of a
	// 24-byte slice header; use ExtraRunes to read it.
	Extra *[]rune
}

// Width returns the number of grid columns this cell occupies: 2 for the
// lead cell of a wide rune, 0 for a continuation cell, 1 otherwise.
func (c Cell) Width() int {
	if c.Continuation {
		return 0
	}
	if c.Wide {
		return 2
	}
	return 1
}

// ExtraRunes returns the zero-width runes attached to this cell, or nil
// when there are none. Extra is a pointer so the common case costs 8
// bytes; read it through here instead of dereferencing directly.
func (c Cell) ExtraRunes() []rune {
	if c.Extra == nil {
		return nil
	}
	return *c.Extra
}

// Equal reports whether two cells render and extract identically. (Cell
// cannot be compared with == because Extra is a pointer.)
func (c Cell) Equal(o Cell) bool {
	if c.Char != o.Char || c.FG != o.FG || c.BG != o.BG ||
		c.Bold != o.Bold || c.Underline != o.Underline || c.Inverse != o.Inverse ||
		c.Wide != o.Wide || c.Continuation != o.Continuation ||
		c.FGKind != o.FGKind || c.BGKind != o.BGKind ||
		c.FGIdx != o.FGIdx || c.BGIdx != o.BGIdx {
		return false
	}
	a, b := c.ExtraRunes(), o.ExtraRunes()
	if len(a) != len(b) {
		return false
	}
	for i, r := range a {
		if r != b[i] {
			return false
		}
	}
	return true
}

func EmptyCell() Cell {
	return Cell{
		Char: ' ',
		FG:   ColorDefaultFG,
		BG:   ColorDefaultBG,
	}
}
