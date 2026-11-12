package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/doctor"
	"github.com/seletz/passierschein/internal/protocol"
)

// markerValue stands for a secret value. No output may contain it.
const markerValue = "beispiel-wert-marker"

// fakeInfo is a fake file.
type fakeInfo struct {
	fs.FileInfo
	mode fs.FileMode
}

func (f fakeInfo) Mode() fs.FileMode { return f.mode }
func (f fakeInfo) Sys() any          { return nil }
func (f fakeInfo) IsDir() bool       { return f.mode.IsDir() }

// fakeEnv is a fake environment of the doctor. The zero value after
// healthyEnv is a healthy system.
type fakeEnv struct {
	cfg        *config.Config
	cfgErr     error
	dirMode    fs.FileMode
	statusErr  error
	status     *protocol.StatusResult
	writeErr   error
	free       uint64
	verify     audit.Verification
	verifyErr  error
	backend    doctor.Component
	marker     string // not used by any check; it must never appear
	auditExist bool
	tokenStore *fakeStore
}

func healthyEnv(t *testing.T) *fakeEnv {
	t.Helper()
	project := t.TempDir() // exists, so the path check finds nothing
	cfg, err := config.Parse([]byte(`version: 1
vaults:
  - {name: beispiel.app.k8s-dev, environment: dev}
projects:
  - name: beispielprojekt
    paths: [` + project + `]
    vaults: [beispiel.app.k8s-dev]
paths:
  socket_dir: /tmp/a38-doctor/run
  audit_file: /tmp/a38-doctor/audit.db
backend:
  onepassword:
    token_file: /tmp/a38-doctor/onepassword-token
`))
	if err != nil {
		t.Fatal(err)
	}
	return &fakeEnv{
		cfg:        cfg,
		dirMode:    fs.ModeDir | 0o700,
		status:     &protocol.StatusResult{DaemonVersion: "v0.0.0-test", Sessions: 1, Grants: 2},
		free:       10 << 30,
		verify:     audit.Verification{OK: true, Entries: 7},
		backend:    doctor.Component{State: doctor.StateOK},
		marker:     markerValue,
		auditExist: true,
	}
}

func (e *fakeEnv) LoadConfig() (*config.Config, error) { return e.cfg, e.cfgErr }
func (e *fakeEnv) Lstat(path string) (fs.FileInfo, error) {
	switch {
	case strings.HasSuffix(path, ".sock"):
		return fakeInfo{mode: fs.ModeSocket | 0o600}, nil
	case strings.HasSuffix(path, "onepassword-token"):
		return nil, fs.ErrNotExist
	case strings.HasSuffix(path, "audit.db"):
		if !e.auditExist {
			return nil, fs.ErrNotExist
		}
		return fakeInfo{mode: 0o600}, nil
	}
	return fakeInfo{mode: e.dirMode}, nil
}
func (e *fakeEnv) Writable(string) error            { return e.writeErr }
func (e *fakeEnv) FreeBytes(string) (uint64, error) { return e.free, nil }
func (e *fakeEnv) Status(context.Context, string) (*protocol.StatusResult, error) {
	return e.status, e.statusErr
}
func (e *fakeEnv) VerifyAudit(context.Context, string) (audit.Verification, error) {
	return e.verify, e.verifyErr
}
func (e *fakeEnv) Backend(*protocol.StatusResult) doctor.Component { return e.backend }
func (e *fakeEnv) TokenStore() doctor.TokenStore {
	if e.tokenStore == nil {
		e.tokenStore = newFakeStore()
	}
	return e.tokenStore
}
func (e *fakeEnv) UI(*protocol.StatusResult) doctor.Component {
	return doctor.Component{State: doctor.StateUnknown, Detail: "not reported"}
}

