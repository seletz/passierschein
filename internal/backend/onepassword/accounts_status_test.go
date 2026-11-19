package onepassword

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/token"
)

// accountSpec is one account of statusWorld.
type accountSpec struct {
	name   string
	vault  string
	locked string // the account has no token
	hang   bool   // each SDK call waits until its context ends
	reject bool   // 1Password rejects the token
}

// statusWorld builds a backend with one fake SDK for each account.
func statusWorld(t *testing.T, specs ...accountSpec) *Accounts {
	t.Helper()
	apis := map[string]*fakeAPI{}
	var accounts []Account
	for _, s := range specs {
		api := &fakeAPI{vaults: []vaultInfo{{ID: "v-" + s.name, Title: s.vault}}, items: map[string][]*fakeItem{}, errs: map[string]error{}, block: s.hang}
		if s.reject {
			api.errs["vaults"] = errors.New("unauthorized: invalid service account token")
		}
		apis[s.name] = api
		acc := Account{Name: s.name, Vaults: []string{s.vault}, Locked: s.locked}
		if s.locked == "" {
			acc.Token = token.New("ops_beispiel-" + s.name)
		}
		accounts = append(accounts, acc)
	}
	b, err := newAccounts(AccountsOptions{Accounts: accounts, Version: "0.3.0", Logger: slog.New(slog.DiscardHandler)},
		func(account string) connectFunc {
			return func(context.Context, token.Token, string) (api, error) { return apis[account], nil }
		})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

var (
	healthy = accountSpec{name: "privat", vault: devVault}
	hanging = accountSpec{name: "firma", vault: firmaVault, hang: true}
	locked  = accountSpec{name: "labor", vault: "beispielprojekt", locked: "the keychain has no item a38-test-status/labor"}
)

func TestAccountsCheckEachAccount(t *testing.T) {
	// T49: the check asks each account side by side, each with its own
	// time limit. A hanging or locked account does not change the state
	// of a healthy account (D78).
	const limit = 300 * time.Millisecond
	tests := []struct {
		name  string
		specs []accountSpec
		want  map[string]protocol.Code // "" means the account works
	}{
		{"hanging account first", []accountSpec{hanging, healthy}, map[string]protocol.Code{"firma": protocol.CodeBackendUnavailable, "privat": ""}},
		{"locked account beside a healthy account", []accountSpec{locked, healthy}, map[string]protocol.Code{"labor": protocol.CodeBackendLocked, "privat": ""}},
		{"hanging, locked and healthy", []accountSpec{hanging, locked, healthy}, map[string]protocol.Code{"firma": protocol.CodeBackendUnavailable, "labor": protocol.CodeBackendLocked, "privat": ""}},
		{"rejected token", []accountSpec{{name: "firma", vault: firmaVault, reject: true}, healthy}, map[string]protocol.Code{"firma": protocol.CodeBackendLocked, "privat": ""}},
		{"two hanging accounts", []accountSpec{hanging, {name: "labor", vault: "beispielprojekt", hang: true}, healthy}, map[string]protocol.Code{"firma": protocol.CodeBackendUnavailable, "labor": protocol.CodeBackendUnavailable, "privat": ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := statusWorld(t, tt.specs...)
			start := time.Now()
			states := b.Check(context.Background(), limit)
			// Side by side: two hanging accounts take one time limit, not two.
			if d := time.Since(start); d > limit+limit/2 {
				t.Errorf("Check took %s, want less than %s", d, limit+limit/2)
			}
			var names []string
			for _, st := range states {
				names = append(names, st.Name)
				if st.Configured != 1 {
					t.Errorf("%s: configured = %d, want 1", st.Name, st.Configured)
				}
				want := tt.want[st.Name]
				if got := backend.Code(st.Err); got != want {
					t.Errorf("%s: code = %q (%v), want %q", st.Name, got, st.Err, want)
				}
				if want == "" && st.Visible != 1 {
					t.Errorf("%s: visible = %d, want 1", st.Name, st.Visible)
				}
				if st.Err != nil && strings.Contains(st.Err.Error(), "ops_") {
					t.Errorf("%s: the error shows a token: %v", st.Name, st.Err)
				}
			}
			var order []string
			for _, s := range tt.specs {
				order = append(order, s.name)
			}
			if !reflect.DeepEqual(names, order) {
				t.Errorf("order = %v, want the order of the configuration %v", names, order)
			}
		})
	}

	t.Run("a hanging account names the time limit", func(t *testing.T) {
		b := statusWorld(t, hanging)
		st := b.Check(context.Background(), 20*time.Millisecond)[0]
		if st.Err == nil || !strings.Contains(st.Err.Error(), "20ms") {
			t.Errorf("err = %v, want the time limit in the text", st.Err)
		}
	})
}

func TestAccountsVaultsSideBySide(t *testing.T) {
	// T49: a slow account does not use up the time of the other accounts.
	// Vaults returns the vaults of each account that answers in time.
	b := statusWorld(t, hanging, locked, healthy)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	vaults, err := b.Vaults(ctx)
	if err != nil {
		t.Fatalf("Vaults = %v", err)
	}
	if !reflect.DeepEqual(vaults, []string{devVault}) {
		t.Errorf("Vaults = %v, want only the vault of privat", vaults)
	}

	t.Run("no account answers in time", func(t *testing.T) {
		b := statusWorld(t, hanging)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		if _, err := b.Vaults(ctx); !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Vaults = %v, want the error of the context", err)
		}
	})
}
