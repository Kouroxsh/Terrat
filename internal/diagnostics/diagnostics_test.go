package diagnostics

import (
	"os"
	"testing"
)

func TestLevenshteinDistance(t *testing.T) {
	cases := []struct {
		s, t     string
		expected int
	}{
		{"git", "gti", 2},
		{"docker", "dokcer", 2},
		{"ls", "sl", 2},
		{"cat", "cat", 0},
		{"make", "mkae", 2},
	}

	for _, c := range cases {
		d := LevenshteinDistance(c.s, c.t)
		if d != c.expected {
			t.Errorf("LevenshteinDistance(%q, %q) = %d; expected %d", c.s, c.t, d, c.expected)
		}
	}
}

func TestAnalyzeTypos(t *testing.T) {
	d := Analyze("gti status")
	if d == nil || d.Suggestion != "Did you mean 'git'?" {
		t.Fatalf("expected git suggestion for 'gti status', got %+v", d)
	}

	d = Analyze("dokcer ps")
	if d == nil || d.Suggestion != "Did you mean 'docker'?" {
		t.Fatalf("expected docker suggestion for 'dokcer ps', got %+v", d)
	}

	d = Analyze("git puch origin")
	if d == nil || d.Suggestion != "Did you mean 'push'?" {
		t.Fatalf("expected push suggestion for 'git puch origin', got %+v", d)
	}

	d = Analyze("git stats")
	if d == nil || d.Suggestion != "Did you mean 'status'?" {
		t.Fatalf("expected status suggestion for 'git stats', got %+v", d)
	}

	d = Analyze("git commit -m \"hello world")
	if d == nil || d.Severity != SeverityWarning {
		t.Fatalf("expected unclosed quote warning, got %+v", d)
	}

	if os.Geteuid() != 0 {
		d = Analyze("apt install neovim")
		if d == nil || d.Severity != SeverityWarning {
			t.Fatalf("expected root privilege warning for 'apt install', got %+v", d)
		}
	}

	if d := Analyze("apt search neovim"); d != nil {
		t.Fatalf("expected nil for 'apt search neovim', got %+v", d)
	}
	if d := Analyze("pacman -Ss firefox"); d != nil {
		t.Fatalf("expected nil for 'pacman -Ss firefox', got %+v", d)
	}

	if d := Analyze("git remote -v"); d != nil {
		t.Fatalf("expected nil for 'git remote -v', got %+v", d)
	}
	if d := Analyze("git config --global user.name foo"); d != nil {
		t.Fatalf("expected nil for 'git config', got %+v", d)
	}
	if d := Analyze("git blame main.go"); d != nil {
		t.Fatalf("expected nil for 'git blame', got %+v", d)
	}
	if d := Analyze("docker info"); d != nil {
		t.Fatalf("expected nil for 'docker info', got %+v", d)
	}
	if d := Analyze("docker stats"); d != nil {
		t.Fatalf("expected nil for 'docker stats', got %+v", d)
	}

	if d := Analyze("cd /tmp"); d != nil {
		t.Fatalf("expected nil for builtin 'cd', got %+v", d)
	}
	if d := Analyze("export PATH=$PATH:/foo"); d != nil {
		t.Fatalf("expected nil for builtin 'export', got %+v", d)
	}

	d = Analyze("sudo gti status")
	if d == nil || d.Suggestion != "Did you mean 'git'?" {
		t.Fatalf("expected git suggestion for 'sudo gti status', got %+v", d)
	}

	d = Analyze("cat main.go | gerp func")
	if d == nil || d.Suggestion != "Did you mean 'grep'?" {
		t.Fatalf("expected grep suggestion for pipeline 'cat main.go | gerp func', got %+v", d)
	}

	RegisterAlias("ll", "ls -alF")
	RegisterAlias("cls", "clear")
	RegisterAlias("gs", "git status")

	if d := Analyze("ll /home"); d != nil {
		t.Fatalf("expected nil for alias 'll', got %+v", d)
	}
	if d := Analyze("cls"); d != nil {
		t.Fatalf("expected nil for alias 'cls', got %+v", d)
	}
	if d := Analyze("gs"); d != nil {
		t.Fatalf("expected nil for alias 'gs', got %+v", d)
	}

	RegisterAliasFromLine("alias mytest='echo hello'")
	if !IsAlias("mytest") {
		t.Fatalf("expected 'mytest' to be registered as alias")
	}
	if d := Analyze("mytest"); d != nil {
		t.Fatalf("expected nil for alias 'mytest', got %+v", d)
	}

	RegisterGitAlias("deploy")
	RegisterGitAlias("st")
	if d := Analyze("git deploy \"my feature\""); d != nil {
		t.Fatalf("expected nil for git alias 'deploy', got %+v", d)
	}
	if d := Analyze("git st"); d != nil {
		t.Fatalf("expected nil for git alias 'st', got %+v", d)
	}

	if d := Analyze("cd \"my space folder\""); d != nil {
		t.Fatalf("expected nil for 'cd \"my space folder\"', got %+v", d)
	}
	if d := Analyze("cd my\\ space\\ folder"); d != nil {
		t.Fatalf("expected nil for 'cd my\\ space\\ folder', got %+v", d)
	}
	if d := Analyze("./my\\ script.sh"); d != nil {
		t.Fatalf("expected nil for './my\\ script.sh', got %+v", d)
	}

	d = Analyze("git status")
	if d != nil {
		t.Fatalf("expected nil for valid command, got %+v", d)
	}
}

