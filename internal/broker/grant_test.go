package broker

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/approval/fakeui"
	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/protocol"
)

// T4: a request in the scope of a grant needs no approval; after the
// lifetime it needs one again, and the audit log has the expiry.
func TestGrantCoversUntilLifetimeEnds(t *testing.T) {
	tests := []struct {
		name     string
		lifetime string
		reply    fakeui.Reply
		want     time.Duration
	}{
		{"default four hours", "4h", fakeui.ReplyApproveAll(), 4 * time.Hour},
		{"thirty minutes", "30m", fakeui.ReplyApproveAll(), 30 * time.Minute},
		{"twelve hours, no fixed maximum", "12h", fakeui.ReplyApproveAll(), 12 * time.Hour},
		{"human selects fifteen minutes", "4h", fakeui.ReplyApprove(900, "c1"), 15 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, withLifetime(tt.lifetime))
			s := sess("s-1", "beispielprojekt")
			h.rec.Enqueue(tt.reply)
			if r := h.approveFlow(s, refPass); r.err != nil {
				t.Fatalf("first: %v", r.err)
			}
			g := h.eventsOf(audit.EventGrant)
			if len(g) != 1 || g[0].Result.LifetimeSeconds != int64(tt.want/time.Second) {
				t.Fatalf("grant events = %+v", g)
			}
			// Shortly before the end the grant covers the request.
			h.clk.Advance(tt.want - time.Second)
			if r := h.do(s, refPass); r.err != nil {
				t.Fatalf("second: %v", r.err)
			}
			if n := len(h.rec.Requests()); n != 1 {
				t.Fatalf("approval requests = %d, want 1", n)
			}
			// At the end it does not.
			h.clk.Advance(time.Second)
			h.rec.Enqueue(fakeui.ReplyApproveAll())
			if r := h.approveFlow(s, refPass); r.err != nil {
				t.Fatalf("third: %v", r.err)
			}
			if n := len(h.rec.Requests()); n != 2 {
				t.Fatalf("approval requests = %d, want 2", n)
			}
			exp := h.eventsOf(audit.EventExpiry)
			if len(exp) != 1 || exp[0].Result.Detail != audit.DetailLifetimeEnded || exp[0].Result.GrantID != g[0].Result.GrantID {
				t.Errorf("expiry events = %+v", exp)
			}
		})
	}
}

// T4, T23, D32: the broker validates each decision itself. An invalid
// approval creates no grant, delivers nothing and ends with ui_failed. An
// approval without a selected entry is a denial.
func TestInvalidDecisions(t *testing.T) {
	approve := func(mut func(*approval.Decision)) fakeui.Reply {
		return func(ctx context.Context, req approval.Request) (approval.Decision, error) {
			d, err := fakeui.AutoApprove{}.Approve(ctx, req)
			mut(&d)
			return d, err
		}
	}
	tests := []struct {
		name  string
		reply fakeui.Reply
		want  protocol.Code
	}{
		{"lifetime zero", fakeui.ReplyApprove(0, "c1"), protocol.CodeUIFailed},
		{"lifetime negative", fakeui.ReplyApprove(-60, "c1"), protocol.CodeUIFailed},
		{"lifetime longer than the configuration", fakeui.ReplyApprove(4*3600+1, "c1"), protocol.CodeUIFailed},
		{"entry not in the request", fakeui.ReplyApprove(60, "c1", "c9"), protocol.CodeUIFailed},
		{"entry twice", fakeui.ReplyApprove(60, "c1", "c1"), protocol.CodeUIFailed},
		{"unknown outcome", approve(func(d *approval.Decision) { d.Outcome = "maybe" }), protocol.CodeUIFailed},
		{"outcome error", approve(func(d *approval.Decision) { d.Outcome = approval.OutcomeError }), protocol.CodeUIFailed},
		{"other request ID", approve(func(d *approval.Decision) { d.RequestID = "approval-fremd" }), protocol.CodeUIFailed},
		{"wrong version", approve(func(d *approval.Decision) { d.Version = 2 }), protocol.CodeUIFailed},
		{"wrong type", approve(func(d *approval.Decision) { d.Type = approval.TypeRequest }), protocol.CodeUIFailed},
		{"failed authentication", approve(func(d *approval.Decision) { d.Auth = &approval.AuthResult{Policy: "biometrics"} }), protocol.CodeUIFailed},
		{"approval without entry", fakeui.ReplyApprove(60), protocol.CodeDenied},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.rec.Enqueue(tt.reply)
			r := h.approveFlow(sess("s-1", "beispielprojekt"), refPass)
			if r.code() != tt.want || r.creds != nil {
				t.Fatalf("result = %v, %v; want %s", r.creds, r.err, tt.want)
			}
			if len(h.grants()) != 0 || len(h.eventsOf(audit.EventGrant)) != 0 {
				t.Errorf("a grant was created")
			}
			if h.be.Reads() != 0 {
				t.Errorf("reads = %d, want 0", h.be.Reads())
			}
			last := h.events()[len(h.events())-1]
			if last.Result.Code != tt.want {
				t.Errorf("last audit event = %+v", last)
			}
		})
	}
}

