//go:build darwin

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/seletz/passierschein/internal/keychain"
	"github.com/seletz/passierschein/internal/token"
)

// envTestKeychainService names a test item instead of the product item. It
// exists only for a manual test of a built binary with a dummy value; the
// name must start with "a38-test-". It never carries a token.
const envTestKeychainService = "A38_TEST_KEYCHAIN_SERVICE"

// systemTokenStore returns the keychain item of the token (D51).
func systemTokenStore() (token.Store, error) {
	svc := os.Getenv(envTestKeychainService)
	if svc == "" {
		return keychain.Product(), nil
	}
	if !strings.HasPrefix(svc, keychain.TestPrefix) || len(svc) > 64 || strings.ContainsFunc(svc, func(r rune) bool { return r <= ' ' || r > '~' }) {
		return nil, fmt.Errorf("%s must start with %q and hold printable ASCII only", envTestKeychainService, keychain.TestPrefix)
	}
	return keychain.Store{Service: svc, Account: "a38-test", Label: "a38 test item (dummy value)"}, nil
}
