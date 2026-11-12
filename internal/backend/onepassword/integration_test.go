//go:build darwin && opintegration

package onepassword

// Integration test T15 against the real 1Password. It is opt-in: it runs
// only with the build tag "opintegration" and only if A38_OP_TOKEN_FILE
// names a token file with the mode 0600. Run it with
// "mise run test:onepassword".
//
// Rules (Stefan, 2026-11-12): the test touches only the vault
// passierschein.dev. It only reads. It never prints a value or the token:
// it checks only that a value is not empty, or compares its SHA-256 with
// A38_OP_EXPECT_SHA256 if that is set.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/tokenfile"
)

// testVault is the only vault that the integration test may touch.
const testVault = "passierschein.dev"

// Environment variables of the integration test.
const (
	envTokenFile   = "A38_OP_TOKEN_FILE"
	envExpectedSHA = "A38_OP_EXPECT_SHA256"
)

// The test items. The owner creates them in the vault by hand (how-to
// "1Password-Backend einrichten").
var testRefs = []protocol.Reference{
	{Vault: testVault, Item: "datenbank-dev", Field: "username"},
	{Vault: testVault, Item: "datenbank-dev", Field: "password"},
	{Vault: testVault, Item: "objektspeicher-dev", Field: "access-key"},
}

// vaultGuard wraps the SDK. It learns the ID of the test vault from the
// list of the vaults and fails the test if the backend asks for an item of
// any other vault. It records each call.
type vaultGuard struct {
	t     *testing.T
	inner api

	mu    sync.Mutex
	id    string
	calls []string
}

func (g *vaultGuard) ListVaults(ctx context.Context) ([]vaultInfo, error) {
	vs, err := g.inner.ListVaults(ctx)
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls = append(g.calls, "vaults")
	for _, v := range vs {
		if v.Title == testVault {
			g.id = v.ID
		}
	}
	return vs, err
}

func (g *vaultGuard) check(op, vaultID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls = append(g.calls, op)
	if g.id == "" || vaultID != g.id {
		g.t.Errorf("FAIL: the backend asked for %s in a vault other than %s", op, testVault)
		return errors.New("the test allows only the vault " + testVault)
	}
	return nil
}

func (g *vaultGuard) ListItems(ctx context.Context, vaultID string) ([]itemInfo, error) {
	if err := g.check("list items", vaultID); err != nil {
		return nil, err
	}
	return g.inner.ListItems(ctx, vaultID)
}

func (g *vaultGuard) GetItem(ctx context.Context, vaultID, itemID string) (itemData, error) {
	if err := g.check("get item", vaultID); err != nil {
		return itemData{}, err
	}
	return g.inner.GetItem(ctx, vaultID, itemID)
}

func (g *vaultGuard) callCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.calls)
}

// newIntegrationBackend returns the backend with only the test vault in
// the configuration, and the guard around the SDK.
func newIntegrationBackend(t *testing.T, tok tokenfile.Token) (*Backend, *vaultGuard) {
	t.Helper()
	g := &vaultGuard{t: t}
	connect := func(ctx context.Context, tok tokenfile.Token, version string) (api, error) {
		a, err := connectSDK(ctx, tok, version)
		if err != nil {
			return nil, err
		}
		g.inner = a
		return g, nil
	}
	be, err := newBackend(Options{Vaults: []string{testVault}, Token: tok, Version: "integration-test", CallTimeout: 30 * time.Second}, connect)
	if err != nil {
		t.Fatalf("FAIL: build the backend: %v", err)
	}
	return be, g
}

