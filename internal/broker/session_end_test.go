package broker

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/approval/fakeui"
	"github.com/seletz/passierschein/internal/approval/queue"
	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/clock"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// slowBackend blocks Resolve or Read until the test releases it. With
// honorCtx it returns the error of the context when the context ends
// first; else it ignores the context.
type slowBackend struct {
	backend.Backend
	onResolve, onRead bool
	honorCtx          bool
	entered           chan struct{}
	release           chan struct{}
}

func newSlow(be backend.Backend) *slowBackend {
	return &slowBackend{Backend: be, entered: make(chan struct{}, 1), release: make(chan struct{})}
}

func (s *slowBackend) block(ctx context.Context) error {
	s.entered <- struct{}{}
	if s.honorCtx {
		select {
		case <-s.release:
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	}
	<-s.release
	return nil
}

func (s *slowBackend) Resolve(ctx context.Context, ref protocol.Reference) (backend.Names, error) {
	if s.onResolve {
		if err := s.block(ctx); err != nil {
			return backend.Names{}, err
		}
		ctx = context.Background()
	}
	return s.Backend.Resolve(ctx, ref)
}

func (s *slowBackend) Read(ctx context.Context, ref protocol.Reference) (protocol.Secret, error) {
	if s.onRead {
		if err := s.block(ctx); err != nil {
			return protocol.Secret{}, err
		}
		ctx = context.Background()
	}
	return s.Backend.Read(ctx, ref)
}

// T16, D33: a request that is in flight when its session ends never
// creates a new state for the session, never reaches the human and never
// gets a grant. The daemon calls SessionEnded before the handler of the
// request returns. The second case is the mirror: a request under a grant
// writes no delivery event after the end.
func TestSessionEndedWhileRequestInFlight(t *testing.T) {
	tests := []struct {
		name    string
		covered bool
		cancel  bool
	}{
		{"new credential, context ends", false, true},
		{"new credential, only SessionEnded", false, false},
		{"covered credential, context ends", true, true},
		{"covered credential, only SessionEnded", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			s := sess("s-dead", "beispielprojekt")
			if tt.covered {
				h.rec.Enqueue(fakeui.ReplyApproveAll())
				if r := h.approveFlow(s, refPass); r.err != nil {
					t.Fatal(r.err)
				}
			}
			slow := newSlow(h.be)
			slow.onResolve = true
			h.b.be = slow
			h.rec.Enqueue(fakeui.ReplyApproveAll())
			deliveries := len(h.eventsOf(audit.EventDelivery))
			approvals := len(h.rec.Requests())

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ch := h.async(ctx, s, newRequest(t, refPass))
			<-slow.entered
			if tt.cancel {
				cancel()
			}
			h.b.SessionEnded(s)
			close(slow.release)
			r := h.wait(ch)
			if r.err == nil || r.creds != nil {
				t.Fatalf("result = %v, %v; want an error and no value", r.creds, r.err)
			}
			if h.clk.Pending() != 0 {
				t.Errorf("a bundle window started for the ended session")
			}
			if n := len(h.rec.Requests()); n != approvals {
				t.Errorf("approval requests for the ended session = %d", n-approvals)
			}
			if len(h.grants()) != 0 {
				t.Errorf("a grant exists for the ended session")
			}
			if n := len(h.eventsOf(audit.EventDelivery)); n != deliveries {
				t.Errorf("delivery events after the end = %d", n-deliveries)
			}
			if h.b.openLen(s.Facts.SessionID) != 0 || h.b.hasState(s.Facts.SessionID) {
				t.Errorf("the ended session has a state again")
			}
		})
	}
}

// hasState reports whether the broker holds a state for the session.
func (b *Broker) hasState(sid session.ID) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, ok := b.sessions[sid]
	return ok
}

// D35, F10: when the connection ends during a call of the backend, the
// audit log shows the end of the session and not a fault of the backend.
func TestContextEndsDuringBackendCall(t *testing.T) {
	for _, atRead := range []bool{false, true} {
		name := map[bool]string{false: "resolve", true: "read"}[atRead]
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			s := sess("s-1", "beispielprojekt")
			if atRead {
				h.rec.Enqueue(fakeui.ReplyApproveAll())
				if r := h.approveFlow(s, refPass); r.err != nil {
					t.Fatal(r.err)
				}
			}
			slow := newSlow(h.be)
			slow.honorCtx = true
			slow.onResolve, slow.onRead = !atRead, atRead
			h.b.be = slow
			ctx, cancel := context.WithCancel(context.Background())
			ch := h.async(ctx, s, newRequest(t, refPass))
			<-slow.entered
			cancel()
			r := h.wait(ch)
			if r.err == nil || r.creds != nil {
				t.Fatalf("result = %v, %v", r.creds, r.err)
			}
			if n := len(h.eventsOf(audit.EventError)); n != 0 {
				t.Errorf("error events = %d; the backend did not fail", n)
			}
			exp := h.eventsOf(audit.EventExpiry)
			if len(exp) != 1 || exp[0].Result.Detail != audit.DetailSessionEnded {
				t.Errorf("expiry events = %+v", exp)
			}
		})
	}
}

