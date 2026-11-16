package onepassword

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/token"
)

// firmaVault is the vault of the account firma in the tests.
const firmaVault = "datenbank-dev"

// newFirmaAPI returns a fake SDK of the account firma with one vault.
func newFirmaAPI() *fakeAPI {
	return &fakeAPI{
		vaults: []vaultInfo{{ID: "v-firma", Title: firmaVault}},
		items: map[string][]*fakeItem{
			"v-firma": {{id: "i-db", title: "postgres", fields: []fakeField{{id: "password", title: "password", value: "beispiel-firma-wert"}}}},
		},
		errs: map[string]error{},
	}
}

// accountsWorld builds a backend with the accounts privat and firma. Each
// account has its own fake SDK.
func accountsWorld(t *testing.T, firmaToken token.Token, firmaReason string) (*Accounts, map[string]*fakeAPI, *bytes.Buffer) {
	t.Helper()
	apis := map[string]*fakeAPI{"privat": newFakeAPI(), "firma": newFirmaAPI()}
	var logs bytes.Buffer
	b, err := newAccounts(AccountsOptions{
		Accounts: []Account{
			{Name: "privat", Vaults: []string{devVault}, Token: testToken(t)},
			{Name: "firma", Vaults: []string{firmaVault}, Token: firmaToken, Locked: firmaReason},
		},
		Version: "0.3.0",
		Logger:  slog.New(slog.NewTextHandler(&logs, nil)),
	}, func(account string) connectFunc {
		return func(context.Context, token.Token, string) (api, error) { return apis[account], nil }
	})
	if err != nil {
		t.Fatal(err)
	}
	return b, apis, &logs
}

