//go:build darwin && appe2e

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/client"
	"github.com/seletz/passierschein/internal/protocol"
)

// This file runs the real daemon with the real approval app (debug build)
// over real processes. It is built only with the tags "darwin appe2e", so
// "go test ./..." never opens a window. "mise run app:e2e" builds the
// debug app and runs it (scripts/app-e2e.sh).
//
// NEVER add a case that calls the real Touch ID. Every wrapper below passes
// "--skip-auth" (fake authenticator) or ends by "--auto-close" before any
// confirmation. The windows appear for about two seconds on the screen.

// e2eWrapper writes the wrapper script that the daemon starts as the app.
// The daemon passes no argument, so the wrapper adds the test switches. It
// records what the app gets and writes, so that the test can check that
// the JSON flows both ways. It passes the file checks of the daemon: a
// regular file of the user, mode 0700.
func e2eWrapper(t *testing.T, dir, app, switches string) (path, rec string) {
	t.Helper()
	rec = filepath.Join(dir, "rec")
	if err := os.MkdirAll(rec, 0o700); err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf(`#!/bin/sh
d=%q
env | cut -d= -f1 | sort > "$d/env.txt"
cat > "$d/in.json"
%q %s < "$d/in.json" > "$d/out.txt" 2> "$d/err.txt"
code=$?
echo "$code" > "$d/code.tmp" && mv "$d/code.tmp" "$d/code.txt"
cat "$d/out.txt"
exit "$code"
`, rec, app, switches)
	path = filepath.Join(dir, "Passierschein")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path, rec
}

// waitFile waits for a file of the wrapper and returns its content.
func waitFile(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		if b, err := os.ReadFile(path); err == nil {
			return string(b)
		}
		if time.Now().After(deadline) {
			t.Fatalf("the wrapper did not write %s", path)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// readRequest reads what the app got from the daemon, strictly.
func readRequest(t *testing.T, rec string) approval.Request {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(rec, "in.json"))
	if err != nil {
		t.Fatal(err)
	}
	var req approval.Request
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		t.Fatalf("the request of the daemon: %v", err)
	}
	if req.Type != approval.TypeRequest || req.Version != approval.Version || len(req.Credentials) != 2 ||
		req.TimeoutSeconds < approval.MinTimeoutSeconds || req.TimeoutSeconds > approval.MaxTimeoutSeconds ||
		req.Verified.Project != "beispielprojekt" || req.Claims.SessionName != "beispiel-koordinator" {
		t.Fatalf("request = %+v", req)
	}
	if strings.Contains(string(data), "beispiel-wert") {
		t.Fatal("the request of the app holds a value")
	}
	return req
}

// checkEnv checks that the app got only the short environment of the
// daemon (the wrapper is the app here).
func checkEnv(t *testing.T, rec string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(rec, "env.txt"))
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{"HOME": true, "USER": true, "LOGNAME": true, "TMPDIR": true, "LANG": true,
		"LC_ALL": true, "PATH": true, "__CF_USER_TEXT_ENCODING": true,
		// The shell adds these three.
		"PWD": true, "SHLVL": true, "_": true}
	for _, name := range strings.Fields(string(data)) {
		if !allowed[name] {
			t.Errorf("the app got the variable %s", name)
		}
	}
}