func TestChainedCommandQuickFix(t *testing.T) {
	cases := []struct {
		input           string
		expectedSuggest string
		expectedFix     string
	}{
		{
			input:           "echo \"hello\" && sl",
			expectedSuggest: "Did you mean 'ls'?",
			expectedFix:     "echo \"hello\" && ls",
		},
		{
			input:           "cat file | gerp pattern",
			expectedSuggest: "Did you mean 'grep'?",
			expectedFix:     "cat file | grep pattern",
		},
		{
			input:           "git status && git puch origin",
			expectedSuggest: "Did you mean 'push'?",
			expectedFix:     "git status && git push origin",
		},
		{
			input:           "sl",
			expectedSuggest: "Did you mean 'ls'?",
			expectedFix:     "ls",
		},
	}

	for _, c := range cases {
		d := Analyze(c.input)
		if d == nil {
			t.Fatalf("expected diagnostic for %q, got nil", c.input)
		}
		if d.Suggestion != c.expectedSuggest {
			t.Errorf("for %q: expected Suggestion %q, got %q", c.input, c.expectedSuggest, d.Suggestion)
		}
		if d.QuickFix != c.expectedFix {
			t.Errorf("for %q: expected QuickFix %q, got %q", c.input, c.expectedFix, d.QuickFix)
		}
	}
}

func TestLexerEdgeCases(t *testing.T) {
	lex := LexCommandLine(`echo "hello && world"`)
	if lex.Prefix != "" {
		t.Fatalf("expected empty prefix, got %q", lex.Prefix)
	}
	if lex.ActiveSegment != `echo "hello && world"` {
		t.Fatalf("expected full segment, got %q", lex.ActiveSegment)
	}
	if len(lex.Tokens) != 2 || lex.Tokens[1] != "hello && world" {
		t.Fatalf("expected tokenized arguments without quotes, got %v", lex.Tokens)
	}

	lexQuotes := LexCommandLine(`echo "it's working"`)
	if lexQuotes.UnclosedQuote != QuoteNone {
		t.Fatalf("expected no unclosed quote, got %v", lexQuotes.UnclosedQuote)
	}

	lexEscaped := LexCommandLine(`echo hello \&\& world`)
	if lexEscaped.Prefix != "" {
		t.Fatalf("expected empty prefix for escaped &&, got %q", lexEscaped.Prefix)
	}
}

func TestSafeOperationsExactMatching(t *testing.T) {
	if !isSafeOperation("apt", []string{"apt", "search", "neovim"}) {
		t.Fatalf("expected apt search to be recognized as safe")
	}

	if isSafeOperation("apt", []string{"apt", "searchsomething", "neovim"}) {
		t.Fatalf("expected apt searchsomething to NOT match safe operation")
	}
}

func TestLexerRedirectAmpersand(t *testing.T) {
	lex := LexCommandLine("gti 2>&1")
	if lex.ActiveSegment != "gti 2>&1" {
		t.Fatalf("expected active segment %q, got %q", "gti 2>&1", lex.ActiveSegment)
	}
	if len(lex.Tokens) == 0 || lex.Tokens[0] != "gti" {
		t.Fatalf("expected first token %q, got %v", "gti", lex.Tokens)
	}
	if d := Analyze("gti 2>&1"); d == nil || d.Suggestion != "Did you mean 'git'?" {
		t.Fatalf("expected typo suggestion for 'gti 2>&1', got %#v", d)
	}

	lex = LexCommandLine("gti &> /dev/null")
	if lex.ActiveSegment != "gti &> /dev/null" {
		t.Fatalf("expected active segment %q, got %q", "gti &> /dev/null", lex.ActiveSegment)
	}
	if d := Analyze("gti &> /dev/null"); d == nil || d.Suggestion != "Did you mean 'git'?" {
		t.Fatalf("expected typo suggestion for 'gti &> /dev/null', got %#v", d)
	}

	lex = LexCommandLine("gti <&0")
	if lex.ActiveSegment != "gti <&0" {
		t.Fatalf("expected active segment %q, got %q", "gti <&0", lex.ActiveSegment)
	}

	lex = LexCommandLine("echo hi |& cat")
	if lex.ActiveSegment != "cat" {
		t.Fatalf("expected active segment %q, got %q", "cat", lex.ActiveSegment)
	}

	lex = LexCommandLine("sleep 1 & gti status")
	if lex.ActiveSegment != "gti status" {
		t.Fatalf("expected active segment %q, got %q", "gti status", lex.ActiveSegment)
	}

	lex = LexCommandLine("gti 2>&1 && ls")
	if lex.ActiveSegment != "ls" {
		t.Fatalf("expected active segment %q, got %q", "ls", lex.ActiveSegment)
	}
}

