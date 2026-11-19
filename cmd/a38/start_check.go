package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/seletz/passierschein/internal/backend"
)

// logStartCheck asks each account of the backend for its vaults, side by
// side, each with the time limit limit, and logs the state of each account
// (D73, D78). A hanging account does not change the line of another
// account. It reads no value.
func logStartCheck(ctx context.Context, c backend.AccountChecker, limit time.Duration, log *slog.Logger) {
	for _, st := range c.Check(ctx, limit) {
		if st.Err != nil {
			log.Warn("the 1password account does not work yet; each request for its vaults ends with this code until it works",
				"account", st.Name, "code", string(backend.Code(st.Err)))
			continue
		}
		log.Info("the 1password account works", "account", st.Name, "vaults_configured", st.Configured, "vaults_visible", st.Visible)
		if st.Visible < st.Configured {
			log.Warn("the service account of the account cannot see each of its vaults of the configuration; references to the other vaults end with not_found", "account", st.Name)
		}
	}
}