// G21: a panic while the broker decides a bundle is an internal fault.
// The daemon keeps running, the fault is raised, the request ends with
// daemon_faulted and nothing is delivered. The approval slot is free
// again.
func TestPanicInUIIsDaemonFault(t *testing.T) {
	lock := &spyLock{}
	h := newHarness(t, withLockOpt(lock), withUI(func(rec *fakeui.Recorder, _ *clock.Fake) approval.UI { return rec }))
	h.rec.Enqueue(func(context.Context, approval.Request) (approval.Decision, error) {
		panic("the UI broke")
	}, fakeui.ReplyApproveAll())
	r := h.approveFlow(sess("s-1", "beispielprojekt"), refPass)
	if r.code() != protocol.CodeDaemonFaulted || h.be.Reads() != 0 || len(h.grants()) != 0 {
		t.Fatalf("result = %v, reads %d", r.err, h.be.Reads())
	}
	if !slices.Contains(lock.raisedCodes(), protocol.CodeDaemonFaulted) {
		t.Errorf("the fault was not raised")
	}
	errs := h.eventsOf(audit.EventError)
	if len(errs) != 1 || errs[0].Result.Code != protocol.CodeDaemonFaulted {
		t.Errorf("error events = %+v", errs)
	}
	// The slot is free: a later bundle reaches the UI.
	if r := h.approveFlow(sess("s-2", "beispielprojekt"), refPass); r.err != nil {
		t.Fatalf("later request: %v", r.err)
	}
}

// T5, D8: the approval request shows the grants at the time of the
// display. A second bundle of the session waits while the first is at the
// UI; the grant of the first bundle is the existing grant of the second.
func TestApprovalRequestBuiltAtItsTurn(t *testing.T) {
	h := newHarness(t)
	s := sess("s-1", "beispielprojekt")
	release := make(chan struct{})
	h.rec.Enqueue(func(ctx context.Context, req approval.Request) (approval.Decision, error) {
		<-release
		return fakeui.AutoApprove{}.Approve(ctx, req)
	}, fakeui.ReplyApproveAll())
	ctx := context.Background()
	ch1 := h.async(ctx, s, newRequest(t, refUser))
	h.waitTimers(1)
	h.clk.Advance(h.cfg.Bundle.Window)
	h.waitFor("the first approval request", func() bool { return len(h.rec.Requests()) == 1 })
	ch2 := h.async(ctx, s, newRequest(t, refPass, refUser))
	h.waitOpen(s, 2)
	// The first approval request holds a queue timer.
	h.waitTimers(2)
	h.clk.Advance(h.cfg.Bundle.Window)
	// The second bundle closed and waits for its turn at the UI.
	h.waitFor("the second bundle", func() bool { return len(h.eventsOf(audit.EventBundle)) == 2 })
	time.Sleep(20 * time.Millisecond) // let the runner reach the wait for the slot
	close(release)
	if r := h.wait(ch1); r.err != nil {
		t.Fatal(r.err)
	}
	if r := h.wait(ch2); r.err != nil || len(values(r.creds)) != 2 {
		t.Fatalf("second: %v, %v", r.creds, r.err)
	}
	reqs := h.rec.Requests()
	first := h.eventsOf(audit.EventGrant)[0].Result.GrantID
	second := reqs[1]
	if second.ExistingGrant == nil || second.ExistingGrant.GrantID != first {
		t.Fatalf("existing grant = %+v, want %s", second.ExistingGrant, first)
	}
	// The entry that the first grant covers now is not in the request.
	if len(second.Credentials) != 1 || second.Credentials[0].Field != "password" ||
		second.Credentials[0].Deviation != approval.DeviationNewItem {
		t.Fatalf("credentials = %+v", second.Credentials)
	}
}

// T2: a bundle whose window ended but whose runner did not close it yet
// takes no further request. The request opens a new bundle.
func TestBundleOpenButExpiredOpensNewBundle(t *testing.T) {
	h := newHarness(t, withUI(func(rec *fakeui.Recorder, clk *clock.Fake) approval.UI { return queue.New(rec, clk, 0) }))
	s := sess("s-1", "beispielprojekt")
	h.rec.Enqueue(fakeui.ReplyApproveAll(), fakeui.ReplyApproveAll())
	ctx := context.Background()
	ch1 := h.async(ctx, s, newRequest(t, refUser))
	h.waitTimers(1)
	h.waitOpen(s, 1)

	late := &call{s: s, req: newRequest(t, refPass)}
	late.claims, late.refs = s.Claims("", ""), late.req.References
	if e := h.b.resolve(ctx, late); e != nil {
		t.Fatal(e)
	}
	// Hold the lock: the runner of the first bundle cannot close it.
	h.b.mu.Lock()
	old := h.b.sessions[s.Facts.SessionID].open
	h.clk.Advance(h.cfg.Bundle.Window)
	if old.b.Accepts(h.clk.Now()) {
		h.b.mu.Unlock()
		t.Fatal("the old bundle still accepts")
	}
	p := h.b.placeLocked(late)
	next := h.b.sessions[s.Facts.SessionID].open
	h.b.mu.Unlock()
	if next == old || p.wait == nil || len(old.requests) != 1 || len(next.requests) != 1 {
		t.Fatalf("the late request joined the old bundle")
	}
	if r := h.wait(ch1); r.err != nil {
		t.Fatal(r.err)
	}
	h.waitTimers(1)
	h.clk.Advance(h.cfg.Bundle.Window)
	<-p.wait.done
	reqs := h.rec.Requests()
	if len(reqs) != 2 || len(reqs[0].Credentials) != 1 || len(reqs[1].Credentials) != 1 {
		t.Fatalf("approval requests = %+v", reqs)
	}
}
