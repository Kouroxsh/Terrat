package pty

import (
	"os"
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

func ResolveUnixShell(configuredShell string, customCmd ...string) []string {
	if len(customCmd) > 0 && customCmd[0] != "" {
		return customCmd
	}

	shellParts := parseCommandLine(configuredShell, os.Stat)
	if len(shellParts) > 0 {
		return shellParts
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
		if _, err := os.Stat(shell); err != nil {
			shell = "/bin/sh"
		}
	}
	return []string{shell}
}
