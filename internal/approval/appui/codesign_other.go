//go:build !darwin

package appui

// SystemCodeSigner returns nil: only macOS has codesign. The daemon runs
// with the approval app on macOS only (D53); elsewhere the signature check
// is off.
func SystemCodeSigner() CodeSigner { return nil }
