package render

import (
	"testing"

	"github.com/Kouroxsh/Terrat/internal/terminal"
)

func TestDisplayWidth(t *testing.T) {
	if got := displayWidth("abc"); got != 3 {
		t.Fatalf("displayWidth(abc) = %d, want 3", got)
	}
	// 4 Persian letters are 8 bytes but 4 cells; the emoji is 4
	// bytes but 1 cell.
	if got := displayWidth("سلام"); got != 4 {
		t.Fatalf("displayWidth(سلام) = %d, want 4", got)
	}
	if got := displayWidth("💡 hi"); got != 4 {
		t.Fatalf("displayWidth(💡 hi) = %d, want 4", got)
	}
}

func TestTruncateQueryEnd(t *testing.T) {
	// Keeps the tail, never cuts a rune in half.
	if got := truncateQueryEnd("سلامدنیا", 4); got != "دنیا" {
		t.Fatalf("truncateQueryEnd = %q, want دنیا", got)
	}
	if got := truncateQueryEnd("abcdef", 3); got != "def" {
		t.Fatalf("truncateQueryEnd = %q, want def", got)
	}
	if got := truncateQueryEnd("ab", 10); got != "ab" {
		t.Fatalf("truncateQueryEnd = %q, want ab", got)
	}
	if got := truncateQueryEnd("ab", 0); got != "" {
		t.Fatalf("truncateQueryEnd(0) = %q, want empty", got)
	}
}

func TestRenderSearchBarTinyWindow(t *testing.T) {
	fe, err := NewFontEngine(13)
	if err != nil {
		t.Fatal(err)
	}
	defer fe.Close()

	for _, size := range [][2]int{{100, 60}, {30, 40}, {10, 10}} {
		c := NewCanvas(fe)
		c.Resize(size[0], size[1])
		// A Persian query must not panic or slice mid-rune, even
		// when the bar is narrower than the query.
		barX, barY, barW, barH := c.RenderSearchBar(terminal.ThemeTokyoNight, "سلام دنیا", 1, 3)
		if barW < 1 || barH < 1 || barX < 0 || barY < 0 {
			t.Fatalf("size %v: bad search bar geometry x=%d y=%d w=%d h=%d", size, barX, barY, barW, barH)
		}
		if barX+barW > c.Width || barY+barH > c.Height {
			t.Fatalf("size %v: search bar overflows canvas: x=%d w=%d (canvas %d), y=%d h=%d (canvas %d)",
				size, barX, barW, c.Width, barY, barH, c.Height)
		}
	}
}

func TestRenderPreferencesModalTinyWindow(t *testing.T) {
	fe, err := NewFontEngine(13)
	if err != nil {
		t.Fatal(err)
	}
	defer fe.Close()

	options := []PrefOption{
		{ID: "diagnostics", Label: "Live Diagnostics", IsToggle: true, Enabled: true},
		{ID: "shell", Label: "Shell", Value: "/home/کاربر/zsh"},
		{ID: "theme", Label: "Color Theme", Value: "Tokyo Night"},
	}

	for _, th := range []*terminal.Theme{terminal.ThemeTokyoNight, terminal.ThemeMinecraft} {
		for _, size := range [][2]int{{100, 60}, {30, 40}, {10, 10}} {
			c := NewCanvas(fe)
			c.Resize(size[0], size[1])
			modalX, modalY, modalW, modalH, rowH := c.RenderPreferencesModal(th, 0, options, "")
			if modalW < 1 || modalH < 1 || rowH < 1 || modalX < 0 || modalY < 0 {
				t.Fatalf("theme %s size %v: bad modal geometry x=%d y=%d w=%d h=%d rowH=%d",
					th.ID, size, modalX, modalY, modalW, modalH, rowH)
			}
		}
	}
}
