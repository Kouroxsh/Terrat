package autosuggest

import (
	"bufio"
	"container/list"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
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

	// M24: load the real shell history first (most recent first), then the
	// built-in defaults as fallback. The old order put the defaults first,
	// so `git st` ghosted `atus` from the default `git status` instead of
	// the user's own `git stash` from history.
	e.loadShellHistory()
	for _, cmd := range defaultCommonCommands {
		e.addCommandInternal(cmd)
	}

	return e
}

// isTimestampLine reports whether line is a bare `#<digits>` history
// timestamp (bash HISTTIMEFORMAT), e.g. `#1600000000`.
func isTimestampLine(line string) bool {
	if len(line) < 2 || line[0] != '#' {
		return false
	}
	for _, r := range line[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// maxHistoryFileBytes caps how much of a history file is read (L30):
// history files are append-only, so the most recent entries live at the
// end — reading a 50MB .zsh_history in full on every startup just to
// keep the newest 2000 lines was pure waste.
const maxHistoryFileBytes = 2 << 20 // 2 MiB tail

// maxHistoryLineBytes caps a single history line (L30): the default
// bufio.Scanner cap is 64KB, so one very long one-liner truncated the
// scan and silently dropped every entry below it.
const maxHistoryLineBytes = 1 << 20 // 1 MiB

// newHistoryScanner returns a scanner over file, positioned at the last
// maxHistoryFileBytes when the file is larger, so only the recent tail
// is read. The first line after the seek is partial and skipped.
func newHistoryScanner(file *os.File) *bufio.Scanner {
	if info, err := file.Stat(); err == nil && info.Size() > maxHistoryFileBytes {
		if _, err := file.Seek(info.Size()-maxHistoryFileBytes, io.SeekStart); err == nil {
			br := bufio.NewReader(file)
			_, _ = br.ReadString('\n') // skip partial first line
			scanner := bufio.NewScanner(br)
			scanner.Buffer(make([]byte, 64*1024), maxHistoryLineBytes)
			return scanner
		}
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), maxHistoryLineBytes)
	return scanner
}

// parseZshExtended extracts the command from a zsh extended-history line
// (": <begin>:<elapsed>;<command>"). ok=false when the line is not in
// extended format — a plain command starting with ":" (the no-op builtin)
// must not be mistaken for one. An empty command after the ";" reports
// ok=true with cmd="" so the caller skips the entry instead of
// suggesting the raw ": ..." line (L31).
func parseZshExtended(line string) (cmd string, ok bool) {
	if !strings.HasPrefix(line, ": ") {
		return "", false
	}
	rest := line[2:]
	i := 0
	for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
		i++
	}
	if i == 0 || i >= len(rest) || rest[i] != ':' {
		return "", false
	}
	j := i + 1
	for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
		j++
	}
	if j >= len(rest) || rest[j] != ';' {
		return "", false
	}
	return strings.TrimSpace(rest[j+1:]), true
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

	for _, hf := range historyFiles {
		if len(e.history) >= e.maxItems {
			break
		}
		file, err := os.Open(hf)
		if err != nil {
			continue
		}

		scanner := newHistoryScanner(file)
		lines := list.New()
		entries := make(map[string]*list.Element)
		isZsh := strings.HasSuffix(hf, ".zsh_history")
		// An entry under assembly: in zsh extended history a new entry
		// always starts with ": <ts>:<dur>;", so any other line is a
		// continuation of the current multiline entry (L31) and must be
		// joined to it instead of suggested as a fragment.
		var curEntry string
		hasCur := false
		flush := func() {
			if !hasCur {
				return
			}
			hasCur = false
			line := curEntry
			if len(line) >= 2 {
				if entry, ok := entries[line]; ok {
					lines.MoveToBack(entry)
					return
				}
				if lines.Len() == e.maxItems {
					oldest := lines.Front()
					delete(entries, oldest.Value.(string))
					lines.Remove(oldest)
				}
				entries[line] = lines.PushBack(line)
			}
		}
		for scanner.Scan() {
			raw := scanner.Text()
			line := strings.TrimSpace(raw)
			if line == "" {
				// A blank line inside a multiline zsh entry belongs to it.
				if isZsh && hasCur {
					curEntry += "\n"
				}
				continue
			}
			// M27: bash HISTTIMEFORMAT timestamps look like `#1600000000`
			// and must never be suggested as commands.
			if isTimestampLine(line) {
				flush()
				continue
			}
			// Handle zsh extended history format: ": 1600000000:0;cmd"
			if cmd, ok := parseZshExtended(line); ok {
				flush()
				if cmd != "" {
					curEntry = cmd
					hasCur = true
				}
				continue
			}
			if isZsh && hasCur {
				// Continuation line: keep the raw line (indentation
				// included) so the entry stays whole.
				curEntry += "\n" + raw
				continue
			}
			flush()
			curEntry = line
			hasCur = true
		}
		flush()
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

// foldPrefixLen returns the byte index in s just past the runes that
// case-fold-match prefix, or -1 when prefix is not a case-insensitive
// prefix of s. It folds rune by rune (unicode.ToLower): slicing by
// len(prefix) bytes is wrong when folding changes byte lengths —
// strings.ToLower("İ") is two runes ("i̇"), so cmd[len(input):] could cut
// a rune in half and produce invalid UTF-8 in the ghost suffix (L32).
func foldPrefixLen(s, prefix string) int {
	si, pi := 0, 0
	for pi < len(prefix) {
		if si >= len(s) {
			return -1
		}
		sr, sw := utf8.DecodeRuneInString(s[si:])
		pr, pw := utf8.DecodeRuneInString(prefix[pi:])
		if unicode.ToLower(sr) != unicode.ToLower(pr) {
			return -1
		}
		si += sw
		pi += pw
	}
	return si
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

	for _, cmd := range e.history {
		if idx := foldPrefixLen(cmd, input); idx >= 0 && idx < len(cmd) {
			// Return suffix preserving original case of the match.
			return cmd[idx:]
		}
	}

	return ""
}

// dirCacheTTL bounds how stale a directory listing can be (L33).
const dirCacheTTL = time.Second

type dirCacheEntry struct {
	entries []os.DirEntry
	expires time.Time
}

var (
	dirCacheMu sync.Mutex
	dirCache   = map[string]dirCacheEntry{}
)

// cachedReadDir returns a directory listing, reusing one read less than a
// second ago. suggestPath runs on every keystroke; without this, a large
// directory was re-read from disk per keystroke for the same prefix.
func cachedReadDir(dir string) ([]os.DirEntry, error) {
	dirCacheMu.Lock()
	if ent, ok := dirCache[dir]; ok && time.Now().Before(ent.expires) {
		dirCacheMu.Unlock()
		return ent.entries, nil
	}
	dirCacheMu.Unlock()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	dirCacheMu.Lock()
	if len(dirCache) > 128 {
		// Bound the cache: drop everything rather than tracking LRU.
		dirCache = map[string]dirCacheEntry{}
	}
	dirCache[dir] = dirCacheEntry{entries: entries, expires: time.Now().Add(dirCacheTTL)}
	dirCacheMu.Unlock()
	return entries, nil
}

// expandTilde replaces a leading ~, ~/ or ~user[/...] with the absolute
// home directory (L29). A bare ~ expands to the home directory with a
// trailing slash so it lists its contents, like ~/. An unknown user (or
// no home) leaves the token unchanged, so completion falls through to
// the normal path logic and finds nothing instead of completing from
// the wrong directory.
func expandTilde(token string) string {
	if token == "" || token[0] != '~' {
		return token
	}
	if token == "~" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return home + "/"
		}
		return token
	}
	if strings.HasPrefix(token, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return home + token[1:]
		}
		return token
	}
	rest := token[1:]
	userPart := rest
	tail := ""
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		userPart, tail = rest[:i], rest[i:]
	}
	if u, err := user.Lookup(userPart); err == nil && u.HomeDir != "" {
		return u.HomeDir + tail
	}
	return token
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

	// M26: don't path-complete the command word itself (the first token)
	// unless it looks like a path — otherwise typing `do` in a directory
	// containing docker-compose.yml suggests a file instead of the `docker`
	// command from history.
	if lastTokenStart == 0 && !isQuoted && !strings.ContainsAny(cleanToken, "/~.") {
		return ""
	}

	// L29: expand a leading ~ / ~/ / ~user first, so the directory
	// and base below are computed from the real path. The old code
	// special-cased ~ and ~/ in the branches and never resolved ~user
	// at all; a bare ~ also set basePart "" without listDir, so it
	// never matched anything.
	searchPath := expandTilde(pathToSearch)

	// Split directory and base using Linux path separators.
	normPath := searchPath
	dirPart := filepath.Dir(normPath)
	basePart := filepath.Base(normPath)

	// listDir is set only when the token ends in `/`: the user named a
	// directory and wants its first entry, not a completion of the name.
	listDir := false
	searchDir := cwd
	if strings.HasSuffix(searchPath, "/") {
		listDir = true
		// M25: a token ending in `/` names a directory — complete *inside*
		// it (empty base) instead of completing the directory's own name
		// (`cd /usr/` used to suggest `/`, and `cd ../` nothing at all).
		dir := strings.TrimSuffix(searchPath, "/")
		if dir == "" {
			dir = "/"
		}
		if filepath.IsAbs(dir) {
			searchDir = dir
		} else {
			searchDir = filepath.Join(cwd, dir)
		}
		basePart = ""
	} else if filepath.IsAbs(searchPath) {
		if dirPart == "/" {
			searchDir = "/"
		} else {
			searchDir = filepath.FromSlash(dirPart)
		}
	} else if strings.Contains(searchPath, "/") {
		searchDir = filepath.Join(cwd, filepath.FromSlash(dirPart))
	} else {
		searchDir = cwd
		basePart = searchPath
	}

	// L33: one listing per directory per second, not one per keystroke.
	entries, err := cachedReadDir(searchDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(basePart, ".") {
			continue // Skip hidden files unless explicitly requested
		}

		// L32: the suffix starts at the rune boundary foldPrefixLen
		// returns, not at len(basePart) bytes.
		if idx := foldPrefixLen(name, basePart); (len(basePart) > 0 || listDir) && idx >= 0 {
			suffix := name[idx:]
			dirSep := "/"
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
			suffix = strings.ReplaceAll(suffix, " ", "\\ ")
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
			target := prevToken + " "
			for _, entry := range entries {
				name := entry.Name()
				// L32: rune-boundary suffix, as above.
				if idx := foldPrefixLen(name, target); idx >= 0 {
					suffix := name[idx:]
					dirSep := "/"
					if entry.IsDir() {
						suffix += dirSep
					}
					suffix = strings.ReplaceAll(suffix, " ", "\\ ")
					return suffix
				}
			}
		}
	}

	return ""
}
