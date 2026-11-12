//go:build !darwin

package main

import (
	"fmt"

	"github.com/seletz/passierschein/internal/token"
)

// systemTokenStore fails: the keychain and the 1Password backend are part
// of the macOS build only (D42, D53). The linux build does not import the
// package keychain.
func systemTokenStore() (token.Store, error) {
	return nil, fmt.Errorf("the keychain is %w; the 1Password backend and its token are part of the macOS build only", token.ErrUnavailable)
}
