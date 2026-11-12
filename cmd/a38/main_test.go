package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/seletz/passierschein/internal/skill"
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
		{"serve without configuration", []string{"serve", "--config", "/tmp/a38-missing/config.yaml", "--backend", "fake", "--fake-file", "/tmp/a38-missing/fake.yaml", "--ui", "auto-approve"}, 1, "", "cannot read the configuration file"},
		{"mcp without configuration", []string{"mcp", "--config", "/tmp/a38-missing/config.yaml"}, 1, "", "cannot read the configuration file"},
		{"audit without subcommand", []string{"audit"}, 2, "", "a38 audit list"},
		{"grants without subcommand", []string{"grants"}, 2, "", "a38 grants list"},
		{"grants without configuration", []string{"grants", "list", "--config", "/tmp/a38-missing/config.yaml"}, 1, "", "cannot read the configuration file"},
		{"fault without subcommand", []string{"fault"}, 2, "", "a38 fault status"},
		{"fault without configuration", []string{"fault", "status", "--config", "/tmp/a38-missing/config.yaml"}, 1, "", "cannot read the configuration file"},
		{"config without subcommand", []string{"config"}, 2, "", "onepassword-token"},
		{"config help", []string{"config", "--help"}, 0, "onepassword-token", ""},
		{"config unknown subcommand", []string{"config", "vaults"}, 2, "", `unknown subcommand "vaults"`},
		{"config token without subcommand", []string{"config", "onepassword-token"}, 2, "", "a38 config onepassword-token set"},
		{"old token command is gone", []string{"token", "status"}, 2, "", `unknown command "token"`},
		{"doctor without configuration", []string{"doctor", "--config", "/tmp/a38-missing/config.yaml"}, 1, "FINDING", ""},
		{"llm without flag", []string{"llm"}, 2, "", "usage: a38 llm --skill"},
		{"llm with unknown flag", []string{"llm", "--other"}, 2, "", "usage: a38 llm --skill"},
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
	for _, name := range []string{"serve", "mcp", "audit", "grants", "fault", "config", "doctor", "llm", "version"} {
		if !strings.Contains(stdout.String(), "\n  "+name+" ") {
			t.Errorf("the usage text does not list the command %q", name)
		}
	}
}

// T20: "a38 llm --skill" prints exactly the embedded file.
func TestLLMSkillPrintsEmbeddedFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := run([]string{"llm", "--skill"}, &stdout, &stderr); got != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr %q)", got, stderr.String())
	}
	if stdout.String() != skill.Text() {
		t.Error("the output differs from the embedded file")
	}
	if stdout.Len() == 0 || stderr.Len() != 0 {
		t.Errorf("stdout has %d bytes, stderr = %q", stdout.Len(), stderr.String())
	}
}
