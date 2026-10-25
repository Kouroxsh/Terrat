package autosuggest

import (
	"bufio"
	"container/list"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var defaultCommonCommands = []string{
	"git status",
	"git commit -m \"\"",
	"git push origin main",
	"git pull --rebase",
	"git log --oneline -n 10",
	"git diff",
	"git checkout",
	"git branch -a",
	"docker ps -a",
	"docker compose up -d",
	"docker compose down",
	"docker logs -f",
	"make build",
	"make install",
	"make clean",
	"make run",
	"make test",
	"systemctl status",
	"systemctl restart",
	"journalctl -xeu",
	"sudo apt update && sudo apt upgrade",
	"sudo apt install -y",
	"sudo pacman -Syu",
	"go test ./...",
	"go run main.go",
	"go build -o",
	"cargo build --release",
	"cargo run",
	"npm run dev",
	"npm run build",
	"pnpm dev",
	"yarn start",
	"cat /etc/os-release",
	"uname -r",
	"ls -lah",
	"mkdir -p",
	"rm -rf",
	"grep -rn",
	"find . -name",
	"tar -czvf",
	"tar -xzvf",
	"htop",
	"btop",
	"neofetch",
	"fastfetch",
}

type Engine struct {
	mu       sync.RWMutex
	history  []string
	historyM map[string]struct{}
	maxItems int
}

func NewEngine() *Engine {
	e := &Engine{
		historyM: make(map[string]struct{}),
		maxItems: 2000,
	}

	for _, cmd := range defaultCommonCommands {
		e.addCommandInternal(cmd)
	}

	e.loadShellHistory()
	return e
}

func (e *Engine) loadShellHistory() {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return
	}

	historyFiles := []string{
		filepath.Join(home, ".bash_history"),
		filepath.Join(home, ".zsh_history"),
	}
	if appData := os.Getenv("APPDATA"); appData != "" {
		historyFiles = append(historyFiles, filepath.Join(appData, "Microsoft", "Windows", "PowerShell", "PSReadLine", "ConsoleHost_history.txt"))
	} else {
		historyFiles = append(historyFiles, filepath.Join(home, "AppData", "Roaming", "Microsoft", "Windows", "PowerShell", "PSReadLine", "ConsoleHost_history.txt"))
	}

	for _, hf := range historyFiles {
		if len(e.history) >= e.maxItems {
			break
		}
		file, err := os.Open(hf)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		lines := list.New()
		entries := make(map[string]*list.Element)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			// Handle zsh extended history format: ": 1600000000:0;cmd"
			if strings.HasPrefix(line, ": ") {
				if semi := strings.Index(line, ";"); semi != -1 && semi+1 < len(line) {
					line = strings.TrimSpace(line[semi+1:])
				}
			}
			if len(line) >= 2 {
				if entry, ok := entries[line]; ok {
					lines.MoveToBack(entry)
					continue
				}
				if lines.Len() == e.maxItems {
					oldest := lines.Front()
					delete(entries, oldest.Value.(string))
					lines.Remove(oldest)
				}
				entries[line] = lines.PushBack(line)
			}
		}
		_ = scanner.Err()
		_ = file.Close()

		// Read in reverse (most recent commands first)
		for entry := lines.Back(); entry != nil; entry = entry.Prev() {
			e.addCommandInternal(entry.Value.(string))
			if len(e.history) >= e.maxItems {
				break
			}
		}
	}
}

func (e *Engine) addCommandInternal(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if len(cmd) < 2 || len(e.history) >= e.maxItems {
		return
	}
	if _, exists := e.historyM[cmd]; exists {
		return
	}
	e.historyM[cmd] = struct{}{}
	e.history = append(e.history, cmd)
}

// Add learns a new command executed by the user.
func (e *Engine) Add(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" || len(cmd) < 2 {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.historyM[cmd]; exists {
		for i, h := range e.history {
			if h == cmd {
				copy(e.history[1:i+1], e.history[:i])
				e.history[0] = cmd
				return
			}
		}
	}
	if len(e.history) >= e.maxItems {
		last := e.history[len(e.history)-1]
		delete(e.historyM, last)
		e.history = e.history[:len(e.history)-1]
	}
	e.historyM[cmd] = struct{}{}
	e.history = append(e.history, "")
	copy(e.history[1:], e.history[:len(e.history)-1])
	e.history[0] = cmd
}

// Suggest returns the ghost suffix (continuation) for the given input prefix.
// If cwd is provided, it prioritizes local file/directory path completion (including paths with spaces).
// Otherwise or as fallback, it searches command history.
func (e *Engine) Suggest(input string, cwd ...string) string {
	if len(input) < 1 {
		return ""
	}

	// 1. Try local filesystem path completion if cwd is provided
	if len(cwd) > 0 && cwd[0] != "" {
		if pathSuffix := suggestPath(input, cwd[0]); pathSuffix != "" {
			return pathSuffix
		}
	}

	// 2. Fall back to command history
	e.mu.RLock()
	defer e.mu.RUnlock()

	lowerInput := strings.ToLower(input)

	for _, cmd := range e.history {
		lowerCmd := strings.ToLower(cmd)
		if strings.HasPrefix(lowerCmd, lowerInput) && len(cmd) > len(input) {
			// Return suffix preserving original case of the match
			return cmd[len(input):]
		}
	}

	return ""
}

