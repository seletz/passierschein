package broker

import (
	"context"
	"time"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/bundle"
	"github.com/seletz/passierschein/internal/policy"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// call holds one request while it runs through the broker.
type call struct {
	s      session.Session
	req    *protocol.RequestCredentials
	claims session.Claims
	// canon maps each reference of the request to the reference with
	// the names of the backend. A grant and the audit events after the
	// request use these references.
	canon map[protocol.Reference]protocol.Reference
	names map[protocol.Reference]backend.Names
}

// RequestCredentials implements daemon.AgentHandler. See the package
// documentation for the flow.
func (b *Broker) RequestCredentials(ctx context.Context, s session.Session, req *protocol.RequestCredentials) (*protocol.Credentials, *protocol.Error) {
	b.sweep()
	// Gate of the delivery path (D48): with a fault nothing starts.
	if e := b.faults.Check(); e != nil {
		return nil, e
	}
	c := &call{s: s, req: req}
	if e := b.admit(c); e != nil {
		return nil, e
	}
	if e := b.resolve(ctx, c); e != nil {
		return nil, e
	}
	return b.dispatch(ctx, c)
}

// admit checks the request and the policy. It writes the event "request"
// and, for a rejection, the event "denial".
func (b *Broker) admit(c *call) *protocol.Error {
	if c.req == nil || c.s.Facts.SessionID == "" {
		// A request without a session ID would share grants with each
		// other such request.
		return b.reject(c, audit.EventDenial, nil, protocol.Errorf(protocol.CodeInvalidRequest, "the request has no session"))
	}
	if err := c.req.Validate(); err != nil {
		// The references can be invalid; the event does not name them.
		return b.reject(c, audit.EventDenial, nil, asProtocolError(err))
	}
	c.claims = c.s.Claims(c.req.Task, c.req.Reason)
	if e := b.record(b.event(c, audit.EventRequest, c.req.References, audit.Result{})); e != nil {
		return e
	}
	e := policy.Check(b.cfg, c.s.Facts.Project, c.req.References)
	if e == nil {
		return nil
	}
	if e.Code == protocol.CodeConfigInvalid {
		b.notifyInvalidProject(c.s.Facts.Project)
	}
	return b.reject(c, audit.EventDenial, c.req.References, e)
}

// resolve gets the names of the backend for each reference. It reads no
// value. Each failure ends the whole request: not_found, backend_locked or
// backend_unavailable.
func (b *Broker) resolve(ctx context.Context, c *call) *protocol.Error {
	c.canon = map[protocol.Reference]protocol.Reference{}
	c.names = map[protocol.Reference]backend.Names{}
	for _, ref := range c.req.References {
		if _, done := c.canon[ref]; done {
			continue
		}
		names, err := b.be.Resolve(ctx, ref)
		if err != nil {
			if ctx.Err() != nil {
				// The connection ended, not the backend.
				return b.requestEnded(c, c.req.References)
			}
			return b.backendFailed(c, err)
		}
		canon := protocol.Reference{Vault: names.Vault, Item: names.Item, Field: names.Field}
		// The vault name is compared exactly (D12, G23): the backend must
		// give the name that the policy checked.
		if names.Vault != ref.Vault || canon.Validate() != nil {
			b.log.Warn("the backend returned names that do not fit the reference")
			return b.reject(c, audit.EventError, c.req.References,
				protocol.Errorf(protocol.CodeBackendUnavailable, "the backend returned invalid names"))
		}
		c.canon[ref], c.names[canon] = canon, names
	}
	return nil
}

// backendFailed ends the request after a failed call of the backend.
func (b *Broker) backendFailed(c *call, err error) *protocol.Error {
	code := backend.Code(err)
	if code == protocol.CodeNotFound {
		// D27: an unknown item or field in an allowed vault.
		return b.reject(c, audit.EventDenial, c.req.References,
			protocol.Errorf(protocol.CodeNotFound, "a credential of the request does not exist"))
	}
	b.log.Warn("the backend failed", "code", string(code))
	return b.reject(c, audit.EventError, c.req.References, protocol.Errorf(code, "the backend failed: %s", code))
}

// plan is the result of the step that sorts the references of a request.
type plan struct {
	// tooLarge lists the references that the full bundle rejected, in
	// the sequence of the request.
	tooLarge []protocol.Reference
	// wait is the bundle request to wait for, or nil if a grant covers
	// each other reference.
	wait *pendingRequest
}

// dispatch sorts the references into covered, bundled and rejected, waits
// for the bundle and delivers.
func (b *Broker) dispatch(ctx context.Context, c *call) (*protocol.Credentials, *protocol.Error) {
	p, ok := b.place(ctx, c)
	if !ok {
		return nil, b.requestEnded(c, c.req.References)
	}
	if len(p.tooLarge) > 0 {
		if e := b.record(b.event(c, audit.EventDenial, p.tooLarge,
			audit.Result{Code: protocol.CodeBundleTooLarge})); e != nil {
			return nil, e
		}
	}
	rejected := map[protocol.Reference]protocol.Code{}
	for _, ref := range p.tooLarge {
		rejected[ref] = protocol.CodeBundleTooLarge
	}
	if p.wait != nil {
		select {
		case <-p.wait.done:
		case <-ctx.Done():
			return nil, sessionEndedError()
		}
		if p.wait.result.err != nil {
			return nil, p.wait.result.err
		}
		for ref := range p.wait.result.denied {
			rejected[ref] = protocol.CodeDenied
		}
	}
	return b.deliver(ctx, c, rejected)
}

// place puts each reference that no live grant covers into the open
// bundle of the session, or opens a bundle. A request that a grant covers
// in full opens no window (D29). It returns false if the session ended:
// a request that was in flight at the end of its session never opens a
// bundle and never reaches the human (D33).
func (b *Broker) place(ctx context.Context, c *call) (plan, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.endedLocked(ctx, c.s.Facts.SessionID) {
		return plan{}, false
	}
	return b.placeLocked(c), true
}

// placeLocked is place for a live session. The caller holds mu.
func (b *Broker) placeLocked(c *call) plan {
	now := b.clock.Now()
	var uncovered []protocol.Reference
	seen := map[protocol.Reference]bool{}
	for _, ref := range c.req.References {
		canon := c.canon[ref]
		if seen[canon] {
			continue
		}
		seen[canon] = true
		if _, ok := b.grants.Cover(c.s.Facts.SessionID, canon, now); !ok {
			uncovered = append(uncovered, canon)
		}
	}
	var p plan
	if len(uncovered) == 0 {
		return p
	}
	st := b.stateLocked(c.s)
	pb := st.open
	// A bundle whose window ended but whose runner did not close it yet
	// takes no further request: the request opens a new bundle.
	if pb == nil || !pb.b.Accepts(now) {
		pb = b.openBundleLocked(st, c.s, now)
	}
	accepted := pb.b.Add(uncovered)
	var joined []protocol.Reference
	for i, ref := range uncovered {
		if !accepted[i] {
			p.tooLarge = append(p.tooLarge, ref)
			continue
		}
		joined = append(joined, ref)
		pb.names[ref] = c.names[ref]
	}
	if len(joined) > 0 {
		p.wait = &pendingRequest{claims: c.claims, done: make(chan struct{})}
		pb.requests = append(pb.requests, p.wait)
	}
	return p
}

// openBundleLocked opens a new bundle for the session and starts its
// window. The timer starts before the call returns, so that a test with a
// fake clock can wait for it. The caller holds mu.
func (b *Broker) openBundleLocked(st *sessionState, s session.Session, now time.Time) *pendingBundle {
	pb := &pendingBundle{
		session:   s,
		requestID: newID("approval"),
		b:         bundle.New(now, b.cfg.Bundle.Window, b.cfg.Bundle.MaxEntries),
		names:     map[protocol.Reference]backend.Names{},
	}
	st.open = pb
	timer := b.clock.NewTimer(b.cfg.Bundle.Window)
	go b.runBundle(st, pb, timer)
	return pb
}

// requestEnded ends a request whose session ended while the request was
// in flight. It writes the expiry with session_ended, not a fault of the
// backend.
func (b *Broker) requestEnded(c *call, refs []protocol.Reference) *protocol.Error {
	_ = b.record(b.event(c, audit.EventExpiry, refs, audit.Result{Detail: audit.DetailSessionEnded}))
	return sessionEndedError()
}

// reject writes one event for a request that ends as a whole and returns
// the error for the agent. A failed write returns audit_failed instead.
func (b *Broker) reject(c *call, t audit.EventType, refs []protocol.Reference, e *protocol.Error) *protocol.Error {
	if ae := b.record(b.event(c, t, refs, audit.Result{Code: e.Code, Detail: e.Message})); ae != nil {
		return ae
	}
	return e
}

// event returns an audit event of the session of c.
func (b *Broker) event(c *call, t audit.EventType, refs []protocol.Reference, r audit.Result) audit.Event {
	claims := c.claims
	if claims == (session.Claims{}) {
		claims = c.s.Claims("", "")
	}
	return audit.Event{Type: t, Verified: c.s.Facts, Claims: claims, References: refs, Result: r}
}

// notifyInvalidProject tells the human once per project and daemon run
// that the configuration of the project is invalid (D28). The result of
// the notice changes nothing; an error goes to the log.
func (b *Broker) notifyInvalidProject(project string) {
	p, ok := b.cfg.Project(project)
	if !ok {
		return
	}
	b.mu.Lock()
	first := !b.notified[project]
	b.notified[project] = true
	b.mu.Unlock()
	if !first {
		return
	}
	n := approval.Notice{
		Version:    approval.Version,
		Type:       approval.TypeNotice,
		NoticeID:   newID("notice"),
		Kind:       approval.NoticeProjectConfigInvalid,
		Code:       protocol.CodeConfigInvalid,
		Project:    project,
		Message:    p.Invalid + " Correct the configuration file and restart a38 serve.",
		OccurredAt: b.clock.Now(),
	}
	if err := b.ui.Notify(context.Background(), n); err != nil {
		b.log.Warn("cannot show the notice", "kind", string(n.Kind), "error", err)
	}
}

// asProtocolError returns err as a *protocol.Error with the code
// invalid_request if it has no code.
func asProtocolError(err error) *protocol.Error {
	if e, ok := err.(*protocol.Error); ok {
		return e
	}
	return protocol.Errorf(protocol.CodeInvalidRequest, "the request is not valid")
}
