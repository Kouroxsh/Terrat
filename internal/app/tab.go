package app

import (
	"context"
	"os"
	"sync"

	"github.com/Kouroxsh/Terrat/internal/pty"
	"github.com/Kouroxsh/Terrat/internal/terminal"
)

type tabTitleMsg struct {
	tabID int
	title string
}

type Tab struct {
	ID        int
	Title     string
	Term      *terminal.Terminal
	PTY       *pty.TerminalPTY
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
	HasBell   bool

	// wg tracks the tab's goroutines (exit waiter, PTY reader, title
	// pump, response pump); done closes when they have all exited, so
	// App.Close can wait for them instead of racing process exit (L20).
	wg   sync.WaitGroup
	done chan struct{}

	// exitState is the reaped child status, set by the exit waiter once
	// Wait returns. exitCode is -1 until the process has exited.
	mu        sync.Mutex
	exitState *os.ProcessState
	exitCode  int
}

// Done returns a channel that closes once all of the tab's goroutines
// have exited.
func (t *Tab) Done() <-chan struct{} {
	return t.done
}

// ExitCode returns the child process exit code once Wait has reaped it,
// or -1 if the process has not exited (yet) or no status was recorded.
func (t *Tab) ExitCode() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.exitCode
}

// ExitState returns the reaped *os.ProcessState, or nil if not exited yet.
func (t *Tab) ExitState() *os.ProcessState {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.exitState
}

func (t *Tab) Close() {
	t.closeOnce.Do(func() {
		if t.cancel != nil {
			t.cancel()
		}
		if t.PTY != nil {
			_ = t.PTY.Close()
		}
	})
}

func createTab(
	ctx context.Context,
	id int,
	cols, rows int,
	pixelWidth, pixelHeight uint16,
	th *terminal.Theme,
	tabExitNotifyCh chan<- int,
	ptyDataNotifyCh chan<- int,
	tabTitleNotifyCh chan<- tabTitleMsg,
	customCmd ...string,
) (*Tab, error) {
	pMaster, err := pty.Start(uint16(cols), uint16(rows), pixelWidth, pixelHeight, customCmd...)
	if err != nil {
		return nil, err
	}

	t := terminal.New(cols, rows)
	t.SetTheme(th)

	initialTitle := "bash"
	if pMaster != nil && pMaster.ShellName() != "" {
		initialTitle = pMaster.ShellName()
	}

	tabCtx, tabCancel := context.WithCancel(ctx)

	tab := &Tab{
		ID:       id,
		Title:    initialTitle,
		Term:     t,
		PTY:      pMaster,
		ctx:      tabCtx,
		cancel:   tabCancel,
		exitCode: -1,
		done:     make(chan struct{}),
	}
	tab.wg.Add(4)
	go func() {
		tab.wg.Wait()
		close(tab.done)
	}()

	// Process exit waiter. The exit status is recorded first, then the
	// tab is closed (releasing the PTY) *before* notifying the app loop:
	// notifying first could block on the bounded channel while the PTY
	// stays open, and with >16 tabs exiting at once a waiter could block
	// forever. Once the tab is closed, a blocked notify only delays this
	// goroutine.
	go func() {
		defer tab.wg.Done()
		state, _ := tab.PTY.Wait()
		tab.mu.Lock()
		tab.exitState = state
		if state != nil {
			tab.exitCode = state.ExitCode()
		} else {
			tab.exitCode = -1
		}
		tab.mu.Unlock()
		tab.Close()
		select {
		case tabExitNotifyCh <- id:
		case <-ctx.Done():
		}
	}()

	// PTY output reader -> terminal write
	go func() {
		defer tab.wg.Done()
		buf := make([]byte, 8192)
		for {
			n, readErr := tab.PTY.Read(buf)
			if n > 0 {
				_, _ = tab.Term.Write(buf[:n])
				select {
				case ptyDataNotifyCh <- id:
				default:
				}
			}
			if readErr != nil {
				break
			}
		}
		// L14: a UTF-8 sequence cut short by the child exiting is
		// flushed as U+FFFD instead of being silently dropped.
		tab.Term.Flush()
		// The child is gone and this goroutine is no longer blocked in
		// Read, so it is safe to release the master fd now. Closing it
		// here (rather than in Close) avoids closing it under a blocked
		// Read, which could let a new tab's PTY reuse the fd number.
		_ = tab.PTY.CloseFile()
	}()

	// Title update pump
	go func() {
		defer tab.wg.Done()
		for {
			select {
			case <-tab.ctx.Done():
				return
			case title, ok := <-t.TitleChan:
				if !ok {
					return
				}
				select {
				case tabTitleNotifyCh <- tabTitleMsg{tabID: id, title: title}:
				case <-tab.ctx.Done():
					return
				}
			}
		}
	}()

	// Terminal response -> PTY input pump (e.g. CPR cursor position responses)
	go func() {
		defer tab.wg.Done()
		for {
			select {
			case <-tab.ctx.Done():
				return
			case resp, ok := <-t.ResponseChan:
				if !ok {
					return
				}
				if len(resp) > 0 {
					writePTY(tab.PTY, resp)
				}
			}
		}
	}()

	return tab, nil
}
