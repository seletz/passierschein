package onepassword

import (
	"context"
	"errors"

	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/token"
)

// api is the small part of the 1Password SDK that the backend uses. It
// holds no SDK type, so that the tests can fake it (spike 10, section 6).
// The real implementation is in sdk_darwin.go.
type api interface {
	// ListVaults returns the vaults that the service account can see. It
	// reads no item.
	ListVaults(ctx context.Context) ([]vaultInfo, error)
	// ListItems returns the active items of one vault without fields and
	// without values.
	ListItems(ctx context.Context, vaultID string) ([]itemInfo, error)
	// GetItem returns one item with its fields and values. The SDK has no
	// call that returns the field labels without the values.
	GetItem(ctx context.Context, vaultID, itemID string) (itemData, error)
}

// connectFunc builds the client of the SDK. It can call the network.
type connectFunc func(ctx context.Context, token token.Token, version string) (api, error)

// vaultInfo is the overview of one vault.
type vaultInfo struct {
	ID    string
	Title string
}

// itemInfo is the overview of one item.
type itemInfo struct {
	ID    string
	Title string
}

// fieldInfo is one field of an item. Value is the secret value.
type fieldInfo struct {
	ID    string
	Title string
	Value protocol.Secret
}

// itemData is one item with its fields.
type itemData struct {
	ID      string
	Title   string
	VaultID string
	Fields  []fieldInfo
}

// errRateLimit is the error of the adapter for a rate limit of 1Password.
var errRateLimit = errors.New("rate limit of 1Password exceeded")

// errNotAvailable is the error of connect on a platform without the SDK.
var errNotAvailable = errors.New("the 1Password backend is not available on this platform; it is part of the macOS build only")
