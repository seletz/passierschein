package auditstats

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// base is the time of the first synthetic entry: a Tuesday, 09:00 UTC.
var base = time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)

var (
	refDB   = protocol.Reference{Vault: "beispiel.app.k8s-dev", Item: "datenbank-dev", Field: "password"}
	refUser = protocol.Reference{Vault: "beispiel.app.k8s-dev", Item: "datenbank-dev", Field: "username"}
	refAPI  = protocol.Reference{Vault: "beispiel.app.k8s-dev", Item: "api-token-dev", Field: "credential"}
)

// log builds a synthetic audit log. Each call of add appends one entry
// with the next number at base plus the offset.
type log struct{ entries []audit.Entry }

type sess struct {
	id, name, project string
}

var (
	koord = sess{"s-1", "beispiel-koordinator", "beispielprojekt"}
	tests = sess{"s-2", "beispiel-tests", "demo-projekt"}
)

func (l *log) add(offset time.Duration, s sess, t audit.EventType, refs []protocol.Reference, r audit.Result) *log {
	e := audit.Entry{
		Seq:  uint64(len(l.entries) + 1),
		Time: base.Add(offset),
		Event: audit.Event{
			Type:       t,
			Verified:   session.Facts{SessionID: session.ID(s.id), Origin: session.OriginLocal, Project: s.project},
			Claims:     session.Claims{SessionName: s.name, Agent: "coding-agent"},
			References: refs,
			Result:     r,
		},
	}
	l.entries = append(l.entries, e)
	return l
}

// approved adds a request with a popup that the human approves: request,
// bundle, decision, grant, delivery.
func (l *log) approved(offset time.Duration, s sess, req, grant string, refs ...protocol.Reference) *log {
	l.add(offset, s, audit.EventRequest, refs, audit.Result{})
	l.add(offset+3*time.Second, s, audit.EventBundle, refs, audit.Result{RequestID: req})
	l.add(offset+10*time.Second, s, audit.EventDecision, refs, audit.Result{RequestID: req, LifetimeSeconds: 14400, Detail: "approved"})
	l.add(offset+10*time.Second, s, audit.EventGrant, refs, audit.Result{RequestID: req, GrantID: grant, LifetimeSeconds: 14400})
	return l.add(offset+10*time.Second, s, audit.EventDelivery, refs, audit.Result{GrantID: grant})
}

// covered adds a request that a grant covers: request, delivery.
func (l *log) covered(offset time.Duration, s sess, grant string, refs ...protocol.Reference) *log {
	l.add(offset, s, audit.EventRequest, refs, audit.Result{})
	return l.add(offset, s, audit.EventDelivery, refs, audit.Result{GrantID: grant})
}

// failed adds a request with a popup that ends with the event t and the
// code.
func (l *log) failed(offset time.Duration, s sess, req string, t audit.EventType, code protocol.Code, refs ...protocol.Reference) *log {
	l.add(offset, s, audit.EventRequest, refs, audit.Result{})
	l.add(offset+3*time.Second, s, audit.EventBundle, refs, audit.Result{RequestID: req})
	return l.add(offset+30*time.Second, s, t, refs, audit.Result{Code: code, RequestID: req})
}

