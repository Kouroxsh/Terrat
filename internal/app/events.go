package app

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"terrat/internal/config"
	"terrat/internal/diagnostics"
	"terrat/internal/platform"
	"terrat/internal/render"
	"terrat/internal/terminal"
)

func (a *App) toCellCoords(pixelX, pixelY int) (int, int) {
	cx := (pixelX - a.gridStartX) / a.charW
	cy := (pixelY - a.gridStartY) / a.charH
	if cx < 0 {
		cx = 0
	}
	if cx >= a.canvas.Cols() {
		cx = a.canvas.Cols() - 1
	}
	if cy < 0 {
		cy = 0
	}
	if cy >= a.canvas.Rows() {
		cy = a.canvas.Rows() - 1
	}
	return cx, cy
}

func (a *App) sendMouseEvent(btn int, press bool, pixelX, pixelY int, state uint16) bool {
	if a.activeTabIdx < 0 || a.activeTabIdx >= len(a.tabs) {
		return false
	}
	curTerm := a.tabs[a.activeTabIdx].Term
	curPTY := a.tabs[a.activeTabIdx].PTY
	if !curTerm.MouseTrackingLocked() {
		return false
	}
	cx, cy := a.toCellCoords(pixelX, pixelY)
	col := cx + 1
	row := cy + 1

	cb := btn
	if (state & platform.ModShift) != 0 {
		cb |= 4
	}
	if (state & platform.ModAlt) != 0 {
		cb |= 8
	}
	if (state & platform.ModCtrl) != 0 {
		cb |= 16
	}

	if curTerm.MouseSGRLocked() {
		terminator := 'M'
		if !press {
			terminator = 'm'
		}
		seq := fmt.Sprintf("\x1b[<%d;%d;%d%c", cb, col, row, terminator)
		_, _ = curPTY.Write([]byte(seq))
		return true
	} else if press {
		if col > 223 {
			col = 223
		}
		if row > 223 {
			row = 223
		}
		b := []byte{0x1b, '[', 'M', byte(cb + 32), byte(col + 32), byte(row + 32)}
		_, _ = curPTY.Write(b)
		return true
	}
	return false
}

func (a *App) executeSettingsAction(item render.PrefOption, isSecondaryAction bool) {
	switch item.ID {
	case "diagnostics":
		a.config.Diagnostics = !a.config.Diagnostics
		_ = config.Save(a.config)
		if !a.config.Diagnostics {
			a.activeDiag = nil
		} else {
			a.updateDiagnostics()
		}
	case "ghost_text":
		a.config.GhostText = !a.config.GhostText
		_ = config.Save(a.config)
		if !a.config.GhostText {
			a.activeGhostText = ""
		} else {
			a.updateGhostText()
		}
	case "sanitize_paste":
		a.config.SanitizePaste = !a.config.SanitizePaste
		_ = config.Save(a.config)
	case "search":
		a.isSearchOpen = !a.isSearchOpen
		if a.isSearchOpen {
			a.isPrefOpen = false
			a.updateSearchMatches()
		}
	case "theme":
		themesList := []string{"tokyo-night", "catppuccin-mocha", "minecraft", "tokyo-day", "solarized-light", "auto"}
		curIdx := 0
		for i, thID := range themesList {
			if thID == a.config.Theme {
				curIdx = i
				break
			}
		}
		nextIdx := (curIdx + 1) % len(themesList)
		if isSecondaryAction {
			nextIdx = (curIdx - 1 + len(themesList)) % len(themesList)
		}
		a.config.Theme = themesList[nextIdx]
		a.savedThemeID = a.config.Theme
		_ = config.Save(a.config)
		a.activeTheme = terminal.ResolveTheme(a.savedThemeID, a.systemIsDark)
		for _, t := range a.tabs {
			t.Term.SetTheme(a.activeTheme)
		}
	case "font_size":
		if isSecondaryAction {
			a.applyZoom(platform.ActionZoomOut)
		} else {
			a.applyZoom(platform.ActionZoomIn)
		}
	case "opacity":
		opacities := []float64{1.0, 0.95, 0.90, 0.85, 0.80, 0.75}
		curIdx := 0
		for i, op := range opacities {
			diff := op - a.config.Opacity
			if diff < 0 {
				diff = -diff
			}
			if diff < 0.02 {
				curIdx = i
				break
			}
		}
		nextIdx := (curIdx + 1) % len(opacities)
		if isSecondaryAction {
			nextIdx = (curIdx - 1 + len(opacities)) % len(opacities)
		}
		a.config.Opacity = opacities[nextIdx]
		_ = config.Save(a.config)
		a.win.SetOpacity(a.config.Opacity)
	}
	a.triggerRedraw()
}

