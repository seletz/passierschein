//go:build darwin

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/term"

	"github.com/seletz/passierschein/internal/audit/sqlite"
	"github.com/seletz/passierschein/internal/auditstats"
	"github.com/seletz/passierschein/internal/client"
	"github.com/seletz/passierschein/internal/clock"
	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/tui"
)

const tuiUsage = `Usage:
  a38 tui [--config FILE] [--file AUDIT] [--range R] [--refresh DURATION]

The view reads the audit log only. It never writes. With 1Password
accounts in the configuration it asks the daemon for its status and shows
the state of each account. R is today, 7d (default), 30d or all. --refresh reads the audit log
again after the interval, for example 30s; 0 (default) reads it only on
the key "r".
`

// runTUI shows the audit log in the terminal (F13, UC6). Only the macOS
// build has the view: the audit log lives on the Mac with the daemon (D53).
func runTUI(args []string, stdout, stderr io.Writer) int {
	var (
		af        auditFlags
		rangeName string
		refresh   time.Duration
	)
	fs := flag.NewFlagSet("a38 tui", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { _, _ = fmt.Fprint(stderr, tuiUsage) }
	af.register(fs)
	fs.StringVar(&rangeName, "range", string(auditstats.Range7Days), "time range at the start: today, 7d, 30d or all")
	fs.DurationVar(&refresh, "refresh", 0, "read the audit log again after this interval (0: only on the key r)")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return exitUsage
	}
	if len(pos) != 0 {
		_, _ = fmt.Fprintln(stderr, "a38 tui: unexpected argument")
		return exitUsage
	}
	rng, err := auditstats.ParseRange(rangeName)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 tui: --range: %v\n", err)
		return exitUsage
	}
	if refresh < 0 || (refresh > 0 && refresh < time.Second) {
		_, _ = fmt.Fprintln(stderr, "a38 tui: --refresh must be 0 or at least 1s")
		return exitUsage
	}
	path, err := af.path()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 tui: %v\n", err)
		return exitFailure
	}
	// Check the file before the screen changes, so that an error stays
	// readable.
	s, err := sqlite.OpenReadOnly(path)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 tui: %v\n", err)
		return exitFailure
	}
	_ = s.Close()
	out, ok := stdout.(*os.File)
	if !ok || !term.IsTerminal(out.Fd()) || !term.IsTerminal(os.Stdin.Fd()) {
		_, _ = fmt.Fprintln(stderr, `a38 tui: needs a terminal; use "a38 audit list" or "a38 audit stats" in a script`)
		return exitFailure
	}
	r := auditReader{path: path}
	m := tui.New(tui.Options{
		Source:   tui.Source{Name: displayPath(path), Load: r.Load, Verify: r.Verify, Accounts: af.accountsSource()},
		Clock:    clock.System{},
		Location: time.Local,
		Range:    rng,
		Refresh:  refresh,
	})
	opts := []tea.ProgramOption{tea.WithOutput(out), tea.WithInput(os.Stdin)}
	if os.Getenv("NO_COLOR") != "" {
		// https://no-color.org: each non-empty value switches the colours
		// off. Bold and reverse stay.
		opts = append(opts, tea.WithColorProfile(colorprofile.Ascii))
	}
	if _, err := tea.NewProgram(m, opts...).Run(); err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 tui: %v\n", err)
		return exitFailure
	}
	return exitOK
}

// accountsSource returns the source of the account states for a38 tui
// (D78). It is nil if the configuration does not load or names no
// 1Password account.
func (a *auditFlags) accountsSource() func(context.Context) ([]tui.AccountState, error) {
	cfgPath := a.configPath
	if cfgPath == "" {
		var err error
		if cfgPath, err = config.DefaultPath(); err != nil {
			return nil
		}
	}
	cfg, err := config.Load(cfgPath)
	if err != nil || len(cfg.Backend.OnePassword.Accounts) == 0 {
		return nil
	}
	return tuiAccounts(cfg.AdminSocket())
}

// tuiAccounts asks the daemon at the admin socket for its status and
// returns the state of each 1Password account. It only reads.
func tuiAccounts(socket string) func(context.Context) ([]tui.AccountState, error) {
	return func(ctx context.Context) ([]tui.AccountState, error) {
		ctx, cancel := context.WithTimeout(ctx, adminTimeout)
		defer cancel()
		c, err := client.DialAdmin(ctx, socket)
		if err != nil {
			return nil, err
		}
		defer func() { _ = c.Close() }()
		st, err := c.Status(ctx)
		if err != nil {
			return nil, err
		}
		return accountStatesOf(st), nil
	}
}

// accountStatesOf returns the state of each account in the status. It is
// empty for another backend and for an old daemon.
func accountStatesOf(st *protocol.StatusResult) []tui.AccountState {
	if st == nil || st.Backend == nil {
		return nil
	}
	var out []tui.AccountState
	for _, a := range st.Backend.Accounts {
		out = append(out, tui.AccountState{Name: a.Name, State: a.State, Reason: a.Reason, Configured: a.VaultsConfigured, Visible: a.VaultsVisible})
	}
	return out
}
