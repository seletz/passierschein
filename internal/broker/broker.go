// Package broker decides whether the daemon delivers a credential. It runs
// one request from the policy to the delivery and implements
// daemon.AgentHandler and daemon.AdminHandler.
//
// # Flow of a request
//
//  1. The global delivery lock (D48) is open.
//  2. The request is valid. The broker writes the audit event "request".
//  3. The policy allows each reference (D12, D24, D28, D47). Else the whole
//     request ends with not_allowed or config_invalid. Nothing reaches the
//     backend or the human.
//  4. The backend resolves each reference to its stored names. It reads no
//     value. An unknown item or field ends the whole request with
//     not_found (D27).
//  5. A live grant of the session covers a reference, or the reference
//     joins the bundle of the session (D5, D29). A full bundle rejects the
//     entry with bundle_too_large.
//  6. At the end of the window the bundle becomes one approval request
//     (D7, D30). The broker validates the decision itself (D32).
//  7. An approval creates one grant for the approved credentials (D8, D32).
//  8. The broker writes the audit event "delivery", then reads the values
//     and answers (D34, D36).
//
// # Invariants
//
//   - A value leaves the broker only for a reference that a live grant of
//     the same session covers at the time of delivery. The broker checks
//     this under its lock in the same step in which it writes the audit
//     event "delivery".
//   - The broker reads a value only after the audit event "delivery" is
//     written. A failed audit write ends the request with audit_failed and
//     reads nothing (D36).
//   - A grant exists only after its audit event "grant" is written.
//   - Each failure ends with a stable code and an audit event (D35). An
//     error of a whole request delivers no value at all.
//   - No value is kept after the answer, and no value goes to a log, an
//     audit event, an error or an approval request (D34). The broker holds
//     a value only as a protocol.Secret.
//   - The project of a session comes from the verified facts, never from a
//     claim (D4, D25).
package broker

import (
	"context"
	"crypto/rand"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/clock"
	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/fault"
	"github.com/seletz/passierschein/internal/grant"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// SweepInterval is the time between two runs of the expiry sweep in
// [Broker.Run].
const SweepInterval = 30 * time.Second

// Option configures a [Broker].
type Option func(*Broker)

// WithLock sets the global delivery lock. The default is [fault.NoLock].
func WithLock(l fault.Lock) Option {
	return func(b *Broker) { b.faults = l }
}

// Broker is the decision half of the daemon. All methods are safe for
// concurrent use.
type Broker struct {
	cfg    *config.Config
	be     backend.Backend
	ui     approval.UI
	audit  audit.Store
	clock  clock.Clock
	log    *slog.Logger
	faults fault.Lock

	// slot lets only one approval request reach the UI at a time (D30).
	// The UI that the caller gives is also a queue with the timeout;
	// this slot keeps the rule also if it is not.
	slot chan struct{}

	// mu guards the fields below. The broker holds mu while it writes the
	// audit events that change or use grants (grant, delivery,
	// revocation, expiry), so that the sequence in the audit log is the
	// sequence of the changes. It never holds mu while it calls the UI or
	// the backend.
	mu       sync.Mutex
	grants   *grant.Store
	sessions map[session.ID]*sessionState
	notified map[string]bool
}

// sessionState is the state of one session that has sent a request.
type sessionState struct {
	// ctx ends when the session ends. Open bundles and approval requests
	// of the session use it.
	ctx    context.Context
	cancel context.CancelFunc
	// ended is set by SessionEnded. A grant is never created for an
	// ended session.
	ended bool
	// open is the bundle whose window is open, or nil.
	open *pendingBundle
}

// New returns a broker. ui must serialise the approval requests and end
// each one after the wait time (approval/queue); the daemon wires it. clk
// nil means the system clock, logger nil means a logger that discards.
//
// New starts no goroutine. The daemon calls [Broker.Run] for the expiry
// sweep.
func New(cfg *config.Config, be backend.Backend, ui approval.UI, store audit.Store, clk clock.Clock, logger *slog.Logger, opts ...Option) *Broker {
	if clk == nil {
		clk = clock.System{}
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	b := &Broker{
		cfg:      cfg,
		be:       be,
		ui:       ui,
		audit:    store,
		clock:    clk,
		log:      logger,
		faults:   fault.NoLock{},
		slot:     make(chan struct{}, 1),
		grants:   grant.NewStore(),
		sessions: map[session.ID]*sessionState{},
		notified: map[string]bool{},
	}
	for _, o := range opts {
		o(b)
	}
	return b
}

// Run removes each grant at the end of its lifetime and writes its audit
// event, every [SweepInterval], until ctx ends. A lookup never uses a grant
// after its end, also without Run; Run makes the audit event timely.
func (b *Broker) Run(ctx context.Context) {
	for {
		t := b.clock.NewTimer(SweepInterval)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C():
			b.sweep()
		}
	}
}

// sweep removes the grants at the end of their lifetime.
func (b *Broker) sweep() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.expireLocked()
}

// expireLocked removes the grants at the end of their lifetime and writes
// one expiry event for each. The caller holds mu.
func (b *Broker) expireLocked() {
	for _, g := range b.grants.Expired(b.clock.Now()) {
		_ = b.record(audit.Event{
			Type:       audit.EventExpiry,
			Verified:   g.Facts,
			Claims:     g.Claims,
			References: g.Refs,
			Result:     audit.Result{GrantID: g.ID, Detail: audit.DetailLifetimeEnded},
		})
	}
}

// state returns the state of the session and creates it if necessary. The
// caller holds mu.
func (b *Broker) stateLocked(s session.Session) *sessionState {
	st, ok := b.sessions[s.Facts.SessionID]
	if !ok {
		ctx, cancel := context.WithCancel(context.Background())
		st = &sessionState{ctx: ctx, cancel: cancel}
		b.sessions[s.Facts.SessionID] = st
	}
	return st
}

// record writes one audit event. The write does not depend on the context
// of a request: an event is written also when the connection ended. A
// failure is an audit_failed error; the caller then delivers nothing.
func (b *Broker) record(ev audit.Event) *protocol.Error {
	if _, err := b.audit.Append(context.Background(), ev); err != nil {
		return b.auditFailed(err)
	}
	return nil
}

// auditFailed handles a failed audit write: it logs the error and raises
// the global fault (D36, D48). It returns the error for the request.
func (b *Broker) auditFailed(err error) *protocol.Error {
	// The error of the store holds no value: an event holds none.
	b.log.Error("cannot write the audit log", "error", err)
	b.faults.Raise(protocol.CodeAuditFailed, "The daemon cannot write the audit log.")
	return protocol.Errorf(protocol.CodeAuditFailed, "the daemon cannot write the audit log; nothing is delivered")
}

// newID returns a new random identifier with the prefix. It is a valid ID
// of the wire protocol.
func newID(prefix string) string {
	return prefix + "-" + strings.ToLower(rand.Text())
}

// sessionEndedError is the answer to a request whose session ended while
// the request was open. The connection is gone; the code is for the log.
func sessionEndedError() *protocol.Error {
	return protocol.Errorf(protocol.CodeDenied, "the session ended before the decision")
}
