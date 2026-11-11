// Package faultstore wraps an [audit.Store] so that a test can make Append
// fail. The broker tests use it for the case "the audit log cannot be
// written" (test T26). Production code does not import this package.
package faultstore

import (
	"context"
	"errors"
	"sync"

	"github.com/seletz/passierschein/internal/audit"
)

// ErrInjected is the default error of a failing Append.
var ErrInjected = errors.New("audit: injected write failure")

// Store forwards each call to the inner store, except Append while a fault
// is on. A failing Append writes nothing to the inner store.
type Store struct {
	audit.Store

	mu      sync.Mutex
	err     error
	skip    int
	when    func(audit.Event) bool
	failed  int
	appends int
}

var _ audit.Store = (*Store)(nil)

// New returns a wrapper around inner. At first it has no fault.
func New(inner audit.Store) *Store { return &Store{Store: inner} }

// FailAppends makes each following Append fail with err. A nil err means
// [ErrInjected].
func (s *Store) FailAppends(err error) { s.FailAfter(0, err) }

// FailAfter lets n more Append calls pass, then makes each following one
// fail with err. A nil err means [ErrInjected].
func (s *Store) FailAfter(n int, err error) {
	if err == nil {
		err = ErrInjected
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err, s.skip = err, n
}

// FailWhen makes Append fail with [ErrInjected] for each event for which
// match returns true, for example the events of one type. A nil match
// removes the rule.
func (s *Store) FailWhen(match func(audit.Event) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.when = match
}

// Heal removes each fault.
func (s *Store) Heal() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err, s.skip, s.when = nil, 0, nil
}

// Failures returns the number of Append calls that the wrapper failed.
func (s *Store) Failures() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failed
}

// Append fails while a fault is on, else it calls the inner store.
func (s *Store) Append(ctx context.Context, ev audit.Event) (audit.Entry, error) {
	s.mu.Lock()
	s.appends++
	var fail error
	switch {
	case s.when != nil && s.when(ev):
		fail = ErrInjected
	case s.err != nil && s.skip > 0:
		s.skip--
	case s.err != nil:
		fail = s.err
	}
	if fail != nil {
		s.failed++
	}
	s.mu.Unlock()
	if fail != nil {
		return audit.Entry{}, fail
	}
	return s.Store.Append(ctx, ev)
}

// Probe fails with the injected error while the next Append would fail:
// a fault is on and no Append may pass, or a rule of FailWhen is set.
// Else it calls Probe of the inner store, if the inner store has one.
func (s *Store) Probe(ctx context.Context) error {
	s.mu.Lock()
	var fail error
	switch {
	case s.when != nil:
		fail = ErrInjected
	case s.err != nil && s.skip == 0:
		fail = s.err
	}
	s.mu.Unlock()
	if fail != nil {
		return fail
	}
	if p, ok := s.Store.(interface{ Probe(context.Context) error }); ok {
		return p.Probe(ctx)
	}
	return nil
}
