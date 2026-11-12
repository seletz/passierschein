// Package auditstats computes figures from the entries of the audit log:
// approval requests per day, the share of denials, deviations, deliveries
// with and without a popup, and the size of the bundles (feature F13, use
// case UC6, gap G12).
//
// The package is pure: it reads a slice of entries and returns values. It
// has no terminal code and no clock. The TUI ("a38 tui") and
// "a38 audit stats" use the same figures.
//
// The audit log has no field for a popup or a deviation. The package
// derives them from the events (see [Compute]). The definitions are a
// proposal from development and wait for the confirmation of the owner.
package auditstats

import (
	"cmp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/protocol"
)

// Clean returns s fit for a terminal. Each control character, each format
// character (this includes the bidi controls) and each invalid byte
// becomes "?". So text from the audit log cannot move the cursor, change
// colours or reverse the reading direction. Each string from the audit
// log is untrusted: the session name and the claims come from the agent.
func Clean(s string) string {
	return strings.Map(func(r rune) rune {
		if protocol.IsControl(r) {
			return '?'
		}
		return r
	}, s)
}

// Scope selects the entries that the figures count. A zero field selects
// all.
type Scope struct {
	// Since selects entries at or after the time.
	Since time.Time
	// Until selects entries before the time.
	Until time.Time
	// Project selects entries of one project (verified fact).
	Project string
	// SessionName selects entries of sessions that claim the name.
	SessionName string
}

// Contains reports whether the scope selects e.
func (s Scope) Contains(e audit.Entry) bool {
	if !s.Since.IsZero() && e.Time.Before(s.Since) {
		return false
	}
	if !s.Until.IsZero() && !e.Time.Before(s.Until) {
		return false
	}
	if s.Project != "" && e.Verified.Project != s.Project {
		return false
	}
	if s.SessionName != "" && e.Claims.SessionName != s.SessionName {
		return false
	}
	return true
}

// Outcome is the end of an approval request (a popup).
type Outcome string

// The outcomes of an approval request.
const (
	// OutcomePending: the bundle has no outcome event yet. A grant covered
	// each entry at the turn of the bundle, or the daemon stopped.
	OutcomePending Outcome = ""
	// OutcomeApproved: the human approved all entries.
	OutcomeApproved Outcome = "approved"
	// OutcomePartial: the human approved some entries and denied the
	// others.
	OutcomePartial Outcome = "partial"
	// OutcomeDenied: the human denied the request.
	OutcomeDenied Outcome = "denied"
	// OutcomeTimeout: the request expired without a decision.
	OutcomeTimeout Outcome = "timeout"
	// OutcomeFailed: the approval UI failed or gave an invalid decision.
	OutcomeFailed Outcome = "failed"
	// OutcomeAbandoned: the session ended before a decision.
	OutcomeAbandoned Outcome = "abandoned"
)

// Day holds the figures of one calendar day in the local time.
type Day struct {
	// Date is the day as YYYY-MM-DD.
	Date string `json:"date"`
	// Popups is the number of approval requests that the human saw.
	Popups int `json:"popups"`
	// Denied is the number of approval requests that the human denied.
	Denied int `json:"denied"`
	// Deliveries is the number of delivery events.
	Deliveries int `json:"deliveries"`
}

// Rank holds the counts of one session name, project, credential or code.
type Rank struct {
	// Name is the session name, the project, the reference or the code.
	Name string `json:"name"`
	// Requests is the number of request events.
	Requests int `json:"requests,omitempty"`
	// Popups is the number of approval requests that the human saw.
	Popups int `json:"popups,omitempty"`
	// Deliveries is the number of delivery events.
	Deliveries int `json:"deliveries,omitempty"`
	// Denials is the number of denial events.
	Denials int `json:"denials,omitempty"`
	// Count is the number of events, for a code.
	Count int `json:"count,omitempty"`
}