// suggestPath checks the active directory for matching files or folders (supporting spaces and quotes)
func suggestPath(input string, cwd string) string {
	if cwd == "" || len(input) == 0 {
		return ""
	}

	// Check if input looks like a command with arguments or a path
	inDouble := false
	inSingle := false
	escaped := false
	lastTokenStart := 0

	for i := 0; i < len(input); i++ {
		b := input[i]
		if escaped {
			escaped = false
			continue
		}
		if b == '\\' && !inSingle {
			if i+1 < len(input) && (input[i+1] == ' ' || input[i+1] == '\t' || input[i+1] == '"' || input[i+1] == '\'') {
				escaped = true
				continue
			}
		}
		if b == '"' && !inSingle {
			inDouble = !inDouble
			if inDouble {
				lastTokenStart = i
			}
			continue
		}
		if b == '\'' && !inDouble {
			inSingle = !inSingle
			if inSingle {
				lastTokenStart = i
			}
			continue
		}
		if (b == ' ' || b == '\t') && !inDouble && !inSingle {
			lastTokenStart = i + 1
		}
	}

	rawToken := input[lastTokenStart:]

	// Determine quote mode
	isQuoted := false
	quoteChar := byte(0)
	cleanToken := rawToken

	if len(rawToken) > 0 {
		switch rawToken[0] {
		case '"', '\'':
			isQuoted = true
			quoteChar = rawToken[0]
			cleanToken = rawToken[1:]
		}
	}

	// Unescape if unquoted (e.g. "my\ " -> "my ")
	var unescapedToken strings.Builder
	for i := 0; i < len(cleanToken); i++ {
		if cleanToken[i] == '\\' && !isQuoted && i+1 < len(cleanToken) && (cleanToken[i+1] == ' ' || cleanToken[i+1] == '\t' || cleanToken[i+1] == '"' || cleanToken[i+1] == '\'') {
			i++
			unescapedToken.WriteByte(cleanToken[i])
		} else {
			unescapedToken.WriteByte(cleanToken[i])
		}
	}
	pathToSearch := unescapedToken.String()

	// Split directory and base (handling both / and \)
	normPath := strings.ReplaceAll(pathToSearch, "\\", "/")
	dirPart := filepath.Dir(normPath)
	basePart := filepath.Base(normPath)

	searchDir := cwd
	if filepath.IsAbs(pathToSearch) || strings.HasPrefix(normPath, "/") {
		if dirPart == "/" {
			searchDir = "/"
		} else {
			searchDir = filepath.FromSlash(dirPart)
		}
	} else if pathToSearch == "~" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			searchDir = home
			basePart = ""
		}
	} else if (strings.HasPrefix(pathToSearch, "~/") || strings.HasPrefix(pathToSearch, "~\\")) && len(pathToSearch) > 2 {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			rem := strings.ReplaceAll(pathToSearch[2:], "\\", "/")
			d := filepath.Dir(rem)
			b := filepath.Base(rem)
			searchDir = filepath.Join(home, filepath.FromSlash(d))
			basePart = b
		}
	} else if strings.Contains(pathToSearch, "/") || strings.Contains(pathToSearch, "\\") {
		searchDir = filepath.Join(cwd, filepath.FromSlash(dirPart))
	} else {
		searchDir = cwd
		basePart = pathToSearch
	}

	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return ""
	}

	lowerBase := strings.ToLower(basePart)

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(basePart, ".") {
			continue // Skip hidden files unless explicitly requested
		}

		if len(basePart) > 0 && strings.HasPrefix(strings.ToLower(name), lowerBase) && len(name) >= len(basePart) {
			suffix := name[len(basePart):]
			dirSep := "/"
			if strings.Contains(pathToSearch, "\\") {
				dirSep = "\\"
			}
			if entry.IsDir() {
				suffix += dirSep
			}

			if isQuoted {
				if entry.IsDir() {
					return suffix
				}
				return suffix + string(quoteChar)
			}

			// In unquoted mode on Unix, spaces in suffix must be escaped
			if runtime.GOOS != "windows" {
				suffix = strings.ReplaceAll(suffix, " ", "\\ ")
			}
			return suffix
		}
	}

	// Also check if the user typed an unescaped space at the end, e.g. "cd my "
	// where the folder name contains a space ("my space folder")
	if !isQuoted && len(rawToken) == 0 && lastTokenStart > 1 && input[lastTokenStart-1] == ' ' {
		prevTokenStart := strings.LastIndexAny(strings.TrimRight(input[:lastTokenStart-1], " \t"), " \t")
		var prevToken string
		if prevTokenStart == -1 {
			prevToken = strings.TrimSpace(input[:lastTokenStart-1])
		} else {
			prevToken = strings.TrimSpace(input[prevTokenStart+1 : lastTokenStart-1])
		}
		if len(prevToken) > 0 {
			targetPrefix := strings.ToLower(prevToken + " ")
			for _, entry := range entries {
				name := entry.Name()
				if strings.HasPrefix(strings.ToLower(name), targetPrefix) {
					suffix := name[len(targetPrefix):]
					dirSep := "/"
					if strings.Contains(input, "\\") {
						dirSep = "\\"
					}
					if entry.IsDir() {
						suffix += dirSep
					}
					if runtime.GOOS != "windows" {
						suffix = strings.ReplaceAll(suffix, " ", "\\ ")
					}
					return suffix
				}
			}
		}
	}

	return ""
}
