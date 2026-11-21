package terminal

type Color uint32

const (
	ColorDefaultFG Color = 0x01000000
	ColorDefaultBG Color = 0x02000000
)

var ansi16 = [16]Color{
	0x15161e,
	0xf7768e,
	0x9ece6a,
	0xe0af68,
	0x7aa2f7,
	0xbb9af7,
	0x7dcfff,
	0xa9b1d6,
	0x414868,
	0xf7768e,
	0x73daca,
	0xe0af68,
	0x7aa2f7,
	0xbb9af7,
	0x2ac3de,
	0xc0caf5,
}

func RGB(r, g, b byte) Color {
	return Color(uint32(r)<<16 | uint32(g)<<8 | uint32(b))
}

func (c Color) ToPixel() uint32 {
	return uint32(c&0x00ffffff) | 0xff000000
}
