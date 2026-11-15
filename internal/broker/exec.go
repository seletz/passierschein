package broker

import (
	"context"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/grant"
	"github.com/seletz/passierschein/internal/policy"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// Flow of a request of the exec way (request_exec, D61 to D66):
//
//  1. The global delivery lock (D48) is open.
//  2. The request is valid. The broker writes the audit event "request"
//     with the command.
//  3. The number of variables is not more than bundle.max_entries
//     (bundle_too_large), and the time limit is not more than
//     exec.max_timeout (invalid_request).
//  4. The policy allows each reference for the exec way (D62). The
//     working directory is in the project of the session, after the
//     symbolic links are resolved (D64); else not_allowed.
//  5. The backend resolves the names. It reads no value.
//  6. A live grant whose binding allows the command covers each
//     reference (D63), or the human approves the command. The request is
//     a bundle of its own and does not wait for a window (D61).
//  7. The broker reads the values. If an argument or the working directory
//     contains a value, it delivers nothing (invalid_request, D65). Else
//     it writes the audit event "delivery" and answers. A grant with the
//     lifetime 0 ends with this delivery (D63).
//  8. "a38 mcp" reports the result with exec_result. The broker writes the
//     audit event "execution" (D65, G39).
//
// The order of step 7 follows D65: the check of the command needs the
// values. The broker sends a value only after the event "delivery" is
// written (D36); a failed write drops the values.

// programsWithoutStatement are the programs whose path says nothing about
// the command: shells, interpreters and starters (D63). For them the
// binding program is the binding command. The comparison uses the last
// part of the path.
var programsWithoutStatement = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "dash": true, "fish": true,
	"env": true, "sudo": true, "xargs": true, "nohup": true, "time": true,
	"timeout": true, "nice": true, "python": true, "python3": true,
	"node": true, "deno": true, "bun": true, "ruby": true, "perl": true,
	"osascript": true,
}

// maxPendingExecs is the maximum number of deliveries of one session that
// wait for their exec_result. A further delivery drops the oldest.
const maxPendingExecs = 64

// valueInCommandMessage is the answer to a command that contains a value.
// It never names the value.
const valueInCommandMessage = "an argument or the working directory contains a value of a credential; nothing is delivered"

// execCall holds the terms of one request of the exec way.
type execCall struct {
	req *protocol.RequestExec
	// cmd is the part of the command that a binding compares.
	cmd grant.Command
	// env is the strictest environment of the references (D62).
	env config.Environment
	// binding is the effective binding of a new grant (D63).
	binding config.Binding
	// ttl is the lifetime of a new grant: the shortest lifetime of the
	// environments. Zero means a grant for one delivery.
	ttl time.Duration
	// timeout is the time limit of the command in seconds.
	timeout int
}

// execRecord is a delivery of the exec way that waits for its
// exec_result.
type execRecord struct {
	requestID string
	claims    session.Claims
	refs      []protocol.Reference
	command   *audit.Command
	grantID   string
}

// RequestExec implements daemon.AgentHandler. See the flow above.
func (b *Broker) RequestExec(ctx context.Context, s session.Session, req *protocol.RequestExec) (out *protocol.ExecCredentials, e *protocol.Error) {
	defer b.guard("request_exec", func() { out, e = nil, faultedError() })
	b.sweep()
	c := &call{s: s}
	if e := b.faults.Check(); e != nil {
		b.recordExecLockRejection(c, req, e)
		return nil, e
	}
	x, e := b.admitExec(c, req)
	if e != nil {
		return nil, e
	}
	if e := b.resolve(ctx, c); e != nil {
		return nil, e
	}
	return b.runExec(ctx, c, x)
}

// recordExecLockRejection writes the event "error" for a request_exec that
// the global lock rejected. It is best effort, like
// recordLockRejection.
func (b *Broker) recordExecLockRejection(c *call, req *protocol.RequestExec, e *protocol.Error) {
	if c.s.Facts.SessionID == "" {
		return
	}
	var refs []protocol.Reference
	if req != nil && req.Validate() == nil {
		c.claims = c.s.Claims(req.Task, req.Reason)
		c.command = commandOf(req)
		refs = execRefs(req)
	}
	_, _ = b.audit.Append(context.Background(), b.event(c, audit.EventError, refs, audit.Result{Code: e.Code, Detail: "delivery is locked"}))
}

