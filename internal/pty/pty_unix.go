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

	// mu serialises Write, Resize and the final file close so concurrent
	// writers (UI keystrokes/paste, the CPR-response pump, ghost-text
	// writes) can never interleave bytes mid-escape-sequence.
	mu sync.Mutex
	// done is closed by Close: no new I/O may start afterwards.
	done chan struct{}
	// closeOnce makes Close idempotent.
	closeOnce sync.Once
	// fileClosed guards the master fd; it is only ever closed by
	// CloseFile, and only once.
	fileClosed bool
	// waitDone is closed by Wait once the child has been reaped.
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
	// Serialised by p.mu, so concurrent writers cannot interleave.
	// Loop so a short write cannot silently truncate an escape sequence.
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

// Close shuts the PTY down: it stops new I/O and kills the child's process
// group (SIGHUP, escalating to SIGTERM then SIGKILL) so HUP-ignoring
// processes cannot survive as orphans. It is idempotent.
//
// The master fd itself is NOT closed here: the reader goroutine is blocked
// in Read, and closing the fd under it risks the fd number being reused by
// a new tab's PTY. Instead the reader closes the fd itself via CloseFile
// once its Read has returned; Close starts a bounded watchdog that closes
// it after 2s if the reader is somehow still stuck.
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

// CloseFile closes the master fd, at most once. It must only be called
// after no goroutine is blocked in Read on this PTY (the reader calls it
// when its Read loop exits); the Close watchdog calls it as a last resort.
func (p *TerminalPTY) CloseFile() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fileClosed {
		return nil
	}
	p.fileClosed = true
	return p.file.Close()
}

// killProcessGroup signals the child's whole process group. The child is a
// session leader (creack/pty sets Setsid), so its pgid equals its pid.
// SIGHUP goes out immediately; a goroutine escalates to SIGTERM after
// 200ms and SIGKILL after a further 500ms if the group is still alive,
// stopping early once Wait has reaped the child.
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
			// Signal 0 probes existence; EPERM means alive but unpermitted.
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

// processAlive reports whether pid still refers to a live process:
// Kill(pid, 0) fails with ESRCH only once the pid is gone (EPERM means it
// exists but cannot be signalled).
func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

func (p *TerminalPTY) IsForegroundShell() bool {
	if p == nil || p.file == nil || p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	// L17: fail closed instead of fail-open — on any error (dead child,
	// ioctl failure) report "not the foreground shell" so ghost text and
	// diagnostics are not offered for an app that is actually running.
	if !processAlive(p.cmd.Process.Pid) {
		return false
	}
	// The fd fetch and the ioctl run under mu: CloseFile closes the same
	// os.File under mu, and calling Fd() concurrently with Close() is a
	// data race on the file's internals (flagged by -race).
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
	// L17: do not trust /proc/<pid>/cwd for a dead (or already reaped and
	// reused) pid.
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
