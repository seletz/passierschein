package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/token"
)

// tokenMarker stands for a token. No output may contain it.
const tokenMarker = "ops_beispiel-token-marker-5c1e"

func TestMain(m *testing.M) {
	// Issue 59: no test of this package touches the keychain. A test that
	// needs a store installs a fake one.
	newTokenStore = func(string) (token.Store, error) {
		return nil, errors.New("test: no token store installed")
	}
	os.Exit(m.Run())
}

// fakeStore is a token store in memory for one account.
type fakeStore struct {
	account  string
	mu       sync.Mutex
	value    *string
	readable bool
	trusted  []string
	sig      token.Signature
	getErr   error
	putErr   error
	block    chan struct{} // if set, Token waits for it
	reads    int
}

// newFakeStore returns an empty store of the account privat.
func newFakeStore() *fakeStore { return newFakeStoreFor("privat") }

// newFakeStoreFor returns an empty store of the account.
func newFakeStoreFor(account string) *fakeStore {
	return &fakeStore{account: account, readable: true, sig: token.Signature{Path: "/x/a38", Kind: token.SignatureAdhoc, CDHash: "abcd"}}
}

func (f *fakeStore) Name() string { return "a38-test-fake/" + f.account }
func (f *fakeStore) Token() (token.Token, error) {
	if f.block != nil {
		<-f.block
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	if f.getErr != nil {
		return token.Token{}, f.getErr
	}
	if f.value == nil {
		return token.Token{}, fmt.Errorf("fake: %w", token.ErrNotFound)
	}
	return token.New(*f.value), nil
}
func (f *fakeStore) Put(t token.Token, replace bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.putErr != nil {
		return f.putErr
	}
	if f.value != nil && !replace {
		return fmt.Errorf("fake: %w", token.ErrExists)
	}
	v := t.Reveal()
	f.value = &v
	return nil
}
func (f *fakeStore) Delete() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.value == nil {
		return fmt.Errorf("fake: %w", token.ErrNotFound)
	}
	f.value = nil
	return nil
}
func (f *fakeStore) Describe() (token.StoreInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	info := token.StoreInfo{Name: f.Name(), Exists: f.value != nil}
	if info.Exists {
		info.Created = time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
		info.Modified = info.Created
		info.Trusted = f.trusted
		if info.Trusted == nil {
			info.Trusted = []string{f.sig.Path}
		}
		info.Partitions = []string{"cdhash:" + f.sig.CDHash}
		info.ReadableWithoutDialog = f.readable
	}
	return info, nil
}
func (f *fakeStore) Signature() (token.Signature, error) { return f.sig, nil }

// tokenEnv runs the token commands with a fake store, a configuration
// with the given source and a token file in a temporary directory.
type tokenEnv struct {
	// store is the item of the account privat; stores holds each item by
	// account, also the item of the configuration with one token.
	store    *fakeStore
	stores   map[string]*fakeStore
	cfgDir   string
	cfgPath  string
	file     string
	terminal bool
	typed    string
	prompts  int
}

