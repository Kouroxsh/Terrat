package main

import (
	"flag"
	"fmt"
	"os"

	"terrat/internal/app"
	"terrat/internal/config"
	"terrat/internal/pty"
)

const (
	AppName = "TerraTerminal"
)

var (
	Version   = "0.2.5"
	AppBanner = "TerraTerminal (Terrat) v" + Version + " - Blazing fast minimalist terminal for Linux & Windows"
)

func main() {
	var customCmd []string
	var remainingArgs []string
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "-e" {
			if i+1 < len(os.Args) {
				customCmd = os.Args[i+1:]
			}
			break
		}
		remainingArgs = append(remainingArgs, os.Args[i])
	}

	fs := flag.NewFlagSet("terrat", flag.ContinueOnError)
	verFlag := fs.Bool("v", false, "Print version and exit")
	versionFlag := fs.Bool("version", false, "Print version and exit")
	titleFlag := fs.String("title", app.AppName, "Set initial window title")
	fontSizeFlag := fs.Float64("font-size", 0.0, "Font size in points (defaults to config)")
	themeFlag := fs.String("theme", "", "Color theme override (tokyo-night, catppuccin-mocha, minecraft, tokyo-day, solarized-light)")
	shellFlag := fs.String("shell", "", "Default shell override (e.g., bash, wsl.exe, powershell.exe)")
	_ = fs.Parse(remainingArgs)

	if *verFlag || *versionFlag {
		fmt.Println(AppBanner)
		return
	}

	appConfig := config.Load()
	if *shellFlag != "" {
		appConfig.Shell = *shellFlag
	}
	pty.SetDefaultShell(appConfig.Shell)
	if *fontSizeFlag > 0 {
		appConfig.FontSize = *fontSizeFlag
	}
	if *themeFlag != "" {
		appConfig.Theme = *themeFlag
	}

	a, err := app.NewApp(appConfig, *titleFlag, customCmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing TerraTerminal: %v\n", err)
		os.Exit(1)
	}
	defer a.Close()

	if err := a.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TerraTerminal: %v\n", err)
		os.Exit(1)
	}
}
