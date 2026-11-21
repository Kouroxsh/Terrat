package diagnostics

import (
	"bufio"
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	shellAliases sync.Map
	gitAliases   sync.Map
	aliasesOnce  sync.Once
	// aliasProbesDone closes once the async alias probes (shell + git
	// config) have finished, so the first lookups can wait for them (M39).
	aliasProbesDone = make(chan struct{})
	// aliasProbeWaitOnce makes the bounded wait below happen exactly once:
	// only the very first lookup stalls for the probes, later ones answer
	// immediately with whatever is known so far.
	aliasProbeWaitOnce sync.Once
)

// aliasProbeWait is how long a first alias lookup waits for the async
// probes before giving up and answering with what is known so far.
const aliasProbeWait = 500 * time.Millisecond

// waitForAliasProbes blocks (once, on the first lookup) until the alias
// probes finish, or the bounded wait above elapses. Later lookups return
// immediately: stalling every keystroke while the probes run would freeze
// typing for the first second after startup.
func waitForAliasProbes() {
	aliasProbeWaitOnce.Do(func() {
		select {
		case <-aliasProbesDone:
		case <-time.After(aliasProbeWait):
		}
	})
}

func RegisterAlias(name, expansion string) {
	name = strings.TrimSpace(name)
	if name != "" {
		shellAliases.Store(name, expansion)
	}
}

func RegisterAliasFromLine(line string) {
	parseAliasLine(line)
}

func IsAlias(name string) bool {
	InitAliases()
	waitForAliasProbes()
	_, ok := shellAliases.Load(name)
	return ok
}

func RegisterGitAlias(name string) {
	name = strings.TrimSpace(name)
	if name != "" {
		gitAliases.Store(name, true)
	}
}

func IsGitAlias(sub string) bool {
	InitAliases()
	waitForAliasProbes()
	_, ok := gitAliases.Load(sub)
	return ok
}

func InitAliases() {
	aliasesOnce.Do(func() {
		loadAliasesFromFiles()
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			probeGitAliases()
		}()
		go func() {
			defer wg.Done()
			probeShellAliases()
		}()
		go func() {
			wg.Wait()
			close(aliasProbesDone)
		}()
	})
}

func parseAliasLine(line string) {
	trimmed := strings.TrimSpace(line)
	// L28: `alias` may be followed by a tab, not just a space
	// (`alias	ll='ls'`). The separator char must be a blank — a word
	// like `aliasfoo=bar` is not an alias definition.
	if strings.HasPrefix(trimmed, "alias") && len(trimmed) > len("alias") && (trimmed[len("alias")] == ' ' || trimmed[len("alias")] == '\t') {
		rest := strings.TrimSpace(trimmed[len("alias"):])
		if eqIdx := strings.IndexByte(rest, '='); eqIdx > 0 {
			name := strings.TrimSpace(rest[:eqIdx])
			val := strings.TrimSpace(rest[eqIdx+1:])
			val = strings.Trim(val, `'"`)
			RegisterAlias(name, val)
		} else if spIdx := strings.IndexByte(rest, ' '); spIdx > 0 {
			name := strings.TrimSpace(rest[:spIdx])
			val := strings.TrimSpace(rest[spIdx+1:])
			val = strings.Trim(val, `'"`)
			RegisterAlias(name, val)
		}
	} else if strings.HasPrefix(trimmed, "abbr ") {
		parts := strings.Fields(trimmed)
		if len(parts) >= 3 {
			if parts[1] == "-a" || parts[1] == "--add" {
				if len(parts) >= 4 {
					RegisterAlias(parts[2], parts[3])
				}
			} else if !strings.HasPrefix(parts[1], "-") {
				RegisterAlias(parts[1], parts[2])
			}
		}
	}
}

// scanAliasFile feeds every line of path to fn. The scanner buffer is
// raised to 1MB (L28): the default bufio.Scanner cap is 64KB, so a single
// long alias line used to abort the scan and silently drop every alias
// below it in the file. A read error is logged instead of swallowed —
// the lines already read stay registered, but a truncated file must not
// look fully loaded.
func scanAliasFile(path string, fn func(line string)) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		fn(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Printf("terrat: error reading alias file %s: %v", path, err)
	}
}

func loadAliasesFromFiles() {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return
	}

	// L28: .bash_profile, .profile, .zsh_aliases and .aliases were never
	// scanned, so aliases defined only there were invisible; neither were
	// fish conf.d snippets.
	filesToScan := []string{
		filepath.Join(home, ".bash_aliases"),
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".bash_profile"),
		filepath.Join(home, ".profile"),
		filepath.Join(home, ".zsh_aliases"),
		filepath.Join(home, ".aliases"),
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".config", "fish", "config.fish"),
	}
	if matches, err := filepath.Glob(filepath.Join(home, ".config", "fish", "conf.d", "*.fish")); err == nil {
		filesToScan = append(filesToScan, matches...)
	}

	for _, p := range filesToScan {
		scanAliasFile(p, parseAliasLine)
	}
}

func loadGitAliases() {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		gitConfigFile := filepath.Join(home, ".gitconfig")
		inAliasSection := false
		scanAliasFile(gitConfigFile, func(text string) {
			line := strings.TrimSpace(text)
			if strings.HasPrefix(line, "[") {
				inAliasSection = strings.EqualFold(line, "[alias]")
				return
			}
			if inAliasSection && strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				aliasName := strings.TrimSpace(parts[0])
				if aliasName != "" {
					RegisterGitAlias(aliasName)
				}
			}
		})
	}

}

// probeGitAliases reads aliases from the git config (all scopes). It runs
// in a tracked goroutine so first-use lookups can wait for it (M39).
func probeGitAliases() {
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "config", "--get-regexp", `^alias\.`)
	out, err := cmd.Output()
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "alias.") {
			rest := strings.TrimPrefix(line, "alias.")
			parts := strings.Fields(rest)
			if len(parts) > 0 {
				RegisterGitAlias(parts[0])
			}
		}
	}
	_ = scanner.Err()
}

func probeShellAliases() {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
	defer cancel()

	var cmd *exec.Cmd
	baseShell := filepath.Base(shell)
	if baseShell == "fish" {
		cmd = exec.CommandContext(ctx, shell, "-c", "alias; abbr --show")
	} else {
		cmd = exec.CommandContext(ctx, shell, "-i", "-c", "alias")
	}

	out, err := cmd.Output()
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		parseAliasLine(scanner.Text())
	}
	_ = scanner.Err()
}
