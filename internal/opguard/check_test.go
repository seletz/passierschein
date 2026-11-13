package opguard

import (
	"strings"
	"testing"
)

// T29 (steps 1 and 2).
// F16, UC12, D15: the detection table. A positive case names the rule that
// must fire; a negative case has the empty rule. No test runs a command.
func TestCheck(t *testing.T) {
	const token = "~/.config/a38/onepassword-token"
	tests := []struct {
		name    string
		command string
		want    Rule
	}{
		// The command word op.
		{"bare op", "op", RuleOpCLI},
		{"op read", "op read op://beispiel.app.k8s-dev/datenbank-dev/password", RuleOpCLI},
		{"op version", "op --version", RuleOpCLI},
		{"op item get", "op item get datenbank-dev --vault beispiel.app.k8s-dev --fields password", RuleOpCLI},
		{"leading spaces", "   op whoami", RuleOpCLI},
		{"absolute path", "/opt/homebrew/bin/op read op://v/i/f", RuleOpCLI},
		{"relative path", "./op read op://v/i/f", RuleOpCLI},
		{"home path", "~/bin/op whoami", RuleOpCLI},
		{"variable dir path", "$HOME/bin/op whoami", RuleOpCLI},
		{"assignment prefix", "OP_ACCOUNT=beispiel op read op://v/i/f", RuleOpCLI},
		// Quoting and escapes of the command word.
		{"double quoted word", `"op" read op://v/i/f`, RuleOpCLI},
		{"single quoted word", `'op' read op://v/i/f`, RuleOpCLI},
		{"split quotes", `o'p' read op://v/i/f`, RuleOpCLI},
		{"backslash in word", `o\p read op://v/i/f`, RuleOpCLI},
		{"backslash before word", `\op read op://v/i/f`, RuleOpCLI},
		{"ansi c quoted word", `$'op' read op://v/i/f`, RuleOpCLI},
		{"line continuation", "op \\\n  read op://v/i/f", RuleOpCLI},
		// Wrappers.
		{"sudo", "sudo op read op://v/i/f", RuleOpCLI},
		{"sudo with user", "sudo -u beispiel op read op://v/i/f", RuleOpCLI},
		{"sudo with flags and env", "sudo -E -H X=1 op whoami", RuleOpCLI},
		{"env", "env op read op://v/i/f", RuleOpCLI},
		{"env with variables", "env A=1 B=2 op read op://v/i/f", RuleOpCLI},
		{"env with options", "env -i -u HOME PATH=/usr/bin op whoami", RuleOpCLI},
		{"env split string", "env -S 'op read op://v/i/f'", RuleOpCLI},
		{"command", "command op whoami", RuleOpCLI},
		{"command -p", "command -p op whoami", RuleOpCLI},
		{"exec", "exec op whoami", RuleOpCLI},
		{"exec -a", "exec -a name op whoami", RuleOpCLI},
		{"nohup background", "nohup op signin &", RuleOpCLI},
		{"time keyword", "time op whoami", RuleOpCLI},
		{"time program", "/usr/bin/time -p op whoami", RuleOpCLI},
		{"nice", "nice -n 10 op whoami", RuleOpCLI},
		{"timeout", "timeout -s KILL 5 op whoami", RuleOpCLI},
		{"chained wrappers", "sudo env A=1 nohup op whoami", RuleOpCLI},
		{"xargs", "echo op://v/i/f | xargs op read", RuleOpCLI},
		{"xargs with options", "printf '%s\\n' a | xargs -n 1 -I{} op read {}", RuleOpCLI},
		{"xargs attached option", "xargs -n1 op read", RuleOpCLI},
		{"find exec", `find . -name '*.ref' -exec op inject -i {} \;`, RuleOpCLI},
		// Lists, pipelines and compound commands.
		{"pipeline end", "echo x | op read op://v/i/f", RuleOpCLI},
		{"pipeline start", "op read op://v/i/f | pbcopy", RuleOpCLI},
		{"and list", "cd /tmp && op whoami", RuleOpCLI},
		{"or list", "false || op whoami", RuleOpCLI},
		{"semicolon", "ls; op whoami", RuleOpCLI},
		{"newline", "ls\nop whoami", RuleOpCLI},
		{"subshell", "( op whoami )", RuleOpCLI},
		{"block", "{ op whoami; }", RuleOpCLI},
		{"if condition", "if op whoami; then echo ok; fi", RuleOpCLI},
		{"for body", "for v in a b; do op read op://$v/i/f; done", RuleOpCLI},
		{"function body", "f() { op whoami; }; f", RuleOpCLI},
		{"negation", "! op whoami", RuleOpCLI},
		// Substitutions.
		{"command substitution", "TOKEN=$(op read op://v/i/f)", RuleOpCLI},
		{"export with substitution", "export TOKEN=$(op read op://v/i/f)", RuleOpCLI},
		{"substitution in quotes", `echo "$(op read op://v/i/f)"`, RuleOpCLI},
		{"backticks", "echo `op read op://v/i/f`", RuleOpCLI},
		{"process substitution", "cat <(op read op://v/i/f)", RuleOpCLI},
		{"substitution in argument", "curl -H \"Authorization: $(op read op://v/i/f)\" https://beispiel.invalid", RuleOpCLI},
		// Shell strings and eval.
		{"bash -c", "bash -c 'op read op://v/i/f'", RuleOpCLI},
		{"sh -c", `sh -c "op whoami"`, RuleOpCLI},
		{"zsh -c", "zsh -c 'op whoami'", RuleOpCLI},
		{"bash -lc", "bash -lc 'cd /tmp && op whoami'", RuleOpCLI},
		{"bash path -c", "/bin/bash --norc -c 'op whoami' arg0", RuleOpCLI},
		{"sudo sh -c", "sudo sh -c 'op whoami'", RuleOpCLI},
		{"nested shell strings", `bash -c "sh -c 'op whoami'"`, RuleOpCLI},
		{"eval", `eval "op read op://v/i/f"`, RuleOpCLI},
		{"eval words", "eval op whoami", RuleOpCLI},
		{"zsh syntax", "echo ${(U)name}; op whoami", RuleOpCLI},

		// Not op.
		{"top", "top -l 1", ""},
		{"cp", "cp a b", ""},
		{"opam", "opam install dune", ""},
		{"open", "open .", ""},
		{"option --op", "tool --op read", ""},
		{"op.txt as file", "cat op.txt", ""},
		{"op.txt as command", "./op.txt", ""},
		{"echo op", "echo op", ""},
		{"echo op read", "echo op read op://v/i/f", ""},
		{"commit message", `git commit -m "fix op"`, ""},
		{"commit message with op read", `git commit -m "op read is forbidden"`, ""},
		{"grep op", "grep op file", ""},
		{"grep op read", `grep -rn "op read" .`, ""},
		{"reference as argument", "echo op://beispiel.app.k8s-dev/datenbank-dev/password", ""},
		{"reference to other program", "./deploy.sh --secret op://beispiel.app.k8s-dev/datenbank-dev/password", ""},
		{"comment", "ls # op read op://v/i/f", ""},
		{"comment line", "# op whoami\nls", ""},
		{"op as argument", "ls op", ""},
		{"op directory", "mkdir -p op && cd op", ""},
		{"op-like script", "./scripts/op-check.sh", ""},
		{"opx", "opx run", ""},
		{"which op", "which op", ""},
		{"command -v op", "command -v op", ""},
		{"command -V op", "command -V op", ""},
		{"type op", "type op", ""},
		{"brew install", "brew install 1password-cli", ""},
		{"man op", "man op", ""},
		{"printf string", `printf '%s\n' "op read op://v/i/f"`, ""},
		{"heredoc data", "cat <<EOF\nop read op://v/i/f\nEOF", ""},
		{"bash -c echo", "bash -c 'echo op'", ""},
		{"eval echo", `eval "echo op"`, ""},
		{"bash script file", "bash ./op-setup.sh", ""},
		{"xargs without command", "echo op | xargs", ""},
		{"variable command word", "$TOOL read op://v/i/f", ""},
		{"bash -c with variable", `bash -c "$CMD"`, ""},
		{"empty", "", ""},
		{"blank", "   \n  ", ""},
		{"unparsable", "echo 'unterminated", ""},
		{"unparsable op", "op read 'unterminated", ""},

		// Rule a38-token-file.
		{"cat token file", "cat " + token, RuleTokenFile},
		{"redirect token file", "base64 < " + token, RuleTokenFile},
		{"copy token file", "cp " + token + " /tmp/x", RuleTokenFile},
		{"token file in variable path", `cat "$XDG_CONFIG_HOME/a38/onepassword-token"`, RuleTokenFile},
		{"token file in substitution", "export OP_SERVICE_ACCOUNT_TOKEN=$(cat " + token + ")", RuleTokenFile},
		{"token file through sudo", "sudo head -c 10 " + token, RuleTokenFile},
		{"token file other path", "cat /srv/a38/onepassword-token", RuleTokenFile},
		{"ls token file", "ls -l " + token, ""},
		{"stat token file", "stat -f '%Lp' " + token, ""},
		{"test token file", "test -f " + token + " && echo yes", ""},
		{"a38 token status", "a38 config onepassword-token status", ""},
		{"word onepassword-token", "echo onepassword-token", ""},

		// Rule a38-keychain-token.
		{"security -w a38", "security find-generic-password -s a38.onepassword -w", RuleKeychainToken},
		{"security -w first", "security find-generic-password -w -s a38.onepassword -a service-account", RuleKeychainToken},
		{"security combined flags", "security find-generic-password -ws a38.onepassword", RuleKeychainToken},
		{"security -g", "security find-generic-password -g -s a38.onepassword", RuleKeychainToken},
		{"security by label", `security find-generic-password -w -l "a38: 1Password Service Account"`, RuleKeychainToken},
		{"security in substitution", "T=$(security find-generic-password -s a38.onepassword -w)", RuleKeychainToken},
		{"security metadata only", "security find-generic-password -s a38.onepassword", ""},
		{"security other item", "security find-generic-password -s beispiel-dienst -w", ""},
		{"security other command", "security list-keychains", ""},
	}
	positive, negative := 0, 0
	for _, tt := range tests {
		if tt.want == "" {
			negative++
		} else {
			positive++
		}
		t.Run(tt.name, func(t *testing.T) {
			got, _ := Check(tt.command)
			if got != tt.want {
				t.Errorf("Check(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
	if positive < 30 || negative < 30 {
		t.Errorf("table has %d positive and %d negative cases, want at least 30 each", positive, negative)
	}
}

// Shell strings nest up to MaxDepth levels. A deeper string is not
// inspected: the guard allows it (D15: a hint, not a protection).
func TestCheckDepth(t *testing.T) {
	cmd := "op whoami"
	for range MaxDepth {
		cmd = "bash -c " + quote(cmd)
	}
	if got, _ := Check(cmd); got != RuleOpCLI {
		t.Errorf("depth %d: Check = %q, want %q", MaxDepth, got, RuleOpCLI)
	}
	cmd = "bash -c " + quote(cmd)
	if got, _ := Check(cmd); got != "" {
		t.Errorf("depth %d: Check = %q, want no rule", MaxDepth+1, got)
	}
}

// quote returns s in single quotes for the shell.
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// The second result tells whether the parser understood the command. The
// hook allows in both cases; the flag is only for the tests and the docs.
func TestCheckParsed(t *testing.T) {
	if _, ok := Check("echo 'unterminated"); ok {
		t.Error("Check reports an unparsable command as parsed")
	}
	if _, ok := Check("ls -l"); !ok {
		t.Error("Check reports a simple command as unparsed")
	}
}
