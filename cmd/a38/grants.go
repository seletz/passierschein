package main

import "io"

// runGrants lists and revokes grants over the admin socket. It is not implemented yet.
func runGrants(_ []string, _, stderr io.Writer) int {
	return notImplemented("grants", stderr)
}
