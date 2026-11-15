//go:build darwin

package main

import (
	"context"

	"github.com/seletz/passierschein/internal/backend/onepassword"
)

// selftestSDKCore runs the WebAssembly core of the 1Password SDK once.
func selftestSDKCore(ctx context.Context) error {
	return onepassword.SelfTest(ctx)
}
