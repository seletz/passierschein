package doctor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/token"
)

// shortDir returns a short directory under /tmp.
func shortDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "a38-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// stateFor returns a state with a configuration for the directory.
func stateFor(dir string) *State {
	return &State{Config: &config.Config{Paths: config.Paths{
		SocketDir: filepath.Join(dir, "run"), AuditFile: filepath.Join(dir, "audit.db"),
	}}}
}

func TestSocketCheckWithRealFiles(t *testing.T) {
	// T27 (part), D43: the real modes of the directory and the sockets.
	tests := []struct {
		name    string
		prepare func(t *testing.T, run string)
		wantOK  bool
		want    string
	}{
		{"missing directory", func(*testing.T, string) {}, true, "does not exist yet"},
		{"right modes", func(t *testing.T, run string) {
			mustMkdir(t, run, 0o700)
		}, true, "0700"},
		{"directory mode 0755", func(t *testing.T, run string) {
			mustMkdir(t, run, 0o755)
		}, false, "mode 0755"},
		{"socket mode 0644", func(t *testing.T, run string) {
			mustMkdir(t, run, 0o700)
			if err := os.WriteFile(filepath.Join(run, "agent.sock"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}, false, "mode 0644"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := shortDir(t)
			st := stateFor(dir)
			tt.prepare(t, st.Config.Paths.SocketDir)
			res := socketCheck{SystemEnv{}}.Run(context.Background(), st)
			if len(res) != 1 || res[0].OK != tt.wantOK {
				t.Fatalf("result = %+v", res)
			}
			if got := res[0].Detail + res[0].Cause; !strings.Contains(got, tt.want) {
				t.Errorf("text %q does not contain %q", got, tt.want)
			}
			if !tt.wantOK && res[0].NextStep == "" {
				t.Error("a finding has no next step")
			}
		})
	}
}

func mustMkdir(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func TestAuditChecksWithRealFiles(t *testing.T) {
	// T27 (part): a missing audit file, a file that is no audit log and a
	// read-only file are findings; the free space check reads the disk.
	dir := shortDir(t)
	st := stateFor(dir)
	env := SystemEnv{}
	ctx := context.Background()

	if res := (auditFileCheck{env}).Run(ctx, st); len(res) != 1 || res[0].OK || !strings.Contains(res[0].Finding, "does not exist") {
		t.Errorf("missing file: %+v", res)
	}
	if err := os.WriteFile(st.Config.Paths.AuditFile, []byte("no database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if res := (auditFileCheck{env}).Run(ctx, st); len(res) != 1 || !res[0].OK {
		t.Errorf("existing file: %+v", res)
	}
	if res := (auditChainCheck{env}).Run(ctx, st); len(res) != 1 || res[0].OK {
		t.Errorf("file that is no audit log: %+v", res)
	}
	if os.Getuid() != 0 {
		if err := os.Chmod(st.Config.Paths.AuditFile, 0o400); err != nil {
			t.Fatal(err)
		}
		if res := (auditFileCheck{env}).Run(ctx, st); len(res) != 1 || res[0].OK || !strings.Contains(res[0].Finding, "not writable") {
			t.Errorf("read-only file: %+v", res)
		}
	}
	if res := (auditSpaceCheck{env}).Run(ctx, st); len(res) != 1 || res[0].Check != "audit space" {
		t.Errorf("space: %+v", res)
	}
}

func TestSystemEnvComponents(t *testing.T) {
	// T27 (part), D48: the doctor reads the backend state and the approval
	// UI from the status of the daemon. A missing field is unknown.
	tests := []struct {
		name        string
		st          *protocol.StatusResult
		wantBackend ComponentState
		wantUI      ComponentState
	}{
		{"old daemon", &protocol.StatusResult{}, StateUnknown, StateUnknown},
		{"all ok", &protocol.StatusResult{
			Backend: &protocol.BackendStatus{State: protocol.BackendOK},
			UI:      &protocol.UIStatus{Mode: "script", Usable: true},
		}, StateOK, StateOK},
		{"backend locked", &protocol.StatusResult{
			Backend: &protocol.BackendStatus{State: protocol.BackendLocked, Reason: "The backend is locked. Unlock it."},
			UI:      &protocol.UIStatus{Mode: "app", Reason: "The daemon has no approval UI."},
		}, StateLocked, StateUnreachable},
		{"backend unreachable", &protocol.StatusResult{
			Backend: &protocol.BackendStatus{State: protocol.BackendUnreachable, Reason: "The backend does not answer."},
		}, StateUnreachable, StateUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := SystemEnv{}
			if got := env.Backend(tt.st); got.State != tt.wantBackend {
				t.Errorf("Backend = %+v, want %s", got, tt.wantBackend)
			}
			if got := env.UI(tt.st); got.State != tt.wantUI {
				t.Errorf("UI = %+v, want %s", got, tt.wantUI)
			}
		})
	}
	// The backend check names the finding and the next step.
	st := &State{Status: &protocol.StatusResult{Backend: &protocol.BackendStatus{State: protocol.BackendLocked, Reason: "The backend is locked. Unlock it."}}}
	res := backendCheck{env: SystemEnv{}}.Run(context.Background(), st)
	if len(res) != 1 || res[0].OK || !strings.Contains(res[0].Cause, "locked") || res[0].NextStep == "" {
		t.Errorf("backend check = %+v", res)
	}
}

func TestSystemEnvUIApp(t *testing.T) {
	// D6, D40: in the mode app the doctor checks the file of the approval
	// app as the daemon does. A bad file is a finding with the cause.
	dir := shortDir(t)
	write := func(name string, mode os.FileMode) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, mode); err != nil {
			t.Fatal(err)
		}
		return p
	}
	tests := []struct {
		name      string
		path      string
		wantState ComponentState
		want      string
	}{
		{"good", write("good", 0o755), StateOK, "mode app"},
		{"writable by others", write("bad", 0o757), StateUnreachable, "writable by group or others"},
		{"missing", filepath.Join(dir, "nope"), StateUnreachable, "no such file"},
		{"no path reported", "", StateUnreachable, "does not report the path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &protocol.StatusResult{UI: &protocol.UIStatus{Mode: "app", Usable: true, AppPath: tt.path}}
			got := SystemEnv{}.UI(st)
			if got.State != tt.wantState || !strings.Contains(got.Detail, tt.want) {
				t.Errorf("UI = %+v, want %s with %q", got, tt.wantState, tt.want)
			}
			res := uiCheck{SystemEnv{}}.Run(context.Background(), &State{Status: st})
			if len(res) != 1 || res[0].OK != (tt.wantState == StateOK) {
				t.Errorf("check = %+v", res)
			}
		})
	}
}

