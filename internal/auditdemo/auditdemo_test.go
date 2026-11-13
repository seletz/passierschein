package auditdemo

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/audit/sqlite"
	"github.com/seletz/passierschein/internal/auditstats"
	"github.com/seletz/passierschein/internal/clock"
)

var end = time.Date(2026, 11, 12, 17, 30, 0, 0, time.UTC)

// write generates a log into a new file and returns its entries.
func write(t *testing.T, o Options) ([]audit.Entry, audit.Verification) {
	t.Helper()
	clk := clock.NewFake(o.End.AddDate(0, 0, -o.Days-1))
	s, err := sqlite.Open(filepath.Join(t.TempDir(), "audit.db"), sqlite.WithClock(clk))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err := Generate(context.Background(), s, clk, o); err != nil {
		t.Fatal(err)
	}
	entries, err := s.List(context.Background(), audit.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.Verify(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return entries, v
}

func TestGenerate(t *testing.T) {
	entries, v := write(t, Options{End: end, Days: 30})
	if !v.OK || v.Entries != uint64(len(entries)) || len(entries) < 300 {
		t.Fatalf("verification %+v, %d entries", v, len(entries))
	}
	if last := entries[len(entries)-1].Time; last.After(end) {
		t.Errorf("an entry after the end: %v", last)
	}
	st := auditstats.Compute(entries, auditstats.Scope{}, time.UTC, 5)
	checks := []struct {
		name string
		ok   bool
	}{
		{"popups", st.Popups > 30},
		{"denied", st.Denied > 0},
		{"timed out", st.TimedOut > 0},
		{"partial", st.Partial > 0},
		{"failed", st.Failed == 1},
		{"abandoned", st.Abandoned == 1},
		{"deviations", st.Deviations > 0},
		{"policy denials", st.PolicyDenials > 0},
		{"deliveries under a grant", st.DeliveriesUnderGrant > 0},
		{"deliveries after a popup", st.DeliveriesAfterPopup > 0},
		{"one revocation", st.Revocations == 1},
		{"one lock", st.LockBegins == 1 && st.LockEnds == 1},
		{"errors: backend, lock, UI", st.Errors == 3},
		{"two projects", len(st.TopProjects) == 2},
		{"30 days", len(st.Days) == 30},
		{"events today", st.Days[len(st.Days)-1].Popups > 0},
	}
	for _, c := range checks {
		if !c.ok {
			t.Errorf("%s: %+v", c.name, st)
		}
	}
}

func TestGenerateOnEachWeekday(t *testing.T) {
	// The special cases (revocation, lock, backend error, UI failure,
	// abandoned popup) come on each weekday of the end, also when their
	// day is on a weekend.
	for d := range 7 {
		e := end.AddDate(0, 0, d)
		entries, _ := write(t, Options{End: e, Days: 30})
		st := auditstats.Compute(entries, auditstats.Scope{}, time.UTC, 5)
		if st.Revocations != 1 || st.LockBegins != 1 || st.Failed != 1 || st.Abandoned != 1 || st.Errors != 3 || st.Partial == 0 {
			t.Errorf("end %s (%s): revocations %d, locks %d, failed %d, abandoned %d, errors %d, partial %d",
				e.Format("2006-01-02"), e.Weekday(), st.Revocations, st.LockBegins, st.Failed, st.Abandoned, st.Errors, st.Partial)
		}
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	a, _ := write(t, Options{End: end, Days: 7})
	b, _ := write(t, Options{End: end, Days: 7})
	if len(a) != len(b) || a[len(a)-1].Hash != b[len(b)-1].Hash {
		t.Errorf("two runs differ: %d and %d entries", len(a), len(b))
	}
	c, _ := write(t, Options{End: end, Days: 7, Seed: 2})
	if len(c) == len(a) && c[len(c)-1].Hash == a[len(a)-1].Hash {
		t.Error("another seed gives the same log")
	}
}

func TestGenerateRejectsNoDays(t *testing.T) {
	if err := Generate(context.Background(), nil, clock.NewFake(end), Options{End: end}); err == nil {
		t.Error("want an error")
	}
}
