package main

import (
	"context"
	"sync"
	"terrat/pty"
	"terrat/terminal"
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
		ID:     id,
		Title:  initialTitle,
		Term:   t,
		PTY:    pMaster,
		ctx:    tabCtx,
		cancel: tabCancel,
	}

	// Process exit waiter
	go func() {
		_, _ = tab.PTY.Wait()
		select {
		case tabExitNotifyCh <- id:
		case <-ctx.Done():
		}
		tab.Close()
	}()

	// PTY output reader -> terminal write
	go func() {
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
				return
			}
		}
	}()

	// Title update pump
	go func() {
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
		for {
			select {
			case <-tab.ctx.Done():
				return
			case resp, ok := <-t.ResponseChan:
				if !ok {
					return
				}
				if len(resp) > 0 {
					_, _ = tab.PTY.Write(resp)
				}
			}
		}
	}()

	return tab, nil
}
