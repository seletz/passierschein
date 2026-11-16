package broker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/grant"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// GrantStatus implements daemon.AgentHandler. It shows only the live
// grants of the own session, with the remaining lifetime and without the
// session fields. A grant never holds a value (T25).
func (b *Broker) GrantStatus(_ context.Context, s session.Session, req *protocol.GrantStatus) (res *protocol.Grants, e *protocol.Error) {
	defer b.guard("grant_status", func() { res, e = nil, faultedError() })
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
	defer b.guard("session_ended", nil)
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

// Shutdown ends each live grant with an expiry event and the reason
// daemon_stopped. The daemon calls it on a graceful stop (SIGINT, SIGTERM)
// before the audit log closes. Grants live only in memory, so the end of
// the daemon ends them all. Limit: an ungraceful end (SIGKILL, crash)
// writes nothing; the audit log then has no end event for the grants
// that were live. Shutdown is safe to call more than once.
func (b *Broker) Shutdown() {
	defer b.guard("shutdown", nil)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.expireLocked()
	now := b.clock.Now()
	for _, g := range b.grants.All(now) {
		if _, ok := b.grants.Remove(g.ID, now); !ok {
			continue
		}
		_ = b.record(audit.Event{
			Type:       audit.EventExpiry,
			Verified:   g.Facts,
			Claims:     g.Claims,
			References: g.Refs,
			Result:     audit.Result{GrantID: g.ID, Detail: audit.DetailDaemonStopped},
		})
	}
}

// GrantsList implements daemon.AdminHandler. It shows the live grants of
// all sessions.
func (b *Broker) GrantsList(_ context.Context, req *protocol.GrantsList) (res *protocol.Grants, e *protocol.Error) {
	defer b.guard("grants_list", func() { res, e = nil, faultedError() })
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
func (b *Broker) GrantRevoke(_ context.Context, req *protocol.GrantRevoke) (res *protocol.GrantRevoked, e *protocol.Error) {
	defer b.guard("grant_revoke", func() { res, e = nil, faultedError() })
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
func (b *Broker) Status(ctx context.Context, req *protocol.Status) (res *protocol.StatusResult, e *protocol.Error) {
	defer b.guard("status", func() { res, e = nil, faultedError() })
	if req == nil {
		return nil, protocol.Errorf(protocol.CodeInvalidRequest, "the request is empty")
	}
	// Inspect checks the cause of a fault without a change: the status
	// shows the lock, its cause, its start and whether the check of the
	// cause passes now (D48, T27).
	resp := &protocol.StatusResult{ID: req.ID, Grants: b.liveGrants(), Fault: b.faults.Inspect(ctx)}
	for _, p := range b.cfg.InvalidProjects() {
		resp.InvalidProjects = append(resp.InvalidProjects, protocol.InvalidProject{Project: p.Name, Reason: p.Invalid})
	}
	resp.Backend = b.backendStatus(ctx)
	if b.uiMode != "" {
		// The UI has no health check. The daemon starts only with a UI
		// that it could build; a failure shows at the next approval
		// request as ui_failed.
		resp.UI = &protocol.UIStatus{Mode: string(b.uiMode), Usable: b.ui != nil, AppPath: b.uiApp}
		if b.ui == nil {
			resp.UI.Reason = "The daemon has no approval UI."
		}
	}
	return resp, nil
}

// accountStatusLimit is the time limit of each account for the status
// (D78). It is shorter than the time limit of a38 doctor for the whole
// status (5 s), so that a hanging account does not hide the others.
const accountStatusLimit = 3 * time.Second

// backendStatus asks the backend for its vaults. It reads no value. The
// state is ok, locked or unreachable. A backend with more than one
// account gives the state of each account (D78).
func (b *Broker) backendStatus(ctx context.Context) *protocol.BackendStatus {
	kind := string(b.backendKind)
	if c, ok := b.be.(backend.AccountChecker); ok {
		return accountsStatus(kind, c.Check(ctx, accountStatusLimit))
	}
	_, err := b.be.Vaults(ctx)
	switch {
	case err == nil:
		return &protocol.BackendStatus{Kind: kind, State: protocol.BackendOK}
	case backend.Code(err) == protocol.CodeBackendLocked:
		return &protocol.BackendStatus{Kind: kind, State: protocol.BackendLocked, Reason: "The backend is locked. Unlock it."}
	default:
		return &protocol.BackendStatus{Kind: kind, State: protocol.BackendUnreachable, Reason: "The backend does not answer."}
	}
}

// accountsStatus returns the status of a backend with more than one
// account (D78). The backend is ok while one account answers. Else it is
// locked if one account is locked, because only the human can help, and
// unreachable if no account answers.
func accountsStatus(kind string, states []backend.AccountState) *protocol.BackendStatus {
	out := &protocol.BackendStatus{Kind: kind, State: protocol.BackendUnreachable, Accounts: []protocol.AccountStatus{}}
	anyOK, anyLocked := false, false
	for _, st := range states {
		a := protocol.AccountStatus{Name: st.Name, State: protocol.BackendOK, VaultsConfigured: st.Configured, VaultsVisible: st.Visible}
		switch {
		case st.Err == nil:
			anyOK = true
		case backend.Code(st.Err) == protocol.CodeBackendLocked:
			anyLocked = true
			a.State, a.VaultsVisible, a.Reason = protocol.BackendLocked, 0, st.Err.Error()
		case errors.Is(st.Err, context.DeadlineExceeded) || errors.Is(st.Err, context.Canceled):
			a.State, a.VaultsVisible = protocol.BackendUnreachable, 0
			a.Reason = fmt.Sprintf("the account gave no answer within the time of the status (%s)", accountStatusLimit)
		default:
			a.State, a.VaultsVisible, a.Reason = protocol.BackendUnreachable, 0, st.Err.Error()
		}
		out.Accounts = append(out.Accounts, a)
	}
	switch {
	case anyOK:
		out.State = protocol.BackendOK
	case anyLocked:
		out.State, out.Reason = protocol.BackendLocked, "No account of the backend answers, and at least one account is locked. The accounts name the cause."
	default:
		out.Reason = "No account of the backend answers. The accounts name the cause."
	}
	return out
}

// liveGrants returns the number of live grants of all sessions.
func (b *Broker) liveGrants() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.expireLocked()
	return len(b.grants.All(b.clock.Now()))
}

// FaultClear implements daemon.AdminHandler. It removes the global
// delivery lock if the check of the cause passes (D48). If the check
// fails, the lock stays and the answer names the finding. The command is
// not an error: the answer says whether the lock is lifted.
func (b *Broker) FaultClear(ctx context.Context, req *protocol.FaultClear) (res *protocol.FaultClearResult, e *protocol.Error) {
	defer b.guard("fault_clear", func() { res, e = nil, faultedError() })
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
