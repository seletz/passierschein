package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/daemon"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// testBroker implements daemon.AgentHandler and daemon.AdminHandler with
// fixed answers.
type testBroker struct {
	mu      sync.Mutex
	grants  []protocol.Grant
	fault   *protocol.Fault
	clear   *protocol.FaultClearResult
	revoked []string
}

func (b *testBroker) RequestCredentials(context.Context, session.Session, *protocol.RequestCredentials) (*protocol.Credentials, *protocol.Error) {
	return nil, protocol.Errorf(protocol.CodeNotAllowed, "not in this test")
}
func (b *testBroker) GrantStatus(context.Context, session.Session, *protocol.GrantStatus) (*protocol.Grants, *protocol.Error) {
	return &protocol.Grants{}, nil
}
func (b *testBroker) SessionEnded(session.Session) {}

func (b *testBroker) GrantsList(context.Context, *protocol.GrantsList) (*protocol.Grants, *protocol.Error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return &protocol.Grants{Grants: append([]protocol.Grant(nil), b.grants...)}, nil
}

func (b *testBroker) GrantRevoke(_ context.Context, req *protocol.GrantRevoke) (*protocol.GrantRevoked, *protocol.Error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, g := range b.grants {
		if g.GrantID == req.GrantID {
			b.grants = append(b.grants[:i], b.grants[i+1:]...)
			b.revoked = append(b.revoked, g.GrantID)
			return &protocol.GrantRevoked{GrantID: g.GrantID}, nil
		}
	}
	return nil, protocol.Errorf(protocol.CodeNotFound, "the grant does not exist")
}

func (b *testBroker) Status(context.Context, *protocol.Status) (*protocol.StatusResult, *protocol.Error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return &protocol.StatusResult{Grants: len(b.grants), Fault: b.fault}, nil
}

func (b *testBroker) FaultClear(context.Context, *protocol.FaultClear) (*protocol.FaultClearResult, *protocol.Error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.clear == nil {
		return &protocol.FaultClearResult{Cleared: true}, nil
	}
	return b.clear, nil
}

// startAdminDaemon runs an in-process daemon with the test broker and
// returns the path of the configuration.
func startAdminDaemon(t *testing.T, b *testBroker) string {
	t.Helper()
	cfgPath, _, _ := writeConfig(t)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := daemon.New(daemon.Options{Config: cfg, Agent: b, Admin: b})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = srv.Serve(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return cfgPath
}

func sampleGrant(t *testing.T, id string) protocol.Grant {
	t.Helper()
	ref, err := protocol.ParseReference("op://beispiel.app.k8s-dev/datenbank-dev/password")
	if err != nil {
		t.Fatal(err)
	}
	return protocol.Grant{
		GrantID: id, SessionID: "s-0123456789abcdef", SessionName: "beispiel-koordinator",
		Project: "beispielprojekt", References: []protocol.Reference{ref},
		GrantedAt: time.Now(), ExpiresAt: time.Now().Add(3*time.Hour + 59*time.Minute), RemainingSeconds: 3*3600 + 59*60,
	}
}

func runCmd(args ...string) (code int, stdout, stderr string) {
	var o, e bytes.Buffer
	code = run(args, &o, &e)
	return code, o.String(), e.String()
}

func TestGrantsList(t *testing.T) {
	// F5: the table shows no value; the session ID is short;
	b := &testBroker{grants: []protocol.Grant{sampleGrant(t, "g-1")}}
	cfg := startAdminDaemon(t, b)

	code, out, errOut := runCmd("grants", "list", "--config", cfg)
	if code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, errOut)
	}
	for _, want := range []string{"GRANT", "g-1", "s-012345", "beispiel-koordinator", "beispielprojekt", "op://beispiel.app.k8s-dev/datenbank-dev/password", "3h59m0s"} {
		if !strings.Contains(out, want) {
			t.Errorf("table does not contain %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "s-0123456789") {
		t.Errorf("table has the long session ID:\n%q", out)
	}

	code, out, _ = runCmd("grants", "list", "--json", "--config", cfg)
	var got []protocol.Grant
	if code != 0 || json.Unmarshal([]byte(out), &got) != nil || len(got) != 1 || got[0].GrantID != "g-1" || got[0].SessionID != "s-0123456789abcdef" {
		t.Errorf("json: code %d, %q", code, out)
	}

	b.mu.Lock()
	b.grants = nil
	b.mu.Unlock()
	if code, out, _ = runCmd("grants", "list", "--config", cfg); code != 0 || !strings.Contains(out, "No active grants.") {
		t.Errorf("empty list: code %d, %q", code, out)
	}
	if code, out, _ = runCmd("grants", "list", "--json", "--config", cfg); code != 0 || strings.TrimSpace(out) != "[]" {
		t.Errorf("empty json list: code %d, %q", code, out)
	}
}

func TestGrantsRevoke(t *testing.T) {
	// F5: revoke ends one grant; an unknown ID is a clear message and a
	// non-zero exit code.
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantOut    string
		wantErr    string
		wantRevoke []string
	}{
		{"known", []string{"g-1"}, 0, "The grant g-1 ended", "", []string{"g-1"}},
		{"known json", []string{"--json", "g-1"}, 0, `{"revoked":"g-1"}`, "", []string{"g-1"}},
		{"unknown", []string{"g-9"}, 1, "", `no grant with the ID "g-9"`, nil},
		{"no id", nil, 2, "", "one grant ID is necessary", nil},
		{"two ids", []string{"g-1", "g-2"}, 2, "", "one grant ID is necessary", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &testBroker{grants: []protocol.Grant{sampleGrant(t, "g-1")}}
			cfg := startAdminDaemon(t, b)
			code, out, errOut := runCmd(append([]string{"grants", "revoke", "--config", cfg}, tt.args...)...)
			if code != tt.wantCode || !strings.Contains(out, tt.wantOut) || !strings.Contains(errOut, tt.wantErr) {
				t.Errorf("code %d, stdout %q, stderr %q", code, out, errOut)
			}
			if strings.Join(b.revoked, ",") != strings.Join(tt.wantRevoke, ",") {
				t.Errorf("revoked = %v, want %v", b.revoked, tt.wantRevoke)
			}
		})
	}
}

