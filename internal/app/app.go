package app

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"terrat/assets"
	"terrat/internal/autosuggest"
	"terrat/internal/config"
	"terrat/internal/diagnostics"
	"terrat/internal/paste"
	"terrat/internal/platform"
	"terrat/internal/render"
	"terrat/internal/terminal"
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
	nextTabID    int
	customCmd    []string

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
}

func NewApp(cfg *config.Config, initialTitle string, customCmd []string) (*App, error) {
	fontEngine, err := render.NewFontEngine(cfg.FontSize)
	if err != nil {
		return nil, fmt.Errorf("initializing font engine: %w", err)
	}

	initCols := 120
	initRows := 32
	if runtime.GOOS == "windows" {
		initCols = 128
		initRows = 33
	}

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

	systemIsDark := platform.DetectSystemColorScheme()
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

	return app, nil
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

func (a *App) switchTab(idx int) {
	if idx < 0 || idx >= len(a.tabs) {
		return
	}
	a.isSelecting = false
	a.isDraggingScrollbar = false
	a.dragScrollDelta = 0
	a.activeTabIdx = idx
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

	a.tabs = append(a.tabs[:idx], a.tabs[idx+1:]...)
	if len(a.tabs) == 0 {
		a.Close()
		os.Exit(0)
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

func (a *App) doWritePastedText(text string) {
	if len(a.tabs) == 0 || a.activeTabIdx >= len(a.tabs) {
		return
	}
	activeTerm := a.tabs[a.activeTabIdx].Term
	activePTY := a.tabs[a.activeTabIdx].PTY
	var toSend []byte
	if a.config.BracketedPaste && activeTerm.BracketedPaste() {
		toSend = []byte("\x1b[200~" + text + "\x1b[201~")
	} else {
		toSend = []byte(text)
	}
	_, _ = activePTY.Write(toSend)
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
	} else {
		text = raw
	}

	if text == "" {
		return
	}

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

func (a *App) Close() {
	if a.cancel != nil {
		a.cancel()
	}
	for _, t := range a.tabs {
		t.Close()
	}
	if a.win != nil {
		a.win.Close()
	}
	if a.fontEngine != nil {
		a.fontEngine.Close()
	}
}

func (a *App) Run() error {
	cursorTicker := time.NewTicker(500 * time.Millisecond)
	defer cursorTicker.Stop()
	cursorBlink := true

	autoScrollTicker := time.NewTicker(40 * time.Millisecond)
	defer autoScrollTicker.Stop()

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
				if a.activeTabIdx < len(a.tabs) && a.tabs[a.activeTabIdx].ID == tabID {
					if a.isSearchOpen {
						a.updateSearchMatches()
					}
					a.triggerRedraw()
				} else {
					for i := range a.tabs {
						if a.tabs[i].ID == tabID {
							a.tabs[i].HasBell = true
							a.triggerRedraw()
							break
						}
					}
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

			case <-autoScrollTicker.C:
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
