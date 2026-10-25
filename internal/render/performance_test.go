package render

import (
	"strings"
	"testing"

	"github.com/Kouroxsh/Terrat/internal/terminal"
)

func BenchmarkRender(b *testing.B) {
	fe, err := NewFontEngine(14)
	if err != nil {
		b.Fatal(err)
	}
	defer fe.Close()
	c := NewCanvas(fe)
	c.Resize(1024, 640)
	term := terminal.New(c.Cols(), c.Rows())
	term.Write([]byte(strings.Repeat("build completed successfully 0123456789\r\n", 32)))
	tabs := []TabInfo{{ID: 1, Title: "bash", Active: true}}
	for _, search := range []bool{false, true} {
		name := "Normal"
		var matches []SearchMatch
		if search {
			name = "Search"
			for y := 0; y < c.Rows(); y++ {
				for x := 0; x < c.Cols()-1; x += 3 {
					matches = append(matches, SearchMatch{Row: y, StartCol: x, EndCol: x + 1})
				}
			}
		}
		b.Run(name, func(b *testing.B) {
			c.Render(term, true, "Terrat", "", tabs, matches, 0, nil, "", nil)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				c.Render(term, true, "Terrat", "", tabs, matches, 0, nil, "", nil)
			}
		})
	}
}

func BenchmarkFillRect(b *testing.B) {
	pixels := make([]byte, 1024*640*4)
	b.ReportAllocs()
	b.SetBytes(int64(len(pixels)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FillRect(pixels, 1024*4, 0, 0, 1024, 640, 0x1a1b26)
	}
}

func BenchmarkFontStartup(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		fe, err := NewFontEngine(14)
		if err != nil {
			b.Fatal(err)
		}
		fe.Close()
	}
}
