package broker

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/approval/fakeui"
	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/clock"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// checkNoValueInLog fails if the log of the daemon holds a value of the
// fake backend.
func (h *harness) checkNoValueInLog() {
	h.t.Helper()
	logs := h.logs.String()
	if strings.Contains(logs, marker) || strings.Contains(logs, "beispiel-wert") {
		h.t.Errorf("the log holds a value:\n%s", logs)
	}
}

// T26: a failed write of the audit log delivers nothing, creates no grant,
// reads no value after the failure, and tells the human exactly once. The
// next request ends with daemon_faulted (D48). The log names the failure
// without a value.
func TestT26AuditWriteFailure(t *testing.T) {
	tests := []struct {
		name string
		// covered: the session has a grant before the failure.
		covered bool
		failOn  audit.EventType
	}{
		{"entry of the request", false, audit.EventRequest},
		{"entry of the grant after the approval", false, audit.EventGrant},
		{"entry of the delivery under an existing grant", true, audit.EventDelivery},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, withRealLock())
			s := sess("s-1", "beispielprojekt")
			if tt.covered {
				h.rec.Enqueue(fakeui.ReplyApproveAll())
				if r := h.approveFlow(s, refPass); r.err != nil {
					t.Fatal(r.err)
				}
			}
			grants, reads := len(h.grants()), h.be.Reads()
			h.store.FailWhen(func(ev audit.Event) bool { return ev.Type == tt.failOn })
			h.rec.Enqueue(fakeui.ReplyApproveAll())
			var r result
			if tt.failOn == audit.EventGrant {
				r = h.approveFlow(s, refPass)
			} else {
				r = h.do(s, refPass)
			}
			if r.code() != protocol.CodeAuditFailed || r.creds != nil {
				t.Fatalf("result = %v, %v; want audit_failed", r.creds, r.err)
			}
			if got := len(h.grants()); got != grants {
				t.Errorf("grants = %d, want %d", got, grants)
			}
			if len(h.eventsOf(audit.EventGrant)) != grants {
				t.Errorf("a grant event exists for the failed request")
			}
			if h.be.Reads() != reads {
				t.Errorf("reads after the failure = %d", h.be.Reads()-reads)
			}
			// The next request is locked: no backend, no human.
			approvals, resolves := len(h.rec.Requests()), h.be.Resolves()
			if r := h.do(s, refPass); r.code() != protocol.CodeDaemonFaulted || r.creds != nil {
				t.Fatalf("next request = %v; want daemon_faulted", r.err)
			}
			if h.be.Reads() != reads || h.be.Resolves() != resolves || len(h.rec.Requests()) != approvals {
				t.Errorf("the locked request reached the backend or the human")
			}
			ns := h.faultNotices()
			if len(ns) != 1 || len(h.notices()) != 1 {
				t.Fatalf("notices = %+v", h.notices())
			}
			if ns[0].Code != protocol.CodeAuditFailed || !strings.Contains(ns[0].Message, "audit log") {
				t.Errorf("notice = %+v", ns[0])
			}
			for _, want := range []string{"cannot write the audit log", "delivery is locked", "audit_failed"} {
				if !strings.Contains(h.logs.String(), want) {
					t.Errorf("the log lacks %q", want)
				}
			}
			h.checkNoValueInLog()
		})
	}
}