func TestTokenSourceCheckFile(t *testing.T) {
	// D51, F20: with the source file the doctor checks the token file of
	// the 1Password backend without reading it.
	const content = "ops_beispiel-kein-token"
	tests := []struct {
		name    string
		goos    string
		kind    string
		prepare func(t *testing.T, path string)
		wantOK  bool
		want    string
	}{
		{"linux", "linux", "", nil, true, "macOS build only"},
		{"fake backend", "darwin", "fake", nil, true, "backend fake"},
		{"missing, daemon down", "darwin", "", nil, true, "only a38 serve --backend 1password needs it"},
		{"missing, 1password daemon", "darwin", "1password", nil, false, "does not exist"},
		{"good file", "darwin", "1password", func(t *testing.T, p string) { writeFile(t, p, content, 0o600) }, true, "mode 0600"},
		{"mode 0644", "darwin", "", func(t *testing.T, p string) { writeFile(t, p, content, 0o644) }, false, "0644"},
		{"directory", "darwin", "1password", func(t *testing.T, p string) { mustMkdir(t, p, 0o700) }, false, "not a regular file"},
		{"symbolic link", "darwin", "1password", func(t *testing.T, p string) {
			writeFile(t, p+".real", content, 0o600)
			if err := os.Symlink(p+".real", p); err != nil {
				t.Fatal(err)
			}
		}, false, "symbolic link"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := shortDir(t)
			st := stateFor(dir)
			path := filepath.Join(dir, "onepassword-token")
			st.Config.Backend.OnePassword = config.OnePassword{TokenSource: config.TokenSourceFile, TokenFile: path}
			if tt.kind != "" {
				st.Status = &protocol.StatusResult{Backend: &protocol.BackendStatus{Kind: tt.kind, State: protocol.BackendOK}}
			}
			if tt.prepare != nil {
				tt.prepare(t, path)
			}
			res := tokenSourceCheck{SystemEnv{}, tt.goos}.Run(context.Background(), st)
			if len(res) != 1 || res[0].OK != tt.wantOK {
				t.Fatalf("result = %+v", res)
			}
			text := res[0].Detail + res[0].Finding + res[0].Cause + res[0].NextStep
			if !strings.Contains(text, tt.want) {
				t.Errorf("text %q does not contain %q", text, tt.want)
			}
			if strings.Contains(text, "kein-token") {
				t.Error("the doctor shows the content of the token file")
			}
		})
	}
}

// fakeTokenStore is a keychain item without a value: the doctor has no
// way to read a value.
type fakeTokenStore struct {
	info    token.StoreInfo
	sig     token.Signature
	descErr error
}

func (f fakeTokenStore) Name() string                        { return "a38-test-doctor/a38-test" }
func (f fakeTokenStore) Describe() (token.StoreInfo, error)  { return f.info, f.descErr }
func (f fakeTokenStore) Signature() (token.Signature, error) { return f.sig, nil }

