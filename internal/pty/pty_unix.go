//go:build linux

package pty

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

type TerminalPTY struct {
	file      *os.File
	cmd       *exec.Cmd
	shellName string

	mu sync.Mutex

	done chan struct{}

	closeOnce sync.Once

	fileClosed bool

	waitDone chan struct{}
	waitOnce sync.Once
}

func Start(cols, rows, pixelWidth, pixelHeight uint16, customCmd ...string) (*TerminalPTY, error) {
	var cmd *exec.Cmd
	var shellName string
	if len(customCmd) > 0 && customCmd[0] != "" {
		cmd = exec.Command(customCmd[0], customCmd[1:]...)
		shellName = ExtractShellName(customCmd[0])
	} else {
		cmdArgs := ResolveUnixShell(GetDefaultShell())
		if len(cmdArgs) > 1 {
			cmd = exec.Command(cmdArgs[0], cmdArgs[1:]...)
		} else {
			cmd = exec.Command(cmdArgs[0])
		}
		shellName = ExtractShellName(cmdArgs[0])
	}

	var cleanEnv []string
	for _, v := range os.Environ() {
		if strings.HasPrefix(v, "TERM=") || strings.HasPrefix(v, "COLORTERM=") || strings.HasPrefix(v, "TERRAT_TERMINAL=") {
			continue
		}
		cleanEnv = append(cleanEnv, v)
	}
	customEnv := []string{
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"TERRAT_TERMINAL=1",
	}
	cmd.Env = append(cleanEnv, customEnv...)

	ws := &pty.Winsize{
		Rows: rows,
		Cols: cols,
		X:    pixelWidth,
		Y:    pixelHeight,
	}

	f, err := pty.StartWithSize(cmd, ws)
	if err != nil {
		return nil, fmt.Errorf("failed to start pty: %w", err)
	}

	return &TerminalPTY{
		file:      f,
		cmd:       cmd,
		shellName: shellName,
		done:      make(chan struct{}),
		waitDone:  make(chan struct{}),
	}, nil
}

func (p *TerminalPTY) Read(b []byte) (int, error) {
	select {
	case <-p.done:
		return 0, os.ErrClosed
	default:
	}
	return p.file.Read(b)
}

func (p *TerminalPTY) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case <-p.done:
		return 0, os.ErrClosed
	default:
	}

	total := 0
	for total < len(b) {
		n, err := p.file.Write(b[total:])
		total += n
		if err != nil {
			return total, err
		}
		if n == 0 {
			return total, io.ErrShortWrite
		}
	}
	return total, nil
}

func (p *TerminalPTY) Resize(cols, rows, pixelWidth, pixelHeight uint16) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case <-p.done:
		return os.ErrClosed
	default:
	}
	ws := &pty.Winsize{
		Rows: rows,
		Cols: cols,
		X:    pixelWidth,
		Y:    pixelHeight,
	}
	return pty.Setsize(p.file, ws)
}

func (p *TerminalPTY) Close() error {
	p.closeOnce.Do(func() {
		close(p.done)
		p.killProcessGroup()
		go func() {
			select {
			case <-p.waitDone:
			case <-time.After(2 * time.Second):
			}
			p.CloseFile()
		}()
	})
	return nil
}

func (p *TerminalPTY) CloseFile() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fileClosed {
		return nil
	}
	p.fileClosed = true
	return p.file.Close()
}

func (p *TerminalPTY) killProcessGroup() {
	if p.cmd == nil || p.cmd.Process == nil {
		return
	}
	pid := p.cmd.Process.Pid
	_ = syscall.Kill(-pid, syscall.SIGHUP)
	go func() {
		for _, step := range []struct {
			delay time.Duration
			sig   syscall.Signal
		}{
			{200 * time.Millisecond, syscall.SIGTERM},
			{500 * time.Millisecond, syscall.SIGKILL},
		} {
			timer := time.NewTimer(step.delay)
			select {
			case <-timer.C:
			case <-p.waitDone:
				timer.Stop()
				return
			}

			if err := syscall.Kill(pid, 0); err != nil {
				return
			}
			_ = syscall.Kill(-pid, step.sig)
		}
	}()
}

func (p *TerminalPTY) Wait() (*os.ProcessState, error) {
	if p.cmd == nil {
		p.waitOnce.Do(func() { close(p.waitDone) })
		return nil, nil
	}
	err := p.cmd.Wait()
	p.waitOnce.Do(func() { close(p.waitDone) })
	return p.cmd.ProcessState, err
}

func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

func (p *TerminalPTY) IsForegroundShell() bool {
	if p == nil || p.file == nil || p.cmd == nil || p.cmd.Process == nil {
		return false
	}

	if !processAlive(p.cmd.Process.Pid) {
		return false
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fileClosed || p.file == nil {
		return false
	}
	pgrp, err := unix.IoctlGetInt(int(p.file.Fd()), unix.TIOCGPGRP)
	if err != nil {
		return false
	}
	shellPgid, err := syscall.Getpgid(p.cmd.Process.Pid)
	if err != nil {
		return false
	}
	return pgrp == shellPgid
}

func (p *TerminalPTY) GetCwd() string {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return ""
	}

	if !processAlive(p.cmd.Process.Pid) {
		return ""
	}
	cwd, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", p.cmd.Process.Pid))
	if err != nil {
		return ""
	}
	return cwd
}

func (p *TerminalPTY) ShellName() string {
	return p.shellName
}
