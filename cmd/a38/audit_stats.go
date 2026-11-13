package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/auditstats"
)

// runAuditStats prints the figures of the audit log: the same figures as
// the overview of "a38 tui" (F13, G12). It opens the audit file read-only.
func runAuditStats(args []string, stdout, stderr io.Writer) int {
	var (
		af               auditFlags
		asJSON           bool
		rangeName        string
		project, session string
		top              int
	)
	fs := newAuditFlagSet("stats", stderr)
	af.register(fs)
	fs.BoolVar(&asJSON, "json", false, "print the figures as one JSON object")
	fs.StringVar(&rangeName, "range", string(auditstats.Range7Days), "time range: today, 7d, 30d or all")
	fs.StringVar(&project, "project", "", "only entries of this project")
	fs.StringVar(&session, "session-name", "", "only entries of sessions with this claimed name")
	fs.IntVar(&top, "top", 5, "length of the ranked lists (0: all)")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return exitUsage
	}
	if len(pos) != 0 {
		_, _ = fmt.Fprintln(stderr, "a38 audit stats: unexpected argument")
		return exitUsage
	}
	rng, err := auditstats.ParseRange(rangeName)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 audit stats: --range: %v\n", err)
		return exitUsage
	}
	if top < 0 {
		_, _ = fmt.Fprintln(stderr, "a38 audit stats: --top must not be negative")
		return exitUsage
	}
	store, err := af.open()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 audit stats: %v\n", err)
		return exitFailure
	}
	defer func() { _ = store.Close() }()
	entries, err := store.List(context.Background(), audit.Filter{})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 audit stats: %v\n", err)
		return exitFailure
	}
	sc := rng.Scope(time.Now(), time.Local)
	sc.Project, sc.SessionName = project, session
	st := auditstats.Compute(entries, sc, time.Local, top)
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(st); err != nil {
			_, _ = fmt.Fprintf(stderr, "a38 audit stats: %v\n", err)
			return exitFailure
		}
		return exitOK
	}
	printStats(stdout, rng, st)
	return exitOK
}

// printStats prints the main figures as lines "name: value".
func printStats(w io.Writer, rng auditstats.Range, st auditstats.Stats) {
	p := func(format string, a ...any) { _, _ = fmt.Fprintf(w, format+"\n", a...) }
	p("Range:                  %s (%d days)", rng.Label(), len(st.Days))
	p("Entries:                %d", st.Entries)
	p("Requests:               %d", st.Requests)
	p("Popups:                 %d (%.1f per day)", st.Popups, st.PopupsPerDay)
	p("Approved:               %d (partial %d)", st.Approved, st.Partial)
	p("Denied:                 %d (denial share %.0f%%)", st.Denied, st.DenialShare*100)
	p("Timed out:              %d", st.TimedOut)
	p("UI failed:              %d", st.Failed)
	p("Deviations:             %d", st.Deviations)
	p("Entries per popup:      median %g, largest %d", st.BundleMedian, st.BundleMax)
	p("Deliveries:             %d (after a popup %d, under a grant %d)", st.Deliveries, st.DeliveriesAfterPopup, st.DeliveriesUnderGrant)
	p("Policy denials:         %d", st.PolicyDenials)
	p("Revocations:            %d", st.Revocations)
	p("Errors:                 %d", st.Errors)
	p("Lock begins and ends:   %d, %d", st.LockBegins, st.LockEnds)
	for _, r := range st.TopSessions {
		p("Session:                %s (%d requests, %d popups)", dash(r.Name), r.Requests, r.Popups)
	}
	for _, r := range st.TopProjects {
		p("Project:                %s (%d requests, %d popups)", dash(r.Name), r.Requests, r.Popups)
	}
	for _, r := range st.TopReferences {
		p("Credential:             %s (%d requests)", dash(r.Name), r.Requests)
	}
}
