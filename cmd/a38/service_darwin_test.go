//go:build darwin

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/doctor"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/service"
	"github.com/seletz/passierschein/internal/service/fakelaunchctl"
	"github.com/seletz/passierschein/internal/token"
)

func init() {
	// Issue 70: no test of this package asks the real launchd. The init
	// function runs before TestMain.
	newServiceState = func(context.Context) *doctor.ServiceState { return nil }
}

// serviceWorld is a temporary HOME with a fake bundle, a configuration
// and a fake launchctl.
type serviceWorld struct {
	home, self, app, cfgPath string
	lc                       *fakelaunchctl.Fake
	item                     token.StoreInfo
	daemon                   *protocol.StatusResult
}

func newServiceWorld(t *testing.T) *serviceWorld {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	short, err := os.MkdirTemp("/tmp", "a38-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(short) })
	w := &serviceWorld{home: filepath.Join(root, "home"), lc: fakelaunchctl.New(),
		item: token.StoreInfo{Exists: true, ReadableWithoutDialog: true}}
	macos := filepath.Join(root, "Applications", "Passierschein.app", "Contents", "MacOS")
	for _, p := range []string{w.home, macos} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	w.self, w.app = filepath.Join(macos, "a38"), filepath.Join(macos, "Passierschein")
	for _, p := range []string{w.self, w.app} {
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	w.cfgPath = filepath.Join(root, "config.yaml")
	cfg := "version: 1\nvaults:\n  - {name: beispiel.app.k8s-dev, environment: dev}\npaths:\n  socket_dir: " + short + "/run\n  audit_file: " + short + "/audit.db\n"
	if err := os.WriteFile(w.cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.EnvConfig, "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("SSH_CONNECTION", "")
	return w
}

func (w *serviceWorld) deps() serviceDeps {
	return serviceDeps{
		manager: func() (service.Manager, error) {
			return service.Manager{Layout: service.Layout{Home: w.home, UID: 501}, Launchctl: w.lc, Self: w.self}, nil
		},
		probe: func() service.Probe {
			return service.Probe{
				LoadConfig:    config.Load,
				AppPath:       func(*config.Config) string { return w.app },
				TokenItem:     func() (token.StoreInfo, error) { return w.item, nil },
				DaemonAnswers: func(context.Context, string) bool { return false },
				UID:           os.Getuid(),
			}
		},
		daemon: func(_ context.Context, path string) (*protocol.StatusResult, string, error) {
			if w.daemon == nil {
				return nil, "/tmp/a38-x/run/admin.sock", errors.New("connection refused")
			}
			return w.daemon, "/tmp/a38-x/run/admin.sock", nil
		},
	}
}

func (w *serviceWorld) run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := runServiceWith(args, &out, &errOut, w.deps())
	return code, out.String(), errOut.String()
}

func (w *serviceWorld) plist(t *testing.T) service.Plist {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(w.home, "Library", "LaunchAgents", service.Label+".plist"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := service.ParsePlist(data)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestServiceInstallStatusUninstall(t *testing.T) {
	// F15, UC11 step 4 and 6: install, status, uninstall against a fake
	// launchctl and a temporary HOME.
	w := newServiceWorld(t)
	code, out, errOut := w.run(t, "install", "--config", w.cfgPath)
	if code != exitOK {
		t.Fatalf("install: %d\n%s%s", code, out, errOut)
	}
	if !strings.Contains(out, "Installed the LaunchAgent") || !strings.Contains(out, "Login Items") {
		t.Errorf("install output:\n%s", out)
	}
	p := w.plist(t)
	want := []string{w.self, "serve", "--backend", "1password", "--ui", "app", "--config", w.cfgPath}
	if !slices.Equal(p.ProgramArguments, want) {
		t.Errorf("ProgramArguments = %q, want %q", p.ProgramArguments, want)
	}

	w.daemon = &protocol.StatusResult{DaemonVersion: "0.1.0", Sessions: 1}
	code, out, _ = w.run(t, "status")
	if code != exitOK || !strings.Contains(out, "loaded, running, pid") || !strings.Contains(out, "the same a38") || !strings.Contains(out, "delivery open") {
		t.Errorf("status: %d\n%s", code, out)
	}

	w.daemon = &protocol.StatusResult{DaemonVersion: "0.1.0", Fault: &protocol.Fault{Code: protocol.CodeAuditFailed}}
	if _, out, _ = w.run(t, "status"); !strings.Contains(out, "delivery LOCKED (audit_failed)") {
		// D48: the daemon stays up with the lock; status shows it.
		t.Errorf("status with lock:\n%s", out)
	}

	code, out, _ = w.run(t, "status", "--json")
	var js serviceStatusJSON
	if err := json.Unmarshal([]byte(out), &js); err != nil || code != exitOK || !js.OK || !js.Daemon.Answers || js.Service.PID == 0 {
		t.Errorf("status --json: %d %v\n%s", code, err, out)
	}

	code, out, _ = w.run(t, "uninstall")
	if code != exitOK || !strings.Contains(out, "Removed") || !strings.Contains(out, "stays") {
		t.Errorf("uninstall: %d\n%s", code, out)
	}
	code, out, _ = w.run(t, "status")
	if code != exitFailure || !strings.Contains(out, "not installed") {
		t.Errorf("status after uninstall: %d\n%s", code, out)
	}
	code, out, _ = w.run(t, "uninstall")
	if code != exitOK || !strings.Contains(out, "not installed") {
		t.Errorf("second uninstall: %d\n%s", code, out)
	}
}

func TestServiceInstallConfigFromEnvironment(t *testing.T) {
	// D49: launchd does not pass the shell environment. A38_CONFIG goes
	// into the plist as --config, the XDG variables into
	// EnvironmentVariables. Without them the plist names no --config.
	w := newServiceWorld(t)
	t.Setenv(config.EnvConfig, w.cfgPath)
	t.Setenv("XDG_STATE_HOME", "/tmp/a38-state")
	if code, out, errOut := w.run(t, "install"); code != exitOK {
		t.Fatalf("install: %d\n%s%s", code, out, errOut)
	}
	p := w.plist(t)
	if p.Arg("--config") != w.cfgPath {
		t.Errorf("--config = %q", p.Arg("--config"))
	}
	env, _ := p.Values["EnvironmentVariables"].(map[string]any)
	if env["XDG_STATE_HOME"] != "/tmp/a38-state" {
		t.Errorf("EnvironmentVariables = %v", env)
	}
}

func TestServiceInstallRefusals(t *testing.T) {
	// F15: the preflight refusals reach the human with a next step, and
	// the exit code is 1. A wrong command line is exit code 2.
	tests := []struct {
		name    string
		args    func(w *serviceWorld) []string
		prepare func(t *testing.T, w *serviceWorld)
		code    int
		want    string
	}{
		{"no keychain item", func(w *serviceWorld) []string { return []string{"install", "--config", w.cfgPath} },
			func(_ *testing.T, w *serviceWorld) { w.item.Exists = false }, exitFailure, "a38 config onepassword-token set"},
		{"no approval app", func(w *serviceWorld) []string { return []string{"install", "--config", w.cfgPath} },
			func(t *testing.T, w *serviceWorld) {
				if err := os.Remove(w.app); err != nil {
					t.Fatal(err)
				}
			}, exitFailure, "approval app"},
		{"configuration does not load", func(w *serviceWorld) []string {
			return []string{"install", "--config", filepath.Join(filepath.Dir(w.cfgPath), "none.yaml")}
		}, func(*testing.T, *serviceWorld) {}, exitFailure, "does not load"},
		{"real backend with fake UI (D40)", func(w *serviceWorld) []string {
			return []string{"install", "--config", w.cfgPath, "--ui", "auto-approve"}
		}, func(*testing.T, *serviceWorld) {}, exitUsage, "needs --ui app"},
		{"positional argument", func(*serviceWorld) []string { return []string{"install", "x"} },
			func(*testing.T, *serviceWorld) {}, exitUsage, "unexpected argument"},
		{"unknown subcommand", func(*serviceWorld) []string { return []string{"start"} },
			func(*testing.T, *serviceWorld) {}, exitUsage, "unknown subcommand"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newServiceWorld(t)
			tt.prepare(t, w)
			code, out, errOut := w.run(t, tt.args(w)...)
			if code != tt.code || !strings.Contains(out+errOut, tt.want) {
				t.Errorf("code %d, want %d; output does not contain %q:\n%s%s", code, tt.code, tt.want, out, errOut)
			}
			if got := w.lc.Changing(); len(got) != 0 {
				t.Errorf("launchctl calls: %q", got)
			}
			if _, err := os.Stat(filepath.Join(w.home, "Library", "LaunchAgents")); err == nil {
				t.Error("the refusal created ~/Library/LaunchAgents")
			}
		})
	}
}

func TestServiceInstallDryRunAndFakeBackend(t *testing.T) {
	// The fake backend needs the explicit flags; --dry-run prints the
	// plist and writes nothing.
	w := newServiceWorld(t)
	fake := filepath.Join(filepath.Dir(w.cfgPath), "fake.yaml")
	if err := os.WriteFile(fake, []byte("vaults: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := w.run(t, "install", "--config", w.cfgPath, "--backend", "fake", "--ui", "auto-approve", "--fake-file", fake, "--dry-run")
	if code != exitOK || !strings.Contains(out, "<string>auto-approve</string>") || !strings.Contains(out, "Dry run") {
		t.Fatalf("dry run: %d\n%s%s", code, out, errOut)
	}
	if len(w.lc.Changing()) != 0 {
		t.Errorf("launchctl calls: %q", w.lc.Changing())
	}
}

func TestServiceStatusWithFailedStart(t *testing.T) {
	// A start error stays: launchd does not restart the daemon. status
	// shows the last exit and the log.
	w := newServiceWorld(t)
	w.lc.StartFails = true
	if code, out, errOut := w.run(t, "install", "--config", w.cfgPath); code != exitOK {
		t.Fatalf("install: %d\n%s%s", code, out, errOut)
	}
	code, out, _ := w.run(t, "status")
	if code != exitFailure || !strings.Contains(out, "not running") || !strings.Contains(out, "exit code 1") || !strings.Contains(out, "serve.log") {
		t.Errorf("status: %d\n%s", code, out)
	}
	if code, out, _ := w.run(t, "restart"); code != exitOK || !strings.Contains(out, "Restarted") {
		t.Errorf("restart: %d\n%s", code, out)
	}
}

func TestServeExitCodesDoNotRestart(t *testing.T) {
	// F15, D48: the exit codes of a38 serve under the plist of the
	// service. A configuration error ends with 1, a wrong command line
	// with 2; launchd restarts neither.
	p, err := service.ParsePlist(service.Spec{Program: "/x/a38", LogPath: "/x/log"}.Render())
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(bad, []byte("version: 1\nvaults:\n  - name: beispiel.app.k8s-dev\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		args []string
		code int
	}{
		{"configuration error", []string{"--config", bad, "--backend", "fake", "--fake-file", bad, "--ui", "auto-approve"}, exitFailure},
		{"missing configuration", []string{"--config", filepath.Join(dir, "none.yaml"), "--backend", "fake", "--fake-file", bad, "--ui", "auto-approve"}, exitFailure},
		{"wrong command line", []string{"--backend", "1password", "--ui", "auto-approve"}, exitUsage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := serve(context.Background(), tt.args, &out, &errOut)
			if code != tt.code {
				t.Fatalf("exit code %d, want %d: %s", code, tt.code, errOut.String())
			}
			if service.Restarts(p, service.Exit{Code: code}) {
				t.Errorf("launchd would restart the daemon after exit code %d", code)
			}
		})
	}
}

func TestServiceDoctorStateWithoutPlist(t *testing.T) {
	// doctorState passes the fields that the check of the doctor needs.
	s := doctorState(service.Status{Installed: true, Loaded: true, LastExitMeaning: "exit code 1: x", LogPath: "/x/serve.log", Program: "/x/a38", ProgramDiffers: true, Current: "/y/a38"})
	if !s.Installed || !s.Loaded || s.LastExit != "exit code 1: x" || !s.ProgramDiffers || s.Current != "/y/a38" {
		t.Errorf("state = %+v", s)
	}
}