func newTokenEnv(t *testing.T, source config.TokenSource) *tokenEnv {
	t.Helper()
	dir := t.TempDir()
	// The token file of the configuration with one token lies below
	// XDG_CONFIG_HOME; the test never looks at the one of the human.
	t.Setenv("XDG_CONFIG_HOME", dir)
	e := &tokenEnv{store: newFakeStore(), cfgDir: dir, cfgPath: filepath.Join(dir, "config.yaml"), file: filepath.Join(dir, "onepassword-token-privat"), terminal: true}
	e.stores = map[string]*fakeStore{"privat": e.store}
	data := fmt.Sprintf("version: 1\nvaults:\n  - {name: beispiel.app.k8s-dev, environment: dev, account: privat}\n"+
		"backend:\n  onepassword:\n    accounts:\n      - name: privat\n        token_source: %s\n        token_file: %s\n", source, e.file)
	if err := os.WriteFile(e.cfgPath, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return e
}

// storeOf returns the item of the account and creates it if necessary.
func (e *tokenEnv) storeOf(account string) *fakeStore {
	if e.stores[account] == nil {
		e.stores[account] = newFakeStoreFor(account)
	}
	return e.stores[account]
}

func (e *tokenEnv) run(args ...string) (int, string, string) {
	deps := tokenDeps{
		terminal: e.terminal,
		readSecret: func(string) ([]byte, error) {
			e.prompts++
			return []byte(e.typed), nil
		},
		newStore: func(account string) (token.Store, error) { return e.storeOf(account), nil },
		lstat:    os.Lstat,
	}
	if len(args) > 0 {
		args = append(args, "--config", e.cfgPath)
	}
	var o, er bytes.Buffer
	code := runTokenWith(args, &o, &er, deps)
	return code, o.String(), er.String()
}

// noToken fails if an output contains the marker or a part of it.
func noToken(t *testing.T, outs ...string) {
	t.Helper()
	for _, out := range outs {
		if strings.Contains(out, tokenMarker) || strings.Contains(out, "token-marker") {
			t.Errorf("output contains the token: %q", out)
		}
	}
}

// noDerived fails if an output shows a value derived from the token: its
// length or its SHA-256 (Stefan, 2026-11-12: only metadata).
func noDerived(t *testing.T, outs ...string) {
	t.Helper()
	sum := sha256.Sum256([]byte(tokenMarker))
	fp := hex.EncodeToString(sum[:])
	for _, out := range outs {
		for _, bad := range []string{fp[:8], fmt.Sprintf("%d bytes", len(tokenMarker)), "fingerprint", "length"} {
			if strings.Contains(strings.ToLower(out), bad) {
				t.Errorf("output shows %q: %q", bad, out)
			}
		}
	}
}

func TestTokenSet(t *testing.T) {
	// Issue 59, D51: "set" reads the token only from the terminal, checks
	// the form, stores, and prints that it stored, nothing derived.
	tests := []struct {
		name       string
		existing   bool
		typed      string
		args       []string
		wantCode   int
		wantOut    string
		wantErr    string
		wantStored string
		wantPrompt bool
	}{
		{"terminal", false, tokenMarker, nil, 0, `Stored the token of the account "privat" in the keychain item a38-test-fake/privat.`, "", tokenMarker, true},
		{"empty", false, "", nil, 1, "", "the token is empty", "", true},
		{"inner space", false, "ops_beispiel token-marker-5c1e", nil, 1, "", "white space", "", true},
		{"control character", false, "ops_beispiel\x1btoken-marker-5c1e", nil, 1, "", "control character", "", true},
		{"too short", false, "ops_kurz", nil, 1, "", "too short", "", true},
		{"too long", false, "ops_" + strings.Repeat("a", token.MaxLength), nil, 1, "", "too long", "", true},
		{"no prefix warns", false, "beispiel-token-marker-5c1e", nil, 0, "Stored", "warning", "beispiel-token-marker-5c1e", true},
		{"existing without replace", true, tokenMarker, nil, 1, "", "exists already; nothing changed", "ops_beispiel-alt-0000000000", false},
		{"existing with replace", true, tokenMarker, []string{"--replace"}, 0, "Stored", "", tokenMarker, true},
		{"argument", false, tokenMarker, []string{tokenMarker}, 2, "", "never an argument", "", false},
		{"unknown flag", false, tokenMarker, []string{"--value"}, 2, "", "flag provided but not defined", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTokenEnv(t, config.TokenSourceKeychain)
			if tt.existing {
				old := "ops_beispiel-alt-0000000000"
				e.store.value = &old
			}
			e.typed = tt.typed
			code, out, errOut := e.run(append([]string{"set", "--account", "privat"}, tt.args...)...)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d; stderr %q", code, tt.wantCode, errOut)
			}
			if !strings.Contains(out, tt.wantOut) || (tt.wantOut == "" && out != "") {
				t.Errorf("stdout = %q, want %q", out, tt.wantOut)
			}
			if !strings.Contains(errOut, tt.wantErr) {
				t.Errorf("stderr = %q, want %q", errOut, tt.wantErr)
			}
			got := ""
			if e.store.value != nil {
				got = *e.store.value
			}
			if got != tt.wantStored {
				t.Error("the stored value is not the expected one")
			}
			if (e.prompts > 0) != tt.wantPrompt {
				t.Errorf("prompted %d times, want a prompt: %v", e.prompts, tt.wantPrompt)
			}
			noToken(t, out, errOut)
			noDerived(t, out, errOut)
		})
	}
}

