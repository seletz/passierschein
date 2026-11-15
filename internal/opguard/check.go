// Package opguard decides whether a shell command of an agent runs the
// 1Password CLI "op" or reads the 1Password token of a38 (F16, UC12, D15).
//
// The package is a hint for a well-behaved agent, not a protection: an
// agent that runs as the same user can read a secret in many other ways
// (RI5). The guard therefore allows every command that it cannot parse or
// cannot understand. It never runs the command that it inspects.
//
// The package imports only the standard library and the shell parser
// mvdan.cc/sh/v3/syntax. It has no build constraint and is part of the
// linux build of a38.
package opguard

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// Rule names the reason for a block. The empty rule means "allow".
type Rule string

// The rules of the guard.
const (
	// RuleOpCLI: the command runs the 1Password CLI "op" (D15).
	RuleOpCLI Rule = "op-cli"
	// RuleTokenFile: the command reads a token file of a38
	// (".../onepassword-token", D51 file source, or
	// ".../onepassword-token-NAME", the file of an account, D72).
	RuleTokenFile Rule = "a38-token-file"
	// RuleKeychainToken: the command prints a keychain item of a38 with
	// "security find-generic-password -w" or "-g" (D51, D72).
	RuleKeychainToken Rule = "a38-keychain-token"
)

// MaxDepth is the number of nested shell strings ("bash -c", "sh -c",
// "eval", "env -S") that the guard inspects. A deeper string is allowed.
const MaxDepth = 3

// Names of the a38 token in the keychain and on disk. The keychain values
// must match internal/keychain (checked by a darwin test in cmd/a38); this
// package does not import internal/keychain, because that package is not
// part of the linux build.
const (
	keychainService = "a38.onepassword"
	keychainLabel   = "a38: 1Password Service Account"
	tokenFileName   = "onepassword-token"
)

// hole stands for a part of a word that is not literal, for example "$HOME".
const hole = "\x00"

// Check inspects a shell command. It returns the first rule that fires, or
// the empty rule. parsed is false when no shell parser understood the
// command; the rule is then empty (fail-open, D15).
func Check(command string) (rule Rule, parsed bool) {
	return inspect(command, 0)
}

// inspect parses src as bash, then as zsh, and walks the syntax tree.
func inspect(src string, depth int) (Rule, bool) {
	file := parse(src)
	if file == nil {
		return "", false
	}
	var found Rule
	syntax.Walk(file, func(node syntax.Node) bool {
		if found != "" {
			return false
		}
		switch n := node.(type) {
		case *syntax.CallExpr:
			found = checkCall(words(n.Args), depth)
		case *syntax.Redirect:
			if (n.Op == syntax.RdrIn || n.Op == syntax.RdrInOut) && n.Word != nil && isTokenPath(wordText(n.Word)) {
				found = RuleTokenFile
			}
		}
		return found == ""
	})
	return found, true
}

// parse returns the syntax tree of src, or nil.
func parse(src string) *syntax.File {
	for _, lang := range []syntax.LangVariant{syntax.LangBash, syntax.LangZsh} {
		f, err := syntax.NewParser(syntax.Variant(lang)).Parse(strings.NewReader(src), "")
		if err == nil {
			return f
		}
	}
	return nil
}

// words returns the best-effort text of each word.
func words(ws []*syntax.Word) []string {
	out := make([]string, len(ws))
	for i, w := range ws {
		out[i] = wordText(w)
	}
	return out
}

// literal reports whether a word text has no non-literal part.
func literal(s string) bool {
	return !strings.Contains(s, hole)
}

// commandName returns the last path element of a command word, or "" when
// that element is not literal.
func commandName(word string) string {
	name := word
	if i := strings.LastIndex(word, "/"); i >= 0 {
		name = word[i+1:]
	}
	if !literal(name) {
		return ""
	}
	return name
}

// checkCall inspects one simple command: the words after the assignments.
func checkCall(args []string, depth int) Rule {
	for len(args) > 0 {
		name := commandName(args[0])
		rest := args[1:]
		switch name {
		case "op":
			return RuleOpCLI
		case "sudo", "doas":
			args = skipOptions(rest, "ughpCDrtUT")
			args = skipAssignments(args)
		case "env":
			var split string
			args, split = skipEnvOptions(rest)
			if split != "" {
				if r := inspectString(split, depth); r != "" {
					return r
				}
			}
			args = skipAssignments(args)
		case "command":
			for len(rest) > 0 && strings.HasPrefix(rest[0], "-") && rest[0] != "--" {
				if strings.ContainsAny(rest[0], "vV") {
					return "" // a lookup: "command -v op" runs nothing
				}
				rest = rest[1:]
			}
			args = dropDashDash(rest)
		case "exec":
			args = skipOptions(rest, "a")
		case "nohup", "builtin":
			args = dropDashDash(rest)
		case "time":
			args = skipOptions(rest, "of")
		case "nice":
			args = skipOptions(rest, "n")
		case "timeout":
			args = skipOptions(rest, "sk")
			if len(args) > 0 {
				args = args[1:] // the duration
			}
		case "xargs":
			args = skipOptions(rest, "IJLnPsEdaRS")
		case "find":
			return checkFind(rest, depth)
		case "eval":
			return inspectString(strings.Join(rest, " "), depth)
		case "sh", "bash", "zsh", "dash", "ksh", "mksh":
			return checkShell(rest, depth)
		case "security":
			return checkSecurity(rest)
		default:
			return checkTokenArgs(name, rest)
		}
	}
	return ""
}

