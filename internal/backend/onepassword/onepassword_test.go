package onepassword

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
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

// testToken returns a token with the marker, read from a 0600 file.
func testToken(t *testing.T) tokenfile.Token {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "a38-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "token")
	if err := os.WriteFile(path, []byte(markerToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tok, err := tokenfile.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// Markers: no error, no log line and no formatted backend may contain
// them. The values are fake.
const (
	markerValue  = "beispiel-wert-geheim-4711"
	markerToken  = "ops_beispiel-token-0815"
	markerSDKMsg = "sdk-text-mit-beispiel-wert-geheim-4711"
	devVault     = "beispiel.app.k8s-dev"
	otherVault   = "fremder.vault"
)

// fakeItem is one item of the fake SDK.
type fakeItem struct {
	id, title string
	fields    []fakeField
}

type fakeField struct {
	id, title, value string
}

// fakeAPI is a fake of the SDK. It records each call.
type fakeAPI struct {
	mu     sync.Mutex
	vaults []vaultInfo
	items  map[string][]*fakeItem // by vault ID
	// errs maps an operation ("vaults", "items", "get") to an error.
	errs map[string]error
	// block makes each call wait until the context ends.
	block bool
	calls []string
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{
		vaults: []vaultInfo{{ID: "v-dev", Title: devVault}, {ID: "v-other", Title: otherVault}},
		items: map[string][]*fakeItem{
			"v-dev": {
				{id: "i-db", title: "datenbank-dev", fields: []fakeField{
					{id: "username", title: "username", value: "beispiel-nutzer"},
					{id: "password", title: "password", value: markerValue},
				}},
				{id: "i-s3", title: "objektspeicher-dev", fields: []fakeField{
					{id: "f-ak", title: "access-key", value: "beispiel-schluessel"},
				}},
			},
			"v-other": {{id: "i-x", title: "datenbank-dev", fields: []fakeField{{id: "password", title: "password", value: "x"}}}},
		},
		errs: map[string]error{},
	}
}

func (f *fakeAPI) record(ctx context.Context, op string) error {
	f.mu.Lock()
	f.calls = append(f.calls, op)
	err, block := f.errs[strings.Fields(op)[0]], f.block
	f.mu.Unlock()
	if block {
		<-ctx.Done()
		return ctx.Err()
	}
	return err
}

func (f *fakeAPI) ListVaults(ctx context.Context) ([]vaultInfo, error) {
	if err := f.record(ctx, "vaults"); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]vaultInfo(nil), f.vaults...), nil
}

func (f *fakeAPI) ListItems(ctx context.Context, vaultID string) ([]itemInfo, error) {
	if err := f.record(ctx, "items "+vaultID); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []itemInfo
	for _, it := range f.items[vaultID] {
		out = append(out, itemInfo{ID: it.id, Title: it.title})
	}
	return out, nil
}

func (f *fakeAPI) GetItem(ctx context.Context, vaultID, itemID string) (itemData, error) {
	if err := f.record(ctx, "get "+vaultID+" "+itemID); err != nil {
		return itemData{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, it := range f.items[vaultID] {
		if it.id != itemID {
			continue
		}
		d := itemData{ID: it.id, Title: it.title, VaultID: vaultID}
		for _, fl := range it.fields {
			d.Fields = append(d.Fields, fieldInfo{ID: fl.id, Title: fl.title, Value: protocol.NewSecret(fl.value)})
		}
		return d, nil
	}
	return itemData{}, errors.New("item not found")
}

func (f *fakeAPI) setErr(op string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errs[op] = err
}

func (f *fakeAPI) callList() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

// harness builds a backend with the fake SDK.
type harness struct {
	be       *Backend
	api      *fakeAPI
	log      *bytes.Buffer
	connects int
	now      time.Time
	connErr  error
	mu       sync.Mutex
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{api: newFakeAPI(), log: &bytes.Buffer{}, now: time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)}
	connect := func(_ context.Context, tok tokenfile.Token, _ string) (api, error) {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.connects++
		if tok.Reveal() != markerToken {
			t.Error("connect got another token")
		}
		if h.connErr != nil {
			return nil, h.connErr
		}
		return h.api, nil
	}
	be, err := newBackend(Options{
		Vaults:      []string{devVault, "beispiel.app.k8s-fehlt"},
		Token:       testToken(t),
		Logger:      slog.New(slog.NewTextHandler(h.log, nil)),
		CallTimeout: 2 * time.Second,
		RetryAfter:  30 * time.Second,
		Now:         h.clock,
	}, connect)
	if err != nil {
		t.Fatal(err)
	}
	h.be = be
	return h
}

func (h *harness) clock() time.Time {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.now
}

func (h *harness) advance(d time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.now = h.now.Add(d)
}

// noLeak checks that no output contains a marker.
func noLeak(t *testing.T, outs ...string) {
	t.Helper()
	for _, out := range outs {
		for _, m := range []string{markerValue, markerToken, markerSDKMsg, "beispiel-token"} {
			if strings.Contains(out, m) {
				t.Errorf("output contains a marker %q: %q", m, out)
			}
		}
	}
}

func ref(vault, item, field string) protocol.Reference {
	return protocol.Reference{Vault: vault, Item: item, Field: field}
}

func TestResolveAndRead(t *testing.T) {
	// F12, D27: names as 1Password writes them; the value only from Read.
	h := newHarness(t)
	ctx := context.Background()
	tests := []struct {
		ref  protocol.Reference
		want string
	}{
		{ref(devVault, "datenbank-dev", "username"), "beispiel-nutzer"},
		{ref(devVault, "datenbank-dev", "password"), markerValue},
		// A custom field in a section: the label is unique in the item.
		{ref(devVault, "objektspeicher-dev", "access-key"), "beispiel-schluessel"},
	}
	for _, tt := range tests {
		names, err := h.be.Resolve(ctx, tt.ref)
		if err != nil {
			t.Fatalf("Resolve(%v): %v", tt.ref, err)
		}
		if names != (backend.Names{Vault: tt.ref.Vault, Item: tt.ref.Item, Field: tt.ref.Field}) {
			t.Errorf("Resolve(%v) = %+v", tt.ref, names)
		}
		v, err := h.be.Read(ctx, tt.ref)
		if err != nil {
			t.Fatalf("Read(%v): %v", tt.ref, err)
		}
		if v.Reveal() != tt.want {
			t.Errorf("Read(%v) gave the wrong value", tt.ref)
		}
	}
	noLeak(t, h.log.String(), fmt.Sprintf("%v %+v %#v", h.be, h.be, h.be))
}

func TestVaultsOnlyFromConfiguration(t *testing.T) {
	// D12: a vault that the service account sees but the configuration
	// does not list does not exist for a38. A listed vault that the
	// service account does not see is missing from the list.
	h := newHarness(t)
	got, err := h.be.Vaults(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != devVault {
		t.Errorf("Vaults = %v, want [%s]", got, devVault)
	}
}

func TestNotFound(t *testing.T) {
	// D27, T24 (part), G23: an unknown item or field in an allowed vault is
	// not_found. Item and field names are compared exactly, like the fake
	// backend does. A vault outside the configuration is never queried.
	tests := []struct {
		name string
		ref  protocol.Reference
		// wantCalls is the list of SDK calls; nil means no call.
		wantCalls []string
	}{
		{"unknown item", ref(devVault, "gibt-es-nicht", "password"), []string{"vaults", "items v-dev"}},
		{"item in another spelling", ref(devVault, "Datenbank-Dev", "password"), []string{"vaults", "items v-dev"}},
		{"item ID instead of title", ref(devVault, "i-db", "password"), []string{"vaults", "items v-dev"}},
		{"unknown field", ref(devVault, "datenbank-dev", "pin"), []string{"vaults", "items v-dev", "get v-dev i-db"}},
		{"field in another spelling", ref(devVault, "datenbank-dev", "Password"), []string{"vaults", "items v-dev", "get v-dev i-db"}},
		{"vault in the configuration, not visible", ref("beispiel.app.k8s-fehlt", "datenbank-dev", "password"), []string{"vaults"}},
		{"vault outside the configuration", ref(otherVault, "datenbank-dev", "password"), nil},
		{"vault in another spelling", ref("Beispiel.App.K8s-Dev", "datenbank-dev", "password"), nil},
		// G25: a reference with an attribute is out of scope.
		{"attribute", ref(devVault, "datenbank-dev", "password?attribute=otp"), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			_, err := h.be.Resolve(context.Background(), tt.ref)
			if !errors.Is(err, backend.ErrNotFound) || backend.Code(err) != protocol.CodeNotFound {
				t.Fatalf("Resolve error = %v, want not_found", err)
			}
			if got := h.api.callList(); fmt.Sprint(got) != fmt.Sprint(tt.wantCalls) {
				t.Errorf("SDK calls = %v, want %v", got, tt.wantCalls)
			}
			for _, c := range h.api.callList() {
				if strings.Contains(c, "v-other") {
					t.Errorf("the backend queried a vault outside the configuration: %s", c)
				}
			}
			if tt.wantCalls == nil && h.connects != 0 {
				t.Error("the backend connected for a reference it rejects itself")
			}
			if _, err := h.be.Read(context.Background(), tt.ref); err == nil {
				t.Error("Read of a reference that did not resolve gave a value")
			}
		})
	}
	h := newHarness(t)
	_, err := h.be.Resolve(context.Background(), ref(devVault, "datenbank-dev", "password?attribute=otp"))
	if err == nil || !strings.Contains(err.Error(), "attribute") {
		t.Errorf("attribute error = %v, want a clear text", err)
	}
}

func TestDuplicateTitlesFailClosed(t *testing.T) {
	// G30: two items, two fields or two vaults with the same title end
	// with backend_unavailable, not with not_found: the reference exists,
	// but the backend cannot say which entry the human approves.
	tests := []struct {
		name    string
		prepare func(f *fakeAPI)
		ref     protocol.Reference
	}{
		{"two items", func(f *fakeAPI) {
			f.items["v-dev"] = append(f.items["v-dev"], &fakeItem{id: "i-db2", title: "datenbank-dev", fields: []fakeField{{id: "password", title: "password", value: "y"}}})
		}, ref(devVault, "datenbank-dev", "password")},
		{"two fields", func(f *fakeAPI) {
			it := f.items["v-dev"][1]
			it.fields = append(it.fields, fakeField{id: "f-ak2", title: "access-key", value: "z"})
		}, ref(devVault, "objektspeicher-dev", "access-key")},
		{"two vaults", func(f *fakeAPI) {
			f.vaults = append(f.vaults, vaultInfo{ID: "v-dev2", Title: devVault})
		}, ref(devVault, "datenbank-dev", "password")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			tt.prepare(h.api)
			_, err := h.be.Resolve(context.Background(), tt.ref)
			if backend.Code(err) != protocol.CodeBackendUnavailable || errors.Is(err, backend.ErrNotFound) {
				t.Fatalf("error = %v, want backend_unavailable", err)
			}
			if !strings.Contains(err.Error(), "not unique") {
				t.Errorf("error %q does not say that the title is not unique", err)
			}
			if !strings.Contains(h.log.String(), "not unique") {
				t.Errorf("log does not name the finding: %s", h.log.String())
			}
			noLeak(t, err.Error(), h.log.String())
		})
	}
}

func TestReadByID(t *testing.T) {
	// G30: Read uses the IDs that Resolve found. A renamed item still
	// gives the approved item; a new item with the old title does not
	// replace it. A second Resolve that finds another item fails closed.
	h := newHarness(t)
	ctx := context.Background()
	r := ref(devVault, "datenbank-dev", "password")
	if _, err := h.be.Resolve(ctx, r); err != nil {
		t.Fatal(err)
	}
	h.api.mu.Lock()
	h.api.items["v-dev"][0].title = "datenbank-alt"
	h.api.items["v-dev"] = append(h.api.items["v-dev"], &fakeItem{id: "i-new", title: "datenbank-dev",
		fields: []fakeField{{id: "password", title: "password", value: "beispiel-anderes-item"}}})
	h.api.mu.Unlock()
	v, err := h.be.Read(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if v.Reveal() != markerValue {
		t.Error("Read did not read the approved item")
	}
	calls := h.api.callList()
	if last := calls[len(calls)-1]; last != "get v-dev i-db" {
		t.Errorf("last call = %q, want the read by ID", last)
	}
	_, err = h.be.Resolve(ctx, r)
	if backend.Code(err) != protocol.CodeBackendUnavailable || !strings.Contains(err.Error(), "changed") {
		t.Errorf("second Resolve error = %v, want backend_unavailable (changed)", err)
	}
}

func TestReadWithoutResolveAndDeletedItem(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	r := ref(devVault, "datenbank-dev", "password")
	if _, err := h.be.Read(ctx, r); backend.Code(err) != protocol.CodeBackendUnavailable {
		t.Errorf("Read without Resolve: %v, want backend_unavailable", err)
	}
	if len(h.api.callList()) != 0 {
		t.Error("Read without Resolve called the SDK")
	}
	if _, err := h.be.Resolve(ctx, r); err != nil {
		t.Fatal(err)
	}
	h.api.mu.Lock()
	h.api.items["v-dev"] = h.api.items["v-dev"][1:]
	h.api.mu.Unlock()
	if _, err := h.be.Read(ctx, r); !errors.Is(err, backend.ErrNotFound) {
		t.Errorf("Read of a deleted item: %v, want not_found", err)
	}
	// A field that is gone from the approved item is not_found too.
	h2 := newHarness(t)
	if _, err := h2.be.Resolve(ctx, r); err != nil {
		t.Fatal(err)
	}
	h2.api.mu.Lock()
	h2.api.items["v-dev"][0].fields = h2.api.items["v-dev"][0].fields[:1]
	h2.api.mu.Unlock()
	if _, err := h2.be.Read(ctx, r); !errors.Is(err, backend.ErrNotFound) {
		t.Errorf("Read of a deleted field: %v, want not_found", err)
	}
}

func TestReadIsNeverCached(t *testing.T) {
	// D34: each Read asks the SDK; a changed value shows at once.
	h := newHarness(t)
	ctx := context.Background()
	r := ref(devVault, "datenbank-dev", "username")
	if _, err := h.be.Resolve(ctx, r); err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"beispiel-nutzer", "beispiel-nutzer-neu"} {
		if i == 1 {
			h.api.mu.Lock()
			h.api.items["v-dev"][0].fields[0].value = want
			h.api.mu.Unlock()
		}
		v, err := h.be.Read(ctx, r)
		if err != nil || v.Reveal() != want {
			t.Errorf("read %d: wrong value or error %v", i, err)
		}
	}
	gets := 0
	for _, c := range h.api.callList() {
		if strings.HasPrefix(c, "get ") {
			gets++
		}
	}
	if gets != 3 { // one for Resolve, two for Read
		t.Errorf("GetItem calls = %d, want 3", gets)
	}
}

func TestErrorMapping(t *testing.T) {
	// F12, D35: each SDK error maps to backend_locked, backend_unavailable
	// or not_found; an unknown error is backend_unavailable (fail closed).
	// No error and no log line repeats the text of the SDK.
	tests := []struct {
		name string
		op   string
		err  error
		want protocol.Code
	}{
		{"token rejected", "vaults", fmt.Errorf("Unauthorized: invalid service account token %s", markerSDKMsg), protocol.CodeBackendLocked},
		{"authentication", "vaults", errors.New("error initializing client: authentication failed " + markerSDKMsg), protocol.CodeBackendLocked},
		{"token expired", "items", errors.New("service account token has expired " + markerSDKMsg), protocol.CodeBackendLocked},
		{"network", "vaults", errors.New("dial tcp: lookup my.1password.com: no such host " + markerSDKMsg), protocol.CodeBackendUnavailable},
		{"rate limit", "items", fmt.Errorf("%w: %s", errRateLimit, markerSDKMsg), protocol.CodeBackendUnavailable},
		{"unknown", "get", errors.New("something strange " + markerSDKMsg), protocol.CodeBackendUnavailable},
		{"item gone", "get", errors.New("item not found " + markerSDKMsg), protocol.CodeNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.api.setErr(tt.op, tt.err)
			_, err := h.be.Resolve(context.Background(), ref(devVault, "datenbank-dev", "password"))
			if got := backend.Code(err); got != tt.want {
				t.Errorf("code = %q (%v), want %q", got, err, tt.want)
			}
			noLeak(t, err.Error(), fmt.Sprintf("%+v", err), h.log.String())
		})
	}
}

