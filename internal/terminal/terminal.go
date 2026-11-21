package terminal

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

type parserState int

const (
	stateNormal parserState = iota
	stateEscape
	stateCSI
	stateOSC
	stateOSCEscape
	stateOSCDiscard
	stateOSCDiscardEscape
	stateCharset
)

type Terminal struct {
	mu sync.RWMutex

	cols int
	rows int

	lines        [][]Cell
	scrollback   [][]Cell
	historyStore [][]Cell
	maxHistory   int
	scrollOff    int

	// Wrapped flags, parallel to the grids and the scrollback: wrapped[i]
	// is true when row i is a continuation of the previous logical line
	// (the line wrapped onto it). Search joins wrapped rows so a query
	// spanning a wrap is found; every operation that moves rows must move
	// these flags in lockstep (see scrollUp, reverseIndex, executeCSI,
	// Resize). historyStoreWrapped is the reusable backing store for
	// scrollbackWrapped, mirroring historyStore.
	linesWrapped        []bool
	altWrapped          []bool
	scrollbackWrapped   []bool
	historyStoreWrapped []bool

	altLines [][]Cell
	isAlt    bool

	bracketedPaste bool

	autoWrap      bool
	wrapNext      bool
	savedWrapNext bool

	cursorX        int
	cursorY        int
	cursorVisible  bool
	savedX         int
	savedY         int
	savedFG        Color
	savedBG        Color
	savedFGKind    ColorKind
	savedBGKind    ColorKind
	savedFGIdx     int
	savedBGIdx     int
	savedBold      bool
	savedUnderline bool
	savedInverse   bool

	currFG        Color
	currBG        Color
	currFGKind    ColorKind
	currBGKind    ColorKind
	currFGIdx     int
	currBGIdx     int
	currBold      bool
	currUnderline bool
	currInverse   bool

	scrollTop    int
	scrollBottom int

	appCursor     bool
	mouseTracking bool
	mouseSGR      bool

	state       parserState
	pendingUTF8 []byte
	csiParams   []string
	csiPrivate  bool
	csiLeader   rune
	// csiIntermediates collects CSI intermediate bytes (0x20-0x2F, e.g.
	// the space in "CSI ! p") so they are not mistaken for the final
	// byte (L2). Reset when a new CSI sequence starts.
	csiIntermediates string
	oscBuffer        []rune

	title        string
	TitleChan    chan string
	ResponseChan chan []byte

	sel Selection

	theme *Theme
}

func New(cols, rows int) *Terminal {
	// L10: zero/negative dimensions used to build empty grids that panicked
	// on the first output; fall back to the classic 80x24 instead.
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	th := ThemeTokyoNight
	t := &Terminal{
		cols:          cols,
		rows:          rows,
		maxHistory:    5000,
		cursorVisible: true,
		autoWrap:      true,
		theme:         th,
		currFG:        ColorDefaultFG,
		currBG:        ColorDefaultBG,
		savedFG:       ColorDefaultFG,
		savedBG:       ColorDefaultBG,
		scrollTop:     0,
		scrollBottom:  rows - 1,
		TitleChan:     make(chan string, 16),
		ResponseChan:  make(chan []byte, 16),
	}

	t.lines = make([][]Cell, rows)
	t.altLines = make([][]Cell, rows)
	t.linesWrapped = make([]bool, rows)
	t.altWrapped = make([]bool, rows)
	for i := 0; i < rows; i++ {
		t.lines[i] = t.makeRow()
		t.altLines[i] = t.makeRow()
	}

	return t
}

func (t *Terminal) emptyCell() Cell {
	return Cell{
		Char: ' ',
		FG:   t.currFG,
		BG:   t.currBG,
	}
}

func (t *Terminal) emptyRow() []Cell {
	row := make([]Cell, t.cols)
	cell := t.emptyCell()
	for i := range row {
		row[i] = cell
	}
	return row
}

func (t *Terminal) makeRow() []Cell {
	row := make([]Cell, t.cols)
	for i := range row {
		row[i] = EmptyCell()
	}
	return row
}

func (t *Terminal) RLock()   { t.mu.RLock() }
func (t *Terminal) RUnlock() { t.mu.RUnlock() }

func (t *Terminal) Cursor() (int, int, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.CursorLocked()
}

func (t *Terminal) CursorLocked() (int, int, bool) {
	return t.cursorX, t.cursorY, t.cursorVisible
}

func (t *Terminal) GetCell(x, y int) Cell {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.GetCellLocked(x, y)
}

func (t *Terminal) GetCellLocked(x, y int) Cell {
	if x < 0 || x >= t.cols || y >= t.rows {
		return EmptyCell()
	}

	if t.scrollOff > 0 && !t.isAlt {
		targetIdx := (len(t.scrollback) - t.scrollOff) + y
		if targetIdx < 0 {
			return EmptyCell()
		}
		if targetIdx < len(t.scrollback) {
			if x >= 0 && x < len(t.scrollback[targetIdx]) {
				return t.scrollback[targetIdx][x]
			}
			return EmptyCell()
		}
		gridRow := targetIdx - len(t.scrollback)
		grid := t.activeGrid()
		if gridRow >= 0 && gridRow < len(grid) && x >= 0 && x < len(grid[gridRow]) {
			return grid[gridRow][x]
		}
		return EmptyCell()
	}

	if y < 0 && !t.isAlt {
		targetIdx := len(t.scrollback) + y
		if targetIdx >= 0 && targetIdx < len(t.scrollback) {
			if x >= 0 && x < len(t.scrollback[targetIdx]) {
				return t.scrollback[targetIdx][x]
			}
		}
		return EmptyCell()
	}

	grid := t.activeGrid()
	if y >= 0 && y < len(grid) && x >= 0 && x < len(grid[y]) {
		return grid[y][x]
	}
	return EmptyCell()
}

func (t *Terminal) IsAlt() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.isAlt
}

func (t *Terminal) IsAltLocked() bool {
	return t.isAlt
}

func (t *Terminal) BracketedPaste() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.bracketedPaste
}

func (t *Terminal) AppCursorLocked() bool {
	return t.appCursor
}

// AppCursor is the locking version of AppCursorLocked, for callers that do
// not already hold t.mu (e.g. the UI event handlers in internal/app).
func (t *Terminal) AppCursor() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.appCursor
}

func (t *Terminal) MouseTrackingLocked() bool {
	return t.mouseTracking
}

// MouseTracking is the locking version of MouseTrackingLocked, for callers
// that do not already hold t.mu.
func (t *Terminal) MouseTracking() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.mouseTracking
}

func (t *Terminal) MouseSGRLocked() bool {
	return t.mouseSGR
}

// MouseSGR is the locking version of MouseSGRLocked, for callers that do
// not already hold t.mu.
func (t *Terminal) MouseSGR() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.mouseSGR
}

func (t *Terminal) ScrollOff() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.scrollOff
}

func (t *Terminal) ScrollOffLocked() int {
	return t.scrollOff
}

func (t *Terminal) ScrollbackLen() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.scrollback)
}

func (t *Terminal) ScrollbackLenLocked() int {
	return len(t.scrollback)
}

func (t *Terminal) Rows() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.rows
}