func TestDoctor(t *testing.T) {
	// T27: each state names the finding, the cause and the next step. The
	// state without a fault ends with 0. No output holds a value.
	tests := []struct {
		name     string
		change   func(e *fakeEnv)
		wantCode int
		want     []string // finding, cause, next step
	}{
		{"all ok", func(*fakeEnv) {}, 0, nil},
		{"daemon not running", func(e *fakeEnv) {
			e.status, e.statusErr = nil, errors.New("connection refused")
		}, 1, []string{"the daemon does not answer", "connection refused", "a38 serve"}},
		{"audit file not writable", func(e *fakeEnv) {
			e.writeErr = errors.New("permission denied")
		}, 1, []string{"the audit file is not writable", "permission denied", "fix the rights"}},
		{"audit chain broken", func(e *fakeEnv) {
			e.verify = audit.Verification{FirstBadSeq: 4, Finding: "the hash does not fit"}
		}, 1, []string{"the hash chain of the audit log is broken", "entry 4: the hash does not fit", "a38 audit list"}},
		{"project with invalid configuration", func(e *fakeEnv) {
			e.cfg.Projects[0].Invalid = "the vault beispiel.app.k8s-prod is not in the vault list"
		}, 1, []string{`the configuration of the project "beispielprojekt" is invalid`, "beispiel.app.k8s-prod is not in the vault list", "restart the daemon"}},
		{"project path does not exist", func(e *fakeEnv) {
			e.cfg.Projects[0].MissingPaths = []string{"/tmp/beispielprojekt-fehlt"}
		}, 1, []string{`the project "beispielprojekt" has a path that does not exist`, "/tmp/beispielprojekt-fehlt", "create the directory or fix the path"}},
		{"backend locked", func(e *fakeEnv) {
			e.backend = doctor.Component{State: doctor.StateLocked, Detail: "the vault app is locked"}
		}, 1, []string{"backend is locked", "the vault app is locked", "unlock"}},
		{"socket directory with wrong mode", func(e *fakeEnv) {
			e.dirMode = fs.ModeDir | 0o755
		}, 1, []string{"the socket directory has the wrong mode", "mode 0755", "chmod 700"}},
		{"delivery locked", func(e *fakeEnv) {
			e.status.Fault = &protocol.Fault{Code: protocol.CodeAuditFailed, Reason: "the audit log is not writable", Since: time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}
		}, 1, []string{"delivery is locked for all sessions since 2026-11-11T10:00:00Z", "audit_failed: the audit log is not writable", "a38 fault clear"}},
		{"delivery locked without cause", func(e *fakeEnv) {
			e.status.Fault = &protocol.Fault{}
		}, 1, []string{"delivery is locked for all sessions", "does not name the cause", "a38 fault clear"}},
		{"little free space", func(e *fakeEnv) { e.free = 1 << 20 }, 1, []string{"little free space", "1 MiB are free", "free disk space"}},
		{"configuration does not load", func(e *fakeEnv) {
			e.cfgErr = errors.New("line 3: unknown key")
		}, 1, []string{"the configuration does not load", "line 3: unknown key", "fix the configuration"}},
		// Issue 59, D51: the keychain is the default source of the token.
		{"keychain item missing with 1password daemon", func(e *fakeEnv) {
			e.status.Backend = &protocol.BackendStatus{Kind: "1password", State: protocol.BackendOK}
		}, 1, []string{"the keychain has no token of the 1Password backend", "no item a38-test-fake/a38-test", "run a38 config onepassword-token set"}},
		{"keychain item present with 1password daemon", func(e *fakeEnv) {
			e.status.Backend = &protocol.BackendStatus{Kind: "1password", State: protocol.BackendOK}
			v := markerValue
			e.tokenStore = newFakeStore()
			e.tokenStore.value = &v
		}, 0, nil},
		{"project only the daemon rejects", func(e *fakeEnv) {
			e.status.InvalidProjects = []protocol.InvalidProject{{Project: "anderes-projekt", Reason: "no vault"}}
		}, 1, []string{`the daemon rejects the project "anderes-projekt"`, "no vault", "restart the daemon"}},
	}
	for _, tt := range tests {
		for _, asJSON := range []bool{false, true} {
			name := tt.name
			args := []string{}
			if asJSON {
				name += " json"
				args = append(args, "--json")
			}
			t.Run(name, func(t *testing.T) {
				if strings.HasPrefix(tt.name, "keychain") && runtime.GOOS != "darwin" {
					t.Skip("the token check is part of the macOS build only")
				}
				e := healthyEnv(t)
				tt.change(e)
				var stdout, stderr bytes.Buffer
				code := runDoctorWith(args, &stdout, &stderr, func(string) doctor.Env { return e })
				if code != tt.wantCode {
					t.Errorf("exit code = %d, want %d; output:\n%s", code, tt.wantCode, stdout.String())
				}
				out := stdout.String()
				if strings.Contains(out, markerValue) || strings.Contains(stderr.String(), markerValue) {
					t.Error("the output contains a value")
				}
				if asJSON {
					var rep doctor.Report
					if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
						t.Fatalf("invalid JSON: %v\n%s", err, out)
					}
					if rep.OK != (tt.wantCode == 0) {
						t.Errorf("ok = %v", rep.OK)
					}
					// Compare the decoded text: JSON escapes quotes.
					out = ""
					for _, r := range rep.Results {
						out += r.Finding + "\n" + r.Cause + "\n" + r.NextStep + "\n"
					}
				}
				if tt.wantCode == 0 && !strings.Contains(out, "No findings.") && !asJSON {
					t.Errorf("output:\n%s", out)
				}
				for _, w := range tt.want {
					if !strings.Contains(out, w) {
						t.Errorf("output does not contain %q:\n%s", w, out)
					}
				}
			})
		}
	}
}

func TestDoctorTokenNote(t *testing.T) {
	// Issue 59: the ad-hoc signature of a38 is a note, not a finding (G5).
	if runtime.GOOS != "darwin" {
		t.Skip("the token check is part of the macOS build only")
	}
	e := healthyEnv(t)
	v := markerValue
	e.tokenStore = newFakeStore()
	e.tokenStore.value = &v
	var stdout, stderr bytes.Buffer
	code := runDoctorWith(nil, &stdout, &stderr, func(string) doctor.Env { return e })
	out := stdout.String()
	if code != 0 || !strings.Contains(out, "NOTE     1password token") || !strings.Contains(out, "each new build of a38 needs a confirmation") {
		t.Errorf("code %d, output:\n%s", code, out)
	}
	if !strings.Contains(out, "item a38-test-fake/a38-test exists") || strings.Contains(out, markerValue) {
		t.Errorf("output:\n%s", out)
	}
	if e.tokenStore.reads != 0 {
		t.Error("the doctor read the token")
	}
}
