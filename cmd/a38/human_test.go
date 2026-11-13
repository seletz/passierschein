package main

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/client"
	"github.com/seletz/passierschein/internal/protocol"
)

// a38Out runs the binary and returns the exit code, stdout and stderr.
func a38Out(t *testing.T, bin string, args ...string) (int, string, string) {
	t.Helper()
	cmd := a38Cmd(context.Background(), t, bin, args...)
	var o, e syncBuffer
	cmd.Stdout, cmd.Stderr = &o, &e
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return code, o.String(), e.String()
}

func TestHumanCommandsWithDaemonProcess(t *testing.T) {
	// T16 (part), F5: a grant is created on the agent socket, listed and
	// revoked with the commands; the next request needs a new approval.
	// T12 (part), D44: the agent socket refuses an admin message.
	// T27 (part): doctor ends with 0 on a healthy system, without a value.
	if testing.Short() {
		t.Skip("builds the binary")
	}
	bin := filepath.Join(t.TempDir(), "a38")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	cfgPath, socketDir, fakeFile := writeConfig(t)

	// Before the start: the daemon does not run.
	if code, _, errOut := a38Out(t, bin, "grants", "list", "--config", cfgPath); code != 1 || !strings.Contains(errOut, "the daemon does not run") {
		t.Errorf("without daemon: code %d, stderr %q", code, errOut)
	}
	if code, out, _ := a38Out(t, bin, "doctor", "--config", cfgPath); code != 1 || !strings.Contains(out, "the daemon does not answer") {
		t.Errorf("doctor without daemon: code %d, stdout %q", code, out)
	}

	stop := startServe(t, bin, cfgPath, socketDir, "--backend", "fake", "--fake-file", fakeFile, "--ui", "auto-approve")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	agent, err := client.DialAgent(ctx, filepath.Join(socketDir, "agent.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = agent.Close() }()
	if _, err := agent.Hello(ctx, "beispiel-koordinator", "coding-agent"); err != nil {
		t.Fatal(err)
	}
	ref, _ := protocol.ParseReference("op://beispiel.app.k8s-dev/datenbank-dev/password")
	request := func() {
		t.Helper()
		creds, err := agent.RequestCredentials(ctx, &protocol.RequestCredentials{
			References: []protocol.Reference{ref}, Task: "Run the tests", Reason: "The tests need the login.",
		})
		if err != nil || len(creds.Entries) != 1 || creds.Entries[0].Value == nil {
			t.Fatalf("request: %+v, %v", creds, err)
		}
	}
	request()

	code, out, errOut := a38Out(t, bin, "grants", "list", "--config", cfgPath)
	if code != 0 || !strings.Contains(out, ref.String()) || !strings.Contains(out, "beispiel-koordinator") || !strings.Contains(out, "beispielprojekt") {
		t.Fatalf("grants list: code %d, stdout %q, stderr %q", code, out, errOut)
	}
	code, out, _ = a38Out(t, bin, "grants", "list", "--json", "--config", cfgPath)
	var grants []protocol.Grant
	if code != 0 || json.Unmarshal([]byte(out), &grants) != nil || len(grants) != 1 {
		t.Fatalf("grants list --json: code %d, %q", code, out)
	}
	if strings.Contains(out, "beispiel-wert") {
		t.Error("the list contains a value")
	}

	// Doctor on a healthy system.
	code, out, errOut = a38Out(t, bin, "doctor", "--config", cfgPath)
	if code != 0 || !strings.Contains(out, "No findings.") || strings.Contains(out, "beispiel-wert") {
		t.Errorf("doctor: code %d, stdout:\n%s\nstderr %q", code, out, errOut)
	}
	if code, out, _ := a38Out(t, bin, "fault", "status", "--config", cfgPath); code != 0 || !strings.Contains(out, "Delivery is open.") {
		t.Errorf("fault status: code %d, %q", code, out)
	}
	if code, out, _ := a38Out(t, bin, "fault", "clear", "--config", cfgPath); code != 0 || !strings.Contains(out, "Delivery is open.") {
		t.Errorf("fault clear: code %d, %q", code, out)
	}

	// The agent socket refuses the admin messages.
	conn, err := net.Dial("unix", filepath.Join(socketDir, "agent.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	r := bufio.NewReader(conn)
	for _, line := range []string{
		`{"version":1,"type":"hello","session_name":"beispiel-admin"}`,
		`{"version":1,"type":"grants_list","id":"x-1"}`,
		`{"version":1,"type":"grant_revoke","id":"x-2","grant_id":"` + grants[0].GrantID + `"}`,
	} {
		if _, err := conn.Write([]byte(line + "\n")); err != nil {
			t.Fatal(err)
		}
		resp, err := protocol.ReadLine(r)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(line, `"hello"`) {
			continue
		}
		if !strings.Contains(string(resp), `"invalid_request"`) {
			t.Errorf("agent socket answer to %s: %s", line, resp)
		}
	}

	// Revoke: the grant is gone, and the same request needs a new approval.
	if code, out, errOut := a38Out(t, bin, "grants", "revoke", "--config", cfgPath, grants[0].GrantID); code != 0 || !strings.Contains(out, "ended") {
		t.Fatalf("grants revoke: code %d, stdout %q, stderr %q", code, out, errOut)
	}
	if code, _, errOut := a38Out(t, bin, "grants", "revoke", "--config", cfgPath, grants[0].GrantID); code != 1 || !strings.Contains(errOut, "no grant with the ID") {
		t.Errorf("second revoke: code %d, stderr %q", code, errOut)
	}
	if code, out, _ := a38Out(t, bin, "grants", "list", "--config", cfgPath); code != 0 || !strings.Contains(out, "No active grants.") {
		t.Errorf("list after revoke: code %d, %q", code, out)
	}
	request()
	_ = agent.Close()
	stop()

	// The audit log shows the revocation and two approvals.
	types := auditTypes(t, bin, cfgPath)
	var decisions, revocations int
	for _, ty := range types {
		switch ty {
		case "decision":
			decisions++
		case "revocation":
			revocations++
		}
	}
	if decisions != 2 || revocations != 1 {
		t.Errorf("audit = %v, want 2 decisions and 1 revocation", types)
	}
}
