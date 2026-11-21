package diagnostics

import (
	"strings"
)

type QuoteState int

const (
	QuoteNone QuoteState = iota
	QuoteSingle
	QuoteDouble
)

type LexResult struct {
	UnclosedQuote QuoteState
	Prefix        string
	ActiveSegment string
	Tokens        []string
}

// LexCommandLine parses a raw command line into prefix, active command segment,
// and tokenized arguments using a shell grammar state machine.
func LexCommandLine(input string) LexResult {
	trimmed := strings.TrimRight(input, " \t\r\n")
	if len(trimmed) == 0 {
		return LexResult{}
	}

	var quoteState QuoteState = QuoteNone
	escaped := false
	lastSplitIndex := -1
	delimLen := 0
	// L27: stack of open command/parameter substitutions — '(' for $(,
	// '{' for ${, '`' for backquotes. A ';', '|' or '&' inside one is
	// not a command separator for the outer line: splitting there made
	// the active segment start mid-substitution (e.g. `echo $(foo; gti)`
	// produced a bogus "gti)" typo with a quick fix that dropped the
	// closing paren). Only separators at depth 0 split.
	var subStack []byte

	for i := 0; i < len(input); i++ {
		b := input[i]

		if escaped {
			escaped = false
			continue
		}

		if b == '\\' && quoteState != QuoteSingle {
			escaped = true
			continue
		}

		if quoteState == QuoteSingle {
			if b == '\'' {
				quoteState = QuoteNone
			}
			continue
		}

		if quoteState == QuoteDouble {
			if b == '"' {
				quoteState = QuoteNone
			}
			continue
		}

		// Currently outside quotes
		if b == '\'' {
			quoteState = QuoteSingle
			continue
		}
		if b == '"' {
			quoteState = QuoteDouble
			continue
		}

		// Substitution tracking (outside quotes; an escaped $ or backtick
		// was already consumed above).
		if b == '$' && i+1 < len(input) && (input[i+1] == '(' || input[i+1] == '{') {
			subStack = append(subStack, input[i+1])
			i++
			continue
		}
		if b == '`' {
			if len(subStack) > 0 && subStack[len(subStack)-1] == '`' {
				subStack = subStack[:len(subStack)-1]
			} else {
				subStack = append(subStack, '`')
			}
			continue
		}
		if len(subStack) > 0 {
			top := subStack[len(subStack)-1]
			if top == '(' && b == ')' {
				subStack = subStack[:len(subStack)-1]
				continue
			}
			if top == '{' && b == '}' {
				subStack = subStack[:len(subStack)-1]
				continue
			}
			if b == ';' || b == '|' || b == '&' {
				// Literal separator inside a substitution — no split.
				continue
			}
		}

		// Check chaining delimiters outside quotes.
		//
		// '&' is a separator only when it is not part of a redirection:
		// 2>&1, >&2, <&0, &>file and &>>file all glue '&' to a redirect
		// operator, so splitting there would chop the command in half
		// ("gti 2>&1" would leave an active segment of just "1" and the
		// typo would never be reported). A lone '&' (background) and
		// '&&' still split; so does '&' after '|' ("cmd |& cat").
		if b == '&' {
			if i+1 < len(input) && input[i+1] == '&' {
				lastSplitIndex = i
				delimLen = 2
				i++
				continue
			}
			if i > 0 && (input[i-1] >= '0' && input[i-1] <= '9' || input[i-1] == '>' || input[i-1] == '<') {
				continue
			}
			if i+1 < len(input) && input[i+1] == '>' {
				continue
			}
			lastSplitIndex = i
			delimLen = 1
			continue
		}
		if b == ';' || b == '|' {
			if i+1 < len(input) {
				next := input[i+1]
				if b == '|' && next == '|' {
					lastSplitIndex = i
					delimLen = 2
					i++
					continue
				}
			}
			lastSplitIndex = i
			delimLen = 1
			continue
		}
	}

	prefix := ""
	activeSeg := input
	if lastSplitIndex >= 0 {
		splitEnd := lastSplitIndex + delimLen
		prefix = input[:splitEnd]
		rem := input[splitEnd:]
		trimmedLeft := strings.TrimLeft(rem, " \t")
		prefix += rem[:len(rem)-len(trimmedLeft)]
		activeSeg = trimmedLeft
	}

	tokens := tokenizeActiveSegment(activeSeg)

	return LexResult{
		UnclosedQuote: quoteState,
		Prefix:        prefix,
		ActiveSegment: activeSeg,
		Tokens:        tokens,
	}
}

func tokenizeActiveSegment(seg string) []string {
	var tokens []string
	var cur strings.Builder
	var quoteState QuoteState = QuoteNone
	escaped := false
	// L26: a token is "started" the moment a quote opens, even if nothing
	// is written inside it — `""` is an empty argument in the shell, not
	// the absence of one. Without this, `"" gti` tokenized to ["gti"] and
	// the typo was wrongly reported (the shell fails on the empty command
	// name, it never runs `gti`).
	tokenStarted := false

	flushToken := func() {
		if tokenStarted {
			tokens = append(tokens, cur.String())
			cur.Reset()
			tokenStarted = false
		}
	}

	for i := 0; i < len(seg); i++ {
		b := seg[i]

		if escaped {
			cur.WriteByte(b)
			tokenStarted = true
			escaped = false
			continue
		}

		if b == '\\' && quoteState != QuoteSingle {
			escaped = true
			continue
		}

		if quoteState == QuoteSingle {
			if b == '\'' {
				quoteState = QuoteNone
				continue
			}
			cur.WriteByte(b)
			tokenStarted = true
			continue
		}

		if quoteState == QuoteDouble {
			if b == '"' {
				quoteState = QuoteNone
				continue
			}
			cur.WriteByte(b)
			tokenStarted = true
			continue
		}

		// Outside quotes
		if b == '\'' {
			quoteState = QuoteSingle
			tokenStarted = true
			continue
		}
		if b == '"' {
			quoteState = QuoteDouble
			tokenStarted = true
			continue
		}

		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			flushToken()
			continue
		}

		cur.WriteByte(b)
		tokenStarted = true
	}

	flushToken()
	return tokens
}
