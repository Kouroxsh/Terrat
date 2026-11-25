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

func LexCommandLine(input string) LexResult {
	trimmed := strings.TrimRight(input, " \t\r\n")
	if len(trimmed) == 0 {
		return LexResult{}
	}

	var quoteState QuoteState = QuoteNone
	escaped := false
	lastSplitIndex := -1
	delimLen := 0

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

		if b == '\'' {
			quoteState = QuoteSingle
			continue
		}
		if b == '"' {
			quoteState = QuoteDouble
			continue
		}

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
				continue
			}
		}

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