// T28: a fault of the daemon locks delivery for all sessions and projects
// until the human lifts the lock.
func TestT28GlobalLock(t *testing.T) {
	h := newHarness(t, withRealLock())
	ctx := context.Background()
	a := sess("s-a", "beispielprojekt")
	b := sess("s-b", "anderesprojekt")

	// A has two grants.
	h.rec.Enqueue(fakeui.ReplyApproveAll(), fakeui.ReplyApproveAll())
	if r := h.approveFlow(a, refPass); r.err != nil {
		t.Fatal(r.err)
	}
	if r := h.approveFlow(a, refObj); r.err != nil {
		t.Fatal(r.err)
	}
	gs := h.grants()
	if len(gs) != 2 {
		t.Fatalf("grants = %d", len(gs))
	}

	// The write fails at the request of B.
	h.store.FailAppends(nil)
	if r := h.do(b, refExtern); r.code() != protocol.CodeAuditFailed {
		t.Fatalf("B = %v; want audit_failed", r.err)
	}
	failedSeq := len(h.events())

	// Each request of A and B is daemon_faulted, also under the grant of A.
	approvals, reads, resolves := len(h.rec.Requests()), h.be.Reads(), h.be.Resolves()
	locked := func(when string) {
		t.Helper()
		for _, c := range []struct {
			name string
			s    session.Session
			ref  string
		}{
			{"A under its grant", a, refPass},
			{"A without grant", a, refUser},
			{"B", b, refExtern},
		} {
			if r := h.do(c.s, c.ref); r.code() != protocol.CodeDaemonFaulted || r.creds != nil {
				t.Errorf("%s: %s = %v; want daemon_faulted", when, c.name, r.err)
			}
		}
		if len(h.rec.Requests()) != approvals || h.be.Reads() != reads || h.be.Resolves() != resolves {
			t.Errorf("%s: a locked request reached the human or the backend", when)
		}
	}
	locked("during the fault")
	if n := len(h.faultNotices()); n != 1 {
		t.Fatalf("notices = %d, want 1", n)
	}

	// Admin commands work during the lock.
	if len(h.grants()) != 2 {
		t.Errorf("grants list during the lock")
	}
	st, e := h.b.Status(ctx, &protocol.Status{ID: "st"})
	if e != nil || st.Fault == nil || st.Fault.Code != protocol.CodeAuditFailed || st.Fault.Since.IsZero() ||
		st.Fault.CheckPassed == nil || *st.Fault.CheckPassed || st.Fault.Finding == "" || !st.Fault.BeginPending {
		t.Fatalf("status = %+v, %v", st, e)
	}
	if err := st.Validate(); err != nil {
		t.Errorf("status is not valid: %v", err)
	}

	// Clear with the cause present keeps the lock and names the finding.
	res, e := h.b.FaultClear(ctx, &protocol.FaultClear{ID: "fc1"})
	if e != nil || res.Cleared || !strings.Contains(res.Finding, "audit log cannot be written") {
		t.Fatalf("clear with the cause = %+v, %v", res, e)
	}
	if err := res.Validate(); err != nil {
		t.Errorf("clear result is not valid: %v", err)
	}
	locked("after a failed clear")

	// The cause goes away: the lock stays.
	h.store.Heal()
	locked("after the repair")
	st, _ = h.b.Status(ctx, &protocol.Status{ID: "st"})
	if st.Fault == nil || st.Fault.CheckPassed == nil || !*st.Fault.CheckPassed {
		t.Fatalf("status after the repair = %+v", st.Fault)
	}
	// A revocation works during the lock.
	if _, e := h.b.GrantRevoke(ctx, &protocol.GrantRevoke{ID: "rv", GrantID: gs[1].GrantID}); e != nil {
		t.Fatalf("revoke during the lock: %v", e)
	}
	if len(h.grants()) != 1 {
		t.Fatal("the revocation did not end the grant")
	}

	// The human lifts the lock: both sessions work again.
	res, e = h.b.FaultClear(ctx, &protocol.FaultClear{ID: "fc2"})
	if e != nil || !res.Cleared || res.Finding != "" {
		t.Fatalf("clear = %+v, %v", res, e)
	}
	if st, _ := h.b.Status(ctx, &protocol.Status{ID: "st"}); st.Fault != nil {
		t.Fatalf("status after the clear = %+v", st.Fault)
	}
	if r := h.do(a, refPass); r.err != nil || values(r.creds)[refPass] != marker {
		t.Fatalf("A after the clear = %v", r.err)
	}
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	if r := h.approveFlow(b, refExtern); r.err != nil || values(r.creds)[refExtern] == "" {
		t.Fatalf("B after the clear = %v", r.err)
	}

	// The audit log has the begin and the end, after the failed request.
	var details []string
	for _, ev := range h.events()[failedSeq:] {
		if ev.Type == audit.EventError && (ev.Result.Detail == audit.DetailFaultBegin || ev.Result.Detail == audit.DetailFaultEnd) {
			details = append(details, ev.Result.Detail)
		}
	}
	if !slices.Equal(details, []string{audit.DetailFaultBegin, audit.DetailFaultEnd}) {
		t.Fatalf("lock entries = %v", details)
	}
	if v, err := h.inner.Verify(ctx); err != nil || !v.OK {
		t.Fatalf("chain = %+v, %v", v, err)
	}
	if n := len(h.faultNotices()); n != 1 {
		t.Errorf("notices = %d, want 1", n)
	}
	h.checkNoValueInLog()

	// A restart with the cause present leads into the lock again.
	h.store.FailAppends(nil)
	h.restartWithLock()
	if r := h.do(a, refPass); r.code() != protocol.CodeDaemonFaulted {
		t.Fatalf("after the restart = %v; want daemon_faulted", r.err)
	}
	if n := len(h.faultNotices()); n != 2 {
		t.Errorf("notices after the restart = %d, want 2", n)
	}
	// A restart without the cause starts open.
	h.store.Heal()
	h.restartWithLock()
	if e := h.lock.Check(); e != nil {
		t.Fatalf("after the restart without the cause: %v", e)
	}
}

