package diagnostics

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type IssueSeverity int

const (
	SeverityNone IssueSeverity = iota
	SeverityHint
	SeverityWarning
	SeverityError
)

type Diagnostic struct {
	Severity   IssueSeverity
	Message    string
	Suggestion string
	QuickFix   string
}

// pathCacheEntry is a cached LookPath result. Positive results are kept
// forever; negative ones expire (M12) so a command installed after Terrat
// started stops being reported as unknown within half a minute.
type pathCacheEntry struct {
	exists    bool
	checkedAt time.Time
}

const negativePathCacheTTL = 30 * time.Second

var pathCache sync.Map // string -> pathCacheEntry

func isCommandOnPath(name string) bool {
	if len(name) < 2 || len(name) > 40 {
		return false
	}
	for _, r := range name {
		if r > 127 || r == '/' || r == '\\' || r == ':' || r <= 32 {
			return false
		}
	}
	if v, ok := pathCache.Load(name); ok {
		e := v.(pathCacheEntry)
		if e.exists || time.Since(e.checkedAt) < negativePathCacheTTL {
			return e.exists
		}
	}
	_, err := exec.LookPath(name)
	exists := (err == nil)
	pathCache.Store(name, pathCacheEntry{exists: exists, checkedAt: time.Now()})
	return exists
}

var shellBuiltins = map[string]bool{
	"alias": true, "bg": true, "bind": true, "break": true, "builtin": true,
	"case": true, "cd": true, "command": true, "continue": true, "declare": true,
	"dirs": true, "disown": true, "echo": true, "enable": true, "eval": true,
	"exec": true, "exit": true, "export": true, "fc": true, "fg": true,
	"getopts": true, "hash": true, "help": true, "history": true, "jobs": true,
	"kill": true, "let": true, "local": true, "logout": true, "popd": true,
	"pushd": true, "pwd": true, "read": true, "readonly": true, "return": true,
	"set": true, "shift": true, "shopt": true, "source": true, "suspend": true,
	"test": true, "times": true, "trap": true, "type": true, "typeset": true,
	"ulimit": true, "umask": true, "unalias": true, "unset": true, "wait": true,
	"true": true, "false": true, ":": true, ".": true, "[": true, "[[": true,
	"if": true, "then": true, "else": true, "elif": true, "fi": true,
	"for": true, "while": true, "until": true, "do": true, "done": true,
	"in": true, "esac": true, "select": true, "function": true, "time": true,
}

// L24: this is a list of commands that exist on Linux. The Windows-only
// commands (cls, cmd, dir, ipconfig, winget) used to be here too, which
// silently suppressed them — typing one on Linux produced no diagnostic
// at all. They are gone now, so a typo like `cmd` suggests `cd` instead
// of being swallowed.
var commonBaseCommands = []string{
	"cat", "cd", "cp", "curl", "chmod", "chown", "clear",
	"df", "diff", "docker", "du",
	"echo", "exit", "export",
	"find", "free",
	"git", "go", "grep", "gzip",
	"head", "history", "htop",
	"ip",
	"journalctl",
	"kill", "killall",
	"less", "ln", "ls",
	"make", "man", "mkdir", "mv",
	"nano", "nc", "netstat", "npm", "nvim",
	"ping", "pkill", "ps", "pwd", "pnpm", "python", "python3",
	"rm", "rmdir",
	"sed", "ssh", "sudo", "systemctl",
	"tail", "tar", "top", "touch", "tree",
	"uname", "uptime",
	"vim",
	"wget", "where", "which", "whoami",
	"yarn",
}

var commonBaseSet = func() map[string]bool {
	m := make(map[string]bool, len(commonBaseCommands))
	for _, c := range commonBaseCommands {
		m[c] = true
	}
	return m
}()

var commonTypos = map[string]string{
	"sl":     "ls",
	"gti":    "git",
	"gut":    "git",
	"mkae":   "make",
	"dokcer": "docker",
	"dcoker": "docker",
	"clea":   "clear",
	"claer":  "clear",
	"cLear":  "clear",
	"gerp":   "grep",
	"grpe":   "grep",
	"exist":  "exit",
	"exti":   "exit",
	// L25: "qui" used to map to "quit", but there is no `quit` command
	// or builtin on Linux — suggesting it sent users to a dead end.
	"pyhton": "python",
	"phtyon": "python",
	"shh":    "ssh",
	"whcih":  "which",
	"sudp":   "sudo",
	"sduo":   "sudo",
}

