package token

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// The kinds of error of a [Store]. An error of a store wraps one of them,
// so that a caller can use errors.Is. No error contains the token.
var (
	// ErrNotFound: the store has no token.
	ErrNotFound = errors.New("the store has no token")
	// ErrExists: the store has a token already, and the caller did not ask
	// to replace it.
	ErrExists = errors.New("the store has a token already")
	// ErrInteraction: the store gives or changes the token only after a
	// confirmation of the human, or it refuses the access.
	ErrInteraction = errors.New("the store refuses the access or needs a confirmation of the human")
	// ErrUnavailable: the store does not exist on this platform.
	ErrUnavailable = errors.New("not available on this platform")
)

// Store keeps the token for the daemon, for example the macOS keychain
// (D51). The daemon reads it through [Source]; the human changes it with
// "a38 config onepassword-token".
type Store interface {
	Source
	// Name names the place of the token for an output line, for example
	// the service and the account of the keychain item.
	Name() string
	// Put stores t. If the store has a token already, Put returns an error
	// that wraps [ErrExists], unless replace is true.
	Put(t Token, replace bool) error
	// Delete removes the token. If the store has none, Delete returns an
	// error that wraps [ErrNotFound].
	Delete() error
	// Describe returns what the store knows about the token, without the
	// token. It never asks the human.
	Describe() (StoreInfo, error)
	// Signature returns the code signature of the running program. The
	// keychain binds the token to it.
	Signature() (Signature, error)
}

// StoreInfo describes the stored token without the token.
type StoreInfo struct {
	// Name names the place of the token, for example the service and the
	// account of the keychain item.
	Name string `json:"name"`
	// Exists is true if the store has a token.
	Exists bool `json:"exists"`
	// Created is the time of creation; zero if unknown.
	Created time.Time `json:"created,omitzero"`
	// Modified is the time of the last change; zero if unknown.
	Modified time.Time `json:"modified,omitzero"`
	// Trusted lists the programs that can read the token without a
	// confirmation. The entry AnyProgram means: each program.
	Trusted []string `json:"trusted_programs,omitempty"`
	// Partitions lists the partition list of the keychain item, for
	// example "cdhash:…" or "teamid:…".
	Partitions []string `json:"partitions,omitempty"`
	// ReadableWithoutDialog is true only if the access list shows that
	// the running program reads the token without a dialog: the item trusts
	// the path of this program and its partition list names the hash of
	// exactly this build.
	ReadableWithoutDialog bool `json:"readable_without_dialog"`
	// AccessListError says why the access list cannot be read; empty if
	// it was read.
	AccessListError string `json:"access_list_error,omitempty"`
}

// AnyProgram in [StoreInfo.Trusted] means: each program can read the token.
const AnyProgram = "(any program)"

// SignatureKind is the kind of the code signature of a program.
type SignatureKind string

// The kinds of a code signature.
const (
	// SignatureUnknown: the program cannot read its own signature.
	SignatureUnknown SignatureKind = "unknown"
	// SignatureNone: the program is not signed.
	SignatureNone SignatureKind = "unsigned"
	// SignatureAdhoc: an ad-hoc signature without certificate, for
	// example the signature of the Go linker.
	SignatureAdhoc SignatureKind = "ad-hoc"
	// SignatureDeveloperID: a signature with a Developer ID certificate.
	SignatureDeveloperID SignatureKind = "developer-id"
	// SignatureOther: a signature with another certificate, for example a
	// development certificate.
	SignatureOther SignatureKind = "other-certificate"
)

// Signature describes the code signature of the running program.
type Signature struct {
	// Path is the path of the program.
	Path string `json:"path"`
	// Kind is the kind of the signature.
	Kind SignatureKind `json:"kind"`
	// TeamID is the team of the certificate; empty without certificate.
	TeamID string `json:"team_id,omitempty"`
	// Identifier is the signing identifier.
	Identifier string `json:"identifier,omitempty"`
	// CDHash is the hash of the code directory in hex.
	CDHash string `json:"cdhash,omitempty"`
}

// Stable reports whether the keychain can recognize a new build of the
// program as the same program: only a Developer ID signature is stable.
func (s Signature) Stable() bool { return s.Kind == SignatureDeveloperID }

// Limits of a token for [CheckShape]. A token of a 1Password service
// account is about 1 KiB long.
const (
	// MinLength is the shortest token that CheckShape accepts.
	MinLength = 16
	// MaxLength is the longest token that CheckShape accepts.
	MaxLength = 4096
)

// ExpectedPrefix starts each token of a 1Password service account. This is
// an assumption from the spike (issue 53); a token with another start
// gives a warning only.
const ExpectedPrefix = "ops_"

// CheckShape checks the form of a token without the service: length, no
// white space and no control character. It returns warnings that do not
// stop the use of the token. No error and no warning contains the token.
func CheckShape(s string) (warnings []string, err error) {
	switch {
	case s == "":
		return nil, errors.New("the token is empty")
	case !utf8.ValidString(s):
		return nil, errors.New("the token is not valid UTF-8")
	case len(s) < MinLength:
		// No length in the text: only metadata are shown, never a value
		// derived from the token (Stefan, 2026-11-12).
		return nil, errors.New("the token is too short; check that the paste is complete")
	case len(s) > MaxLength:
		return nil, fmt.Errorf("the token is too long (more than %d bytes)", MaxLength)
	}
	for _, r := range s {
		if unicode.IsSpace(r) {
			return nil, errors.New("the token contains white space")
		}
		if unicode.IsControl(r) || !unicode.IsPrint(r) {
			return nil, errors.New("the token contains a control character")
		}
	}
	if !strings.HasPrefix(s, ExpectedPrefix) {
		warnings = append(warnings, fmt.Sprintf("the token does not start with %q; a token of a 1Password service account usually does", ExpectedPrefix))
	}
	return warnings, nil
}

// UnstableSignatureWarning explains the limit of a keychain item for a
// program without a Developer ID signature (spike in issue 53, G5).
const UnstableSignatureWarning = "each new build of a38 needs a confirmation in a macOS dialog; another program can ask for the same confirmation; the protection against a process of the same user starts with the Developer ID signature (G5)"