// GetRowString returns the text of grid row y (grid coordinates: the
// cursor row, the row a program wrote to). It is NOT viewport-relative:
// when scrolled, the row visible at screen position y is a different row
// — use GetViewportRowString for screen coordinates (L11).
func (t *Terminal) GetRowString(y int) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if y < 0 || y >= t.rows {
		return ""
	}

	runes := make([]rune, t.cols)
	for x := 0; x < t.cols; x++ {
		cell := t.lines[y][x]
		r := cell.Char
		if r == 0 {
			r = ' '
		}
		runes[x] = r
	}
	return string(runes)
}

// GetViewportRowString returns the text of the row visible at screen row y,
// applying the same scrollback mapping as GetCellLocked: when scrolled,
// the top rows come from the scrollback. Callers with screen coordinates
// (mouse hover, URL detection) must use this, not GetRowString (L11).
func (t *Terminal) GetViewportRowString(y int) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if y < 0 || y >= t.rows {
		return ""
	}

	runes := make([]rune, t.cols)
	for x := 0; x < t.cols; x++ {
		cell := t.GetCellLocked(x, y)
		r := cell.Char
		if r == 0 {
			r = ' '
		}
		runes[x] = r
	}
	return string(runes)
}

// lowerRunes folds each rune individually. strings.ToLower can change
// the rune count (e.g. 'İ' U+0130 lowercases to two runes "i̇"), which
// would shift every column after it; per-rune folding keeps the rune
// count — and therefore the column map — intact (L12).
func lowerRunes(r []rune) []rune {
	out := make([]rune, len(r))
	for i, c := range r {
		out[i] = unicode.ToLower(c)
	}
	return out
}

// matchRunes reports whether query (already folded) matches text at
// position x. Matches are non-overlapping: after a match the scan
// continues past it instead of counting the same characters twice (L12).
func matchAt(text, query []rune, x int) bool {
	if x+len(query) > len(text) {
		return false
	}
	for j, qr := range query {
		if text[x+j] != qr {
			return false
		}
	}
	return true
}

// SearchMatchInfo reports a match position inside the visible grid.
// StartCol/EndCol are rune-based column indices (one rune per cell).
type SearchMatchInfo struct {
	Row      int
	StartCol int
	EndCol   int
}

// FindRowMatches returns match column ranges for a single row string that
// contains exactly one rune per grid cell. Comparison is case-insensitive
// and rune-based, so multi-byte characters align with grid columns.
func FindRowMatches(rowStr, query string) []SearchMatchInfo {
	if query == "" {
		return nil
	}
	q := lowerRunes([]rune(query))
	r := lowerRunes([]rune(rowStr))
	var matches []SearchMatchInfo
	for x := 0; x+len(q) <= len(r); {
		if matchAt(r, q, x) {
			matches = append(matches, SearchMatchInfo{StartCol: x, EndCol: x + len(q) - 1})
			x += len(q)
		} else {
			x++
		}
	}
	return matches
}

// RowText returns the row's text with continuation cells skipped and
// zero-width runes (Extra) included, plus a column map: colMap[i] is the
// grid column of the i-th rune of the text. Unlike GetRowString (one rune
// per cell), this is the text a user would copy, so it is what search runs
// against when wide runes are present.
func (t *Terminal) RowText(y int) (string, []int) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.rowTextLocked(y)
}

func (t *Terminal) rowTextLocked(y int) (string, []int) {
	if y < 0 || y >= t.rows {
		return "", nil
	}
	var runes []rune
	var colMap []int
	for x := 0; x < t.cols; x++ {
		cell := t.GetCellLocked(x, y)
		if cell.Continuation {
			continue
		}
		r := cell.Char
		if r == 0 {
			r = ' '
		}
		runes = append(runes, r)
		colMap = append(colMap, x)
		for _, extra := range cell.Extra {
			runes = append(runes, extra)
			colMap = append(colMap, x)
		}
	}
	// Trailing blank cells are not text.
	end := len(runes)
	for end > 0 && runes[end-1] == ' ' {
		end--
	}
	return string(runes[:end]), colMap[:end]
}

// FindRowMatchesCols is FindRowMatches for a row produced by RowText: it
// matches against the text runes and translates the match back to grid
// columns via colMap. EndCol covers the whole last matched cell (both
// columns of a wide rune).
func FindRowMatchesCols(text string, colMap []int, query string) []SearchMatchInfo {
	if query == "" {
		return nil
	}
	q := lowerRunes([]rune(query))
	r := lowerRunes([]rune(text))
	var matches []SearchMatchInfo
	for x := 0; x+len(q) <= len(r); {
		if matchAt(r, q, x) {
			start := colMap[x]
			end := colMap[x+len(q)-1]
			if w := RuneWidth(r[x+len(q)-1]); w == 2 {
				end++
			}
			matches = append(matches, SearchMatchInfo{StartCol: start, EndCol: end})
			x += len(q)
		} else {
			x++
		}
	}
	return matches
}

// rowTextOf extracts the text of one physical row: continuation cells are
// skipped, zero-width runes (Extra) are included, and colMap[i] is the grid
// column of the i-th rune. Trailing blank cells are NOT trimmed here — the
// caller trims the end of a logical line, so a real space at a wrap point
// survives joining.
func rowTextOf(row []Cell) (string, []int) {
	var runes []rune
	var colMap []int
	for x := 0; x < len(row); x++ {
		cell := row[x]
		if cell.Continuation {
			continue
		}
		r := cell.Char
		if r == 0 {
			r = ' '
		}
		runes = append(runes, r)
		colMap = append(colMap, x)
		for _, extra := range cell.Extra {
			runes = append(runes, extra)
			colMap = append(colMap, x)
		}
	}
	return string(runes), colMap
}

// AbsoluteSearchMatch is one physical row of a search match in the full
// history (scrollback + grid). AbsRow counts from the oldest scrollback row
// (0) through the grid rows; a match spanning wrapped rows is split into
// one entry per physical row so the renderer can highlight each row.
type AbsoluteSearchMatch struct {
	AbsRow   int
	StartCol int
	EndCol   int
}

