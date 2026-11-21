package paste

import (
	"regexp"
	"strings"
)

// L34: the CSI class missed valid forms — parameter bytes 0x30-0x3F
// (digits, ; : < = > ?), intermediate bytes 0x20-0x2F (space, !, ",
// #, $, ...), final byte 0x40-0x7E. The old `[0-9;?]*[a-zA-Z~]` dropped
// mouse reports (`ESC[<0;10;5M`), modifyOtherKeys (`ESC[>4;2m`),
// sub-parameters (`ESC[1:2m`), soft reset (`ESC[!p`) and ICH (`ESC[@`).
// Unterminated OSC/DCS/SOS/PM/APC sequences (no BEL/ST before the next
// ESC or end of text) now match too: the old regex left their payload
// in the pasted text, newlines and all. Each unterminated alternative
// sits after its terminated one, so a terminated sequence still matches
// whole (the terminator is not left behind for the control filter).
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
	// L35: control characters were stripped silently — track them so
	// the user is told something was removed from their paste.
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
	// M22: C1 controls U+0080–U+009F. U+009B is a single-byte CSI, U+009D
	// an OSC introducer and U+009C the string terminator — all of them
	// used to pass Sanitize untouched.
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
		// M23: more invisible/bidi characters that can reorder what the
		// user sees: Arabic Letter Mark, invisible operators, Mongolian
		// vowel separator, combining grapheme joiner.
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
