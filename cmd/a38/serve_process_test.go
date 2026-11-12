package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/client"
	"github.com/seletz/passierschein/internal/protocol"
)

// buildA38 builds the binary into a temporary directory.
func buildA38(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("builds the binary")
	}
	bin := filepath.Join(t.TempDir(), "a38")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

// daemonProc is a running "a38 serve" process.
type daemonProc struct {
	cmd    *exec.Cmd
	stderr *syncBuffer
}

// startDaemon starts "a38 serve" with the fake backend and automatic
// approval and waits for the line "daemon started" (waitDaemonStarted).
func startDaemon(t *testing.T, bin, cfgPath, socketDir, fakeFile string) *daemonProc {
	t.Helper()
	p := &daemonProc{stderr: &syncBuffer{}}
	p.cmd = exec.Command(bin, "serve", "--config", cfgPath, "--backend", "fake", "--fake-file", fakeFile, "--ui", "auto-approve")
	p.cmd.Stderr = p.stderr
	if err := p.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.cmd.Process.Kill(); _ = p.cmd.Wait() })
	waitDaemonStarted(t, p.stderr, nil)
	return p
}

// signalAndWait sends sig and waits for the end. It returns the exit code.
func (p *daemonProc) signalAndWait(t *testing.T, sig syscall.Signal) int {
	t.Helper()
	if err := p.cmd.Process.Signal(sig); err != nil {
		t.Fatal(err)
	}
	err := p.cmd.Wait()
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return 0
}

// requestPass runs one request of a session with the name and returns
// after the delivery.
func requestPass(t *testing.T, socketDir, name string) *client.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	agent, err := client.DialAgent(ctx, filepath.Join(socketDir, "agent.sock"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := agent.Hello(ctx, name, "coding-agent"); err != nil {
		t.Fatal(err)
	}
	ref, _ := protocol.ParseReference("op://beispiel.app.k8s-dev/datenbank-dev/password")
	creds, err := agent.RequestCredentials(ctx, &protocol.RequestCredentials{
		References: []protocol.Reference{ref}, Task: "Run the tests", Reason: "The tests need the login.",
	})
	if err != nil || len(creds.Entries) != 1 || creds.Entries[0].Value == nil {
		t.Fatalf("request: %+v, %v", creds, err)
	}
	return agent
}

// auditEntries reads the audit log with "a38 audit list --json".
func auditEntries(t *testing.T, bin, cfgPath string) []audit.Entry {
	t.Helper()
	out, err := exec.Command(bin, "audit", "list", "--config", cfgPath, "--json").Output()
	if err != nil {
		t.Fatalf("a38 audit list: %v", err)
	}
	var entries []audit.Entry
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var e audit.Entry
		if err := dec.Decode(&e); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, e)
	}
	return entries
}

func TestServeRestartEndsGrants(t *testing.T) {
	// T16: a stop of the daemon with SIGTERM ends each live grant, and the
	// audit log names the reason daemon_stopped. After the restart on the
	// same audit file, the same request needs a new approval, and the
	// chain is intact. An ungraceful end writes nothing (SIGKILL: not
	// tested, nothing runs).
	bin := buildA38(t)
	cfgPath, socketDir, fakeFile := writeConfig(t)

	d := startDaemon(t, bin, cfgPath, socketDir, fakeFile)
	agent := requestPass(t, socketDir, "beispiel-koordinator")
	// The connection stays open: the stop, not the end of the session,
	// ends the grant.
	if code := d.signalAndWait(t, syscall.SIGTERM); code != 0 {
		t.Fatalf("exit code after SIGTERM = %d; stderr:\n%s", code, d.stderr.String())
	}
	_ = agent.Close()

	var ends []audit.Entry
	var grantID string
	for _, e := range auditEntries(t, bin, cfgPath) {
		switch e.Type {
		case audit.EventGrant:
			grantID = e.Result.GrantID
		case audit.EventExpiry:
			ends = append(ends, e)
		}
	}
	if len(ends) != 1 || ends[0].Result.Detail != audit.DetailDaemonStopped || ends[0].Result.GrantID != grantID || grantID == "" {
		t.Fatalf("end events = %+v, grant %q; want one daemon_stopped for the grant", ends, grantID)
	}
	if code, out, errOut := a38Out(t, bin, "audit", "verify", "--config", cfgPath); code != 0 {
		t.Fatalf("audit verify after the stop: %d %s %s", code, out, errOut)
	}

	d = startDaemon(t, bin, cfgPath, socketDir, fakeFile)
	agent = requestPass(t, socketDir, "beispiel-koordinator")
	_ = agent.Close()
	if code := d.signalAndWait(t, syscall.SIGTERM); code != 0 {
		t.Fatalf("exit code after the second SIGTERM = %d", code)
	}
	var decisions int
	for _, e := range auditEntries(t, bin, cfgPath) {
		if e.Type == audit.EventDecision {
			decisions++
		}
	}
	if decisions != 2 {
		t.Errorf("decisions = %d; want a new approval after the restart", decisions)
	}
	if code, out, errOut := a38Out(t, bin, "audit", "verify", "--config", cfgPath); code != 0 || !strings.Contains(out, "intact") {
		t.Errorf("audit verify across the restart: %d %s %s", code, out, errOut)
	}
}