// T5, UC3, D8: a request outside the grant is a deviation. The approval
// request names the existing grant and lists only the new entry. The new
// approval creates a further grant; the existing grant does not change.
func TestDeviationCreatesFurtherGrant(t *testing.T) {
	h := newHarness(t)
	s := sess("s-1", "beispielprojekt")
	h.rec.Enqueue(fakeui.ReplyApproveAll(), fakeui.ReplyApproveAll(), fakeui.ReplyApproveAll())
	if r := h.approveFlow(s, refUser); r.err != nil {
		t.Fatalf("first: %v", r.err)
	}
	first := h.grants()
	h.clk.Advance(time.Minute)

	// A new item in the same vault.
	if r := h.approveFlow(s, refUser, refPass); r.err != nil || len(values(r.creds)) != 2 {
		t.Fatalf("second: %v, %v", r.creds, r.err)
	}
	req := h.rec.Requests()[1]
	if req.ExistingGrant == nil || req.ExistingGrant.GrantID != first[0].GrantID || req.ExistingGrant.ItemCount != 1 {
		t.Fatalf("existing grant = %+v", req.ExistingGrant)
	}
	if len(req.Credentials) != 1 || req.Credentials[0].Field != "password" ||
		req.Credentials[0].Deviation != approval.DeviationNewItem {
		t.Fatalf("credentials = %+v", req.Credentials)
	}
	now := h.grants()
	if len(now) != 2 || now[0].GrantID != first[0].GrantID || !now[0].ExpiresAt.Equal(first[0].ExpiresAt) ||
		!slices.Equal(now[0].References, first[0].References) {
		t.Fatalf("grants = %+v; the first grant changed", now)
	}

	// A new vault.
	if r := h.approveFlow(s, refInfra); r.err != nil {
		t.Fatalf("third: %v", r.err)
	}
	req = h.rec.Requests()[2]
	if req.Credentials[0].Deviation != approval.DeviationNewVault || req.ExistingGrant.GrantID != now[1].GrantID {
		t.Errorf("third request = %+v", req)
	}
	// A first request has no deviation.
	if first := h.rec.Requests()[0]; first.ExistingGrant != nil || first.Credentials[0].Deviation != "" {
		t.Errorf("first request = %+v", first)
	}
}

