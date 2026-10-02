package render

import (
	"terrat/internal/terminal"
)

// CellFlags represent rendering attributes for a terminal cell.
type CellFlags uint32

const (
	CellFlagNone      CellFlags = 0
	CellFlagBold      CellFlags = 1 << 0
	CellFlagItalic    CellFlags = 1 << 1
	CellFlagUnderline CellFlags = 1 << 2
	CellFlagInverse   CellFlags = 1 << 3
	CellFlagBlink     CellFlags = 1 << 4
	CellFlagCursor    CellFlags = 1 << 5
)

// CellInstance represents a single character cell formatted for GPU instancing or batch rendering.
// Aligned for efficient GPU uniform/vertex buffer transfers.
type CellInstance struct {
	X, Y          float32
	Width, Height float32
	U1, V1        float32
	U2, V2        float32
	FG, BG        uint32
	Flags         CellFlags
}

// Renderer is the presentation interface implemented by both the CPU Canvas
// and the hardware-accelerated GPU backend.
type Renderer interface {
	Resize(width, height int)
	Render(term *terminal.Terminal, cursorBlink bool, title string, hudInfo string, tabs []TabInfo, searchMatches []SearchMatch, activeSearchIdx int, hoveredURL *URLRange, ghostText string, diag *DiagnosticInfo)
	Close()
}
