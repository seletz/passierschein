// Package backend defines the interface between the daemon and a vault
// service. The daemon reads names and secret values only through
// [Backend].
//
// Implementations live in sub-packages: the fake backend reads a YAML file;
// the 1Password backend is behind a build tag and is not part of the linux
// build.
package backend

import (
	"context"
	"errors"

	"github.com/seletz/passierschein/internal/protocol"
)

// Kind names a backend implementation. It is the value of the flag
// "--backend" of "a38 serve".
type Kind string

// The backend kinds.
const (
	// KindFake reads vaults from a YAML file. It is for tests and local
	// development.
	KindFake Kind = "fake"
	// KindOnePassword uses the 1Password SDK. The daemon starts with it
	// only together with the approval app.
	KindOnePassword Kind = "1password"
)

// Real reports whether the backend kind holds real secrets. With a real
// backend, a faked approval is forbidden.
func (k Kind) Real() bool { return k != KindFake }

// The error states of a backend. An implementation wraps one of them, so
// that errors.Is finds it. An error of a backend never contains a secret
// value.
var (
	// ErrLocked: the backend is locked. The human must unlock it.
	ErrLocked = errors.New("backend: the backend is locked")
	// ErrUnavailable: the daemon cannot reach the backend.
	ErrUnavailable = errors.New("backend: the backend is not available")
	// ErrNotFound: the vault, the item or the field does not exist.
	ErrNotFound = errors.New("backend: the vault, item or field does not exist")
)

// Names holds the names of a credential as the backend stores them. The
// approval request shows these names, not the text of the agent.
type Names struct {
	// Vault is the stored name of the vault.
	Vault string
	// Item is the stored title of the item.
	Item string
	// Field is the stored label of the field.
	Field string
}

// Backend gives access to the vaults. All methods are safe for concurrent
// use and obey the context.
//
// The daemon calls a Backend only for a vault that the configuration lists
// and that the policy of the project allows. A request outside the policy
// never reaches the backend.
type Backend interface {
	// Vaults returns the names of the vaults that the backend can see. It
	// reads no secret value.
	Vaults(ctx context.Context) ([]string, error)
	// Resolve returns the stored names for a reference. It reads no secret
	// value. It returns ErrNotFound if the vault, the item or the field
	// does not exist.
	Resolve(ctx context.Context, ref protocol.Reference) (Names, error)
	// Read returns the value of the field. The daemon calls it only for
	// the delivery, after the audit entry for the delivery is written. The
	// backend does not keep the value in a cache.
	Read(ctx context.Context, ref protocol.Reference) (protocol.Secret, error)
}

// Code returns the stable error code for an error of a backend:
// backend_locked, not_found, or backend_unavailable for each other error.
// It returns the empty code for a nil error.
func Code(err error) protocol.Code {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrLocked):
		return protocol.CodeBackendLocked
	case errors.Is(err, ErrNotFound):
		return protocol.CodeNotFound
	default:
		return protocol.CodeBackendUnavailable
	}
}