// Stats holds the figures of one scope. The JSON form is the output of
// "a38 audit stats --json".
type Stats struct {
	// Since and Until are the bounds of the scope; zero means open.
	Since time.Time `json:"since,omitzero"`
	Until time.Time `json:"until,omitzero"`
	// Entries is the number of entries in the scope.
	Entries int `json:"entries"`
	// Requests is the number of request events of agents.
	Requests int `json:"requests"`
	// Popups is the number of approval requests that the human saw: a
	// bundle event followed by a decision, a denial, a timeout or a
	// failure with the same request ID.
	Popups int `json:"popups"`
	// Approved counts the popups with a decision, with the partial ones.
	Approved int `json:"approved"`
	// Partial counts the popups with a decision and a denial.
	Partial int `json:"partial"`
	// Denied counts the popups that the human denied.
	Denied int `json:"denied"`
	// TimedOut counts the popups that expired without a decision.
	TimedOut int `json:"timed_out"`
	// Failed counts the popups that ended with a failure of the UI.
	Failed int `json:"failed"`
	// Abandoned counts the bundles whose session ended before a decision.
	// They are not popups.
	Abandoned int `json:"abandoned"`
	// DenialShare is Denied divided by Popups, from 0 to 1.
	DenialShare float64 `json:"denial_share"`
	// Deviations counts the popups of a session that had a live grant at
	// the time of the bundle: the request went beyond the grant.
	Deviations int `json:"deviations"`
	// PolicyDenials counts the denial events without a request ID: the
	// policy, an unknown item or a full bundle denied the request before a
	// popup.
	PolicyDenials int `json:"policy_denials"`
	// Grants counts the grant events.
	Grants int `json:"grants"`
	// Deliveries counts the delivery events.
	Deliveries int `json:"deliveries"`
	// DeliveriesAfterPopup counts the deliveries whose grant came after the
	// last request of the session: the human approved this request.
	DeliveriesAfterPopup int `json:"deliveries_after_popup"`
	// DeliveriesUnderGrant counts the other deliveries: an earlier grant
	// covered the request, without a popup.
	DeliveriesUnderGrant int `json:"deliveries_under_grant"`
	// Revocations counts the revocation events.
	Revocations int `json:"revocations"`
	// Errors counts the error events without the marks of the global
	// lock.
	Errors int `json:"errors"`
	// ErrorCodes counts the errors per code, the most frequent first.
	ErrorCodes []Rank `json:"error_codes"`
	// LockBegins and LockEnds count the begin and the end of the global
	// delivery lock.
	LockBegins int `json:"lock_begins"`
	LockEnds   int `json:"lock_ends"`
	// BundleMax is the largest number of entries in one popup.
	BundleMax int `json:"bundle_max"`
	// BundleMedian is the median number of entries in one popup.
	BundleMedian float64 `json:"bundle_median"`
	// Days holds one value per calendar day of the scope, the oldest
	// first.
	Days []Day `json:"days"`
	// PopupsPerDay is Popups divided by the number of days.
	PopupsPerDay float64 `json:"popups_per_day"`
	// BusiestDay is the day with the most popups.
	BusiestDay Day `json:"busiest_day"`
	// TopSessions, TopProjects and TopReferences rank the session names,
	// the projects and the credentials by the number of requests.
	TopSessions   []Rank `json:"top_sessions"`
	TopProjects   []Rank `json:"top_projects"`
	TopReferences []Rank `json:"top_references"`
}

// popup is the state of one bundle while Compute reads the log.
type popup struct {
	inScope   bool
	time      time.Time
	session   string
	project   string
	outcome   Outcome
	entries   int
	deviation bool
}

// grantState is the state of one grant while the log is read.
type grantState struct {
	seq     uint64
	session string
	expires time.Time
}

