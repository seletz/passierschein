package broker

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/bundle"
	"github.com/seletz/passierschein/internal/clock"
	"github.com/seletz/passierschein/internal/grant"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// pendingBundle is one bundle of one session, from the first request to
// the decision.
type pendingBundle struct {
	session   session.Session
	requestID string
	b         *bundle.Bundle
	// names holds the names of the backend for each entry.
	names map[protocol.Reference]backend.Names
	// requests lists the requests that wait for the bundle. Only code
	// that holds the lock of the broker changes it.
	requests []*pendingRequest
	// claims is set when the window closes.
	claims session.Claims
}

// pendingRequest is one request that waits for its bundle.
type pendingRequest struct {
	claims session.Claims
	// done is closed when result is set.
	done   chan struct{}
	result bundleResult
}

// bundleResult is the result of a bundle for each of its requests.
type bundleResult struct {
	// err ends each request of the bundle as a whole: nothing is
	// delivered, also not for a reference that a grant covers.
	err *protocol.Error
	// denied lists the entries that the human did not select. Each other
	// entry is covered by a grant; the delivery checks this again.
	denied map[protocol.Reference]bool
}

// runBundle waits for the end of the window, then decides the bundle and
// gives the result to each waiting request.
func (b *Broker) runBundle(st *sessionState, pb *pendingBundle, timer clock.Timer) {
	select {
	case <-timer.C():
	case <-st.ctx.Done():
		timer.Stop()
	}
	b.mu.Lock()
	refs := pb.b.Close()
	if st.open == pb {
		st.open = nil
	}
	requests := slices.Clone(pb.requests)
	b.mu.Unlock()

	pb.claims = joinClaims(pb.session, requests)
	res := b.decideSafe(st, pb, refs)
	for _, r := range requests {
		r.result = res
		close(r.done)
	}
}

// decideSafe runs decide. A panic in the UI, the audit store or the broker
// is an internal fault of the daemon (G21): the broker raises the fault,
// and each request of the bundle ends with daemon_faulted. Nothing is
// delivered.
func (b *Broker) decideSafe(st *sessionState, pb *pendingBundle, refs []protocol.Reference) (res bundleResult) {
	defer func() {
		if r := recover(); r != nil {
			b.log.Error("internal fault while the broker decided a bundle", "request_id", pb.requestID, "panic", fmt.Sprint(r))
			b.faults.Raise(protocol.CodeDaemonFaulted, "The broker found an internal fault.")
			b.recordSafe(pb.event(audit.EventError, refs, audit.Result{
				Code: protocol.CodeDaemonFaulted, RequestID: pb.requestID, Detail: "internal fault of the broker",
			}))
			res = bundleResult{err: protocol.Errorf(protocol.CodeDaemonFaulted, "the daemon found an internal fault; nothing is delivered")}
		}
	}()
	return b.decide(st, pb, refs)
}

// recordSafe writes an event after a panic. A second panic is dropped.
func (b *Broker) recordSafe(ev audit.Event) {
	defer func() { _ = recover() }()
	_ = b.record(ev)
}

// decide writes the event "bundle", asks the human for the entries that no
// grant covers, and applies the decision.
func (b *Broker) decide(st *sessionState, pb *pendingBundle, refs []protocol.Reference) bundleResult {
	if st.ctx.Err() != nil {
		return b.sessionEnded(pb, refs)
	}
	if e := b.record(pb.event(audit.EventBundle, refs, audit.Result{RequestID: pb.requestID})); e != nil {
		return bundleResult{err: e}
	}
	// Wait for the turn at the UI first. Only then build the request, so
	// that the entries, the deviation and the existing grant show the
	// grants at the time of the display (D8).
	release, err := b.acquireSlot(st.ctx)
	if err != nil {
		return b.sessionEnded(pb, refs)
	}
	defer release()
	open, existing := b.uncovered(pb, refs)
	if len(open) == 0 {
		// A grant of an earlier bundle covers each entry now.
		return bundleResult{}
	}
	req := b.approvalRequest(pb, open, existing)
	d, err := b.ui.Approve(st.ctx, req)
	if st.ctx.Err() != nil {
		// The session ended while the approval request was open: no
		// grant and no delivery, whatever the UI answered.
		return b.sessionEnded(pb, open)
	}
	if err != nil {
		b.log.Warn("the approval UI failed", "request_id", req.RequestID, "error", err)
		return b.fail(pb, open, audit.EventError, protocol.CodeUIFailed, "the approval UI failed")
	}
	return b.apply(st, pb, req, open, d)
}

