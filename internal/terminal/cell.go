package terminal

type ColorKind int8

const (
	ColorKindDefault ColorKind = iota
	ColorKindANSI16
	ColorKindANSI256
	ColorKindRGB
)

type Cell struct {
	Char rune
	FG   Color
	BG   Color

	FGKind       ColorKind
	BGKind       ColorKind
	FGIdx        int16
	BGIdx        int16
	Bold         bool
	Underline    bool
	Inverse      bool
	Wide         bool
	Continuation bool

	Extra *[]rune
}

func (c Cell) ExtraRunes() []rune {
	if c.Extra == nil {
		return nil
	}
	return *c.Extra
}

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