func TestIntegration(t *testing.T) {
	path := os.Getenv(envTokenFile)
	if path == "" {
		t.Skip("SKIP: " + envTokenFile + " is not set; the integration test does not run")
	}
	tok, err := tokenfile.Read(path)
	if err != nil {
		t.Fatalf("FAIL: token file: %v", err)
	}
	t.Logf("vault: %s (read only); token file: %s (content not shown)", testVault, path)
	be, guard := newIntegrationBackend(t, tok)
	ctx := context.Background()

	t.Run("vault_visible", func(t *testing.T) {
		vs, err := be.Vaults(ctx)
		if err != nil {
			t.Fatalf("FAIL: Vaults: %v", err)
		}
		if len(vs) != 1 || vs[0] != testVault {
			t.Fatalf("FAIL: Vaults = %v, want [%s]", vs, testVault)
		}
		t.Logf("PASS: the service account sees the vault %s", testVault)
	})

	t.Run("names_resolve", func(t *testing.T) {
		for _, ref := range testRefs {
			names, err := be.Resolve(ctx, ref)
			if err != nil {
				t.Errorf("FAIL: Resolve %s: %v", ref, err)
				continue
			}
			if names != (backend.Names{Vault: ref.Vault, Item: ref.Item, Field: ref.Field}) {
				t.Errorf("FAIL: Resolve %s gave other names", ref)
				continue
			}
			t.Logf("PASS: %s resolves", ref)
		}
	})

	t.Run("values_read", func(t *testing.T) {
		want := strings.ToLower(strings.TrimSpace(os.Getenv(envExpectedSHA)))
		for _, ref := range testRefs {
			v, err := be.Read(ctx, ref)
			if err != nil {
				t.Errorf("FAIL: Read %s: %v", ref, err)
				continue
			}
			if v.Reveal() == "" {
				t.Errorf("FAIL: Read %s gave an empty value", ref)
				continue
			}
			t.Logf("PASS: %s has a value that is not empty (value not shown)", ref)
			if want != "" && ref.Item == "datenbank-dev" && ref.Field == "password" {
				sum := sha256.Sum256([]byte(v.Reveal()))
				if hex.EncodeToString(sum[:]) != want {
					t.Errorf("FAIL: the SHA-256 of %s is not %s", ref, envExpectedSHA)
				} else {
					t.Logf("PASS: the SHA-256 of %s matches %s", ref, envExpectedSHA)
				}
			}
		}
	})

	t.Run("missing_item_not_found", func(t *testing.T) {
		ref := protocol.Reference{Vault: testVault, Item: "gibt-es-nicht-dev", Field: "password"}
		if _, err := be.Resolve(ctx, ref); backend.Code(err) != protocol.CodeNotFound {
			t.Fatalf("FAIL: Resolve %s: %v, want not_found", ref, err)
		}
		t.Log("PASS: a missing item ends with not_found")
	})

	t.Run("other_vault_not_queried", func(t *testing.T) {
		before := guard.callCount()
		ref := protocol.Reference{Vault: "beispiel.app.k8s-dev", Item: "datenbank-dev", Field: "password"}
		if _, err := be.Resolve(ctx, ref); backend.Code(err) != protocol.CodeNotFound {
			t.Fatalf("FAIL: Resolve of a vault outside the configuration: %v, want not_found", err)
		}
		if guard.callCount() != before {
			t.Fatal("FAIL: the backend called 1Password for a vault outside the configuration")
		}
		t.Log("PASS: a vault outside the configuration is not queried")
	})

	t.Run("broken_token", func(t *testing.T) {
		dir, err := os.MkdirTemp("/tmp", "a38-")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.RemoveAll(dir) }()
		bad := filepath.Join(dir, "onepassword-token")
		// A token that is not valid; it is no secret.
		if err := os.WriteFile(bad, []byte("ops_beispiel-ungueltig\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		badTok, err := tokenfile.Read(bad)
		if err != nil {
			t.Fatal(err)
		}
		be, _ := newIntegrationBackend(t, badTok)
		_, err = be.Vaults(ctx)
		code := backend.Code(err)
		if code != protocol.CodeBackendLocked && code != protocol.CodeBackendUnavailable {
			t.Fatalf("FAIL: a broken token gave %q, want backend_locked or backend_unavailable", code)
		}
		t.Logf("PASS: a broken token ends with %s", code)
	})
}