func TestComputeOutcomes(t *testing.T) {
	// G12 (proposal): popups, their outcomes and the share of denials.
	l := (&log{}).
		approved(0, koord, "r1", "g1", refDB).
		failed(time.Hour, tests, "r2", audit.EventDenial, protocol.CodeDenied, refAPI).
		failed(2*time.Hour, tests, "r3", audit.EventExpiry, protocol.CodeTimeout, refAPI).
		failed(3*time.Hour, tests, "r4", audit.EventError, protocol.CodeUIFailed, refAPI)
	// A partial selection: decision for one entry, denial for the other.
	l.add(4*time.Hour, tests, audit.EventRequest, []protocol.Reference{refDB, refUser}, audit.Result{})
	l.add(4*time.Hour, tests, audit.EventBundle, []protocol.Reference{refDB, refUser}, audit.Result{RequestID: "r5"})
	l.add(4*time.Hour, tests, audit.EventDecision, []protocol.Reference{refDB}, audit.Result{RequestID: "r5", Detail: "approved"})
	l.add(4*time.Hour, tests, audit.EventGrant, []protocol.Reference{refDB}, audit.Result{RequestID: "r5", GrantID: "g2", LifetimeSeconds: 3600})
	l.add(4*time.Hour, tests, audit.EventDenial, []protocol.Reference{refUser}, audit.Result{Code: protocol.CodeDenied, RequestID: "r5"})
	l.add(4*time.Hour, tests, audit.EventDelivery, []protocol.Reference{refDB}, audit.Result{GrantID: "g2"})
	// The session ends while a popup is open: not a popup that counts.
	l.failed(5*time.Hour, koord, "r6", audit.EventExpiry, "", refAPI)
	l.entries[len(l.entries)-1].Result.Detail = audit.DetailSessionEnded
	// A policy denial: no popup.
	l.add(6*time.Hour, koord, audit.EventRequest, []protocol.Reference{refAPI}, audit.Result{})
	l.add(6*time.Hour, koord, audit.EventDenial, []protocol.Reference{refAPI}, audit.Result{Code: protocol.CodeNotAllowed})

	st := Compute(l.entries, Scope{}, time.UTC, 5)
	want := map[string]int{
		"Requests": 7, "Popups": 5, "Approved": 2, "Partial": 1, "Denied": 1, "TimedOut": 1, "Failed": 1,
		"Abandoned": 1, "PolicyDenials": 1, "Grants": 2, "Deliveries": 2, "DeliveriesAfterPopup": 2,
		"DeliveriesUnderGrant": 0, "Deviations": 0, "BundleMax": 2,
	}
	got := map[string]int{
		"Requests": st.Requests, "Popups": st.Popups, "Approved": st.Approved, "Partial": st.Partial,
		"Denied": st.Denied, "TimedOut": st.TimedOut, "Failed": st.Failed, "Abandoned": st.Abandoned,
		"PolicyDenials": st.PolicyDenials, "Grants": st.Grants, "Deliveries": st.Deliveries,
		"DeliveriesAfterPopup": st.DeliveriesAfterPopup, "DeliveriesUnderGrant": st.DeliveriesUnderGrant,
		"Deviations": st.Deviations, "BundleMax": st.BundleMax,
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %d, want %d", k, got[k], w)
		}
	}
	if st.DenialShare != 0.2 {
		t.Errorf("DenialShare = %v, want 0.2", st.DenialShare)
	}
	// Popup sizes 1, 1, 1, 1, 2: the median is 1.
	if st.BundleMedian != 1 {
		t.Errorf("BundleMedian = %v, want 1", st.BundleMedian)
	}
}