// admitExec checks the request, the limits, the policy and the working
// directory. It writes the event "request" and, for a rejection, the event
// "denial".
func (b *Broker) admitExec(c *call, req *protocol.RequestExec) (*execCall, *protocol.Error) {
	if req == nil || c.s.Facts.SessionID == "" {
		return nil, b.reject(c, audit.EventDenial, nil, protocol.Errorf(protocol.CodeInvalidRequest, "the request has no session"))
	}
	if err := req.Validate(); err != nil {
		// The references can be invalid; the event does not name them.
		return nil, b.reject(c, audit.EventDenial, nil, asProtocolError(err))
	}
	c.claims = c.s.Claims(req.Task, req.Reason)
	c.command = commandOf(req)
	c.refs = execRefs(req)
	if e := b.record(b.event(c, audit.EventRequest, c.refs, audit.Result{})); e != nil {
		return nil, e
	}
	if len(req.Env) > b.cfg.Bundle.MaxEntries {
		return nil, b.reject(c, audit.EventDenial, c.refs, protocol.Errorf(protocol.CodeBundleTooLarge,
			"the command names more variables than bundle.max_entries (%d)", b.cfg.Bundle.MaxEntries))
	}
	maxTimeout := int(b.cfg.Exec.MaxTimeout / time.Second)
	if req.TimeoutSeconds > maxTimeout {
		return nil, b.reject(c, audit.EventDenial, c.refs, protocol.Errorf(protocol.CodeInvalidRequest,
			"timeout_seconds is more than exec.max_timeout (%d seconds)", maxTimeout))
	}
	if e := policy.CheckWay(b.cfg, c.s.Facts.Project, c.refs, config.DeliveryExec); e != nil {
		if e.Code == protocol.CodeConfigInvalid {
			b.notifyInvalidProject(c.s.Facts.Project)
		}
		return nil, b.reject(c, audit.EventDenial, c.refs, e)
	}
	if !b.inProject(c.s.Facts.Project, req.Cwd) {
		return nil, b.reject(c, audit.EventDenial, c.refs, protocol.Errorf(protocol.CodeNotAllowed,
			"the working directory is not in the project of the session"))
	}
	x := &execCall{
		req: req,
		cmd: grant.Command{Program: req.Program, Args: req.Args, Cwd: req.Cwd},
	}
	for _, v := range req.Env {
		x.cmd.EnvNames = append(x.cmd.EnvNames, v.Name)
	}
	x.timeout = req.TimeoutSeconds
	if x.timeout == 0 {
		x.timeout = int(b.cfg.Exec.DefaultTimeout / time.Second)
	}
	x.env, x.binding, x.ttl = b.execTerms(c.refs, req.Program)
	// The events from here on name the time limit and the binding.
	cmd := *c.command
	cmd.TimeoutSeconds, cmd.Binding = x.timeout, string(x.binding)
	c.command = &cmd
	return x, nil
}

// inProject reports whether the working directory belongs to the project
// after the symbolic links are resolved (D64). A path that cannot be
// resolved does not belong to a project.
func (b *Broker) inProject(project, cwd string) bool {
	if project == "" {
		return false
	}
	real, err := b.realPath(cwd)
	if err != nil {
		return false
	}
	p, ok := b.cfg.ProjectFor(real)
	return ok && p.Name == project
}

// execTerms returns the strictest environment of the references, the
// strictest binding and the shortest lifetime of their environments (D62,
// D63). prod has the lifetime 0 and the binding command. For a program
// without a statement the binding program becomes command.
func (b *Broker) execTerms(refs []protocol.Reference, program string) (config.Environment, config.Binding, time.Duration) {
	env := policy.Strictest(b.cfg, refs)
	binding := config.BindingNone
	ttl := time.Duration(-1)
	for _, ref := range refs {
		v, _ := b.cfg.Vault(ref.Vault)
		if r := b.cfg.Binding(v.Environment); bindingRank(r) > bindingRank(binding) {
			binding = r
		}
		if l := b.cfg.GrantLifetime(v.Environment); ttl < 0 || l < ttl {
			ttl = l
		}
	}
	if env == config.EnvProd || ttl < 0 {
		ttl, binding = 0, config.BindingCommand
	}
	if binding == config.BindingProgram && programsWithoutStatement[path.Base(program)] {
		binding = config.BindingCommand
	}
	return env, binding, ttl
}

