package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Kouroxsh/Terrat/internal/app"
	"github.com/Kouroxsh/Terrat/internal/config"
	"github.com/Kouroxsh/Terrat/internal/pty"
	"github.com/Kouroxsh/Terrat/internal/terminal"
)

const (
	AppName = "TerraTerminal"
)

var (
	Version   = "3.0.0"
	AppBanner = "TerraTerminal (Terrat) v" + Version + " - Blazing fast minimalist terminal for Linux"
)

func main() {
	var customCmd []string
	var remainingArgs []string

	valueFlags := map[string]bool{
		"-title": true, "--title": true,
		"-font-size": true, "--font-size": true,
		"-theme": true, "--theme": true,
		"-shell": true, "--shell": true,
	}
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "--" {
			remainingArgs = append(remainingArgs, os.Args[i:]...)
			break
		}
		if valueFlags[arg] {
			remainingArgs = append(remainingArgs, arg)
			if i+1 < len(os.Args) {
				i++
				remainingArgs = append(remainingArgs, os.Args[i])
			}
			continue
		}
		if arg == "-e" {
			if i+1 >= len(os.Args) {
				fmt.Fprintln(os.Stderr, "terrat: -e requires a command")
				os.Exit(2)
			}
			customCmd = os.Args[i+1:]
			break
		}
		remainingArgs = append(remainingArgs, arg)
	}

	fs := flag.NewFlagSet("terrat", flag.ContinueOnError)
	verFlag := fs.Bool("v", false, "Print version and exit")
	versionFlag := fs.Bool("version", false, "Print version and exit")
	titleFlag := fs.String("title", app.AppName, "Set initial window title")
	fontSizeFlag := fs.Float64("font-size", 0.0, "Font size in points (defaults to config)")
	themeFlag := fs.String("theme", "", "Color theme override (tokyo-night, catppuccin-mocha, minecraft, tokyo-day, solarized-light)")
	shellFlag := fs.String("shell", "", "Default shell override (e.g., bash, zsh)")

	if err := fs.Parse(remainingArgs); err != nil {
		if err == flag.ErrHelp {
			return
		}
		os.Exit(2)
	}

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
		size := *fontSizeFlag
		if size < 7.0 {
			size = 7.0
		}
		if size > 36.0 {
			size = 36.0
		}
		appConfig.FontSize = size
	}
	if *themeFlag != "" {
		if !terminal.IsValidThemePreference(*themeFlag) {
			fmt.Fprintf(os.Stderr, "terrat: unknown theme %q, falling back to \"auto\"\n", *themeFlag)
			appConfig.Theme = "auto"
		} else {
			appConfig.Theme = *themeFlag
		}
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

	os.Exit(a.ExitCode())
}
