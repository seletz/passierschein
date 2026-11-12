//go:build darwin

package keychain

import (
	"errors"
	"testing"

	"github.com/seletz/passierschein/internal/token"
)

// check refuses an empty name and, in a test binary, the product item: a
// test must never touch the item of the human (issue 59).
func (s Store) check() error {
	if s.Service == "" || s.Account == "" {
		return errors.New("keychain: the service and the account of the item must not be empty")
	}
	if testing.Testing() && s.Service == ProductService {
		return errors.New("keychain: a test must not use the product item; use a service name that starts with " + TestPrefix)
	}
	return nil
}

// OSStatus values of Security.framework.
const (
	errSecSuccess               = 0
	errSecUserCanceled          = -128
	errSecAuthFailed            = -25293
	errSecDuplicateItem         = -25299
	errSecItemNotFound          = -25300
	errSecInteractionNotAllowed = -25308
	errSecInteractionRequired   = -25315
	errSecNoAccessForItem       = -25243
)

// kindOf maps an OSStatus to a kind of error.
func kindOf(status int32) error {
	switch status {
	case errSecItemNotFound:
		return token.ErrNotFound
	case errSecDuplicateItem:
		return token.ErrExists
	case errSecUserCanceled, errSecAuthFailed, errSecInteractionNotAllowed, errSecInteractionRequired, errSecNoAccessForItem:
		return token.ErrInteraction
	}
	return nil
}
