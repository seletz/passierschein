package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/seletz/passierschein/internal/protocol"
)

const faultUsage = `Usage:
  a38 fault status [--config FILE] [--json]
  a38 fault clear  [--config FILE] [--json]

"status" shows whether the daemon locks delivery for all sessions (D48). It
exits with 1 while the lock is set.
"clear" asks the daemon to remove the lock. The daemon removes it only if
the cause is gone. Otherwise the command prints the finding and exits with 1.
`

// runFault shows and lifts the global delivery lock over the admin socket
// (D48, G20).
func runFault(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprint(stderr, faultUsage)
		return exitUsage
	}
	switch args[0] {
	case "status":
		return runFaultStatus(args[1:], stdout, stderr)
	case "clear":
		return runFaultClear(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		_, _ = fmt.Fprint(stdout, faultUsage)
		return exitOK
	}
	_, _ = fmt.Fprintf(stderr, "a38 fault: unknown subcommand %q\n\n%s", args[0], faultUsage)
	return exitUsage
}

func runFaultStatus(args []string, stdout, stderr io.Writer) int {
	var af adminFlags
	fs := newFlagSet("fault status", stderr)
	af.register(fs)
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return exitUsage
	}
	if len(pos) != 0 {
		_, _ = fmt.Fprintln(stderr, "a38 fault status: unexpected argument")
		return exitUsage
	}
	s, err := dialAdmin(af.configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 fault status: %v\n", err)
		return exitFailure
	}
	defer s.close()
	st, err := s.conn.Status(s.ctx)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 fault status: %s\n", adminError(err))
		return exitFailure
	}
	locked := st.Fault != nil
	if af.asJSON {
		_ = json.NewEncoder(stdout).Encode(map[string]any{"locked": locked, "fault": st.Fault})
	} else if !locked {
		_, _ = fmt.Fprintln(stdout, "Delivery is open.")
	} else {
		f := st.Fault
		_, _ = fmt.Fprintln(stdout, "Delivery is locked for all sessions.")
		if f.Code != "" {
			_, _ = fmt.Fprintf(stdout, "Cause: %s\n", dash(string(f.Code)))
		}
		if f.Reason != "" {
			_, _ = fmt.Fprintf(stdout, "Finding: %s\n", dash(f.Reason))
		}
		if !f.Since.IsZero() {
			_, _ = fmt.Fprintf(stdout, "Since: %s\n", f.Since.UTC().Format("2006-01-02T15:04:05Z"))
		}
		_, _ = fmt.Fprintln(stdout, "Next step: fix the cause, then run \"a38 fault clear\".")
	}
	if locked {
		return exitFailure
	}
	return exitOK
}

func runFaultClear(args []string, stdout, stderr io.Writer) int {
	var af adminFlags
	fs := newFlagSet("fault clear", stderr)
	af.register(fs)
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return exitUsage
	}
	if len(pos) != 0 {
		_, _ = fmt.Fprintln(stderr, "a38 fault clear: unexpected argument")
		return exitUsage
	}
	s, err := dialAdmin(af.configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 fault clear: %v\n", err)
		return exitFailure
	}
	defer s.close()
	res, err := s.conn.FaultClear(s.ctx)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 fault clear: %s\n", adminError(err))
		return exitFailure
	}
	return printFaultClear(res, af.asJSON, stdout, stderr)
}

// printFaultClear prints the result of fault_clear and returns the exit
// code.
func printFaultClear(res *protocol.FaultClearResult, asJSON bool, stdout, stderr io.Writer) int {
	if asJSON {
		_ = json.NewEncoder(stdout).Encode(map[string]any{"cleared": res.Cleared, "finding": res.Finding})
	}
	if res.Cleared {
		if !asJSON {
			_, _ = fmt.Fprintln(stdout, "Delivery is open.")
		}
		return exitOK
	}
	if !asJSON {
		_, _ = fmt.Fprintf(stderr, "a38 fault clear: the lock stays. The cause is still present: %s\n"+
			"Fix the cause, then run \"a38 fault clear\" again. You can also restart the daemon.\n", dash(res.Finding))
	}
	return exitFailure
}