// T14 (part 1, real processes), D7, D35, D40: the
// daemon starts the real debug app as a child process. The JSON flows both
// ways. A faked authentication never delivers a value.
func TestAppE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary and opens windows")
	}
	app := os.Getenv("A38_APP_DEBUG")
	if app == "" {
		t.Fatal("A38_APP_DEBUG is not set: run mise run app:e2e")
	}
	if _, err := os.Stat(app); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "a38")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	refs := []string{
		"op://beispiel.app.k8s-dev/datenbank-dev/username",
		"op://beispiel.app.k8s-dev/datenbank-dev/password",
	}
	request := func(t *testing.T, socketDir string) (*protocol.Credentials, error) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		c, err := client.DialAgent(ctx, filepath.Join(socketDir, "agent.sock"))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = c.Close() }()
		if _, err := c.Hello(ctx, "beispiel-koordinator", "coding-agent"); err != nil {
			t.Fatal(err)
		}
		var rs []protocol.Reference
		for _, r := range refs {
			ref, err := protocol.ParseReference(r)
			if err != nil {
				t.Fatal(err)
			}
			rs = append(rs, ref)
		}
		return c.RequestCredentials(ctx, &protocol.RequestCredentials{
			References: rs, Task: "Run the tests", Reason: "The tests need the login.",
		})
	}
	noGrants := func(t *testing.T, socketDir string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		c, err := client.DialAdmin(ctx, filepath.Join(socketDir, "admin.sock"))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = c.Close() }()
		g, err := c.GrantsList(ctx)
		if err != nil || len(g.Grants) != 0 {
			t.Errorf("grants = %+v, %v; want none", g, err)
		}
	}
	// serveWith starts the daemon with the wrapper as the app.
	serveWith := func(t *testing.T, cfgPath, socketDir, fakeFile, switches string) (rec string, stop func()) {
		t.Helper()
		dir, err := os.MkdirTemp("/tmp", "a38-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		wrapper, rec := e2eWrapper(t, dir, app, switches)
		stop = startServe(t, bin, cfgPath, socketDir, "--backend", "fake", "--fake-file", fakeFile, "--ui", "app", "--ui-app", wrapper)
		return rec, stop
	}

	t.Run("fake authentication is rejected", func(t *testing.T) {
		// The app approves everything with the fake authenticator (policy
		// "skipped"). The daemon must reject it: no value, no grant.
		cfgPath, socketDir, fakeFile := writeConfig(t)
		rec, stop := serveWith(t, cfgPath, socketDir, fakeFile, "--no-window --skip-auth --demo-select all --demo-confirm")
		res, err := request(t, socketDir)
		if protocol.CodeOf(err) != protocol.CodeUIFailed {
			t.Fatalf("code = %q (%v), want ui_failed", protocol.CodeOf(err), err)
		}
		if res != nil && len(res.Entries) > 0 {
			t.Fatal("a value was delivered")
		}
		if code := strings.TrimSpace(waitFile(t, filepath.Join(rec, "code.txt"))); code != "0" {
			t.Errorf("exit code of the app = %q, want 0 (approved)", code)
		}
		// The JSON flowed to the app ...
		req := readRequest(t, rec)
		checkEnv(t, rec)
		// ... and the decision flowed back: approved, both entries,
		// fake authentication.
		out, _ := os.ReadFile(filepath.Join(rec, "out.txt"))
		var d approval.Decision
		dec := json.NewDecoder(bytes.NewReader(out))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&d); err != nil {
			t.Fatalf("the decision of the app: %v", err)
		}
		if d.Outcome != approval.OutcomeApproved || d.RequestID != req.RequestID || len(d.ApprovedCredentialIDs) != 2 ||
			d.Auth == nil || d.Auth.Policy != "skipped" || !d.Auth.OK {
			t.Errorf("decision = %+v", d)
		}
		noGrants(t, socketDir)
		stop()
		out2, err := a38Cmd(context.Background(), t, bin, "audit", "list", "--config", cfgPath, "--json").Output()
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(out2), "beispiel-wert") {
			t.Error("the audit log holds a value")
		}
		types := auditTypes(t, bin, cfgPath)
		if !containsStr(types, "error") || containsStr(types, "grant") || containsStr(types, "delivery") {
			t.Errorf("audit = %v, want an error event and no grant or delivery", types)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		// The app shows its window for two seconds (a window opens on the
		// screen) and ends with the outcome timeout.
		cfgPath, socketDir, fakeFile := writeConfig(t)
		rec, stop := serveWith(t, cfgPath, socketDir, fakeFile, "--skip-auth --auto-close 2")
		res, err := request(t, socketDir)
		if protocol.CodeOf(err) != protocol.CodeTimeout {
			t.Fatalf("code = %q (%v), want timeout", protocol.CodeOf(err), err)
		}
		if res != nil && len(res.Entries) > 0 {
			t.Fatal("a value was delivered")
		}
		readRequest(t, rec)
		if code := strings.TrimSpace(waitFile(t, filepath.Join(rec, "code.txt"))); code != "11" {
			t.Errorf("exit code of the app = %q, want 11 (timeout)", code)
		}
		out, _ := os.ReadFile(filepath.Join(rec, "out.txt"))
		if !strings.Contains(string(out), `"outcome":"timeout"`) {
			t.Errorf("the app wrote %q", out)
		}
		noGrants(t, socketDir)
		stop()
		types := auditTypes(t, bin, cfgPath)
		if !containsStr(types, "expiry") || containsStr(types, "grant") {
			t.Errorf("audit = %v, want an expiry and no grant", types)
		}
	})

	t.Run("notice", func(t *testing.T) {
		// A project with an unknown vault is invalid. The first request of
		// the project gives config_invalid and a notice. The app shows the
		// notice for two seconds, writes nothing and ends with 0.
		cfgPath, socketDir, fakeFile := writeConfig(t)
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			t.Fatal(err)
		}
		data = bytes.Replace(data, []byte("vaults: [beispiel.app.k8s-dev]"), []byte("vaults: [beispiel.app.k8s-dev, fehlt.k8s-dev]"), 1)
		if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
			t.Fatal(err)
		}
		rec, _ := serveWith(t, cfgPath, socketDir, fakeFile, "--skip-auth --auto-close 2")
		res, err := request(t, socketDir)
		if protocol.CodeOf(err) != protocol.CodeConfigInvalid {
			t.Fatalf("code = %q (%v), want config_invalid", protocol.CodeOf(err), err)
		}
		if res != nil && len(res.Entries) > 0 {
			t.Fatal("a value was delivered")
		}
		if code := strings.TrimSpace(waitFile(t, filepath.Join(rec, "code.txt"))); code != "0" {
			t.Errorf("exit code of the app = %q, want 0", code)
		}
		in, _ := os.ReadFile(filepath.Join(rec, "in.json"))
		var n approval.Notice
		dec := json.NewDecoder(bytes.NewReader(in))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&n); err != nil {
			t.Fatalf("the notice of the daemon: %v", err)
		}
		if n.Type != approval.TypeNotice || n.Kind != approval.NoticeProjectConfigInvalid || n.Project != "beispielprojekt" {
			t.Errorf("notice = %+v", n)
		}
		if out, _ := os.ReadFile(filepath.Join(rec, "out.txt")); len(out) != 0 {
			t.Errorf("the app wrote %q for a notice", out)
		}
		noGrants(t, socketDir)
	})
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
