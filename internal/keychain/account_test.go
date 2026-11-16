package keychain

import "testing"

func TestForAccount(t *testing.T) {
	// T44 (part): each account has its own item under the service of the
	// product; the account of the item is the name of the account (D72).
	// The test only compares fields; it does not touch the keychain.
	s := ForAccount("privat")
	if s.Service != ProductService || s.Account != "privat" || s.Label != "a38: 1Password Service Account (privat)" {
		t.Errorf("ForAccount(privat) = %+v", s)
	}
	if s.Name() != "a38.onepassword/privat" {
		t.Errorf("Name = %q", s.Name())
	}
	l := Legacy()
	if l.Service != ProductService || l.Account != LegacyAccount {
		t.Errorf("Legacy() = %+v", l)
	}
}
