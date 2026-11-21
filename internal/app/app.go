package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Kouroxsh/Terrat/assets"
	"github.com/Kouroxsh/Terrat/internal/autosuggest"
	"github.com/Kouroxsh/Terrat/internal/config"
	"github.com/Kouroxsh/Terrat/internal/diagnostics"
	"github.com/Kouroxsh/Terrat/internal/paste"
	"github.com/Kouroxsh/Terrat/internal/platform"
	"github.com/Kouroxsh/Terrat/internal/pty"
	"github.com/Kouroxsh/Terrat/internal/render"
	"github.com/Kouroxsh/Terrat/internal/terminal"
)

const (
	AppName = "TerraTerminal"
)

type App struct {
	config       *config.Config
	win          *platform.Window
	canvas       *render.Canvas
	fontEngine   *render.FontEngine
	activeTheme  *terminal.Theme
	systemIsDark bool

	tabs         []*Tab
	activeTabIdx int
	// activeTermPtr tracks the active tab's terminal for the window event
	// loop (M9): the loop runs on its own goroutine and asks, via
	// AppCursorFunc, whether application-cursor mode is on. An atomic
	// pointer keeps that read race-free without locking the Run loop.
	activeTermPtr atomic.Pointer[terminal.Terminal]
	nextTabID     int
	customCmd     []string

	currentTitle  string
	currentWidth  uint16
	currentHeight uint16
	currentCols   int
	currentRows   int
	charW         int
	charH         int
	gridStartX    int
	gridStartY    int

	redrawCh         chan struct{}
	tabExitNotifyCh  chan int
	ptyDataNotifyCh  chan int
	tabTitleNotifyCh chan tabTitleMsg

	isSearchOpen    bool
	searchQuery     string
	searchMatches   []render.SearchMatch
	activeSearchIdx int

	isPrefOpen   bool
	prefIndex    int
	savedThemeID string

	hoveredURL *render.URLRange

	// L35: the last paste-sanitizer warnings, shown in the header HUD
	// until pasteWarningUntil (the 500ms cursor ticker redraws, so the
	// warning clears itself shortly after expiry).
	pasteWarning      string
	pasteWarningUntil time.Time

	isSelecting         bool
	isDraggingScrollbar bool
	dragScrollDelta     int
	lastDragCol         int

	lastClickTime       time.Time
	lastClickX          int
	lastClickY          int
	clickCount          int
	lastHeaderClickTime time.Time
	lastHeaderClickX    int
	lastHeaderClickY    int

	currentInputBuffer string
	activeGhostText    string
	activeDiag         *render.DiagnosticInfo
	suggestEngine      *autosuggest.Engine

	ctx    context.Context
	cancel context.CancelFunc

	// closeOnce makes Close idempotent (L20); exitCode is the code the
	// process should exit with once the last tab closes (M13), read by
	// main after Run returns instead of os.Exit-ing mid-event-loop.
	closeOnce sync.Once
	exitCode  int
}