func TestTokenSourceCheckKeychain(t *testing.T) {
	// D51, issue 59, F20: with the source keychain (the default) the
	// doctor shows the item without its value, the signature of a38 as a
	// note (G5), and a token file that is left as a finding.
	adhoc := token.Signature{Path: "/x/a38", Kind: token.SignatureAdhoc, CDHash: "abcd"}
	devID := token.Signature{Path: "/x/a38", Kind: token.SignatureDeveloperID, TeamID: "ABCDE12345"}
	item := token.StoreInfo{Exists: true, Trusted: []string{"/x/a38"}, Partitions: []string{"cdhash:abcd"}, ReadableWithoutDialog: true,
		Modified: time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)}
	type want struct {
		ok   bool
		note bool
		text string
	}
	tests := []struct {
		name     string
		kind     string
		store    TokenStore
		fileLeft bool
		want     []want
	}{
		{"no item, daemon down", "", fakeTokenStore{sig: adhoc}, false, []want{
			{true, false, "no item a38-test-doctor/a38-test; only a38 serve --backend 1password needs it"},
			{true, true, "each new build of a38 needs a confirmation"},
		}},
		{"no item, 1password daemon", "1password", fakeTokenStore{sig: adhoc}, false, []want{
			{false, false, "run a38 config onepassword-token set"},
			{true, true, "ad-hoc"},
		}},
		{"item of this build", "1password", fakeTokenStore{info: item, sig: adhoc}, false, []want{
			{true, false, "item a38-test-doctor/a38-test exists, changed 2026-11-12T08:00:00Z, trusted: /x/a38 (value not read)"},
			{true, true, "the protection against a process of the same user starts with the Developer ID signature (G5)"},
		}},
		{"item of another build", "1password", fakeTokenStore{info: func() token.StoreInfo { i := item; i.ReadableWithoutDialog = false; return i }(), sig: adhoc}, false, []want{
			{true, false, "exists"},
			{true, true, "a38 config onepassword-token set --replace instead of confirming the dialog"},
			{true, true, "ad-hoc"},
		}},
		{"item trusts security", "", fakeTokenStore{info: func() token.StoreInfo { i := item; i.Trusted = []string{"/usr/bin/security"}; return i }(), sig: adhoc}, false, []want{
			{true, false, "exists"},
			{false, false, "/usr/bin/security"},
			{true, true, "ad-hoc"},
		}},
		{"developer id", "1password", fakeTokenStore{info: item, sig: devID}, false, []want{
			{true, false, "exists"},
			{true, false, "Developer ID, team ABCDE12345"},
		}},
		{"file left", "1password", fakeTokenStore{info: item, sig: adhoc}, true, []want{
			{true, false, "exists"},
			{true, true, "ad-hoc"},
			{false, false, "rm "},
		}},
		{"describe fails", "", fakeTokenStore{descErr: errors.New("keychain: status -25294")}, false, []want{
			{false, false, "-25294"},
		}},
		{"no keychain", "", nil, false, []want{
			{false, false, "token_source: file"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := shortDir(t)
			st := stateFor(dir)
			path := filepath.Join(dir, "onepassword-token")
			st.Config.Backend.OnePassword = config.OnePassword{TokenSource: config.TokenSourceKeychain, TokenFile: path}
			if tt.kind != "" {
				st.Status = &protocol.StatusResult{Backend: &protocol.BackendStatus{Kind: tt.kind, State: protocol.BackendOK}}
			}
			if tt.fileLeft {
				writeFile(t, path, "ops_beispiel-kein-token", 0o600)
			}
			res := tokenSourceCheck{SystemEnv{Store: tt.store}, "darwin"}.Run(context.Background(), st)
			if len(res) != len(tt.want) {
				t.Fatalf("results = %+v", res)
			}
			for i, w := range tt.want {
				r := res[i]
				text := r.Detail + r.Finding + r.Cause + r.NextStep
				if r.OK != w.ok || r.Note != w.note || !strings.Contains(text, w.text) {
					t.Errorf("result %d = %+v, want ok %v, note %v, text %q", i, r, w.ok, w.note, w.text)
				}
				if strings.Contains(text, "kein-token") {
					t.Error("the doctor shows a token")
				}
			}
		})
	}
}

func TestRenderNote(t *testing.T) {
	var b strings.Builder
	Render(&b, Report{OK: true, Results: []Result{note("1password token", "signature of a38: ad-hoc")}})
	if !strings.HasPrefix(b.String(), "NOTE     1password token") || !strings.Contains(b.String(), "No findings.") {
		t.Errorf("output:\n%s", b.String())
	}
}

func TestBackendCheckOnePassword(t *testing.T) {
	// F12, F20: for the 1Password backend the doctor names the token as
	// the cause of the state locked.
	for _, state := range []string{protocol.BackendLocked, protocol.BackendUnreachable} {
		st := &State{Status: &protocol.StatusResult{Backend: &protocol.BackendStatus{Kind: "1password", State: state, Reason: "x"}}}
		res := backendCheck{SystemEnv{}}.Run(context.Background(), st)
		if len(res) != 1 || res[0].OK {
			t.Fatalf("%s: %+v", state, res)
		}
		if !strings.Contains(res[0].Cause+res[0].NextStep, "1Password") && !strings.Contains(res[0].NextStep, "token") {
			t.Errorf("%s: no hint for 1Password: %+v", state, res[0])
		}
	}
}

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}
