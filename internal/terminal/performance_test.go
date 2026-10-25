package terminal

import (
	"strings"
	"testing"
)

func BenchmarkTerminalOutput(b *testing.B) {
	for _, tc := range []struct {
		name, line string
		alt        bool
	}{
		{"Plain", "build completed successfully: 0123456789\r\n", false},
		{"ANSI", "\x1b[32mbuild\x1b[0m completed successfully: 0123456789\r\n", false},
		{"Alternate", "build completed successfully: 0123456789\r\n", true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			term := New(120, 32)
			if tc.alt {
				term.Write([]byte("\x1b[?1049h"))
			}
			data := []byte(strings.Repeat(tc.line, 128))
			for i := 0; i < 100; i++ {
				term.Write(data)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				term.Write(data)
			}
		})
	}
}
