//go:build !darwin

package main

import (
	"context"
	"errors"
)

// selftestSDKCore fails: the linux build of a38 contains no code for
// 1Password (D2, D42, T13).
func selftestSDKCore(context.Context) error {
	return errors.New("the 1Password SDK is not available on this platform; it is part of the macOS build only")
}