// bindingRank orders the bindings by strictness. An unknown binding is
// the strictest.
func bindingRank(b config.Binding) int {
	switch b {
	case config.BindingNone:
		return 0
	case config.BindingProgram:
		return 1
	}
	return 2
}

// runExec finds the grants or asks the human, and delivers.
func (b *Broker) runExec(ctx context.Context, c *call, x *execCall) (*protocol.ExecCredentials, *protocol.Error) {
	canon := c.canonRefs()
	ctx, cancel, ok := b.execContext(ctx, c.s)
	if !ok {
		return nil, b.requestEnded(c, canon)
	}
	defer cancel()
	var once *grant.Grant
	if !b.coveredExec(c, x, canon) {
		g, e := b.approveExec(ctx, c, x, canon)
		if e != nil {
			return nil, e
		}
		once = g
	}
	return b.deliverExec(ctx, c, x, canon, once)
}

// canonRefs returns the different references with the names of the
// backend, in the sequence of the request.
func (c *call) canonRefs() []protocol.Reference {
	var out []protocol.Reference
	seen := map[protocol.Reference]bool{}
	for _, ref := range c.refs {
		canon := c.canon[ref]
		if !seen[canon] {
			seen[canon] = true
			out = append(out, canon)
		}
	}
	return out
}

// execContext returns a context that ends with the request and with the
// session. It returns false if the session ended.
func (b *Broker) execContext(ctx context.Context, s session.Session) (context.Context, context.CancelFunc, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.endedLocked(ctx, s.Facts.SessionID) {
		return nil, nil, false
	}
	st := b.stateLocked(s)
	ctx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(st.ctx, cancel)
	return ctx, func() { stop(); cancel() }, true
}

// coveredExec reports whether live grants of the session cover each
// reference for the command.
func (b *Broker) coveredExec(c *call, x *execCall, canon []protocol.Reference) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.clock.Now()
	for _, ref := range canon {
		if _, ok := b.grants.CoverCommand(c.s.Facts.SessionID, ref, x.cmd, now); !ok {
			return false
		}
	}
	return true
}

// approveExec asks the human for the command and creates the grant. It
// returns the grant if its lifetime is 0: such a grant is not in the store
// and covers exactly the next delivery of this request. It returns nil if
// the grant is in the store or if a grant covers the command now.
func (b *Broker) approveExec(ctx context.Context, c *call, x *execCall, canon []protocol.Reference) (*grant.Grant, *protocol.Error) {
	// One approval request at a time (D30).
	release, err := b.acquireSlot(ctx)
	if err != nil {
		return nil, b.requestEnded(c, canon)
	}
	defer release()
	if b.coveredExec(c, x, canon) {
		// A grant of an earlier approval covers the command now.
		return nil, nil
	}
	req := b.execApprovalRequest(c, x, canon)
	d, err := b.ui.Approve(ctx, req)
	if ctx.Err() != nil {
		// The session or the connection ended while the approval request
		// was open: no grant and no delivery.
		return nil, b.requestEnded(c, canon)
	}
	if err != nil {
		b.log.Warn("the approval UI failed", "request_id", req.RequestID, "error", err)
		return nil, b.reject(c, audit.EventError, canon, protocol.Errorf(protocol.CodeUIFailed, "the approval UI failed"))
	}
	result := func(t audit.EventType, code protocol.Code, msg string) *protocol.Error {
		if e := b.record(b.event(c, t, canon, audit.Result{Code: code, RequestID: req.RequestID, Detail: msg})); e != nil {
			return e
		}
		return protocol.Errorf(code, "%s", msg)
	}
	v := checkDecision(req, d, b.authRequired)
	switch v.outcome {
	case approval.OutcomeDenied:
		return nil, result(audit.EventDenial, protocol.CodeDenied, "the human denied the request")
	case approval.OutcomeTimeout:
		return nil, result(audit.EventExpiry, protocol.CodeTimeout, "the approval request expired without a decision")
	case approval.OutcomeApproved:
	default:
		b.log.Warn("the approval UI gave an invalid decision", "request_id", req.RequestID, "finding", v.finding)
		return nil, result(audit.EventError, protocol.CodeUIFailed, "the approval UI gave an invalid decision: "+v.finding)
	}
	if e := b.record(b.event(c, audit.EventDecision, canon, audit.Result{
		RequestID: req.RequestID, LifetimeSeconds: v.ttlSeconds, Detail: "approved",
	})); e != nil {
		return nil, e
	}
	return b.createExecGrant(ctx, c, x, canon, req.RequestID, time.Duration(v.ttlSeconds)*time.Second)
}