// T28 (part), D28, D35: these failures are not a fault of the daemon. They
// end the request with their own code and never lock delivery.
func TestFailuresThatDoNotLock(t *testing.T) {
	tests := []struct {
		name  string
		setup func(h *harness)
		flow  bool
		sess  string
		ref   string
		want  protocol.Code
	}{
		{"invalid project configuration", nil, false, "kaputtprojekt", refPass, protocol.CodeConfigInvalid},
		{"policy", nil, false, "beispielprojekt", refExtern, protocol.CodeNotAllowed},
		{"backend locked", func(h *harness) { h.be.SetLocked(true) }, false, "beispielprojekt", refPass, protocol.CodeBackendLocked},
		{"backend unreachable", func(h *harness) { h.be.SetUnavailable(true) }, false, "beispielprojekt", refPass, protocol.CodeBackendUnavailable},
		{"UI fails", func(h *harness) { h.rec.Enqueue(fakeui.ReplyFail(errors.New("the program failed"))) }, true, "beispielprojekt", refPass, protocol.CodeUIFailed},
		{"UI timeout", func(h *harness) { h.rec.Enqueue(fakeui.ReplyTimeout()) }, true, "beispielprojekt", refPass, protocol.CodeTimeout},
		{"human denies", func(h *harness) { h.rec.Enqueue(fakeui.ReplyDeny()) }, true, "beispielprojekt", refPass, protocol.CodeDenied},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, withRealLock())
			if tt.setup != nil {
				tt.setup(h)
			}
			s := sess("s-1", tt.sess)
			var r result
			if tt.flow {
				r = h.approveFlow(s, tt.ref)
			} else {
				r = h.do(s, tt.ref)
			}
			if r.code() != tt.want {
				t.Fatalf("result = %v; want %s", r.err, tt.want)
			}
			if e := h.lock.Check(); e != nil {
				t.Fatalf("the failure locked delivery: %v", e)
			}
			if n := len(h.faultNotices()); n != 0 {
				t.Errorf("fault notices = %d", n)
			}
			// Another project still works.
			h.be.SetLocked(false)
			h.be.SetUnavailable(false)
			h.rec.Enqueue(fakeui.ReplyApproveAll())
			if r := h.approveFlow(sess("s-2", "anderesprojekt"), refExtern); r.err != nil {
				t.Fatalf("other project: %v", r.err)
			}
		})
	}
}

