package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"version flag", []string{"--version"}, 0, "a38 ", ""},
		{"version command", []string{"version"}, 0, "a38 ", ""},
		{"help flag", []string{"--help"}, 0, "Usage:", ""},
		{"short help flag", []string{"-h"}, 0, "Usage:", ""},
		{"help command", []string{"help"}, 0, "Commands:", ""},
		{"no arguments", nil, 2, "", "Usage:"},
		{"unknown command", []string{"execute"}, 2, "", `unknown command "execute"`},
		{"unknown flag", []string{"--verbose"}, 2, "", `unknown command "--verbose"`},
		{"serve", []string{"serve", "--backend", "fake", "--ui", "auto-approve"}, 1, "", "a38 serve: not implemented"},
		{"mcp", []string{"mcp"}, 1, "", "a38 mcp: not implemented"},
		{"audit without subcommand", []string{"audit"}, 2, "", "a38 audit list"},
		{"grants", []string{"grants", "list"}, 1, "", "a38 grants: not implemented"},
		{"doctor", []string{"doctor"}, 1, "", "a38 doctor: not implemented"},
		{"llm", []string{"llm", "--skill"}, 1, "", "a38 llm: not implemented"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != tt.wantCode {
				t.Errorf("exit code = %d, want %d", got, tt.wantCode)
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tt.wantStdout)
			}
			if tt.wantStdout == "" && stdout.Len() != 0 {
				t.Errorf("stdout = %q, want no output", stdout.String())
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestUsageListsEachCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	run([]string{"--help"}, &stdout, &stderr)
	for _, name := range []string{"serve", "mcp", "audit", "grants", "doctor", "llm", "version"} {
		if !strings.Contains(stdout.String(), "\n  "+name+" ") {
			t.Errorf("the usage text does not list the command %q", name)
		}
	}
}
