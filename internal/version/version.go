// Package version holds the build data of the a38 binary.
package version

import "fmt"

// The linker sets these values at build time (-ldflags -X).
var (
	// Version is the release version, or "dev" for a local build.
	Version = "dev"
	// Commit is the short hash of the source commit.
	Commit = "unknown"
	// Date is the build date in RFC 3339 format.
	Date = "unknown"
)

// String returns the version line that "a38 --version" prints.
func String() string {
	return fmt.Sprintf("a38 %s (commit %s, built %s)", Version, Commit, Date)
}
