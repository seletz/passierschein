package broker

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/approval/fakeui"
	"github.com/seletz/passierschein/internal/approval/queue"
	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/clock"
	"github.com/seletz/passierschein/internal/protocol"
)

// T1 (in process), UC1, UC2, D36: the flow with automatic approval
// delivers both values and writes request, bundle, decision, grant and
// delivery in this sequence. A covered request writes request and
// delivery with the grant.
func TestFlowAuditSequence(t *testing.T) {
	h := newHarness(t, withUI(func(_ *fakeui.Recorder, clk *clock.Fake) approval.UI {
		return queue.New(fakeui.AutoApprove{}, clk, 0)
	}))
	s := sess("s-1", "beispielprojekt")
	r := h.approveFlow(s, refUser, refPass)
	if r.err != nil {
		t.Fatal(r.err)
	}
	if v := values(r.creds); v[refUser] != "beispiel-wert-1" || v[refPass] != marker {
		t.Fatalf("values missing")
	}
	if r.creds.ID != "r1" || r.creds.Entries[0].Reference != ref(t, refUser) {
		t.Errorf("answer = %+v", r.creds)
	}
	want := []audit.EventType{
		audit.EventRequest, audit.EventBundle, audit.EventDecision, audit.EventGrant, audit.EventDelivery,
	}
	if got := h.types(); !slices.Equal(got, want) {
		t.Fatalf("audit = %v, want %v", got, want)
	}
	grantID := h.eventsOf(audit.EventGrant)[0].Result.GrantID
	if d := h.eventsOf(audit.EventDelivery)[0]; d.Result.GrantID != grantID || len(d.References) != 2 {
		t.Errorf("delivery = %+v", d)
	}
	if r := h.do(s, refPass); r.err != nil {
		t.Fatal(r.err)
	}
	got := h.types()[len(want):]
	if !slices.Equal(got, []audit.EventType{audit.EventRequest, audit.EventDelivery}) {
		t.Errorf("covered audit = %v", got)
	}
	if d := h.eventsOf(audit.EventDelivery)[1]; d.Result.GrantID != grantID {
		t.Errorf("covered delivery names grant %q", d.Result.GrantID)
	}
	// Each entry passes the check of the wire protocol.
	if err := r.creds.Validate(); err != nil {
		t.Errorf("answer is not valid: %v", err)
	}
}

// T17, UC14, D35: a locked or unavailable backend gives backend_locked or
// backend_unavailable, never a value, and one error event with the code.
func TestBackendErrors(t *testing.T) {
	tests := []struct {
		name   string
		set    func(h *harness)
		want   protocol.Code
		atRead bool
	}{
		{"locked at resolve", func(h *harness) { h.be.SetLocked(true) }, protocol.CodeBackendLocked, false},
		{"unavailable at resolve", func(h *harness) { h.be.SetUnavailable(true) }, protocol.CodeBackendUnavailable, false},
		{"locked at read", func(h *harness) { h.be.SetLocked(true) }, protocol.CodeBackendLocked, true},
		{"unavailable at read", func(h *harness) { h.be.SetUnavailable(true) }, protocol.CodeBackendUnavailable, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			var r result
			if tt.atRead {
				// The backend fails after the approval.
				h.rec.Enqueue(func(ctx context.Context, req approval.Request) (approval.Decision, error) {
					tt.set(h)
					return fakeui.AutoApprove{}.Approve(ctx, req)
				})
				r = h.approveFlow(sess("s-1", "beispielprojekt"), refUser, refPass)
			} else {
				tt.set(h)
				r = h.do(sess("s-1", "beispielprojekt"), refUser, refPass)
				if len(h.rec.Requests()) != 0 {
					t.Errorf("an approval request was made")
				}
			}
			if r.code() != tt.want || r.creds != nil {
				t.Fatalf("result = %v, %v; want %s", r.creds, r.err, tt.want)
			}
			errs := h.eventsOf(audit.EventError)
			if len(errs) != 1 || errs[0].Result.Code != tt.want {
				t.Errorf("error events = %+v", errs)
			}
		})
	}
}

