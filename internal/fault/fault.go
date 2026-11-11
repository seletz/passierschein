// Package fault holds the global delivery lock of the daemon (D48).
//
// While a fault is set, the daemon delivers no value and creates no grant,
// for no session and no project. Each request ends with daemon_faulted.
// Only the human removes the lock.
//
// This package is the seam for issue 23. Today it has the interfaces and
// [NoLock], an implementation that never locks. The broker already calls
// [Gate.Check] at the top of each delivery path and [Lock.Raise] at each
// failure of the audit log, so that issue 23 only has to give a [Lock] that
// raises and clears faults.
package fault

import (
	"context"

	"github.com/seletz/passierschein/internal/protocol"
)

// Gate tells whether delivery is open.
type Gate interface {
	// Check returns nil while delivery is open. While a fault locks
	// delivery, it returns an error with the code daemon_faulted.
	Check() *protocol.Error
}

// Lock is the global delivery lock. All methods are safe for concurrent
// use.
type Lock interface {
	Gate
	// Raise sets a fault. code is the code of the cause, for example
	// audit_failed. reason is one English sentence without a secret value.
	// A second Raise while a fault is set keeps the first fault.
	Raise(code protocol.Code, reason string)
	// Fault returns the current fault, or nil if delivery is open.
	Fault() *protocol.Fault
	// Clear removes the fault if the check of the cause passes. It returns
	// whether delivery is open after the call and, if not, the finding.
	Clear(ctx context.Context) (cleared bool, finding string)
}

// NoLock is a [Lock] that never locks. Raise does nothing. It is the lock
// of stage 1 until issue 23 replaces it.
type NoLock struct{}

var _ Lock = NoLock{}

// Check returns nil: delivery is always open.
func (NoLock) Check() *protocol.Error { return nil }

// Raise does nothing.
func (NoLock) Raise(protocol.Code, string) {}

// Fault returns nil.
func (NoLock) Fault() *protocol.Fault { return nil }

// Clear reports that delivery is open.
func (NoLock) Clear(context.Context) (bool, string) { return true, "" }
