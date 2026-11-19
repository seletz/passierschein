package main

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/token"
	"github.com/seletz/passierschein/internal/tokenfile"
)

// keychainReadTimeout limits the read of a keychain item at the start of
// the daemon. A read by a new build of a38 waits for a dialog of macOS
// (spike in issue 53, V4); the daemon does not wait for ever. The reads of
// the accounts run side by side, so the start waits this time at most
// once (D79).
var keychainReadTimeout = 2 * time.Minute

// accountToken is the result of the start for one account (D73): the
// token, or the reason why the account has none.
type accountToken struct {
	// Name is the name of the account.
	Name string
	// Vaults lists the vaults of the account.
	Vaults []string
	// Token is the token; empty if Locked is set.
	Token token.Token
	// Locked says, without a value, why the account has no token.
	Locked string
}

// openAccounts reads the token of each account that a vault names, once
// (D73). It reads the sources side by side (D79): two keychain dialogs of
// macOS cost one wait of keychainReadTimeout, not one for each account.
// The result and the log keep the order of the configuration. It does not
// read the source of an account without vault, so that no dialog comes
// for it. An account whose source fails is locked: the log names it and
// the next step. If no account has a token, openAccounts fails and names
// each account.
func openAccounts(cfg *config.Config, sourceFor func(config.Account) token.Source, log *slog.Logger) ([]accountToken, error) {
	var out []accountToken
	var sources []token.Source
	for _, a := range cfg.Backend.OnePassword.Accounts {
		vaults := cfg.VaultsOf(a.Name)
		if len(vaults) == 0 {
			continue
		}
		out = append(out, accountToken{Name: a.Name, Vaults: vaults})
		sources = append(sources, sourceFor(a))
	}
	if len(out) == 0 {
		return nil, errors.New("the configuration names no vault with a 1Password account; add an account (a38 config account add) and a vault (a38 config vault add --account)")
	}
	var wg sync.WaitGroup
	for i := range out {
		wg.Go(func() {
			tok, err := sources[i].Token()
			switch {
			case err != nil:
				out[i].Locked = err.Error()
			case tok.Empty():
				out[i].Locked = "the token is empty"
			default:
				out[i].Token = tok
			}
		})
	}
	wg.Wait()
	var failures []string
	for _, at := range out {
		if at.Locked == "" {
			continue
		}
		failures = append(failures, fmt.Sprintf("account %q: %s", at.Name, at.Locked))
		log.Warn("the 1password account has no token; the daemon starts, and each request for its vaults ends with backend_locked",
			"account", at.Name, "reason", at.Locked)
	}
	if len(failures) == len(out) {
		return nil, fmt.Errorf("no account has a token, so the daemon does not start:\n%s", strings.Join(failures, "\n"))
	}
	return out, nil
}

// tokenSourceFor returns the source of the token of an account that the
// configuration selects (D72): the keychain item of the account (default)
// or its token file (fallback for development and CI).
func tokenSourceFor(a config.Account) token.Source {
	if a.TokenSource == config.TokenSourceFile {
		return fileSource{f: tokenfile.File{Path: a.TokenFile}}
	}
	return keychainSource{account: a.Name, newStore: newTokenStore, timeout: keychainReadTimeout}
}

// fileSource names the file and the configuration keys in each error of
// the token file. openAccounts adds the name of the account.
type fileSource struct {
	f tokenfile.File
}

func (s fileSource) Token() (token.Token, error) {
	tok, err := s.f.Token()
	if err != nil {
		return token.Token{}, fmt.Errorf("token file %s: %w (token_source: file, token_file)", s.f.Path, err)
	}
	return tok, nil
}

// keychainSource reads the token of an account from its keychain item
// once. Each error names the finding and the next step; none contains the
// token.
type keychainSource struct {
	account  string
	newStore func(account string) (token.Store, error)
	timeout  time.Duration
}

func (s keychainSource) Token() (token.Token, error) {
	store, err := s.newStore(s.account)
	if err != nil {
		return token.Token{}, fmt.Errorf("token source keychain: %w", err)
	}
	type result struct {
		tok token.Token
		err error
	}
	done := make(chan result, 1)
	go func() {
		tok, err := store.Token()
		done <- result{tok, err}
	}()
	set := tokenCmd + " set --account " + s.account
	var r result
	select {
	case r = <-done:
	case <-time.After(s.timeout):
		return token.Token{}, fmt.Errorf("the keychain item %s gave no token within %s; macOS may wait for a confirmation in a dialog. Confirm only a dialog that you caused yourself, or store the token again for this build with \"%s --replace\"", store.Name(), s.timeout, set)
	}
	switch {
	case r.err == nil && r.tok.Empty():
		return token.Token{}, fmt.Errorf("the keychain item %s is empty; store the token with \"%s --replace\"", store.Name(), set)
	case r.err == nil:
		return r.tok, nil
	case errors.Is(r.err, token.ErrNotFound):
		return token.Token{}, fmt.Errorf("the keychain has no item %s (token_source: keychain, the default); store the token with \"%s\", or set token_source: file for the account to use its token file; then start the daemon again", store.Name(), set)
	case errors.Is(r.err, token.ErrInteraction):
		return token.Token{}, fmt.Errorf("the keychain refused the item %s or needs a confirmation (%w); check it with \"%s status --account %s\" and store the token again with \"%s --replace\"", store.Name(), r.err, tokenCmd, s.account, set)
	}
	return token.Token{}, fmt.Errorf("the keychain item %s cannot be read: %w", store.Name(), r.err)
}
