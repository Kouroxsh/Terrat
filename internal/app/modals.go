package app

import (
	"fmt"
	"net/url"
	"regexp"

	"github.com/Kouroxsh/Terrat/internal/config"
	"github.com/Kouroxsh/Terrat/internal/platform"
	"github.com/Kouroxsh/Terrat/internal/render"
	"github.com/Kouroxsh/Terrat/internal/terminal"
)

var urlRegex = regexp.MustCompile(`https?://[^\s<>"'()]+`)

func openURL(urlStr string) {
	parsed, err := url.ParseRequestURI(urlStr)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return
	}
	_ = platform.OpenURL(parsed.String())
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
			if len(matches) >= 1000 {
				return matches
			}
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
			Label:    "Command diagnostics",
			IsToggle: true,
			Enabled:  appConfig.Diagnostics,
		},
		{
			ID:       "ghost_text",
			Label:    "Inline suggestions",
			IsToggle: true,
			Enabled:  appConfig.GhostText,
		},
		{
			ID:       "sanitize_paste",
			Label:    "Clean pasted text",
			IsToggle: true,
			Enabled:  appConfig.SanitizePaste,
		},
		{
			ID:       "search",
			Label:    "Find in scrollback",
			IsToggle: true,
			Enabled:  isSearchOpen,
		},
		{
			ID:    "theme",
			Label: "Color theme",
			Value: themeDisplay,
		},
		{
			ID:    "font_size",
			Label: "Font size",
			Value: fmt.Sprintf("%.0fpt", appConfig.FontSize),
		},
		{
			ID:    "opacity",
			Label: "Window opacity",
			Value: fmt.Sprintf("%d%%", int(appConfig.Opacity*100)),
		},
	}
}
