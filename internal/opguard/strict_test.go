package opguard

import (
	"encoding/json"
	"strings"
	"testing"
)

// bashHookInput returns a PreToolUse input of Claude Code for a Bash
// command.
func bashHookInput(t *testing.T, command string) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       ShellTool,
		"tool_input":      map[string]string{"command": command},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// deepOp returns "op whoami" in MaxDepth+1 nested "bash -c" strings.
func deepOp() string {
	cmd := "op whoami"
	for range MaxDepth + 1 {
		cmd = "bash -c " + quote(cmd)
	}
	return cmd
}

// T40: the guard with and without --strict. Without the switch it allows
// in doubt (D58, point 5). With --strict it blocks the three cases of
// doubt of D67 if the raw command has the word op. Input errors pass in
// both modes.
func TestDecideStrict(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantNormal Rule
		wantStrict Rule
	}{
		// Step 1.
		{"op read", bashHookInput(t, "op read op://beispiel.app.k8s-dev/datenbank-dev/password"), RuleOpCLI, RuleOpCLI},
		// Step 2: the parser does not understand the command.
		{"unparsable with op", bashHookInput(t, "op read 'op://beispiel.app.k8s-dev/datenbank-dev/password"), "", RuleOpDoubt},
		// Step 3: a command word is built at run time.
		{"runtime word with op", bashHookInput(t, "TOOL=op; $TOOL read op://beispiel.app.k8s-dev/datenbank-dev/password"), "", RuleOpDoubt},
		{"shell string from variable", bashHookInput(t, `CMD="op whoami"; bash -c "$CMD"`), "", RuleOpDoubt},
		{"eval of variable", bashHookInput(t, `CMD="op whoami"; eval "$CMD"`), "", RuleOpDoubt},
		// Third case of doubt: the nesting is deeper than MaxDepth.
		{"too deep with op", bashHookInput(t, deepOp()), "", RuleOpDoubt},
		// Step 4: a case of doubt without op.
		{"runtime word without op", bashHookInput(t, "X=1; $X --version"), "", ""},
		{"unparsable without op", bashHookInput(t, "echo 'unterminated"), "", ""},
		{"op only as part of a word", bashHookInput(t, "$TOOL --config ./op.yaml --opt top"), "", ""},
		{"op only in another word", bashHookInput(t, "$TOOL beispielprojekt-op op_x"), "", ""},
		// Step 5 and the other input errors of D67.
		{"not JSON", "op read op://beispiel.app.k8s-dev/datenbank-dev/password", "", ""},
		{"larger than 1 MiB", bashHookInput(t, "$TOOL op "+strings.Repeat("x", MaxInput)), "", ""},
		{"no command", `{"tool_name":"Bash","tool_input":{}}`, "", ""},
		// No doubt: the parser understood the command and found no op.
		{"op as argument", bashHookInput(t, `git commit -m "fix op"`), "", ""},
		{"other tool", `{"tool_name":"Write","tool_input":{"file_path":"op","content":"$X op"}}`, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Decide(strings.NewReader(tt.input), Options{}); got.Rule != tt.wantNormal {
				t.Errorf("without --strict: Decide = %+v, want rule %q", got, tt.wantNormal)
			}
			got := Decide(strings.NewReader(tt.input), Options{Strict: true})
			if got.Rule != tt.wantStrict {
				t.Errorf("with --strict: Decide = %+v, want rule %q", got, tt.wantStrict)
			}
			if strings.Contains(got.Note, "op ") || strings.Contains(got.Note, "$") {
				t.Errorf("Note repeats the input: %q", got.Note)
			}
		})
	}
}

// T40 (part): without --strict, a case of doubt allows with a note that
// names the reason and no part of the input (D58, point 5).
func TestDecideDoubtNote(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		wantNote string
	}{
		{"unparsable", "op read 'x", "not parsable"},
		{"runtime word", "$TOOL read x", "built at run time"},
		{"shell string from variable", `bash -c "$CMD"`, "built at run time"},
		{"too deep", deepOp(), "deeper than 3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Decide(strings.NewReader(bashHookInput(t, tt.command)), Options{})
			if d.Blocked() {
				t.Fatalf("Decide blocks with %q, want allow", d.Rule)
			}
			if !strings.Contains(d.Note, tt.wantNote) {
				t.Errorf("Note = %q, want %q", d.Note, tt.wantNote)
			}
		})
	}
}

// T40 (part): the word pattern of D67,
// (^|[^A-Za-z0-9_.-])op([^A-Za-z0-9_.-]|$).
func TestMentionsOp(t *testing.T) {
	tests := []struct {
		command string
		want    bool
	}{
		{"op", true},
		{"op read x", true},
		{"TOOL=op; $TOOL", true},
		{"$X 'op'", true},
		{"$X op://beispiel.app.k8s-dev/datenbank-dev/password", true},
		{"/usr/local/bin/op", true},
		{"$(op)", true},
		{"a\nop\n", true},
		{"top", false},
		{"opam", false},
		{"./op.txt", false},
		{"op-check", false},
		{"my_op", false},
		{"x.op", false},
		{"op1", false},
		{"OP", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := MentionsOp(tt.command); got != tt.want {
			t.Errorf("MentionsOp(%q) = %v, want %v", tt.command, got, tt.want)
		}
	}
}

// T40 (part): Inspect names the case of doubt, and a rule wins over a
// doubt.
func TestInspectDoubt(t *testing.T) {
	tests := []struct {
		command   string
		wantRule  Rule
		wantDoubt Doubt
	}{
		{"ls -l", "", ""},
		{"op whoami", RuleOpCLI, ""},
		{"echo 'x", "", DoubtUnparsable},
		{"$TOOL x", "", DoubtRuntimeWord},
		{"sudo $TOOL x", "", DoubtRuntimeWord},
		{`bash -c "$CMD"`, "", DoubtRuntimeWord},
		{"eval $CMD", "", DoubtRuntimeWord},
		{"bash -c 'echo \"x'", "", DoubtUnparsable},
		{deepOp(), "", DoubtTooDeep},
		{"$TOOL x; op whoami", RuleOpCLI, ""},
	}
	for _, tt := range tests {
		rule, doubt := Inspect(tt.command)
		if rule != tt.wantRule || doubt != tt.wantDoubt {
			t.Errorf("Inspect(%q) = %q, %q; want %q, %q", tt.command, rule, doubt, tt.wantRule, tt.wantDoubt)
		}
	}
}