// createExecGrant creates the grant for the approved command. The grant
// exists only after its audit event is written. A grant with the lifetime
// 0 does not go into the store: no other request and no grant_status sees
// it (D63).
func (b *Broker) createExecGrant(ctx context.Context, c *call, x *execCall, canon []protocol.Reference, requestID string, ttl time.Duration) (*grant.Grant, *protocol.Error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.endedLocked(ctx, c.s.Facts.SessionID) {
		_ = b.record(b.event(c, audit.EventExpiry, canon, audit.Result{RequestID: requestID, Detail: audit.DetailSessionEnded}))
		return nil, sessionEndedError()
	}
	if e := b.faults.Check(); e != nil {
		return nil, e
	}
	now := b.clock.Now()
	g := grant.Grant{
		ID:        newID("grant"),
		Facts:     c.s.Facts,
		Claims:    c.s.Claims("", ""),
		Refs:      canon,
		GrantedAt: now,
		ExpiresAt: now.Add(ttl),
		Binding:   x.binding,
		Command:   x.cmd,
	}
	if e := b.record(b.event(c, audit.EventGrant, canon, audit.Result{
		RequestID: requestID, GrantID: g.ID, LifetimeSeconds: int64(ttl / time.Second),
	})); e != nil {
		return nil, e
	}
	if ttl > 0 {
		b.grants.Add(g)
		return nil, nil
	}
	return &g, nil
}

// execApprovalRequest builds the approval request for the command (D65).
// The entries show the names of the backend (D27) and never a value (D34).
func (b *Broker) execApprovalRequest(c *call, x *execCall, canon []protocol.Reference) approval.Request {
	req := approval.Request{
		Version:   approval.Version,
		Type:      approval.TypeRequest,
		RequestID: newID("approval"),
		Verified:  c.s.Facts,
		Claims:    c.claims,
		Grant:     approval.GrantProposal{TTLSeconds: int64(x.ttl / time.Second)},
	}
	ids := map[protocol.Reference]string{}
	var vaults []string
	for i, ref := range canon {
		n := c.names[ref]
		v, _ := b.cfg.Vault(ref.Vault)
		id := "c" + strconv.Itoa(i+1)
		ids[ref] = id
		req.Credentials = append(req.Credentials, approval.Credential{
			ID: id, Vault: n.Vault, Title: n.Item, Field: n.Field, Environment: v.Environment,
		})
		vaults = appendUnique(vaults, n.Vault)
	}
	req.Grant.Scope = approval.GrantScope{Project: c.s.Facts.Project, Vaults: vaults}
	cmd := &approval.ExecCommand{
		Environment: x.env,
		Program:     x.req.Program,
		Args:        append([]string(nil), x.req.Args...),
		Cwd:         x.req.Cwd,
	}
	for _, v := range x.req.Env {
		cmd.Env = append(cmd.Env, approval.ExecVariable{Name: v.Name, CredentialID: ids[c.canon[v.Reference]]})
	}
	if x.env == config.EnvProd {
		cmd.Approval = approval.ExecApproval{Kind: approval.ExecApprovalOnce, Binding: approval.BindingCommand}
	} else {
		cmd.Approval = approval.ExecApproval{
			Kind:                   approval.ExecApprovalGrant,
			Binding:                approval.Binding(x.binding),
			LifetimeChoicesSeconds: approval.ExecLifetimeChoices(req.Grant.TTLSeconds),
		}
	}
	req.Exec = cmd
	return req
}

