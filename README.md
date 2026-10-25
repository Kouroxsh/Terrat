<p align="center">
  <img src="assets/icon.png" width="96" height="96" alt="TerraTerminal Logo">
</p>

<h1 align="center">TerraTerminal (<code>terrat</code>)</h1>

<p align="center">
  <b>A terminal emulator for Linux and Windows, written in Go.</b>
  <br>
  <i>Zero CGO &bull; Linux and Windows &bull; Single Binary &bull; v0.2.5</i>
</p>

<p align="center">
  <a href="#overview">Overview</a> &bull;
  <a href="#features">Features</a> &bull;
  <a href="#keyboard-shortcuts">Shortcuts</a> &bull;
  <a href="#cli-usage">CLI Usage</a> &bull;
  <a href="#configuration">Configuration</a> &bull;
  <a href="#installation--building">Installation</a> &bull;
  <a href="#benchmarks">Benchmarks</a> &bull;
  <a href="#license">License</a>
</p>

<p align="center">
  <img src="assets/screenshot.png" alt="TerraTerminal Screenshot" width="850">
</p>

---

## Overview

TerraTerminal (`terrat`) is a desktop terminal emulator for Linux and Windows. It uses X11 or XWayland on Linux and Win32 with ConPTY on Windows, and builds without CGO. It includes tabs, configurable themes, font scaling, search, shell-history suggestions, and command diagnostics.

Each tab has its own terminal state and shell session. Rendering uses an in-memory canvas, while platform-specific code presents the window. See the benchmark results below for measurements on one Linux system; they describe selected operations, not whole-application memory use or startup time.

---

## Features

### 1. Client-Side Window Frame (CSD) & Multi-Theme Controls
- Integrated titlebar with 1px hairline boundary and live dimensions micro-HUD (e.g. `80x24`).
- Clean right-aligned controls for Settings, Minimize, Maximize, and Close.
- Full custom-themed titlebar button rendering (including retro block-art for the Minecraft theme).
- 8-direction interactive border resizing with directional cursor morphing.

### 2. Isolated Multi-Tab Architecture
- Each tab runs its own pseudo-terminal session (PTY on Linux, ConPTY on Windows) and terminal state.
- Tab pills display live process names (`bash`, `nvim`, `htop`, `pwsh`, etc.).
- Intuitive mouse controls: switch tabs, close individual tabs via `×`, or spawn sessions with `+`.
- Clean process exit handling: exiting the active shell terminates the tab; closing the final tab cleanly exits the application.

### 3. Real-Time Font Scaling & Glyphs
- Dynamic font scaling with `Ctrl+=` / `Ctrl+-` or `Ctrl + MouseWheel`.
- In-memory glyph caching and system-font fallback.
- Box-drawing support.
- Persian/Arabic display is temporarily disabled; Arabic-script glyphs render as replacement characters. Right-to-left layout is unsupported.
- Automatic PTY window recalculation with `SIGWINCH` propagation on Linux and ConPTY resize events on Windows.

### 4. Interactive In-Buffer Search
- Minimalist floating search bar toggled with `Ctrl+Shift+F`.
- Live visual match highlighting:
  - **All matches:** Warm amber glow.
  - **Active match:** Vivid emerald highlight.
- Real-time match counter with `Enter` (next) and `Shift+Enter` (previous) navigation.

### 5. Curated Themes & System Dark Mode
- Preferences modal toggled via `Ctrl+,` or `Ctrl+Shift+P`.
- Built-in color schemes:
  - **Tokyo Night** (Dark)
  - **Catppuccin Mocha** (Dark)
  - **Minecraft** (Dark custom palette)
  - **Tokyo Day** (Light)
  - **Solarized Light** (shown as “Solarized” in Preferences; Light)
  - **Auto** (Synchronizes with OS desktop theme via DBus / Desktop Portal)

