//go:build !darwin

package onepassword

import (
	"context"

	"github.com/seletz/passierschein/internal/tokenfile"
)

// connectSDK fails: the SDK is part of the macOS build only (D42).
func connectSDK(context.Context, tokenfile.Token, string) (api, error) {
	return nil, errNotAvailable
}
