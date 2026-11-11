package main

import "io"

// runAudit lists the audit log and verifies its hash chain. It is not implemented yet.
func runAudit(_ []string, _, stderr io.Writer) int {
	return notImplemented("audit", stderr)
}