func TestComputeDeliveriesAndDeviations(t *testing.T) {
	tt := []struct {
		name                         string
		build                        func() *log
		after, under, deviations, wp int
	}{
		{
			name: "covered requests after one approval",
			build: func() *log {
				return (&log{}).approved(0, koord, "r1", "g1", refDB).covered(time.Minute, koord, "g1", refDB).covered(2*time.Minute, koord, "g1", refDB)
			},
			after: 1, under: 2, wp: 1,
		},
		{
			name: "a new item while a grant lives is a deviation",
			build: func() *log {
				return (&log{}).approved(0, koord, "r1", "g1", refDB).approved(time.Hour, koord, "r2", "g2", refAPI)
			},
			after: 2, deviations: 1, wp: 2,
		},
		{
			name: "after the expiry of the grant it is no deviation",
			build: func() *log {
				l := (&log{}).approved(0, koord, "r1", "g1", refDB)
				l.add(time.Hour, koord, audit.EventExpiry, []protocol.Reference{refDB}, audit.Result{GrantID: "g1", Detail: audit.DetailLifetimeEnded})
				return l.approved(2*time.Hour, koord, "r2", "g2", refAPI)
			},
			after: 2, wp: 2,
		},
		{
			name: "after the revocation of the grant it is no deviation",
			build: func() *log {
				l := (&log{}).approved(0, koord, "r1", "g1", refDB)
				l.add(time.Hour, koord, audit.EventRevocation, []protocol.Reference{refDB}, audit.Result{GrantID: "g1"})
				return l.approved(2*time.Hour, koord, "r2", "g2", refAPI)
			},
			after: 2, wp: 2,
		},
		{
			name: "after the end of the lifetime without an event it is no deviation",
			build: func() *log {
				return (&log{}).approved(0, koord, "r1", "g1", refDB).approved(5*time.Hour, koord, "r2", "g2", refAPI)
			},
			after: 2, wp: 2,
		},
		{
			name: "a grant of another session is no deviation",
			build: func() *log {
				return (&log{}).approved(0, koord, "r1", "g1", refDB).approved(time.Hour, tests, "r2", "g2", refAPI)
			},
			after: 2, wp: 2,
		},
		{
			name: "two requests in one bundle: both deliveries follow the popup",
			build: func() *log {
				l := &log{}
				l.add(0, koord, audit.EventRequest, []protocol.Reference{refDB}, audit.Result{})
				l.add(time.Second, koord, audit.EventRequest, []protocol.Reference{refAPI}, audit.Result{})
				l.add(3*time.Second, koord, audit.EventBundle, []protocol.Reference{refDB, refAPI}, audit.Result{RequestID: "r1"})
				l.add(9*time.Second, koord, audit.EventDecision, []protocol.Reference{refDB, refAPI}, audit.Result{RequestID: "r1"})
				l.add(9*time.Second, koord, audit.EventGrant, []protocol.Reference{refDB, refAPI}, audit.Result{RequestID: "r1", GrantID: "g1", LifetimeSeconds: 60})
				l.add(9*time.Second, koord, audit.EventDelivery, []protocol.Reference{refDB}, audit.Result{GrantID: "g1"})
				return l.add(9*time.Second, koord, audit.EventDelivery, []protocol.Reference{refAPI}, audit.Result{GrantID: "g1"})
			},
			after: 2, wp: 1,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			st := Compute(tc.build().entries, Scope{}, time.UTC, 5)
			if st.DeliveriesAfterPopup != tc.after || st.DeliveriesUnderGrant != tc.under || st.Deviations != tc.deviations || st.Popups != tc.wp {
				t.Errorf("after %d, under %d, deviations %d, popups %d; want %d, %d, %d, %d",
					st.DeliveriesAfterPopup, st.DeliveriesUnderGrant, st.Deviations, st.Popups,
					tc.after, tc.under, tc.deviations, tc.wp)
			}
		})
	}
}

func TestComputeErrorsAndLock(t *testing.T) {
	l := &log{}
	l.add(0, sess{}, audit.EventError, nil, audit.Result{Code: protocol.CodeAuditFailed, Detail: audit.DetailFaultBegin})
	l.add(time.Minute, koord, audit.EventError, []protocol.Reference{refDB}, audit.Result{Code: protocol.CodeDaemonFaulted, Detail: "delivery is locked"})
	l.add(2*time.Minute, sess{}, audit.EventError, nil, audit.Result{Code: protocol.CodeAuditFailed, Detail: audit.DetailFaultEnd})
	l.add(3*time.Minute, koord, audit.EventError, []protocol.Reference{refDB}, audit.Result{Code: protocol.CodeBackendUnavailable})
	l.add(4*time.Minute, koord, audit.EventRevocation, []protocol.Reference{refDB}, audit.Result{GrantID: "g9"})
	st := Compute(l.entries, Scope{}, time.UTC, 5)
	if st.Errors != 2 || st.LockBegins != 1 || st.LockEnds != 1 || st.Revocations != 1 {
		t.Errorf("errors %d, lock %d/%d, revocations %d; want 2, 1/1, 1", st.Errors, st.LockBegins, st.LockEnds, st.Revocations)
	}
	if len(st.ErrorCodes) != 2 || st.ErrorCodes[0].Name != "backend_unavailable" || st.ErrorCodes[0].Count != 1 {
		t.Errorf("ErrorCodes = %+v", st.ErrorCodes)
	}
}

func TestComputeScope(t *testing.T) {
	// UC6: the figures follow the time range, the project and the session
	// name. A grant before the range still makes a deviation in the range.
	l := (&log{}).
		approved(0, koord, "r1", "g1", refDB).
		approved(2*time.Hour, koord, "r2", "g2", refAPI).
		approved(24*time.Hour, tests, "r3", "g3", refDB)
	tt := []struct {
		name               string
		scope              Scope
		popups, deviations int
		entries            int
	}{
		{"all", Scope{}, 3, 1, 15},
		{"since one hour", Scope{Since: base.Add(time.Hour)}, 2, 1, 10},
		{"until one hour", Scope{Until: base.Add(time.Hour)}, 1, 0, 5},
		{"project", Scope{Project: "demo-projekt"}, 1, 0, 5},
		{"session name", Scope{SessionName: "beispiel-koordinator"}, 2, 1, 10},
		{"unknown project", Scope{Project: "nirgendwo"}, 0, 0, 0},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			st := Compute(l.entries, tc.scope, time.UTC, 5)
			if st.Popups != tc.popups || st.Deviations != tc.deviations || st.Entries != tc.entries {
				t.Errorf("popups %d, deviations %d, entries %d; want %d, %d, %d",
					st.Popups, st.Deviations, st.Entries, tc.popups, tc.deviations, tc.entries)
			}
		})
	}
}