func TestStateErrorsOfVaults(t *testing.T) {
	// The status of the daemon calls Vaults: one SDK call, no value.
	tests := []struct {
		err  error
		want protocol.Code
	}{
		{nil, ""},
		{errors.New("unauthorized"), protocol.CodeBackendLocked},
		{errors.New("connection refused"), protocol.CodeBackendUnavailable},
	}
	for _, tt := range tests {
		h := newHarness(t)
		h.api.setErr("vaults", tt.err)
		_, err := h.be.Vaults(context.Background())
		if backend.Code(err) != tt.want {
			t.Errorf("Vaults with %v: %v, want %q", tt.err, err, tt.want)
		}
		if calls := h.api.callList(); len(calls) != 1 || calls[0] != "vaults" {
			t.Errorf("calls = %v, want one list of the vaults", calls)
		}
	}
}

func TestTimeout(t *testing.T) {
	// F12: each SDK call has a time limit. A call that hangs ends with
	// backend_unavailable; the end of the request context ends it at once
	// with the error of the context.
	h := newHarness(t)
	h.be.timeout = 50 * time.Millisecond
	h.api.block = true
	start := time.Now()
	_, err := h.be.Resolve(context.Background(), ref(devVault, "datenbank-dev", "password"))
	if backend.Code(err) != protocol.CodeBackendUnavailable || !strings.Contains(err.Error(), "time") {
		t.Errorf("error = %v, want backend_unavailable (time limit)", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Error("the call did not end at the time limit")
	}

	h.be.timeout = time.Minute
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	_, err = h.be.Vaults(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error with a canceled context = %v, want context.Canceled", err)
	}
}

func TestConnectIsLazyAndRetries(t *testing.T) {
	// Start with a bad token or without network (working assumption of
	// stage 3): the backend does not connect at New; each call fails with
	// a stable code; the backend tries again after RetryAfter, not at
	// each call.
	h := newHarness(t)
	if h.connects != 0 {
		t.Fatal("New connected to 1Password")
	}
	h.connErr = errors.New("invalid service account token " + markerSDKMsg)
	ctx := context.Background()
	for range 3 {
		_, err := h.be.Vaults(ctx)
		if backend.Code(err) != protocol.CodeBackendLocked {
			t.Fatalf("error = %v, want backend_locked", err)
		}
		noLeak(t, err.Error())
	}
	if h.connects != 1 {
		t.Errorf("connects = %d, want 1 inside the retry interval", h.connects)
	}
	h.connErr = nil
	h.advance(31 * time.Second)
	if _, err := h.be.Vaults(ctx); err != nil {
		t.Fatalf("after the retry interval: %v", err)
	}
	if h.connects != 2 {
		t.Errorf("connects = %d, want 2", h.connects)
	}
	noLeak(t, h.log.String())
}

func TestNewChecksOptions(t *testing.T) {
	if _, err := New(Options{Vaults: []string{devVault}}); err == nil {
		t.Error("New without a token gave no error")
	}
	if _, err := New(Options{Token: testToken(t)}); err == nil {
		t.Error("New without vaults gave no error")
	}
}

func TestBackendNeverPrintsToken(t *testing.T) {
	h := newHarness(t)
	var buf bytes.Buffer
	slog.New(slog.NewTextHandler(&buf, nil)).Info("x", "backend", h.be)
	noLeak(t, fmt.Sprint(h.be), fmt.Sprintf("%#v", h.be), buf.String())
}