// D48, D36: a failed write of the audit log on each path locks delivery,
// also for the expiry of a grant and the end of a session.
func TestAuditFailureOnEachPathLocks(t *testing.T) {
	tests := []struct {
		name   string
		failOn audit.EventType
		act    func(h *harness)
	}{
		{"end of a session", audit.EventExpiry, func(h *harness) { h.b.SessionEnded(sess("s-1", "beispielprojekt")) }},
		{"end of the lifetime", audit.EventExpiry, func(h *harness) {
			h.clk.Advance(5 * time.Hour)
			h.b.sweep()
		}},
		{"revocation", audit.EventRevocation, func(h *harness) {
			g := h.grants()[0]
			_, _ = h.b.GrantRevoke(context.Background(), &protocol.GrantRevoke{ID: "rv", GrantID: g.GrantID})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, withRealLock())
			h.rec.Enqueue(fakeui.ReplyApproveAll())
			if r := h.approveFlow(sess("s-1", "beispielprojekt"), refPass); r.err != nil {
				t.Fatal(r.err)
			}
			h.store.FailWhen(func(ev audit.Event) bool { return ev.Type == tt.failOn })
			tt.act(h)
			if f := h.lock.Fault(); f == nil || f.Code != protocol.CodeAuditFailed {
				t.Fatalf("fault = %+v", f)
			}
			if r := h.do(sess("s-2", "anderesprojekt"), refExtern); r.code() != protocol.CodeDaemonFaulted {
				t.Fatalf("next request = %v", r.err)
			}
			if n := len(h.faultNotices()); n != 1 {
				t.Errorf("notices = %d", n)
			}
		})
	}
}

// G21, D48: a panic of the broker is a fault of the daemon. The audit log
// works, so the begin entry is written at once, and the lock can be lifted
// at once. A panic in a handler is caught by the broker.
func TestPanicLocksDelivery(t *testing.T) {
	h := newHarness(t, withRealLock(), withUI(func(rec *fakeui.Recorder, _ *clock.Fake) approval.UI { return rec }))
	h.rec.Enqueue(func(context.Context, approval.Request) (approval.Decision, error) {
		panic("the UI broke")
	})
	r := h.approveFlow(sess("s-1", "beispielprojekt"), refPass)
	if r.code() != protocol.CodeDaemonFaulted || h.be.Reads() != 0 {
		t.Fatalf("result = %v", r.err)
	}
	if r := h.do(sess("s-2", "anderesprojekt"), refExtern); r.code() != protocol.CodeDaemonFaulted {
		t.Fatalf("next request = %v", r.err)
	}
	f := h.lock.Fault()
	if f == nil || f.Code != protocol.CodeDaemonFaulted || f.BeginPending {
		t.Fatalf("fault = %+v", f)
	}
	if n := len(h.faultNotices()); n != 1 {
		t.Errorf("notices = %d", n)
	}
	// The locked request is in the audit log (D36).
	locked := 0
	for _, ev := range h.eventsOf(audit.EventError) {
		if ev.Result.Code == protocol.CodeDaemonFaulted && ev.Verified.SessionID == "s-2" {
			locked++
		}
	}
	if locked != 1 {
		t.Errorf("events of the locked request = %d", locked)
	}
	if res, _ := h.b.FaultClear(context.Background(), &protocol.FaultClear{ID: "fc"}); !res.Cleared {
		t.Fatalf("clear = %+v", res)
	}
}

// G21: a panic in a handler on the request path raises the fault and
// answers daemon_faulted.
func TestPanicInHandlerLocks(t *testing.T) {
	h := newHarness(t, withRealLock())
	h.b.be = nil // makes the call of the backend panic
	r := h.do(sess("s-1", "beispielprojekt"), refPass)
	if r.code() != protocol.CodeDaemonFaulted || r.creds != nil {
		t.Fatalf("result = %v", r.err)
	}
	if h.lock.Check() == nil {
		t.Fatal("the panic did not lock delivery")
	}
	if !strings.Contains(h.logs.String(), "internal fault of the broker") {
		t.Error("the log does not name the fault")
	}
}

