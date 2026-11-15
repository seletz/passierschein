//go:build darwin

package main

import (
	"context"

	"github.com/seletz/passierschein/internal/backend/onepassword"
	"github.com/seletz/passierschein/internal/version"
)

// selftestSDKCore runs the WebAssembly core of the 1Password SDK once.
func selftestSDKCore(ctx context.Context) error {
	return onepassword.SelfTest(ctx, version.Version)
}
