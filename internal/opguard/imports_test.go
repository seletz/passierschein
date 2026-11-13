package opguard_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// F16, D53: the guard is part of the agent side, which also runs on Linux.
// The linux build of a38 contains the package, and the package depends only
// on the standard library and the shell parser: no package of a38, no
// darwin-only package (keychain, purego), no 1Password code.
func TestLinuxBuild(t *testing.T) {
	const pkg = "github.com/seletz/passierschein/internal/opguard"
	list := func(args ...string) []string {
		t.Helper()
		cmd := exec.Command("go", append([]string{"list"}, args...)...)
		cmd.Env = append(os.Environ(), "GOOS=linux", "CGO_ENABLED=0")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("go list %v: %v", args, err)
		}
		return strings.Fields(string(out))
	}
	found := false
	for _, p := range list("-deps", "github.com/seletz/passierschein/cmd/a38") {
		if p == pkg {
			found = true
		}
	}
	if !found {
		t.Errorf("the linux build of a38 does not contain %s", pkg)
	}
	for _, p := range list("-deps", "-f", "{{if not .Standard}}{{.ImportPath}}{{end}}", pkg) {
		switch {
		case p == pkg, strings.HasPrefix(p, "mvdan.cc/sh/v3/"):
		default:
			t.Errorf("%s depends on %s", pkg, p)
		}
	}
}
