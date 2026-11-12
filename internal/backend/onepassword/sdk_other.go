//go:build !darwin

package onepassword

import (
	"context"

	"github.com/seletz/passierschein/internal/token"
)

// connectSDK fails: the SDK is part of the macOS build only (D42).
func connectSDK(context.Context, token.Token, string) (api, error) {
	return nil, errNotAvailable
}
