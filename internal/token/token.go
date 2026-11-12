// Package token holds the token of the 1Password service account in a
// type that never prints it, the interface of a token source and the
// interface of a token store.
//
// There are two sources (D51): the macOS keychain (package keychain, the
// default; it implements [Store]) and the token file (package tokenfile,
// the fallback for development and CI). The configuration key
// backend.onepassword.token_source selects one. The backend does not know
// the source.
package token

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

// Source gives the token of the service account. The daemon calls it once
// at its start. An error never contains the token.
type Source interface {
	// Token returns the token.
	Token() (Token, error)
}

// redacted replaces the token in each output.
const redacted = "[redacted]"

// Token holds the token of a service account. Formatting with package
// fmt, logging with package slog, json.Marshal and MarshalText all give
// "[redacted]". Only [Token.Reveal] returns the token.
type Token struct {
	// A pointer, so that fmt prints an address and not the token when a
	// Token is an unexported field of a different struct.
	v *string
}

// New returns a Token that holds s. Only a [Source] calls it.
func New(s string) Token { return Token{v: &s} }

// Reveal returns the token. Call it only to give the token to the SDK.
func (t Token) Reveal() string {
	if t.v == nil {
		return ""
	}
	return *t.v
}

// Empty reports whether the token is empty.
func (t Token) Empty() bool { return t.Reveal() == "" }

// String returns "[redacted]".
func (t Token) String() string { return redacted }

// GoString returns "[redacted]".
func (t Token) GoString() string { return redacted }

// Format writes "[redacted]" for each verb.
func (t Token) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(redacted)) }

// LogValue returns "[redacted]" for package slog.
func (t Token) LogValue() slog.Value { return slog.StringValue(redacted) }

// MarshalJSON writes "[redacted]".
func (t Token) MarshalJSON() ([]byte, error) { return json.Marshal(redacted) }

// MarshalText writes "[redacted]".
func (t Token) MarshalText() ([]byte, error) { return []byte(redacted), nil }