// uncovered returns the entries that no live grant of the session covers,
// and the live grants of the session.
func (b *Broker) uncovered(pb *pendingBundle, refs []protocol.Reference) ([]protocol.Reference, []grant.Grant) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.clock.Now()
	sid := pb.session.Facts.SessionID
	var open []protocol.Reference
	for _, ref := range refs {
		if _, ok := b.grants.Cover(sid, ref, now); !ok {
			open = append(open, ref)
		}
	}
	return open, b.grants.Live(sid, now)
}

// acquireSlot waits for the turn at the UI: only one approval request is
// open at a time (D30). The caller calls release when the UI answered.
func (b *Broker) acquireSlot(ctx context.Context) (release func(), err error) {
	select {
	case b.slot <- struct{}{}:
		return func() { <-b.slot }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// apply validates the decision and acts on it.
func (b *Broker) apply(st *sessionState, pb *pendingBundle, req approval.Request, open []protocol.Reference, d approval.Decision) bundleResult {
	v := checkDecision(req, d, b.authRequired)
	switch v.outcome {
	case approval.OutcomeDenied:
		return b.fail(pb, open, audit.EventDenial, protocol.CodeDenied, "the human denied the request")
	case approval.OutcomeTimeout:
		return b.fail(pb, open, audit.EventExpiry, protocol.CodeTimeout, "the approval request expired without a decision")
	case approval.OutcomeApproved:
	default:
		b.log.Warn("the approval UI gave an invalid decision", "request_id", req.RequestID, "finding", v.finding)
		return b.fail(pb, open, audit.EventError, protocol.CodeUIFailed, "the approval UI gave an invalid decision: "+v.finding)
	}
	byID := make(map[string]protocol.Reference, len(open))
	for i, c := range req.Credentials {
		byID[c.ID] = open[i]
	}
	var approved, denied []protocol.Reference
	for _, c := range req.Credentials {
		if slices.Contains(v.ids, c.ID) {
			approved = append(approved, byID[c.ID])
		} else {
			denied = append(denied, byID[c.ID])
		}
	}
	ttl := time.Duration(v.ttlSeconds) * time.Second
	if e := b.record(pb.event(audit.EventDecision, approved, audit.Result{
		RequestID: pb.requestID, LifetimeSeconds: v.ttlSeconds, Detail: "approved",
	})); e != nil {
		return bundleResult{err: e}
	}
	if e := b.createGrant(st, pb, approved, ttl); e != nil {
		return bundleResult{err: e}
	}
	res := bundleResult{denied: map[protocol.Reference]bool{}}
	for _, ref := range denied {
		res.denied[ref] = true
	}
	if len(denied) > 0 {
		if e := b.record(pb.event(audit.EventDenial, denied, audit.Result{
			Code: protocol.CodeDenied, RequestID: pb.requestID, Detail: "the human did not select the entries",
		})); e != nil {
			return bundleResult{err: e}
		}
	}
	return res
}

// createGrant creates one grant for the approved entries. The grant exists
// only after its audit event is written. No grant is created for an ended
// session or while a fault locks delivery.
func (b *Broker) createGrant(st *sessionState, pb *pendingBundle, refs []protocol.Reference, ttl time.Duration) *protocol.Error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if st.ended {
		_ = b.record(pb.event(audit.EventExpiry, refs, audit.Result{RequestID: pb.requestID, Detail: audit.DetailSessionEnded}))
		return sessionEndedError()
	}
	if e := b.faults.Check(); e != nil {
		return e
	}
	now := b.clock.Now()
	g := grant.Grant{
		ID:        newID("grant"),
		Facts:     pb.session.Facts,
		Claims:    pb.session.Claims("", ""),
		Refs:      refs,
		GrantedAt: now,
		ExpiresAt: now.Add(ttl),
	}
	if e := b.record(pb.event(audit.EventGrant, refs, audit.Result{
		RequestID: pb.requestID, GrantID: g.ID, LifetimeSeconds: int64(ttl / time.Second),
	})); e != nil {
		return e
	}
	b.grants.Add(g)
	return nil
}

// fail ends the bundle with an event and a code for each of its requests.
func (b *Broker) fail(pb *pendingBundle, refs []protocol.Reference, t audit.EventType, code protocol.Code, msg string) bundleResult {
	if e := b.record(pb.event(t, refs, audit.Result{Code: code, RequestID: pb.requestID, Detail: msg})); e != nil {
		return bundleResult{err: e}
	}
	return bundleResult{err: protocol.Errorf(code, "%s", msg)}
}

// sessionEnded ends the bundle of a session that ended.
func (b *Broker) sessionEnded(pb *pendingBundle, refs []protocol.Reference) bundleResult {
	_ = b.record(pb.event(audit.EventExpiry, refs, audit.Result{RequestID: pb.requestID, Detail: audit.DetailSessionEnded}))
	return bundleResult{err: sessionEndedError()}
}

// approvalRequest builds the approval request for the entries. Verified
// facts and claims stay apart (D4): the session name and the agent name
// are claims only. The entries show the names of the backend (D27) and
// never a value (D34).
func (b *Broker) approvalRequest(pb *pendingBundle, open []protocol.Reference, existing []grant.Grant) approval.Request {
	req := approval.Request{
		Version:   approval.Version,
		Type:      approval.TypeRequest,
		RequestID: pb.requestID,
		Verified:  pb.session.Facts,
		Claims:    pb.claims,
		Grant: approval.GrantProposal{
			TTLSeconds: int64(b.cfg.Grant.Lifetime / time.Second),
		},
	}
	var vaults []string
	for i, ref := range open {
		n := pb.names[ref]
		v, _ := b.cfg.Vault(ref.Vault)
		req.Credentials = append(req.Credentials, approval.Credential{
			ID:          "c" + strconv.Itoa(i+1),
			Vault:       n.Vault,
			Title:       n.Item,
			Field:       n.Field,
			Environment: v.Environment,
			Deviation:   deviation(ref, existing),
		})
		vaults = appendUnique(vaults, n.Vault)
	}
	req.Grant.Scope = approval.GrantScope{Project: pb.session.Facts.Project, Vaults: vaults}
	if len(existing) > 0 {
		// The newest grant marks the request as a deviation (D8, UC3).
		g := existing[len(existing)-1]
		req.ExistingGrant = &approval.ExistingGrant{
			GrantID:   g.ID,
			ExpiresAt: g.ExpiresAt,
			Scope:     approval.GrantScope{Project: g.Facts.Project, Vaults: vaultsOf(g.Refs)},
			ItemCount: len(g.Refs),
		}
	}
	return req
}

// deviation says how ref goes beyond the live grants of the session. It
// is empty if the session has no live grant.
func deviation(ref protocol.Reference, existing []grant.Grant) approval.Deviation {
	if len(existing) == 0 {
		return ""
	}
	for _, g := range existing {
		if slices.Contains(vaultsOf(g.Refs), ref.Vault) {
			return approval.DeviationNewItem
		}
	}
	return approval.DeviationNewVault
}

// event returns an audit event of the bundle.
func (pb *pendingBundle) event(t audit.EventType, refs []protocol.Reference, r audit.Result) audit.Event {
	return audit.Event{Type: t, Verified: pb.session.Facts, Claims: pb.claims, References: refs, Result: r}
}

// joinClaims returns the claims of the bundle: the session name and the
// agent name of the session, and each different task and reason of the
// requests, one per line, in the sequence of arrival. The human sees each
// claim of the bundle.
func joinClaims(s session.Session, requests []*pendingRequest) session.Claims {
	var tasks, reasons []string
	for _, r := range requests {
		if r.claims.Task != "" {
			tasks = appendUnique(tasks, r.claims.Task)
		}
		if r.claims.Reason != "" {
			reasons = appendUnique(reasons, r.claims.Reason)
		}
	}
	return s.Claims(strings.Join(tasks, "\n"), strings.Join(reasons, "\n"))
}

// vaultsOf returns the different vaults of refs.
func vaultsOf(refs []protocol.Reference) []string {
	var out []string
	for _, r := range refs {
		out = appendUnique(out, r.Vault)
	}
	return out
}

// appendUnique appends s if list does not contain it.
func appendUnique(list []string, s string) []string {
	if slices.Contains(list, s) {
		return list
	}
	return append(list, s)
}
