package broker

import (
	"context"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/approval/fakeui"
	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/protocol"
)

// credentialFields returns "item/field" of each entry of an approval
// request.
func credentialFields(req approval.Request) []string {
	var out []string
	for _, c := range req.Credentials {
		out = append(out, c.Title+"/"+c.Field)
	}
	return out
}

// T2: three requests of one session in the window, 200 ms apart, give
// exactly one approval request with three entries. A fourth request after
// the window gives a second approval request.
func TestBundleCollectsRequestsOfTheWindow(t *testing.T) {
	h := newHarness(t)
	s := sess("s-1", "beispielprojekt")
	h.rec.Enqueue(fakeui.ReplyApproveAll(), fakeui.ReplyApproveAll())
	ctx := context.Background()

	ch1 := h.async(ctx, s, newRequest(t, refUser))
	h.waitTimers(1)
	h.waitOpen(s, 1)
	h.clk.Advance(200 * time.Millisecond)
	ch2 := h.async(ctx, s, newRequest(t, refPass))
	h.waitOpen(s, 2)
	h.clk.Advance(200 * time.Millisecond)
	ch3 := h.async(ctx, s, newRequest(t, refObj))
	h.waitOpen(s, 3)
	h.clk.Advance(h.cfg.Bundle.Window - 400*time.Millisecond)
	for i, ch := range []<-chan result{ch1, ch2, ch3} {
		if r := h.wait(ch); r.err != nil || len(values(r.creds)) != 1 {
			t.Fatalf("request %d: %v, %v", i+1, r.creds, r.err)
		}
	}
	reqs := h.rec.Requests()
	if len(reqs) != 1 || len(reqs[0].Credentials) != 3 {
		t.Fatalf("approval requests = %d with %v; want 1 with 3 entries", len(reqs), credentialFields(reqs[0]))
	}

	// The fourth request after the window opens a new bundle.
	r := h.approveFlow(s, refObjPass)
	if r.err != nil {
		t.Fatalf("fourth request: %v", r.err)
	}
	if n := len(h.rec.Requests()); n != 2 {
		t.Fatalf("approval requests = %d, want 2", n)
	}
	if n := len(h.eventsOf(audit.EventBundle)); n != 2 {
		t.Errorf("bundle events = %d, want 2", n)
	}
}

// T2: a request shortly before the end of the window joins the bundle; a
// request at the end opens a new bundle. A late request does not extend
// the window.
func TestBundleWindowEdges(t *testing.T) {
	h := newHarness(t)
	s := sess("s-1", "beispielprojekt")
	h.rec.Enqueue(fakeui.ReplyApproveAll(), fakeui.ReplyApproveAll())
	ctx := context.Background()

	ch1 := h.async(ctx, s, newRequest(t, refUser))
	h.waitTimers(1)
	h.waitOpen(s, 1)
	h.clk.Advance(h.cfg.Bundle.Window - time.Millisecond)
	ch2 := h.async(ctx, s, newRequest(t, refPass))
	h.waitOpen(s, 2)
	// One millisecond more ends the window: the late request did not
	// extend it.
	h.clk.Advance(time.Millisecond)
	h.wait(ch1)
	h.wait(ch2)
	reqs := h.rec.Requests()
	if len(reqs) != 1 || len(reqs[0].Credentials) != 2 {
		t.Fatalf("approval requests = %d; want 1 with 2 entries", len(reqs))
	}

	// A request exactly at the end of a window opens a new window.
	ch3 := h.async(ctx, s, newRequest(t, refObj))
	h.waitTimers(1)
	h.waitOpen(s, 1)
	h.clk.Advance(h.cfg.Bundle.Window - time.Millisecond)
	if n := len(h.rec.Requests()); n != 1 {
		t.Fatalf("approval before the end of the window")
	}
	ch4 := h.async(ctx, s, newRequest(t, refObjPass))
	h.waitOpen(s, 2)
	h.clk.Advance(time.Millisecond)
	h.wait(ch3)
	h.wait(ch4)
	if n := len(h.rec.Requests()); n != 2 {
		t.Fatalf("approval requests = %d, want 2", n)
	}
}

