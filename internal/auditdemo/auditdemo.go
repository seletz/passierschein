// Package auditdemo writes a synthetic audit log for tests, demos and
// screenshots of "a38 tui". It is not part of the release: only tests and
// the development program in the sub-directory gen use it.
//
// The log is realistic but neutral: two projects, four session names,
// references of the vaults beispiel.app.k8s-dev and demo.app.k8s-dev,
// approvals, denials, timeouts, deviations, policy denials, a revocation,
// a failure of the approval UI, a backend error and one fault with the
// begin and the end of the global lock. It holds no secret value: an audit
// event has no field for it.
//
// Generate is deterministic: the same options give the same events.
package auditdemo

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/clock"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// Options configure Generate.
type Options struct {
	// End is the time of the newest possible event. The last day of the
	// log is the day of End; it has events only before End.
	End time.Time
	// Days is the number of calendar days, with the day of End.
	Days int
	// Location gives the calendar days. Nil means UTC.
	Location *time.Location
	// Seed makes a different but still deterministic log.
	Seed uint64
}

// project is one project of the demo.
type project struct {
	name     string
	sessions []string
	refs     []protocol.Reference
	outside  protocol.Reference
}

func ref(s string) protocol.Reference {
	r, err := protocol.ParseReference(s)
	if err != nil {
		panic(err)
	}
	return r
}

var projects = []project{
	{
		name:     "beispielprojekt",
		sessions: []string{"beispiel-koordinator", "beispiel-tests"},
		refs: []protocol.Reference{
			ref("op://beispiel.app.k8s-dev/datenbank-dev/password"),
			ref("op://beispiel.app.k8s-dev/datenbank-dev/username"),
			ref("op://beispiel.app.k8s-dev/api-token-dev/credential"),
			ref("op://beispiel.app.k8s-dev/registry-dev/password"),
			ref("op://beispiel.app.k8s-dev/smtp-dev/password"),
		},
		outside: ref("op://beispiel.app.k8s-test/datenbank-test/password"),
	},
	{
		name:     "demo-projekt",
		sessions: []string{"demo-review", "demo-migration"},
		refs: []protocol.Reference{
			ref("op://demo.app.k8s-dev/datenbank-dev/password"),
			ref("op://demo.app.k8s-dev/api-token-dev/credential"),
			ref("op://demo.app.k8s-dev/storage-dev/secret-key"),
		},
		outside: ref("op://demo.app.k8s-test/storage-test/secret-key"),
	},
}

// gen holds the state of one run.
type gen struct {
	ctx      context.Context
	store    audit.Store
	clk      *clock.Fake
	rnd      *rand.Rand
	end      time.Time
	sessions int
	requests int
	grants   int
	err      error
}

// liveGrant is a grant of the current session.
type liveGrant struct {
	id      string
	refs    []protocol.Reference
	expires time.Time
}

// demoSession is one session while it runs.
type demoSession struct {
	facts  session.Facts
	name   string
	proj   project
	grants []liveGrant
}

// Generate writes the events through store. clk must be the clock of the
// store; Generate moves it forward to the time of each event. The clock
// must not be after the start of the first day.
func Generate(ctx context.Context, store audit.Store, clk *clock.Fake, o Options) error {
	if o.Days < 1 {
		return fmt.Errorf("auditdemo: days must be at least 1")
	}
	loc := o.Location
	if loc == nil {
		loc = time.UTC
	}
	g := &gen{ctx: ctx, store: store, clk: clk, rnd: rand.New(rand.NewPCG(o.Seed, 71)), end: o.End}
	e := o.End.In(loc)
	lastDay := time.Date(e.Year(), e.Month(), e.Day(), 0, 0, 0, 0, loc)
	for d := 0; d < o.Days; d++ {
		day := lastDay.AddDate(0, 0, d-(o.Days-1))
		g.day(day, o.Days-1-d)
		if g.err != nil {
			return g.err
		}
	}
	return g.err
}

// at moves the clock to t. It reports false if t is after the end.
func (g *gen) at(t time.Time) bool {
	if t.After(g.end) {
		return false
	}
	if d := t.Sub(g.clk.Now()); d > 0 {
		g.clk.Advance(d)
	}
	return true
}

// specialDays are the days, counted back from the last day, whose first
// session has a special case.
var specialDays = map[int]bool{3: true, 5: true, 7: true, 12: true, 16: true}

// day writes the sessions of one day. back is the number of days before
// the last day; the special cases use it.
func (g *gen) day(day time.Time, back int) {
	t := day.Add(8*time.Hour + time.Duration(g.rnd.IntN(90))*time.Minute)
	if back == 9 {
		g.fault(day.Add(7 * time.Hour))
	}
	n := 1 + g.rnd.IntN(3)
	if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		n = g.rnd.IntN(2)
		if specialDays[back] {
			// The special cases must come on each weekday of the end.
			n = 1
		}
	}
	for i := 0; i < n && g.err == nil; i++ {
		p := projects[g.rnd.IntN(len(projects))]
		name := p.sessions[g.rnd.IntN(len(p.sessions))]
		if back == 0 && i == 0 {
			p, name = projects[0], "beispiel-koordinator"
		}
		end, ok := g.session(t, p, name, back, i)
		if !ok {
			return
		}
		t = end.Add(time.Duration(10+g.rnd.IntN(60)) * time.Minute)
	}
}