var gitSubcommands = []string{
	"add", "am", "annotate", "apply", "archive", "bisect", "blame", "branch", "bundle",
	"checkout", "cherry", "cherry-pick", "clean", "clone", "commit", "config",
	"describe", "diff", "fetch", "format-patch", "gc", "grep", "help", "init",
	"log", "merge", "mv", "notes", "pull", "push", "range-diff", "rebase", "reflog",
	"remote", "repack", "replace", "reset", "restore", "revert", "rm", "shortlog",
	"show", "sparse-checkout", "stash", "status", "submodule", "switch", "tag",
	"version", "whatchanged", "worktree",
}

var dockerSubcommands = []string{
	"attach", "build", "builder", "checkpoint", "commit", "compose", "config",
	"container", "context", "cp", "create", "diff", "events", "exec", "export",
	"history", "image", "images", "import", "info", "init", "inspect", "kill",
	"load", "login", "logout", "logs", "manifest", "network", "node", "pause",
	"plugin", "port", "ps", "pull", "push", "rename", "restart", "rm", "rmi",
	"run", "save", "search", "secret", "service", "stack", "start", "stats",
	"stop", "swarm", "system", "tag", "top", "trust", "unpause", "update",
	"version", "volume", "wait",
}

var systemctlSubcommands = []string{
	"cat", "clean", "daemon-reexec", "daemon-reload", "default", "disable",
	"edit", "emergency", "enable", "exit", "halt", "help", "hibernate",
	"hybrid-sleep", "is-active", "is-enabled", "is-failed", "isolate", "kexec",
	"kill", "list-dependencies", "list-jobs", "list-sockets", "list-timers",
	"list-unit-files", "list-units", "mask", "poweroff", "preset", "reboot",
	"reenable", "reload", "reload-or-restart", "rescue", "reset-failed", "restart",
	"set-default", "show", "start", "status", "stop", "suspend", "switch-root",
	"try-restart", "unmask",
}

var goSubcommands = []string{
	"build", "clean", "doc", "env", "fix", "fmt", "generate", "get", "install",
	"list", "mod", "work", "run", "test", "tool", "version", "vet",
}

var cargoSubcommands = []string{
	"build", "check", "clean", "doc", "new", "init", "run", "test", "bench",
	"update", "search", "publish", "install", "uninstall", "add", "remove",
	"metadata", "clippy", "fmt", "version",
}

var npmSubcommands = []string{
	"install", "i", "ci", "test", "run", "start", "build", "init", "publish",
	"audit", "cache", "config", "outdated", "update", "uninstall", "link",
	"list", "pack", "version", "view", "exec",
}

var privilegedCommands = map[string]string{
	"apt":      "sudo apt",
	"apt-get":  "sudo apt-get",
	"pacman":   "sudo pacman",
	"dnf":      "sudo dnf",
	"yum":      "sudo yum",
	"reboot":   "sudo reboot",
	"poweroff": "sudo poweroff",
	"shutdown": "sudo shutdown",
	"useradd":  "sudo useradd",
	"userdel":  "sudo userdel",
	"groupadd": "sudo groupadd",
	"fdisk":    "sudo fdisk",
	"mkfs":     "sudo mkfs",
	"iptables": "sudo iptables",
	"ufw":      "sudo ufw",
}