// T2: a request at the end of the window does not join the closing
// bundle.
func TestBundleRequestAtDeadlineOpensNewBundle(t *testing.T) {
	h := newHarness(t)
	s := sess("s-1", "beispielprojekt")
	release := make(chan struct{})
	h.rec.Enqueue(func(ctx context.Context, req approval.Request) (approval.Decision, error) {
		<-release
		return fakeui.ReplyApproveAll()(ctx, req)
	}, fakeui.ReplyApproveAll())
	ctx := context.Background()
	ch1 := h.async(ctx, s, newRequest(t, refUser))
	h.waitTimers(1)
	h.clk.Advance(h.cfg.Bundle.Window)
	// The first bundle is at the UI now.
	h.waitFor("first approval", func() bool { return len(h.rec.Requests()) == 1 })
	ch2 := h.async(ctx, s, newRequest(t, refPass))
	h.waitOpen(s, 1)
	close(release)
	h.wait(ch1)
	h.clk.Advance(h.cfg.Bundle.Window)
	h.wait(ch2)
	reqs := h.rec.Requests()
	if len(reqs) != 2 || len(reqs[1].Credentials) != 1 || reqs[1].Credentials[0].Field != "password" {
		t.Fatalf("approval requests = %+v", reqs)
	}
}

// T2, UC2: a request that a grant covers opens no window and needs no
// approval request.
func TestBundleCoveredRequestOpensNoWindow(t *testing.T) {
	h := newHarness(t)
	s := sess("s-1", "beispielprojekt")
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	if r := h.approveFlow(s, refUser); r.err != nil {
		t.Fatalf("first: %v", r.err)
	}
	r := h.do(s, refUser)
	if r.err != nil || values(r.creds)[refUser] != "beispiel-wert-1" {
		t.Fatalf("covered: %v, %v", r.creds, r.err)
	}
	if h.clk.Pending() != 0 {
		t.Errorf("pending timers = %d, want 0", h.clk.Pending())
	}
	if n := len(h.rec.Requests()); n != 1 {
		t.Errorf("approval requests = %d, want 1", n)
	}
}

