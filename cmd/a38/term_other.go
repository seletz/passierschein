//go:build !darwin

package main

import (
	"errors"
	"io"
	"os"
)

// isTerminal reports false: "a38 config onepassword-token" exists on macOS only.
func isTerminal(*os.File) bool { return false }

// readSecretLine fails: "a38 config onepassword-token" exists on macOS only.
func readSecretLine(*os.File, io.Writer, string, int) ([]byte, error) {
	return nil, errors.New("not available on this platform")
}