// T23, UC4, D32: a grant covers exactly the selected entries. The other
// entries end with denied. A later request for such an entry is a
// deviation with only this entry.
func TestPartialSelection(t *testing.T) {
	h := newHarness(t)
	s := sess("s-1", "beispielprojekt")
	h.rec.Enqueue(fakeui.ReplyApprove(3600, "c1", "c3"), fakeui.ReplyApproveAll())
	r := h.approveFlow(s, refUser, refPass, refObj)
	if r.err != nil {
		t.Fatalf("error = %v", r.err)
	}
	vals, codes := values(r.creds), entryCodes(r.creds)
	if len(vals) != 2 || vals[refUser] == "" || vals[refObj] == "" || codes[refPass] != protocol.CodeDenied {
		t.Fatalf("values = %d, codes = %v", len(vals), codes)
	}
	g := h.grants()
	if len(g) != 1 || !slices.Equal(g[0].References, refs(t, refUser, refObj)) {
		t.Fatalf("grants = %+v", g)
	}
	if h.be.ReadsOf(ref(t, refPass)) != 0 {
		t.Errorf("the denied entry was read")
	}
	want := []audit.EventType{
		audit.EventRequest, audit.EventBundle, audit.EventDecision, audit.EventGrant,
		audit.EventDenial, audit.EventDelivery,
	}
	if got := h.types(); !slices.Equal(got, want) {
		t.Errorf("audit = %v, want %v", got, want)
	}

	if r := h.approveFlow(s, refPass); r.err != nil {
		t.Fatalf("later request: %v", r.err)
	}
	req := h.rec.Requests()[1]
	if len(req.Credentials) != 1 || req.Credentials[0].Field != "password" || req.ExistingGrant == nil {
		t.Errorf("later approval request = %+v", req)
	}
}

// T16, D33: a grant ends with the session, with the revocation and with a
// restart. After each case the same request needs a new approval, and the
// audit log names the reason.
func TestGrantEnds(t *testing.T) {
	tests := []struct {
		name   string
		end    func(h *harness, grantID string)
		detail string
		event  audit.EventType
	}{
		{"session ended", func(h *harness, _ string) {
			h.b.SessionEnded(sess("s-1", "beispielprojekt"))
		}, audit.DetailSessionEnded, audit.EventExpiry},
		{"revoked", func(h *harness, id string) {
			res, e := h.b.GrantRevoke(context.Background(), &protocol.GrantRevoke{ID: "rv", GrantID: id})
			if e != nil || res.GrantID != id {
				h.t.Fatalf("revoke: %v, %v", res, e)
			}
		}, "the human revoked the grant", audit.EventRevocation},
		{"restart", func(h *harness, _ string) { h.restart() }, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			s := sess("s-1", "beispielprojekt")
			h.rec.Enqueue(fakeui.ReplyApproveAll(), fakeui.ReplyApproveAll())
			if r := h.approveFlow(s, refPass); r.err != nil {
				t.Fatalf("first: %v", r.err)
			}
			id := h.grants()[0].GrantID
			tt.end(h, id)
			if g := h.grants(); len(g) != 0 {
				t.Fatalf("grants after the end = %+v", g)
			}
			if r := h.approveFlow(s, refPass); r.err != nil {
				t.Fatalf("after the end: %v", r.err)
			}
			if n := len(h.rec.Requests()); n != 2 {
				t.Errorf("approval requests = %d, want 2", n)
			}
			if tt.event == "" {
				return
			}
			ev := h.eventsOf(tt.event)
			if len(ev) != 1 || ev[0].Result.GrantID != id || ev[0].Result.Detail != tt.detail {
				t.Errorf("end events = %+v", ev)
			}
		})
	}
}

// T16: a revocation of an unknown grant is not_found.
func TestRevokeUnknownGrant(t *testing.T) {
	h := newHarness(t)
	_, e := h.b.GrantRevoke(context.Background(), &protocol.GrantRevoke{ID: "rv", GrantID: "grant-unbekannt"})
	if e == nil || e.Code != protocol.CodeNotFound {
		t.Fatalf("error = %v, want not_found", e)
	}
}

