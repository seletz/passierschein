package skill

import (
	"strings"
	"testing"

	"github.com/seletz/passierschein/internal/mcp"
	"github.com/seletz/passierschein/internal/protocol"
)

func TestFrontMatter(t *testing.T) {
	for _, want := range []string{"---\nname: a38\n", "\ndescription: "} {
		if !strings.Contains(Text(), want) {
			t.Errorf("the front matter lacks %q", want)
		}
	}
	if !strings.HasPrefix(Text(), "---\n") {
		t.Error("the text does not start with a front matter")
	}
}

// T20 (part) and T41 (part): the text names every error code of the
// protocol and every MCP tool.
func TestTextNamesCodesAndTools(t *testing.T) {
	for _, c := range protocol.Codes() {
		if !strings.Contains(Text(), "`"+string(c)+"`") {
			t.Errorf("the skill does not name the error code %q", c)
		}
	}
	for _, tool := range []string{string(protocol.TypeRequestCredentials), string(protocol.TypeGrantStatus), mcp.ToolRunWithCredentials} {
		if !strings.Contains(Text(), "`"+string(tool)+"`") {
			t.Errorf("the skill does not name the tool %q", tool)
		}
	}
	for _, want := range []string{"op://", "references", "task", "reason", "`op`"} {
		if !strings.Contains(Text(), want) {
			t.Errorf("the skill lacks %q", want)
		}
	}
}

// T41: the skill names run_with_credentials with all parameters, the codes
// exec_required and exec_failed with their rule, the rule "never put a
// value into the arguments", the limit of the masking and the rule for
// grants per environment (D68). The rule "delivers only credentials of dev
// vaults" is gone. T20 checks that each code of protocol.Codes() is in the
// skill.
func TestTextExecWay(t *testing.T) {
	text := Text()
	for _, want := range []string{
		// The new rules.
		"- Use `run_with_credentials` when a command needs a credential. You do not see the value.\n",
		"- For a test or prod vault, `run_with_credentials` is the only way. `request_credentials` returns `exec_required`.\n",
		"- For a dev vault, prefer `run_with_credentials`. Use `request_credentials` only if no single command can use the value.\n",
		"- Never put a value into the arguments of a command. Give it through `env` only.\n",
		"- Never try to print, encode or copy a value out of a command. The output is masked, but only the exact value.\n",
		// The section of the tool.
		"## Tool `run_with_credentials`\n",
		"- `command`: the program, for example `psql`. Required. Not a shell text.\n",
		"- `args`: the arguments, a list of strings.\n",
		"- `env`: a map from the name of an environment variable to a reference. Required.\n",
		"- `cwd`: the working directory. It must be in your project. Default: your working directory.\n",
		"- `timeout_seconds`: the time limit. Default 120.\n",
		"- `task` and `reason`: as for `request_credentials`. Give them.\n",
		"The human sees the full command. Use a short and clear command.\n",
		"The result has `exit_code`, `timed_out`, `stdout`, `stderr`, the flags `stdout_truncated` and `stderr_truncated` and `masked`.\n",
		"A non-zero exit code is a result, not an a38 error. Read the output and continue your work.\n",
		"`[masked:NAME]` in the output replaces the value of the variable NAME.\n",
		"stdin is empty. Interactive commands do not work.\n",
		"The output keeps the last 64 KiB of each stream.\n",
		// The grants.
		"A grant lasts as long as the human allows. In dev it can last some hours. In test it is short and is valid only for the same program.\n",
		"In prod there is no grant: each command needs a new approval.\n",
		// The codes.
		"- `exec_required`: this vault gives values only through `run_with_credentials`. Send the same task once with `run_with_credentials`. Do not use another way.\n",
		"- `exec_failed`: a38 could not start the command after delivery. Stop and tell the human.\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the skill lacks %q", want)
		}
	}
	for _, gone := range []string{
		"delivers only credentials of dev vaults",
		"Pass a value only to the program that needs it",
		"It is valid for a short time",
		"not a dev vault",
		"If `a38 mcp` has no such tool",
	} {
		if strings.Contains(text, gone) {
			t.Errorf("the skill still has the replaced text %q", gone)
		}
	}
}
