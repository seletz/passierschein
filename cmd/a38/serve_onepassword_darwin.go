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
	"github.com/seletz/passierschein/internal/version"
)

// startCheckTimeout is the time limit of the first state check of each
// 1Password account (D78).
const startCheckTimeout = 30 * time.Second

// newOnePassword builds the 1Password backend with one client for each
// account (D73). It reads the token of each account once, here, from the
// configured source (D72): the keychain item of the account or its token
// file. An account without token is locked: each request for its vaults
// ends with backend_locked, and the log names the account and the next
// step. If no account has a token, the start fails.
//
// A bad token or a missing network does not stop the start: the daemon
// runs, each request ends with backend_locked or backend_unavailable, and
// a38 doctor shows the state. A check in the background logs the first
// state of each account; it asks the accounts side by side, each with its
// own time limit (D78), and reads no value.
func newOnePassword(cfg *config.Config, log *slog.Logger) (backend.Backend, error) {
	tokens, err := openAccounts(cfg, tokenSourceFor, log)
	if err != nil {
		return nil, fmt.Errorf("--backend 1password: %w", err)
	}
	accounts := make([]onepassword.Account, 0, len(tokens))
	for _, t := range tokens {
		accounts = append(accounts, onepassword.Account{Name: t.Name, Vaults: t.Vaults, Token: t.Token, Locked: t.Locked})
	}
	be, err := onepassword.NewAccounts(onepassword.AccountsOptions{Accounts: accounts, Version: version.Version, Logger: log})
	if err != nil {
		return nil, err
	}
	go logStartCheck(context.Background(), be, startCheckTimeout, log)
	return be, nil
}
