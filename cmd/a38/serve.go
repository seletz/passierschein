package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/daemon"
)

// Values of the flags --backend (D38) and --ui (D40).
var (
	backends = []string{"fake", "1password"}
	uis      = []string{"app", "auto-approve", "auto-deny", "script"}
)

// runServe runs the daemon until SIGINT or SIGTERM.
func runServe(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return serve(ctx, args, stdout, stderr)
}

// serveFlags holds the command line of "a38 serve".
type serveFlags struct {
	config  string
	backend string
	ui      string
}

// parseServeFlags reads and checks the command line.
func parseServeFlags(args []string, stderr io.Writer) (serveFlags, error) {
	var f serveFlags
	fs := flag.NewFlagSet("a38 serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&f.config, "config", "", "path of the configuration file (default: $"+config.EnvConfig+", else $XDG_CONFIG_HOME/a38/config.yaml)")
	fs.StringVar(&f.backend, "backend", "", "backend: "+strings.Join(backends, ", "))
	fs.StringVar(&f.ui, "ui", "", "approval UI: "+strings.Join(uis, ", "))
	if err := fs.Parse(args); err != nil {
		return f, err
	}
	switch {
	case fs.NArg() > 0:
		return f, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	case !slices.Contains(backends, f.backend):
		return f, fmt.Errorf("--backend must be one of: %s", strings.Join(backends, ", "))
	case !slices.Contains(uis, f.ui):
		return f, fmt.Errorf("--ui must be one of: %s", strings.Join(uis, ", "))
	case f.backend != "fake" && f.ui != "app":
		// D40: a fake approval is possible only with the fake backend.
		return f, fmt.Errorf("--backend %s needs --ui app", f.backend)
	}
	return f, nil
}

// serve runs the daemon until ctx ends.
func serve(ctx context.Context, args []string, _, stderr io.Writer) int {
	f, err := parseServeFlags(args, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return exitOK
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 serve: %v\n", err)
		return exitUsage
	}
	// D49: the flag has priority over A38_CONFIG.
	path := f.config
	if path == "" {
		if path, err = config.DefaultPath(); err != nil {
			_, _ = fmt.Fprintf(stderr, "a38 serve: %v\n", err)
			return exitFailure
		}
	}
	cfg, err := config.Load(path)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 serve: %v\n", err)
		return exitFailure
	}

	log := slog.New(slog.NewTextHandler(stderr, nil))
	// The broker does not exist yet: each request ends with daemon_faulted
	// (fail closed). --backend and --ui are checked, but no backend and
	// no UI is built.
	broker := daemon.Faulted{}
	srv, err := daemon.New(daemon.Options{Config: cfg, Agent: broker, Admin: broker, Logger: log})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 serve: %v\n", err)
		return exitFailure
	}
	if err := srv.Listen(); err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 serve: %v\n", err)
		return exitFailure
	}
	for _, p := range cfg.InvalidProjects() {
		log.Warn("project configuration is invalid", "project", p.Name, "finding", p.Invalid)
	}
	log.Info("daemon started; no broker yet: each request ends with daemon_faulted",
		"config", path, "backend", f.backend, "ui", f.ui,
		"agent_socket", srv.AgentSocket(), "admin_socket", srv.AdminSocket())
	if err := srv.Serve(ctx); err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 serve: %v\n", err)
		return exitFailure
	}
	log.Info("daemon stopped")
	return exitOK
}