// Compute returns the figures of the entries in the scope. entries must be
// in the sequence of their numbers. Compute reads all entries to follow
// the grants and the requests, also those before the scope, and counts
// only the entries in the scope. loc gives the calendar days. top limits
// the ranked lists; 0 means no limit.
//
// The definitions:
//
//   - A popup is a bundle event with a request ID that a decision, a
//     denial with the code denied, an expiry with the code timeout or an
//     error with the same request ID follows. A bundle without such an
//     event had no popup: a grant covered each entry at its turn.
//   - The size of a popup is the number of references in its outcome
//     events (the open entries that the human saw).
//   - A deviation is a popup of a session that had a live grant at the
//     time of the bundle. A grant lives from its event until its lifetime
//     ends or an expiry or a revocation event with its ID.
//   - A delivery follows a popup if its grant event came after the last
//     request event of the session before the delivery. Otherwise an
//     earlier grant covered it.
func Compute(entries []audit.Entry, sc Scope, loc *time.Location, top int) Stats {
	st := Stats{Since: sc.Since, Until: sc.Until}
	if loc == nil {
		loc = time.UTC
	}
	popups := map[string]*popup{}
	var order []*popup
	grants := map[string]*grantState{}
	lastRequest := map[string]uint64{}
	days := map[string]*Day{}
	sessions, projects, refs, codes := ranks{}, ranks{}, ranks{}, ranks{}
	var first, last time.Time

	for _, e := range entries {
		sid := string(e.Verified.SessionID)
		in := sc.Contains(e)
		if in {
			st.Entries++
			if first.IsZero() {
				first = e.Time
			}
			last = e.Time
		}
		day := func() *Day {
			k := e.Time.In(loc).Format(time.DateOnly)
			d := days[k]
			if d == nil {
				d = &Day{Date: k}
				days[k] = d
			}
			return d
		}
		rid := e.Result.RequestID
		switch e.Type {
		case audit.EventRequest:
			lastRequest[sid] = e.Seq
			if in {
				st.Requests++
				sessions.get(sessionName(e)).Requests++
				projects.get(projectName(e)).Requests++
				for _, r := range e.References {
					refs.get(r.String()).Requests++
				}
			}
		case audit.EventBundle:
			if rid == "" {
				break
			}
			p := &popup{inScope: in, time: e.Time, session: sessionName(e), project: projectName(e)}
			for _, g := range grants {
				if g.session == sid && e.Time.Before(g.expires) {
					p.deviation = true
				}
			}
			popups[rid] = p
			order = append(order, p)
		case audit.EventDecision:
			if p := popups[rid]; p != nil {
				p.entries += len(e.References)
				switch p.outcome {
				case OutcomeDenied:
					p.outcome = OutcomePartial
				case OutcomePending:
					p.outcome = OutcomeApproved
				}
			}
		case audit.EventDenial:
			if in {
				sessions.get(sessionName(e)).Denials++
				projects.get(projectName(e)).Denials++
				for _, r := range e.References {
					refs.get(r.String()).Denials++
				}
			}
			if p := popups[rid]; rid != "" && p != nil {
				p.entries += len(e.References)
				switch p.outcome {
				case OutcomeApproved:
					p.outcome = OutcomePartial
				case OutcomePending:
					p.outcome = OutcomeDenied
				}
			} else if rid == "" && in {
				st.PolicyDenials++
			}
		case audit.EventExpiry:
			if g := grants[e.Result.GrantID]; e.Result.GrantID != "" && g != nil {
				g.expires = e.Time
			}
			if p := popups[rid]; rid != "" && p != nil && p.outcome == OutcomePending {
				p.entries += len(e.References)
				if e.Result.Code == protocol.CodeTimeout {
					p.outcome = OutcomeTimeout
				} else {
					p.outcome = OutcomeAbandoned
				}
			}
		case audit.EventError:
			if p := popups[rid]; rid != "" && p != nil && p.outcome == OutcomePending {
				p.entries += len(e.References)
				p.outcome = OutcomeFailed
			}
			if !in {
				break
			}
			switch e.Result.Detail {
			case audit.DetailFaultBegin:
				st.LockBegins++
			case audit.DetailFaultEnd:
				st.LockEnds++
			default:
				st.Errors++
				codes.get(string(e.Result.Code)).Count++
			}
		case audit.EventGrant:
			grants[e.Result.GrantID] = &grantState{
				seq: e.Seq, session: sid,
				expires: e.Time.Add(time.Duration(e.Result.LifetimeSeconds) * time.Second),
			}
			if in {
				st.Grants++
			}
		case audit.EventRevocation:
			if g := grants[e.Result.GrantID]; g != nil {
				g.expires = e.Time
			}
			if in {
				st.Revocations++
			}
		case audit.EventDelivery:
			if !in {
				break
			}
			st.Deliveries++
			day().Deliveries++
			sessions.get(sessionName(e)).Deliveries++
			projects.get(projectName(e)).Deliveries++
			for _, r := range e.References {
				refs.get(r.String()).Deliveries++
			}
			if g := grants[e.Result.GrantID]; g != nil && g.seq > lastRequest[sid] {
				st.DeliveriesAfterPopup++
			} else {
				st.DeliveriesUnderGrant++
			}
		}
	}

	var sizes []int
	for _, p := range order {
		if !p.inScope {
			continue
		}
		switch p.outcome {
		case OutcomePending:
			continue
		case OutcomeAbandoned:
			st.Abandoned++
			continue
		case OutcomeApproved:
			st.Approved++
		case OutcomePartial:
			st.Approved++
			st.Partial++
		case OutcomeDenied:
			st.Denied++
		case OutcomeTimeout:
			st.TimedOut++
		case OutcomeFailed:
			st.Failed++
		}
		st.Popups++
		if p.deviation {
			st.Deviations++
		}
		sizes = append(sizes, p.entries)
		k := p.time.In(loc).Format(time.DateOnly)
		d := days[k]
		if d == nil {
			d = &Day{Date: k}
			days[k] = d
		}
		d.Popups++
		if p.outcome == OutcomeDenied {
			d.Denied++
		}
		sessions.get(p.session).Popups++
		projects.get(p.project).Popups++
	}
	if st.Popups > 0 {
		st.DenialShare = float64(st.Denied) / float64(st.Popups)
	}
	st.BundleMax, st.BundleMedian = maxMedian(sizes)
	st.Days = dayList(days, sc, first, last, loc)
	if len(st.Days) > 0 {
		st.PopupsPerDay = float64(st.Popups) / float64(len(st.Days))
		for _, d := range st.Days {
			if d.Popups > st.BusiestDay.Popups {
				st.BusiestDay = d
			}
		}
	}
	byRequests := func(a, b Rank) int {
		return cmp.Or(cmp.Compare(b.Requests, a.Requests), cmp.Compare(b.Popups, a.Popups), strings.Compare(a.Name, b.Name))
	}
	st.TopSessions = sessions.sorted(byRequests, top)
	st.TopProjects = projects.sorted(byRequests, top)
	st.TopReferences = refs.sorted(byRequests, top)
	st.ErrorCodes = codes.sorted(func(a, b Rank) int {
		return cmp.Or(cmp.Compare(b.Count, a.Count), strings.Compare(a.Name, b.Name))
	}, 0)
	return st
}

