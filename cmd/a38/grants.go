package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/seletz/passierschein/internal/protocol"
)

const grantsUsage = `Usage:
  a38 grants list   [--config FILE] [--json]
  a38 grants revoke [--config FILE] [--json] GRANT_ID

The commands talk to the daemon on the admin socket. They show no value.
`

// runGrants lists and revokes grants over the admin socket (D44).
func runGrants(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprint(stderr, grantsUsage)
		return exitUsage
	}
	switch args[0] {
	case "list":
		return runGrantsList(args[1:], stdout, stderr)
	case "revoke":
		return runGrantsRevoke(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		_, _ = fmt.Fprint(stdout, grantsUsage)
		return exitOK
	}
	_, _ = fmt.Fprintf(stderr, "a38 grants: unknown subcommand %q\n\n%s", args[0], grantsUsage)
	return exitUsage
}

func runGrantsList(args []string, stdout, stderr io.Writer) int {
	var af adminFlags
	fs := newFlagSet("grants list", stderr)
	af.register(fs)
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "a38 grants list: unexpected argument")
		return exitUsage
	}
	s, err := dialAdmin(af.configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 grants list: %v\n", err)
		return exitFailure
	}
	defer s.close()
	res, err := s.conn.GrantsList(s.ctx)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 grants list: %s\n", adminError(err))
		return exitFailure
	}
	if af.asJSON {
		grants := res.Grants
		if grants == nil {
			grants = []protocol.Grant{}
		}
		if err := json.NewEncoder(stdout).Encode(grants); err != nil {
			_, _ = fmt.Fprintf(stderr, "a38 grants list: %v\n", err)
			return exitFailure
		}
		return exitOK
	}
	printGrants(stdout, res.Grants)
	return exitOK
}

// shortID returns the first characters of a session ID.
func shortID(id string) string {
	const n = 8
	if r := []rune(id); len(r) > n {
		return string(r[:n])
	}
	return id
}

// remaining formats the remaining lifetime of a grant.
func remaining(seconds int64) string {
	if seconds <= 0 {
		return "0s"
	}
	return (time.Duration(seconds) * time.Second).String()
}

func printGrants(w io.Writer, grants []protocol.Grant) {
	if len(grants) == 0 {
		_, _ = fmt.Fprintln(w, "No active grants.")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "GRANT\tSESSION\tNAME\tPROJECT\tCREDENTIALS\tREMAINING")
	for _, g := range grants {
		refs := make([]string, len(g.References))
		for i, r := range g.References {
			refs[i] = r.String()
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", dash(g.GrantID), dash(shortID(g.SessionID)),
			dash(g.SessionName), dash(g.Project), dash(strings.Join(refs, ",")), remaining(g.RemainingSeconds))
	}
	_ = tw.Flush()
}

func runGrantsRevoke(args []string, stdout, stderr io.Writer) int {
	var af adminFlags
	fs := newFlagSet("grants revoke", stderr)
	af.register(fs)
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 1 {
		_, _ = fmt.Fprintf(stderr, "a38 grants revoke: one grant ID is necessary\n\n%s", grantsUsage)
		return exitUsage
	}
	id := fs.Arg(0)
	s, err := dialAdmin(af.configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 grants revoke: %v\n", err)
		return exitFailure
	}
	defer s.close()
	res, err := s.conn.GrantRevoke(s.ctx, id)
	if err != nil {
		if protocol.CodeOf(err) == protocol.CodeNotFound {
			_, _ = fmt.Fprintf(stderr, "a38 grants revoke: no grant with the ID %q. List the grants with \"a38 grants list\".\n", dash(id))
			return exitFailure
		}
		_, _ = fmt.Fprintf(stderr, "a38 grants revoke: %s\n", adminError(err))
		return exitFailure
	}
	if af.asJSON {
		_ = json.NewEncoder(stdout).Encode(map[string]any{"revoked": res.GrantID})
		return exitOK
	}
	_, _ = fmt.Fprintf(stdout, "The grant %s ended. The next request needs a new approval.\n", dash(res.GrantID))
	return exitOK
}
