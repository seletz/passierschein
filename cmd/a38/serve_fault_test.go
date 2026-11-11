package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/client"
	"github.com/seletz/passierschein/internal/protocol"
)

// T26, T28 (part): the real process "a38 serve" with an audit log that
// cannot be written, over the real sockets.
//
// The failure is deterministic: the test removes the write permission of
// the directory of the audit file (mode 0500). SQLite must create a
// rollback journal next to the file for each write, so each write fails at
// once; reading still works. Mode 0700 repairs it.
func TestServeGlobalLock(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores the file mode")
	}
	bin := filepath.Join(t.TempDir(), "a38")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	cfgPath, socketDir, fakeFile := writeConfig(t)
	auditDir := filepath.Join(filepath.Dir(cfgPath), "audit")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte(filepath.Join(filepath.Dir(cfgPath), "audit.db")), []byte(filepath.Join(auditDir, "audit.db")), 1)
	if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	breakAudit := func(broken bool) {
		t.Helper()
		mode := os.FileMode(0o700)
		if broken {
			mode = 0o500
		}
		if err := os.Chmod(auditDir, mode); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _ = os.Chmod(auditDir, 0o700) })

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	args := []string{"--backend", "fake", "--fake-file", fakeFile, "--ui", "auto-approve"}
	stop := startServe(t, bin, cfgPath, socketDir, args...)

	agent := func(name string) *client.Conn {
		t.Helper()
		c, err := client.DialAgent(ctx, filepath.Join(socketDir, "agent.sock"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = c.Close() })
		if _, err := c.Hello(ctx, name, "coding-agent"); err != nil {
			t.Fatal(err)
		}
		return c
	}
	adminConn := func() *client.Conn {
		t.Helper()
		c, err := client.DialAdmin(ctx, filepath.Join(socketDir, "admin.sock"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = c.Close() })
		return c
	}
	pass, _ := protocol.ParseReference("op://beispiel.app.k8s-dev/datenbank-dev/password")
	user, _ := protocol.ParseReference("op://beispiel.app.k8s-dev/datenbank-dev/username")
	ask := func(c *client.Conn, ref protocol.Reference) (string, protocol.Code) {
		t.Helper()
		creds, err := c.RequestCredentials(ctx, &protocol.RequestCredentials{References: []protocol.Reference{ref}})
		if err != nil {
			return "", protocol.CodeOf(err)
		}
		if len(creds.Entries) != 1 || creds.Entries[0].Value == nil {
			t.Fatalf("answer without value: %+v", creds)
		}
		return creds.Entries[0].Value.Reveal(), ""
	}

	a := agent("beispiel-koordinator")
	if v, code := ask(a, pass); code != "" || v != "beispiel-wert-serve" {
		t.Fatalf("A before the fault: %s", code)
	}
	admin := adminConn()

	breakAudit(true)
	if _, code := ask(a, user); code != protocol.CodeAuditFailed {
		t.Fatalf("request at the fault = %s, want audit_failed", code)
	}
	b := agent("beispiel-koordinator-b")
	for name, c := range map[string]*client.Conn{"A under its grant": a, "B": b} {
		if _, code := ask(c, pass); code != protocol.CodeDaemonFaulted {
			t.Errorf("%s = %s, want daemon_faulted", name, code)
		}
	}
	st, err := admin.Status(ctx)
	if err != nil || st.Fault == nil || st.Fault.Code != protocol.CodeAuditFailed ||
		st.Fault.CheckPassed == nil || *st.Fault.CheckPassed || !st.Fault.BeginPending {
		t.Fatalf("status = %+v, %v", st, err)
	}
	if g, err := admin.GrantsList(ctx); err != nil || len(g.Grants) != 1 {
		t.Errorf("grants list during the lock = %+v, %v", g, err)
	}
	if res, err := admin.FaultClear(ctx); err != nil || res.Cleared || res.Finding == "" {
		t.Fatalf("clear with the cause = %+v, %v", res, err)
	}
	if _, err := a.FaultClear(ctx); protocol.CodeOf(err) != protocol.CodeInvalidRequest {
		t.Fatalf("clear on the agent socket = %v, want invalid_request", err)
	}

	breakAudit(false)
	if _, code := ask(a, pass); code != protocol.CodeDaemonFaulted {
		t.Fatalf("after the repair = %s, want daemon_faulted", code)
	}
	if st, err := admin.Status(ctx); err != nil || st.Fault == nil || st.Fault.CheckPassed == nil || !*st.Fault.CheckPassed {
		t.Fatalf("status after the repair = %+v, %v", st, err)
	}
	if res, err := admin.FaultClear(ctx); err != nil || !res.Cleared {
		t.Fatalf("clear = %+v, %v", res, err)
	}
	if v, code := ask(a, pass); code != "" || v != "beispiel-wert-serve" {
		t.Fatalf("A after the clear: %s", code)
	}
	stop()
	if got := lockDetails(t, bin, cfgPath); strings.Join(got, ",") != "fault_begin,fault_end" {
		t.Fatalf("lock entries = %v", got)
	}

	// A restart with the cause present starts locked.
	breakAudit(true)
	stop = startServe(t, bin, cfgPath, socketDir, args...)
	admin = adminConn()
	if st, err := admin.Status(ctx); err != nil || st.Fault == nil || st.Fault.Code != protocol.CodeAuditFailed {
		t.Fatalf("status after the restart = %+v, %v", st, err)
	}
	if _, code := ask(agent("beispiel-koordinator"), pass); code != protocol.CodeDaemonFaulted {
		t.Fatalf("request after the restart = %s, want daemon_faulted", code)
	}
	breakAudit(false)
	if res, err := admin.FaultClear(ctx); err != nil || !res.Cleared {
		t.Fatalf("clear after the restart = %+v, %v", res, err)
	}
	stop()
}

// lockDetails returns the details fault_begin and fault_end of the audit
// log, in their sequence.
func lockDetails(t *testing.T, bin, cfgPath string) []string {
	t.Helper()
	out, err := exec.Command(bin, "audit", "list", "--config", cfgPath, "--json").Output()
	if err != nil {
		t.Fatalf("a38 audit list: %v", err)
	}
	var got []string
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var e struct {
			Type   string `json:"type"`
			Result struct {
				Detail string `json:"detail"`
			} `json:"result"`
		}
		if err := dec.Decode(&e); err != nil {
			t.Fatal(err)
		}
		if e.Type == "error" && strings.HasPrefix(e.Result.Detail, "fault_") {
			got = append(got, e.Result.Detail)
		}
	}
	return got
}
