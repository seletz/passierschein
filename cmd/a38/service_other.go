//go:build !darwin

package main

import (
	"context"
	"fmt"
	"io"

	"github.com/seletz/passierschein/internal/doctor"
)

// runService fails: launchd and the daemon exist on macOS only (D53). The
// linux build does not contain the package service.
func runService(_ []string, _, stderr io.Writer) int {
	_, _ = fmt.Fprintln(stderr, "a38 service: not available on this platform; the daemon runs on macOS only")
	return exitFailure
}

// newServiceState is nil: there is no LaunchAgent.
var newServiceState = func(context.Context) *doctor.ServiceState { return nil }
