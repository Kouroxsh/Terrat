package render

import (
	"bytes"
	"testing"

	"github.com/Kouroxsh/Terrat/internal/terminal"
)

func TestDiagnosticAppearsOnlyInTerminalBody(t *testing.T) {
	fe, err := NewFontEngine(13)
	if err != nil {
		t.Fatal(err)
	}
	defer fe.Close()
	c := NewCanvas(fe)
	c.Resize(1200, 400)
	term := terminal.New(c.Cols(), c.Rows())
	c.Render(term, false, "Terrat", "", nil, nil, 0, nil, "", nil)
	before := bytes.Clone(c.Pixels)
	diag := &DiagnosticInfo{
		IsError: true, Message: "Command 'kir' not recognized",
		Suggestion: "Did you mean 'dir'?", QuickFix: "dir",
	}
	c.Render(term, false, "Terrat", "", nil, nil, 0, nil, "", diag)
	headerEnd := HeaderHeight * c.Stride
	if !bytes.Equal(before[:headerEnd], c.Pixels[:headerEnd]) {
		t.Error("diagnostic must not appear in the header")
	}
	if bytes.Equal(before[headerEnd:], c.Pixels[headerEnd:]) {
		t.Error("diagnostic must remain visible in the terminal body")
	}
}

func TestRenderPreferencesModal(t *testing.T) {
	fe, err := NewFontEngine(13.0)
	if err != nil {
		t.Fatalf("failed to init font engine: %v", err)
	}
	defer fe.Close()

	c := NewCanvas(fe)
	c.Resize(800, 600)

	th := terminal.ThemeTokyoNight

	options := []PrefOption{
		{ID: "diagnostics", Label: "Live Diagnostics", IsToggle: true, Enabled: true},
		{ID: "ghost_text", Label: "Ghost Autocomplete", IsToggle: true, Enabled: false},
		{ID: "theme", Label: "Color Theme", Value: "Tokyo Night"},
	}

	modalX, modalY, modalW, modalH, rowH := c.RenderPreferencesModal(th, 0, options, "tokyo-night")

	if modalW <= 0 || modalH <= 0 || rowH <= 0 {
		t.Fatalf("expected positive modal dimensions, got w=%d, h=%d, rowH=%d", modalW, modalH, rowH)
	}

	// Modal should be anchored in top-right corner under header
	if modalX < c.Width/2 {
		t.Fatalf("expected modal to be in the right corner, got modalX=%d in width %d", modalX, c.Width)
	}
	if modalY < HeaderHeight {
		t.Fatalf("expected modal to be below header, got modalY=%d, HeaderHeight=%d", modalY, HeaderHeight)
	}
}

func TestSearchHighlightsHandleOverlapsAndReset(t *testing.T) {
	fe, err := NewFontEngine(13)
	if err != nil {
		t.Fatal(err)
	}
	defer fe.Close()
	c := NewCanvas(fe)
	c.Resize(800, 400)
	term := terminal.New(c.Cols(), c.Rows())
	matches := []SearchMatch{{Row: 0, StartCol: 1, EndCol: 3}, {Row: 0, StartCol: 2, EndCol: 2}, {Row: -1, StartCol: 0, EndCol: 100}}
	c.Render(term, false, "Terrat", "", nil, matches, 1, nil, "", nil)
	for x, want := range map[int]uint32{1: 0xe0af68, 2: 0x9ece6a, 3: 0xe0af68} {
		p := (HeaderHeight+PaddingTop)*c.Stride + (PaddingLeft+x*fe.CharWidth())*4
		got := uint32(c.Pixels[p+2])<<16 | uint32(c.Pixels[p+1])<<8 | uint32(c.Pixels[p])
		if got != want {
			t.Fatalf("column %d background = %x, want %x", x, got, want)
		}
	}
	c.Render(term, false, "Terrat", "", nil, nil, 0, nil, "", nil)
	p := (HeaderHeight+PaddingTop)*c.Stride + (PaddingLeft+2*fe.CharWidth())*4
	got := uint32(c.Pixels[p+2])<<16 | uint32(c.Pixels[p+1])<<8 | uint32(c.Pixels[p])
	if got != uint32(term.Theme().BG) {
		t.Fatal("search highlight persisted after closing search")
	}
}

func TestFillRectClipsWithoutTouchingOtherPixels(t *testing.T) {
	for _, rect := range [][4]int{{-2, -1, 4, 3}, {1, 1, 10, 10}, {0, 0, 4, 3}, {5, 0, 2, 2}, {0, 4, 2, 2}} {
		buf := bytes.Repeat([]byte{0x77}, 4*3*4)
		FillRect(buf, 4*4, rect[0], rect[1], rect[2], rect[3], 0x123456)
		for y := 0; y < 3; y++ {
			for x := 0; x < 4; x++ {
				want := []byte{0x77, 0x77, 0x77, 0x77}
				if x >= rect[0] && x < rect[0]+rect[2] && y >= rect[1] && y < rect[1]+rect[3] {
					want = []byte{0x56, 0x34, 0x12, 0xff}
				}
				p := (y*4 + x) * 4
				if !bytes.Equal(buf[p:p+4], want) {
					t.Fatalf("rect %v: wrong pixel at %d,%d", rect, x, y)
				}
			}
		}
	}
}

func TestCanvasReusesBuffersAndReleasesLargeResizes(t *testing.T) {
	fe, err := NewFontEngine(13)
	if err != nil {
		t.Fatal(err)
	}
	defer fe.Close()
	c := NewCanvas(fe)
	c.Resize(1024, 640)
	first := &c.Pixels[0]
	c.Resize(800, 600)
	c.Resize(1024, 640)
	if &c.Pixels[0] != first {
		t.Fatal("resize allocated despite sufficient buffer capacity")
	}
	c.Resize(128, 128)
	if cap(c.Pixels) > 2*len(c.Pixels) {
		t.Fatal("small window retained oversized framebuffer")
	}
}