// T8, D34: a value appears only in the answer of the approved case. It
// never appears in the log, the audit log, an error, an approval request or
// a notice, on the paths approval, denial, timeout and backend error.
func TestSecretValueNeverLeaks(t *testing.T) {
	h := newHarness(t)
	s := sess("s-1", "beispielprojekt")
	var errs []string
	collect := func(r result) {
		if r.err != nil {
			errs = append(errs, r.err.Error())
		}
		if r.creds != nil {
			for _, e := range r.creds.Entries {
				if e.Error != nil {
					errs = append(errs, e.Error.Error())
				}
			}
		}
	}

	// Approval: the only place of the value.
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	ok := h.approveFlow(s, refPass)
	if values(ok.creds)[refPass] != marker {
		t.Fatalf("approved case has no value")
	}
	collect(ok)
	// Denial, timeout and UI error of another session.
	other := sess("s-2", "beispielprojekt")
	h.rec.Enqueue(fakeui.ReplyDeny())
	collect(h.approveFlow(other, refPass))
	h.rec.Enqueue(fakeui.ReplyTimeout())
	collect(h.approveFlow(other, refPass))
	h.rec.Enqueue(fakeui.ReplyFail(errors.New("ui failed")))
	collect(h.approveFlow(other, refPass))
	// Backend error at the read under the grant.
	h.be.SetUnavailable(true)
	collect(h.do(s, refPass))
	h.be.SetUnavailable(false)
	// Policy and config errors.
	collect(h.do(s, "op://beispiel.app.k8s-prod/datenbank-prod/password"))
	collect(h.do(sess("s-3", "kaputtprojekt"), refPass))

	// The formatted answer shows no value.
	out, _ := json.Marshal(ok.creds)
	places := map[string]string{
		"log":            h.logs.String(),
		"errors":         strings.Join(errs, "\n"),
		"formatted":      string(out),
		"approval":       mustJSON(t, h.rec.Requests()),
		"notices":        mustJSON(t, h.rec.Notices()),
		"audit entries":  mustJSON(t, h.events()),
		"grants (admin)": mustJSON(t, h.grants()),
	}
	for _, suffix := range []string{"", "-wal", "-journal"} {
		raw, err := os.ReadFile(h.auditPath + suffix)
		if err != nil && suffix == "" {
			t.Fatal(err)
		}
		places["audit file"+suffix] = string(raw)
	}
	for name, text := range places {
		if strings.Contains(text, marker) {
			t.Errorf("the value appears in the %s", name)
		}
	}
}

// T8, D34: the broker keeps no value. A changed value in the backend shows
// at the next delivery under the grant, and each delivery reads once.
func TestNoValueCache(t *testing.T) {
	h := newHarness(t)
	s := sess("s-1", "beispielprojekt")
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	if r := h.approveFlow(s, refUser); values(r.creds)[refUser] != "beispiel-wert-1" {
		t.Fatalf("first: %v", r.err)
	}
	before := h.be.ReadsOf(ref(t, refUser))
	if err := h.be.SetValue(ref(t, refUser), "beispiel-wert-neu"); err != nil {
		t.Fatal(err)
	}
	if r := h.do(s, refUser, refUser); values(r.creds)[refUser] != "beispiel-wert-neu" {
		t.Fatalf("second: %v", r.err)
	}
	if got := h.be.ReadsOf(ref(t, refUser)) - before; got != 1 {
		t.Errorf("reads = %d, want 1", got)
	}
}

// T26 (part), D36: a failed audit write delivers nothing, creates no
// grant, reads nothing after the failure and answers audit_failed. The
// broker raises the fault (the lock itself is issue 23).
func TestAuditFailure(t *testing.T) {
	tests := []struct {
		name      string
		failOn    audit.EventType
		covered   bool
		wantGrant bool
	}{
		{"request", audit.EventRequest, false, false},
		{"bundle", audit.EventBundle, false, false},
		{"decision", audit.EventDecision, false, false},
		{"grant", audit.EventGrant, false, false},
		{"delivery after approval", audit.EventDelivery, false, true},
		{"delivery under a grant", audit.EventDelivery, true, true},
		{"denial of the policy", audit.EventDenial, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lock := &spyLock{}
			h := newHarness(t, withLockOpt(lock))
			s := sess("s-1", "beispielprojekt")
			if tt.covered {
				h.rec.Enqueue(fakeui.ReplyApproveAll())
				if r := h.approveFlow(s, refPass); r.err != nil {
					t.Fatal(r.err)
				}
			}
			reads := h.be.Reads()
			h.store.FailWhen(func(ev audit.Event) bool { return ev.Type == tt.failOn })
			h.rec.Enqueue(fakeui.ReplyApproveAll())
			var r result
			switch {
			case tt.failOn == audit.EventDenial:
				r = h.do(s, refExtern)
			case tt.covered, tt.failOn == audit.EventRequest:
				r = h.do(s, refPass)
			default:
				r = h.approveFlow(s, refPass)
			}
			if r.code() != protocol.CodeAuditFailed || r.creds != nil {
				t.Fatalf("result = %v, %v; want audit_failed", r.creds, r.err)
			}
			if h.be.Reads() != reads {
				t.Errorf("reads after the failure = %d", h.be.Reads()-reads)
			}
			if got := len(h.grants()) > 0; got != tt.wantGrant {
				t.Errorf("grant exists = %v, want %v", got, tt.wantGrant)
			}
			if !slices.Contains(lock.raisedCodes(), protocol.CodeAuditFailed) {
				t.Errorf("the fault was not raised")
			}
			if !strings.Contains(h.logs.String(), "cannot write the audit log") {
				t.Errorf("the log does not name the failure")
			}
		})
	}
}

