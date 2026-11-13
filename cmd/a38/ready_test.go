package main

import (
	"strings"
	"testing"
	"time"
)

// daemonStartTimeout is the time that a test waits for a daemon to start.
// It is long on purpose: a slow runner may need seconds, and a test that
// passes waits only as long as the start takes.
const daemonStartTimeout = 30 * time.Second

// stringer is a log that a test can read while the daemon writes to it.
type stringer interface{ String() string }

// waitDaemonStarted waits until the daemon has logged "daemon started". This
// is the ONE way for a test of this package to wait for a daemon, whether it
// runs in the test process (serve) or as a process (a38 serve).
//
// The daemon logs the line after Listen has returned: both sockets listen,
// so a connection is accepted from then on. Do not wait for the socket file
// and do not poll with a pause: the file of an older daemon exists from
// bind(2) on, before listen(2), and a connect in between is refused (flake
// of TestServeRuns, 2026-11-12). The daemon now publishes the file only
// when it listens (internal/daemon), but a test must not rely on that.
//
// exited is optional. It reports that the daemon ended before the line; the
// test then fails at once with the log instead of after the timeout.
func waitDaemonStarted(t *testing.T, log stringer, exited <-chan struct{}) {
	t.Helper()
	deadline := time.Now().Add(daemonStartTimeout)
	for {
		if strings.Contains(log.String(), "daemon started") {
			return
		}
		select {
		case <-exited:
			t.Fatalf("the daemon ended before it started; log:\n%s", log.String())
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("the daemon did not log \"daemon started\" within %v; log:\n%s", daemonStartTimeout, log.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
}
