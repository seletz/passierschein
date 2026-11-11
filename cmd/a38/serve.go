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

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/approval/fakeui"
	"github.com/seletz/passierschein/internal/approval/queue"
	"github.com/seletz/passierschein/internal/audit/sqlite"
	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/backend/fake"
	"github.com/seletz/passierschein/internal/broker"
	"github.com/seletz/passierschein/internal/clock"
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
	config   string
	backend  string
	ui       string
	fakeFile string
	script   string
}

// parseServeFlags reads and checks the command line.
func parseServeFlags(args []string, stderr io.Writer) (serveFlags, error) {
	var f serveFlags
	fs := flag.NewFlagSet("a38 serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&f.config, "config", "", "path of the configuration file (default: $"+config.EnvConfig+", else $XDG_CONFIG_HOME/a38/config.yaml)")
	fs.StringVar(&f.backend, "backend", "", "backend: "+strings.Join(backends, ", "))
	fs.StringVar(&f.ui, "ui", "", "approval UI: "+strings.Join(uis, ", "))
	fs.StringVar(&f.fakeFile, "fake-file", "", "YAML file of the fake backend (with --backend fake)")
	fs.StringVar(&f.script, "ui-script", "", "program and arguments of the approval script, split at white space (with --ui script)")
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
	case f.backend == "fake" && f.fakeFile == "":
		return f, errors.New("--backend fake needs --fake-file")
	case f.ui == "script" && strings.TrimSpace(f.script) == "":
		return f, errors.New("--ui script needs --ui-script")
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
	b, closeAll, err := newBroker(f, cfg, log)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 serve: %v\n", err)
		return exitFailure
	}
	defer closeAll()
	srv, err := daemon.New(daemon.Options{Config: cfg, Agent: b, Admin: b, Logger: log})
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
	log.Info("daemon started",
		"config", path, "backend", f.backend, "ui", f.ui,
		"agent_socket", srv.AgentSocket(), "admin_socket", srv.AdminSocket())
	sweepCtx, stopSweep := context.WithCancel(ctx)
	defer stopSweep()
	go b.Run(sweepCtx)
	if err := srv.Serve(ctx); err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 serve: %v\n", err)
		return exitFailure
	}
	log.Info("daemon stopped")
	return exitOK
}

// newBroker builds the backend, the approval UI, the audit log and the
// broker. It opens no socket. The returned function closes the audit log.
// Each failure stops the start: the daemon does not run without an audit
// log (D36).
func newBroker(f serveFlags, cfg *config.Config, log *slog.Logger) (*broker.Broker, func(), error) {
	be, err := newBackend(f)
	if err != nil {
		return nil, nil, err
	}
	mode := approval.Mode(f.ui)
	if err := fakeui.CheckCombination(backend.Kind(f.backend).Real(), mode); err != nil {
		return nil, nil, err
	}
	clk := clock.System{}
	ui, err := fakeui.New(mode, fakeui.Options{ScriptCommand: strings.Fields(f.script), Clock: clk})
	if err != nil {
		return nil, nil, err
	}
	store, err := sqlite.Open(cfg.Paths.AuditFile)
	if err != nil {
		return nil, nil, err
	}
	closeAll := func() {
		if err := store.Close(); err != nil {
			log.Error("cannot close the audit log", "error", err)
		}
	}
	// The queue shows one approval request at a time and ends it after
	// the wait time (D30).
	b := broker.New(cfg, be, queue.New(ui, clk, 0), store, clk, log, broker.WithUIMode(mode))
	return b, closeAll, nil
}

// newBackend returns the backend of the flag --backend.
func newBackend(f serveFlags) (backend.Backend, error) {
	switch backend.Kind(f.backend) {
	case backend.KindFake:
		return fake.Load(f.fakeFile)
	default:
		return nil, fmt.Errorf("--backend %s is not implemented in stage 1", f.backend)
	}
}
