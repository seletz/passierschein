package broker

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/approval/fakeui"
	"github.com/seletz/passierschein/internal/approval/queue"
	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/clock"
	"github.com/seletz/passierschein/internal/protocol"
)

// Many sessions at the same time: each session gets its own bundle, its own
// approval request and its own grant. Each delivered value has a delivery
// event of its session before the answer.
func TestConcurrentSessions(t *testing.T) {
	const n = 16
	h := newHarness(t, withUI(func(_ *fakeui.Recorder, clk *clock.Fake) approval.UI {
		return queue.New(fakeui.AutoApprove{}, clk, 0)
	}))
	chans := make([]<-chan result, n)
	for i := range n {
		chans[i] = h.async(context.Background(), sess(fmt.Sprintf("s-%02d", i), "beispielprojekt"), newRequest(t, refPass))
	}
	h.waitTimers(n)
	h.clk.Advance(h.cfg.Bundle.Window)
	for i, ch := range chans {
		if r := h.wait(ch); r.err != nil || values(r.creds)[refPass] != marker {
			t.Fatalf("session %d: %v", i, r.err)
		}
	}
	if g := h.grants(); len(g) != n {
		t.Fatalf("grants = %d, want %d", len(g), n)
	}
	deliveries := map[string]bool{}
	for _, e := range h.eventsOf(audit.EventDelivery) {
		deliveries[string(e.Verified.SessionID)] = true
	}
	if len(deliveries) != n {
		t.Errorf("sessions with a delivery event = %d, want %d", len(deliveries), n)
	}
}

// Many requests of one session in the window at the same time give one
// approval request.
func TestConcurrentRequestsInWindow(t *testing.T) {
	h := newHarness(t)
	s := sess("s-1", "beispielprojekt")
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	first := h.async(context.Background(), s, newRequest(t, "op://beispiel.app.k8s-dev/dienst-dev/f01"))
	h.waitTimers(1)
	chans := []<-chan result{first}
	for i := 2; i <= 8; i++ {
		chans = append(chans, h.async(context.Background(), s, newRequest(t, fmt.Sprintf("op://beispiel.app.k8s-dev/dienst-dev/f%02d", i))))
	}
	h.waitOpen(s, 8)
	h.clk.Advance(h.cfg.Bundle.Window)
	for _, ch := range chans {
		if r := h.wait(ch); r.err != nil {
			t.Fatal(r.err)
		}
	}
	if reqs := h.rec.Requests(); len(reqs) != 1 || len(reqs[0].Credentials) != 8 {
		t.Fatalf("approval requests = %d", len(reqs))
	}
}

// D33: the session ends while its approval request is open. The request
// ends, no grant is created and nothing is delivered, also if the UI
// approves after the end.
func TestSessionEndsDuringApproval(t *testing.T) {
	tests := []struct {
		name  string
		reply func(ended <-chan struct{}) fakeui.Reply
	}{
		{"UI waits", func(<-chan struct{}) fakeui.Reply { return fakeui.ReplyBlock() }},
		{"UI approves after the end", func(ended <-chan struct{}) fakeui.Reply {
			return func(_ context.Context, req approval.Request) (approval.Decision, error) {
				<-ended
				return fakeui.AutoApprove{}.Approve(context.Background(), req)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The UI without the queue, so that the UI can answer after
			// the end of the session.
			h := newHarness(t, withUI(func(rec *fakeui.Recorder, _ *clock.Fake) approval.UI { return rec }))
			s := sess("s-1", "beispielprojekt")
			ended := make(chan struct{})
			h.rec.Enqueue(tt.reply(ended))
			ctx, cancel := context.WithCancel(context.Background())
			ch := h.async(ctx, s, newRequest(t, refPass))
			h.waitTimers(1)
			h.clk.Advance(h.cfg.Bundle.Window)
			h.waitFor("the approval request", func() bool { return len(h.rec.Requests()) == 1 })
			// The socket layer ends the context and reports the end.
			cancel()
			h.b.SessionEnded(s)
			close(ended)
			r := h.wait(ch)
			if r.err == nil || r.creds != nil {
				t.Fatalf("result = %v, %v; want an error", r.creds, r.err)
			}
			h.waitFor("the expiry event", func() bool { return len(h.eventsOf(audit.EventExpiry)) == 1 })
			if len(h.grants()) != 0 || len(h.eventsOf(audit.EventGrant)) != 0 || h.be.Reads() != 0 {
				t.Errorf("grant or read after the end of the session")
			}
			if e := h.eventsOf(audit.EventExpiry)[0]; e.Result.Detail != audit.DetailSessionEnded {
				t.Errorf("expiry = %+v", e)
			}
		})
	}
}

// D33, D36: a revocation during deliveries. Each delivery event comes
// before the revocation event, and each answer with a value has its
// delivery event. After the revocation no request of the session gets a
// value without a new approval.
func TestRevokeDuringDelivery(t *testing.T) {
	h := newHarness(t)
	s := sess("s-1", "beispielprojekt")
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	if r := h.approveFlow(s, refPass); r.err != nil {
		t.Fatal(r.err)
	}
	id := h.grants()[0].GrantID
	const n = 40
	var wg sync.WaitGroup
	results := make(chan result, n)
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, e := h.b.RequestCredentials(context.Background(), s, newRequest(t, refPass))
			results <- result{c, e}
		}()
	}
	if _, e := h.b.GrantRevoke(context.Background(), &protocol.GrantRevoke{ID: "rv", GrantID: id}); e != nil {
		t.Fatal(e)
	}
	// Requests after the revocation open a bundle; end each window. The
	// recorder denies when it has no reply.
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	for {
		select {
		case <-done:
		default:
			if h.clk.Pending() > 0 {
				h.clk.Advance(h.cfg.Bundle.Window)
			}
			continue
		}
		break
	}
	close(results)
	delivered := 0
	for r := range results {
		if v := values(r.creds)[refPass]; v != "" {
			delivered++
		} else if c := r.code(); c != protocol.CodeDenied && c != protocol.CodeTimeout {
			// The loop above moves the clock, so an approval request can
			// also time out.
			t.Errorf("result without value has code %q", r.code())
		}
	}
	revSeq, deliveries := uint64(0), 0
	for _, e := range h.events() {
		switch e.Type {
		case audit.EventRevocation:
			revSeq = e.Seq
		case audit.EventDelivery:
			if revSeq != 0 {
				t.Errorf("delivery event %d after the revocation", e.Seq)
			}
			deliveries++
		}
	}
	// The first delivery event belongs to the approval.
	if deliveries-1 != delivered {
		t.Errorf("delivery events = %d, delivered answers = %d", deliveries-1, delivered)
	}
	if r := h.approveFlow(s, refPass); r.code() != protocol.CodeDenied {
		t.Errorf("after the revocation: %v", r.err)
	}
}
