// Package fakeui holds the implementations of [approval.UI] that replace
// the approval app in tests: auto-approve, auto-deny and script, and a
// recording fake for in-process tests.
//
// A faked UI gives a decision without a human. The daemon therefore starts
// with one only together with the fake backend ([CheckCombination], D40).
//
// Every failure of a faked UI is an error or the outcome timeout. It is
// never an approval (fail closed, D35).
package fakeui

import (
	"errors"
	"fmt"
	"time"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/clock"
)

// ErrNotImplemented is returned for the mode "app". The approval app is
// part of stage 2.
var ErrNotImplemented = errors.New("approval mode \"app\" is not implemented in stage 1")

// ErrForbiddenCombination is returned for a faked UI with a real backend.
var ErrForbiddenCombination = errors.New("a faked approval is allowed only with the fake backend")

// Options holds the settings for [New].
type Options struct {
	// ScriptCommand is the program and its arguments for the mode script.
	ScriptCommand []string
	// ScriptTimeout is the time that the script has for one request. Zero
	// means [approval.DefaultTimeout].
	ScriptTimeout time.Duration
	// Clock is the time source. Nil means the system clock.
	Clock clock.Clock
}

// CheckCombination enforces D40: with a real backend only the mode app is
// allowed. Each other combination is an error. The daemon calls it before
// it opens a socket. The caller passes backend.Kind.Real() as realBackend.
//
// An unknown mode is an error with each backend.
func CheckCombination(realBackend bool, mode approval.Mode) error {
	switch mode {
	case approval.ModeApp:
		return nil
	case approval.ModeAutoApprove, approval.ModeAutoDeny, approval.ModeScript:
		if realBackend {
			return fmt.Errorf("%w: --ui %s with a real backend", ErrForbiddenCombination, mode)
		}
		return nil
	default:
		return fmt.Errorf("unknown approval mode %q", mode)
	}
}

// New returns the UI for a mode. The mode app returns [ErrNotImplemented]
// in stage 1.
func New(mode approval.Mode, opts Options) (approval.UI, error) {
	switch mode {
	case approval.ModeApp:
		return nil, ErrNotImplemented
	case approval.ModeAutoApprove:
		return AutoApprove{}, nil
	case approval.ModeAutoDeny:
		return AutoDeny{}, nil
	case approval.ModeScript:
		return NewScript(opts.ScriptCommand, opts.ScriptTimeout, opts.Clock)
	default:
		return nil, fmt.Errorf("unknown approval mode %q", mode)
	}
}
