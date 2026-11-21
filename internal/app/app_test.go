package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Kouroxsh/Terrat/internal/config"
	"github.com/Kouroxsh/Terrat/internal/platform"
	"github.com/Kouroxsh/Terrat/internal/terminal"
)

func TestURLRegexDetection(t *testing.T) {
	cases := []struct {
		input       string
		expectedURL string
	}{
		{
			input:       "Visit https://github.com/Kouroxsh/Terrat for details.",
			expectedURL: "https://github.com/Kouroxsh/Terrat",
		},
		{
			input:       "Check http://localhost:8080/api/v1?user=test&mode=debug!",
			expectedURL: "http://localhost:8080/api/v1?user=test&mode=debug",
		},
		{
			input:       "Download: (https://example.com/file.tar.gz)",
			expectedURL: "https://example.com/file.tar.gz",
		},
		{
			input:       "No links here",
			expectedURL: "",
		},
	}

	for _, c := range cases {
		matches := urlRegex.FindAllStringIndex(c.input, -1)
		var foundURL string
		if len(matches) > 0 {
			raw := c.input[matches[0][0]:matches[0][1]]
			foundURL = strings.TrimRight(raw, ".,;:!?)'\"")
		}
		if foundURL != c.expectedURL {
			t.Errorf("for input %q: expected URL %q, got %q", c.input, c.expectedURL, foundURL)
		}
	}
}

func TestFindSearchMatches(t *testing.T) {
	term := terminal.New(40, 10)
	_, _ = term.Write([]byte("Hello TerraTerminal world!\r\nSecond line with terra in it."))

	matches := findSearchMatches(term, "terra")
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches for 'terra', got %d", len(matches))
	}

	if matches[0].Row != 0 {
		t.Errorf("expected first match on row 0, got row %d", matches[0].Row)
	}
	if matches[1].Row != 1 {
		t.Errorf("expected second match on row 1, got row %d", matches[1].Row)
	}
}

func TestFindSearchMatchesCapped(t *testing.T) {
	term := terminal.New(80, 50)
	for i := 0; i < 50; i++ {
		_, _ = term.Write([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\r\n"))
	}
	matches := findSearchMatches(term, "a")
	if len(matches) != 1000 {
		t.Fatalf("expected matches capped at 1000, got %d", len(matches))
	}
}

func TestSettingsItemsGeneration(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Theme = "tokyo-night"
	cfg.FontSize = 14.0

	items := getSettingsItems(cfg, false)
	if len(items) == 0 {
		t.Fatalf("expected settings items to be generated, got empty")
	}

	itemMap := make(map[string]string)
	for _, item := range items {
		itemMap[item.ID] = item.Value
	}

	if itemMap["theme"] != "Tokyo Night" {
		t.Errorf("expected theme display 'Tokyo Night', got %q", itemMap["theme"])
	}
	if itemMap["font_size"] != "14pt" {
		t.Errorf("expected font_size display '14pt', got %q", itemMap["font_size"])
	}
}

func TestTabCreationAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tabExitNotifyCh := make(chan int, 4)
	ptyDataNotifyCh := make(chan int, 16)
	tabTitleNotifyCh := make(chan tabTitleMsg, 4)

	th := terminal.ThemeTokyoNight
	tab, err := createTab(
		ctx,
		1,
		80,
		24,
		800,
		600,
		th,
		tabExitNotifyCh,
		ptyDataNotifyCh,
		tabTitleNotifyCh,
	)
	if err != nil {
		t.Skipf("PTY start not supported in this test container: %v", err)
		return
	}
	defer tab.Close()

	if tab.ID != 1 {
		t.Errorf("expected tab ID 1, got %d", tab.ID)
	}
	if tab.Term == nil {
		t.Errorf("expected non-nil terminal")
	}
	if tab.PTY == nil {
		t.Errorf("expected non-nil PTY")
	}

	// Verify clean Close()
	tab.Close()
	select {
	case <-tab.ctx.Done():
		// Closed cleanly
	default:
		t.Errorf("expected tab context to be canceled on Close()")
	}
}

func TestHandleEventInvalidTabBounds(t *testing.T) {
	app := &App{
		activeTabIdx: -1,
		tabs:         nil,
	}
	var pending platform.Event
	ch := make(chan platform.Event)
	// Must not panic on invalid activeTabIdx or nil tabs
	shouldExit := app.handleEvent(platform.ExposeEvent{}, &pending, ch)
	if shouldExit {
		t.Errorf("expected handleEvent to return false on invalid activeTabIdx")
	}
}

// M7/M13: a tab whose command exits must report its exit code, and the
// exit waiter must close the PTY before notifying (so the notification
// cannot block on the bounded channel while the PTY stays open).
func TestTabExitCodePropagated(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tabExitNotifyCh := make(chan int, 4)
	ptyDataNotifyCh := make(chan int, 16)
	tabTitleNotifyCh := make(chan tabTitleMsg, 4)

	tab, err := createTab(
		ctx, 1, 80, 24, 800, 600,
		terminal.ThemeTokyoNight,
		tabExitNotifyCh, ptyDataNotifyCh, tabTitleNotifyCh,
		"sh", "-c", "exit 3",
	)
	if err != nil {
		t.Skipf("PTY start not supported in this test container: %v", err)
		return
	}
	defer tab.Close()

	select {
	case id := <-tabExitNotifyCh:
		if id != 1 {
			t.Fatalf("expected exit notification for tab 1, got %d", id)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for tab exit notification")
	}

	if got := tab.ExitCode(); got != 3 {
		t.Fatalf("expected exit code 3, got %d", got)
	}
	if tab.ExitState() == nil {
		t.Fatalf("expected non-nil exit state")
	}
}

// M9: the window's app-cursor query must follow the active tab's terminal
// (DECCKM state), so Translate can emit SS3 sequences for every key.
func TestIsAppCursorFollowsActiveTab(t *testing.T) {
	off := terminal.New(80, 24)
	on := terminal.New(80, 24)
	if _, err := on.Write([]byte("\x1b[?1h")); err != nil {
		t.Fatalf("enabling DECCKM failed: %v", err)
	}
	if !on.AppCursor() {
		t.Fatal("expected AppCursor() true after DECCKM set")
	}

	app := &App{}
	if app.isAppCursor() {
		t.Fatal("expected false with no tabs")
	}
	app.activeTermPtr.Store(off)
	if app.isAppCursor() {
		t.Fatal("expected false for a normal-cursor terminal")
	}
	app.activeTermPtr.Store(on)
	if !app.isAppCursor() {
		t.Fatal("expected true for an application-cursor terminal")
	}
	// switchTab stores the new tab's terminal in the same pointer (kept in
	// sync by the Store in switchTab); here we move it the same way.
	app.activeTermPtr.Store(off)
	if app.isAppCursor() {
		t.Fatal("expected false after moving the pointer back")
	}
}