func NewApp(cfg *config.Config, initialTitle string, customCmd []string) (*App, error) {
	fontEngine, err := render.NewFontEngine(cfg.FontSize)
	if err != nil {
		return nil, fmt.Errorf("initializing font engine: %w", err)
	}

	initCols := 120
	initRows := 32

	initWidth := uint16(render.PaddingLeft + (initCols * fontEngine.CharWidth()) + render.PaddingRight)
	initHeight := uint16(render.HeaderHeight + render.PaddingTop + (initRows * fontEngine.CharHeight()) + render.PaddingBottom)

	win, err := platform.NewWindow(initialTitle, initWidth, initHeight, "", assets.IconPNG)
	if err != nil {
		fontEngine.Close()
		return nil, fmt.Errorf("creating window: %w", err)
	}

	win.SetOpacity(cfg.Opacity)

	canvas := render.NewCanvas(fontEngine)
	canvas.Resize(int(initWidth), int(initHeight))

	systemIsDark, systemSchemeKnown := platform.DetectSystemColorScheme()
	if !systemSchemeKnown {
		// M30: unknown is not dark — say so instead of silently assuming.
		fmt.Fprintln(os.Stderr, "terrat: could not detect system color scheme; assuming dark")
	}
	activeTheme := terminal.ResolveTheme(cfg.Theme, systemIsDark)

	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		config:           cfg,
		win:              win,
		canvas:           canvas,
		fontEngine:       fontEngine,
		activeTheme:      activeTheme,
		systemIsDark:     systemIsDark,
		nextTabID:        1,
		customCmd:        customCmd,
		currentTitle:     initialTitle,
		currentWidth:     initWidth,
		currentHeight:    initHeight,
		currentCols:      canvas.Cols(),
		currentRows:      canvas.Rows(),
		charW:            fontEngine.CharWidth(),
		charH:            fontEngine.CharHeight(),
		gridStartX:       render.PaddingLeft,
		gridStartY:       render.HeaderHeight + render.PaddingTop,
		redrawCh:         make(chan struct{}, 1),
		tabExitNotifyCh:  make(chan int, 16),
		ptyDataNotifyCh:  make(chan int, 128),
		tabTitleNotifyCh: make(chan tabTitleMsg, 32),
		savedThemeID:     cfg.Theme,
		suggestEngine:    autosuggest.NewEngine(),
		ctx:              ctx,
		cancel:           cancel,
	}

	initTab, err := app.createTab(canvas.Cols(), canvas.Rows(), initWidth, initHeight, customCmd...)
	if err != nil {
		app.Close()
		return nil, fmt.Errorf("starting shell PTY: %w", err)
	}
	app.tabs = append(app.tabs, initTab)
	app.activeTermPtr.Store(initTab.Term)
	// M9: feed the active terminal's application-cursor state to the key
	// translator, so SS3 sequences are produced for every key (including
	// keypad and modified arrows), not just the plain arrows the old
	// workaround in handleEvent covered.
	win.AppCursorFunc = app.isAppCursor
	win.AltScreenFunc = app.isAltScreen

	return app, nil
}

// isAppCursor reports whether the active terminal is in application-cursor
// mode (DECCKM). It backs Window.AppCursorFunc, which the window event loop
// calls from its own goroutine — the atomic pointer keeps the read safe.
// A nil terminal (no tabs) means normal-cursor mode.
func (a *App) isAppCursor() bool {
	if t := a.activeTermPtr.Load(); t != nil {
		return t.AppCursor()
	}
	return false
}

// isAltScreen reports whether the active terminal is on the alternate
// screen. It backs Window.AltScreenFunc (L42): Shift+PgUp/PgDn/Home/End
// must reach fullscreen apps instead of scrolling the terminal.
func (a *App) isAltScreen() bool {
	if t := a.activeTermPtr.Load(); t != nil {
		return t.IsAlt()
	}
	return false
}

// reportTabError surfaces a failed tab creation (M18). Without it a
// broken shell binary made Ctrl+Shift+T / the new-tab button silently do
// nothing at all. There is no in-window status UI yet, so the error goes
// to stderr, where it is visible when Terrat is launched from a terminal.
func (a *App) reportTabError(err error) {
	fmt.Fprintf(os.Stderr, "terrat: failed to create tab: %v\n", err)
}

func (a *App) createTab(cols, rows int, width, height uint16, cmd ...string) (*Tab, error) {
	tab, err := createTab(
		a.ctx,
		a.nextTabID,
		cols,
		rows,
		width,
		height,
		a.activeTheme,
		a.tabExitNotifyCh,
		a.ptyDataNotifyCh,
		a.tabTitleNotifyCh,
		cmd...,
	)
	if err != nil {
		return nil, err
	}
	a.nextTabID++
	return tab, nil
}

func (a *App) triggerRedraw() {
	select {
	case a.redrawCh <- struct{}{}:
	default:
	}
}

// tabIndexByID returns the index of the tab with the given ID, or -1.
// Hitboxes are rebuilt on every render, so a click can arrive with
// hitboxes from before a tab closed; matching by stable tab ID (M19)
// instead of by position keeps a stale hitbox from closing or switching
// the wrong tab.
func (a *App) tabIndexByID(id int) int {
	for i, t := range a.tabs {
		if t.ID == id {
			return i
		}
	}
	return -1
}