// deliverExec reads the values, checks the command and delivers. once is
// the grant for this delivery only, or nil: then live grants of the store
// must cover the command.
func (b *Broker) deliverExec(ctx context.Context, c *call, x *execCall, canon []protocol.Reference, once *grant.Grant) (*protocol.ExecCredentials, *protocol.Error) {
	if _, e := b.authorizeExec(ctx, c, x, canon, once, false); e != nil {
		b.endOnce(c, canon, once, "the grant of one delivery ended without a delivery")
		return nil, e
	}
	values, e := b.read(ctx, c, canon)
	if e != nil {
		b.endOnce(c, canon, once, "the grant of one delivery ended without a delivery")
		return nil, e
	}
	if commandContainsValue(x.req, values) {
		// RI14: the event names the reason, never the value. The events
		// after the check do not repeat the command: it contains a value.
		clear(values)
		bare := *c
		bare.command = nil
		ae := b.record(b.event(&bare, audit.EventDenial, canon, audit.Result{
			Code: protocol.CodeInvalidRequest, Detail: audit.DetailValueInCommand,
		}))
		b.endOnce(&bare, canon, once, "the grant of one delivery ended without a delivery")
		if ae != nil {
			return nil, ae
		}
		return nil, protocol.Errorf(protocol.CodeInvalidRequest, "%s", valueInCommandMessage)
	}
	grantID, e := b.authorizeExec(ctx, c, x, canon, once, true)
	if e != nil {
		clear(values)
		return nil, e
	}
	b.keepExec(c, x, canon, grantID)
	out := &protocol.ExecCredentials{
		ID:             x.req.ID,
		TimeoutSeconds: x.timeout,
		OutputLimit:    b.cfg.Exec.OutputLimit,
	}
	for _, v := range x.req.Env {
		out.Values = append(out.Values, protocol.ExecValue{Name: v.Name, Value: values[c.canon[v.Reference]]})
	}
	return out, nil
}

// authorizeExec checks under the lock of the broker that the session is
// live, delivery is not locked and a grant covers each reference for the
// command. With write set it also writes the event "delivery" for each
// grant and the expiry of a grant for one delivery, in the same step. It
// returns the ID of the first grant.
func (b *Broker) authorizeExec(ctx context.Context, c *call, x *execCall, canon []protocol.Reference, once *grant.Grant, write bool) (string, *protocol.Error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.expireLocked()
	if b.endedLocked(ctx, c.s.Facts.SessionID) {
		return "", b.requestEnded(c, canon)
	}
	if e := b.faults.Check(); e != nil {
		return "", e
	}
	var order []string
	groups := map[string][]protocol.Reference{}
	if once != nil {
		order, groups[once.ID] = []string{once.ID}, canon
	} else {
		now := b.clock.Now()
		for _, ref := range canon {
			g, ok := b.grants.CoverCommand(c.s.Facts.SessionID, ref, x.cmd, now)
			if !ok {
				// A grant ended or was revoked after the check: no value.
				return "", b.reject(c, audit.EventDenial, canon, protocol.Errorf(protocol.CodeDenied,
					"no live grant covers the credentials of the command"))
			}
			if _, known := groups[g.ID]; !known {
				order = append(order, g.ID)
			}
			groups[g.ID] = append(groups[g.ID], ref)
		}
	}
	if !write {
		return order[0], nil
	}
	for _, id := range order {
		if e := b.record(b.event(c, audit.EventDelivery, groups[id], audit.Result{GrantID: id})); e != nil {
			return "", e
		}
	}
	if once != nil {
		if e := b.record(b.event(c, audit.EventExpiry, canon, audit.Result{GrantID: once.ID, Detail: audit.DetailSingleDelivery})); e != nil {
			return "", e
		}
	}
	return order[0], nil
}

// endOnce writes the end of a grant for one delivery that did not deliver.
// It does nothing for nil.
func (b *Broker) endOnce(c *call, canon []protocol.Reference, once *grant.Grant, detail string) {
	if once == nil {
		return
	}
	_ = b.record(b.event(c, audit.EventExpiry, canon, audit.Result{GrantID: once.ID, Detail: detail}))
}

// commandContainsValue reports whether an argument or the working
// directory contains one of the values (D65). An empty value is never
// found.
func commandContainsValue(req *protocol.RequestExec, values map[protocol.Reference]protocol.Secret) bool {
	for _, v := range values {
		s := v.Reveal()
		if s == "" {
			continue
		}
		if strings.Contains(req.Cwd, s) {
			return true
		}
		for _, a := range req.Args {
			if strings.Contains(a, s) {
				return true
			}
		}
	}
	return false
}

