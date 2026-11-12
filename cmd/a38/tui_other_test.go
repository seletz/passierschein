//go:build !darwin

package main

import (
	"strings"
	"testing"
)

func TestTUINotAvailable(t *testing.T) {
	// D53: the linux build has no terminal view.
	code, _, errOut := runCLI("tui")
	if code != 1 || !strings.Contains(errOut, "not available on this platform") {
		t.Errorf("code %d, stderr %q", code, errOut)
	}
}