func (a *App) switchTab(idx int) {
	if idx < 0 || idx >= len(a.tabs) {
		return
	}
	a.isSelecting = false
	a.isDraggingScrollbar = false
	a.dragScrollDelta = 0
	a.activeTabIdx = idx
	a.activeTermPtr.Store(a.tabs[idx].Term)
	a.tabs[a.activeTabIdx].HasBell = false
	title := a.tabs[a.activeTabIdx].Title
	if title == "" {
		title = "bash"
	}
	a.currentTitle = title
	a.win.SetTitle(AppName + " - " + title)
	a.currentInputBuffer = ""
	a.activeDiag = nil
	a.updateGhostText()
	a.updateDiagnostics()
	a.triggerRedraw()
}

func (a *App) closeTab(idx int) {
	if idx < 0 || idx >= len(a.tabs) {
		return
	}
	closingTab := a.tabs[idx]
	closingTab.Close()

	copy(a.tabs[idx:], a.tabs[idx+1:])
	a.tabs[len(a.tabs)-1] = nil
	a.tabs = a.tabs[:len(a.tabs)-1]
	if len(a.tabs) == 0 {
		// Propagate the shell/command exit code (M13): `terrat -e <cmd>`
		// used to always exit 0 even when the command failed. Unknown
		// status (e.g. user closed the tab by hand) still exits 0.
		// L20: record the code and shut down via the Run loop instead of
		// os.Exit, so deferred cleanup in main still runs.
		code := closingTab.ExitCode()
		if code < 0 {
			code = 0
		}
		a.exitCode = code
		a.Close()
	}

	targetIdx := a.activeTabIdx
	if targetIdx >= len(a.tabs) {
		targetIdx = len(a.tabs) - 1
	} else if idx < targetIdx {
		targetIdx--
	}
	if targetIdx < 0 {
		targetIdx = 0
	}

	a.switchTab(targetIdx)
}

func (a *App) applyZoom(action platform.ActionType) {
	var changed bool
	switch action {
	case platform.ActionZoomIn:
		changed = a.fontEngine.ZoomIn()
	case platform.ActionZoomOut:
		changed = a.fontEngine.ZoomOut()
	case platform.ActionZoomReset:
		changed = a.fontEngine.ZoomReset()
	}
	if !changed {
		return
	}

	a.charW = a.fontEngine.CharWidth()
	a.charH = a.fontEngine.CharHeight()

	a.canvas.Resize(int(a.currentWidth), int(a.currentHeight))
	newCols := a.canvas.Cols()
	newRows := a.canvas.Rows()

	for _, t := range a.tabs {
		t.Term.Resize(newCols, newRows)
		_ = t.PTY.Resize(uint16(newCols), uint16(newRows), a.currentWidth, a.currentHeight)
	}

	a.currentCols = newCols
	a.currentRows = newRows

	a.config.FontSize = a.fontEngine.FontSize()
	_ = config.Save(a.config)

	if a.isSearchOpen {
		a.updateSearchMatches()
	}

	a.triggerRedraw()
}

func (a *App) isPasswordPrompt(term *terminal.Terminal) bool {
	if term == nil {
		return false
	}
	_, cy, _ := term.Cursor()
	rowStr := strings.ToLower(term.GetRowString(cy))
	lower := strings.TrimSpace(rowStr)
	if strings.Contains(lower, "[sudo]") {
		return true
	}
	if strings.HasSuffix(lower, ":") || strings.HasSuffix(lower, ": ") {
		if strings.Contains(lower, "password") || strings.Contains(lower, "passphrase") || strings.Contains(lower, "pin") {
			return true
		}
	}
	return false
}

func (a *App) updateGhostText() {
	if a.activeTabIdx >= len(a.tabs) {
		a.activeGhostText = ""
		return
	}
	curTab := a.tabs[a.activeTabIdx]
	if !a.config.GhostText || curTab.Term.IsAlt() || a.isSearchOpen || a.isPrefOpen {
		a.activeGhostText = ""
		return
	}
	if !curTab.PTY.IsForegroundShell() || a.isPasswordPrompt(curTab.Term) {
		a.activeGhostText = ""
		return
	}
	trimmed := strings.TrimLeft(a.currentInputBuffer, " ")
	if len(trimmed) >= 1 {
		a.activeGhostText = a.suggestEngine.Suggest(trimmed, curTab.PTY.GetCwd())
	} else {
		a.activeGhostText = ""
	}
}

