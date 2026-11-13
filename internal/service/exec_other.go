//go:build !darwin

package service

// NewLaunchctl fails: launchd exists on macOS only (D53).
func NewLaunchctl() (Launchctl, error) { return nil, ErrUnavailable }