func TestTokenSetRefusesNonTerminal(t *testing.T) {
	// Stefan, 2026-11-12: the token comes only from the terminal. Without
	// a terminal the command reads nothing and ends with 2.
	e := newTokenEnv(t, config.TokenSourceKeychain)
	e.terminal, e.typed = false, tokenMarker
	code, out, errOut := e.run("set", "--account", "privat")
	if code != 2 || !strings.Contains(errOut, "standard input is not a terminal") || out != "" {
		t.Errorf("code %d, stdout %q, stderr %q", code, out, errOut)
	}
	if e.prompts != 0 || e.store.value != nil {
		t.Error("the command read or stored a token without a terminal")
	}
}

func TestTokenSetNotesFileSource(t *testing.T) {
	e := newTokenEnv(t, config.TokenSourceFile)
	e.typed = tokenMarker
	code, out, errOut := e.run("set", "--account", "privat")
	if code != 0 || !strings.Contains(errOut, "token_source: file") {
		t.Errorf("code %d, stderr %q", code, errOut)
	}
	noToken(t, out, errOut)
}

func TestTokenSetStoreError(t *testing.T) {
	e := newTokenEnv(t, config.TokenSourceKeychain)
	e.store.putErr = errors.New("keychain: SecItemAdd: status -25308 (User interaction is not allowed.)")
	e.typed = tokenMarker
	code, out, errOut := e.run("set", "--account", "privat")
	if code != 1 || !strings.Contains(errOut, "nothing stored") || !strings.Contains(errOut, "-25308") {
		t.Errorf("code %d, stderr %q", code, errOut)
	}
	noToken(t, out, errOut)
}

