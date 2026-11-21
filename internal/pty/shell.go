package pty

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var (
	defaultShellMu sync.RWMutex
	defaultShell   string
)

func SetDefaultShell(shell string) {
	defaultShellMu.Lock()
	defer defaultShellMu.Unlock()
	defaultShell = shell
}

func GetDefaultShell() string {
	defaultShellMu.RLock()
	defer defaultShellMu.RUnlock()
	return defaultShell
}

func cleanBaseName(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	return filepath.Base(p)
}

func ExtractShellName(exe string) string {
	exe = strings.TrimSpace(exe)
	if exe == "" {
		return "bash"
	}
	base := strings.ToLower(cleanBaseName(exe))
	if base == "" || base == "." {
		return "bash"
	}
	return base
}

func parseCommandLine(cmdStr string, stat func(string) (os.FileInfo, error)) []string {
	cmdStr = strings.TrimSpace(cmdStr)
	if cmdStr == "" {
		return nil
	}

	if stat != nil {
		if fi, err := stat(cmdStr); err == nil && !fi.IsDir() {
			return []string{cmdStr}
		}
	}

	var parts []string
	var cur strings.Builder
	inQuotes := false
	quoteChar := byte(0)

	for i := 0; i < len(cmdStr); i++ {
		ch := cmdStr[i]
		if inQuotes {
			if ch == quoteChar {
				inQuotes = false
			} else {
				cur.WriteByte(ch)
			}
		} else {
			switch ch {
			case '"', '\'':
				inQuotes = true
				quoteChar = ch
			case ' ', '\t':
				if cur.Len() > 0 {
					parts = append(parts, cur.String())
					cur.Reset()
				}
			default:
				cur.WriteByte(ch)
			}
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

// executableShell reports whether parts[0] names a runnable binary:
// an absolute/relative path must exist and have an exec bit, a bare name
// must resolve on PATH.
func executableShell(parts []string) bool {
	if len(parts) == 0 || parts[0] == "" {
		return false
	}
	if strings.Contains(parts[0], "/") {
		fi, err := os.Stat(parts[0])
		return err == nil && !fi.IsDir() && fi.Mode()&0111 != 0
	}
	_, err := exec.LookPath(parts[0])
	return err == nil
}

// ResolveUnixShell picks the shell to run: an explicit -e command wins,
// then the configured shell, then $SHELL, then /bin/bash, then /bin/sh.
// Every candidate is validated (M17): a configured or $SHELL value that
// does not exist used to be returned verbatim, so pty.Start failed and
// the app exited 1 without ever opening a window.
func ResolveUnixShell(configuredShell string, customCmd ...string) []string {
	if len(customCmd) > 0 && customCmd[0] != "" {
		return customCmd
	}

	if parts := parseCommandLine(configuredShell, os.Stat); executableShell(parts) {
		return parts
	}

	if sh := os.Getenv("SHELL"); sh != "" {
		if parts := parseCommandLine(sh, os.Stat); executableShell(parts) {
			return parts
		}
	}

	for _, cand := range []string{"/bin/bash", "/bin/sh"} {
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() && fi.Mode()&0111 != 0 {
			return []string{cand}
		}
	}
	return []string{"/bin/sh"}
}
