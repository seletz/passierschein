//go:build !darwin

package keychain

import (
	"fmt"

	"github.com/seletz/passierschein/internal/token"
)

// Token returns [token.ErrUnavailable]: the keychain exists on macOS only.
func (s Store) Token() (token.Token, error) { return token.Token{}, unavailable() }

// Put returns [token.ErrUnavailable].
func (s Store) Put(token.Token, bool) error { return unavailable() }

// Delete returns [token.ErrUnavailable].
func (s Store) Delete() error { return unavailable() }

// Describe returns [token.ErrUnavailable].
func (s Store) Describe() (token.StoreInfo, error) { return token.StoreInfo{}, unavailable() }

// Signature returns [token.ErrUnavailable].
func (s Store) Signature() (token.Signature, error) { return token.Signature{}, unavailable() }

func unavailable() error {
	return fmt.Errorf("keychain: %w", token.ErrUnavailable)
}
