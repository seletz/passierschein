package mcp_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestImportRule checks the rule of D2 and T13: the agent zone has no code
// of the broker, a backend or the audit log, and no 1Password code. The
// daemon is also forbidden (architecture page, rule 3).
func TestImportRule(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "github.com/seletz/passierschein/internal/mcp").Output()
	if err != nil {
		t.Fatal(err)
	}
	const mod = "github.com/seletz/passierschein/internal/"
	for _, pkg := range strings.Fields(string(out)) {
		for _, bad := range []string{"broker", "backend", "audit", "daemon", "approval"} {
			if pkg == mod+bad || strings.HasPrefix(pkg, mod+bad+"/") {
				t.Errorf("internal/mcp depends on %s", pkg)
			}
		}
		if strings.HasPrefix(pkg, "github.com/1password/") || strings.Contains(pkg, "onepassword") {
			t.Errorf("internal/mcp depends on %s", pkg)
		}
	}
}
