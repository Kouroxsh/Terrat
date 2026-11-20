package terminal

import (
	"sync"
	"testing"
)

// TestLockingAccessorsConcurrentWithWrites (H3): the locking accessors must
// be safe to call from the UI goroutine while the PTY goroutine mutates the
// same fields via Write. Run with -race: before the fix, the *Locked()
// accessors read these fields without holding mu and the race detector
// fires here.
func TestLockingAccessorsConcurrentWithWrites(t *testing.T) {
	term := New(80, 24)

	// Toggle the modes the accessors read, so Write actually mutates them.
	seqs := []string{
		"\x1b[?1000h", "\x1b[?1000l", // mouse tracking
		"\x1b[?1006h", "\x1b[?1006l", // mouse SGR
		"\x1b[?1h", "\x1b[?1l", // application cursor keys
		"\x1b[?2004h", "\x1b[?2004l", // bracketed paste
		"\x1b[?1049h", "\x1b[?1049l", // alt screen
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
