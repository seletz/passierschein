// Package bundle collects the credentials of one session in the bundle
// window (F2: D5, D29).
//
// The window starts with the first request of a session that no grant
// covers. It has a fixed end: a later request does not extend it. Each
// request of the session before the end joins the bundle. A bundle holds at
// most a maximum number of entries; the first entries in the sequence of
// arrival stay, each further entry is rejected.
//
// A Bundle is a data structure only. It is not safe for concurrent use; the
// caller holds a lock. The caller also owns the timer of the window.
package bundle

import (
	"time"

	"github.com/seletz/passierschein/internal/protocol"
)

// Bundle is the open bundle of one session.
type Bundle struct {
	deadline time.Time
	max      int
	refs     []protocol.Reference
	index    map[protocol.Reference]bool
	closed   bool
}

// New returns a bundle whose window starts at now and lasts window.
// maxEntries is the maximum number of entries; a value below 1 means 1.
func New(now time.Time, window time.Duration, maxEntries int) *Bundle {
	return &Bundle{
		deadline: now.Add(window),
		max:      max(1, maxEntries),
		index:    map[protocol.Reference]bool{},
	}
}

// Deadline returns the end of the window. It never changes.
func (b *Bundle) Deadline() time.Time { return b.deadline }

// Accepts reports whether a request that arrives at now can join. A
// request at the end of the window or later opens a new bundle.
func (b *Bundle) Accepts(now time.Time) bool {
	return !b.closed && now.Before(b.deadline)
}

// Add adds the references in their sequence and reports for each one
// whether it is part of the bundle after the call. A reference that is
// already part of the bundle is accepted and uses no further place. A new
// reference is rejected when the bundle is full. Add on a closed bundle
// rejects each reference.
func (b *Bundle) Add(refs []protocol.Reference) []bool {
	ok := make([]bool, len(refs))
	for i, ref := range refs {
		switch {
		case b.closed:
		case b.index[ref]:
			ok[i] = true
		case len(b.refs) < b.max:
			b.index[ref] = true
			b.refs = append(b.refs, ref)
			ok[i] = true
		}
	}
	return ok
}

// Len returns the number of entries.
func (b *Bundle) Len() int { return len(b.refs) }

// Close ends the bundle and returns its entries in the sequence of
// arrival. After Close the bundle accepts nothing.
func (b *Bundle) Close() []protocol.Reference {
	b.closed = true
	return append([]protocol.Reference(nil), b.refs...)
}
