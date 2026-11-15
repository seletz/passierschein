package opguard

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// T29 (steps 1 and 2).
// F16, UC12: the samples in testdata have the shape of the PreToolUse input
// of Claude Code (testdata/SOURCE.txt names the page and the date).
func TestDecideSamples(t *testing.T) {
	tests := []struct {
		file string
		want Rule
	}{
		{"claude-code-bash-op.json", RuleOpCLI},
		{"claude-code-bash-op-in-substitution.json", RuleOpCLI},
		{"claude-code-bash-token-file.json", RuleTokenFile},
		{"claude-code-bash-allowed.json", ""},
		{"claude-code-bash-mentions-op.json", ""},
		{"claude-code-read-tool.json", ""},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", tt.file))
			if err != nil {
				t.Fatal(err)
			}
			d := Decide(bytes.NewReader(data), Options{})
			if d.Rule != tt.want {
				t.Errorf("Decide = %q, want %q", d.Rule, tt.want)
			}
			if d.Note != "" {
				t.Errorf("Note = %q, want none for a valid sample", d.Note)
			}
		})
	}
}

// T29 (steps 2 and 3): other tool, no JSON.
// D15 (fail-open): input that the guard cannot read or understand is
// allowed. The note never repeats the input.
func TestDecideFailOpen(t *testing.T) {
	big := `{"tool_name":"Bash","tool_input":{"command":"op whoami ` + strings.Repeat("x", MaxInput) + `"}}`
	tests := []struct {
		name     string
		input    string
		wantNote string
	}{
		{"empty", "", "not JSON"},
		{"not JSON", "op read op://v/i/f", "not JSON"},
		{"truncated JSON", `{"tool_name":"Bash","tool_input":{"command":"op whoami"`, "not JSON"},
		{"wrong type of tool_name", `{"tool_name":7,"tool_input":{"command":"op whoami"}}`, "unexpected shape"},
		{"command not a string", `{"tool_name":"Bash","tool_input":{"command":["op","whoami"]}}`, "no command"},
		{"no command", `{"tool_name":"Bash","tool_input":{}}`, "no command"},
		{"no tool_input", `{"tool_name":"Bash"}`, "no command"},
		{"other event", `{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"op whoami"}}`, "not a PreToolUse"},
		{"unparsable command", `{"tool_name":"Bash","tool_input":{"command":"op read 'op://v/i/f"}}`, "not parsable"},
		{"huge input", big, "larger than 1 MiB"},
		{"other tool", `{"tool_name":"Write","tool_input":{"file_path":"op","content":"op read op://v/i/f"}}`, ""},
		{"shell tool of another agent tool", `{"tool_name":"run_shell_command","tool_input":{"command":"op whoami"}}`, ""},
		{"JSON null", "null", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Decide(strings.NewReader(tt.input), Options{})
			if d.Blocked() {
				t.Fatalf("Decide blocks with %q, want allow", d.Rule)
			}
			if !strings.Contains(d.Note, tt.wantNote) || (tt.wantNote == "" && d.Note != "") {
				t.Errorf("Note = %q, want %q", d.Note, tt.wantNote)
			}
			if strings.Contains(d.Note, "op") && strings.Contains(d.Note, "whoami") {
				t.Errorf("Note repeats the input: %q", d.Note)
			}
		})
	}
}

// The guard reads at most MaxInput+1 bytes of a huge input.
func TestDecideReadsAtMostTheLimit(t *testing.T) {
	r := &countingReader{}
	if d := Decide(r, Options{}); d.Blocked() {
		t.Fatal("Decide blocks an endless input")
	}
	if r.n > MaxInput+1+32*1024 {
		t.Errorf("Decide read %d bytes, want at most about %d", r.n, MaxInput+1)
	}
}

// countingReader returns endless spaces and counts them.
type countingReader struct{ n int }

func (c *countingReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	c.n += len(p)
	return len(p), nil
}

// A read error allows.
func TestDecideReadError(t *testing.T) {
	d := Decide(errReader{}, Options{})
	if d.Blocked() || d.Note == "" {
		t.Errorf("Decide = %+v, want allow with a note", d)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("broken pipe") }

// T29 (step 1) and T40 (step 1): the message names both ways of a38
// and no other way.
// F24, D67: the block message names the MCP tools run_with_credentials and
// request_credentials of the server a38 and suggests no other way.
func TestMessage(t *testing.T) {
	for _, rule := range []Rule{RuleOpCLI, RuleOpDoubt, RuleTokenFile, RuleKeychainToken} {
		t.Run(string(rule), func(t *testing.T) {
			msg := Message(rule)
			for _, want := range []string{
				"Blocked by a38: do not ", "use the MCP tool\nrun_with_credentials of the server a38.",
				"For a dev credential\nvalue, you can also use request_credentials.",
			} {
				if !strings.Contains(msg, want) {
					t.Errorf("message lacks %q:\n%s", want, msg)
				}
			}
			lower := strings.ToLower(msg)
			for _, bad := range []string{
				"instead run", "try", "sudo", "env ", "bash -c", "export",
				"op read", "op item", "op inject", "op run", "op signin",
				"service_account", "token file", "keychain", "security ", "1password app",
				"--", "retry", "grant_status", "http",
			} {
				if strings.Contains(lower, bad) {
					t.Errorf("message contains %q, which hints at another way:\n%s", bad, msg)
				}
			}
			if strings.Count(msg, "\n") > 8 {
				t.Errorf("message has more than 8 lines:\n%s", msg)
			}
		})
	}
	const d67 = `Blocked by a38: do not run the 1Password CLI "op".
To run a command with a credential, use the MCP tool
run_with_credentials of the server a38. For a dev credential
value, you can also use request_credentials.
`
	for _, rule := range []Rule{RuleOpCLI, RuleOpDoubt} {
		if got := Message(rule); got != d67 {
			t.Errorf("Message(%q) =\n%s\nwant the text of D67:\n%s", rule, got, d67)
		}
	}
	if !strings.Contains(Message(RuleTokenFile), "token of a38") {
		t.Error("the message for the token file does not name the token")
	}
}

// Decide never panics, whatever the input (the hook must not break the
// agent tool).
func FuzzDecide(f *testing.F) {
	f.Add(`{"tool_name":"Bash","tool_input":{"command":"op whoami"}}`)
	f.Add(`{"tool_name":"Bash","tool_input":{"command":"bash -c \"eval 'op read x'\""}}`)
	f.Add(`{"tool_name":"Bash","tool_input":{"command":"echo $'\\x6f'p ${a:-op} $((1+2)) {a,b}"}}`)
	f.Add("not json")
	f.Fuzz(func(t *testing.T, in string) {
		_ = Decide(strings.NewReader(in), Options{})
		_ = Decide(strings.NewReader(in), Options{Strict: true})
	})
}

// Check never panics on a command.
func FuzzCheck(f *testing.F) {
	for _, s := range []string{"op", "sudo -u", "env -S", "xargs -n", "find -exec", "bash -c", "security find-generic-password -w", "timeout"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, cmd string) {
		_, _ = Check(cmd)
	})
}
