package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/backend"
)

// fakeChecker returns fixed account states and records the call.
type fakeChecker struct {
	states   []backend.AccountState
	limit    time.Duration
	deadline bool
}

func (f *fakeChecker) Check(ctx context.Context, limit time.Duration) []backend.AccountState {
	f.limit = limit
	_, f.deadline = ctx.Deadline()
	return f.states
}

func TestLogStartCheck(t *testing.T) {
	// T49: the check after the start gives each account its own time
	// limit and no shared deadline. The log names each account with its
	// own state: a hanging first account does not mark the next one (D78).
	f := &fakeChecker{states: []backend.AccountState{
		{Name: "firma", Configured: 1, Err: fmt.Errorf("no answer: %w", backend.ErrUnavailable)},
		{Name: "labor", Configured: 1, Err: fmt.Errorf("no token: %w", backend.ErrLocked)},
		{Name: "privat", Configured: 2, Visible: 2},
	}}
	var logs bytes.Buffer
	logStartCheck(context.Background(), f, 30*time.Second, slog.New(slog.NewTextHandler(&logs, nil)))
	if f.limit != 30*time.Second || f.deadline {
		t.Errorf("limit %s, shared deadline %v; want 30s for each account and no shared deadline", f.limit, f.deadline)
	}
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	want := []string{
		"account=firma code=backend_unavailable",
		"account=labor code=backend_locked",
		`msg="the 1password account works" account=privat vaults_configured=2 vaults_visible=2`,
	}
	if len(lines) != len(want) {
		t.Fatalf("log:\n%s", logs.String())
	}
	for i, w := range want {
		if !strings.Contains(lines[i], w) {
			t.Errorf("line %d = %q, want %q", i, lines[i], w)
		}
	}
}