// FindMatches returns all case-insensitive matches of query in the full
// history: the scrollback lines followed by the visible grid (or just the
// alt grid when the alternate screen is active, which has no scrollback).
// Lines wrapped across physical rows are joined, so a query spanning a
// wrap is found; a match that spans rows is split into one entry per row.
// At most 1000 entries are returned, oldest match first.
func (t *Terminal) FindMatches(query string) []AbsoluteSearchMatch {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if query == "" {
		return nil
	}
	q := lowerRunes([]rune(query))

	total := t.rows
	if !t.isAlt {
		total = len(t.scrollback) + t.rows
	}
	rowAt := func(abs int) ([]Cell, bool) {
		if !t.isAlt && abs < len(t.scrollback) {
			w := false
			if abs < len(t.scrollbackWrapped) {
				w = t.scrollbackWrapped[abs]
			}
			return t.scrollback[abs], w
		}
		g := abs
		if !t.isAlt {
			g = abs - len(t.scrollback)
		}
		grid := t.activeGrid()
		wrapped := t.activeWrapped()
		if g < 0 || g >= len(grid) {
			return nil, false
		}
		w := false
		if g < len(wrapped) {
			w = wrapped[g]
		}
		return grid[g], w
	}

	var matches []AbsoluteSearchMatch
	// Accumulator for the logical line being built: per-rune grid column
	// and absolute row, so matches map back to physical rows.
	var lineRunes []rune
	var lineCols []int
	var lineRows []int
	flush := func() {
		// Only the end of a logical line is padding; trim it.
		end := len(lineRunes)
		for end > 0 && lineRunes[end-1] == ' ' {
			end--
		}
		lineRunes = lineRunes[:end]
		lineCols = lineCols[:end]
		lineRows = lineRows[:end]
		if len(lineRunes) == 0 {
			lineRunes = lineRunes[:0]
			lineCols = lineCols[:0]
			lineRows = lineRows[:0]
			return
		}
		lower := lowerRunes(lineRunes)
		for x := 0; x+len(q) <= len(lower); {
			if !matchAt(lower, q, x) {
				x++
				continue
			}
			// Split the match per physical row.
			for s := x; s < x+len(q); {
				row := lineRows[s]
				e := s
				for e+1 < x+len(q) && lineRows[e+1] == row {
					e++
				}
				endCol := lineCols[e]
				if w := RuneWidth(lineRunes[e]); w == 2 {
					endCol++
				}
				matches = append(matches, AbsoluteSearchMatch{AbsRow: row, StartCol: lineCols[s], EndCol: endCol})
				if len(matches) >= 1000 {
					return
				}
				s = e + 1
			}
			x += len(q)
		}
		lineRunes = lineRunes[:0]
		lineCols = lineCols[:0]
		lineRows = lineRows[:0]
	}

	for abs := 0; abs < total; abs++ {
		row, wrapped := rowAt(abs)
		if row == nil {
			continue
		}
		if abs > 0 && !wrapped {
			flush()
			if len(matches) >= 1000 {
				return matches
			}
		}
		text, colMap := rowTextOf(row)
		for i, r := range []rune(text) {
			lineRunes = append(lineRunes, r)
			lineCols = append(lineCols, colMap[i])
			lineRows = append(lineRows, abs)
		}
	}
	flush()
	return matches
}

func (t *Terminal) Theme() *Theme {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.ThemeLocked()
}

func (t *Terminal) ThemeLocked() *Theme {
	if t.theme == nil {
		return ThemeTokyoNight
	}
	return t.theme
}

func (t *Terminal) SetTheme(theme *Theme) {
	if theme == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	t.theme = theme

	t.currFG = ColorDefaultFG
	t.currBG = ColorDefaultBG
	t.currFGKind = ColorKindDefault
	t.currBGKind = ColorKindDefault

	// L6: remap by how the colour was specified, not by value matching.
	// Palette entries (ANSI16/ANSI256) are re-resolved against the new
	// theme; explicit RGB values and defaults are left exactly as they
	// are — the old value-matching code demoted bright colours whenever
	// two palette entries happened to share a value, and could remap an
	// explicit RGB that coincided with a palette entry.
	remap := func(c Color, kind ColorKind, idx int) Color {
		switch kind {
		case ColorKindANSI16:
			if idx >= 0 && idx < 16 {
				return theme.ANSI[idx]
			}
		case ColorKindANSI256:
			return t.ansi256(idx)
		}
		return c
	}
	remapCell := func(cell *Cell) {
		cell.FG = remap(cell.FG, cell.FGKind, cell.FGIdx)
		cell.BG = remap(cell.BG, cell.BGKind, cell.BGIdx)
	}

	for r := range t.lines {
		for c := range t.lines[r] {
			remapCell(&t.lines[r][c])
		}
	}
	for r := range t.altLines {
		for c := range t.altLines[r] {
			remapCell(&t.altLines[r][c])
		}
	}
	for r := range t.scrollback {
		for c := range t.scrollback[r] {
			remapCell(&t.scrollback[r][c])
		}
	}
	// L7: the DECSC-saved colours are part of the terminal state too —
	// they used to keep the old theme's values after a theme change.
	t.savedFG = remap(t.savedFG, t.savedFGKind, t.savedFGIdx)
	t.savedBG = remap(t.savedBG, t.savedBGKind, t.savedBGIdx)
}

// ansi256 resolves a 256-colour index against the active theme: indices
// 0-15 are the theme palette, the rest are the standard xterm cube/greys.
// Out-of-range indices are clamped instead of resetting the foreground
// (L5); the old package-level ANSI256 ignored the active theme entirely.
func (t *Terminal) ansi256(index int) Color {
	if index < 0 {
		index = 0
	}
	if index > 255 {
		index = 255
	}
	if index < 16 {
		return t.themeANSI(index)
	}
	if index < 232 {
		idx := index - 16
		r := byte((idx / 36) * 51)
		g := byte(((idx % 36) / 6) * 51)
		b := byte((idx % 6) * 51)
		return RGB(r, g, b)
	}
	gray := byte(8 + (index-232)*10)
	return RGB(gray, gray, gray)
}

func (t *Terminal) themeANSI(idx int) Color {
	if t.theme != nil && idx >= 0 && idx < 16 {
		return t.theme.ANSI[idx]
	}
	if idx >= 0 && idx < 16 {
		return ansi16[idx]
	}
	return ColorDefaultFG
}

func (t *Terminal) activeGrid() [][]Cell {
	if t.isAlt {
		return t.altLines
	}
	return t.lines
}

// activeWrapped returns the wrapped flags parallel to the active grid.
// The returned slice shares the grid's backing array, so in-place updates
// to it move with the rows.
func (t *Terminal) activeWrapped() []bool {
	if t.isAlt {
		return t.altWrapped
	}
	return t.linesWrapped
}

func (t *Terminal) Scroll(delta int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.isAlt {
		return
	}

	t.scrollOff += delta
	if t.scrollOff > len(t.scrollback) {
		t.scrollOff = len(t.scrollback)
	}
	if t.scrollOff < 0 {
		t.scrollOff = 0
	}
	if t.sel.Mode != SelectionModeAll {
		t.sel.Active = false
	}
}

func (t *Terminal) ScrollKeepSelection(delta int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.isAlt {
		return
	}

	if t.sel.Mode == SelectionModeAll {
		return
	}

	oldOff := t.scrollOff
	t.scrollOff += delta
	if t.scrollOff > len(t.scrollback) {
		t.scrollOff = len(t.scrollback)
	}
	if t.scrollOff < 0 {
		t.scrollOff = 0
	}

	actualDelta := t.scrollOff - oldOff
	if actualDelta != 0 && t.sel.Active {
		t.sel.StartY += actualDelta
		t.sel.OrigStartY += actualDelta
		t.sel.EndY += actualDelta
		// Clamping Start/End independently used to collapse a multi-row
		// selection into a single row when it scrolled past an edge.
		// The selection now keeps its shape (out-of-view rows simply do
		// not render); it is cleared only once it has scrolled entirely
		// out of the viewport.
		_, sY, _, eY := t.sel.Normalized()
		if eY < 0 || sY >= t.rows {
			t.sel.Active = false
		}
	}
}

func (t *Terminal) SetScrollOff(off int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.isAlt {
		return
	}

	t.scrollOff = off
	if t.scrollOff > len(t.scrollback) {
		t.scrollOff = len(t.scrollback)
	}
	if t.scrollOff < 0 {
		t.scrollOff = 0
	}
}

