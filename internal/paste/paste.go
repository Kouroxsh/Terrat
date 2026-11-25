package paste

import (
	"regexp"
	"strings"
)

var ansiRegex = regexp.MustCompile(`\x1b(\[[0-9:;<=>?]*[ -/]*[@-~]|\[[0-9:;<=>?]*[ -/]*|\][^\x07\x1b]*(\x07|\x1b\\)|\][^\x07\x1b]*|[PX^_][^\x1b]*\x1b\\|[PX^_][^\x1b]*|.)`)

type SanitizeResult struct {
	Sanitized string
	Warnings  []string
}

func Sanitize(raw string) SanitizeResult {
	var warnings []string

	text := strings.ReplaceAll(raw, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	if ansiRegex.MatchString(text) {
		text = ansiRegex.ReplaceAllString(text, "")
		warnings = append(warnings, "Stripped raw ANSI escape sequences (potential pastejacking attack)")
	}

	var sb strings.Builder
	hasInvisible := false

	hasControl := false
	for _, r := range text {
		if isDangerousUnicode(r) {
			hasInvisible = true
			continue
		}

		if (r < 0x20 && r != '\t' && r != '\n') || r == 0x7f {
			hasControl = true
			continue
		}

		sb.WriteRune(r)
	}

	if hasControl {
		warnings = append(warnings, "Stripped control characters")
	}

	if hasInvisible {
		warnings = append(warnings, "Stripped invisible zero-width and bidirectional override characters")
	}

	return SanitizeResult{
		Sanitized: sb.String(),
		Warnings:  warnings,
	}
}

func isDangerousUnicode(r rune) bool {
	if r >= '\u0080' && r <= '\u009F' {
		return true
	}
	switch r {
	case '\u200B',
		'\u200D',
		'\u2060',
		'\uFEFF',
		'\u00AD',
		'\u200E',
		'\u200F',
		'\u202A',
		'\u202B',
		'\u202C',
		'\u202D',
		'\u202E',
		'\u2066',
		'\u2067',
		'\u2068',
		'\u2069',

		'\u061C',
		'\u2061',
		'\u2062',
		'\u2063',
		'\u2064',
		'\u180E',
		'\u034F':
		return true
	}
	return false
}
