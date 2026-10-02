package app

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"

	"terrat/internal/config"
	"terrat/internal/render"
	"terrat/internal/terminal"
)

var urlRegex = regexp.MustCompile(`https?://[^\s<>"'()]+`)

func openURL(urlStr string) {
	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", urlStr).Start()
	case "darwin":
		_ = exec.Command("open", urlStr).Start()
	default:
		_ = exec.Command("xdg-open", urlStr).Start()
	}
}

func drainChannel(ch chan struct{}) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// findSearchMatches scans the visible grid of the terminal for all
// case-insensitive occurrences of query. Column indices are rune-based so
// they align with grid cells even for multi-byte characters.
func findSearchMatches(term *terminal.Terminal, query string) []render.SearchMatch {
	if query == "" || term == nil {
		return nil
	}
	var matches []render.SearchMatch
	for y := 0; y < term.Rows(); y++ {
		for _, m := range terminal.FindRowMatches(term.GetRowString(y), query) {
			matches = append(matches, render.SearchMatch{
				Row:      y,
				StartCol: m.StartCol,
				EndCol:   m.EndCol,
			})
		}
	}
	return matches
}

func getSettingsItems(appConfig *config.Config, isSearchOpen bool) []render.PrefOption {
	themeDisplay := appConfig.Theme
	switch appConfig.Theme {
	case "auto":
		themeDisplay = "Auto"
	case "tokyo-night":
		themeDisplay = "Tokyo Night"
	case "catppuccin-mocha":
		themeDisplay = "Catppuccin"
	case "minecraft":
		themeDisplay = "Minecraft"
	case "tokyo-day":
		themeDisplay = "Tokyo Day"
	case "solarized-light":
		themeDisplay = "Solarized"
	}

	return []render.PrefOption{
		{
			ID:       "diagnostics",
			Label:    "Live Diagnostics",
			IsToggle: true,
			Enabled:  appConfig.Diagnostics,
		},
		{
			ID:       "ghost_text",
			Label:    "Ghost Autocomplete",
			IsToggle: true,
			Enabled:  appConfig.GhostText,
		},
		{
			ID:       "sanitize_paste",
			Label:    "Sanitize Pasted Text",
			IsToggle: true,
			Enabled:  appConfig.SanitizePaste,
		},
		{
			ID:       "search",
			Label:    "Find in Buffer",
			IsToggle: true,
			Enabled:  isSearchOpen,
		},
		{
			ID:    "theme",
			Label: "Color Theme",
			Value: themeDisplay,
		},
		{
			ID:    "font_size",
			Label: "Font Size",
			Value: fmt.Sprintf("%.0fpt", appConfig.FontSize),
		},
		{
			ID:    "opacity",
			Label: "Window Opacity",
			Value: fmt.Sprintf("%d%%", int(appConfig.Opacity*100)),
		},
	}
}
