package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/audit/sqlite"
	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/protocol"
)

const auditUsage = `Usage:
  a38 audit list   [--config FILE] [--file AUDIT] [--json] [--project NAME]
                   [--session-name NAME] [--ref op://VAULT/ITEM/FIELD]
                   [--type T[,T...]] [--code CODE] [--since TIME] [--until TIME]
                   [--limit N]
  a38 audit verify [--config FILE] [--file AUDIT]

TIME is an RFC 3339 time or a duration ("24h" means 24 hours ago).
`

// runAudit lists the audit log and verifies its hash chain. It opens the
// audit file read-only.
func runAudit(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprint(stderr, auditUsage)
		return exitUsage
	}
	switch args[0] {
	case "list":
		return runAuditList(args[1:], stdout, stderr)
	case "verify":
		return runAuditVerify(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		_, _ = fmt.Fprint(stdout, auditUsage)
		return exitOK
	}
	_, _ = fmt.Fprintf(stderr, "a38 audit: unknown subcommand %q\n\n%s", args[0], auditUsage)
	return exitUsage
}

// auditFlags are the flags of both subcommands.
type auditFlags struct {
	configPath string
	file       string
}

func (a *auditFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&a.configPath, "config", "", "configuration file (default: the standard location)")
	fs.StringVar(&a.file, "file", "", "audit file (default: paths.audit_file of the configuration)")
}

// open opens the audit file read-only.
func (a *auditFlags) open() (*sqlite.Store, error) {
	path := a.file
	if path == "" {
		cfgPath := a.configPath
		if cfgPath == "" {
			var err error
			if cfgPath, err = config.DefaultPath(); err != nil {
				return nil, err
			}
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return nil, err
		}
		path = cfg.Paths.AuditFile
	}
	return sqlite.OpenReadOnly(path)
}

func newAuditFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("a38 audit "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

func runAuditVerify(args []string, stdout, stderr io.Writer) int {
	var af auditFlags
	fs := newAuditFlagSet("verify", stderr)
	af.register(fs)
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "a38 audit verify: unexpected argument")
		return exitUsage
	}
	store, err := af.open()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 audit verify: %v\n", err)
		return exitFailure
	}
	defer func() { _ = store.Close() }()
	res, err := store.Verify(context.Background())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 audit verify: %v\n", err)
		return exitFailure
	}
	if !res.OK {
		_, _ = fmt.Fprintf(stderr, "a38 audit verify: the chain is broken at entry %d: %s\n", res.FirstBadSeq, res.Finding)
		return exitFailure
	}
	_, _ = fmt.Fprintf(stdout, "The chain is intact: %d entries.\n", res.Entries)
	return exitOK
}

func runAuditList(args []string, stdout, stderr io.Writer) int {
	var (
		af                        auditFlags
		asJSON                    bool
		project, name, ref, types string
		code, since, until        string
		limit                     int
	)
	fs := newAuditFlagSet("list", stderr)
	af.register(fs)
	fs.BoolVar(&asJSON, "json", false, "print one JSON object per line")
	fs.StringVar(&project, "project", "", "only entries of this project")
	fs.StringVar(&name, "session-name", "", "only entries of sessions with this claimed name")
	fs.StringVar(&ref, "ref", "", "only entries that name this credential")
	fs.StringVar(&types, "type", "", "only these event types, separated by commas")
	fs.StringVar(&code, "code", "", "only entries with this error code")
	fs.StringVar(&since, "since", "", "only entries at or after this time")
	fs.StringVar(&until, "until", "", "only entries before this time")
	fs.IntVar(&limit, "limit", 0, "show at most this many entries, the newest")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "a38 audit list: unexpected argument")
		return exitUsage
	}
	f, err := buildFilter(time.Now(), project, name, ref, types, code, since, until, limit)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 audit list: %v\n", err)
		return exitUsage
	}
	store, err := af.open()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 audit list: %v\n", err)
		return exitFailure
	}
	defer func() { _ = store.Close() }()
	entries, err := store.List(context.Background(), f)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 audit list: %v\n", err)
		return exitFailure
	}
	if asJSON {
		enc := json.NewEncoder(stdout)
		for _, e := range entries {
			if err := enc.Encode(e); err != nil {
				_, _ = fmt.Fprintf(stderr, "a38 audit list: %v\n", err)
				return exitFailure
			}
		}
		return exitOK
	}
	printTable(stdout, entries)
	return exitOK
}

// buildFilter turns the flag values into a filter. now is the base of a
// relative time.
func buildFilter(now time.Time, project, name, ref, types, code, since, until string, limit int) (audit.Filter, error) {
	f := audit.Filter{Project: project, SessionName: name, Code: protocol.Code(code), Limit: limit}
	if limit < 0 {
		return f, fmt.Errorf("--limit must not be negative")
	}
	if ref != "" {
		r, err := protocol.ParseReference(ref)
		if err != nil {
			return f, fmt.Errorf("--ref: %w", err)
		}
		f.Reference = &r
	}
	if types != "" {
		for _, t := range strings.Split(types, ",") {
			et := audit.EventType(strings.TrimSpace(t))
			if !et.Valid() {
				return f, fmt.Errorf("--type: unknown event type %q", t)
			}
			f.Types = append(f.Types, et)
		}
	}
	var err error
	if f.Since, err = parseTime(now, since); err != nil {
		return f, fmt.Errorf("--since: %w", err)
	}
	if f.Until, err = parseTime(now, until); err != nil {
		return f, fmt.Errorf("--until: %w", err)
	}
	return f, nil
}

// parseTime reads an RFC 3339 time or a duration that counts back from now.
// Empty text is the zero time.
func parseTime(now time.Time, s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 {
		return time.Time{}, fmt.Errorf("%q is neither an RFC 3339 time nor a duration", s)
	}
	return now.Add(-d), nil
}

func printTable(w io.Writer, entries []audit.Entry) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "SEQ\tTIME\tTYPE\tPROJECT\tSESSION\tREFERENCES\tRESULT")
	for _, e := range entries {
		refs := make([]string, len(e.References))
		for i, r := range e.References {
			refs[i] = r.String()
		}
		result := string(e.Result.Code)
		if result == "" {
			result = "ok"
		}
		if e.Result.Detail != "" {
			result += " (" + e.Result.Detail + ")"
		}
		result = dash(result)
		_, _ = fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n", e.Seq,
			e.Time.UTC().Format("2006-01-02T15:04:05Z"), e.Type, dash(e.Verified.Project),
			dash(e.Claims.SessionName), dash(strings.Join(refs, ",")), result)
	}
	_ = tw.Flush()
}

// dash returns the text for a table cell. An empty text becomes "-". A
// control character becomes "?", so that an entry cannot steer the
// terminal.
func dash(s string) string {
	if s == "" {
		return "-"
	}
	return strings.Map(func(r rune) rune {
		if protocol.IsControl(r) {
			return '?'
		}
		return r
	}, s)
}