// T2, T19: two sessions in the same window give two bundles. The two
// approval requests run one after the other, never at the same time.
func TestBundleTwoSessionsTwoBundlesInSequence(t *testing.T) {
	h := newHarness(t)
	var open, maxOpen atomic.Int32
	reply := func(ctx context.Context, req approval.Request) (approval.Decision, error) {
		n := open.Add(1)
		for {
			m := maxOpen.Load()
			if n <= m || maxOpen.CompareAndSwap(m, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond) // give a second request the chance to overlap
		defer open.Add(-1)
		return fakeui.ReplyApproveAll()(ctx, req)
	}
	h.rec.Enqueue(reply, reply)
	ctx := context.Background()
	a, b := sess("s-a", "beispielprojekt"), sess("s-b", "beispielprojekt")
	chA := h.async(ctx, a, newRequest(t, refUser))
	chB := h.async(ctx, b, newRequest(t, refUser))
	h.waitTimers(2)
	h.clk.Advance(h.cfg.Bundle.Window)
	if r := h.wait(chA); r.err != nil {
		t.Fatalf("session a: %v", r.err)
	}
	if r := h.wait(chB); r.err != nil {
		t.Fatalf("session b: %v", r.err)
	}
	reqs := h.rec.Requests()
	if len(reqs) != 2 || reqs[0].Verified.SessionID == reqs[1].Verified.SessionID {
		t.Fatalf("approval requests = %d", len(reqs))
	}
	if maxOpen.Load() != 1 {
		t.Errorf("approval requests at the same time = %d, want 1", maxOpen.Load())
	}
}

// T19: a bundle above the maximum is cut. The first entries in the
// sequence of arrival stay; each further entry ends with bundle_too_large.
// Exactly the maximum is allowed. The maximum comes from the
// configuration.
func TestBundleMaximum(t *testing.T) {
	all := make([]string, 12)
	for i := range all {
		all[i] = fmt.Sprintf("op://beispiel.app.k8s-dev/dienst-dev/f%02d", i+1)
	}
	tests := []struct {
		name     string
		max      string
		refs     []string
		wantKept int
	}{
		{"default ten, twelve references", "10", all, 10},
		{"three, twelve references", "3", all, 3},
		{"exactly the maximum", "3", all[:3], 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, withMaxEntries(tt.max))
			h.rec.Enqueue(fakeui.ReplyApproveAll())
			r := h.approveFlow(sess("s-1", "beispielprojekt"), tt.refs...)
			if r.err != nil {
				t.Fatalf("error = %v", r.err)
			}
			reqs := h.rec.Requests()
			if len(reqs) != 1 || len(reqs[0].Credentials) != tt.wantKept {
				t.Fatalf("approval entries = %v, want %d", credentialFields(reqs[0]), tt.wantKept)
			}
			for i, c := range reqs[0].Credentials {
				if want := fmt.Sprintf("f%02d", i+1); c.Field != want {
					t.Errorf("entry %d = %s, want %s (sequence of arrival)", i, c.Field, want)
				}
			}
			codes := entryCodes(r.creds)
			vals := values(r.creds)
			for i, ref := range tt.refs {
				if i < tt.wantKept {
					if _, ok := vals[ref]; !ok {
						t.Errorf("%s: no value", ref)
					}
				} else if codes[ref] != protocol.CodeBundleTooLarge {
					t.Errorf("%s: code = %q, want bundle_too_large", ref, codes[ref])
				}
			}
			cut := len(tt.refs) - tt.wantKept
			denials := h.eventsOf(audit.EventDenial)
			if cut == 0 && len(denials) != 0 || cut > 0 && (len(denials) != 1 || len(denials[0].References) != cut ||
				denials[0].Result.Code != protocol.CodeBundleTooLarge) {
				t.Errorf("denials = %+v", denials)
			}
		})
	}
}

// T19: the maximum counts over the requests of the window. The requests
// that arrive first keep their place.
func TestBundleMaximumOverRequests(t *testing.T) {
	h := newHarness(t, withMaxEntries("2"))
	s := sess("s-1", "beispielprojekt")
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	ctx := context.Background()
	ch1 := h.async(ctx, s, newRequest(t, refUser, refPass))
	h.waitTimers(1)
	h.waitOpen(s, 2)
	// A full bundle rejects at once; the request does not wait.
	r2 := h.wait(h.async(ctx, s, newRequest(t, refObj)))
	if r2.code() != protocol.CodeBundleTooLarge {
		t.Fatalf("second request = %v, want bundle_too_large", r2.err)
	}
	// A reference that is already in the bundle uses no place.
	ch3 := h.async(ctx, s, newRequest(t, refPass))
	h.waitFor("the third request to join", func() bool { return h.b.openRequests(s.Facts.SessionID) == 2 })
	h.clk.Advance(h.cfg.Bundle.Window)
	if r := h.wait(ch1); r.err != nil || len(values(r.creds)) != 2 {
		t.Fatalf("first request: %v, %v", r.creds, r.err)
	}
	if r := h.wait(ch3); r.err != nil || values(r.creds)[refPass] == "" {
		t.Fatalf("third request: %v, %v", r.creds, r.err)
	}
	if reqs := h.rec.Requests(); len(reqs) != 1 || len(reqs[0].Credentials) != 2 {
		t.Fatalf("approval requests = %+v", reqs)
	}
	want := []audit.EventType{
		audit.EventRequest, audit.EventRequest, audit.EventDenial, audit.EventRequest,
		audit.EventBundle, audit.EventDecision, audit.EventGrant,
	}
	if got := h.types(); !slices.Equal(got[:len(want)], want) {
		t.Errorf("audit = %v", got)
	}
}