var safeReadOperations = map[string][]string{
	"apt":     {"search", "show", "list", "depends", "rdepends", "policy", "help", "--help", "-h", "--version", "-v"},
	"apt-get": {"help", "--help", "-h", "--version", "-v", "source"},
	// Pacman entries are case-sensitive (M37): -Ss/-Si are read-only sync
	// search/info, -Qs/-Qi/... are read-only queries, while -SS is a sync
	// (install) operation and must NOT match. Lowercase duplicates are kept
	// for leniency — they match no privileged operation.
	"pacman":   {"-ss", "-Ss", "-si", "-Si", "-q", "-Q", "-qs", "-Qs", "-qi", "-Qi", "-ql", "-Ql", "-qe", "-Qe", "-qo", "-Qo", "-qu", "-Qu", "-qk", "-Qk", "-qd", "-Qd", "-qdt", "-Qdt", "-f", "-F", "-fy", "-Fy", "-fs", "-Fs", "-v", "--help", "--version"},
	"dnf":      {"search", "info", "list", "repoquery", "check-update", "help", "--help", "--version"},
	"yum":      {"search", "info", "list", "check-update", "help", "--help", "--version"},
	"ufw":      {"status", "show", "version", "--help"},
	"fdisk":    {"-l", "--list", "-v", "--version"},
	"iptables": {"-l", "-s", "-v", "--list", "-n", "--version"},
}