func (t *Terminal) ScrollToTop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.isAlt {
		return
	}

	t.scrollOff = len(t.scrollback)
	if t.sel.Mode != SelectionModeAll {
		t.sel.Active = false
	}
}

func (t *Terminal) ResetScroll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.scrollOff != 0 {
		t.scrollOff = 0
		if t.sel.Mode != SelectionModeAll {
			t.sel.Active = false
		}
	}
}

// logicalLines joins physical rows into logical lines using the wrapped
// flags: a row whose flag is set continues the previous line.
func logicalLines(rows [][]Cell, wrapped []bool) [][]Cell {
	var lines [][]Cell
	for i, row := range rows {
		if i > 0 && i < len(wrapped) && wrapped[i] && len(lines) > 0 {
			lines[len(lines)-1] = append(lines[len(lines)-1], row...)
		} else {
			line := make([]Cell, len(row))
			copy(line, row)
			lines = append(lines, line)
		}
	}
	return lines
}

// reflowLines re-splits logical lines into physical rows of width cols. A
// wide rune is never split across a row boundary: if its lead cell would
// be the last cell of a row, the pair moves to the next row. It returns
// the rows and their wrapped flags (true for every row but the first of a
// logical line).
func reflowLines(lines [][]Cell, cols int) ([][]Cell, []bool) {
	var rows [][]Cell
	var wrapped []bool
	for _, line := range lines {
		// Drop trailing blank cells of the logical line: they are padding,
		// and keeping them would grow the history with empty rows. Only
		// exactly-empty cells are trimmed — a trailing space with a
		// background colour or attributes is real content.
		end := len(line)
		for end > 0 && line[end-1].Equal(EmptyCell()) {
			end--
		}
		line = line[:end]
		if len(line) == 0 {
			rows = append(rows, make([]Cell, cols))
			for i := range rows[len(rows)-1] {
				rows[len(rows)-1][i] = EmptyCell()
			}
			wrapped = append(wrapped, false)
			continue
		}
		first := true
		for len(line) > 0 {
			take := cols
			if take > len(line) {
				take = len(line)
			}
			// Do not leave a wide lead as the last cell of the row.
			if take < len(line) && take > 0 && line[take-1].Wide {
				take--
			}
			if take == 0 {
				// A 1-column terminal cannot hold a wide rune at all;
				// take the lead alone and let fixRowWide demote it.
				take = 1
			}
			row := make([]Cell, cols)
			for i := range row {
				row[i] = EmptyCell()
			}
			copy(row, line[:take])
			rows = append(rows, row)
			wrapped = append(wrapped, !first)
			first = false
			line = line[take:]
		}
	}
	return rows, wrapped
}

func (t *Terminal) Resize(newCols, newRows int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if newCols == t.cols && newRows == t.rows {
		return
	}
	if newCols <= 0 || newRows <= 0 {
		return
	}

	// L9: reflow instead of truncating. Logical lines (scrollback + grid,
	// joined via the wrapped flags) are re-split at the new width, so
	// narrowing no longer drops text off the right edge and the scrollback
	// is re-wrapped to the new width too. The newest rows stay on screen;
	// older rows move into (or stay in) the scrollback, capped at
	// maxHistory.
	sbLines := logicalLines(t.scrollback, t.scrollbackWrapped)
	gridLines := logicalLines(t.lines, t.linesWrapped)
	allLines := sbLines
	// A logical line can span the scrollback/grid boundary: when the
	// first grid row is a wrapped continuation it belongs to the last
	// scrollback line.
	if len(gridLines) > 0 && len(t.linesWrapped) > 0 && t.linesWrapped[0] && len(allLines) > 0 {
		allLines[len(allLines)-1] = append(allLines[len(allLines)-1], gridLines[0]...)
		gridLines = gridLines[1:]
	}
	allLines = append(allLines, gridLines...)
	reflowed, reflowedWrapped := reflowLines(allLines, newCols)

	// Keep at most newRows on screen; the rest is scrollback.
	var newScrollback [][]Cell
	var newScrollbackWrapped []bool
	if len(reflowed) > newRows {
		newScrollback = reflowed[:len(reflowed)-newRows]
		newScrollbackWrapped = reflowedWrapped[:len(reflowed)-newRows]
		if len(newScrollback) > t.maxHistory {
			newScrollback = newScrollback[len(newScrollback)-t.maxHistory:]
			newScrollbackWrapped = newScrollbackWrapped[len(newScrollbackWrapped)-t.maxHistory:]
		}
		reflowed = reflowed[len(reflowed)-newRows:]
		reflowedWrapped = reflowedWrapped[len(reflowedWrapped)-newRows:]
	}
	newLines := make([][]Cell, newRows)
	newLinesWrapped := make([]bool, newRows)
	for r := 0; r < newRows; r++ {
		if r < len(reflowed) {
			newLines[r] = reflowed[r]
			newLinesWrapped[r] = reflowedWrapped[r]
		} else {
			newLines[r] = make([]Cell, newCols)
			for c := range newLines[r] {
				newLines[r][c] = EmptyCell()
			}
		}
	}

	// The alternate screen has no scrollback: reflow it on its own and
	// keep only the newest rows.
	altLines := logicalLines(t.altLines, t.altWrapped)
	altReflowed, altReflowedWrapped := reflowLines(altLines, newCols)
	if len(altReflowed) > newRows {
		altReflowed = altReflowed[len(altReflowed)-newRows:]
		altReflowedWrapped = altReflowedWrapped[len(altReflowedWrapped)-newRows:]
	}
	newAlt := make([][]Cell, newRows)
	newAltWrapped := make([]bool, newRows)
	for r := 0; r < newRows; r++ {
		if r < len(altReflowed) {
			newAlt[r] = altReflowed[r]
			newAltWrapped[r] = altReflowedWrapped[r]
		} else {
			newAlt[r] = make([]Cell, newCols)
			for c := range newAlt[r] {
				newAlt[r][c] = EmptyCell()
			}
		}
	}

	t.cols = newCols
	t.rows = newRows
	t.lines = newLines
	t.altLines = newAlt
	t.linesWrapped = newLinesWrapped
	t.altWrapped = newAltWrapped
	t.scrollback = newScrollback
	t.scrollbackWrapped = newScrollbackWrapped
	// The history backing stores were built for the old widths; let
	// scrollUp rebuild them lazily.
	t.historyStore = nil
	t.historyStoreWrapped = nil
	if t.scrollOff > len(t.scrollback) {
		t.scrollOff = len(t.scrollback)
	}

	for _, row := range t.lines {
		t.fixRowWide(row)
	}
	for _, row := range t.altLines {
		t.fixRowWide(row)
	}

	t.scrollTop = 0
	t.scrollBottom = newRows - 1

	if t.cursorY >= newRows {
		t.cursorY = newRows - 1
	}
	if t.cursorX >= newCols {
		t.cursorX = newCols - 1
	}
	t.wrapNext = false
	t.sel.Active = false
}

