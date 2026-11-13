//go:build !darwin

package main

import (
	"fmt"
	"io"
)

// runTUI is not available outside macOS: the audit log lives on the Mac
// with the daemon (D53). The linux build of a38 is the client "a38 mcp" and
// carries no terminal view. "a38 audit list" and "a38 audit stats" work on
// each platform.
func runTUI(_ []string, _, stderr io.Writer) int {
	_, _ = fmt.Fprintln(stderr, `a38 tui: not available on this platform (macOS only); use "a38 audit list" or "a38 audit stats"`)
	return exitFailure
}
