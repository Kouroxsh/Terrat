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

		// Check chaining delimiters outside quotes
		if b == ';' || b == '|' || b == '&' {
			if i+1 < len(input) {
				next := input[i+1]
				if (b == '&' && next == '&') || (b == '|' && next == '|') {
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

	flushToken := func() {
		if cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
		}
	}

	for i := 0; i < len(seg); i++ {
		b := seg[i]

		if escaped {
			cur.WriteByte(b)
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
			continue
		}

		if quoteState == QuoteDouble {
			if b == '"' {
				quoteState = QuoteNone
				continue
			}
			cur.WriteByte(b)
			continue
		}

		// Outside quotes
		if b == '\'' {
			quoteState = QuoteSingle
			continue
		}
		if b == '"' {
			quoteState = QuoteDouble
			continue
		}

		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			flushToken()
			continue
		}

		cur.WriteByte(b)
	}

	flushToken()
	return tokens
}