// D48: the gate at the top of each delivery path. While the lock is set,
// each request ends with daemon_faulted and reaches neither the backend
// nor the human. A fault that starts while an approval request is open
// stops the grant and the delivery.
func TestFaultGate(t *testing.T) {
	lock := &spyLock{}
	h := newHarness(t, withLockOpt(lock))
	s := sess("s-1", "beispielprojekt")
	lock.set(true)
	r := h.do(s, refPass)
	if r.code() != protocol.CodeDaemonFaulted || h.be.Resolves() != 0 || len(h.rec.Requests()) != 0 {
		t.Fatalf("locked: %v", r.err)
	}
	st, _ := h.b.Status(context.Background(), &protocol.Status{ID: "st"})
	if st.Fault == nil {
		t.Errorf("status shows no fault")
	}
	if res, _ := h.b.FaultClear(context.Background(), &protocol.FaultClear{ID: "fc"}); !res.Cleared {
		t.Errorf("fault clear = %+v", res)
	}

	// The fault starts during the approval.
	h.rec.Enqueue(func(ctx context.Context, req approval.Request) (approval.Decision, error) {
		lock.set(true)
		return fakeui.AutoApprove{}.Approve(ctx, req)
	})
	r = h.approveFlow(s, refPass)
	if r.code() != protocol.CodeDaemonFaulted || h.be.Reads() != 0 || len(h.eventsOf(audit.EventGrant)) != 0 {
		t.Fatalf("fault during approval: %v, reads %d", r.err, h.be.Reads())
	}

	// The fault starts after the grant: the delivery under the grant stops.
	lock.set(false)
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	if r := h.approveFlow(s, refPass); r.err != nil {
		t.Fatal(r.err)
	}
	reads := h.be.Reads()
	lock.set(true)
	if r := h.do(s, refPass); r.code() != protocol.CodeDaemonFaulted || h.be.Reads() != reads {
		t.Fatalf("fault under grant: %v", r.err)
	}
	// Admin commands work during the lock.
	if g := h.grants(); len(g) != 1 {
		t.Errorf("grants list during the lock = %d", len(g))
	}
}

// NoLock, the default, never locks.
func TestDefaultLockIsOpen(t *testing.T) {
	h := newHarness(t)
	st, e := h.b.Status(context.Background(), &protocol.Status{ID: "st"})
	if e != nil || st.Fault != nil {
		t.Fatalf("status = %+v, %v", st, e)
	}
	res, e := h.b.FaultClear(context.Background(), &protocol.FaultClear{ID: "fc"})
	if e != nil || !res.Cleared || res.ID != "fc" {
		t.Fatalf("fault clear = %+v, %v", res, e)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// hookStore calls a function before it writes an event.
type hookStore struct {
	audit.Store
	before func(audit.Event)
}

func (s hookStore) Append(ctx context.Context, ev audit.Event) (audit.Entry, error) {
	s.before(ev)
	return s.Store.Append(ctx, ev)
}

// D48: the gate before the delivery event. A fault that starts after the
// grant and before the delivery stops the delivery.
func TestFaultGateBeforeDelivery(t *testing.T) {
	lock := &spyLock{}
	h := newHarness(t, withLockOpt(lock))
	h.b.audit = hookStore{Store: h.store, before: func(ev audit.Event) {
		if ev.Type == audit.EventGrant {
			lock.set(true)
		}
	}}
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	r := h.approveFlow(sess("s-1", "beispielprojekt"), refPass)
	if r.code() != protocol.CodeDaemonFaulted || h.be.Reads() != 0 || len(h.eventsOf(audit.EventDelivery)) != 0 {
		t.Fatalf("result = %v, reads = %d", r.err, h.be.Reads())
	}
}
