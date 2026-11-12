package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/token"
	"github.com/seletz/passierschein/internal/tokenfile"
)

// keychainReadTimeout limits the read of the keychain item at the start of
// the daemon. A read by a new build of a38 waits for a dialog of macOS
// (spike in issue 53, V4); the daemon does not wait for ever.
var keychainReadTimeout = 2 * time.Minute

// tokenSource returns the source of the token of the service account that
// the configuration selects (D51): the keychain item (default) or the
// token file (fallback for development and CI).
func tokenSource(cfg *config.Config) token.Source {
	if cfg.Backend.OnePassword.TokenSource == config.TokenSourceFile {
		return fileSource{tokenfile.File{Path: cfg.Backend.OnePassword.TokenFile}}
	}
	return keychainSource{newStore: newTokenStore, timeout: keychainReadTimeout}
}

// fileSource names the configuration key in each error of the token file.
type fileSource struct{ f tokenfile.File }

func (s fileSource) Token() (token.Token, error) {
	tok, err := s.f.Token()
	if err != nil {
		return token.Token{}, fmt.Errorf("token file %s: %w (backend.onepassword.token_source: file, backend.onepassword.token_file)", s.f.Path, err)
	}
	return tok, nil
}

// keychainSource reads the token from the keychain item once. Each error
// names the finding and the next step; none contains the token.
type keychainSource struct {
	newStore func() (token.Store, error)
	timeout  time.Duration
}

func (s keychainSource) Token() (token.Token, error) {
	store, err := s.newStore()
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
	var r result
	select {
	case r = <-done:
	case <-time.After(s.timeout):
		return token.Token{}, fmt.Errorf("the keychain item %s gave no token within %s; macOS may wait for a confirmation in a dialog. Confirm only a dialog that you caused yourself, or store the token again for this build with \"a38 config onepassword-token set --replace\"", store.Name(), s.timeout)
	}
	switch {
	case r.err == nil && r.tok.Empty():
		return token.Token{}, fmt.Errorf("the keychain item %s is empty; store the token with \"a38 config onepassword-token set --replace\"", store.Name())
	case r.err == nil:
		return r.tok, nil
	case errors.Is(r.err, token.ErrNotFound):
		return token.Token{}, fmt.Errorf("the keychain has no item %s (backend.onepassword.token_source: keychain, the default); store the token with \"a38 config onepassword-token set\", or set backend.onepassword.token_source: file to use the token file; then start the daemon again", store.Name())
	case errors.Is(r.err, token.ErrInteraction):
		return token.Token{}, fmt.Errorf("the keychain refused the item %s or needs a confirmation (%w); check it with \"a38 config onepassword-token status\" and store the token again with \"a38 config onepassword-token set --replace\"", store.Name(), r.err)
	}
	return token.Token{}, fmt.Errorf("the keychain item %s cannot be read: %w", store.Name(), r.err)
}
