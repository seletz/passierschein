package fake_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/backend/fake"
	"github.com/seletz/passierschein/internal/protocol"
)

const marker = "KENNWERT-MARKER-4711"

const small = `
vaults:
  - name: beispiel.app.k8s-dev
    items:
      - name: datenbank-dev
        fields:
          - name: password
            value: ` + marker + `
`

func ref(v, i, f string) protocol.Reference {
	return protocol.Reference{Vault: v, Item: i, Field: f}
}

var good = ref("beispiel.app.k8s-dev", "datenbank-dev", "password")

func mustParse(t *testing.T, y string) *fake.Backend {
	t.Helper()
	b, err := fake.Parse([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

var _ backend.Backend = (*fake.Backend)(nil)

func TestLoadExample(t *testing.T) {
	b, err := fake.Load(filepath.Join("testdata", "beispiel.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	vaults, err := b.Vaults(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(vaults) != 2 || vaults[0] != "beispiel.app.k8s-dev" || vaults[1] != "beispiel.infra.k8s-dev" {
		t.Fatalf("vaults = %v", vaults)
	}
	s, err := b.Read(context.Background(), ref("beispiel.infra.k8s-dev", "datenbank-dev", "username"))
	if err != nil || s.Reveal() != "beispiel-wert-5" {
		t.Fatalf("read failed: %v", err)
	}
}

func TestVaultsIsCopy(t *testing.T) {
	b := mustParse(t, small)
	v, _ := b.Vaults(context.Background())
	v[0] = "changed"
	v2, _ := b.Vaults(context.Background())
	if v2[0] != "beispiel.app.k8s-dev" {
		t.Fatal("Vaults returns internal slice")
	}
}

func TestResolveAndRead(t *testing.T) {
	tests := []struct {
		name     string
		ref      protocol.Reference
		notFound bool
	}{
		{"found", good, false},
		{"unknown vault", ref("beispiel.x", "datenbank-dev", "password"), true},
		{"unknown item", ref("beispiel.app.k8s-dev", "x", "password"), true},
		{"unknown field", ref("beispiel.app.k8s-dev", "datenbank-dev", "x"), true},
		{"case differs", ref("Beispiel.app.k8s-dev", "datenbank-dev", "password"), true},
		{"item of other vault", ref("beispiel.app.k8s-dev", "objektspeicher-dev", "password"), true},
	}
	b := mustParse(t, small)
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := b.Resolve(ctx, tt.ref)
			_, rerr := b.Read(ctx, tt.ref)
			if tt.notFound {
				for _, e := range []error{err, rerr} {
					if !errors.Is(e, backend.ErrNotFound) || backend.Code(e) != protocol.CodeNotFound {
						t.Fatalf("want ErrNotFound, got %v", e)
					}
				}
				return
			}
			if err != nil || rerr != nil {
				t.Fatal(err, rerr)
			}
			if n != (backend.Names{Vault: "beispiel.app.k8s-dev", Item: "datenbank-dev", Field: "password"}) {
				t.Fatalf("names = %+v", n)
			}
		})
	}
}

func TestCounters(t *testing.T) {
	b := mustParse(t, small)
	ctx := context.Background()
	other := ref("beispiel.app.k8s-dev", "datenbank-dev", "x")
	if b.Resolves() != 0 || b.Reads() != 0 {
		t.Fatal("counters not zero at start")
	}
	_, _ = b.Resolve(ctx, good)
	_, _ = b.Resolve(ctx, good)
	_, _ = b.Resolve(ctx, other)
	_, _ = b.Read(ctx, good)
	_, _ = b.Vaults(ctx)
	if b.Resolves() != 3 || b.ResolvesOf(good) != 2 || b.ResolvesOf(other) != 1 {
		t.Fatalf("resolves %d %d %d", b.Resolves(), b.ResolvesOf(good), b.ResolvesOf(other))
	}
	if b.Reads() != 1 || b.ReadsOf(good) != 1 || b.ReadsOf(other) != 0 {
		t.Fatalf("reads %d %d", b.Reads(), b.ReadsOf(good))
	}
	// Resolve does not count as a read.
	_, _ = b.Resolve(ctx, good)
	if b.Reads() != 1 {
		t.Fatal("resolve counted as read")
	}
	b.ResetCounters()
	if b.Resolves() != 0 || b.Reads() != 0 || b.ReadsOf(good) != 0 {
		t.Fatal("reset failed")
	}
}

func TestErrorStates(t *testing.T) {
	tests := []struct {
		name string
		set  func(*fake.Backend)
		want error
		code protocol.Code
	}{
		{"locked", func(b *fake.Backend) { b.SetLocked(true) }, backend.ErrLocked, protocol.CodeBackendLocked},
		{"unavailable", func(b *fake.Backend) { b.SetUnavailable(true) }, backend.ErrUnavailable, protocol.CodeBackendUnavailable},
		{"both: locked wins", func(b *fake.Backend) { b.SetLocked(true); b.SetUnavailable(true) }, backend.ErrLocked, protocol.CodeBackendLocked},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { // T17 (part)
			b := mustParse(t, small)
			tt.set(b)
			ctx := context.Background()
			_, e1 := b.Vaults(ctx)
			_, e2 := b.Resolve(ctx, good)
			s, e3 := b.Read(ctx, good)
			for _, e := range []error{e1, e2, e3} {
				if !errors.Is(e, tt.want) || backend.Code(e) != tt.code {
					t.Fatalf("got %v", e)
				}
				if strings.Contains(e.Error(), marker) {
					t.Fatal("value in error")
				}
			}
			if s.Reveal() != "" {
				t.Fatal("value returned in error state")
			}
			// Clearing the state restores access.
			b.SetLocked(false)
			b.SetUnavailable(false)
			if _, err := b.Read(ctx, good); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSetValue(t *testing.T) { // T8 (part): no cache
	b := mustParse(t, small)
	ctx := context.Background()
	s, _ := b.Read(ctx, good)
	if s.Reveal() != marker {
		t.Fatal("first value")
	}
	if err := b.SetValue(good, "beispiel-neu"); err != nil {
		t.Fatal(err)
	}
	s, _ = b.Read(ctx, good)
	if s.Reveal() != "beispiel-neu" || b.Reads() != 2 {
		t.Fatalf("reads = %d", b.Reads())
	}
	err := b.SetValue(ref("a", "b", "c"), "x")
	if !errors.Is(err, backend.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestContext(t *testing.T) {
	b := mustParse(t, small)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := b.Vaults(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := b.Resolve(ctx, good); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := b.Read(ctx, good); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if b.Reads() != 0 || b.Resolves() != 0 {
		t.Fatal("canceled call counted")
	}
}

func TestNoValueInErrorsOrFormat(t *testing.T) { // T8 (part)
	b := mustParse(t, small)
	ctx := context.Background()
	_, err := b.Read(ctx, ref("beispiel.app.k8s-dev", "datenbank-dev", "nope"))
	if err == nil || strings.Contains(err.Error(), marker) {
		t.Fatalf("err = %v", err)
	}
	s, _ := b.Read(ctx, good)
	if strings.Contains(strings.Join([]string{s.String(), fmtv(s), fmtv(b)}, " "), marker) {
		t.Fatal("marker printed")
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{"malformed", "vaults: [\n  - name: x\n"},
		{"not a list", "vaults: 5\n"},
		{"unknown key", "vaults: []\nextra: 1\n"},
		{"empty file", ""},
		{"no vaults", "vaults: []\n"},
		{"empty vault name", "vaults:\n  - name: ''\n"},
		{"empty item name", "vaults:\n  - name: v\n    items:\n      - name: ''\n"},
		{"empty field name", "vaults:\n  - name: v\n    items:\n      - name: i\n        fields:\n          - name: ''\n"},
		{"duplicate vault", "vaults:\n  - name: v\n  - name: v\n"},
		{"duplicate item", "vaults:\n  - name: v\n    items:\n      - name: i\n      - name: i\n"},
		{"duplicate field", "vaults:\n  - name: v\n    items:\n      - name: i\n        fields:\n          - name: f\n            value: " + marker + "\n          - name: f\n            value: x\n"},
		{"two documents", small + "---\n" + small},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := fake.Parse([]byte(tt.yaml))
			if err == nil {
				t.Fatal("want error")
			}
			if strings.Contains(err.Error(), marker) {
				t.Fatal("value in error")
			}
		})
	}
}

func TestSameItemNameInTwoVaults(t *testing.T) {
	y := "vaults:\n  - name: a\n    items:\n      - name: i\n  - name: b\n    items:\n      - name: i\n"
	mustParse(t, y)
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := fake.Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("want error")
	}
	p := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(p, []byte("vaults: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := fake.Load(p); err == nil {
		t.Fatal("want error")
	}
}

func TestConcurrent(t *testing.T) {
	b := mustParse(t, small)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = b.Read(context.Background(), good)
			_, _ = b.Resolve(context.Background(), good)
			b.SetLocked(false)
			_ = b.SetValue(good, "x")
		}()
	}
	wg.Wait()
	if b.Reads() != 20 || b.Resolves() != 20 {
		t.Fatal(b.Reads(), b.Resolves())
	}
}
