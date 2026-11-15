package opguard

import (
	"encoding/json"
	"errors"
	"io"
)

// MaxInput is the largest hook input that the guard reads (1 MiB). A
// larger input is allowed without inspection.
const MaxInput = 1 << 20

// ShellTool is the only tool name that the guard inspects: the Bash tool of
// Claude Code. Every other tool passes.
const ShellTool = "Bash"

// Decision is the result for one hook input.
type Decision struct {
	// Rule is the rule that fired; empty means "allow".
	Rule Rule
	// Note says why the guard allowed without inspection, for example
	// "input is not JSON". It never contains a part of the input.
	Note string
}

// Blocked reports whether the guard blocks the command.
func (d Decision) Blocked() bool { return d.Rule != "" }

// input is the part of the hook input that the guard reads. Unknown fields
// are ignored, because the agent tool adds fields over time.
type input struct {
	HookEventName string          `json:"hook_event_name"`
	ToolName      string          `json:"tool_name"`
	ToolInput     json.RawMessage `json:"tool_input"`
}

// bashInput is the input of the Bash tool.
type bashInput struct {
	Command *string `json:"command"`
}

// Options change the decision of the guard.
type Options struct {
	// Strict blocks a case of doubt if the raw command has the word op
	// (D67, point 3). An input error still passes.
	Strict bool
}

// Decide reads one hook input (JSON) from r and decides. It reads at most
// MaxInput+1 bytes. Each input that it cannot read or understand is
// allowed (fail-open, D15); Note then names the reason. A command that the
// guard cannot inspect fully (a Doubt) is allowed too, except with
// opts.Strict when the raw command has the word op (RuleOpDoubt).
func Decide(r io.Reader, opts Options) Decision {
	data, err := io.ReadAll(io.LimitReader(r, MaxInput+1))
	if err != nil {
		return Decision{Note: "cannot read the input"}
	}
	if len(data) > MaxInput {
		return Decision{Note: "input is larger than 1 MiB; not inspected"}
	}
	var in input
	if err := json.Unmarshal(data, &in); err != nil {
		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) {
			return Decision{Note: "input is not JSON"}
		}
		return Decision{Note: "input has an unexpected shape"}
	}
	if in.HookEventName != "" && in.HookEventName != "PreToolUse" {
		return Decision{Note: "not a PreToolUse event"}
	}
	if in.ToolName != ShellTool {
		return Decision{}
	}
	var bash bashInput
	if err := json.Unmarshal(in.ToolInput, &bash); err != nil || bash.Command == nil {
		return Decision{Note: "Bash input has no command"}
	}
	rule, doubt := Inspect(*bash.Command)
	switch {
	case rule != "":
		return Decision{Rule: rule}
	case doubt == "":
		return Decision{}
	case opts.Strict && MentionsOp(*bash.Command):
		return Decision{Rule: RuleOpDoubt}
	}
	return Decision{Note: string(doubt) + "; not inspected"}
}
