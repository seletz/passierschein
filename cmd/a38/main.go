// Command a38 is the command-line interface of Passierschein.
//
// This is the skeleton of the repository. It has no product functions yet.
// It prints the version only, so that the build, the release and the
// signature steps have a binary to work on.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/seletz/passierschein/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the command line and returns the exit code.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
		_, _ = fmt.Fprintln(stdout, version.String())
		return 0
	}
	_, _ = fmt.Fprintln(stderr, "a38: no commands are available yet. Use --version.")
	return 2
}
