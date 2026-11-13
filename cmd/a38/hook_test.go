package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/opguard"
)

// sample reads a recorded hook input of Claude Code.
func sample(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "internal", "opguard", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// T29 (steps 1 to 3): exit codes and streams.
// F16, UC12: the stdin, stdout and exit code contract of
// "a38 hook op-guard". A block is exit 2 with the message on stderr and
// nothing on stdout; an allow is exit 0 without any output.
func TestHookOpGuardContract(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantCode   int
		wantStderr string
	}{
		{"block op", []string{"hook", "op-guard"}, sample(t, "claude-code-bash-op.json"), 2, `"request_credentials"`},
		{"block token file", []string{"hook", "op-guard"}, sample(t, "claude-code-bash-token-file.json"), 2, "token of a38"},
		{"allow", []string{"hook", "op-guard"}, sample(t, "claude-code-bash-allowed.json"), 0, ""},
		{"allow text with op", []string{"hook", "op-guard"}, sample(t, "claude-code-bash-mentions-op.json"), 0, ""},
		{"allow other tool", []string{"hook", "op-guard"}, sample(t, "claude-code-read-tool.json"), 0, ""},
		{"allow invalid JSON", []string{"hook", "op-guard"}, "op read op://v/i/f", 0, "allowed without inspection: input is not JSON"},
		{"allow empty input", []string{"hook", "op-guard"}, "", 0, "allowed without inspection"},
		{"allow huge input", []string{"hook", "op-guard"}, `{"tool_name":"Bash","tool_input":{"command":"op ` + strings.Repeat("x", opguard.MaxInput) + `"}}`, 0, "larger than 1 MiB"},
		{"usage: no hook", []string{"hook"}, "", 1, "a38 hook op-guard"},
		{"usage: unknown hook", []string{"hook", "rm-guard"}, "", 1, `unknown hook "rm-guard"`},
		{"usage: unknown flag", []string{"hook", "op-guard", "--bogus"}, sample(t, "claude-code-bash-op.json"), 1, "flag provided but not defined"},
		{"usage: extra argument", []string{"hook", "op-guard", "extra"}, "", 1, `unexpected argument "extra"`},
		{"usage: scope without print-config", []string{"hook", "op-guard", "--scope", "user"}, "", 1, "--scope needs --print-config"},
		{"usage: wrong scope", []string{"hook", "op-guard", "--print-config", "--scope", "team"}, "", 1, "project or user"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := hookStdin
			hookStdin = strings.NewReader(tt.stdin)
			t.Cleanup(func() { hookStdin = old })
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr %q)", got, tt.wantCode, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want no output", stdout.String())
			}
			if tt.wantStderr == "" && stderr.Len() != 0 {
				t.Errorf("stderr = %q, want no output", stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
			if tt.wantCode == hookAllow && strings.Contains(stderr.String(), "op ") {
				t.Errorf("stderr repeats the input: %q", stderr.String())
			}
		})
	}
}

// The help of the hook ends with exit 0 on stdout.
func TestHookHelp(t *testing.T) {
	for _, args := range [][]string{{"hook", "--help"}, {"hook", "op-guard", "--help"}} {
		var stdout, stderr bytes.Buffer
		if got := run(args, &stdout, &stderr); got != 0 {
			t.Errorf("%v: exit code = %d, want 0", args, got)
		}
		if !strings.Contains(stdout.String(), "--print-config") {
			t.Errorf("%v: help lacks --print-config: %q", args, stdout.String())
		}
	}
}

// --print-config prints a hooks block for Claude Code: PreToolUse, matcher
// Bash, exec form with the arguments "hook op-guard". Project scope uses
// the bare name, user scope the absolute path of the running a38.
func TestHookPrintConfig(t *testing.T) {
	old := hookExecutable
	hookExecutable = func() (string, error) { return "/opt/beispiel/bin/a38", nil }
	t.Cleanup(func() { hookExecutable = old })

	tests := []struct {
		name        string
		args        []string
		wantCommand string
	}{
		{"default", []string{"hook", "op-guard", "--print-config"}, "a38"},
		{"project", []string{"hook", "op-guard", "--print-config", "--scope", "project"}, "a38"},
		{"user", []string{"hook", "op-guard", "--print-config", "--scope=user"}, "/opt/beispiel/bin/a38"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != 0 {
				t.Fatalf("exit code = %d, want 0 (stderr %q)", got, stderr.String())
			}
			var settings struct {
				Hooks map[string][]struct {
					Matcher string `json:"matcher"`
					Hooks   []struct {
						Type    string   `json:"type"`
						Command string   `json:"command"`
						Args    []string `json:"args"`
						Timeout int      `json:"timeout"`
					} `json:"hooks"`
				} `json:"hooks"`
			}
			dec := json.NewDecoder(&stdout)
			dec.DisallowUnknownFields()
			if err := dec.Decode(&settings); err != nil {
				t.Fatalf("output is not the expected JSON: %v", err)
			}
			groups := settings.Hooks["PreToolUse"]
			if len(settings.Hooks) != 1 || len(groups) != 1 || groups[0].Matcher != "Bash" || len(groups[0].Hooks) != 1 {
				t.Fatalf("unexpected hooks block: %+v", settings)
			}
			h := groups[0].Hooks[0]
			if h.Type != "command" || h.Command != tt.wantCommand || strings.Join(h.Args, " ") != "hook op-guard" || h.Timeout != opguard.HookTimeout {
				t.Errorf("handler = %+v", h)
			}
		})
	}

	hookExecutable = func() (string, error) { return "", errors.New("no path") }
	var stdout, stderr bytes.Buffer
	if got := run([]string{"hook", "op-guard", "--print-config", "--scope", "user"}, &stdout, &stderr); got != exitFailure || stdout.Len() != 0 {
		t.Errorf("exit code = %d, stdout %q; want 1 and no output", got, stdout.String())
	}
}

// The guard and the configuration use the same name of the token file.
func TestHookTokenFileName(t *testing.T) {
	path, err := config.DefaultTokenFile()
	if err != nil {
		t.Skip(err)
	}
	if rule, _ := opguard.Check("cat " + path); rule != opguard.RuleTokenFile {
		t.Errorf("the guard does not know the default token file %s", filepath.Base(path))
	}
}

// T29 (steps 1 and 2): the same with the real binary.
// F16 (part): the contract holds for the real binary with a pipe on stdin,
// as the agent tool starts it. The test sends the recorded samples only;
// it never runs the inspected command.
func TestHookOpGuardProcess(t *testing.T) {
	bin := buildA38(t)
	tests := []struct {
		file     string
		wantCode int
	}{
		{"claude-code-bash-op.json", 2},
		{"claude-code-bash-op-in-substitution.json", 2},
		{"claude-code-bash-allowed.json", 0},
		{"claude-code-read-tool.json", 0},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			cmd := a38Cmd(context.Background(), t, bin, "hook", "op-guard")
			cmd.Stdin = strings.NewReader(sample(t, tt.file))
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			code := 0
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				code = exitErr.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr %q)", code, tt.wantCode, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want no output", stdout.String())
			}
			if tt.wantCode == 0 && stderr.Len() != 0 {
				t.Errorf("stderr = %q, want no output", stderr.String())
			}
			if tt.wantCode == 2 && !strings.Contains(stderr.String(), `"request_credentials"`) {
				t.Errorf("stderr = %q, want the block message", stderr.String())
			}
		})
	}
}