// isUTF8Prefix reports whether p could be the start of a valid rune
// that was cut short by the end of the chunk: the first byte opens a
// 2/3/4-byte sequence and every byte present so far is a valid
// continuation byte. Anything else is invalid, not truncated (L14).
func isUTF8Prefix(p []byte) bool {
	if len(p) == 0 {
		return false
	}
	var need int
	switch b := p[0]; {
	case b >= 0xC2 && b <= 0xDF:
		need = 2
	case b >= 0xE0 && b <= 0xEF:
		need = 3
	case b >= 0xF0 && b <= 0xF4:
		need = 4
	default:
		return false
	}
	if len(p) >= need {
		return false
	}
	for _, c := range p[1:] {
		if c < 0x80 || c > 0xBF {
			return false
		}
	}
	return true
}

func (t *Terminal) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	n := len(p)
	if len(t.pendingUTF8) != 0 {
		p = append(t.pendingUTF8, p...)
		t.pendingUTF8 = nil
	}
	idx := 0
	for idx < len(p) {
		if !utf8.FullRune(p[idx:]) {
			if isUTF8Prefix(p[idx:]) {
				t.pendingUTF8 = append([]byte(nil), p[idx:]...)
				break
			}
			// Invalid byte, not a truncated rune: show it as U+FFFD
			// now instead of buffering it forever.
			t.processRune(utf8.RuneError)
			idx++
			continue
		}
		r, size := utf8.DecodeRune(p[idx:])
		idx += size
		t.processRune(r)
	}

	return n, nil
}

// Flush emits a single U+FFFD for a truncated UTF-8 sequence still held
// from the last Write (e.g. when the PTY closes mid-rune) instead of
// silently dropping it (L14).
func (t *Terminal) Flush() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.pendingUTF8) != 0 {
		t.pendingUTF8 = nil
		t.processRune(utf8.RuneError)
	}
}

func (t *Terminal) processRune(r rune) {
	switch t.state {
	case stateNormal:
		switch r {
		case '\x1b':
			t.state = stateEscape
		case '\n':
			t.lineFeed()
			t.wrapNext = false
		case '\v', '\f':
			// L3: vertical tab / form feed are line feeds, not no-ops.
			t.lineFeed()
			t.wrapNext = false
		case '\r':
			t.cursorX = 0
			t.wrapNext = false
		case '\b':
			if t.cursorX > 0 {
				t.cursorX--
			}
			t.wrapNext = false
		case '\t':
			t.cursorX = (t.cursorX/8 + 1) * 8
			if t.cursorX >= t.cols {
				t.cursorX = t.cols - 1
			}
			t.wrapNext = false
		case '\a':
		case '\x00':
		default:
			// L3: DEL and the C1 controls (0x80-0x9F) are not printable;
			// they used to reach putChar and show up as glyphs.
			if r >= ' ' && r != 0x7f && !(r >= 0x80 && r <= 0x9f) {
				t.putChar(r)
			}
		}

	case stateEscape:
		switch r {
		case '[':
			t.state = stateCSI
			t.csiParams = t.csiParams[:0]
			t.csiPrivate = false
			t.csiLeader = 0
			t.csiIntermediates = ""
		case ']':
			t.state = stateOSC
			t.oscBuffer = nil
		case '(', ')', '*', '+':
			t.state = stateCharset
		case '7':
			t.saveCursor()
			t.state = stateNormal
		case '8':
			t.restoreCursor()
			t.state = stateNormal
		case 'M':
			t.reverseIndex()
			t.state = stateNormal
		case '=', '>':
			t.state = stateNormal
		default:
			t.state = stateNormal
		}

	case stateCharset:
		t.state = stateNormal

	case stateCSI:
		if r == '\x1b' {
			t.state = stateEscape
			return
		}
		if r == '\x18' || r == '\x1a' {
			t.state = stateNormal
			return
		}
		if r == '?' || r == '>' || r == '=' {
			t.csiLeader = r
			if r == '?' {
				t.csiPrivate = true
			}
			return
		}
		if (r >= '0' && r <= '9') || r == ';' || r == ':' {
			// L2: ':' is a sub-parameter separator (e.g. ESC[38:5:196m);
			// treat it like ';' so the sequence parses instead of
			// printing literally.
			if len(t.csiParams) == 0 {
				t.csiParams = append(t.csiParams, "")
			}
			last := len(t.csiParams) - 1
			if r == ';' || r == ':' {
				if len(t.csiParams) < 32 {
					t.csiParams = append(t.csiParams, "")
				}
			} else if len(t.csiParams[last]) < 16 {
				t.csiParams[last] += string(r)
			}
			return
		}
		if r >= 0x20 && r <= 0x2f {
			// L2: intermediate bytes (space, !, ", #, $, %, &, ', (, ),
			// *, +, ,, -, ., /) are part of the sequence, not the final
			// byte — collect them and keep parsing.
			if len(t.csiIntermediates) < 8 {
				t.csiIntermediates += string(r)
			}
			return
		}

		t.executeCSI(r)
		t.state = stateNormal

	case stateOSC:
		if r == '\x18' || r == '\x1a' {
			t.state = stateNormal
			return
		}
		switch r {
		case '\a':
			t.executeOSC()
			t.state = stateNormal
		case '\x1b':
			t.state = stateOSCEscape
		default:
			if len(t.oscBuffer) < 4096 {
				t.oscBuffer = append(t.oscBuffer, r)
			} else {
				// Over-long OSC: swallow the rest up to the terminator
				// instead of dropping back to stateNormal, where the
				// remainder would leak onto the screen as typed text.
				t.oscBuffer = nil
				t.state = stateOSCDiscard
			}
		}

	case stateOSCDiscard:
		// Discard the over-long sequence without executing it (its
		// content was truncated) and without printing any of it.
		switch r {
		case '\a', '\x18', '\x1a':
			t.state = stateNormal
		case '\x1b':
			t.state = stateOSCDiscardEscape
		}

	case stateOSCDiscardEscape:
		if r == '\\' {
			t.state = stateNormal
		} else {
			// ESC starts a new sequence; reprocess this rune as an escape.
			t.state = stateEscape
			t.processRune(r)
		}

	case stateOSCEscape:
		if r == '\\' {
			t.executeOSC()
			t.state = stateNormal
		} else {
			t.executeOSC()
			t.state = stateEscape
			t.processRune(r)
		}
	}
}

func (t *Terminal) putChar(r rune) {
	// Zero-width runes (combining marks, ZWNJ/ZWJ, variation selectors)
	// attach to the previous cell's grapheme and never advance the cursor;
	// anything else zero-width (controls) is dropped.
	if RuneWidth(r) == 0 {
		if IsZeroWidthAttachable(r) {
			t.attachZeroWidth(r)
		}
		return
	}

	if t.wrapNext {
		if t.autoWrap {
			t.cursorX = 0
			t.lineFeed()
			// The new row continues the logical line that wrapped.
			t.activeWrapped()[t.cursorY] = true
		}
		t.wrapNext = false
	}

	w := RuneWidth(r)
	grid := t.activeGrid()

	// A wide rune that does not fit in the remaining columns wraps to the
	// next line instead of being split across the edge.
	if w == 2 && t.cursorX == t.cols-1 {
		if t.autoWrap {
			t.cursorX = 0
			t.lineFeed()
			t.activeWrapped()[t.cursorY] = true
			grid = t.activeGrid()
		} else {
			return
		}
	}

	if t.cursorY >= 0 && t.cursorY < t.rows && t.cursorX >= 0 && t.cursorX < t.cols {
		// Overwriting half of a wide pair must clear the other half, or
		// the grid would keep a dangling lead/continuation cell.
		t.clearWidePairAt(grid, t.cursorY, t.cursorX)
		grid[t.cursorY][t.cursorX] = Cell{
			Char:      r,
			FG:        t.currFG,
			BG:        t.currBG,
			FGKind:    t.currFGKind,
			BGKind:    t.currBGKind,
			FGIdx:     t.currFGIdx,
			BGIdx:     t.currBGIdx,
			Bold:      t.currBold,
			Underline: t.currUnderline,
			Inverse:   t.currInverse,
			Wide:      w == 2,
		}
		if w == 2 && t.cursorX+1 < t.cols {
			grid[t.cursorY][t.cursorX+1] = Cell{
				Continuation: true,
				FG:           t.currFG,
				BG:           t.currBG,
				FGKind:       t.currFGKind,
				BGKind:       t.currBGKind,
				FGIdx:        t.currFGIdx,
				BGIdx:        t.currBGIdx,
			}
		}
	}

	t.cursorX += w
	if t.cursorX >= t.cols {
		t.cursorX = t.cols - 1
		if t.autoWrap {
			t.wrapNext = true
		}
	}
}