func TestAdminCommandsWithoutDaemon(t *testing.T) {
	// UC7: a daemon that does not run is a clear message and a non-zero
	// exit code.
	cfg, _, _ := writeConfig(t)
	for _, args := range [][]string{
		{"grants", "list"}, {"grants", "revoke", "g-1"}, {"fault", "status"}, {"fault", "clear"},
	} {
		code, out, errOut := runCmd(append([]string{args[0], args[1], "--config", cfg}, args[2:]...)...)
		if code != 1 || out != "" || !strings.Contains(errOut, "the daemon does not run") || !strings.Contains(errOut, "a38 serve") {
			t.Errorf("%v: code %d, stdout %q, stderr %q", args, code, out, errOut)
		}
	}
}

func TestFaultStatus(t *testing.T) {
	// D48: the status shows the lock; without a cause field it still works.
	since := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		fault    *protocol.Fault
		args     []string
		wantCode int
		want     []string
	}{
		{"open", nil, nil, 0, []string{"Delivery is open."}},
		{"locked", &protocol.Fault{Code: protocol.CodeAuditFailed, Reason: "the audit log is not writable", Since: since}, nil, 1,
			[]string{"Delivery is locked for all sessions.", "audit_failed", "the audit log is not writable", "2026-11-11T10:00:00Z", `a38 fault clear`}},
		{"locked without details", &protocol.Fault{Code: protocol.CodeDaemonFaulted}, nil, 1, []string{"Delivery is locked for all sessions.", "a38 fault clear"}},
		{"locked json", &protocol.Fault{Code: protocol.CodeAuditFailed}, []string{"--json"}, 1, []string{`"locked":true`, `"code":"audit_failed"`}},
		{"open json", nil, []string{"--json"}, 0, []string{`"locked":false`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := startAdminDaemon(t, &testBroker{fault: tt.fault})
			code, out, errOut := runCmd(append([]string{"fault", "status", "--config", cfg}, tt.args...)...)
			if code != tt.wantCode {
				t.Errorf("code %d, stderr %q", code, errOut)
			}
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("stdout does not contain %q:\n%s", w, out)
				}
			}
		})
	}
}

func TestFaultClear(t *testing.T) {
	// D48: the daemon lifts the lock only if the cause is gone. Otherwise
	// the command prints the finding and exits with 1.
	tests := []struct {
		name     string
		result   *protocol.FaultClearResult
		args     []string
		wantCode int
		wantOut  string
		wantErr  string
	}{
		{"cleared", &protocol.FaultClearResult{Cleared: true}, nil, 0, "Delivery is open.", ""},
		{"refused", &protocol.FaultClearResult{Finding: "the audit log is still not writable"}, nil, 1, "", "the audit log is still not writable"},
		{"refused json", &protocol.FaultClearResult{Finding: "the audit log is still not writable"}, []string{"--json"}, 1, `"cleared":false`, ""},
		{"cleared json", &protocol.FaultClearResult{Cleared: true}, []string{"--json"}, 0, `"cleared":true`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := startAdminDaemon(t, &testBroker{clear: tt.result})
			code, out, errOut := runCmd(append([]string{"fault", "clear", "--config", cfg}, tt.args...)...)
			if code != tt.wantCode || !strings.Contains(out, tt.wantOut) || !strings.Contains(errOut, tt.wantErr) {
				t.Errorf("code %d, stdout %q, stderr %q", code, out, errOut)
			}
		})
	}
}
