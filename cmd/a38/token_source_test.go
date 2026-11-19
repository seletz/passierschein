package main

import (
	"bytes"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/token"
	"github.com/seletz/passierschein/internal/tokenfile"
)

// readLog records the sources that openAccounts reads. The sources run
// side by side (D79).
type readLog struct {
	mu    sync.Mutex
	names []string
}

func (r *readLog) add(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.names = append(r.names, name)
}

// sorted returns the names in sorted order.
func (r *readLog) sorted() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := slices.Clone(r.names)
	slices.Sort(n)
	return strings.Join(n, ",")
}

// mapSource is a token source in memory for one account.
type mapSource struct {
	tok  *string
	err  error
	read *readLog
	name string
}

func (s mapSource) Token() (token.Token, error) {
	s.read.add(s.name)
	if s.err != nil {
		return token.Token{}, s.err
	}
	return token.New(*s.tok), nil
}

// accountsConfig returns a configuration with the accounts privat and
// firma, each with one vault, and the account reserve without vault.
func accountsConfig(t *testing.T) *config.Config {
	t.Helper()
	t.Setenv("HOME", "/home/beispiel")
	t.Setenv("XDG_CONFIG_HOME", "")
	cfg, err := config.Parse([]byte(`version: 1
vaults:
  - {name: beispiel.app.k8s-dev, environment: dev, account: privat}
  - {name: datenbank-dev, environment: dev, account: firma}
backend:
  onepassword:
    accounts:
      - name: privat
      - name: firma
      - name: reserve
`))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestOpenAccounts(t *testing.T) {
	// T46: the daemon reads the token of each account that a vault names
	// once at its start. An account without token locks only its vaults;
	// without any token the start fails (D73).
	value := tokenMarker
	// The error of a real file source: openAccounts names the account
	// once (review of PR 114).
	_, fileErr := fileSource{f: tokenfile.File{Path: "/tmp/a38-missing/onepassword-token-privat"}}.Token()
	missing := fmt.Errorf("the keychain has no item a38.onepassword/firma; store the token with \"a38 config onepassword-token set --account firma\": %w", token.ErrNotFound)
	tests := []struct {
		name       string
		errs       map[string]error
		wantErr    []string
		wantLocked map[string]string
		wantLog    []string
	}{
		{"both", nil, nil, map[string]string{}, nil},
		{"firma without token", map[string]error{"firma": missing}, nil,
			map[string]string{"firma": "a38 config onepassword-token set --account firma"},
			[]string{"account=firma", "backend_locked"}},
		{"no token", map[string]error{"privat": fileErr, "firma": missing},
			[]string{`account "privat"`, "onepassword-token-privat", `account "firma"`, "--account firma", "no account has a token"}, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := accountsConfig(t)
			var read readLog
			var logs bytes.Buffer
			got, err := openAccounts(cfg, func(a config.Account) token.Source {
				return mapSource{tok: &value, err: tt.errs[a.Name], read: &read, name: a.Name}
			}, slog.New(slog.NewTextHandler(&logs, nil)))
			if got := read.sorted(); got != "firma,privat" {
				t.Errorf("read the sources %v, want privat and firma (not reserve)", got)
			}
			if tt.wantErr != nil {
				if err == nil {
					t.Fatal("openAccounts succeeded, want an error")
				}
				for _, w := range tt.wantErr {
					if !strings.Contains(err.Error(), w) {
						t.Errorf("error %q lacks %q", err, w)
					}
				}
				for _, name := range []string{"privat", "firma"} {
					if n := strings.Count(err.Error(), fmt.Sprintf("account %q", name)); n != 1 {
						t.Errorf("error names the account %s %d times, want once: %q", name, n, err)
					}
				}
				noToken(t, err.Error(), logs.String())
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 2 || got[0].Name != "privat" || got[1].Name != "firma" {
				t.Fatalf("accounts = %+v", got)
			}
			for _, a := range got {
				want, locked := tt.wantLocked[a.Name]
				switch {
				case locked && (!a.Token.Empty() || !strings.Contains(a.Locked, want)):
					t.Errorf("account %s = %+v, want locked with %q", a.Name, a, want)
				case !locked && (a.Token.Reveal() != tokenMarker || a.Locked != ""):
					t.Errorf("account %s has no token: %q", a.Name, a.Locked)
				}
			}
			if got[1].Vaults[0] != "datenbank-dev" {
				t.Errorf("vaults of firma = %v", got[1].Vaults)
			}
			for _, w := range tt.wantLog {
				if !strings.Contains(logs.String(), w) {
					t.Errorf("log lacks %q: %s", w, logs.String())
				}
			}
			noToken(t, logs.String(), fmt.Sprintf("%+v", got))
		})
	}
}

// barrierSource gives its token only after each source of the barrier
// has started. Sources that run one after the other wait until the time
// limit of the test.
type barrierSource struct {
	started *sync.WaitGroup
	all     chan struct{}
}

func (s barrierSource) Token() (token.Token, error) {
	s.started.Done()
	select {
	case <-s.all:
		return token.New(tokenMarker), nil
	case <-time.After(2 * time.Second):
		return token.Token{}, fmt.Errorf("the other sources did not start: %w", token.ErrNotFound)
	}
}

func TestOpenAccountsSideBySide(t *testing.T) {
	// T50: the daemon reads the tokens of the accounts side by side. Two
	// keychain dialogs cost one wait, not two. The result and the log keep
	// the order of the configuration (D79).
	t.Run("the sources run at the same time", func(t *testing.T) {
		cfg := accountsConfig(t)
		var started sync.WaitGroup
		started.Add(2)
		all := make(chan struct{})
		go func() { started.Wait(); close(all) }()
		got, err := openAccounts(cfg, func(config.Account) token.Source { return barrierSource{started: &started, all: all} },
			slog.New(slog.DiscardHandler))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].Name != "privat" || got[1].Name != "firma" {
			t.Fatalf("accounts = %+v, want privat, firma", got)
		}
		for _, a := range got {
			if a.Locked != "" {
				t.Errorf("account %s is locked: %s; the sources ran one after the other", a.Name, a.Locked)
			}
		}
	})

	t.Run("two dialogs wait one time limit", func(t *testing.T) {
		cfg := accountsConfig(t)
		const limit = 300 * time.Millisecond
		var stores []*fakeStore
		var mu sync.Mutex
		start := time.Now()
		_, err := openAccounts(cfg, func(a config.Account) token.Source {
			s := newFakeStore()
			s.block = make(chan struct{})
			mu.Lock()
			stores = append(stores, s)
			mu.Unlock()
			return keychainSource{account: a.Name, newStore: func(string) (token.Store, error) { return s, nil }, timeout: limit}
		}, slog.New(slog.DiscardHandler))
		elapsed := time.Since(start)
		for _, s := range stores {
			close(s.block)
		}
		if err == nil || strings.Count(err.Error(), "gave no token within") != 2 {
			t.Fatalf("error = %v, want both accounts with the time limit", err)
		}
		if elapsed > limit+limit/2 {
			t.Errorf("openAccounts took %s, want about one time limit (%s), not one for each account", elapsed, limit)
		}
	})
}

func TestTokenSourceForAccount(t *testing.T) {
	// T46 (part), D72: keychain is the default source of an account; the
	// file source names the account and the configuration keys.
	cfg := accountsConfig(t)
	privat, _ := cfg.Account("privat")
	if got := fmt.Sprintf("%T", tokenSourceFor(privat)); got != "main.keychainSource" {
		t.Errorf("privat: %s", got)
	}
	file := config.Account{Name: "firma", TokenSource: config.TokenSourceFile, TokenFile: "/tmp/a38-missing/onepassword-token-firma"}
	src := tokenSourceFor(file)
	if got := fmt.Sprintf("%T", src); got != "main.fileSource" {
		t.Errorf("firma: %s", got)
	}
	// openAccounts names the account; the source names the file and the keys.
	if _, err := src.Token(); err == nil || strings.Contains(err.Error(), `account "firma"`) || !strings.Contains(err.Error(), "onepassword-token-firma") || !strings.Contains(err.Error(), "token_source: file") {
		t.Errorf("file source: %v", err)
	}
}
