//go:build darwin

package onepassword

import (
	"context"
	"errors"

	sdk "github.com/1password/onepassword-sdk-go"

	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/token"
)

// integrationName is the name of a38 that the SDK sends to 1Password.
const integrationName = "a38"

// connectSDK builds the client of the SDK with the token of the service
// account. It never uses the desktop app integration (D39).
func connectSDK(ctx context.Context, token token.Token, version string) (api, error) {
	c, err := sdk.NewClient(ctx,
		sdk.WithServiceAccountToken(token.Reveal()),
		sdk.WithIntegrationInfo(integrationName, version),
	)
	if err != nil {
		return nil, sdkErr(err)
	}
	return sdkAPI{c: c}, nil
}

// selfTestConnect builds a client of the SDK with the invalid token of
// [SelfTest]. This loads and runs the WebAssembly core of the SDK.
func selfTestConnect(ctx context.Context) error {
	_, err := sdk.NewClient(ctx,
		sdk.WithServiceAccountToken(selfTestToken),
		sdk.WithIntegrationInfo(integrationName, "selftest"),
	)
	return err
}

// sdkAPI is the adapter from the SDK to [api].
type sdkAPI struct{ c *sdk.Client }

// sdkErr turns the typed errors of the SDK into errors of the package.
func sdkErr(err error) error {
	var rl *sdk.RateLimitExceededError
	if errors.As(err, &rl) {
		return errRateLimit
	}
	return err
}

func (s sdkAPI) ListVaults(ctx context.Context) ([]vaultInfo, error) {
	vs, err := s.c.Vaults().List(ctx)
	if err != nil {
		return nil, sdkErr(err)
	}
	out := make([]vaultInfo, 0, len(vs))
	for _, v := range vs {
		out = append(out, vaultInfo{ID: v.ID, Title: v.Title})
	}
	return out, nil
}

func (s sdkAPI) ListItems(ctx context.Context, vaultID string) ([]itemInfo, error) {
	active := sdk.NewItemListFilterTypeVariantByState(&sdk.ItemListFilterByStateInner{Active: true})
	items, err := s.c.Items().List(ctx, vaultID, active)
	if err != nil {
		return nil, sdkErr(err)
	}
	out := make([]itemInfo, 0, len(items))
	for _, it := range items {
		if it.State != "" && it.State != sdk.ItemStateActive {
			continue
		}
		out = append(out, itemInfo{ID: it.ID, Title: it.Title})
	}
	return out, nil
}

func (s sdkAPI) GetItem(ctx context.Context, vaultID, itemID string) (itemData, error) {
	it, err := s.c.Items().Get(ctx, vaultID, itemID)
	if err != nil {
		return itemData{}, sdkErr(err)
	}
	d := itemData{ID: it.ID, Title: it.Title, VaultID: it.VaultID}
	for _, f := range it.Fields {
		d.Fields = append(d.Fields, fieldInfo{ID: f.ID, Title: f.Title, Value: protocol.NewSecret(f.Value)})
	}
	return d, nil
}
