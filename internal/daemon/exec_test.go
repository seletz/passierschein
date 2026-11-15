package daemon

import (
	"testing"

	"github.com/seletz/passierschein/internal/protocol"
)

func TestExecWayNotImplemented(t *testing.T) {
	// T32 (part): the daemon reads request_exec and exec_result and answers
	// with a clear error until the broker implements the exec way (issue
	// 99). It calls no handler and delivers no value.
	broker := newFakeBroker()
	r := startServer(t, testConfig(t, nil), broker)
	c, _ := r.agent(t, "beispiel-koordinator")
	ref, err := protocol.ParseReference("op://beispiel.app.k8s-dev/datenbank-dev/password")
	if err != nil {
		t.Fatal(err)
	}
	creds, err := c.RequestExec(testCtx(t), &protocol.RequestExec{
		Program: "/usr/bin/env", Cwd: "/",
		Env:  []protocol.ExecVar{{Name: "DB_PASSWORD", Reference: ref}},
		Task: "Run the tests", Reason: "The tests need the login.",
	})
	if creds != nil {
		t.Fatalf("the daemon delivered values: %d", len(creds.Values))
	}
	wantCode(t, err, protocol.CodeNotAllowed)

	rec, err := c.ExecResult(testCtx(t), &protocol.ExecResult{RequestID: "c-2", ExitCode: 0})
	if rec != nil {
		t.Fatal("the daemon recorded an exec_result without delivery")
	}
	wantCode(t, err, protocol.CodeNotAllowed)
	if n := broker.callCount(); n != 0 {
		t.Errorf("the daemon called the broker %d times", n)
	}
	r.wantWorks(t)
}