// keepExec keeps a delivery until its exec_result arrives.
func (b *Broker) keepExec(c *call, x *execCall, canon []protocol.Reference, grantID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	sid := c.s.Facts.SessionID
	if _, ended := b.ended[sid]; ended {
		return
	}
	list := b.execs[sid]
	if len(list) >= maxPendingExecs {
		list = list[1:]
	}
	b.execs[sid] = append(list, &execRecord{
		requestID: x.req.ID, claims: c.claims, refs: canon, command: c.command, grantID: grantID,
	})
}

// takeExecLocked removes and returns the delivery of the session with the
// request ID, or nil. The caller holds mu.
func (b *Broker) takeExecLocked(sid session.ID, requestID string) *execRecord {
	list := b.execs[sid]
	for i, r := range list {
		if r.requestID == requestID {
			b.execs[sid] = append(list[:i:i], list[i+1:]...)
			return r
		}
	}
	return nil
}

// ExecResult implements daemon.AgentHandler. It writes the event
// "execution" for one delivery of the session (D65, G39). The fields of
// the result are claims of "a38 mcp"; the event keeps them apart from the
// verified facts. A result without a delivery is invalid_request.
func (b *Broker) ExecResult(_ context.Context, s session.Session, req *protocol.ExecResult) (out *protocol.ExecRecorded, e *protocol.Error) {
	defer b.guard("exec_result", func() { out, e = nil, faultedError() })
	c := &call{s: s}
	if req == nil || s.Facts.SessionID == "" {
		return nil, protocol.Errorf(protocol.CodeInvalidRequest, "the request has no session")
	}
	if err := req.Validate(); err != nil {
		return nil, asProtocolError(err)
	}
	b.mu.Lock()
	rec := b.takeExecLocked(s.Facts.SessionID, req.RequestID)
	b.mu.Unlock()
	if rec == nil {
		return nil, b.reject(c, audit.EventError, nil, protocol.Errorf(protocol.CodeInvalidRequest,
			"no delivery of the session waits for this result"))
	}
	ev := audit.Event{
		Type:       audit.EventExecution,
		Verified:   s.Facts,
		Claims:     rec.claims,
		References: rec.refs,
		Command:    rec.command,
		Result:     audit.Result{GrantID: rec.grantID},
		Execution: &audit.Execution{
			ExecRequestID:   req.RequestID,
			ExitCode:        req.ExitCode,
			Signal:          req.Signal,
			TimedOut:        req.TimedOut,
			DurationMS:      req.DurationMS,
			StdoutBytes:     req.StdoutBytes,
			StderrBytes:     req.StderrBytes,
			StdoutTruncated: req.StdoutTruncated,
			StderrTruncated: req.StderrTruncated,
			Masked:          req.Masked,
		},
	}
	if req.Error != nil {
		ev.Execution = &audit.Execution{ExecRequestID: req.RequestID, NotStarted: true}
		ev.Result.Code, ev.Result.Detail = protocol.CodeExecFailed, audit.DetailNotStarted
	}
	if e := b.record(ev); e != nil {
		return nil, e
	}
	return &protocol.ExecRecorded{ID: req.ID}, nil
}

// commandOf returns the command of the request for the audit events (D65):
// the arguments up to audit.MaxCommandArgsBytes, the variables with their
// references. It never holds a value.
func commandOf(req *protocol.RequestExec) *audit.Command {
	args, cut := audit.TruncateArgs(req.Args)
	cmd := &audit.Command{
		Way: audit.WayExec, Program: req.Program, Args: args, ArgsTruncated: cut,
		Cwd: req.Cwd, TimeoutSeconds: req.TimeoutSeconds,
	}
	for _, v := range req.Env {
		cmd.Env = append(cmd.Env, audit.EnvVar{Name: v.Name, Reference: v.Reference})
	}
	return cmd
}

// execRefs returns the references of the variables in their sequence.
func execRefs(req *protocol.RequestExec) []protocol.Reference {
	out := make([]protocol.Reference, 0, len(req.Env))
	for _, v := range req.Env {
		out = append(out, v.Reference)
	}
	return out
}
