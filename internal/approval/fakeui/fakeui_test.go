package fakeui_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/approval/fakeui"
	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/session"
)

// helperPath is the path of the compiled test helper testdata/fakescript.
var helperPath string

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	dir, err := os.MkdirTemp("", "a38-fakescript-")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	helperPath = filepath.Join(dir, "fakescript")
	out, err := exec.Command("go", "build", "-o", helperPath, "./testdata/fakescript").CombinedOutput()
	if err != nil {
		panic("build helper: " + string(out))
	}
	return m.Run()
}

// sampleRequest returns a request with three entries and a lifetime of
// one hour.
func sampleRequest() approval.Request {
	return approval.Request{
		Version:        approval.Version,
		Type:           approval.TypeRequest,
		RequestID:      "req-1",
		TimeoutSeconds: 120,
		Verified:       session.Facts{Project: "beispielprojekt"},
		Claims:         session.Claims{},
		Credentials: []approval.Credential{
			{ID: "c1", Vault: "datenbank-dev", Title: "a", Field: "password"},
			{ID: "c2", Vault: "datenbank-dev", Title: "b", Field: "password"},
			{ID: "c3", Vault: "datenbank-dev", Title: "c", Field: "password"},
		},
		Grant: approval.GrantProposal{TTLSeconds: 3600},
	}
}

// T9: the daemon does not start with a real backend and a faked approval.
func TestCheckCombination(t *testing.T) {
	kinds := []backend.Kind{backend.KindFake, backend.KindOnePassword, "other"}
	modes := []approval.Mode{approval.ModeApp, approval.ModeAutoApprove, approval.ModeAutoDeny, approval.ModeScript, "unknown"}
	for _, k := range kinds {
		for _, m := range modes {
			t.Run(string(k)+"/"+string(m), func(t *testing.T) {
				err := fakeui.CheckCombination(k.Real(), m)
				wantErr := m == "unknown" || (k.Real() && m != approval.ModeApp)
				if (err != nil) != wantErr {
					t.Fatalf("CheckCombination(%s, %s) = %v, want error %v", k, m, err, wantErr)
				}
				if k.Real() && m.Faked() && m != "unknown" && !errors.Is(err, fakeui.ErrForbiddenCombination) {
					t.Errorf("error = %v, want ErrForbiddenCombination", err)
				}
			})
		}
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		mode    approval.Mode
		opts    fakeui.Options
		wantErr error
		anyErr  bool
	}{
		{name: "app", mode: approval.ModeApp, wantErr: fakeui.ErrNotImplemented},
		{name: "auto-approve", mode: approval.ModeAutoApprove},
		{name: "auto-deny", mode: approval.ModeAutoDeny},
		{name: "script", mode: approval.ModeScript, opts: fakeui.Options{ScriptCommand: []string{"x"}, ScriptTimeout: time.Second}},
		{name: "script without command", mode: approval.ModeScript, anyErr: true},
		{name: "unknown", mode: "unknown", anyErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ui, err := fakeui.New(tt.mode, tt.opts)
			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
			case tt.anyErr:
				if err == nil {
					t.Fatal("want an error")
				}
			default:
				if err != nil || ui == nil {
					t.Fatalf("New = %v, %v", ui, err)
				}
			}
		})
	}
}
