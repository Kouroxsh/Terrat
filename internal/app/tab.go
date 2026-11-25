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

	wg   sync.WaitGroup
	done chan struct{}

	mu        sync.Mutex
	exitState *os.ProcessState
	exitCode  int
}

func (t *Tab) Done() <-chan struct{} {
	return t.done
}

func (t *Tab) ExitCode() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.exitCode
}

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

		tab.Term.Flush()

		_ = tab.PTY.CloseFile()
	}()

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