### 6. Live Command Diagnostics & Typo Linter
- Integrated command parser supporting quotes, escapes, and chained operators (`&&`, `||`, `;`, `|`).
- Detects unrecognized commands (`gti` &rarr; `git`, `sl` &rarr; `ls`, `mkae` &rarr; `make`), unknown subcommands (`git puch` &rarr; `git push`), unclosed quote strings, and commands requiring root privileges.
- Intelligent path skipping for Windows executables (`C:\...`, `.exe`).
- Press **`Alt + Enter`** to automatically apply the recommended QuickFix directly to the active prompt.
- Press **`Ctrl + Shift + D`** to toggle diagnostics on and off.

### 7. Fish-Style Ghost Autocomplete & Path Suggestions
- Subtle inline suggestions rendered directly ahead of the cursor.
- Automatically reads shell histories across platforms: `~/.bash_history`, `~/.zsh_history` on Linux, and PowerShell PSReadLine (`ConsoleHost_history.txt`) on Windows.
- Intelligent path autocompletion supporting both `/` and `\` separators, spaces with escapes, quoted tokens (`"..."` / `'...'`), and home directory expansion (`~`, `~/`, `~\`).
- Press **`Tab`** or **`Right Arrow`** to accept the suggestion.

### 8. Hardened Clipboard & Anti-Pastejacking
- Block text selection with word (double-click) and row (triple-click) matching.
- **Smart Ctrl+C / Ctrl+V:** `Ctrl+C` copies when text is selected or sends `SIGINT` when unselected. `Ctrl+V` pastes in normal shell mode while remaining transparent in fullscreen editors (`vim`, `nano`).
- **Bracketed Paste (DECSET 2004):** Wraps pasted text in escape markers to prevent broken indentation in REPLs.
- **Anti-Pastejacking Sanitization:** Strips malicious control codes, dangerous escapes, and bidirectional overrides while fully preserving legitimate Unicode format characters (`\u200C`).

---

## Keyboard Shortcuts

### Navigation & Tabs
| Shortcut | Action |
| :--- | :--- |
| `Ctrl + Shift + T` | Open new tab |
| `Ctrl + Shift + W` | Close active tab |
| `Ctrl + Tab` / `Ctrl + PageDown` | Switch to next tab |
| `Ctrl + Shift + Tab` / `Ctrl + PageUp` | Switch to previous tab |
| `Alt + 1` .. `Alt + 9` | Jump directly to tabs 1 through 9 |

### Clipboard & Selection
| Shortcut | Action |
| :--- | :--- |
| `Ctrl + Shift + C` *(or `Ctrl + C` with selection)* | Copy selected text to clipboard |
| `Ctrl + Shift + V` / `Shift + Insert` | Paste text from clipboard |
| `Ctrl + Shift + A` | Select entire terminal buffer |
| `Middle Click` | Paste from X11 `PRIMARY` selection |

### Diagnostics & Search
| Shortcut | Action |
| :--- | :--- |
| `Alt + Enter` | Apply recommended Diagnostic QuickFix |
| `Ctrl + Shift + D` | Toggle Live Command Diagnostics |
| `Tab` / `Right Arrow` | Accept inline Ghost Text completion |
| `Ctrl + Shift + F` | Toggle in-buffer search bar |
| `Enter` / `Shift + Enter` | Next / previous search match |
| `Esc` | Close search bar or preferences modal |
| `Ctrl + Left Click` | Open hovered URL in default browser |

### View & Font Zoom
| Shortcut | Action |
| :--- | :--- |
| `Ctrl + =` / `Ctrl + +` | Increase font size by 1pt |
| `Ctrl + -` / `Ctrl + _` | Decrease font size by 1pt |
| `Ctrl + 0` | Reset font size to default (13pt) |
| `Ctrl + Wheel Up / Down` | Zoom in / out with mouse wheel |
| `Shift + PageUp / PageDown` | Scroll terminal viewport up / down |
| `Ctrl + Shift + Up / Down` *(Windows)* | Scroll terminal viewport up / down |
| `Ctrl + Shift + PageUp / PageDown` *(Windows)* | Scroll to top / bottom of terminal viewport |
| `Ctrl + ,` / `Ctrl + Shift + P` | Open / close Preferences modal |

---

## CLI Usage

```bash
terrat [flags] [-e command [args...]]
```

### Options
| Flag | Description | Default |
| :--- | :--- | :--- |
| `-v`, `-version` | Print version information and exit | `false` |
| `-title <string>` | Set initial window title | `TerraTerminal` |
| `-font-size <float>` | Override font size in points | *(config)* |
| `-theme <string>` | Color theme (`tokyo-night`, `catppuccin-mocha`, `minecraft`, `tokyo-day`, `solarized-light`) | *(config)* |
| `-shell <path>` | Override default shell binary (e.g. `bash`, `wsl.exe`, `powershell.exe`) | *(system)* |
| `-e <cmd...>` | Execute a custom command directly instead of the default shell | &mdash; |

### Examples
```bash
# Launch with custom font size and theme
terrat -font-size 15 -theme catppuccin-mocha