func TestLexerUnicodeContinuationByte(t *testing.T) {
	input := "echo \u0585test"
	lex := LexCommandLine(input)
	if len(lex.Tokens) != 2 || lex.Tokens[1] != "\u0585test" {
		t.Fatalf("expected token %q, got %v", "\u0585test", lex.Tokens)
	}
}

func TestSubcommandExtraWhitespaceQuickFix(t *testing.T) {
	diag := Analyze("git   stauts")
	if diag == nil || diag.QuickFix != "git   status" {
		t.Fatalf("expected quick fix 'git   status', got %#v", diag)
	}
}

func TestAnalyzeCommandPrefixes(t *testing.T) {
	for _, input := range []string{
		"FOO=bar gti status",
		"time gti status",
		"env FOO=bar gti status",
		"nice -n 5 gti status",
		"command gti status",
	} {
		d := Analyze(input)
		if d == nil || d.Suggestion != "Did you mean 'git'?" {
			t.Fatalf("expected git suggestion for %q, got %+v", input, d)
		}
	}

	if d := Analyze("command -v gti"); d != nil {
		t.Fatalf("expected no diagnostic for 'command -v gti', got %+v", d)
	}

	if os.Geteuid() != 0 {
		d := Analyze("FOO=bar apt install neovim")
		if d == nil || d.Severity != SeverityWarning {
			t.Fatalf("expected root privilege warning for prefixed apt install, got %+v", d)
		}
		if d.QuickFix != "sudo apt install neovim" {
			t.Fatalf("expected QuickFix 'sudo apt install neovim', got %q", d.QuickFix)
		}
	}
}

func TestStripCommandPrefixes(t *testing.T) {
	eq := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	cases := []struct {
		in, want []string
	}{
		{[]string{"FOO=bar", "apt", "install"}, []string{"apt", "install"}},
		{[]string{"A=1", "B=2", "gti"}, []string{"gti"}},
		{[]string{"time", "-p", "gti"}, []string{"gti"}},
		{[]string{"nice", "-n", "5", "gti"}, []string{"gti"}},
		{[]string{"env", "-i", "FOO=bar", "gti"}, []string{"gti"}},
		{[]string{"env", "-u", "FOO", "gti"}, []string{"gti"}},
		{[]string{"command", "-v", "git"}, []string{"command", "-v", "git"}},
		{[]string{"sudo", "apt", "install"}, []string{"sudo", "apt", "install"}},
		{[]string{}, []string{}},
	}
	for _, c := range cases {
		if got := stripCommandPrefixes(c.in); !eq(got, c.want) {
			t.Fatalf("stripCommandPrefixes(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestIsSafeOperation(t *testing.T) {
	cases := []struct {
		cmd   string
		parts []string
		want  bool
	}{
		{"pacman", []string{"pacman", "-ss", "foo"}, true},
		{"pacman", []string{"pacman", "-SS", "foo"}, false},
		{"pacman", []string{"pacman", "-Ss", "foo"}, true},
		{"pacman", []string{"pacman", "--color", "auto", "-Ss", "foo"}, true},
		{"pacman", []string{"pacman", "-S", "foo"}, false},
		{"apt", []string{"apt", "-q", "search", "foo"}, true},
		{"apt", []string{"apt", "-o", "Debug::NoLocking=1", "search", "foo"}, true},
		{"apt", []string{"apt", "install", "foo"}, false},
		{"apt", []string{"apt", "-q", "install", "foo"}, false},
		{"apt", []string{"apt"}, false},
		{"apt", []string{"apt", "-c", "/etc/apt/apt.conf", "search", "foo"}, true},
	}
	for _, c := range cases {
		if got := isSafeOperation(c.cmd, c.parts); got != c.want {
			t.Fatalf("isSafeOperation(%q, %v) = %v, want %v", c.cmd, c.parts, got, c.want)
		}
	}
}

func TestAnalyzeSubcommandAfterGlobalFlags(t *testing.T) {
	d := Analyze("git -c core.pager=cat statu")
	if d == nil || d.Suggestion != "Did you mean 'status'?" {
		t.Fatalf("expected status suggestion for 'git -c ... statu', got %+v", d)
	}
}