func (a *App) handleEvent(ev platform.Event, pendingEvent *platform.Event, xEventCh <-chan platform.Event) bool {
	if a.activeTabIdx < 0 || a.activeTabIdx >= len(a.tabs) {
		return false
	}
	activeTerm := a.tabs[a.activeTabIdx].Term
	activePTY := a.tabs[a.activeTabIdx].PTY

	switch e := ev.(type) {
	case platform.MotionNotifyEvent:
		lastMotion := e
	drainMotion:
		for {
			select {
			case nextEv, ok := <-xEventCh:
				if !ok {
					return true
				}
				if m, isMotion := nextEv.(platform.MotionNotifyEvent); isMotion {
					lastMotion = m
					continue
				}
				*pendingEvent = nextEv
				break drainMotion
			default:
				break drainMotion
			}
		}
		e = lastMotion

		if (e.State & platform.ButtonMask1) == 0 {
			if a.isSelecting {
				a.isSelecting = false
				a.dragScrollDelta = 0
			}
			if a.isDraggingScrollbar {
				a.isDraggingScrollbar = false
			}
		}

		if a.isDraggingScrollbar {
			a.updateScrollbar(int(e.EventY))
			return false
		}

		if (e.State&platform.ButtonMask1) != 0 && a.isSelecting {
			rawY := int(e.EventY)
			rawX := int(e.EventX)
			gridEndY := a.gridStartY + (a.canvas.Rows() * a.charH)
			cx, cy := a.toCellCoords(rawX, rawY)
			a.lastDragCol = cx

			if rawY < a.gridStartY {
				dist := (a.gridStartY-rawY)/a.charH + 1
				if dist > 5 {
					dist = 5
				}
				a.dragScrollDelta = dist
				activeTerm.ScrollKeepSelection(a.dragScrollDelta)
				activeTerm.UpdateSelection(cx, 0)
			} else if rawY >= gridEndY {
				dist := (rawY-gridEndY)/a.charH + 1
				if dist > 5 {
					dist = 5
				}
				a.dragScrollDelta = -dist
				activeTerm.ScrollKeepSelection(a.dragScrollDelta)
				activeTerm.UpdateSelection(cx, a.canvas.Rows()-1)
			} else {
				a.dragScrollDelta = 0
				activeTerm.UpdateSelection(cx, cy)
			}
			a.triggerRedraw()
		} else {
			isCtrl := (e.State & platform.ModCtrl) != 0
			if isCtrl && int(e.EventY) >= render.HeaderHeight {
				cx, cy := a.toCellCoords(int(e.EventX), int(e.EventY))
				rowStr := activeTerm.GetRowString(cy)
				matches := urlRegex.FindAllStringIndex(rowStr, -1)
				var foundURL *render.URLRange
				for _, m := range matches {
					startCol, endCol := m[0], m[1]
					if cx >= startCol && cx < endCol {
						rawURL := rowStr[startCol:endCol]
						urlStr := strings.TrimRight(rawURL, ".,;:!?)'\"")
						foundURL = &render.URLRange{
							Row:      cy,
							StartCol: startCol,
							EndCol:   startCol + len(urlStr) - 1,
							URL:      urlStr,
						}
						break
					}
				}
				if foundURL != nil {
					if a.hoveredURL == nil || a.hoveredURL.URL != foundURL.URL || a.hoveredURL.Row != foundURL.Row || a.hoveredURL.StartCol != foundURL.StartCol {
						a.hoveredURL = foundURL
						a.win.SetCursorType(platform.CursorPointer)
						a.triggerRedraw()
					}
				} else {
					if a.hoveredURL != nil {
						a.hoveredURL = nil
						a.win.SetCursorType(platform.CursorText)
						a.triggerRedraw()
					}
				}
			} else {
				if a.hoveredURL != nil {
					a.hoveredURL = nil
					a.triggerRedraw()
				}

				dir := platform.GetResizeDirection(int(e.EventX), int(e.EventY), int(a.currentWidth), int(a.currentHeight), 8)
				switch dir {
				case platform.ResizeTopLeft:
					a.win.SetCursorType(platform.CursorResizeTopLeft)
				case platform.ResizeTop:
					a.win.SetCursorType(platform.CursorResizeTop)
				case platform.ResizeTopRight:
					a.win.SetCursorType(platform.CursorResizeTopRight)
				case platform.ResizeRight:
					a.win.SetCursorType(platform.CursorResizeRight)
				case platform.ResizeBottomRight:
					a.win.SetCursorType(platform.CursorResizeBottomRight)
				case platform.ResizeBottom:
					a.win.SetCursorType(platform.CursorResizeBottom)
				case platform.ResizeBottomLeft:
					a.win.SetCursorType(platform.CursorResizeBottomLeft)
				case platform.ResizeLeft:
					a.win.SetCursorType(platform.CursorResizeLeft)
				default:
					if int(e.EventX) >= int(a.currentWidth)-16 && int(e.EventY) >= render.HeaderHeight && activeTerm.ScrollbackLen() > 0 {
						a.win.SetCursorType(platform.CursorDefault)
					} else if int(e.EventY) < render.HeaderHeight {
						mx := int(e.EventX)
						isPointer := false
						if a.canvas.NewTabHitBox[1] > 0 && mx >= a.canvas.NewTabHitBox[0] && mx <= a.canvas.NewTabHitBox[1] {
							isPointer = true
						} else if mx <= 65 {
							isPointer = true
						} else {
							for _, hb := range a.canvas.TabHitBoxes {
								if mx >= hb.CloseX && mx <= hb.CloseEndX {
									isPointer = true
									break
								}
							}
						}
						if isPointer {
							a.win.SetCursorType(platform.CursorPointer)
						} else {
							a.win.SetCursorType(platform.CursorDefault)
						}
					} else {
						a.win.SetCursorType(platform.CursorText)
					}
				}
			}
		}

	case platform.ButtonPressEvent:
		isCtrl := (e.State & platform.ModCtrl) != 0

		if isCtrl {
			if e.Detail == 4 {
				a.applyZoom(platform.ActionZoomIn)
				return false
			} else if e.Detail == 5 {
				a.applyZoom(platform.ActionZoomOut)
				return false
			}
		}

		if e.Detail == 1 {
			if isCtrl && a.hoveredURL != nil {
				openURL(a.hoveredURL.URL)
				return false
			}

			if a.isPrefOpen {
				items := getSettingsItems(a.config, a.isSearchOpen)
				modalX := a.canvas.PrefModalBox[0]
				modalY := a.canvas.PrefModalBox[1]
				modalW := a.canvas.PrefModalBox[2]
				modalH := a.canvas.PrefModalBox[3]
				rowH := a.canvas.PrefModalBox[4]
				headerH := 36
				if a.activeTheme.ID == "minecraft" {
					headerH = 40
				}

				px := int(e.EventX)
				py := int(e.EventY)

				if px >= modalX && px < modalX+modalW && py >= modalY && py < modalY+modalH {
					optStartY := modalY + headerH + 4
					if py >= optStartY && py < optStartY+len(items)*rowH {
						clickedIdx := (py - optStartY) / rowH
						if clickedIdx >= 0 && clickedIdx < len(items) {
							a.prefIndex = clickedIdx
							a.executeSettingsAction(items[clickedIdx], false)
						}
					}
				} else {
					a.isPrefOpen = false
					a.triggerRedraw()
				}
				return false
			}

			dir := platform.GetResizeDirection(int(e.EventX), int(e.EventY), int(a.currentWidth), int(a.currentHeight), 8)
			if dir != platform.ResizeNone {
				a.win.StartResize(dir, e.RootX, e.RootY)
				return false
			}

			if int(e.EventX) >= int(a.currentWidth)-16 && int(e.EventY) >= render.HeaderHeight {
				if activeTerm.ScrollbackLen() > 0 {
					a.isDraggingScrollbar = true
					a.updateScrollbar(int(e.EventY))
					return false
				}
			}

			if int(e.EventY) < render.HeaderHeight {
				if int(e.EventX) >= int(a.currentWidth)-36 {
					a.Close()
					os.Exit(0)
				} else if int(e.EventX) >= int(a.currentWidth)-72 && int(e.EventX) < int(a.currentWidth)-36 {
					a.win.ToggleMaximize()
					return false
				} else if int(e.EventX) >= int(a.currentWidth)-108 && int(e.EventX) < int(a.currentWidth)-72 {
					a.win.Minimize()
					return false
				} else if int(e.EventX) >= int(a.currentWidth)-144 && int(e.EventX) < int(a.currentWidth)-108 {
					a.isPrefOpen = !a.isPrefOpen
					a.prefIndex = 0
					a.triggerRedraw()
					return false
				}

				if a.canvas.NewTabHitBox[1] > 0 && int(e.EventX) >= a.canvas.NewTabHitBox[0] && int(e.EventX) <= a.canvas.NewTabHitBox[1] {
					newTab, err := a.createTab(a.canvas.Cols(), a.canvas.Rows(), a.currentWidth, a.currentHeight)
					if err == nil {
						a.tabs = append(a.tabs, newTab)
						a.switchTab(len(a.tabs) - 1)
					}
					return false
				}

				for idx, thb := range a.canvas.TabHitBoxes {
					if int(e.EventX) >= thb.CloseX && int(e.EventX) <= thb.CloseEndX {
						a.closeTab(idx)
						return false
					}
				}

				for idx, thb := range a.canvas.TabHitBoxes {
					if int(e.EventX) >= thb.StartX && int(e.EventX) <= thb.EndX {
						if idx != a.activeTabIdx {
							a.switchTab(idx)
						}
						return false
					}
				}

				now := time.Now()
				if now.Sub(a.lastHeaderClickTime) < 300*time.Millisecond {
					dx := int(e.EventX) - a.lastHeaderClickX
					dy := int(e.EventY) - a.lastHeaderClickY
					if dx < 0 {
						dx = -dx
					}
					if dy < 0 {
						dy = -dy
					}
					if dx <= 5 && dy <= 5 {
						a.win.ToggleMaximize()
						a.lastHeaderClickTime = time.Time{}
						return false
					}
				}
				a.lastHeaderClickTime = now
				a.lastHeaderClickX = int(e.EventX)
				a.lastHeaderClickY = int(e.EventY)

				a.win.StartDrag(e.RootX, e.RootY)
				return false
			}

			if activeTerm.MouseTrackingLocked() && (e.State&platform.ModShift) == 0 {
				if a.sendMouseEvent(0, true, int(e.EventX), int(e.EventY), e.State) {
					return false
				}
			}

			now := time.Now()
			cx, cy := a.toCellCoords(int(e.EventX), int(e.EventY))
			dx := int(e.EventX) - a.lastClickX
			dy := int(e.EventY) - a.lastClickY
			if dx < 0 {
				dx = -dx
			}
			if dy < 0 {
				dy = -dy
			}
			if now.Sub(a.lastClickTime) < 350*time.Millisecond && dx <= 5 && dy <= 5 {
				a.clickCount++
			} else {
				a.clickCount = 1
			}
			a.lastClickTime = now
			a.lastClickX = int(e.EventX)
			a.lastClickY = int(e.EventY)

			switch a.clickCount {
			case 1:
				a.isSelecting = true
				a.dragScrollDelta = 0
				a.lastDragCol = cx
				activeTerm.StartSelection(cx, cy)
			case 2:
				a.isSelecting = false
				a.dragScrollDelta = 0
				activeTerm.SelectWord(cx, cy)
				if txt := activeTerm.GetSelectedText(); txt != "" {
					a.win.SetPrimary(txt)
				}
			default:
				a.isSelecting = false
				a.dragScrollDelta = 0
				activeTerm.SelectLine(cy)
				if txt := activeTerm.GetSelectedText(); txt != "" {
					a.win.SetPrimary(txt)
				}
			}
			a.triggerRedraw()
		} else if e.Detail == 2 {
			if int(e.EventY) < render.HeaderHeight {
				for idx, thb := range a.canvas.TabHitBoxes {
					if int(e.EventX) >= thb.StartX && int(e.EventX) <= thb.EndX {
						a.closeTab(idx)
						return false
					}
				}
				return false
			}
			if activeTerm.MouseTrackingLocked() && (e.State&platform.ModShift) == 0 {
				if a.sendMouseEvent(1, true, int(e.EventX), int(e.EventY), e.State) {
					return false
				}
			}
			a.win.PastePrimary()
		} else if e.Detail == 3 {
			if activeTerm.MouseTrackingLocked() && (e.State&platform.ModShift) == 0 {
				if a.sendMouseEvent(2, true, int(e.EventX), int(e.EventY), e.State) {
					return false
				}
			}
			if activeTerm.HasSelection() {
				text := activeTerm.GetSelectedText()
				if text != "" {
					a.win.SetClipboard(text)
					a.win.SetPrimary(text)
				}
				activeTerm.ClearSelection()
				a.triggerRedraw()
			} else {
				a.win.Paste()
			}
		} else if e.Detail == 4 {
			if a.isPrefOpen {
				items := getSettingsItems(a.config, a.isSearchOpen)
				if a.prefIndex > 0 {
					a.prefIndex--
				} else {
					a.prefIndex = len(items) - 1
				}
				a.triggerRedraw()
				return false
			}
			if (e.State & platform.ModCtrl) != 0 {
				a.applyZoom(platform.ActionZoomIn)
				return false
			}
			if activeTerm.MouseTrackingLocked() && (e.State&platform.ModShift) == 0 {
				if a.sendMouseEvent(64, true, int(e.EventX), int(e.EventY), e.State) {
					return false
				}
			}
			if activeTerm.IsAlt() {
				_, _ = activePTY.Write([]byte("\x1b[A\x1b[A\x1b[A"))
			} else {
				activeTerm.Scroll(3)
				a.triggerRedraw()
			}
		} else if e.Detail == 5 {
			if a.isPrefOpen {
				items := getSettingsItems(a.config, a.isSearchOpen)
				if a.prefIndex < len(items)-1 {
					a.prefIndex++
				} else {
					a.prefIndex = 0
				}
				a.triggerRedraw()
				return false
			}
			if (e.State & platform.ModCtrl) != 0 {
				a.applyZoom(platform.ActionZoomOut)
				return false
			}
			if activeTerm.MouseTrackingLocked() && (e.State&platform.ModShift) == 0 {
				if a.sendMouseEvent(65, true, int(e.EventX), int(e.EventY), e.State) {
					return false
				}
			}
			if activeTerm.IsAlt() {
				_, _ = activePTY.Write([]byte("\x1b[B\x1b[B\x1b[B"))
			} else {
				activeTerm.Scroll(-3)
				a.triggerRedraw()
			}
		}

	case platform.ButtonReleaseEvent:
		if activeTerm.MouseTrackingLocked() && (e.State&platform.ModShift) == 0 {
			btn := -1
			switch e.Detail {
			case 1:
				btn = 0
			case 2:
				btn = 1
			case 3:
				btn = 2
			}
			if btn >= 0 && a.sendMouseEvent(btn, false, int(e.EventX), int(e.EventY), e.State) {
				return false
			}
		}
		if e.Detail == 1 {
			if a.isDraggingScrollbar {
				a.isDraggingScrollbar = false
			}
			if a.isSelecting {
				a.isSelecting = false
				a.dragScrollDelta = 0
				if activeTerm.HasSelection() {
					txt := activeTerm.GetSelectedText()
					if txt != "" {
						a.win.SetPrimary(txt)
					}
				} else {
					activeTerm.ClearSelection()
					a.triggerRedraw()
				}
			}
		}

	case platform.KeyPressEvent:
		keysym := e.KeySym
		data := string(e.Bytes)
		action := e.Action

		if activeTerm.AppCursorLocked() && (e.State&(platform.ModCtrl|platform.ModAlt|platform.ModShift)) == 0 {
			switch keysym {
			case 0xff52:
				data = "\x1bOA"
			case 0xff54:
				data = "\x1bOB"
			case 0xff53:
				data = "\x1bOC"
			case 0xff51:
				data = "\x1bOD"
			case 0xff50:
				data = "\x1bOH"
			case 0xff57:
				data = "\x1bOF"
			}
		}

		if action == platform.ActionPreferences {
			a.isPrefOpen = !a.isPrefOpen
			if a.isPrefOpen {
				a.prefIndex = 0
			}
			a.triggerRedraw()
			return false
		}

		if a.isPrefOpen {
			items := getSettingsItems(a.config, a.isSearchOpen)
			switch keysym {
			case 0xff52, 'k', 'K':
				if a.prefIndex > 0 {
					a.prefIndex--
				} else {
					a.prefIndex = len(items) - 1
				}
				a.triggerRedraw()
			case 0xff54, 'j', 'J':
				if a.prefIndex < len(items)-1 {
					a.prefIndex++
				} else {
					a.prefIndex = 0
				}
				a.triggerRedraw()
			case 0xff51, 'h', 'H', '-':
				if a.prefIndex >= 0 && a.prefIndex < len(items) {
					a.executeSettingsAction(items[a.prefIndex], true)
				}
			case 0xff53, 'l', 'L', '+', '=':
				if a.prefIndex >= 0 && a.prefIndex < len(items) {
					a.executeSettingsAction(items[a.prefIndex], false)
				}
			case 0xff0d, ' ':
				if a.prefIndex >= 0 && a.prefIndex < len(items) {
					a.executeSettingsAction(items[a.prefIndex], false)
				}
			case 0xff1b, 'q', 'Q':
				a.isPrefOpen = false
				a.triggerRedraw()
			case '1', '2', '3', '4', '5', '6', '7', '8':
				idx := int(keysym - '1')
				if idx >= 0 && idx < len(items) {
					a.prefIndex = idx
					a.executeSettingsAction(items[a.prefIndex], false)
				}
			}
			return false
		}

		if action == platform.ActionSearch {
			a.isSearchOpen = !a.isSearchOpen
			if !a.isSearchOpen {
				a.searchMatches = nil
			} else {
				a.updateSearchMatches()
			}
			a.triggerRedraw()
			return false
		}

		if a.isSearchOpen {
			isShift := (e.State & platform.ModShift) != 0
			isCtrl := (e.State & platform.ModCtrl) != 0
			isAlt := (e.State & platform.ModAlt) != 0

			switch keysym {
			case 0xff1b:
				a.isSearchOpen = false
				a.searchMatches = nil
				a.triggerRedraw()
			case 0xff0d:
				if len(a.searchMatches) > 0 {
					if isShift {
						a.activeSearchIdx = (a.activeSearchIdx - 1 + len(a.searchMatches)) % len(a.searchMatches)
					} else {
						a.activeSearchIdx = (a.activeSearchIdx + 1) % len(a.searchMatches)
					}
					a.triggerRedraw()
				}
			case 0xff08:
				if len(a.searchQuery) > 0 {
					rs := []rune(a.searchQuery)
					a.searchQuery = string(rs[:len(rs)-1])
					a.updateSearchMatches()
					a.triggerRedraw()
				}
			case 0xff52:
				if len(a.searchMatches) > 0 {
					a.activeSearchIdx = (a.activeSearchIdx - 1 + len(a.searchMatches)) % len(a.searchMatches)
					a.triggerRedraw()
				}
			case 0xff54:
				if len(a.searchMatches) > 0 {
					a.activeSearchIdx = (a.activeSearchIdx + 1) % len(a.searchMatches)
					a.triggerRedraw()
				}
			default:
				if !isCtrl && !isAlt {
					if len(data) > 0 && utf8.ValidString(data) {
						var added bool
						for _, r := range data {
							if r >= 32 && r != 0x7f && unicode.IsPrint(r) {
								a.searchQuery += string(r)
								added = true
							}
						}
						if added {
							a.updateSearchMatches()
							a.triggerRedraw()
						}
					}
				}
			}
			return false
		}

		if action == platform.ActionZoomIn || action == platform.ActionZoomOut || action == platform.ActionZoomReset {
			a.applyZoom(action)
			return false
		}

		if action == platform.ActionToggleDiagnostics {
			a.config.Diagnostics = !a.config.Diagnostics
			_ = config.Save(a.config)
			if !a.config.Diagnostics {
				a.activeDiag = nil
			} else {
				a.updateDiagnostics()
			}
			a.triggerRedraw()
			return false
		}

		if action == platform.ActionNewTab {
			newTab, err := a.createTab(a.canvas.Cols(), a.canvas.Rows(), a.currentWidth, a.currentHeight)
			if err == nil {
				a.tabs = append(a.tabs, newTab)
				a.switchTab(len(a.tabs) - 1)
			}
			return false
		} else if action == platform.ActionCloseTab {
			a.closeTab(a.activeTabIdx)
			return false
		} else if action == platform.ActionNextTab {
			if len(a.tabs) > 1 {
				a.switchTab((a.activeTabIdx + 1) % len(a.tabs))
			}
			return false
		} else if action == platform.ActionPrevTab {
			if len(a.tabs) > 1 {
				a.switchTab((a.activeTabIdx - 1 + len(a.tabs)) % len(a.tabs))
			}
			return false
		} else if action >= platform.ActionSwitchTab1 && action <= platform.ActionSwitchTab9 {
			tIdx := int(action - platform.ActionSwitchTab1)
			if tIdx < len(a.tabs) {
				a.switchTab(tIdx)
			}
			return false
		}

		if action == platform.ActionCopy {
			if activeTerm.HasSelection() {
				text := activeTerm.GetSelectedText()
				if text != "" {
					a.win.SetClipboard(text)
					a.win.SetPrimary(text)
				}
				activeTerm.ClearSelection()
				a.triggerRedraw()
			}
		} else if action == platform.ActionPaste {
			a.win.Paste()
		} else if action == platform.ActionSelectAll {
			activeTerm.SelectAll()
			if txt := activeTerm.GetSelectedText(); txt != "" {
				a.win.SetPrimary(txt)
			}
			a.triggerRedraw()
			return false
		} else if action == platform.ActionScrollUp {
			if activeTerm.IsAlt() {
				_, _ = activePTY.Write([]byte("\x1b[5~"))
			} else {
				activeTerm.Scroll(a.canvas.Rows() / 2)
				a.triggerRedraw()
			}
		} else if action == platform.ActionScrollDown {
			if activeTerm.IsAlt() {
				_, _ = activePTY.Write([]byte("\x1b[6~"))
			} else {
				activeTerm.Scroll(-a.canvas.Rows() / 2)
				a.triggerRedraw()
			}
		} else if action == platform.ActionScrollTop {
			activeTerm.ScrollToTop()
			a.triggerRedraw()
		} else if action == platform.ActionScrollBottom {
			activeTerm.ResetScroll()
			a.triggerRedraw()
		} else if len(data) > 0 {
			if len(data) == 1 && data[0] == 0x03 && activeTerm.HasSelection() {
				a.win.SetClipboard(activeTerm.GetSelectedText())
				activeTerm.ClearSelection()
				a.triggerRedraw()
			} else if len(data) == 1 && data[0] == 0x16 && !activeTerm.IsAlt() {
				a.win.Paste()
			} else {
				if a.activeGhostText != "" && !activeTerm.IsAlt() {
					isAcceptKey := (keysym == 0xff53) || (keysym == 0xff09 && (e.State&platform.ModShift) == 0)
					if isAcceptKey {
						if len(a.currentInputBuffer) > 0 && a.currentInputBuffer[len(a.currentInputBuffer)-1] == ' ' && !strings.HasSuffix(a.currentInputBuffer, "\\ ") {
							if runtime.GOOS != "windows" {
								_, _ = activePTY.Write([]byte{0x08, '\\', ' '})
								a.currentInputBuffer = a.currentInputBuffer[:len(a.currentInputBuffer)-1] + "\\ "
							}
						}
						toWrite := []byte(a.activeGhostText)
						_, _ = activePTY.Write(toWrite)
						a.currentInputBuffer += a.activeGhostText
						a.activeGhostText = ""
						activeTerm.ClearSelection()
						activeTerm.ResetScroll()
						a.triggerRedraw()
						return false
					}
				}

				if !activeTerm.IsAlt() {
					if !activePTY.IsForegroundShell() || a.isPasswordPrompt(activeTerm) {
						a.currentInputBuffer = ""
						a.activeGhostText = ""
						a.activeDiag = nil
					} else {
						isAlt := (e.State & platform.ModAlt) != 0
						if isAlt && (keysym == 0xff0d || keysym == 0xff8d) && a.activeDiag != nil {
							diag := diagnostics.Analyze(strings.TrimSpace(a.currentInputBuffer))
							if diag != nil && diag.QuickFix != "" {
								_, _ = activePTY.Write([]byte{0x15})
								_, _ = activePTY.Write([]byte(diag.QuickFix))
								a.currentInputBuffer = diag.QuickFix
								a.updateGhostText()
								a.updateDiagnostics()
								activeTerm.ClearSelection()
								activeTerm.ResetScroll()
								a.triggerRedraw()
								return false
							}
						}

						if keysym == 0xff0d || keysym == 0xff8d {
							trimmedCmd := strings.TrimSpace(a.currentInputBuffer)
							if len(trimmedCmd) >= 2 {
								a.suggestEngine.Add(trimmedCmd)
								if strings.HasPrefix(trimmedCmd, "alias ") || strings.HasPrefix(trimmedCmd, "abbr ") {
									diagnostics.RegisterAliasFromLine(trimmedCmd)
								}
							}
							a.currentInputBuffer = ""
							a.activeGhostText = ""
							a.activeDiag = nil
						} else if keysym == 0xff08 {
							if len(a.currentInputBuffer) > 0 {
								_, size := utf8.DecodeLastRuneInString(a.currentInputBuffer)
								a.currentInputBuffer = a.currentInputBuffer[:len(a.currentInputBuffer)-size]
								a.updateGhostText()
								a.updateDiagnostics()
							}
						} else if keysym == 0xff1b || (len(data) == 1 && (data[0] == 0x03 || data[0] == 0x15)) {
							a.currentInputBuffer = ""
							a.activeGhostText = ""
							a.activeDiag = nil
						} else if len(data) > 0 && data[0] >= 32 && data[0] != 127 {
							a.currentInputBuffer += data
							a.updateGhostText()
							a.updateDiagnostics()
						}
					}
				}

				activeTerm.ClearSelection()
				_, _ = activePTY.Write([]byte(data))
				activeTerm.ResetScroll()
				a.triggerRedraw()
			}
		}

	case platform.PasteNotifyEvent:
		if len(e.Text) > 0 {
			a.handlePastedData([]byte(e.Text))
		}

	case platform.ConfigureNotifyEvent:
		lastCfg := e
	drainCfg:
		for {
			select {
			case nextEv, ok := <-xEventCh:
				if !ok {
					return true
				}
				if cfg, isCfg := nextEv.(platform.ConfigureNotifyEvent); isCfg {
					lastCfg = cfg
					continue
				}
				*pendingEvent = nextEv
				break drainCfg
			default:
				break drainCfg
			}
		}
		e = lastCfg

		if e.Width != a.currentWidth || e.Height != a.currentHeight {
			a.currentWidth = e.Width
			a.currentHeight = e.Height

			a.canvas.Resize(int(a.currentWidth), int(a.currentHeight))
			newCols := a.canvas.Cols()
			newRows := a.canvas.Rows()

			if newCols > 0 && newRows > 0 && (newCols != a.currentCols || newRows != a.currentRows) {
				a.currentCols = newCols
				a.currentRows = newRows
				for _, t := range a.tabs {
					t.Term.Resize(newCols, newRows)
					_ = t.PTY.Resize(uint16(newCols), uint16(newRows), a.currentWidth, a.currentHeight)
				}
			}

			if a.isSearchOpen {
				a.updateSearchMatches()
			}

			a.renderScreen(true)
		}

	case platform.ExposeEvent:
		a.renderScreen(true)

	case platform.MappingNotifyEvent:

	case platform.FocusOutEvent:
		a.isSelecting = false
		if a.hoveredURL != nil {
			a.hoveredURL = nil
			a.triggerRedraw()
		}

	case platform.CloseRequestEvent:
		return true
	}

	return false
}
