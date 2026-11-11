// Package audit defines the entries of the audit log and the interface of
// its store.
//
// The daemon writes each event when it occurs, also each failure. It
// delivers a value only after the entry for the delivery is written. If
// Append fails, the daemon delivers nothing, answers audit_failed and locks
// delivery for all sessions.
//
// The entries form a hash chain: the hash of an entry is SHA-256 over the
// hash of the entry before it and the canonical form of the entry. An
// entry never contains a secret value; it holds the references.
//
// The package holds types and the interface only. The SQLite store with
// the hash chain lives in a sub-package.
package audit

import (
	"context"
	"time"

	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// EventType is the kind of an event.
type EventType string

// The kinds of event.
const (
	// EventRequest: a request of an agent arrived.
	EventRequest EventType = "request"
	// EventBundle: the bundle window closed and the bundle is complete.
	EventBundle EventType = "bundle"
	// EventDecision: the approval UI gave a decision.
	EventDecision EventType = "decision"
	// EventGrant: the daemon created a grant.
	EventGrant EventType = "grant"
	// EventDelivery: the daemon delivers values. The daemon writes the
	// entry before it reads the values and before the answer.
	EventDelivery EventType = "delivery"
	// EventDenial: the policy or the human denied a request or an entry.
	EventDenial EventType = "denial"
	// EventRevocation: the human revoked a grant.
	EventRevocation EventType = "revocation"
	// EventExpiry: an approval request or a grant expired, or a grant
	// ended with its session.
	EventExpiry EventType = "expiry"
	// EventError: a fault occurred, for example a locked backend, a failed
	// UI, or the start and the end of the global delivery lock.
	EventError EventType = "error"
)

// EventTypes returns all kinds of event. The caller owns the slice.
func EventTypes() []EventType {
	return []EventType{
		EventRequest, EventBundle, EventDecision, EventGrant, EventDelivery,
		EventDenial, EventRevocation, EventExpiry, EventError,
	}
}

// Valid reports whether t is a known kind of event.
func (t EventType) Valid() bool {
	for _, known := range EventTypes() {
		if t == known {
			return true
		}
	}
	return false
}

// The values of [Result.Detail] that code compares. Each other detail is
// free text for the human.
const (
	// DetailFaultBegin marks the error event at the start of the global
	// delivery lock.
	DetailFaultBegin = "fault_begin"
	// DetailFaultEnd marks the error event at the end of the global
	// delivery lock.
	DetailFaultEnd = "fault_end"
	// DetailSessionEnded marks the expiry of a grant because the
	// connection of its session ended.
	DetailSessionEnded = "session_ended"
	// DetailDaemonStopped marks the expiry of a grant because the daemon
	// stopped on SIGINT or SIGTERM. A grant lives only in memory. An
	// ungraceful end (SIGKILL, crash) writes nothing.
	DetailDaemonStopped = "daemon_stopped"
	// DetailLifetimeEnded marks the expiry of a grant at the end of its
	// lifetime.
	DetailLifetimeEnded = "lifetime_ended"
)

// Event is what the daemon gives to the store. The store adds the number,
// the time and the hashes.
type Event struct {
	// Type is the kind of the event.
	Type EventType `json:"type"`
	// Verified holds the verified facts of the session, with the session
	// ID. It is the zero value for an event without a session, for example
	// the start of the global delivery lock.
	Verified session.Facts `json:"verified"`
	// Claims holds the unverified claims of the agent, with the session
	// name. The session name is in this field only.
	Claims session.Claims `json:"claims"`
	// References lists the credentials of the event. An entry never holds
	// a value.
	References []protocol.Reference `json:"references"`
	// Result holds the result of the event.
	Result Result `json:"result"`
}

// Result is the result of an event.
type Result struct {
	// Code is the stable error code of a failure. It is empty for a
	// success.
	Code protocol.Code `json:"code,omitempty"`
	// RequestID is the ID of the approval request, for the events bundle,
	// decision, denial and expiry of an approval request.
	RequestID string `json:"request_id,omitempty"`
	// GrantID is the grant of the event: the new grant, the grant that
	// covers a delivery, or the grant that ended.
	GrantID string `json:"grant_id,omitempty"`
	// LifetimeSeconds is the lifetime of a new grant.
	LifetimeSeconds int64 `json:"lifetime_seconds,omitempty"`
	// Detail is one of the Detail constants or a short English sentence.
	// It never contains a secret value.
	Detail string `json:"detail,omitempty"`
}

// Entry is one stored entry of the audit log.
type Entry struct {
	// Seq is the running number. The first entry has the number 1.
	Seq uint64 `json:"seq"`
	// Time is the time at which the store wrote the entry, in UTC.
	Time time.Time `json:"time"`
	// Event is the content.
	Event
	// PrevHash is the hash of the entry before, as lower-case hex. It is
	// 64 zeros for the first entry.
	PrevHash string `json:"prev_hash"`
	// Hash is SHA-256 over PrevHash and the canonical form of the entry,
	// as lower-case hex.
	Hash string `json:"hash"`
}

// Filter selects entries for List. A zero field selects all.
type Filter struct {
	// Project selects entries of one project (verified fact).
	Project string
	// SessionName selects entries of sessions that claim the name.
	SessionName string
	// Reference selects entries that name the credential.
	Reference *protocol.Reference
	// Types selects entries of the listed kinds.
	Types []EventType
	// Code selects entries with the error code.
	Code protocol.Code
	// Since selects entries at or after the time.
	Since time.Time
	// Until selects entries before the time.
	Until time.Time
	// Limit is the maximum number of entries. Zero means no limit. With a
	// limit, List returns the newest entries.
	Limit int
}

// Verification is the result of a check of the hash chain.
type Verification struct {
	// OK reports whether the chain is intact.
	OK bool
	// Entries is the number of entries that the check read.
	Entries uint64
	// FirstBadSeq is the number of the first entry whose hash or number
	// does not fit. It is zero if OK is true.
	FirstBadSeq uint64
	// Finding says what is wrong with the entry FirstBadSeq.
	Finding string
}

// Store is the audit log. All methods are safe for concurrent use.
type Store interface {
	// Append writes one event as the next entry of the chain and returns
	// the entry. The entry is durable when Append returns. An error means
	// that the entry is not written: the caller then delivers nothing,
	// answers audit_failed and locks delivery.
	Append(ctx context.Context, ev Event) (Entry, error)
	// List returns the entries that the filter selects, in the sequence of
	// their numbers.
	List(ctx context.Context, f Filter) ([]Entry, error)
	// Verify calculates the hash chain again over all entries. A broken
	// chain is a result with OK false, not an error. An error means that
	// the store could not read the entries.
	Verify(ctx context.Context) (Verification, error)
	// Close releases the store.
	Close() error
}
