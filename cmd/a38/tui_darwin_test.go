//go:build darwin

package main

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/tui"
)

func TestTUIArguments(t *testing.T) {
	// F13: the arguments of "a38 tui"; without a terminal the view does
	// not start and names the commands for scripts.
	path := demoAuditFile(t)
	tests := []struct {
		args []string
		code int
		want string
	}{
		{[]string{"tui", "--file", path, "--range", "1y"}, 2, "unknown time range"},
		{[]string{"tui", "--file", path, "--refresh", "10ms"}, 2, "--refresh must be 0 or at least 1s"},
		{[]string{"tui", "--file", path, "extra"}, 2, "unexpected argument"},
		{[]string{"tui", "--file", filepath.Join(t.TempDir(), "missing.db")}, 1, "cannot read the audit file"},
		{[]string{"tui", "--config", "/tmp/a38-missing/config.yaml"}, 1, "cannot read the configuration file"},
		{[]string{"tui", "--file", path}, 1, "needs a terminal"},
	}
	for _, tc := range tests {
		code, _, errOut := runCLI(tc.args...)
		if code != tc.code || !strings.Contains(errOut, tc.want) {
			t.Errorf("%v: code %d, stderr %q", tc.args, code, errOut)
		}
	}
}

func TestTUIAccountStates(t *testing.T) {
	// T49: a38 tui takes the state of each 1Password account from the
	// status of the daemon (D78). Another backend gives no accounts.
	st := &protocol.StatusResult{Backend: &protocol.BackendStatus{Kind: "1password", State: protocol.BackendOK, Accounts: []protocol.AccountStatus{
		{Name: "privat", State: protocol.BackendOK, VaultsConfigured: 2, VaultsVisible: 2},
		{Name: "firma", State: protocol.BackendLocked, Reason: "no token", VaultsConfigured: 1},
	}}}
	want := []tui.AccountState{
		{Name: "privat", State: "ok", Configured: 2, Visible: 2},
		{Name: "firma", State: "locked", Reason: "no token", Configured: 1},
	}
	if got := accountStatesOf(st); !reflect.DeepEqual(got, want) {
		t.Errorf("states = %+v, want %+v", got, want)
	}
	for _, st := range []*protocol.StatusResult{nil, {}, {Backend: &protocol.BackendStatus{Kind: "fake", State: protocol.BackendOK}}} {
		if got := accountStatesOf(st); len(got) != 0 {
			t.Errorf("states of %+v = %+v, want none", st, got)
		}
	}
	if _, err := tuiAccounts(filepath.Join(t.TempDir(), "admin.sock"))(context.Background()); err == nil {
		t.Error("no error without a daemon")
	}
}