// session writes one session from start and returns its end.
func (g *gen) session(start time.Time, p project, name string, back, index int) (time.Time, bool) {
	if !g.at(start) {
		return start, false
	}
	g.sessions++
	s := &demoSession{
		facts: session.Facts{
			SessionID: session.ID(fmt.Sprintf("s-%04d", g.sessions)),
			Origin:    session.OriginLocal,
			Project:   p.name,
			PeerPID:   40000 + g.rnd.IntN(20000),
			PeerUID:   501,
			Process:   "/usr/local/bin/coding-agent",
			Cwd:       "/home/beispiel/develop/" + p.name,
		},
		name: name,
		proj: p,
	}
	t := start
	// The first request opens a popup.
	first := g.pick(p.refs, 1+g.rnd.IntN(3))
	switch {
	case back == 3 && index == 0:
		g.popup(s, t, first, "partial")
	case back == 12 && index == 0:
		g.popup(s, t, first, "ui_failed")
	case back == 16 && index == 0:
		g.popup(s, t, first, "abandoned")
		return t.Add(time.Minute), true
	default:
		g.popup(s, t, first, g.outcome())
	}
	steps := 3 + g.rnd.IntN(8)
	for i := 0; i < steps && g.err == nil; i++ {
		t = t.Add(time.Duration(5+g.rnd.IntN(25)) * time.Minute)
		if !g.at(t) {
			return t, false
		}
		g.expire(s, t)
		switch r := g.rnd.IntN(100); {
		case back == 5 && index == 0 && i == 2 && len(s.grants) > 0:
			gr := s.grants[0]
			g.append(s, audit.EventRevocation, gr.refs, audit.Result{GrantID: gr.id, Detail: "the human revoked the grant"})
			s.grants = s.grants[1:]
		case back == 7 && index == 0 && i == 1:
			refs := g.pick(p.refs, 1)
			g.request(s, refs)
			g.append(s, audit.EventError, refs, audit.Result{Code: protocol.CodeBackendUnavailable, Detail: "the backend failed: backend_unavailable"})
		case r < 8:
			// A request outside the policy of the project.
			g.request(s, []protocol.Reference{p.outside})
			g.append(s, audit.EventDenial, []protocol.Reference{p.outside}, audit.Result{Code: protocol.CodeNotAllowed, Detail: "the policy does not allow the credential"})
		case (r < 26 || len(s.grants) == 0) && len(uncovered(s, p.refs)) > 0:
			// A request beyond the grants: a new popup.
			refs := g.pick(uncovered(s, p.refs), 1+g.rnd.IntN(2))
			g.popup(s, t, refs, g.outcome())
		case len(s.grants) == 0:
			// Nothing to do in this step.
		default:
			// A request that a grant covers.
			gr := s.grants[g.rnd.IntN(len(s.grants))]
			refs := g.pick(gr.refs, 1+g.rnd.IntN(len(gr.refs)))
			g.request(s, refs)
			g.append(s, audit.EventDelivery, refs, audit.Result{GrantID: gr.id})
		}
	}
	t = t.Add(time.Duration(5+g.rnd.IntN(20)) * time.Minute)
	if !g.at(t) {
		return t, false
	}
	g.expire(s, t)
	for _, gr := range s.grants {
		g.append(s, audit.EventExpiry, gr.refs, audit.Result{GrantID: gr.id, Detail: audit.DetailSessionEnded})
	}
	return t, true
}

// outcome draws the outcome of a popup.
func (g *gen) outcome() string {
	switch r := g.rnd.IntN(100); {
	case r < 72:
		return "approved"
	case r < 86:
		return "denied"
	case r < 94:
		return "timeout"
	}
	return "partial"
}

// expire ends the grants of s whose lifetime ended before t.
func (g *gen) expire(s *demoSession, t time.Time) {
	var live []liveGrant
	for _, gr := range s.grants {
		if gr.expires.After(t) {
			live = append(live, gr)
			continue
		}
		g.append(s, audit.EventExpiry, gr.refs, audit.Result{GrantID: gr.id, Detail: audit.DetailLifetimeEnded})
	}
	s.grants = live
}

