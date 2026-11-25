//go:build linux

package pty

import (
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestCloseKillsHupIgnoringProcess(t *testing.T) {
	p, err := Start(80, 24, 0, 0, "sh", "-c", `trap "" HUP; exec sleep 30`)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	pid := p.cmd.Process.Pid

	time.Sleep(200 * time.Millisecond)

	start := time.Now()
	waitDone := make(chan struct{})
	go func() {
		_, _ = p.Wait()
		close(waitDone)
	}()

	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case <-waitDone:
	case <-time.After(5 * time.Second):
		t.Fatal("child did not exit within 5s of Close (SIGHUP/SIGTERM/SIGKILL escalation failed)")
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("Wait returned after %v; child died on SIGHUP instead of surviving to SIGTERM", elapsed)
	}

	if err := syscall.Kill(pid, 0); err == nil {
		t.Fatalf("process %d is still alive after Close", pid)
	}
}

func TestCloseIdempotentAndWriteAfterClose(t *testing.T) {
	p, err := Start(80, 24, 0, 0, "sleep", "30")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	go func() { _, _ = p.Wait() }()

	if err := p.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := p.Write([]byte("x")); err == nil {
		t.Fatal("Write after Close succeeded, want an error")
	}
	if err := p.Resize(80, 24, 0, 0); err == nil {
		t.Fatal("Resize after Close succeeded, want an error")
	}
}

func TestConcurrentWriteAndClose(t *testing.T) {
	p, err := Start(80, 24, 0, 0, "sh")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	go func() { _, _ = p.Wait() }()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				if _, err := p.Write([]byte("true\n")); err != nil {
					return
				}
			}
		}()
	}
	time.Sleep(10 * time.Millisecond)
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("writers did not return after Close")
	}
}

func TestForegroundAndCwdLiveness(t *testing.T) {
	p, err := Start(80, 24, 0, 0, "sleep", "30")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	go func() { _, _ = p.Wait() }()

	deadline := time.Now().Add(5 * time.Second)
	for !p.IsForegroundShell() && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if !p.IsForegroundShell() {
		t.Fatalf("IsForegroundShell = false for a live foreground child")
	}
	if cwd := p.GetCwd(); cwd == "" {
		t.Fatalf("GetCwd = empty for a live child")
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	deadline = time.Now().Add(5 * time.Second)
	for p.IsForegroundShell() && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if p.IsForegroundShell() {
		t.Fatalf("IsForegroundShell = true after Close (fail-open)")
	}
	if cwd := p.GetCwd(); cwd != "" {
		t.Fatalf("GetCwd = %q after Close, want empty", cwd)
	}
}