// attachZeroWidth appends r to the grapheme of the cell just before the
// cursor (the cell the previous character was written to), so the text
// round-trips on copy/paste and search while the cursor stays put.
func (t *Terminal) attachZeroWidth(r rune) {
	grid := t.activeGrid()
	if t.cursorY < 0 || t.cursorY >= t.rows || t.cursorX <= 0 {
		return
	}
	x := t.cursorX - 1
	if grid[t.cursorY][x].Continuation {
		x--
		if x < 0 {
			return
		}
	}
	prev := &grid[t.cursorY][x]
	if prev.Char == 0 || prev.Char == ' ' {
		return // nothing meaningful to attach to
	}
	// Copy on append: cells are copied by value on row shifts, so the
	// backing array may be shared with another cell.
	extra := make([]rune, len(prev.Extra)+1)
	copy(extra, prev.Extra)
	extra[len(prev.Extra)] = r
	prev.Extra = extra
}

// clearWidePairAt clears both halves of a wide pair touching cell (y, x):
// if the cell is a continuation, its lead is cleared too, and if it is a
// wide lead, its continuation is cleared too.
func (t *Terminal) clearWidePairAt(grid [][]Cell, y, x int) {
	if y < 0 || y >= len(grid) || x < 0 || x >= len(grid[y]) {
		return
	}
	if grid[y][x].Continuation && x > 0 {
		grid[y][x-1] = t.emptyCell()
	} else if grid[y][x].Wide && x+1 < len(grid[y]) {
		grid[y][x+1] = t.emptyCell()
	}
}

// fixRowWide repairs dangling wide halves in a row (a lead cell whose
// continuation is gone, or a continuation with no lead), e.g. after an
// erase, insert/delete or resize cut a wide rune in half. A dangling lead
// becomes a normal single cell; a dangling continuation becomes a space.
func (t *Terminal) fixRowWide(row []Cell) {
	for x := 0; x < len(row); x++ {
		switch {
		case row[x].Continuation:
			if x == 0 || !row[x-1].Wide {
				row[x] = t.emptyCell()
			}
		case row[x].Wide:
			if x+1 >= len(row) || !row[x+1].Continuation {
				row[x].Wide = false
			}
		}
	}
}

func (t *Terminal) lineFeed() {
	// Only scroll when the cursor is exactly on the bottom of the scroll
	// region. Below the region (e.g. under DECSTBM) the cursor just moves
	// down, clamped to the last row — scrolling there would corrupt the
	// content above, e.g. inside nano.
	if t.cursorY == t.scrollBottom {
		t.scrollUp(1)
	} else if t.cursorY < t.rows-1 {
		t.cursorY++
	}
}

func (t *Terminal) scrollUp(count int) {
	grid := t.activeGrid()
	wrapped := t.activeWrapped()

	for c := 0; c < count; c++ {
		recycled := grid[t.scrollTop]
		if t.scrollTop == 0 && !t.isAlt && t.maxHistory > 0 {
			// The top row (and its wrapped flag) leaves the grid for the
			// scrollback; capture the flag before the shift below.
			enteringWrapped := wrapped[0]
			if len(t.scrollback) == t.maxHistory {
				recycled = t.scrollback[0]
			} else {
				recycled = nil
			}
			// Compact the sliding history window into reusable storage.
			if len(t.scrollback) == cap(t.scrollback) && len(t.scrollback) >= t.maxHistory {
				if len(t.historyStore) != 2*t.maxHistory {
					t.historyStore = make([][]Cell, 2*t.maxHistory)
				}
				copy(t.historyStore, t.scrollback)
				clear(t.historyStore[len(t.scrollback):])
				t.scrollback = t.historyStore[:len(t.scrollback)]
				if len(t.historyStoreWrapped) != 2*t.maxHistory {
					t.historyStoreWrapped = make([]bool, 2*t.maxHistory)
				}
				copy(t.historyStoreWrapped, t.scrollbackWrapped)
				clear(t.historyStoreWrapped[len(t.scrollbackWrapped):])
				t.scrollbackWrapped = t.historyStoreWrapped[:len(t.scrollbackWrapped)]
			}
			t.scrollback = append(t.scrollback, grid[0])
			t.scrollbackWrapped = append(t.scrollbackWrapped, enteringWrapped)
			if len(t.scrollback) > t.maxHistory {
				t.scrollback[0] = nil
				t.scrollback = t.scrollback[1:]
				t.scrollbackWrapped = t.scrollbackWrapped[1:]
			} else if t.scrollOff > 0 {
				t.scrollOff++
			}
		}

		for r := t.scrollTop; r < t.scrollBottom; r++ {
			grid[r] = grid[r+1]
			wrapped[r] = wrapped[r+1]
		}

		if len(recycled) != t.cols {
			recycled = t.makeRow()
		} else {
			for i := range recycled {
				recycled[i] = EmptyCell()
			}
		}
		grid[t.scrollBottom] = recycled
		wrapped[t.scrollBottom] = false
	}
	if t.scrollOff > len(t.scrollback) {
		t.scrollOff = len(t.scrollback)
	}
}

func (t *Terminal) reverseIndex() {
	// Only scroll when the cursor is exactly on the top of the scroll
	// region. Above the region the cursor just moves up, clamped to the
	// first row — mirroring lineFeed.
	if t.cursorY == t.scrollTop {
		grid := t.activeGrid()
		wrapped := t.activeWrapped()
		recycled := grid[t.scrollBottom]
		for r := t.scrollBottom; r > t.scrollTop; r-- {
			grid[r] = grid[r-1]
			wrapped[r] = wrapped[r-1]
		}
		for i := range recycled {
			recycled[i] = EmptyCell()
		}
		grid[t.scrollTop] = recycled
		wrapped[t.scrollTop] = false
	} else if t.cursorY > 0 {
		t.cursorY--
	}
}

func (t *Terminal) saveCursor() {
	t.savedX = t.cursorX
	t.savedY = t.cursorY
	t.savedWrapNext = t.wrapNext
	t.savedFG = t.currFG
	t.savedBG = t.currBG
	t.savedFGKind = t.currFGKind
	t.savedBGKind = t.currBGKind
	t.savedFGIdx = t.currFGIdx
	t.savedBGIdx = t.currBGIdx
	t.savedBold = t.currBold
	t.savedUnderline = t.currUnderline
	t.savedInverse = t.currInverse
}

