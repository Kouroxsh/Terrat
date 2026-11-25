package terminal

import (
	"sync"
	"testing"
)

func TestLockingAccessorsConcurrentWithWrites(t *testing.T) {
	term := New(80, 24)

	seqs := []string{
		"\x1b[?1000h", "\x1b[?1000l",
		"\x1b[?1006h", "\x1b[?1006l",
		"\x1b[?1h", "\x1b[?1l",
		"\x1b[?2004h", "\x1b[?2004l",
		"\x1b[?1049h", "\x1b[?1049l",
		"hello world\r\n",
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			for _, s := range seqs {
				if _, err := term.Write([]byte(s)); err != nil {
					t.Errorf("Write: %v", err)
					return
				}
			}
		}
	}()
	for i := 0; i < 200; i++ {
		_ = term.AppCursor()
		_ = term.MouseTracking()
		_ = term.MouseSGR()
		_ = term.BracketedPaste()
		_ = term.IsAlt()
		_ = term.Rows()
	}
	wg.Wait()
}