func TestAccountsRouteByVault(t *testing.T) {
	// T45: the backend sends each reference through its vault to the client
	// of its account (D73).
	ctx := context.Background()

	t.Run("each account uses its own client", func(t *testing.T) {
		b, apis, _ := accountsWorld(t, token.New("ops_beispiel-firma-0815"), "")
		vaults, err := b.Vaults(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{devVault, firmaVault}; !reflect.DeepEqual(vaults, want) {
			t.Errorf("Vaults = %v, want %v", vaults, want)
		}
		privat := protocol.Reference{Vault: devVault, Item: "datenbank-dev", Field: "password"}
		firma := protocol.Reference{Vault: firmaVault, Item: "postgres", Field: "password"}
		for _, ref := range []protocol.Reference{privat, firma} {
			if _, err := b.Resolve(ctx, ref); err != nil {
				t.Fatalf("Resolve(%v): %v", ref, err)
			}
		}
		apis["privat"].calls, apis["firma"].calls = nil, nil
		v, err := b.Read(ctx, firma)
		if err != nil || v.Reveal() != "beispiel-firma-wert" {
			t.Fatalf("Read(firma) = %v", err)
		}
		if len(apis["privat"].calls) != 0 || len(apis["firma"].calls) != 1 {
			t.Errorf("calls privat %v, firma %v; want only one call of firma", apis["privat"].calls, apis["firma"].calls)
		}
	})

	t.Run("an account without token locks only its vaults", func(t *testing.T) {
		b, apis, logs := accountsWorld(t, token.Token{}, "the keychain has no item a38.onepassword/firma")
		// The daemon logs the locked account at its start (openAccounts);
		// the backend does not log it a second time.
		if strings.Contains(logs.String(), "has no token") {
			t.Errorf("the backend logs the locked account again: %s", logs)
		}
		_, err := b.Resolve(ctx, protocol.Reference{Vault: firmaVault, Item: "postgres", Field: "password"})
		if backend.Code(err) != protocol.CodeBackendLocked {
			t.Fatalf("Resolve(firma) = %v, want backend_locked", err)
		}
		if !strings.Contains(err.Error(), `account "firma"`) {
			t.Errorf("error %q does not name the account", err)
		}
		if len(apis["firma"].calls) != 0 {
			t.Errorf("calls of firma = %v, want none", apis["firma"].calls)
		}
		if _, err := b.Resolve(ctx, protocol.Reference{Vault: devVault, Item: "datenbank-dev", Field: "password"}); err != nil {
			t.Errorf("Resolve(privat) = %v, want success", err)
		}
		vaults, err := b.Vaults(ctx)
		if err != nil || !reflect.DeepEqual(vaults, []string{devVault}) {
			t.Errorf("Vaults = %v, %v; want only the vault of privat", vaults, err)
		}
	})

	t.Run("a rejected token locks only its account", func(t *testing.T) {
		b, apis, _ := accountsWorld(t, token.New("ops_beispiel-firma-0815"), "")
		apis["privat"].errs["vaults"] = errors.New("unauthorized: invalid service account token")
		_, err := b.Resolve(ctx, protocol.Reference{Vault: devVault, Item: "datenbank-dev", Field: "password"})
		if backend.Code(err) != protocol.CodeBackendLocked {
			t.Fatalf("Resolve(privat) = %v, want backend_locked", err)
		}
		if _, err := b.Resolve(ctx, protocol.Reference{Vault: firmaVault, Item: "postgres", Field: "password"}); err != nil {
			t.Errorf("Resolve(firma) = %v, want success", err)
		}
	})

	t.Run("a vault outside the configuration", func(t *testing.T) {
		b, apis, _ := accountsWorld(t, token.New("ops_beispiel-firma-0815"), "")
		_, err := b.Resolve(ctx, protocol.Reference{Vault: otherVault, Item: "datenbank-dev", Field: "password"})
		if backend.Code(err) != protocol.CodeNotFound {
			t.Fatalf("Resolve(other) = %v, want not_found", err)
		}
		if len(apis["privat"].calls)+len(apis["firma"].calls) != 0 {
			t.Error("a call reached an SDK")
		}
	})

	t.Run("all accounts locked", func(t *testing.T) {
		b, err := newAccounts(AccountsOptions{Accounts: []Account{{Name: "firma", Vaults: []string{firmaVault}, Locked: "no token"}}},
			func(string) connectFunc { return nil })
		if err != nil {
			t.Fatal(err)
		}
		if _, err := b.Vaults(ctx); backend.Code(err) != protocol.CodeBackendLocked {
			t.Errorf("Vaults = %v, want backend_locked", err)
		}
	})
}

func TestAccountsOptions(t *testing.T) {
	// T45 (part): the options are checked; no output shows a token.
	connect := func(string) connectFunc { return nil }
	tok := token.New(markerToken)
	for name, opts := range map[string]AccountsOptions{
		"no account":      {},
		"no vault":        {Accounts: []Account{{Name: "privat", Token: tok}}},
		"vault twice":     {Accounts: []Account{{Name: "privat", Vaults: []string{devVault}, Token: tok}, {Name: "firma", Vaults: []string{devVault}, Token: tok}}},
		"account twice":   {Accounts: []Account{{Name: "privat", Vaults: []string{devVault}, Token: tok}, {Name: "privat", Vaults: []string{firmaVault}, Token: tok}}},
		"no token reason": {Accounts: []Account{{Name: "privat", Vaults: []string{devVault}}}},
	} {
		if _, err := newAccounts(opts, connect); err == nil {
			t.Errorf("%s: newAccounts succeeded", name)
		} else if strings.Contains(err.Error(), markerToken) {
			t.Errorf("%s: the error shows the token", name)
		}
	}
	b, err := newAccounts(AccountsOptions{Accounts: []Account{{Name: "privat", Vaults: []string{devVault}, Token: tok}}}, connect)
	if err != nil {
		t.Fatal(err)
	}
	if out := fmt.Sprintf("%v %+v %#v %s", b, b, b, b); strings.Contains(out, markerToken) {
		t.Errorf("the formatted backend shows the token: %s", out)
	}
}
