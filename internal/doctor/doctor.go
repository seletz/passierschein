// Package doctor holds the checks of "a38 doctor" (F20).
//
// Each check is a small type behind the [Check] interface. A check reads
// the world only through [Env], so that a test can fake the file system,
// the daemon and the audit file. A check never prints or returns a secret
// value; it has none: the daemon never sends a value on the admin socket,
// and the audit log holds references only.
package doctor

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"strings"
	"time"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/protocol"
)

// MinFreeBytes is the least free space that the check of the audit
// directory accepts: 100 MiB. A few thousand entries need far less. The
// threshold warns early, before the log cannot grow and the daemon locks
// delivery (D36, D48).
const MinFreeBytes = 100 << 20

// Result is the outcome of one check. A finding has a cause and a next
// step.
type Result struct {
	// Check is the short name of the check.
	Check string `json:"check"`
	// OK is true if the check found nothing wrong.
	OK bool `json:"ok"`
	// Detail says what the check saw. It is set also for OK.
	Detail string `json:"detail,omitempty"`
	// Finding is the problem in one sentence. Empty if OK.
	Finding string `json:"finding,omitempty"`
	// Cause is the reason of the problem. Empty if OK.
	Cause string `json:"cause,omitempty"`
	// NextStep is what the human does next. Empty if OK.
	NextStep string `json:"next_step,omitempty"`
	// Note marks a result with OK true that names a known limit, for
	// example the unstable signature of a38 (G5). A note is no finding;
	// it does not change the exit code.
	Note bool `json:"note,omitempty"`
}

func ok(check, format string, args ...any) Result {
	return Result{Check: check, OK: true, Detail: fmt.Sprintf(format, args...)}
}

func note(check, format string, args ...any) Result {
	return Result{Check: check, OK: true, Note: true, Detail: fmt.Sprintf(format, args...)}
}

func finding(check, what, cause, next string) Result {
	return Result{Check: check, Finding: what, Cause: cause, NextStep: next}
}

// State is the data that the checks share. A check that needs data of an
// earlier check reads it here and says nothing if the data is missing: the
// earlier check has reported the problem.
type State struct {
	// Config is the loaded configuration; nil if loading failed.
	Config *config.Config
	// Status is the answer of the daemon; nil if the daemon did not answer.
	Status *protocol.StatusResult
}

// Check is one check of the doctor.
type Check interface {
	// Name returns the short name of the check.
	Name() string
	// Run runs the check. It returns no result if it has nothing to say,
	// for example because an earlier check has reported its precondition.
	Run(ctx context.Context, st *State) []Result
}

// Checks returns the checks in the sequence of the output.
func Checks(env Env) []Check {
	return []Check{
		configCheck{env},
		socketCheck{env},
		daemonCheck{env},
		lockCheck{},
		projectCheck{},
		auditFileCheck{env},
		auditSpaceCheck{env},
		auditChainCheck{env},
		backendCheck{env},
		tokenSourceCheck{env, runtime.GOOS},
		uiCheck{env},
	}
}

// Report is the outcome of all checks.
type Report struct {
	// OK is true if no check has a finding.
	OK bool `json:"ok"`
	// Results lists the results in the sequence of the checks.
	Results []Result `json:"results"`
}

// Findings returns the number of findings.
func (r Report) Findings() int {
	n := 0
	for _, res := range r.Results {
		if !res.OK {
			n++
		}
	}
	return n
}

// Run runs all checks.
func Run(ctx context.Context, env Env) Report {
	st := &State{}
	rep := Report{OK: true, Results: []Result{}}
	for _, c := range Checks(env) {
		for _, res := range c.Run(ctx, st) {
			if !res.OK {
				rep.OK = false
			}
			rep.Results = append(rep.Results, res)
		}
	}
	return rep
}

// Render writes the report as text. A line "OK", "NOTE" or "FINDING"
// starts each result; a finding has the cause and the next step below.
func Render(w io.Writer, r Report) {
	for _, res := range r.Results {
		if res.OK {
			label := "OK"
			if res.Note {
				label = "NOTE"
			}
			_, _ = fmt.Fprintf(w, "%-8s %-18s %s\n", label, res.Check, clean(res.Detail))
			continue
		}
		_, _ = fmt.Fprintf(w, "FINDING  %-18s %s\n", res.Check, clean(res.Finding))
		_, _ = fmt.Fprintf(w, "         Cause:     %s\n", clean(res.Cause))
		_, _ = fmt.Fprintf(w, "         Next step: %s\n", clean(res.NextStep))
	}
	if r.OK {
		_, _ = fmt.Fprintln(w, "\nNo findings.")
		return
	}
	n := r.Findings()
	word := "findings"
	if n == 1 {
		word = "finding"
	}
	_, _ = fmt.Fprintf(w, "\n%d %s.\n", n, word)
}

// clean replaces a control character, so that a text from a file or from
// the daemon cannot steer the terminal.
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if protocol.IsControl(r) {
			return '?'
		}
		return r
	}, s)
}

// since formats the start of a lock.
func since(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return " since " + t.UTC().Format("2006-01-02T15:04:05Z")
}
