//go:build linux

package pty

import (
	"sync"
	"syscall"
	"testing"
	"time"
)

// TestCloseKillsHupIgnoringProcess (H5): closing the PTY must kill the
// whole process group, even when the child ignores SIGHUP.
func TestCloseKillsHupIgnoringProcess(t *testing.T) {
	p, err := Start(80, 24, 0, 0, "sh", "-c", `trap "" HUP; exec sleep 30`)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	pid := p.cmd.Process.Pid

	// Let the shell install its HUP trap first: if Close races ahead of
	// it, SIGHUP would kill the shell before the trap is set and the
	// test would pass without ever exercising the escalation path.
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
	// The child ignored SIGHUP, so it must have taken the SIGTERM
	// escalation (~200ms). Anything much faster means the trap race
	// above happened and the escalation was never exercised.
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("Wait returned after %v; child died on SIGHUP instead of surviving to SIGTERM", elapsed)
	}
	// The child has been reaped, so signal 0 must fail with ESRCH.
	if err := syscall.Kill(pid, 0); err == nil {
		t.Fatalf("process %d is still alive after Close", pid)
	}
}

// TestCloseIdempotentAndWriteAfterClose (H6): Close must be safe to call
// twice, and I/O after Close must fail instead of racing the teardown.
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

// TestConcurrentWriteAndClose (H6): concurrent writers and a concurrent
// Close must not race (run with -race) and must not interleave or hang.
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
				// "true" produces no output, so the PTY buffers cannot fill.
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
