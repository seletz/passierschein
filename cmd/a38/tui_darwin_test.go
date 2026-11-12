//go:build darwin

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestTUIArguments(t *testing.T) {
	// F13: the arguments of "a38 tui"; without a terminal the view does
	// not start and names the commands for scripts.
	path := demoAuditFile(t)
	tests := []struct {
		args []string
		code int
		want string
	}{
		{[]string{"tui", "--file", path, "--range", "1y"}, 2, "unknown time range"},
		{[]string{"tui", "--file", path, "--refresh", "10ms"}, 2, "--refresh must be 0 or at least 1s"},
		{[]string{"tui", "--file", path, "extra"}, 2, "unexpected argument"},
		{[]string{"tui", "--file", filepath.Join(t.TempDir(), "missing.db")}, 1, "cannot read the audit file"},
		{[]string{"tui", "--config", "/tmp/a38-missing/config.yaml"}, 1, "cannot read the configuration file"},
		{[]string{"tui", "--file", path}, 1, "needs a terminal"},
	}
	for _, tc := range tests {
		code, _, errOut := runCLI(tc.args...)
		if code != tc.code || !strings.Contains(errOut, tc.want) {
			t.Errorf("%v: code %d, stderr %q", tc.args, code, errOut)
		}
	}
}
