//go:build !darwin

package main

import (
	"errors"
	"log/slog"

	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/config"
)

// newOnePassword fails: the linux build of a38 contains no code for
// 1Password (D2, D42, T13).
func newOnePassword(*config.Config, *slog.Logger) (backend.Backend, error) {
	return nil, errors.New("--backend 1password is not available on this platform; the 1Password backend is part of the macOS build only")
}