# Run a dedicated tool directly
terrat -e htop
terrat -e nvim /path/to/file
```

---

## Configuration

Settings are saved automatically under the user configuration directory in `terrat/config.json`. On Linux, `XDG_CONFIG_HOME` is used when set (otherwise typically `~/.config`); on Windows, the OS user configuration directory is used. If a `config.json` file is present beside the executable, that file takes precedence for portable installs:

```json
{
  "shell": "",
  "theme": "auto",
  "font_size": 13.0,
  "opacity": 0.95,
  "ghost_text": true,
  "diagnostics": true,
  "sanitize_paste": true,
  "bracketed_paste": true
}
```

---

## Benchmarks

These Go microbenchmarks were run five times on Linux/amd64 with Go 1.22.6 on an Intel Core i5-8500 CPU @ 3.00 GHz. Values below are the median of the five runs. Times are per benchmark operation.

| Operation | Result | Allocations |
| :--- | ---: | ---: |
| Write 128 plain terminal lines | 95.3 µs | 0 B/op, 0 allocs/op |
| Write 128 ANSI-colored terminal lines | 119.4 µs | 2,048 B/op, 512 allocs/op |
| Write 128 alternate-screen lines | 89.0 µs | 0 B/op, 0 allocs/op |
| Render a 1024 × 640 frame | 973 µs | 32 B/op, 3 allocs/op |
| Render a 1024 × 640 frame with search matches | 2.06 ms | 32 B/op, 3 allocs/op |
| Fill a 1024 × 640 canvas | 70.8 µs | 0 B/op, 0 allocs/op |
| Create and close the font engine | 1.96 ms | 677,434 B/op, 1,199 allocs/op |
| Add an entry to shell history | 42.4 ns | 0 B/op, 0 allocs/op |

The terminal-write benchmark sends a 128-line payload after warming the terminal. Rendering uses the CPU canvas and does not include X11/Win32 presentation, window composition, or shell startup. These figures are not whole-app latency or resident-memory measurements and should not be read as a cross-platform comparison. To repeat them, run:

```bash
go test ./internal/terminal ./internal/render ./internal/autosuggest -run '^$' -bench . -benchmem -benchtime=500ms -count=5
```

---

## Installation & Building

### Prerequisites
- **Linux:** X11 server or Wayland via XWayland.
- **Windows:** Windows 10 or 11 (build 1809+ for ConPTY).
- **Go 1.22+** (only required if compiling from source).

### Building from Source

```bash
# Clone the repository
git clone https://github.com/Kouroxsh/Terrat.git
cd Terrat

# Linux: Compile optimized static binary
make build

# Linux: Optional install to ~/go/bin with desktop entry and icon
make install

# Windows: Compile native static executable (or cross-compile from Linux)
GOOS=windows go build -ldflags="-s -w" -o terrat.exe ./cmd/terrat

# Build release packages (.deb, .rpm, .pkg.tar.zst, .tar.gz, .zip) via nfpm
make package
```

The package target also builds Windows portable ZIP files. Building packages requires `nfpm`; the packaging script attempts to install it with `go install` if it is not already available.

### Running Tests & Static Analysis

```bash
# Run unit and race detection test suite
go test -race -count=1 ./...

# Run static analysis
go vet ./...

# Cross-compile validation for Windows
GOOS=windows go build ./...
```

---

## License

TerraTerminal is open source under the [GNU General Public License v3.0 (GPLv3)](LICENSE).
