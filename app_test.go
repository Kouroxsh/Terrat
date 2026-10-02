package main

import (
	"context"
	"strings"
	"testing"

	"terrat/config"
	"terrat/platform"
	"terrat/terminal"
)

func TestURLRegexDetection(t *testing.T) {
	cases := []struct {
		input       string
		expectedURL string
	}{
		{
			input:       "Visit https://github.com/vixland4509/Terrat for details.",
			expectedURL: "https://github.com/vixland4509/Terrat",
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
