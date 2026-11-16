package mcp

import "time"

// SetKillGrace sets the time from SIGTERM to SIGKILL and returns a
// function that restores it.
func SetKillGrace(d time.Duration) func() {
	old := killGrace
	killGrace = d
	return func() { killGrace = old }
}