func (t *Terminal) restoreCursor() {
	t.cursorX = t.savedX
	t.cursorY = t.savedY
	t.wrapNext = t.savedWrapNext
	if t.cursorX >= t.cols {
		t.cursorX = t.cols - 1
	}
	if t.cursorX < 0 {
		t.cursorX = 0
	}
	if t.cursorY >= t.rows {
		t.cursorY = t.rows - 1
	}
	if t.cursorY < 0 {
		t.cursorY = 0
	}
	t.currFG = t.savedFG
	t.currBG = t.savedBG
	t.currFGKind = t.savedFGKind
	t.currBGKind = t.savedBGKind
	t.currFGIdx = t.savedFGIdx
	t.currBGIdx = t.savedBGIdx
	t.currBold = t.savedBold
	t.currUnderline = t.savedUnderline
	t.currInverse = t.savedInverse
}

func (t *Terminal) executeCSI(cmd rune) {
	var argBuffer [32]int
	args := argBuffer[:len(t.csiParams)]
	for i, p := range t.csiParams {
		val, _ := strconv.Atoi(p)
		args[i] = val
	}

	arg := func(idx, defaultVal int) int {
		if idx < len(args) && args[idx] > 0 {
			return args[idx]
		}
		return defaultVal
	}

	grid := t.activeGrid()
	// wrapped parallels grid row-for-row; every case below that moves rows
	// must move these flags the same way.
	wrapped := t.activeWrapped()

	switch cmd {
	case 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'f', 'd', 'J', 'K':
		t.wrapNext = false
	}

	switch cmd {
	case 'A':
		n := arg(0, 1)
		if n > t.rows {
			n = t.rows
		}
		t.cursorY -= n
		if t.cursorY < 0 {
			t.cursorY = 0
		}

	case 'B':
		n := arg(0, 1)
		if n > t.rows {
			n = t.rows
		}
		t.cursorY += n
		if t.cursorY >= t.rows {
			t.cursorY = t.rows - 1
		}

	case 'C':
		n := arg(0, 1)
		if n > t.cols {
			n = t.cols
		}
		t.cursorX += n
		if t.cursorX >= t.cols {
			t.cursorX = t.cols - 1
		}

	case 'D':
		n := arg(0, 1)
		if n > t.cols {
			n = t.cols
		}
		t.cursorX -= n
		if t.cursorX < 0 {
			t.cursorX = 0
		}

	case 'E':
		n := arg(0, 1)
		if n > t.rows {
			n = t.rows
		}
		t.cursorY += n
		if t.cursorY >= t.rows {
			t.cursorY = t.rows - 1
		}
		t.cursorX = 0

	case 'F':
		n := arg(0, 1)
		if n > t.rows {
			n = t.rows
		}
		t.cursorY -= n
		if t.cursorY < 0 {
			t.cursorY = 0
		}
		t.cursorX = 0

	case 'G':
		col := arg(0, 1) - 1
		if col < 0 {
			col = 0
		}
		if col >= t.cols {
			col = t.cols - 1
		}
		t.cursorX = col

	case 'H', 'f':
		row := arg(0, 1) - 1
		col := arg(1, 1) - 1
		if row < 0 {
			row = 0
		}
		if row >= t.rows {
			row = t.rows - 1
		}
		if col < 0 {
			col = 0
		}
		if col >= t.cols {
			col = t.cols - 1
		}
		t.cursorX = col
		t.cursorY = row

	case 'J':
		mode := 0
		if len(args) > 0 {
			mode = args[0]
		}
		switch mode {
		case 0:
			for c := t.cursorX; c < t.cols; c++ {
				grid[t.cursorY][c] = t.emptyCell()
			}
			t.fixRowWide(grid[t.cursorY])
			for r := t.cursorY + 1; r < t.rows; r++ {
				grid[r] = t.emptyRow()
				wrapped[r] = false
			}
		case 1:
			for r := 0; r < t.cursorY; r++ {
				grid[r] = t.emptyRow()
				wrapped[r] = false
			}
			for c := 0; c <= t.cursorX && c < t.cols; c++ {
				grid[t.cursorY][c] = t.emptyCell()
			}
			t.fixRowWide(grid[t.cursorY])
		case 2:
			for r := 0; r < t.rows; r++ {
				grid[r] = t.emptyRow()
				wrapped[r] = false
			}
		case 3:
			t.scrollback = nil
			t.scrollbackWrapped = nil
			t.historyStore = nil
			t.historyStoreWrapped = nil
			t.scrollOff = 0
		}

	case 'K':
		mode := 0
		if len(args) > 0 {
			mode = args[0]
		}
		switch mode {
		case 0:
			for c := t.cursorX; c < t.cols; c++ {
				grid[t.cursorY][c] = t.emptyCell()
			}
		case 1:
			for c := 0; c <= t.cursorX && c < t.cols; c++ {
				grid[t.cursorY][c] = t.emptyCell()
			}
		case 2:
			for c := 0; c < t.cols; c++ {
				grid[t.cursorY][c] = t.emptyCell()
			}
		}
		t.fixRowWide(grid[t.cursorY])

	case 'd':
		row := arg(0, 1) - 1
		if row < 0 {
			row = 0
		}
		if row >= t.rows {
			row = t.rows - 1
		}
		t.cursorY = row

	case 'S':
		n := arg(0, 1)
		if n > t.rows {
			n = t.rows
		}
		t.scrollUp(n)

	case 'T':
		n := arg(0, 1)
		if n > t.rows {
			n = t.rows
		}
		for i := 0; i < n; i++ {
			for r := t.scrollBottom; r > t.scrollTop; r-- {
				grid[r] = grid[r-1]
				wrapped[r] = wrapped[r-1]
			}
			grid[t.scrollTop] = t.emptyRow()
			wrapped[t.scrollTop] = false
		}

	case '@':
		n := arg(0, 1)
		if n > t.cols {
			n = t.cols
		}
		row := grid[t.cursorY]
		for c := t.cols - 1; c >= t.cursorX; c-- {
			if c-n >= t.cursorX {
				row[c] = row[c-n]
			} else {
				row[c] = t.emptyCell()
			}
		}
		t.fixRowWide(row)

	case 'X':
		n := arg(0, 1)
		if n > t.cols {
			n = t.cols
		}
		row := grid[t.cursorY]
		for c := t.cursorX; c < t.cursorX+n && c < t.cols; c++ {
			row[c] = t.emptyCell()
		}
		t.fixRowWide(row)

	case 's':
		t.saveCursor()

	case 'u':
		t.restoreCursor()

	case 'L':
		if t.cursorY >= t.scrollTop && t.cursorY <= t.scrollBottom {
			n := arg(0, 1)
			if n > t.rows {
				n = t.rows
			}
			for i := 0; i < n; i++ {
				for r := t.scrollBottom; r > t.cursorY; r-- {
					grid[r] = grid[r-1]
					wrapped[r] = wrapped[r-1]
				}
				grid[t.cursorY] = t.emptyRow()
				wrapped[t.cursorY] = false
			}
		}

	case 'M':
		if t.cursorY >= t.scrollTop && t.cursorY <= t.scrollBottom {
			n := arg(0, 1)
			if n > t.rows {
				n = t.rows
			}
			for i := 0; i < n; i++ {
				for r := t.cursorY; r < t.scrollBottom; r++ {
					grid[r] = grid[r+1]
					wrapped[r] = wrapped[r+1]
				}
				grid[t.scrollBottom] = t.emptyRow()
				wrapped[t.scrollBottom] = false
			}
		}

	case 'P':
		n := arg(0, 1)
		if n > t.cols {
			n = t.cols
		}
		row := grid[t.cursorY]
		for c := t.cursorX; c < t.cols; c++ {
			if c+n < t.cols {
				row[c] = row[c+n]
			} else {
				row[c] = t.emptyCell()
			}
		}
		t.fixRowWide(row)

	case 'r':
		top := arg(0, 1) - 1
		bottom := arg(1, t.rows) - 1
		// L8: a single-line region (top == bottom, e.g. ESC[5;5r) is
		// valid; only an inverted region resets to the full screen.
		if top >= 0 && bottom < t.rows && top <= bottom {
			t.scrollTop = top
			t.scrollBottom = bottom
		} else {
			t.scrollTop = 0
			t.scrollBottom = t.rows - 1
		}
		t.cursorX = 0
		t.cursorY = 0

	case 'm':
		t.executeSGR(args)

	case 'n':
		mode := arg(0, 0)
		switch mode {
		case 6:
			cy := t.cursorY + 1
			if cy < 1 {
				cy = 1
			} else if cy > t.rows {
				cy = t.rows
			}
			cx := t.cursorX + 1
			if cx < 1 {
				cx = 1
			} else if cx > t.cols {
				cx = t.cols
			}
			resp := fmt.Sprintf("\x1b[%d;%dR", cy, cx)
			select {
			case t.ResponseChan <- []byte(resp):
			default:
			}
		case 5:
			select {
			case t.ResponseChan <- []byte("\x1b[0n"):
			default:
			}
		}

	case 'c':
		if t.csiLeader == '>' {
			select {
			case t.ResponseChan <- []byte("\x1b[>0;10;1c"):
			default:
			}
		} else {
			select {
			case t.ResponseChan <- []byte("\x1b[?62;c"):
			default:
			}
		}

	case 'h', 'l':
		enable := (cmd == 'h')
		if t.csiPrivate {
			for _, mode := range args {
				switch mode {
				case 1:
					t.appCursor = enable
				case 7:
					t.autoWrap = enable
					t.wrapNext = false
				case 25:
					t.cursorVisible = enable
				case 1047, 47:
					t.isAlt = enable
					t.scrollTop = 0
					t.scrollBottom = t.rows - 1
					t.scrollOff = 0
				case 1048:
					if enable {
						t.saveCursor()
					} else {
						t.restoreCursor()
					}
				case 1049:
					if enable {
						t.saveCursor()
						t.isAlt = true
						for r := 0; r < t.rows; r++ {
							t.altLines[r] = t.makeRow()
						}
						clear(t.altWrapped)
						t.scrollTop = 0
						t.scrollBottom = t.rows - 1
						t.scrollOff = 0
					} else {
						t.isAlt = false
						t.appCursor = false
						t.mouseTracking = false
						t.mouseSGR = false
						t.restoreCursor()
						t.scrollTop = 0
						t.scrollBottom = t.rows - 1
						t.scrollOff = 0
					}
				case 1000, 1002:
					t.mouseTracking = enable
				case 1006:
					t.mouseSGR = enable
				case 2004:
					t.bracketedPaste = enable
				}
			}
		}
	}
}

