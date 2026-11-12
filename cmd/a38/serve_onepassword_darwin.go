//go:build darwin

package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/backend/onepassword"
	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/token"
	"github.com/seletz/passierschein/internal/tokenfile"
	"github.com/seletz/passierschein/internal/version"
)

// startCheckTimeout is the time limit of the first state check of the
// 1Password backend.
const startCheckTimeout = 30 * time.Second

// tokenSource returns the source of the token of the service account.
// Today it is the token file, the interim source for development and tests
// (D51). The target is the macOS keychain (spike in issue 53); a keychain
// source replaces this constructor only.
func tokenSource(cfg *config.Config) token.Source {
	return fileSource{tokenfile.File{Path: cfg.Backend.OnePassword.TokenFile}}
}

// fileSource names the configuration key in each error of the token file.
type fileSource struct{ f tokenfile.File }

func (s fileSource) Token() (token.Token, error) {
	tok, err := s.f.Token()
	if err != nil {
		return token.Token{}, fmt.Errorf("token file %s: %w (backend.onepassword.token_file)", s.f.Path, err)
	}
	return tok, nil
}

// newOnePassword builds the 1Password backend with the token file of the
// configuration (G3, working assumption). A token file that is missing or
// has the wrong owner or mode stops the start: the human must fix it.
//
// A bad token or a missing network does not stop the start: the daemon
// runs, each request ends with backend_locked or backend_unavailable, and
// a38 doctor shows the state. A check in the background logs the first
// state; it reads no value.
func newOnePassword(cfg *config.Config, log *slog.Logger) (backend.Backend, error) {
	tok, err := tokenSource(cfg).Token()
	if err != nil {
		return nil, fmt.Errorf("--backend 1password: %w", err)
	}
	vaults := make([]string, 0, len(cfg.Vaults))
	for _, v := range cfg.Vaults {
		vaults = append(vaults, v.Name)
	}
	be, err := onepassword.New(onepassword.Options{Vaults: vaults, Token: tok, Version: version.Version, Logger: log})
	if err != nil {
		return nil, err
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), startCheckTimeout)
		defer cancel()
		seen, err := be.Vaults(ctx)
		if err != nil {
			log.Warn("the 1password backend does not work yet; each request ends with this code until it works", "code", string(backend.Code(err)))
			return
		}
		log.Info("the 1password backend works", "vaults_configured", len(vaults), "vaults_visible", len(seen))
		if len(seen) < len(vaults) {
			log.Warn("the service account cannot see each vault of the configuration; references to the other vaults end with not_found")
		}
	}()
	return be, nil
}