func TestTokenStatus(t *testing.T) {
	// Issue 59, Stefan 2026-11-12: status shows only metadata: the source,
	// the item without its value, the signature and a token file that is
	// left. It never reads the token, so it never causes a dialog.
	tests := []struct {
		name     string
		source   config.TokenSource
		prepare  func(t *testing.T, e *tokenEnv)
		wantCode int
		want     []string
	}{
		{"no item", config.TokenSourceKeychain, func(*testing.T, *tokenEnv) {}, 1,
			[]string{"Account privat: vaults beispiel.app.k8s-dev", "Source:      keychain", "no item a38-test-fake/privat", "a38 config onepassword-token set --account privat", "Signature:   ad-hoc", "each new build of a38 needs a confirmation", "does not exist"}},
		{"item of this build", config.TokenSourceKeychain, func(_ *testing.T, e *tokenEnv) { v := tokenMarker; e.store.value = &v }, 0,
			[]string{"item a38-test-fake/privat exists (value not read)", "2026-11-12T08:00:00Z", "Trusted:   /x/a38", "cdhash:abcd"}},
		{"item of another build", config.TokenSourceKeychain, func(_ *testing.T, e *tokenEnv) {
			v := tokenMarker
			e.store.value, e.store.readable = &v, false
		}, 0, []string{"exists", "this build of a38 did not store the item", "set --account privat --replace"}},
		{"item trusts another program", config.TokenSourceKeychain, func(_ *testing.T, e *tokenEnv) {
			v := tokenMarker
			e.store.value, e.store.readable, e.store.trusted = &v, false, []string{"/usr/bin/security"}
		}, 0, []string{"trusts a program other than this a38: /usr/bin/security"}},
		{"developer id", config.TokenSourceKeychain, func(_ *testing.T, e *tokenEnv) {
			e.store.sig = token.Signature{Path: "/x/a38", Kind: token.SignatureDeveloperID, TeamID: "ABCDE12345"}
		}, 1, []string{"developer-id, team ABCDE12345"}},
		{"file left", config.TokenSourceKeychain, func(t *testing.T, e *tokenEnv) {
			v := tokenMarker
			e.store.value = &v
			if err := os.WriteFile(e.file, []byte(tokenMarker+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, 0, []string{"exists (content not read)", "left although the source is the keychain"}},
		{"file source", config.TokenSourceFile, func(t *testing.T, e *tokenEnv) {
			if err := os.WriteFile(e.file, []byte(tokenMarker+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, 0, []string{"Source:      file", "no item", "exists (content not read)"}},
		{"file source with mode 0644", config.TokenSourceFile, func(t *testing.T, e *tokenEnv) {
			writeMode(t, e.file, 0o644)
		}, 1, []string{"Finding:", "0600"}},
		{"file source without file", config.TokenSourceFile, func(*testing.T, *tokenEnv) {}, 1, []string{"does not exist"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTokenEnv(t, tt.source)
			tt.prepare(t, e)
			code, out, errOut := e.run("status")
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d; stdout:\n%s\nstderr: %s", code, tt.wantCode, out, errOut)
			}
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("stdout does not contain %q:\n%s", w, out)
				}
			}
			if e.store.reads != 0 {
				t.Errorf("status read the token %d times", e.store.reads)
			}
			noToken(t, out, errOut)
			noDerived(t, out, errOut)

			// --json gives the same facts, also without a value.
			code2, js, _ := e.run("status", "--json")
			if code2 != code {
				t.Errorf("--json: exit code %d, want %d", code2, code)
			}
			var st tokenStatus
			if err := json.Unmarshal([]byte(js), &st); err != nil {
				t.Fatalf("--json: %v\n%s", err, js)
			}
			if len(st.Accounts) != 1 || st.Accounts[0].Source != tt.source || st.OK != (code == 0) {
				t.Errorf("--json: %+v", st)
			}
			if e.store.reads != 0 {
				t.Error("status --json read the token")
			}
			noToken(t, js)
			noDerived(t, js)
		})
	}
}

func TestTokenStatusWithoutConfiguration(t *testing.T) {
	e := newTokenEnv(t, config.TokenSourceKeychain)
	e.cfgPath = "/tmp/a38-missing/config.yaml"
	code, out, _ := e.run("status")
	if code != 1 || !strings.Contains(out, "does not load") {
		t.Errorf("code %d, stdout:\n%s", code, out)
	}
}

func TestTokenDelete(t *testing.T) {
	// Issue 59: delete removes the item and says what the daemon does.
	tests := []struct {
		name     string
		source   config.TokenSource
		existing bool
		wantCode int
		wantOut  string
		wantErr  string
	}{
		{"keychain source", config.TokenSourceKeychain, true, 0, "the vaults of the account \"privat\" are locked until you run \"a38 config onepassword-token set --account privat\"", ""},
		{"file source", config.TokenSourceFile, true, 0, "uses its token file", ""},
		{"no item", config.TokenSourceKeychain, false, 1, "", "has no item"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTokenEnv(t, tt.source)
			if tt.existing {
				v := tokenMarker
				e.store.value = &v
			}
			code, out, errOut := e.run("delete", "--account", "privat")
			if code != tt.wantCode || !strings.Contains(out, tt.wantOut) || !strings.Contains(errOut, tt.wantErr) {
				t.Errorf("code %d, stdout %q, stderr %q", code, out, errOut)
			}
			if e.store.value != nil {
				t.Error("the item is still there")
			}
			noToken(t, out, errOut)
		})
	}
	e := newTokenEnv(t, config.TokenSourceKeychain)
	if code, _, errOut := e.run("delete", "--account", "privat", "now"); code != 2 || !strings.Contains(errOut, "unexpected argument") {
		t.Errorf("argument: code %d, stderr %q", code, errOut)
	}
}

func TestTokenUsage(t *testing.T) {
	e := newTokenEnv(t, config.TokenSourceKeychain)
	if code, out, _ := e.run("--help"); code != 0 || !strings.Contains(out, "a38 config onepassword-token set") {
		t.Errorf("--help: %d %q", code, out)
	}
	var o, er bytes.Buffer
	if code := runTokenWith([]string{"rotate"}, &o, &er, tokenDeps{}); code != 2 || !strings.Contains(er.String(), "unknown subcommand") {
		t.Errorf("unknown: %d %q", code, er.String())
	}
}

func TestTokenStoreUnavailable(t *testing.T) {
	// D42, D53: without a store (linux) each command says so.
	for _, sub := range []string{"set", "status", "delete"} {
		deps := tokenDeps{
			terminal: true,
			newStore: func(string) (token.Store, error) { return nil, fmt.Errorf("the keychain is %w", token.ErrUnavailable) },
			lstat:    os.Lstat,
		}
		var o, er bytes.Buffer
		if code := runTokenWith([]string{sub, "--account", "privat", "--config", "/tmp/a38-missing/config.yaml"}, &o, &er, deps); code != 1 || !strings.Contains(er.String(), "not available on this platform") && !strings.Contains(er.String(), "does not load") {
			t.Errorf("%s: code %d, stderr %q", sub, code, er.String())
		}
	}
}

func TestKeychainSource(t *testing.T) {
	// D51, issue 59: the daemon reads the keychain item once at its
	// start. A missing item stops the start and names "a38 config onepassword-token set".
	v := tokenMarker
	tests := []struct {
		name    string
		store   func() *fakeStore
		timeout time.Duration
		want    string
	}{
		{"item", func() *fakeStore { s := newFakeStore(); s.value = &v; return s }, time.Second, ""},
		{"no item", newFakeStore, time.Second, `has no item a38-test-fake/privat (token_source: keychain, the default); store the token with "a38 config onepassword-token set --account privat", or set token_source: file for the account`},
		{"interaction", func() *fakeStore {
			s := newFakeStore()
			s.getErr = fmt.Errorf("keychain: %w", token.ErrInteraction)
			return s
		}, time.Second, "a38 config onepassword-token set --account privat --replace"},
		{"other error", func() *fakeStore { s := newFakeStore(); s.getErr = errors.New("keychain: status -50"); return s }, time.Second, "cannot be read: keychain: status -50"},
		{"empty", func() *fakeStore { s := newFakeStore(); e := ""; s.value = &e; return s }, time.Second, "is empty"},
		{"dialog", func() *fakeStore { s := newFakeStore(); s.value = &v; s.block = make(chan struct{}); return s }, 20 * time.Millisecond, "gave no token within"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.store()
			src := keychainSource{account: "privat", newStore: func(string) (token.Store, error) { return s, nil }, timeout: tt.timeout}
			tok, err := src.Token()
			if s.block != nil {
				close(s.block)
			}
			if tt.want == "" {
				if err != nil || tok.Reveal() != tokenMarker {
					t.Fatalf("Token: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			noToken(t, err.Error())
		})
	}
	src := keychainSource{account: "privat", newStore: func(string) (token.Store, error) { return nil, errors.New("not available on this platform") }, timeout: time.Second}
	if _, err := src.Token(); err == nil || !strings.Contains(err.Error(), "not available") {
		t.Errorf("no store: %v", err)
	}
}

func TestTokenAccounts(t *testing.T) {
	// T44: the token commands need the account and show for each account
	// only metadata (D72). No test touches the item a38.onepassword.
	newEnv := func(t *testing.T) *tokenEnv {
		e := newTokenEnv(t, config.TokenSourceKeychain)
		firmaFile := filepath.Join(e.cfgDir, "onepassword-token-firma")
		data := fmt.Sprintf("version: 1\nvaults:\n"+
			"  - {name: beispiel.app.k8s-dev, environment: dev, account: privat}\n"+
			"  - {name: datenbank-dev, environment: dev, account: firma}\n"+
			"backend:\n  onepassword:\n    accounts:\n"+
			"      - name: privat\n        token_file: %s\n"+
			"      - name: firma\n        token_source: file\n        token_file: %s\n", e.file, firmaFile)
		if err := os.WriteFile(e.cfgPath, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		writeMode(t, firmaFile, 0o600)
		return e
	}

	t.Run("set without account", func(t *testing.T) {
		e := newEnv(t)
		e.typed = tokenMarker
		code, out, errOut := e.run("set")
		if code != 2 || !strings.Contains(errOut, "--account NAME") || out != "" || e.prompts != 0 {
			t.Errorf("code %d, prompts %d, stderr %q", code, e.prompts, errOut)
		}
	})
	t.Run("set with unknown account", func(t *testing.T) {
		e := newEnv(t)
		e.typed = tokenMarker
		code, _, errOut := e.run("set", "--account", "unbekannt")
		if code != 1 || !strings.Contains(errOut, `the account "unbekannt" is not in backend.onepassword.accounts`) || e.prompts != 0 {
			t.Errorf("code %d, prompts %d, stderr %q", code, e.prompts, errOut)
		}
		if len(e.stores) != 1 || e.store.value != nil {
			t.Error("the command touched a store")
		}
	})
	t.Run("set never echoes an invalid account name", func(t *testing.T) {
		// T44: a human can paste the token after --account by mistake.
		// The command checks the name first and never shows a value
		// that fails the check.
		e := newEnv(t)
		e.typed = tokenMarker
		code, out, errOut := e.run("set", "--account", tokenMarker)
		if code != 2 || e.prompts != 0 || !strings.Contains(errOut, "the account name") {
			t.Errorf("code %d, prompts %d, stderr %q", code, e.prompts, errOut)
		}
		if strings.Contains(out+errOut, tokenMarker) || strings.Contains(out+errOut, "ops_") {
			t.Errorf("the output shows the value of --account: stdout %q, stderr %q", out, errOut)
		}
		if e.store.value != nil {
			t.Error("the command stored a value")
		}
	})
	t.Run("set stores the item of the account", func(t *testing.T) {
		e := newEnv(t)
		e.typed = tokenMarker
		code, out, errOut := e.run("set", "--account", "privat")
		if code != 0 || e.store.value == nil || *e.store.value != tokenMarker {
			t.Fatalf("code %d, stderr %q", code, errOut)
		}
		if e.stores["firma"] != nil && e.stores["firma"].value != nil {
			t.Error("the item of firma changed")
		}
		noToken(t, out, errOut)
		noDerived(t, out, errOut)
	})
	t.Run("status of all accounts and the old item", func(t *testing.T) {
		e := newEnv(t)
		v, old := tokenMarker, tokenMarker
		e.store.value = &v
		e.storeOf(config.LegacyAccount).value = &old
		code, out, errOut := e.run("status")
		if code != 0 {
			t.Errorf("code %d, stdout:\n%s\nstderr: %s", code, out, errOut)
		}
		for _, want := range []string{
			"Account privat: vaults beispiel.app.k8s-dev",
			"item a38-test-fake/privat exists (value not read)",
			"Account firma: vaults datenbank-dev",
			"Source:      file",
			"onepassword-token-firma exists (content not read)",
			"Old item:    a38-test-fake/service-account exists",
			"a38 config onepassword-token delete --legacy",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("stdout lacks %q:\n%s", want, out)
			}
		}
		for _, s := range e.stores {
			if s.reads != 0 {
				t.Errorf("status read the token of %s", s.account)
			}
		}
		noToken(t, out, errOut)
		noDerived(t, out, errOut)
	})
	t.Run("status of one account", func(t *testing.T) {
		e := newEnv(t)
		code, out, _ := e.run("status", "--account", "firma")
		if code != 0 || !strings.Contains(out, "Account firma") || strings.Contains(out, "Account privat") || strings.Contains(out, "Old item") {
			t.Errorf("code %d, stdout:\n%s", code, out)
		}
	})
	t.Run("status fails for an account without token", func(t *testing.T) {
		e := newEnv(t)
		code, out, _ := e.run("status")
		if code != 1 || !strings.Contains(out, `"a38 config onepassword-token set --account privat"`) {
			t.Errorf("code %d, stdout:\n%s", code, out)
		}
	})
	t.Run("delete one item and the old item", func(t *testing.T) {
		e := newEnv(t)
		v, old := tokenMarker, tokenMarker
		e.store.value = &v
		legacy := e.storeOf(config.LegacyAccount)
		legacy.value = &old
		if code, _, errOut := e.run("delete", "--legacy"); code != 0 || legacy.value != nil || e.store.value == nil {
			t.Errorf("delete --legacy: code %d, stderr %q", code, errOut)
		}
		if code, _, errOut := e.run("delete", "--account", "privat"); code != 0 || e.store.value != nil {
			t.Errorf("delete --account privat: code %d, stderr %q", code, errOut)
		}
		for _, args := range [][]string{{"delete"}, {"delete", "--account", "privat", "--legacy"}, {"delete", "--account", "Privat"}} {
			if code, _, _ := e.run(args...); code != 2 {
				t.Errorf("%v: code %d, want 2", args, code)
			}
		}
	})
}