func LevenshteinDistance(s, t string) int {
	d := make([][]int, len(s)+1)
	for i := range d {
		d[i] = make([]int, len(t)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}
	for j := 1; j <= len(t); j++ {
		for i := 1; i <= len(s); i++ {
			cost := 0
			if s[i-1] != t[j-1] {
				cost = 1
			}
			d[i][j] = min(
				d[i-1][j]+1,
				d[i][j-1]+1,
				d[i-1][j-1]+cost,
			)
		}
	}
	return d[len(s)][len(t)]
}

// isSubsequence reports whether word appears in cand in order (not
// necessarily contiguously): the typo is the candidate with characters
// deleted, the most common typo class for short words.
func isSubsequence(word, cand string) bool {
	if word == "" {
		return true
	}
	j := 0
	for i := 0; i < len(cand) && j < len(word); i++ {
		if cand[i] == word[j] {
			j++
		}
	}
	return j == len(word)
}

// DamerauLevenshteinDistance is Levenshtein distance with an adjacent
// transposition costing 1 instead of 2 (optimal string alignment).
// Transpositions are the most common typo class (`cta` for `cat`, `plu`
// for `pull`), and plain Levenshtein scores them as two edits, which put
// short typos out of reach of any sane threshold: `plu` is 3 edits from
// `pull` under Levenshtein, 2 under Damerau — and 3 from `push`, so the
// match is unambiguous.
func DamerauLevenshteinDistance(s, t string) int {
	d := make([][]int, len(s)+1)
	for i := range d {
		d[i] = make([]int, len(t)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}
	for j := 1; j <= len(t); j++ {
		for i := 1; i <= len(s); i++ {
			cost := 0
			if s[i-1] != t[j-1] {
				cost = 1
			}
			d[i][j] = min(
				d[i-1][j]+1,
				d[i][j-1]+1,
				d[i-1][j-1]+cost,
			)
			if i > 1 && j > 1 && s[i-1] == t[j-2] && s[i-2] == t[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}
	return d[len(s)][len(t)]
}

func FindClosestMatch(word string, candidates []string, maxDist int) string {
	lowerWord := strings.ToLower(word)
	bestDist := maxDist + 1
	bestMatch := ""
	bestIsSubseq := false

	for _, cand := range candidates {
		if strings.EqualFold(cand, word) {
			return ""
		}
		dist := DamerauLevenshteinDistance(lowerWord, strings.ToLower(cand))
		if dist > maxDist {
			continue
		}
		// On a distance tie, prefer a candidate the typo is a subsequence
		// of (a deletion typo, e.g. `rn` for `run`): without this,
		// `docker rn` suggested `rm` — same distance, but a destructive
		// command and the less likely reading.
		sub := isSubsequence(lowerWord, strings.ToLower(cand))
		if dist < bestDist || (dist == bestDist && sub && !bestIsSubseq) {
			bestDist = dist
			bestMatch = cand
			bestIsSubseq = sub
		}
	}
	return bestMatch
}

// globalValueFlags lists, per command, the global flags that consume the
// following token as their value (M38). Unknown flags are skipped as bare
// flags; a flag with "=" attached (e.g. --color=auto) needs no value skip.
var globalValueFlags = map[string]map[string]bool{
	"apt":       {"-c": true, "--config-file": true, "-o": true, "--option": true, "-t": true, "--target-release": true},
	"apt-get":   {"-c": true, "--config-file": true, "-o": true, "--option": true, "-t": true, "--target-release": true},
	"pacman":    {"--color": true, "--config": true, "--dbpath": true, "-b": true, "--root": true, "-r": true, "--cachedir": true, "--logfile": true, "--gpgdir": true},
	"dnf":       {"-c": true, "--config": true, "--setopt": true, "--releasever": true, "--installroot": true},
	"yum":       {"-c": true, "--config": true, "--setopt": true, "--releasever": true, "--installroot": true},
	"git":       {"-c": true, "-C": true, "--git-dir": true, "--work-tree": true, "--namespace": true},
	"docker":    {"-c": true, "--context": true, "-H": true, "--host": true, "--config": true, "--log-level": true},
	"systemctl": {"-H": true, "--host": true, "-M": true, "--machine": true, "--root": true},
	"go":        {"-C": true},
	"cargo":     {"--config": true, "--manifest-path": true, "--color": true, "--target": true},
	"npm":       {"--config": true, "--prefix": true, "--cache": true, "--registry": true},
}

// findSubcommand returns the subcommand token after the command name,
// skipping global flags (and their values). "" when there is none, e.g. a
// bare `apt` or `apt --version`. Pacman is special: its operation itself
// is a flag cluster (`-Ss`), so after skipping global flags the first
// remaining token is the operation even when it starts with `-`.
func findSubcommand(cmd string, parts []string) string {
	valueFlags := globalValueFlags[cmd]
	isSafeLong := func(tok string) bool {
		for _, s := range safeReadOperations[cmd] {
			if s == tok {
				return true
			}
		}
		return false
	}
	for i := 1; i < len(parts); i++ {
		tok := parts[i]
		if strings.HasPrefix(tok, "-") {
			if strings.Contains(tok, "=") {
				continue
			}
			if valueFlags != nil && valueFlags[tok] {
				i++ // skip the flag's value
				continue
			}
			if cmd == "pacman" {
				// Long options are global flags — except the query
				// operations in the safe list (`--help`, `--version`).
				if strings.HasPrefix(tok, "--") && !isSafeLong(tok) {
					continue
				}
				return tok
			}
			continue
		}
		return tok
	}
	return ""
}

func isSafeOperation(cmd string, parts []string) bool {
	safeOps, ok := safeReadOperations[cmd]
	if !ok {
		return false
	}
	sub := findSubcommand(cmd, parts)
	if sub == "" {
		return false
	}
	// M37: pacman short options are case-sensitive — `-SS` (sync) must not
	// be lowercased into `-ss` (search) and treated as safe.
	if cmd != "pacman" {
		sub = strings.ToLower(sub)
	}
	for _, safe := range safeOps {
		if sub == safe {
			return true
		}
	}
	return false
}

// needsRoot reports whether invoking cmd with these arguments actually
// does something privileged. A bare `apt`, `ufw` or `fdisk` only prints
// usage and needs no sudo — warning there was a false positive (L22).
// reboot/poweroff/shutdown are the exceptions: they act with no arguments.
func needsRoot(cmd string, parts []string) bool {
	if findSubcommand(cmd, parts) != "" {
		return true
	}
	switch cmd {
	case "reboot", "poweroff", "shutdown":
		return true
	}
	return false
}

func unnestSudo(parts []string) (cleanedParts []string, hasSudo bool) {
	if len(parts) == 0 || parts[0] != "sudo" {
		return parts, false
	}
	i := 1
	for i < len(parts) {
		arg := parts[i]
		if strings.HasPrefix(arg, "-") {
			if (arg == "-u" || arg == "-g" || arg == "-p" || arg == "-c") && i+1 < len(parts) {
				i += 2
				continue
			}
			i++
			continue
		}
		break
	}
	if i < len(parts) {
		return parts[i:], true
	}
	return nil, true
}

// isEnvAssignment reports whether tok looks like a VAR=value assignment
// that precedes a command (M11).
func isEnvAssignment(tok string) bool {
	eq := strings.IndexByte(tok, '=')
	if eq <= 0 {
		return false
	}
	for i, r := range tok[:eq] {
		if r != '_' && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (i == 0 || r < '0' || r > '9') {
			return false
		}
	}
	return true
}

// stripCommandPrefixes drops leading VAR=value assignments and wrapper
// commands (time/command/env/nice) so the real command is analysed (M11):
// `FOO=bar apt install x` must warn like `apt install x`, and
// `FOO=bar gti status` must suggest `git`.
func stripCommandPrefixes(parts []string) []string {
	i := 0
	for i < len(parts) {
		tok := parts[i]
		if isEnvAssignment(tok) {
			i++
			continue
		}
		switch tok {
		case "time":
			i++
			for i < len(parts) && parts[i] == "-p" {
				i++
			}
		case "command":
			// `command -v/-V name` queries the shell instead of running the
			// command; only strip a bare `command` before a real word.
			if i+1 < len(parts) && !strings.HasPrefix(parts[i+1], "-") {
				i++
			} else {
				return parts[i:]
			}
		case "env":
			i++
			for i < len(parts) {
				t := parts[i]
				if t == "-i" {
					i++
					continue
				}
				if t == "-u" {
					i += 2
					continue
				}
				if strings.HasPrefix(t, "-u") && len(t) > 2 {
					i++
					continue
				}
				if isEnvAssignment(t) {
					i++
					continue
				}
				break
			}
		case "nice":
			i++
			for i < len(parts) {
				t := parts[i]
				if t == "-n" || t == "--adjustment" {
					i += 2
					continue
				}
				if strings.HasPrefix(t, "--adjustment=") || (strings.HasPrefix(t, "-n") && len(t) > 2) || strings.HasPrefix(t, "-") {
					i++
					continue
				}
				break
			}
		default:
			return parts[i:]
		}
	}
	return parts[i:]
}

// indexOfCommand finds cmd in seg at a word boundary (start of string or
// after a blank), so a prefix like `FOO=bar` cannot shadow the match.
func indexOfCommand(seg, cmd string) int {
	for i := 0; i+len(cmd) <= len(seg); {
		j := strings.Index(seg[i:], cmd)
		if j < 0 {
			return -1
		}
		idx := i + j
		if idx == 0 || seg[idx-1] == ' ' || seg[idx-1] == '\t' {
			return idx
		}
		i = idx + 1
	}
	return -1
}

// suffixAfterBase returns the part of seg after the first word-boundary
// occurrence of cmd (the argument list). Falls back to the whole segment
// when cmd is not found, matching the old TrimPrefix behaviour.
func suffixAfterBase(seg, cmd string) string {
	if idx := indexOfCommand(seg, cmd); idx >= 0 {
		return seg[idx+len(cmd):]
	}
	return seg
}

func Analyze(rawInput string) *Diagnostic {
	trimmed := strings.TrimSpace(rawInput)
	if len(trimmed) < 2 {
		return nil
	}

	lex := LexCommandLine(rawInput)
	if lex.UnclosedQuote == QuoteSingle {
		return &Diagnostic{
			Severity:   SeverityWarning,
			Message:    "Unclosed single quote string (')",
			Suggestion: "Add closing single quote",
			QuickFix:   rawInput + "'",
		}
	}
	if lex.UnclosedQuote == QuoteDouble {
		return &Diagnostic{
			Severity:   SeverityWarning,
			Message:    "Unclosed double quote string (\")",
			Suggestion: "Add closing double quote",
			QuickFix:   rawInput + "\"",
		}
	}

	prefix := lex.Prefix
	activeSeg := lex.ActiveSegment
	if len(activeSeg) < 2 {
		return nil
	}

	parts := lex.Tokens
	if len(parts) == 0 {
		return nil
	}

	cmdParts, hasSudo := unnestSudo(stripCommandPrefixes(parts))
	cmdParts = stripCommandPrefixes(cmdParts)
	if len(cmdParts) == 0 {
		return nil
	}

	baseCmd := cmdParts[0]

	if !hasSudo && os.Geteuid() != 0 {
		if fix, ok := privilegedCommands[baseCmd]; ok {
			if !isSafeOperation(baseCmd, cmdParts) && needsRoot(baseCmd, cmdParts) {
				fixedLine := fix + suffixAfterBase(activeSeg, baseCmd)
				return &Diagnostic{
					Severity:   SeverityWarning,
					Message:    fmt.Sprintf("'%s' usually requires root privileges", baseCmd),
					Suggestion: fmt.Sprintf("Try: %s", fixedLine),
					QuickFix:   prefix + fixedLine,
				}
			}
		}
	}

	if sub := findSubcommand(baseCmd, cmdParts); sub != "" {
		if !strings.HasPrefix(sub, "-") {
			var subCandidates []string
			switch baseCmd {
			case "git":
				subCandidates = gitSubcommands
			case "docker":
				subCandidates = dockerSubcommands
			case "systemctl":
				subCandidates = systemctlSubcommands
			case "go":
				subCommands := goSubcommands
				subCandidates = subCommands
			case "cargo":
				subCandidates = cargoSubcommands
			case "npm":
				subCandidates = npmSubcommands
			}

			if len(subCandidates) > 0 {
				isKnown := false
				for _, sc := range subCandidates {
					if sc == sub {
						isKnown = true
						break
					}
				}

				if !isKnown {
					if baseCmd == "git" && IsGitAlias(sub) {
						return nil
					}

					// L23: short subcommands get suggestions too — `git plu`,
					// `docker rn` and `go tst` are one edit away from a real
					// subcommand. Single letters are still excluded (a lone
					// `git a` is more likely an abbreviation than a typo).
					// maxD 1 keeps short suggestions safe from false hits.
					if len(sub) >= 2 {
						maxD := 1
						// Three-letter typos get 2: a transposed + dropped
						// letter (`plu` for `pull`) is 2 Damerau edits.
						// Longer words stay at 1 until len 6, where 2 was
						// already the rule.
						if len(sub) == 3 || len(sub) >= 6 {
							maxD = 2
						}
						if match := FindClosestMatch(sub, subCandidates, maxD); match != "" {
							fixedLine := activeSeg
							baseIdx := indexOfCommand(activeSeg, baseCmd)
							if baseIdx != -1 {
								afterBase := activeSeg[baseIdx+len(baseCmd):]
								relSubIdx := strings.Index(afterBase, sub)
								if relSubIdx != -1 {
									subIdx := baseIdx + len(baseCmd) + relSubIdx
									fixedLine = activeSeg[:subIdx] + match + activeSeg[subIdx+len(sub):]
								}
							}
							return &Diagnostic{
								Severity:   SeverityError,
								Message:    fmt.Sprintf("Unknown %s subcommand '%s'", baseCmd, sub),
								Suggestion: fmt.Sprintf("Did you mean '%s'?", match),
								QuickFix:   prefix + fixedLine,
							}
						}
					}
				}
			}
		}
	}

	if strings.HasPrefix(baseCmd, "#") {
		return nil
	}

	if strings.Contains(baseCmd, "/") || strings.HasPrefix(baseCmd, ".") || strings.HasPrefix(baseCmd, "~") {
		return nil
	}

	if strings.Contains(baseCmd, "=") {
		return nil
	}

	if shellBuiltins[baseCmd] {
		return nil
	}

	if IsAlias(baseCmd) {
		return nil
	}

	if isCommandOnPath(baseCmd) {
		return nil
	}

	if fix, ok := commonTypos[baseCmd]; ok {
		fixedLine := fix + suffixAfterBase(activeSeg, baseCmd)
		return &Diagnostic{
			Severity:   SeverityError,
			Message:    fmt.Sprintf("Command '%s' not recognized", baseCmd),
			Suggestion: fmt.Sprintf("Did you mean '%s'?", fix),
			QuickFix:   prefix + fixedLine,
		}
	}

	if commonBaseSet[baseCmd] {
		return nil
	}

	if len(baseCmd) >= 2 && len(baseCmd) <= 15 {
		maxD := 1
		if len(baseCmd) >= 5 {
			maxD = 2
		}
		if match := FindClosestMatch(baseCmd, commonBaseCommands, maxD); match != "" {
			fixedLine := match + suffixAfterBase(activeSeg, baseCmd)
			return &Diagnostic{
				Severity:   SeverityError,
				Message:    fmt.Sprintf("Command '%s' not recognized", baseCmd),
				Suggestion: fmt.Sprintf("Did you mean '%s'?", match),
				QuickFix:   prefix + fixedLine,
			}
		}
	}

	return nil
}