func (a *App) updateDiagnostics() {
	if a.activeTabIdx >= len(a.tabs) {
		a.activeDiag = nil
		return
	}
	curTab := a.tabs[a.activeTabIdx]
	if !a.config.Diagnostics || curTab.Term.IsAlt() || a.isSearchOpen || a.isPrefOpen {
		a.activeDiag = nil
		return
	}
	if !curTab.PTY.IsForegroundShell() || a.isPasswordPrompt(curTab.Term) {
		a.activeDiag = nil
		return
	}
	trimmed := strings.TrimLeft(a.currentInputBuffer, " ")
	diag := diagnostics.Analyze(trimmed)
	if diag != nil {
		a.activeDiag = &render.DiagnosticInfo{
			IsError:    diag.Severity == diagnostics.SeverityError,
			Message:    diag.Message,
			Suggestion: diag.Suggestion,
			QuickFix:   diag.QuickFix,
		}
	} else {
		a.activeDiag = nil
	}
}

func (a *App) updateSearchMatches() {
	if a.activeTabIdx >= len(a.tabs) {
		a.searchMatches = nil
		return
	}
	a.searchMatches = findSearchMatches(a.tabs[a.activeTabIdx].Term, a.searchQuery)
	a.activeSearchIdx = 0
}

// scrollToActiveMatch scrolls the view just enough to bring the active
// search match into view. It is called only for user-initiated search
// actions (typing, navigating); background recomputes on PTY output or
// resize deliberately do not move the view.
func (a *App) scrollToActiveMatch() {
	if len(a.searchMatches) == 0 || a.activeSearchIdx < 0 || a.activeSearchIdx >= len(a.searchMatches) {
		return
	}
	if a.activeTabIdx >= len(a.tabs) {
		return
	}
	term := a.tabs[a.activeTabIdx].Term
	if term.IsAlt() {
		return
	}
	absRow := a.searchMatches[a.activeSearchIdx].Row
	sbLen := term.ScrollbackLen()
	rows := term.Rows()
	if rows <= 0 {
		return
	}
	top := sbLen - term.ScrollOff() // absolute row at viewport row 0
	if absRow < top {
		term.SetScrollOff(sbLen - absRow)
	} else if absRow >= top+rows {
		term.SetScrollOff(sbLen - (absRow - rows + 1))
	}
}

func (a *App) renderScreen(cursorBlink bool) {
	if len(a.tabs) == 0 || a.activeTabIdx >= len(a.tabs) {
		return
	}
	tabInfos := make([]render.TabInfo, len(a.tabs))
	for i, t := range a.tabs {
		tabInfos[i] = render.TabInfo{
			ID:      t.ID,
			Title:   t.Title,
			Active:  i == a.activeTabIdx,
			HasBell: t.HasBell,
		}
	}

	activeTerm := a.tabs[a.activeTabIdx].Term
	hud := fmt.Sprintf("%dx%d", a.canvas.Cols(), a.canvas.Rows())
	// L35: sanitizer warnings were computed and then dropped — show them
	// in the header for a few seconds instead of the dimensions.
	if a.pasteWarning != "" {
		if time.Now().Before(a.pasteWarningUntil) {
			hud = a.pasteWarning
			if len([]rune(hud)) > 60 {
				hud = string([]rune(hud)[:57]) + "..."
			}
		} else {
			a.pasteWarning = ""
		}
	}
	a.canvas.Render(activeTerm, cursorBlink, a.currentTitle, hud, tabInfos, a.searchMatches, a.activeSearchIdx, a.hoveredURL, a.activeGhostText, a.activeDiag)

	if a.isSearchOpen {
		matchCount := 0
		if len(a.searchMatches) > 0 {
			matchCount = a.activeSearchIdx + 1
		}
		a.canvas.RenderSearchBar(activeTerm.Theme(), a.searchQuery, matchCount, len(a.searchMatches))
	}

	if a.isPrefOpen {
		a.canvas.RenderPreferencesModal(activeTerm.Theme(), a.prefIndex, getSettingsItems(a.config, a.isSearchOpen), a.savedThemeID)
	}

	a.win.Blit(a.canvas.Pixels, a.currentWidth, a.currentHeight)
}

// writePTY sends data to a tab's PTY. Write errors used to be silently
// dropped everywhere (L19): typing into a dead PTY vanished without a
// trace. Real failures are now logged; a closed PTY is expected during
// shutdown and stays quiet.
func writePTY(p *pty.TerminalPTY, data []byte) {
	if _, err := p.Write(data); err != nil && !errors.Is(err, os.ErrClosed) {
		fmt.Fprintf(os.Stderr, "terrat: failed to write to PTY: %v\n", err)
	}
}

