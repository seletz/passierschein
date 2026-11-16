// Package keychain keeps the token of the 1Password service account of each
// account in one generic password item of the macOS keychain (D72, issue
// 88). The items share the service name; the account of an item is the
// name of the account in the configuration.
//
// The package calls Security.framework through purego, without CGO. It
// creates the item itself, never through /usr/bin/security: the keychain
// then trusts only the program that created the item (spike in issue 53,
// tests V1, V2 and V8). A different program that reads the value causes a
// dialog of macOS. kSecUseAuthenticationUIFail does not prevent this
// dialog for the file-based keychain (V4); so a caller must never read the
// value from a program that did not create the item. Describe reads only
// metadata and the access list; this never showed a dialog (V3, V9, V10).
//
// Only the darwin build has the code. The other platforms have a stub that
// returns [token.ErrUnavailable]; the linux build of a38 never imports this
// package (D42, D53, task deps:linux:check).
//
// No error of this package contains the token.
package keychain

import (
	"fmt"
	"strings"

	"github.com/seletz/passierschein/internal/token"
)

// The items of the product. Tests never use them: [Store] refuses the
// service in a test binary.
const (
	// ProductService is the service name of each item of the product.
	ProductService = "a38.onepassword"
	// LegacyAccount is the account name of the item of the configuration
	// with one token (D51). The daemon does not read it any more (D72);
	// no account of the configuration can have this name.
	LegacyAccount = "service-account"
	// ProductLabel is the label that Keychain Access shows.
	ProductLabel = "a38: 1Password Service Account"
)

// TestPrefix starts the service name of each item that a test creates.
const TestPrefix = "a38-test-"

// Store is one generic password item of the login keychain. It implements
// [token.Store].
type Store struct {
	// Service is the service name (kSecAttrService).
	Service string
	// Account is the account name (kSecAttrAccount).
	Account string
	// Label is the label (kSecAttrLabel); only Put uses it.
	Label string
}

// ForAccount returns the store of the item of an account (D72). The
// caller checks the name with config.ValidAccountName.
func ForAccount(account string) Store {
	return Store{Service: ProductService, Account: account, Label: ProductLabel + " (" + account + ")"}
}

// Legacy returns the store of the item of the configuration with one token
// (D51). Only "a38 config onepassword-token status" and "delete --legacy"
// and a38 doctor use it; they never read its value.
func Legacy() Store {
	return Store{Service: ProductService, Account: LegacyAccount, Label: ProductLabel}
}

// Name returns "service/account" for an output line.
func (s Store) Name() string { return s.Service + "/" + s.Account }

var _ token.Store = Store{}

// StatusError is an error of Security.framework. It wraps a kind of
// [token.ErrNotFound], [token.ErrExists] or [token.ErrInteraction], or no
// kind for any other error. It never contains the token.
type StatusError struct {
	// Op is the name of the function that failed.
	Op string
	// Status is the OSStatus.
	Status int32
	// Message is the text of SecCopyErrorMessageString.
	Message string
	kind    error
}

func (e *StatusError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "keychain: %s: ", e.Op)
	if e.kind != nil {
		b.WriteString(e.kind.Error() + ": ")
	}
	fmt.Fprintf(&b, "status %d", e.Status)
	if e.Message != "" {
		fmt.Fprintf(&b, " (%s)", e.Message)
	}
	return b.String()
}

// Unwrap returns the kind of the error.
func (e *StatusError) Unwrap() error { return e.kind }