// sessionName returns the claimed session name of e, or a mark.
func sessionName(e audit.Entry) string {
	if e.Claims.SessionName == "" {
		return "(no name)"
	}
	return e.Claims.SessionName
}

// projectName returns the project of e, or a mark.
func projectName(e audit.Entry) string {
	if e.Verified.Project == "" {
		return "(no project)"
	}
	return e.Verified.Project
}

// maxMedian returns the largest value and the median.
func maxMedian(v []int) (int, float64) {
	if len(v) == 0 {
		return 0, 0
	}
	s := slices.Clone(v)
	slices.Sort(s)
	n := len(s)
	if n%2 == 1 {
		return s[n-1], float64(s[n/2])
	}
	return s[n-1], float64(s[n/2-1]+s[n/2]) / 2
}

// dayList returns one Day per calendar day from the start to the end of
// the scope. An open bound uses the first or the last entry.
func dayList(days map[string]*Day, sc Scope, first, last time.Time, loc *time.Location) []Day {
	from, to := sc.Since, sc.Until
	if from.IsZero() {
		from = first
	}
	if to.IsZero() {
		to = last
	} else {
		to = to.Add(-time.Nanosecond)
	}
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return nil
	}
	var out []Day
	f, t := from.In(loc), to.In(loc)
	d := time.Date(f.Year(), f.Month(), f.Day(), 0, 0, 0, 0, loc)
	end := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	for !d.After(end) {
		k := d.Format(time.DateOnly)
		if v := days[k]; v != nil {
			out = append(out, *v)
		} else {
			out = append(out, Day{Date: k})
		}
		d = d.AddDate(0, 0, 1)
	}
	return out
}