func (a *App) doWritePastedText(text string) {
	if len(a.tabs) == 0 || a.activeTabIdx >= len(a.tabs) {
		return
	}
	// Never let pasted text smuggle its own bracketed-paste terminators:
	// an inner ESC[201~ would end the bracketed paste early and let the
	// following lines execute as commands (pastejacking). Strip them
	// unconditionally, even when paste sanitizing is turned off.
	text = strings.ReplaceAll(text, "\x1b[200~", "")
	text = strings.ReplaceAll(text, "\x1b[201~", "")
	text = strings.ReplaceAll(text, "\u009b200~", "")
	text = strings.ReplaceAll(text, "\u009b201~", "")
	activeTerm := a.tabs[a.activeTabIdx].Term
	activePTY := a.tabs[a.activeTabIdx].PTY
	var toSend []byte
	if a.config.BracketedPaste && activeTerm.BracketedPaste() {
		toSend = []byte("\x1b[200~" + text + "\x1b[201~")
	} else {
		toSend = []byte(text)
	}
	writePTY(activePTY, toSend)
	activeTerm.ResetScroll()
	a.triggerRedraw()
}

func (a *App) handlePastedData(pastedBytes []byte) {
	if len(pastedBytes) == 0 || len(a.tabs) == 0 || a.activeTabIdx >= len(a.tabs) {
		return
	}
	raw := string(pastedBytes)
	var text string

	if a.config.SanitizePaste {
		res := paste.Sanitize(raw)
		text = res.Sanitized
		// L35: the warnings used to be computed and discarded — keep
		// them for the HUD so the user sees what was stripped.
		if len(res.Warnings) > 0 {
			a.pasteWarning = strings.Join(res.Warnings, "; ")
			a.pasteWarningUntil = time.Now().Add(4 * time.Second)
		}
	} else {
		text = raw
	}

	if text == "" {
		return
	}

	// L16: the ghost-text/diagnostics guess tracks the input line, so a
	// paste must update it too: a single-line paste appends to the line,
	// a multi-line paste executes it (and anything pasted while an app
	// runs is not shell input at all).
	activeTerm := a.tabs[a.activeTabIdx].Term
	activePTY := a.tabs[a.activeTabIdx].PTY
	if !activePTY.IsForegroundShell() || a.isPasswordPrompt(activeTerm) || strings.Contains(text, "\n") {
		a.currentInputBuffer = ""
	} else {
		a.currentInputBuffer += text
	}
	a.updateGhostText()
	a.updateDiagnostics()

	a.doWritePastedText(text)
}

func (a *App) updateScrollbar(eventY int) {
	if a.activeTabIdx >= len(a.tabs) {
		return
	}
	activeTerm := a.tabs[a.activeTabIdx].Term
	maxScroll := activeTerm.ScrollbackLen()
	if maxScroll <= 0 {
		return
	}
	trackY := render.HeaderHeight + 4
	trackH := int(a.currentHeight) - render.HeaderHeight - 8
	if trackH <= 24 {
		return
	}
	totalLines := maxScroll + a.canvas.Rows()
	thumbH := (a.canvas.Rows() * trackH) / totalLines
	if thumbH < 20 {
		thumbH = 20
	}
	if thumbH > trackH {
		thumbH = trackH
	}
	availH := trackH - thumbH
	if availH <= 0 {
		return
	}
	clickY := eventY - trackY - (thumbH / 2)
	if clickY < 0 {
		clickY = 0
	}
	if clickY > availH {
		clickY = availH
	}
	progress := float64(clickY) / float64(availH)
	targetOff := int(float64(maxScroll)*(1.0-progress) + 0.5)
	activeTerm.SetScrollOff(targetOff)
	a.triggerRedraw()
}

// ExitCode is the process exit code recorded when the last tab closed
// (M13/L20); main exits with it after Run returns.
func (a *App) ExitCode() int {
	return a.exitCode
}