func TestComputeDays(t *testing.T) {
	l := (&log{}).
		approved(0, koord, "r1", "g1", refDB).
		approved(time.Hour, koord, "r2", "g2", refAPI).
		failed(48*time.Hour, tests, "r3", audit.EventDenial, protocol.CodeDenied, refDB)
	// The range has three days: the day of base, one empty day and the
	// day of the denial.
	sc := Scope{Since: base.Add(-9 * time.Hour), Until: base.Add(63 * time.Hour)}
	st := Compute(l.entries, sc, time.UTC, 5)
	var got []string
	for _, d := range st.Days {
		got = append(got, d.Date+":"+itoa(d.Popups)+"/"+itoa(d.Denied))
	}
	want := "2026-11-10:2/0 2026-11-11:0/0 2026-11-12:1/1"
	if strings.Join(got, " ") != want {
		t.Errorf("Days = %v, want %s", got, want)
	}
	if st.PopupsPerDay != 1 || st.BusiestDay.Date != "2026-11-10" || st.BusiestDay.Popups != 2 {
		t.Errorf("PopupsPerDay %v, BusiestDay %+v", st.PopupsPerDay, st.BusiestDay)
	}
	// Local time decides the day.
	berlin := time.FixedZone("CET", 3600)
	l2 := (&log{}).failed(23*time.Hour+30*time.Minute-9*time.Hour, koord, "r1", audit.EventDenial, protocol.CodeDenied, refDB)
	st = Compute(l2.entries, Scope{}, berlin, 5)
	if len(st.Days) != 1 || st.Days[0].Date != "2026-11-11" {
		t.Errorf("local day: %+v", st.Days)
	}
}

func TestComputeTop(t *testing.T) {
	l := (&log{}).
		approved(0, koord, "r1", "g1", refDB).
		covered(time.Minute, koord, "g1", refDB).
		covered(2*time.Minute, koord, "g1", refDB).
		failed(time.Hour, tests, "r2", audit.EventDenial, protocol.CodeDenied, refAPI)
	st := Compute(l.entries, Scope{}, time.UTC, 1)
	if len(st.TopSessions) != 1 || st.TopSessions[0].Name != "beispiel-koordinator" || st.TopSessions[0].Requests != 3 ||
		st.TopSessions[0].Popups != 1 || st.TopSessions[0].Deliveries != 3 {
		t.Errorf("TopSessions = %+v", st.TopSessions)
	}
	if len(st.TopProjects) != 1 || st.TopProjects[0].Name != "beispielprojekt" {
		t.Errorf("TopProjects = %+v", st.TopProjects)
	}
	if len(st.TopReferences) != 1 || st.TopReferences[0].Name != refDB.String() || st.TopReferences[0].Requests != 3 {
		t.Errorf("TopReferences = %+v", st.TopReferences)
	}
	all := Compute(l.entries, Scope{}, time.UTC, 0)
	if len(all.TopSessions) != 2 || all.TopSessions[1].Denials != 1 {
		t.Errorf("TopSessions without limit = %+v", all.TopSessions)
	}
}

