package daemon

import (
	"context"
	"testing"

	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

func TestExecWayReachesTheHandler(t *testing.T) {
	// T32 (part): the daemon gives request_exec and exec_result of a
	// session to the broker and sets the correlation ID of each answer.
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
	if err != nil {
		t.Fatal(err)
	}
	if len(creds.Values) != 1 || creds.Values[0].Name != "DB_PASSWORD" || creds.OutputLimit != 2048 {
		t.Fatalf("exec_credentials = %d values, limit %d", len(creds.Values), creds.OutputLimit)
	}
	rec, err := c.ExecResult(testCtx(t), &protocol.ExecResult{RequestID: "c-2", ExitCode: 0})
	if err != nil || rec == nil {
		t.Fatalf("exec_result = %v, %v", rec, err)
	}
	if n := broker.callCount(); n != 2 {
		t.Errorf("the daemon called the broker %d times, want 2", n)
	}
	r.wantWorks(t)
}

func TestFaultedRefusesTheExecWay(t *testing.T) {
	// Without a broker the daemon delivers nothing (fail closed).
	ctx := context.Background()
	if c, e := (Faulted{}).RequestExec(ctx, session.Session{}, &protocol.RequestExec{}); c != nil || e == nil || e.Code != protocol.CodeDaemonFaulted {
		t.Errorf("RequestExec = %v, %v", c, e)
	}
	if r, e := (Faulted{}).ExecResult(ctx, session.Session{}, &protocol.ExecResult{}); r != nil || e == nil || e.Code != protocol.CodeDaemonFaulted {
		t.Errorf("ExecResult = %v, %v", r, e)
	}
}
