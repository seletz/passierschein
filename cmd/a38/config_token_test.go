package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
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
	newTokenStore = func() (token.Store, error) {
		return nil, errors.New("test: no token store installed")
	}
	os.Exit(m.Run())
}

// fakeStore is a token store in memory.
type fakeStore struct {
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

func newFakeStore() *fakeStore {
	return &fakeStore{readable: true, sig: token.Signature{Path: "/x/a38", Kind: token.SignatureAdhoc, CDHash: "abcd"}}
}

func (f *fakeStore) Name() string { return "a38-test-fake/a38-test" }
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
	store    *fakeStore
	cfgPath  string
	file     string
	terminal bool
	typed    string
	prompts  int
}

func newTokenEnv(t *testing.T, source config.TokenSource) *tokenEnv {
	t.Helper()
	dir := t.TempDir()
	e := &tokenEnv{store: newFakeStore(), cfgPath: filepath.Join(dir, "config.yaml"), file: filepath.Join(dir, "onepassword-token"), terminal: true}
	data := fmt.Sprintf("version: 1\nbackend:\n  onepassword:\n    token_source: %s\n    token_file: %s\n", source, e.file)
	if err := os.WriteFile(e.cfgPath, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *tokenEnv) run(args ...string) (int, string, string) {
	deps := tokenDeps{
		terminal: e.terminal,
		readSecret: func(string) ([]byte, error) {
			e.prompts++
			return []byte(e.typed), nil
		},
		newStore: func() (token.Store, error) { return e.store, nil },
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
		{"terminal", false, tokenMarker, nil, 0, "Stored the token in the keychain item a38-test-fake/a38-test.", "", tokenMarker, true},
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
			code, out, errOut := e.run(append([]string{"set"}, tt.args...)...)
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
	code, out, errOut := e.run("set")
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
	code, out, errOut := e.run("set")
	if code != 0 || !strings.Contains(errOut, "token_source: file") {
		t.Errorf("code %d, stderr %q", code, errOut)
	}
	noToken(t, out, errOut)
}

func TestTokenSetStoreError(t *testing.T) {
	e := newTokenEnv(t, config.TokenSourceKeychain)
	e.store.putErr = errors.New("keychain: SecItemAdd: status -25308 (User interaction is not allowed.)")
	e.typed = tokenMarker
	code, out, errOut := e.run("set")
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
			[]string{"Source:      keychain", "no item a38-test-fake/a38-test", "a38 config onepassword-token set", "Signature:   ad-hoc", "each new build of a38 needs a confirmation", "does not exist"}},
		{"item of this build", config.TokenSourceKeychain, func(_ *testing.T, e *tokenEnv) { v := tokenMarker; e.store.value = &v }, 0,
			[]string{"item a38-test-fake/a38-test exists (value not read)", "2026-11-12T08:00:00Z", "Trusted:   /x/a38", "cdhash:abcd"}},
		{"item of another build", config.TokenSourceKeychain, func(_ *testing.T, e *tokenEnv) {
			v := tokenMarker
			e.store.value, e.store.readable = &v, false
		}, 0, []string{"exists", "this build of a38 did not store the item", "set --replace"}},
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
			if st.Source != tt.source || st.OK != (code == 0) {
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
	if code != 1 || !strings.Contains(out, "Source:      keychain") || !strings.Contains(out, "did not load") {
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
		{"keychain source", config.TokenSourceKeychain, true, 0, "next start with the 1Password backend fails until you run \"a38 config onepassword-token set\"", ""},
		{"file source", config.TokenSourceFile, true, 0, "uses the token file", ""},
		{"no item", config.TokenSourceKeychain, false, 1, "", "has no item"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTokenEnv(t, tt.source)
			if tt.existing {
				v := tokenMarker
				e.store.value = &v
			}
			code, out, errOut := e.run("delete")
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
	if code, _, errOut := e.run("delete", "now"); code != 2 || !strings.Contains(errOut, "unexpected argument") {
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
			newStore: func() (token.Store, error) { return nil, fmt.Errorf("the keychain is %w", token.ErrUnavailable) },
			lstat:    os.Lstat,
		}
		var o, er bytes.Buffer
		if code := runTokenWith([]string{sub, "--config", "/tmp/a38-missing/config.yaml"}, &o, &er, deps); code != 1 || !strings.Contains(er.String(), "not available on this platform") {
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
		{"no item", newFakeStore, time.Second, `has no item a38-test-fake/a38-test (backend.onepassword.token_source: keychain); store the token with "a38 config onepassword-token set"`},
		{"interaction", func() *fakeStore {
			s := newFakeStore()
			s.getErr = fmt.Errorf("keychain: %w", token.ErrInteraction)
			return s
		}, time.Second, "a38 config onepassword-token set --replace"},
		{"other error", func() *fakeStore { s := newFakeStore(); s.getErr = errors.New("keychain: status -50"); return s }, time.Second, "cannot be read: keychain: status -50"},
		{"empty", func() *fakeStore { s := newFakeStore(); e := ""; s.value = &e; return s }, time.Second, "is empty"},
		{"dialog", func() *fakeStore { s := newFakeStore(); s.value = &v; s.block = make(chan struct{}); return s }, 20 * time.Millisecond, "gave no token within"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.store()
			src := keychainSource{newStore: func() (token.Store, error) { return s, nil }, timeout: tt.timeout}
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
	src := keychainSource{newStore: func() (token.Store, error) { return nil, errors.New("not available on this platform") }, timeout: time.Second}
	if _, err := src.Token(); err == nil || !strings.Contains(err.Error(), "not available") {
		t.Errorf("no store: %v", err)
	}
}

func TestTokenSourceSelection(t *testing.T) {
	// D51, issue 59: keychain is the default source; file is the fallback.
	for _, tt := range []struct {
		source config.TokenSource
		want   string
	}{
		{"", "main.keychainSource"},
		{config.TokenSourceKeychain, "main.keychainSource"},
		{config.TokenSourceFile, "main.fileSource"},
	} {
		cfg := &config.Config{Backend: config.Backend{OnePassword: config.OnePassword{TokenSource: tt.source, TokenFile: "/tmp/a38-missing/onepassword-token"}}}
		if got := fmt.Sprintf("%T", tokenSource(cfg)); got != tt.want {
			t.Errorf("%q: %s, want %s", tt.source, got, tt.want)
		}
	}
	// The file source names the configuration keys.
	cfg := &config.Config{Backend: config.Backend{OnePassword: config.OnePassword{TokenSource: config.TokenSourceFile, TokenFile: "/tmp/a38-missing/onepassword-token"}}}
	if _, err := tokenSource(cfg).Token(); err == nil || !strings.Contains(err.Error(), "token_source: file") || !errors.Is(err, fs.ErrNotExist) && !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("file source: %v", err)
	}
}