// clampByte clamps an SGR colour component to 0-255 (L4): converting
// with byte() used to wrap, so 300 became 44.
func clampByte(v int) byte {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return byte(v)
}

func (t *Terminal) executeSGR(args []int) {
	if len(args) == 0 {
		args = []int{0}
	}

	// setFG/setBG record both the resolved colour and how it was
	// specified, so a later theme change can remap palette colours while
	// leaving explicit RGB alone (L6).
	setFG := func(c Color, kind ColorKind, idx int) {
		t.currFG, t.currFGKind, t.currFGIdx = c, kind, idx
	}
	setBG := func(c Color, kind ColorKind, idx int) {
		t.currBG, t.currBGKind, t.currBGIdx = c, kind, idx
	}

	for i := 0; i < len(args); i++ {
		code := args[i]
		switch {
		case code == 0:
			setFG(ColorDefaultFG, ColorKindDefault, 0)
			setBG(ColorDefaultBG, ColorKindDefault, 0)
			t.currBold = false
			t.currUnderline = false
			t.currInverse = false

		case code == 1:
			t.currBold = true
		case code == 4:
			t.currUnderline = true
		case code == 7:
			t.currInverse = true
		case code == 22:
			t.currBold = false
		case code == 24:
			t.currUnderline = false
		case code == 27:
			t.currInverse = false

		case code >= 30 && code <= 37:
			setFG(t.themeANSI(code-30), ColorKindANSI16, code-30)
		case code == 39:
			setFG(ColorDefaultFG, ColorKindDefault, 0)

		case code >= 40 && code <= 47:
			setBG(t.themeANSI(code-40), ColorKindANSI16, code-40)
		case code == 49:
			setBG(ColorDefaultBG, ColorKindDefault, 0)

		case code >= 90 && code <= 97:
			setFG(t.themeANSI(code-90+8), ColorKindANSI16, code-90+8)
		case code >= 100 && code <= 107:
			setBG(t.themeANSI(code-100+8), ColorKindANSI16, code-100+8)

		case code == 38:
			if i+2 < len(args) && args[i+1] == 5 {
				setFG(t.ansi256(args[i+2]), ColorKindANSI256, args[i+2])
				i += 2
			} else if i+4 < len(args) && args[i+1] == 2 {
				setFG(RGB(clampByte(args[i+2]), clampByte(args[i+3]), clampByte(args[i+4])), ColorKindRGB, 0)
				i += 4
			}

		case code == 48:
			if i+2 < len(args) && args[i+1] == 5 {
				setBG(t.ansi256(args[i+2]), ColorKindANSI256, args[i+2])
				i += 2
			} else if i+4 < len(args) && args[i+1] == 2 {
				setBG(RGB(clampByte(args[i+2]), clampByte(args[i+3]), clampByte(args[i+4])), ColorKindRGB, 0)
				i += 4
			}
		}
	}
}

func (t *Terminal) executeOSC() {
	content := string(t.oscBuffer)
	parts := strings.SplitN(content, ";", 2)
	if len(parts) == 2 {
		cmd := parts[0]
		val := parts[1]
		if cmd == "0" || cmd == "2" {
			var sb strings.Builder
			for _, r := range val {
				if r >= 32 && r != 127 && r != '\x1b' {
					sb.WriteRune(r)
				}
			}
			sanitized := sb.String()
			// L1: truncate by runes — a byte cut could split a
			// multi-byte rune in half and corrupt the title.
			if r := []rune(sanitized); len(r) > 256 {
				sanitized = string(r[:256])
			}
			t.title = sanitized
			select {
			case t.TitleChan <- sanitized:
			default:
			}
		}
	}
}
