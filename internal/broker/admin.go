package broker

import (
	"context"
	"time"

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/grant"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// GrantStatus implements daemon.AgentHandler. It shows only the live
// grants of the own session, with the remaining lifetime and without the
// session fields. A grant never holds a value (T25).
func (b *Broker) GrantStatus(_ context.Context, s session.Session, req *protocol.GrantStatus) (*protocol.Grants, *protocol.Error) {
	if req == nil || s.Facts.SessionID == "" {
		return nil, protocol.Errorf(protocol.CodeInvalidRequest, "the request has no session")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.expireLocked()
	now := b.clock.Now()
	resp := &protocol.Grants{ID: req.ID, Grants: []protocol.Grant{}}
	for _, g := range b.grants.Live(s.Facts.SessionID, now) {
		resp.Grants = append(resp.Grants, wireGrant(g, now, false))
	}
	return resp, nil
}

// SessionEnded implements daemon.AgentHandler. It ends the grants of the
// session, each with an audit event, and ends the open bundles and
// approval requests of the session: they create no grant and deliver
// nothing (D33).
func (b *Broker) SessionEnded(s session.Session) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.expireLocked()
	sid := s.Facts.SessionID
	b.ended[sid] = b.clock.Now()
	if st, ok := b.sessions[sid]; ok {
		st.ended = true
		st.cancel()
		delete(b.sessions, sid)
	}
	for _, g := range b.grants.RemoveSession(sid, b.clock.Now()) {
		_ = b.record(audit.Event{
			Type:       audit.EventExpiry,
			Verified:   g.Facts,
			Claims:     g.Claims,
			References: g.Refs,
			Result:     audit.Result{GrantID: g.ID, Detail: audit.DetailSessionEnded},
		})
	}
}

// GrantsList implements daemon.AdminHandler. It shows the live grants of
// all sessions.
func (b *Broker) GrantsList(_ context.Context, req *protocol.GrantsList) (*protocol.Grants, *protocol.Error) {
	if req == nil {
		return nil, protocol.Errorf(protocol.CodeInvalidRequest, "the request is empty")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.expireLocked()
	now := b.clock.Now()
	resp := &protocol.Grants{ID: req.ID, Grants: []protocol.Grant{}}
	for _, g := range b.grants.All(now) {
		resp.Grants = append(resp.Grants, wireGrant(g, now, true))
	}
	return resp, nil
}

// GrantRevoke implements daemon.AdminHandler. It ends one live grant. An
// unknown or ended grant is not_found. The grant ends also if the audit
// event cannot be written; the answer is then audit_failed.
func (b *Broker) GrantRevoke(_ context.Context, req *protocol.GrantRevoke) (*protocol.GrantRevoked, *protocol.Error) {
	if req == nil {
		return nil, protocol.Errorf(protocol.CodeInvalidRequest, "the request is empty")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.expireLocked()
	g, ok := b.grants.Remove(req.GrantID, b.clock.Now())
	if !ok {
		return nil, protocol.Errorf(protocol.CodeNotFound, "the grant does not exist")
	}
	if e := b.record(audit.Event{
		Type:       audit.EventRevocation,
		Verified:   g.Facts,
		Claims:     g.Claims,
		References: g.Refs,
		Result:     audit.Result{GrantID: g.ID, Detail: "the human revoked the grant"},
	}); e != nil {
		return nil, e
	}
	return &protocol.GrantRevoked{ID: req.ID, GrantID: g.ID}, nil
}

// Status implements daemon.AdminHandler. The socket layer sets the number
// of sessions and the version of the daemon.
func (b *Broker) Status(_ context.Context, req *protocol.Status) (*protocol.StatusResult, *protocol.Error) {
	if req == nil {
		return nil, protocol.Errorf(protocol.CodeInvalidRequest, "the request is empty")
	}
	b.mu.Lock()
	b.expireLocked()
	n := len(b.grants.All(b.clock.Now()))
	b.mu.Unlock()
	resp := &protocol.StatusResult{ID: req.ID, Grants: n, Fault: b.faults.Fault()}
	for _, p := range b.cfg.InvalidProjects() {
		resp.InvalidProjects = append(resp.InvalidProjects, protocol.InvalidProject{Project: p.Name, Reason: p.Invalid})
	}
	return resp, nil
}

// FaultClear implements daemon.AdminHandler. It removes the global
// delivery lock if the check of the cause passes (D48).
func (b *Broker) FaultClear(ctx context.Context, req *protocol.FaultClear) (*protocol.FaultClearResult, *protocol.Error) {
	if req == nil {
		return nil, protocol.Errorf(protocol.CodeInvalidRequest, "the request is empty")
	}
	cleared, finding := b.faults.Clear(ctx)
	return &protocol.FaultClearResult{ID: req.ID, Cleared: cleared, Finding: finding}, nil
}

// wireGrant returns the protocol form of g. The session fields are only
// for the admin socket.
func wireGrant(g grant.Grant, now time.Time, admin bool) protocol.Grant {
	w := protocol.Grant{
		GrantID:          g.ID,
		References:       append([]protocol.Reference(nil), g.Refs...),
		GrantedAt:        g.GrantedAt,
		ExpiresAt:        g.ExpiresAt,
		RemainingSeconds: int64(g.ExpiresAt.Sub(now) / time.Second),
	}
	if admin {
		w.SessionID = string(g.Facts.SessionID)
		w.SessionName = g.Claims.SessionName
		w.Project = g.Facts.Project
	}
	return w
}