// T26, T28 (part), race detector: many requests of two sessions while the
// audit log fails. Each ends with audit_failed or daemon_faulted, nothing
// is read, and the human gets exactly one notice. After the raise, each
// new request is daemon_faulted.
func TestConcurrentRequestsDuringFault(t *testing.T) {
	h := newHarness(t, withRealLock())
	a := sess("s-a", "beispielprojekt")
	b := sess("s-b", "anderesprojekt")
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	if r := h.approveFlow(a, refPass); r.err != nil {
		t.Fatal(r.err)
	}
	reads := h.be.Reads()
	h.store.FailAppends(nil)

	run := func(n int) map[protocol.Code]int {
		var mu sync.Mutex
		codes := map[protocol.Code]int{}
		var wg sync.WaitGroup
		for i := range n {
			wg.Go(func() {
				s, ref := a, refPass
				if i%2 == 1 {
					s, ref = b, refExtern
				}
				c, e := h.b.RequestCredentials(context.Background(), s, newRequest(t, ref))
				mu.Lock()
				defer mu.Unlock()
				if c != nil {
					codes["value"]++
					return
				}
				codes[e.Code]++
			})
		}
		wg.Wait()
		return codes
	}
	first := run(64)
	if first["value"] != 0 || first[protocol.CodeAuditFailed]+first[protocol.CodeDaemonFaulted] != 64 || first[protocol.CodeAuditFailed] == 0 {
		t.Fatalf("codes = %v", first)
	}
	second := run(64)
	if second[protocol.CodeDaemonFaulted] != 64 {
		t.Fatalf("codes after the raise = %v", second)
	}
	if h.be.Reads() != reads {
		t.Errorf("reads during the fault = %d", h.be.Reads()-reads)
	}
	if n := len(h.faultNotices()); n != 1 {
		t.Errorf("notices = %d", n)
	}
}

// T27 (part), D48: the status names the backend state and the UI mode.
func TestStatusBackendAndUI(t *testing.T) {
	tests := []struct {
		name  string
		setup func(h *harness)
		want  string
	}{
		{"backend ok", nil, protocol.BackendOK},
		{"backend locked", func(h *harness) { h.be.SetLocked(true) }, protocol.BackendLocked},
		{"backend unreachable", func(h *harness) { h.be.SetUnavailable(true) }, protocol.BackendUnreachable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, withRealLock())
			if tt.setup != nil {
				tt.setup(h)
			}
			st, e := h.b.Status(context.Background(), &protocol.Status{ID: "st"})
			if e != nil || st.Backend == nil || st.Backend.State != tt.want {
				t.Fatalf("status = %+v, %v", st, e)
			}
			if (tt.want == protocol.BackendOK) != (st.Backend.Reason == "") {
				t.Errorf("reason = %q", st.Backend.Reason)
			}
			if st.UI == nil || st.UI.Mode != string(approval.ModeScript) || !st.UI.Usable {
				t.Errorf("ui = %+v", st.UI)
			}
			if st.Fault != nil {
				t.Errorf("fault = %+v", st.Fault)
			}
			if err := st.Validate(); err != nil {
				t.Errorf("status is not valid: %v", err)
			}
		})
	}
}

// F12, F20: the status names the backend kind, so that a38 doctor checks
// the token file only for the 1Password backend.
func TestStatusBackendKind(t *testing.T) {
	h := newHarness(t)
	h.b = New(h.cfg, h.be, h.rec, h.store, h.clk, slog.New(slog.DiscardHandler), WithBackendKind(backend.KindOnePassword))
	st, e := h.b.Status(context.Background(), &protocol.Status{ID: "st"})
	if e != nil || st.Backend == nil || st.Backend.Kind != string(backend.KindOnePassword) {
		t.Fatalf("status = %+v, %v", st, e)
	}
}
