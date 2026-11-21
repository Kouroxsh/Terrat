package terminal

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// L1: OSC title truncation must be rune-based, not byte-based.
func TestLowOSCTitleRuneTruncation(t *testing.T) {
	term := New(80, 24)
	long := strings.Repeat("é", 300) // 2 bytes per rune
	_, _ = term.Write([]byte("\x1b]2;" + long + "\x07"))
	if got := len([]rune(term.title)); got != 256 {
		t.Fatalf("title truncated to %d runes, want 256", got)
	}
	if !utf8.ValidString(term.title) {
		t.Fatalf("title is not valid UTF-8 after truncation")
	}
}

// L2: ':' is a CSI sub-parameter separator; intermediate bytes are not final.
func TestLowCSIColonSubParameters(t *testing.T) {
	term := New(80, 24)
	_, _ = term.Write([]byte("\x1b[38:5:196mX"))
	cell := term.GetCell(0, 0)
	// 196 -> cube: idx 180 -> r=255, g=0, b=0.
	if cell.FG != RGB(255, 0, 0) {
		t.Fatalf("colon SGR 38:5:196 gave FG %06x, want ff0000", uint32(cell.FG))
	}
}

func TestLowCSIIntermediateBytes(t *testing.T) {
	term := New(80, 24)
	_, _ = term.Write([]byte("\x1b[?25l"))
	if _, _, vis := term.Cursor(); vis {
		t.Fatalf("cursor should be hidden after CSI ? 25 l")
	}
	// The space is an intermediate byte, not the final byte: the sequence
	// must still execute (show cursor) and the 'h' must not print.
	_, _ = term.Write([]byte("\x1b[?25 h"))
	if _, _, vis := term.Cursor(); !vis {
		t.Fatalf("cursor should be visible after CSI ? 25 SP h")
	}
	if got := term.GetRowString(0); strings.Contains(got, "h") {
		t.Fatalf("final byte 'h' leaked as text: %q", got)
	}
}

// L3: VT/FF are line feeds; DEL and C1 controls print nothing.
func TestLowVerticalTabFormFeed(t *testing.T) {
	term := New(80, 24)
	_, _ = term.Write([]byte("a\vb\fc"))
	// LF moves down but keeps the column (like \n), so b/c land in
	// column 1, not 0.
	if got := strings.TrimSpace(term.GetRowString(0)); got != "a" {
		t.Fatalf("row 0 = %q, want a", got)
	}
	if got := strings.TrimSpace(term.GetRowString(1)); got != "b" {
		t.Fatalf("row 1 = %q, want b", got)
	}
	if got := strings.TrimSpace(term.GetRowString(2)); got != "c" {
		t.Fatalf("row 2 = %q, want c", got)
	}
}

func TestLowDELAndC1Dropped(t *testing.T) {
	term := New(80, 24)
	_, _ = term.Write([]byte("a\x7fb"))
	term.processRune(0x85) // C1 control, fed as a rune
	_, _ = term.Write([]byte("c"))
	if got := strings.TrimRight(term.GetRowString(0), " "); got != "abc" {
		t.Fatalf("row 0 = %q, want abc (DEL/C1 dropped)", got)
	}
}

// L4: truecolour components clamp instead of wrapping (byte(300) was 44).
func TestLowTrueColourClamps(t *testing.T) {
	term := New(80, 24)
	_, _ = term.Write([]byte("\x1b[38;2;300;400;500mX"))
	if got := term.GetCell(0, 0).FG; got != RGB(255, 255, 255) {
		t.Fatalf("FG = %06x, want ffffff (clamped)", uint32(got))
	}
}

// L5/L6: 256-colour indices resolve through the active theme, and a theme
// change remaps palette colours but leaves explicit RGB alone.
func TestLowThemeRemapByKind(t *testing.T) {
	term := New(80, 24)
	_, _ = term.Write([]byte("\x1b[31mA\x1b[38;2;10;20;30mB\x1b[38;5;200mC"))
	fgA := term.GetCell(0, 0).FG
	if fgA != ThemeTokyoNight.ANSI[1] {
		t.Fatalf("ANSI red = %06x, want tokyo-night ANSI[1] %06x", uint32(fgA), uint32(ThemeTokyoNight.ANSI[1]))
	}
	term.SetTheme(ThemeCatppuccinMocha)
	if got := term.GetCell(0, 0).FG; got != ThemeCatppuccinMocha.ANSI[1] {
		t.Fatalf("after theme change ANSI red = %06x, want mocha ANSI[1] %06x", uint32(got), uint32(ThemeCatppuccinMocha.ANSI[1]))
	}
	if got := term.GetCell(1, 0).FG; got != RGB(10, 20, 30) {
		t.Fatalf("explicit RGB changed across themes: %06x, want 0a141e", uint32(got))
	}
	// 256-colour cube entries are theme-independent.
	if got := term.GetCell(2, 0).FG; got != term.ansi256(200) {
		t.Fatalf("ANSI256 200 = %06x, want %06x", uint32(got), uint32(term.ansi256(200)))
	}
}