func (a *App) Close() {
	a.closeOnce.Do(func() {
		if a.cancel != nil {
			a.cancel()
		}
		for _, t := range a.tabs {
			t.Close()
		}
		// Wait briefly for the tab goroutines (PTY readers, title and
		// response pumps) to finish, so process exit cannot race them.
		// The context is already cancelled, so a stuck goroutine can
		// only delay this by the timeout.
		done := make(chan struct{})
		go func() {
			for _, t := range a.tabs {
				<-t.Done()
			}
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
		if a.win != nil {
			a.win.Close()
		}
		if a.fontEngine != nil {
			a.fontEngine.Close()
		}
	})
}

func (a *App) Run() error {
	cursorTicker := time.NewTicker(500 * time.Millisecond)
	defer cursorTicker.Stop()
	cursorBlink := true

	var autoScrollTicker *time.Ticker
	defer func() {
		if autoScrollTicker != nil {
			autoScrollTicker.Stop()
		}
	}()

	a.renderScreen(cursorBlink)

	xEventCh := a.win.Events()
	var pendingEvent platform.Event

	for {
		var ev platform.Event
		var ok bool

		if pendingEvent != nil {
			ev = pendingEvent
			pendingEvent = nil
			ok = true
		} else {
			var autoScrollCh <-chan time.Time
			if a.isSelecting && a.dragScrollDelta != 0 {
				if autoScrollTicker == nil {
					autoScrollTicker = time.NewTicker(40 * time.Millisecond)
				}
				autoScrollCh = autoScrollTicker.C
			} else if autoScrollTicker != nil {
				autoScrollTicker.Stop()
				autoScrollTicker = nil
			}

			select {
			case <-a.ctx.Done():
				return nil

			case tabID := <-a.tabExitNotifyCh:
				for i, t := range a.tabs {
					if t.ID == tabID {
						a.closeTab(i)
						break
					}
				}
				continue

			case tabID := <-a.ptyDataNotifyCh:
				activeUpdated := a.activeTabIdx < len(a.tabs) && a.tabs[a.activeTabIdx].ID == tabID
				redrawNeeded := activeUpdated
				if !activeUpdated {
					for i := range a.tabs {
						if a.tabs[i].ID == tabID {
							redrawNeeded = !a.tabs[i].HasBell
							a.tabs[i].HasBell = true
							break
						}
					}
				}
				draining := true
				for drained := 0; draining && drained < cap(a.ptyDataNotifyCh); drained++ {
					select {
					case nextID := <-a.ptyDataNotifyCh:
						if a.activeTabIdx < len(a.tabs) && a.tabs[a.activeTabIdx].ID == nextID {
							activeUpdated = true
							redrawNeeded = true
						} else {
							for i := range a.tabs {
								if a.tabs[i].ID == nextID {
									redrawNeeded = redrawNeeded || !a.tabs[i].HasBell
									a.tabs[i].HasBell = true
									break
								}
							}
						}
					default:
						draining = false
					}
				}
				if activeUpdated && a.isSearchOpen {
					a.updateSearchMatches()
				}
				if redrawNeeded {
					a.triggerRedraw()
				}
				continue

			case tm := <-a.tabTitleNotifyCh:
				for i := range a.tabs {
					if a.tabs[i].ID == tm.tabID {
						a.tabs[i].Title = tm.title
						if i == a.activeTabIdx {
							a.currentTitle = tm.title
							a.win.SetTitle(AppName + " - " + tm.title)
							a.triggerRedraw()
						}
						break
					}
				}
				continue

			case <-cursorTicker.C:
				cursorBlink = !cursorBlink
				a.triggerRedraw()
				continue

			case <-a.redrawCh:
				drainChannel(a.redrawCh)
				a.renderScreen(cursorBlink)
				continue

			case <-autoScrollCh:
				if a.isSelecting && a.dragScrollDelta != 0 && a.activeTabIdx < len(a.tabs) {
					activeTerm := a.tabs[a.activeTabIdx].Term
					activeTerm.ScrollKeepSelection(a.dragScrollDelta)
					if a.dragScrollDelta > 0 {
						activeTerm.UpdateSelection(a.lastDragCol, 0)
					} else {
						activeTerm.UpdateSelection(a.lastDragCol, a.canvas.Rows()-1)
					}
					a.triggerRedraw()
				}
				continue

			case ev, ok = <-xEventCh:
				if !ok {
					return nil
				}
			}
		}

		if shouldExit := a.handleEvent(ev, &pendingEvent, xEventCh); shouldExit {
			return nil
		}

		runtime.Gosched()
	}
}
