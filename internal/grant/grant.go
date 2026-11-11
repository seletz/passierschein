// Package grant keeps the grants of the daemon in memory (F5: D8, D32,
// D33).
//
// A grant belongs to exactly one session and covers exactly the approved
// credentials, for a lifetime. Grants live only in memory: a new daemon
// starts with no grant.
//
// A grant is live while the time is before its end. Each lookup takes the
// time and ignores a grant that is not live, also if [Store.Expired] did
// not remove it yet. A grant can therefore never cover a request after the
// end of its lifetime.
//
// A Store is not safe for concurrent use; the caller holds a lock.
package grant

import (
	"slices"
	"time"

	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// Grant is one grant.
type Grant struct {
	// ID identifies the grant.
	ID string
	// Facts holds the verified facts of the session that holds the grant.
	// Facts.SessionID is the holder.
	Facts session.Facts
	// Claims holds the claims of the session. Only the session name and
	// the agent name are set.
	Claims session.Claims
	// Refs lists the covered credentials with the names of the backend.
	Refs []protocol.Reference
	// GrantedAt is the time of the approval.
	GrantedAt time.Time
	// ExpiresAt is the end of the lifetime.
	ExpiresAt time.Time
}

// Live reports whether the grant is live at now.
func (g Grant) Live(now time.Time) bool { return now.Before(g.ExpiresAt) }

// Covers reports whether the grant covers ref. The comparison is exact.
func (g Grant) Covers(ref protocol.Reference) bool { return slices.Contains(g.Refs, ref) }

// Store holds the grants.
type Store struct {
	grants []Grant
}

// NewStore returns an empty store.
func NewStore() *Store { return &Store{} }

// Add adds g. The caller gives each grant a unique ID.
func (s *Store) Add(g Grant) {
	g.Refs = slices.Clone(g.Refs)
	s.grants = append(s.grants, g)
}

// Cover returns the newest live grant of the session that covers ref. A
// grant of a different session never covers a request.
func (s *Store) Cover(sid session.ID, ref protocol.Reference, now time.Time) (Grant, bool) {
	for i := len(s.grants) - 1; i >= 0; i-- {
		g := s.grants[i]
		if g.Facts.SessionID == sid && g.Live(now) && g.Covers(ref) {
			return g, true
		}
	}
	return Grant{}, false
}

// Live returns the live grants of the session, oldest first.
func (s *Store) Live(sid session.ID, now time.Time) []Grant {
	var out []Grant
	for _, g := range s.grants {
		if g.Facts.SessionID == sid && g.Live(now) {
			out = append(out, g)
		}
	}
	return out
}

// All returns the live grants of all sessions, oldest first.
func (s *Store) All(now time.Time) []Grant {
	var out []Grant
	for _, g := range s.grants {
		if g.Live(now) {
			out = append(out, g)
		}
	}
	return out
}

// Remove removes the live grant with the ID and returns it. A grant that
// is not live is not found and stays for [Store.Expired].
func (s *Store) Remove(id string, now time.Time) (Grant, bool) {
	for i, g := range s.grants {
		if g.ID == id && g.Live(now) {
			s.grants = slices.Delete(s.grants, i, i+1)
			return g, true
		}
	}
	return Grant{}, false
}

// RemoveSession removes each grant of the session and returns the grants
// that were live. The caller calls [Store.Expired] first, so that a grant
// that ended with its lifetime gets its own event.
func (s *Store) RemoveSession(sid session.ID, now time.Time) []Grant {
	var ended []Grant
	s.grants = slices.DeleteFunc(s.grants, func(g Grant) bool {
		if g.Facts.SessionID != sid {
			return false
		}
		if g.Live(now) {
			ended = append(ended, g)
		}
		return true
	})
	return ended
}

// Expired removes each grant that is not live at now and returns them.
func (s *Store) Expired(now time.Time) []Grant {
	var expired []Grant
	s.grants = slices.DeleteFunc(s.grants, func(g Grant) bool {
		if g.Live(now) {
			return false
		}
		expired = append(expired, g)
		return true
	})
	return expired
}
