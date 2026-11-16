//go:build darwin

package main

import (
	"testing"

	"github.com/seletz/passierschein/internal/keychain"
	"github.com/seletz/passierschein/internal/opguard"
)

// The guard knows the service name and the label of the keychain item of
// a38 without importing internal/keychain (not part of the linux build).
// This test keeps the names in step. It reads no keychain item.
func TestHookKeychainNames(t *testing.T) {
	for _, cmd := range []string{
		"security find-generic-password -w -s " + keychain.ProductService,
		"security find-generic-password -w -l '" + keychain.ProductLabel + "'",
		// T44 (part): the item of an account (D72).
		"security find-generic-password -w -l '" + keychain.ForAccount("firma").Label + "'",
	} {
		if rule, _ := opguard.Check(cmd); rule != opguard.RuleKeychainToken {
			t.Errorf("Check(%q) = %q, want %q", cmd, rule, opguard.RuleKeychainToken)
		}
	}
}