func TestComputeEmpty(t *testing.T) {
	st := Compute(nil, Scope{}, time.UTC, 5)
	if st.Entries != 0 || st.Popups != 0 || st.DenialShare != 0 || len(st.Days) != 0 || st.BundleMedian != 0 {
		t.Errorf("empty: %+v", st)
	}
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"popups":0`) {
		t.Errorf("JSON = %s", b)
	}
}

func TestRangeScope(t *testing.T) {
	now := time.Date(2026, 11, 12, 18, 30, 0, 0, time.UTC)
	tt := []struct {
		r     Range
		since string
	}{
		{RangeToday, "2026-11-12"},
		{Range7Days, "2026-11-06"},
		{Range30Days, "2026-10-14"},
		{RangeAll, ""},
	}
	for _, tc := range tt {
		t.Run(string(tc.r), func(t *testing.T) {
			sc := tc.r.Scope(now, time.UTC)
			got := ""
			if !sc.Since.IsZero() {
				got = sc.Since.Format("2006-01-02")
				if sc.Since.Hour() != 0 || sc.Since.Minute() != 0 {
					t.Errorf("Since = %v, want midnight", sc.Since)
				}
			}
			if got != tc.since {
				t.Errorf("Since = %q, want %q", got, tc.since)
			}
			if tc.r != RangeAll && !sc.Until.Equal(now) {
				t.Errorf("Until = %v, want now", sc.Until)
			}
		})
	}
	if r, err := ParseRange("30d"); err != nil || r != Range30Days {
		t.Errorf("ParseRange(30d) = %v, %v", r, err)
	}
	if _, err := ParseRange("1y"); err == nil {
		t.Error("ParseRange(1y): want error")
	}
	if RangeToday.Next() != Range7Days || RangeAll.Next() != RangeToday {
		t.Error("Next does not cycle")
	}
}

func TestSessions(t *testing.T) {
	l := (&log{}).
		approved(0, koord, "r1", "g1", refDB).
		covered(time.Minute, koord, "g1", refDB).
		failed(time.Hour, tests, "r2", audit.EventDenial, protocol.CodeDenied, refAPI)
	l.add(2*time.Hour, koord, audit.EventRevocation, []protocol.Reference{refDB}, audit.Result{GrantID: "g1"})
	l.add(3*time.Hour, sess{}, audit.EventError, nil, audit.Result{Detail: audit.DetailFaultBegin})
	got := Sessions(l.entries, Scope{})
	if len(got) != 2 {
		t.Fatalf("Sessions = %+v", got)
	}
	// The newest activity first.
	k := got[0]
	if k.ID != "s-1" || k.Name != "beispiel-koordinator" || k.Project != "beispielprojekt" || k.Requests != 2 ||
		k.Popups != 1 || k.Grants != 1 || k.Deliveries != 2 || k.Revocations != 1 || !k.Last.Equal(base.Add(2*time.Hour)) {
		t.Errorf("session 1 = %+v", k)
	}
	if got[1].Denials != 1 || got[1].Popups != 1 {
		t.Errorf("session 2 = %+v", got[1])
	}
	if len(Sessions(l.entries, Scope{Project: "demo-projekt"})) != 1 {
		t.Error("project scope ignored")
	}
}

func TestGrants(t *testing.T) {
	l := (&log{}).
		approved(0, koord, "r1", "g1", refDB).
		covered(time.Minute, koord, "g1", refDB).
		approved(time.Hour, koord, "r2", "g2", refAPI)
	l.add(2*time.Hour, koord, audit.EventRevocation, []protocol.Reference{refDB}, audit.Result{GrantID: "g1"})
	l.add(3*time.Hour, koord, audit.EventExpiry, []protocol.Reference{refAPI}, audit.Result{GrantID: "g2", Detail: audit.DetailSessionEnded})
	got := Grants(l.entries, "s-1")
	if len(got) != 2 {
		t.Fatalf("Grants = %+v", got)
	}
	if got[0].ID != "g1" || got[0].Deliveries != 2 || got[0].End != EndRevoked || !got[0].Ended.Equal(base.Add(2*time.Hour)) ||
		got[0].Lifetime != 4*time.Hour || len(got[0].References) != 1 {
		t.Errorf("g1 = %+v", got[0])
	}
	if got[1].End != EndSessionEnded || got[1].Deliveries != 1 {
		t.Errorf("g2 = %+v", got[1])
	}
}

func TestClean(t *testing.T) {
	tt := []struct{ in, want string }{
		{"beispiel-koordinator", "beispiel-koordinator"},
		{"a\x1b[2Jb", "a?[2Jb"},
		{"rechts\u202eslinks", "rechts?slinks"},
		{"isolate\u2066x\u2069", "isolate?x?"},
		{"zeile\nneu\r", "zeile?neu?"},
		{"\x9b31m", "?31m"},
		{"bad\xffutf8", "bad?utf8"},
		{"Umlaute äöü", "Umlaute äöü"},
	}
	for _, tc := range tt {
		if got := Clean(tc.in); got != tc.want {
			t.Errorf("Clean(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