// T25: grant_status shows the grants of the own session with the
// remaining lifetime, never a grant of another session and never a value.
// After the expiry and after the revocation it does not show the grant.
func TestGrantStatusOwnSessionOnly(t *testing.T) {
	h := newHarness(t, withLifetime("1h"))
	a, b := sess("s-a", "beispielprojekt"), sess("s-b", "beispielprojekt")
	h.rec.Enqueue(fakeui.ReplyApproveAll(), fakeui.ReplyApproveAll())
	if r := h.approveFlow(a, refPass); r.err != nil {
		t.Fatal(r.err)
	}
	if r := h.approveFlow(b, refUser); r.err != nil {
		t.Fatal(r.err)
	}
	h.clk.Advance(10 * time.Minute)
	status := func(s string) []protocol.Grant {
		g, e := h.b.GrantStatus(context.Background(), sess(s, "beispielprojekt"), &protocol.GrantStatus{ID: "gs"})
		if e != nil {
			t.Fatalf("status: %v", e)
		}
		out, _ := json.Marshal(g)
		if strings.Contains(string(out), marker) || strings.Contains(string(out), "beispiel-wert") {
			t.Fatalf("grant status shows a value")
		}
		return g.Grants
	}
	ga := status("s-a")
	if len(ga) != 1 || !slices.Equal(ga[0].References, refs(t, refPass)) {
		t.Fatalf("grants of a = %+v", ga)
	}
	if ga[0].SessionID != "" || ga[0].SessionName != "" || ga[0].Project != "" {
		t.Errorf("agent socket shows session fields: %+v", ga[0])
	}
	if ga[0].RemainingSeconds != 3000-3 {
		// The window of the second session took 3 s.
		t.Errorf("remaining = %d", ga[0].RemainingSeconds)
	}
	if gb := status("s-b"); len(gb) != 1 || gb[0].GrantID == ga[0].GrantID {
		t.Errorf("grants of b = %+v", gb)
	}
	all := h.grants()
	if len(all) != 2 || all[0].SessionID != "s-a" || all[0].SessionName != "beispiel-koordinator" || all[0].Project != "beispielprojekt" {
		t.Errorf("grants list = %+v", all)
	}
	if _, e := h.b.GrantRevoke(context.Background(), &protocol.GrantRevoke{ID: "rv", GrantID: ga[0].GrantID}); e != nil {
		t.Fatal(e)
	}
	if ga := status("s-a"); len(ga) != 0 {
		t.Errorf("after revocation = %+v", ga)
	}
	h.clk.Advance(time.Hour)
	if gb := status("s-b"); len(gb) != 0 {
		t.Errorf("after expiry = %+v", gb)
	}
	if st, _ := h.b.Status(context.Background(), &protocol.Status{ID: "st"}); st.Grants != 0 || st.Fault != nil {
		t.Errorf("status = %+v", st)
	}
}

// T11: a grant of another session does not cover a request.
func TestGrantOfOtherSessionDoesNotCover(t *testing.T) {
	h := newHarness(t)
	h.rec.Enqueue(fakeui.ReplyApproveAll(), fakeui.ReplyDeny())
	if r := h.approveFlow(sess("s-a", "beispielprojekt"), refPass); r.err != nil {
		t.Fatal(r.err)
	}
	r := h.approveFlow(sess("s-b", "beispielprojekt"), refPass)
	if r.code() != protocol.CodeDenied {
		t.Fatalf("result = %v, want denied", r.err)
	}
	if n := len(h.rec.Requests()); n != 2 {
		t.Errorf("approval requests = %d, want 2", n)
	}
}

// T4, D36: Run writes the expiry of a grant without a further request.
func TestRunWritesExpiry(t *testing.T) {
	h := newHarness(t, withLifetime("1m"))
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	if r := h.approveFlow(sess("s-1", "beispielprojekt"), refPass); r.err != nil {
		t.Fatal(r.err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { h.b.Run(ctx); close(done) }()
	for range 2 {
		h.waitTimers(1)
		h.clk.Advance(SweepInterval)
	}
	h.waitFor("the expiry event", func() bool { return len(h.eventsOf(audit.EventExpiry)) == 1 })
	cancel()
	<-done
}