func TestServeSessionNameInAuditLog(t *testing.T) {
	// T21: in the real flow, the entries request, bundle, decision, grant
	// and delivery carry the session name in the claims and never in the
	// verified facts. "a38 audit list" shows the name.
	bin := buildA38(t)
	cfgPath, socketDir, fakeFile := writeConfig(t)
	d := startDaemon(t, bin, cfgPath, socketDir, fakeFile)
	agent := requestPass(t, socketDir, "beispiel-koordinator")
	_ = agent.Close()
	if code := d.signalAndWait(t, syscall.SIGTERM); code != 0 {
		t.Fatalf("exit code = %d", code)
	}

	seen := map[audit.EventType]bool{}
	for _, e := range auditEntries(t, bin, cfgPath) {
		seen[e.Type] = true
		switch e.Type {
		case audit.EventRequest, audit.EventBundle, audit.EventDecision, audit.EventGrant, audit.EventDelivery:
			if e.Claims.SessionName != "beispiel-koordinator" {
				t.Errorf("%s: claims name = %q", e.Type, e.Claims.SessionName)
			}
			verified, err := json.Marshal(e.Verified)
			if err != nil || strings.Contains(string(verified), "beispiel-koordinator") || strings.Contains(string(verified), "session_name") {
				t.Errorf("%s: the verified facts hold the name: %s", e.Type, verified)
			}
		}
	}
	for _, ty := range []audit.EventType{audit.EventRequest, audit.EventBundle, audit.EventDecision, audit.EventGrant, audit.EventDelivery} {
		if !seen[ty] {
			t.Errorf("no %s entry", ty)
		}
	}
	code, out, errOut := a38Out(t, bin, "audit", "list", "--config", cfgPath)
	if code != 0 || strings.Count(out, "beispiel-koordinator") < 5 {
		t.Errorf("audit list: code %d, stdout:\n%s\nstderr %q", code, out, errOut)
	}
	// The name is a filter, too.
	if code, out, _ := a38Out(t, bin, "audit", "list", "--config", cfgPath, "--session-name", "beispiel-koordinator", "--type", "delivery"); code != 0 || !strings.Contains(out, "delivery") {
		t.Errorf("audit list --session-name: code %d, %q", code, out)
	}
}

func TestServeNoSessionNameInAuditLog(t *testing.T) {
	// T21: a session without a name has no name in the claims; the audit
	// log invents none.
	bin := buildA38(t)
	cfgPath, socketDir, fakeFile := writeConfig(t)
	d := startDaemon(t, bin, cfgPath, socketDir, fakeFile)
	agent := requestPass(t, socketDir, "")
	_ = agent.Close()
	if code := d.signalAndWait(t, syscall.SIGTERM); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	entries := auditEntries(t, bin, cfgPath)
	if len(entries) == 0 {
		t.Fatal("no entries")
	}
	for _, e := range entries {
		if e.Claims.SessionName != "" {
			t.Errorf("%s: claims name = %q, want none", e.Type, e.Claims.SessionName)
		}
	}
}

func TestServeInvalidVaultList(t *testing.T) {
	// T22: a38 serve does not start with an invalid vault list. The exit
	// code is not 0, the message names the vault, and no socket exists.
	bin := buildA38(t)
	tests := []struct {
		name   string
		vaults string
		want   string
	}{
		{"vault without environment", "  - {name: beispiel.app.k8s-dev}\n", "beispiel.app.k8s-dev"},
		{"unknown environment", "  - {name: beispiel.app.k8s-dev, environment: staging}\n", "beispiel.app.k8s-dev"},
		{"duplicate vault", "  - {name: beispiel.app.k8s-dev, environment: dev}\n  - {name: beispiel.app.k8s-dev, environment: test}\n", "beispiel.app.k8s-dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, socketDir, fakeFile := writeConfig(t)
			cfgPath := filepath.Join(filepath.Dir(socketDir), "invalid.yaml")
			data := "version: 1\nvaults:\n" + tt.vaults + "paths:\n  socket_dir: " + socketDir + "\n  audit_file: " + filepath.Join(filepath.Dir(socketDir), "audit.db") + "\n"
			if err := os.WriteFile(cfgPath, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(bin, "serve", "--config", cfgPath, "--backend", "fake", "--fake-file", fakeFile, "--ui", "auto-approve")
			var stderr syncBuffer
			cmd.Stderr = &stderr
			err := cmd.Run()
			ee, ok := err.(*exec.ExitError)
			if !ok || ee.ExitCode() == 0 {
				t.Fatalf("err = %v; want an exit code that is not 0; stderr:\n%s", err, stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Errorf("stderr does not name the vault:\n%s", stderr.String())
			}
			if _, err := os.Stat(filepath.Join(socketDir, "agent.sock")); !os.IsNotExist(err) {
				t.Error("a socket exists after the failed start")
			}
		})
	}
}