// L7: DECSC-saved colours are remapped by a theme change too.
func TestLowSavedCursorColoursRemapped(t *testing.T) {
	term := New(80, 24)
	_, _ = term.Write([]byte("\x1b[31m\x1b7"))
	term.SetTheme(ThemeCatppuccinMocha)
	_, _ = term.Write([]byte("\x1b8X"))
	if got := term.GetCell(0, 0).FG; got != ThemeCatppuccinMocha.ANSI[1] {
		t.Fatalf("restored FG = %06x, want mocha ANSI[1] %06x", uint32(got), uint32(ThemeCatppuccinMocha.ANSI[1]))
	}
}

// L8: a single-line scroll region (ESC[5;5r) is valid.
func TestLowSingleLineScrollRegion(t *testing.T) {
	term := New(80, 24)
	_, _ = term.Write([]byte("\x1b[5;5r"))
	if term.scrollTop != 4 || term.scrollBottom != 4 {
		t.Fatalf("region = %d..%d, want 4..4", term.scrollTop, term.scrollBottom)
	}
}

// L9: resize reflows instead of truncating.
func TestLowResizeReflows(t *testing.T) {
	term := New(10, 5)
	_, _ = term.Write([]byte("0123456789ABCDEFGHIJ"))
	if got := term.GetAllText(); got != "0123456789ABCDEFGHIJ" {
		t.Fatalf("before resize text = %q", got)
	}
	term.Resize(5, 5)
	if got := term.GetAllText(); got != "0123456789ABCDEFGHIJ" {
		t.Fatalf("after narrowing to 5 cols text = %q, want all 20 chars kept", got)
	}
	term.Resize(20, 5)
	if got := term.GetAllText(); got != "0123456789ABCDEFGHIJ" {
		t.Fatalf("after widening to 20 cols text = %q", got)
	}
}

// L10: New(0, 0) must not panic on first output.
func TestLowNewZeroSize(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("New(0, 0) + Write panicked: %v", r)
		}
	}()
	term := New(0, 0)
	_, _ = term.Write([]byte("hi"))
	if got := strings.TrimRight(term.GetRowString(0), " "); got != "hi" {
		t.Fatalf("row 0 = %q, want hi", got)
	}
}

// L12: case folding must not shift columns; matches do not overlap.
func TestLowSearchFoldingAndOverlap(t *testing.T) {
	if got := FindRowMatches("aaa", "aa"); len(got) != 1 {
		t.Fatalf("overlapping matches counted: %+v", got)
	}
	// 'İ' (U+0130) lowercases to two runes with strings.ToLower; per-rune
	// folding keeps the column map intact.
	matches := FindRowMatches("İİ", "i")
	if len(matches) != 2 || matches[0].StartCol != 0 || matches[1].StartCol != 1 {
		t.Fatalf("folded matches = %+v, want cols 0 and 1", matches)
	}
}

// L13: copying a wrapped line must not insert a newline at the wrap.
func TestLowCopyJoinsWrappedRows(t *testing.T) {
	term := New(5, 5)
	_, _ = term.Write([]byte("hello world"))
	if got := term.GetAllText(); got != "hello world" {
		t.Fatalf("GetAllText = %q, want \"hello world\"", got)
	}
}

// L14: invalid UTF-8 bytes flush as U+FFFD; a truncated prefix flushes on Flush().
func TestLowInvalidUTF8Flushes(t *testing.T) {
	term := New(80, 24)
	_, _ = term.Write([]byte{0xff})
	if got := term.GetCell(0, 0).Char; got != utf8.RuneError {
		t.Fatalf("invalid byte rendered as %U, want U+FFFD", got)
	}
	term2 := New(80, 24)
	_, _ = term2.Write([]byte{0xE2, 0x82}) // truncated '€'
	if got := strings.TrimRight(term2.GetRowString(0), " "); got != "" {
		t.Fatalf("truncated rune printed early: %q", got)
	}
	term2.Flush()
	if got := term2.GetCell(0, 0).Char; got != utf8.RuneError {
		t.Fatalf("after Flush, cell = %U, want U+FFFD", got)
	}
}

// L15: scrolling a selection out of view clears it instead of collapsing it.
func TestLowScrollKeepSelectionNoCollapse(t *testing.T) {
	term := New(80, 24)
	for i := 0; i < 30; i++ {
		_, _ = term.Write([]byte("line\r\n"))
	}
	term.StartSelection(0, 20)
	term.UpdateSelection(5, 22)
	if !term.HasSelection() {
		t.Fatalf("selection not active before scroll")
	}
	term.ScrollKeepSelection(1)
	if !term.HasSelection() {
		t.Fatalf("selection cleared by a 1-row scroll")
	}
	sX, sY, _, eY := term.sel.Normalized()
	if sY != 21 || eY != 23 {
		t.Fatalf("selection moved to rows %d..%d, want 21..23", sY, eY)
	}
	_ = sX
	term.ScrollKeepSelection(100)
	if term.HasSelection() {
		t.Fatalf("selection survived scrolling entirely out of view")
	}
}