// ranks counts per name.
type ranks map[string]*Rank

func (r ranks) get(name string) *Rank {
	v := r[name]
	if v == nil {
		v = &Rank{Name: name}
		r[name] = v
	}
	return v
}

// sorted returns the ranks in the sequence of less, at most top (0 means
// all).
func (r ranks) sorted(less func(a, b Rank) int, top int) []Rank {
	out := make([]Rank, 0, len(r))
	for _, v := range r {
		out = append(out, *v)
	}
	slices.SortFunc(out, less)
	if top > 0 && len(out) > top {
		out = out[:top]
	}
	return out
}

// Range is a time range of the figures, relative to now.
type Range string

// The time ranges.
const (
	// RangeToday is the current calendar day.
	RangeToday Range = "today"
	// Range7Days is the current day and the six days before it.
	Range7Days Range = "7d"
	// Range30Days is the current day and the 29 days before it.
	Range30Days Range = "30d"
	// RangeAll is the whole audit log.
	RangeAll Range = "all"
)

var rangeOrder = []Range{RangeToday, Range7Days, Range30Days, RangeAll}

// ParseRange reads a range name: today, 7d, 30d or all.
func ParseRange(s string) (Range, error) {
	for _, r := range rangeOrder {
		if string(r) == s {
			return r, nil
		}
	}
	return "", &rangeError{s}
}

type rangeError struct{ s string }

func (e *rangeError) Error() string {
	return "unknown time range " + `"` + Clean(e.s) + `"` + " (today, 7d, 30d, all)"
}

// Next returns the next range in the cycle today, 7d, 30d, all.
func (r Range) Next() Range {
	i := slices.Index(rangeOrder, r)
	return rangeOrder[(i+1)%len(rangeOrder)]
}

// Label returns a short text for the range.
func (r Range) Label() string {
	switch r {
	case RangeToday:
		return "today"
	case Range7Days:
		return "7 days"
	case Range30Days:
		return "30 days"
	}
	return "all"
}

// Scope returns the scope of the range: from local midnight of the first
// day until now. RangeAll is open at both ends.
func (r Range) Scope(now time.Time, loc *time.Location) Scope {
	n := now.In(loc)
	midnight := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
	switch r {
	case RangeToday:
		return Scope{Since: midnight, Until: now}
	case Range7Days:
		return Scope{Since: midnight.AddDate(0, 0, -6), Until: now}
	case Range30Days:
		return Scope{Since: midnight.AddDate(0, 0, -29), Until: now}
	}
	return Scope{}
}

// Session holds the figures of one session.
type Session struct {
	// ID is the session ID of the daemon.
	ID string `json:"id"`
	// Name is the claimed session name of the newest entry.
	Name string `json:"name"`
	// Agent is the claimed name of the agent tool.
	Agent string `json:"agent"`
	// Project is the project (verified fact).
	Project string `json:"project"`
	// First and Last are the times of the first and the last entry.
	First time.Time `json:"first"`
	Last  time.Time `json:"last"`
	// The counts of the session.
	Entries     int `json:"entries"`
	Requests    int `json:"requests"`
	Popups      int `json:"popups"`
	Grants      int `json:"grants"`
	Deliveries  int `json:"deliveries"`
	Denials     int `json:"denials"`
	Revocations int `json:"revocations"`
	Errors      int `json:"errors"`
}

