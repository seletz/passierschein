package opguard

import "encoding/json"

// HookTimeout is the timeout of the hook in seconds in the printed
// configuration. The guard needs milliseconds; a timeout lets the command
// through (fail-open).
const HookTimeout = 10

// ClaudeCodeSettings returns the "hooks" block for a settings.json file of
// Claude Code: a PreToolUse hook on the Bash tool that runs
// "<executable> hook op-guard" in exec form (no shell).
//
// Reference: https://code.claude.com/docs/en/hooks (configuration,
// matcher, exec form with "args", "timeout"), read on 2026-11-12.
func ClaudeCodeSettings(executable string) ([]byte, error) {
	type handler struct {
		Type    string   `json:"type"`
		Command string   `json:"command"`
		Args    []string `json:"args"`
		Timeout int      `json:"timeout"`
	}
	type group struct {
		Matcher string    `json:"matcher"`
		Hooks   []handler `json:"hooks"`
	}
	settings := map[string]map[string][]group{
		"hooks": {
			"PreToolUse": {{
				Matcher: ShellTool,
				Hooks: []handler{{
					Type:    "command",
					Command: executable,
					Args:    []string{"hook", "op-guard"},
					Timeout: HookTimeout,
				}},
			}},
		},
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