// popup writes a request with a bundle and the outcome.
func (g *gen) popup(s *demoSession, t time.Time, refs []protocol.Reference, outcome string) {
	if outcome == "partial" && len(refs) < 2 {
		refs = append(refs, g.other(s.proj.refs, refs[0]))
	}
	g.requests++
	rid := fmt.Sprintf("req-%05d", g.requests)
	g.request(s, refs)
	g.at(t.Add(3 * time.Second))
	g.append(s, audit.EventBundle, refs, audit.Result{RequestID: rid})
	wait := time.Duration(4+g.rnd.IntN(20)) * time.Second
	switch outcome {
	case "denied":
		g.at(t.Add(wait))
		g.append(s, audit.EventDenial, refs, audit.Result{Code: protocol.CodeDenied, RequestID: rid, Detail: "the human denied the request"})
	case "timeout":
		g.at(t.Add(63 * time.Second))
		g.append(s, audit.EventExpiry, refs, audit.Result{Code: protocol.CodeTimeout, RequestID: rid, Detail: "the approval request expired without a decision"})
	case "ui_failed":
		g.at(t.Add(wait))
		g.append(s, audit.EventError, refs, audit.Result{Code: protocol.CodeUIFailed, RequestID: rid, Detail: "the approval UI failed"})
	case "abandoned":
		g.at(t.Add(wait))
		g.append(s, audit.EventExpiry, refs, audit.Result{RequestID: rid, Detail: audit.DetailSessionEnded})
	default:
		approved, denied := refs, []protocol.Reference(nil)
		if outcome == "partial" {
			approved, denied = refs[:1], refs[1:]
		}
		g.at(t.Add(wait))
		g.append(s, audit.EventDecision, approved, audit.Result{RequestID: rid, LifetimeSeconds: 14400, Detail: "approved"})
		g.grants++
		gid := fmt.Sprintf("grant-%05d", g.grants)
		g.append(s, audit.EventGrant, approved, audit.Result{RequestID: rid, GrantID: gid, LifetimeSeconds: 14400})
		if len(denied) > 0 {
			g.append(s, audit.EventDenial, denied, audit.Result{Code: protocol.CodeDenied, RequestID: rid, Detail: "the human did not select the entries"})
		}
		g.append(s, audit.EventDelivery, approved, audit.Result{GrantID: gid})
		s.grants = append(s.grants, liveGrant{id: gid, refs: approved, expires: g.clk.Now().Add(4 * time.Hour)})
	}
}

// fault writes a fault: the begin of the lock, a request in the lock and
// the end of the lock.
func (g *gen) fault(t time.Time) {
	if !g.at(t) {
		return
	}
	g.append(nil, audit.EventError, nil, audit.Result{Code: protocol.CodeAuditFailed, Detail: audit.DetailFaultBegin})
	g.sessions++
	s := &demoSession{
		facts: session.Facts{SessionID: session.ID(fmt.Sprintf("s-%04d", g.sessions)), Origin: session.OriginLocal,
			Project: projects[0].name, PeerUID: 501, Process: "/usr/local/bin/coding-agent", Cwd: "/home/beispiel/develop/" + projects[0].name},
		name: "beispiel-koordinator", proj: projects[0],
	}
	g.at(t.Add(4 * time.Minute))
	g.append(s, audit.EventError, projects[0].refs[:1], audit.Result{Code: protocol.CodeDaemonFaulted, Detail: "delivery is locked"})
	g.at(t.Add(17 * time.Minute))
	g.append(nil, audit.EventError, nil, audit.Result{Code: protocol.CodeAuditFailed, Detail: audit.DetailFaultEnd})
}

// request writes the request event.
func (g *gen) request(s *demoSession, refs []protocol.Reference) {
	g.append(s, audit.EventRequest, refs, audit.Result{})
}

// append writes one event of the session s; nil means no session.
func (g *gen) append(s *demoSession, t audit.EventType, refs []protocol.Reference, r audit.Result) {
	if g.err != nil {
		return
	}
	ev := audit.Event{Type: t, References: refs, Result: r}
	if s != nil {
		ev.Verified = s.facts
		ev.Claims = session.Claims{SessionName: s.name, Agent: "coding-agent"}
		if t == audit.EventRequest || t == audit.EventBundle {
			ev.Claims.Task = "Run the integration tests of " + s.proj.name
			ev.Claims.Reason = "The tests need the login for the development database."
		}
	}
	_, g.err = g.store.Append(g.ctx, ev)
}

// pick returns n different references of refs in their sequence.
func (g *gen) pick(refs []protocol.Reference, n int) []protocol.Reference {
	if n > len(refs) {
		n = len(refs)
	}
	idx := g.rnd.Perm(len(refs))[:n]
	var out []protocol.Reference
	for i, r := range refs {
		for _, j := range idx {
			if i == j {
				out = append(out, r)
			}
		}
	}
	return out
}

// other returns a reference of refs that is not r.
func (g *gen) other(refs []protocol.Reference, r protocol.Reference) protocol.Reference {
	for _, o := range refs {
		if o != r {
			return o
		}
	}
	return r
}

// uncovered returns the references of refs that no live grant of s covers.
func uncovered(s *demoSession, refs []protocol.Reference) []protocol.Reference {
	var out []protocol.Reference
	for _, r := range refs {
		covered := false
		for _, gr := range s.grants {
			for _, c := range gr.refs {
				if c == r {
					covered = true
				}
			}
		}
		if !covered {
			out = append(out, r)
		}
	}
	return out
}