// Sessions returns one value per session ID in the scope, the newest
// activity first. Entries without a session ID (the global lock) are not
// part of a session.
func Sessions(entries []audit.Entry, sc Scope) []Session {
	by := map[string]*Session{}
	popupSeen := map[string]bool{}
	for _, e := range entries {
		sid := string(e.Verified.SessionID)
		if sid == "" || !sc.Contains(e) {
			continue
		}
		s := by[sid]
		if s == nil {
			s = &Session{ID: sid, First: e.Time}
			by[sid] = s
		}
		s.Last = e.Time
		if e.Claims.SessionName != "" {
			s.Name = e.Claims.SessionName
		}
		if e.Claims.Agent != "" {
			s.Agent = e.Claims.Agent
		}
		if e.Verified.Project != "" {
			s.Project = e.Verified.Project
		}
		s.Entries++
		switch e.Type {
		case audit.EventRequest:
			s.Requests++
		case audit.EventGrant:
			s.Grants++
		case audit.EventDelivery:
			s.Deliveries++
		case audit.EventDenial:
			s.Denials++
		case audit.EventRevocation:
			s.Revocations++
		case audit.EventError:
			s.Errors++
		}
		// A popup: the first outcome event of a request ID.
		if rid := e.Result.RequestID; rid != "" && !popupSeen[rid] && isPopupOutcome(e) {
			popupSeen[rid] = true
			s.Popups++
		}
	}
	out := make([]Session, 0, len(by))
	for _, s := range by {
		out = append(out, *s)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].Last.Equal(out[j].Last) {
			return out[i].Last.After(out[j].Last)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// isPopupOutcome reports whether e ends an approval request that the human
// saw.
func isPopupOutcome(e audit.Entry) bool {
	switch e.Type {
	case audit.EventDecision, audit.EventError:
		return true
	case audit.EventDenial:
		return e.Result.Code == protocol.CodeDenied
	case audit.EventExpiry:
		return e.Result.Code == protocol.CodeTimeout
	}
	return false
}

// End says how a grant ended.
type End string

// The ends of a grant.
const (
	// EndNone: no event ended the grant. It lives, or its lifetime ended
	// without an event (the daemon writes the event late or not at all
	// after a crash).
	EndNone End = ""
	// EndLifetime: the lifetime ended.
	EndLifetime End = "lifetime_ended"
	// EndSessionEnded: the session ended.
	EndSessionEnded End = "session_ended"
	// EndDaemonStopped: the daemon stopped.
	EndDaemonStopped End = "daemon_stopped"
	// EndRevoked: the human revoked the grant.
	EndRevoked End = "revoked"
)

// Grant holds the life of one grant.
type Grant struct {
	// ID is the grant ID.
	ID string `json:"id"`
	// Granted is the time of the grant event.
	Granted time.Time `json:"granted"`
	// Lifetime is the lifetime of the grant.
	Lifetime time.Duration `json:"lifetime"`
	// References are the credentials of the grant.
	References []protocol.Reference `json:"references"`
	// Deliveries counts the delivery events under the grant.
	Deliveries int `json:"deliveries"`
	// Ended is the time of the expiry or revocation event; zero if none.
	Ended time.Time `json:"ended,omitzero"`
	// End says how the grant ended.
	End End `json:"end,omitempty"`
}

// Grants returns the grants of one session in the sequence of their
// events.
func Grants(entries []audit.Entry, sessionID string) []Grant {
	var out []Grant
	idx := map[string]int{}
	for _, e := range entries {
		if string(e.Verified.SessionID) != sessionID {
			continue
		}
		gid := e.Result.GrantID
		if gid == "" {
			continue
		}
		i, known := idx[gid]
		switch e.Type {
		case audit.EventGrant:
			idx[gid] = len(out)
			out = append(out, Grant{
				ID: gid, Granted: e.Time, References: e.References,
				Lifetime: time.Duration(e.Result.LifetimeSeconds) * time.Second,
			})
		case audit.EventDelivery:
			if known {
				out[i].Deliveries++
			}
		case audit.EventRevocation:
			if known && out[i].End == EndNone {
				out[i].Ended, out[i].End = e.Time, EndRevoked
			}
		case audit.EventExpiry:
			if known && out[i].End == EndNone {
				out[i].Ended, out[i].End = e.Time, End(e.Result.Detail)
			}
		}
	}
	return out
}
