//go:build darwin

package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// execLaunchctl runs the real /bin/launchctl.
type execLaunchctl struct{}

// NewLaunchctl returns the real launchctl: the fixed path
// [LaunchctlPath], no shell, a time limit of [LaunchctlTimeout] for each
// call and only the subcommands that the package uses.
func NewLaunchctl() (Launchctl, error) { return execLaunchctl{}, nil }

func (execLaunchctl) Run(ctx context.Context, args ...string) (Result, error) {
	if err := checkArgs(args); err != nil {
		return Result{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, LaunchctlTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, LaunchctlPath, args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin"}
	cmd.Dir = "/"
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	r := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case ctx.Err() != nil:
		return r, fmt.Errorf("launchctl %s did not end within %s", args[0], LaunchctlTimeout)
	case errors.As(err, &exitErr):
		r.Code = exitErr.ExitCode()
		return r, nil
	case err != nil:
		return r, fmt.Errorf("launchctl %s: %w", args[0], err)
	}
	return r, nil
}