// inspectString inspects a nested shell string. A string with a non-literal
// part or a string deeper than MaxDepth is allowed.
func inspectString(s string, depth int) Rule {
	if depth >= MaxDepth || !literal(s) {
		return ""
	}
	r, _ := inspect(s, depth+1)
	return r
}

// skipOptions drops the options in front of the command word. withValue
// lists the single-letter options that take the next word as value.
func skipOptions(args []string, withValue string) []string {
	for len(args) > 0 {
		a := args[0]
		if a == "--" {
			return args[1:]
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			return args
		}
		args = args[1:]
		if strings.HasPrefix(a, "--") {
			continue // a long option; "--name=value" carries its value
		}
		if len(a) == 2 && strings.ContainsRune(withValue, rune(a[1])) && len(args) > 0 {
			args = args[1:]
		}
	}
	return args
}

// skipAssignments drops the words of the form NAME=VALUE.
func skipAssignments(args []string) []string {
	for len(args) > 0 && isAssignment(args[0]) {
		args = args[1:]
	}
	return args
}

// isAssignment reports whether a word has the form NAME=VALUE.
func isAssignment(s string) bool {
	i := strings.Index(s, "=")
	if i <= 0 {
		return false
	}
	for j, r := range s[:i] {
		if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (j == 0 || r < '0' || r > '9') {
			return false
		}
	}
	return true
}

// dropDashDash drops a leading "--".
func dropDashDash(args []string) []string {
	if len(args) > 0 && args[0] == "--" {
		return args[1:]
	}
	return args
}

// skipEnvOptions drops the options of env and returns the string of
// "-S"/"--split-string", if any.
func skipEnvOptions(args []string) (rest []string, split string) {
	for len(args) > 0 {
		a := args[0]
		switch {
		case a == "--":
			return args[1:], split
		case a == "-S" || a == "--split-string":
			if len(args) > 1 {
				split = args[1]
				args = args[2:]
				continue
			}
			return nil, split
		case strings.HasPrefix(a, "--split-string="):
			split = strings.TrimPrefix(a, "--split-string=")
		case a == "-u" || a == "-C" || a == "-P" || a == "--unset" || a == "--chdir":
			if len(args) > 1 {
				args = args[1:]
			}
		case strings.HasPrefix(a, "-") && a != "-":
		default:
			return args, split
		}
		args = args[1:]
	}
	return args, split
}

// checkFind inspects the commands of "-exec", "-execdir", "-ok" and "-okdir".
func checkFind(args []string, depth int) Rule {
	for i, a := range args {
		switch a {
		case "-exec", "-execdir", "-ok", "-okdir":
			if r := checkCall(args[i+1:], depth); r != "" {
				return r
			}
		}
	}
	return ""
}

// checkShell inspects the string of "-c". The first word after the
// options is the string; "-o" and "-O" take a value.
func checkShell(args []string, depth int) Rule {
	sawC := false
	for len(args) > 0 {
		a := args[0]
		if a == "--" {
			args = args[1:]
			break
		}
		if (!strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "+")) || a == "-" {
			break
		}
		args = args[1:]
		if strings.HasPrefix(a, "--") {
			continue
		}
		if strings.ContainsRune(a, 'c') {
			sawC = true
		}
		if (a == "-o" || a == "+o" || a == "-O" || a == "+O") && len(args) > 0 {
			args = args[1:]
		}
	}
	if !sawC || len(args) == 0 {
		return "" // a script file or standard input: not inspected
	}
	return inspectString(args[0], depth)
}

// checkSecurity fires for "security find-generic-password" with "-w" or
// "-g" (print the password) and the service or the label of the a38 item.
func checkSecurity(args []string) Rule {
	if len(args) == 0 || args[0] != "find-generic-password" {
		return ""
	}
	prints, a38Item := false, false
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsAny(a[1:], "wg") {
			prints = true
		}
		if strings.Contains(a, keychainService) || strings.HasPrefix(a, keychainLabel) {
			a38Item = true
		}
	}
	if prints && a38Item {
		return RuleKeychainToken
	}
	return ""
}

// metadataOnly lists the commands that name the token file without reading
// it.
var metadataOnly = map[string]bool{
	"a38": true, "ls": true, "stat": true, "test": true, "[": true, "echo": true, "printf": true,
}

// checkTokenArgs fires when a command names the token file of a38 as an
// argument. The command word itself is not literal ("") counts as reading.
func checkTokenArgs(name string, args []string) Rule {
	if metadataOnly[name] {
		return ""
	}
	for _, a := range args {
		if isTokenPath(a) {
			return RuleTokenFile
		}
	}
	return ""
}

// isTokenPath reports whether a word is a path that ends with
// "/onepassword-token" or with "/onepassword-token-NAME", the token file of
// an account (D72, issue 88).
func isTokenPath(s string) bool {
	if strings.HasSuffix(s, "/"+tokenFileName) {
		return true
	}
	i := strings.LastIndex(s, "/"+tokenFileName+"-")
	if i < 0 {
		return false
	}
	name := s[i+len(tokenFileName)+2:]
	return name != "" && !strings.ContainsAny(name, "/")
}
