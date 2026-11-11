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
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/approval/appui"
	"github.com/seletz/passierschein/internal/approval/fakeui"
	"github.com/seletz/passierschein/internal/approval/queue"
	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/backend/fake"
	"github.com/seletz/passierschein/internal/broker"
	"github.com/seletz/passierschein/internal/clock"
	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/daemon"
	"github.com/seletz/passierschein/internal/fault"
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
	uiApp    string
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
	fs.StringVar(&f.uiApp, "ui-app", "", "path of the approval app (with --ui app; default: paths.ui_app of the configuration, else next to the a38 binary)")
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
	case f.uiApp != "" && f.ui != "app":
		return f, errors.New("--ui-app needs --ui app")
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
	b, lock, closeAll, err := newBroker(f, cfg, log)
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
	// D48, T28: a restart with the cause still present leads into the
	// lock again. The daemon starts locked and does not refuse to start,
	// so that a38 doctor and the admin commands can reach it.
	if err := lock.SelfCheck(ctx); err != nil {
		log.Error("the daemon starts with the global lock; run a38 doctor")
	}
	log.Info("daemon started",
		"config", path, "backend", f.backend, "ui", f.ui,
		"agent_socket", srv.AgentSocket(), "admin_socket", srv.AdminSocket())
	sweepCtx, stopSweep := context.WithCancel(ctx)
	defer stopSweep()
	go b.Run(sweepCtx)
	go lock.Run(sweepCtx)
	// T16: on SIGINT or SIGTERM the broker ends the live grants with the
	// reason daemon_stopped before the connections close (a closed
	// connection would end them with session_ended) and before the audit
	// log closes. SIGKILL and a crash write nothing.
	serveCtx, stopServe := context.WithCancel(context.Background())
	defer stopServe()
	go func() {
		select {
		case <-ctx.Done():
			b.Shutdown()
		case <-serveCtx.Done():
		}
		stopServe()
	}()
	err = srv.Serve(serveCtx)
	b.Shutdown() // a grant that a request created during the stop
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 serve: %v\n", err)
		return exitFailure
	}
	log.Info("daemon stopped")
	return exitOK
}

// newBroker builds the backend, the approval UI, the audit log, the global
// lock and the broker. It opens no socket. The returned function closes
// the audit log. A failure of the backend or the UI stops the start. An
// audit file that cannot be opened does not stop it: the self-check of
// the lock then locks delivery (D36, D48).
func newBroker(f serveFlags, cfg *config.Config, log *slog.Logger) (*broker.Broker, *fault.Global, func(), error) {
	be, err := newBackend(f)
	if err != nil {
		return nil, nil, nil, err
	}
	mode := approval.Mode(f.ui)
	if err := fakeui.CheckCombination(backend.Kind(f.backend).Real(), mode); err != nil {
		return nil, nil, nil, err
	}
	clk := clock.System{}
	var ui approval.UI
	var closeUI func()
	appPath := ""
	if mode == approval.ModeApp {
		appPath = resolveUIApp(f.uiApp, cfg.Paths.UIApp, executablePath)
		app, err := appui.New(appui.Options{Path: appPath, Clock: clk, Logger: log})
		if err != nil {
			return nil, nil, nil, err
		}
		// A missing or bad app does not stop the daemon: each request
		// then ends with ui_failed and a38 doctor names the finding.
		if err := app.Check(); err != nil {
			log.Error("the approval app cannot be used; each approval request ends with ui_failed", "path", appPath, "error", err)
		}
		ui, closeUI = app, app.Close
	} else if ui, err = fakeui.New(mode, fakeui.Options{ScriptCommand: strings.Fields(f.script), Clock: clk}); err != nil {
		return nil, nil, nil, err
	}
	store, err := openAuditFile(cfg.Paths.AuditFile)
	if err != nil {
		log.Error("cannot open the audit log", "error", err)
	}
	closeAll := func() {
		if closeUI != nil {
			closeUI() // ends the apps of the notices that still run
		}
		if err := store.Close(); err != nil {
			log.Error("cannot close the audit log", "error", err)
		}
	}
	// The queue shows one approval request at a time and ends it after
	// the wait time (D30). A notice passes the queue at once.
	q := queue.New(ui, clk, 0)
	lock := fault.New(fault.Options{Store: store, UI: q, Clock: clk, Logger: log})
	b := broker.New(cfg, be, q, store, clk, log, broker.WithUIMode(mode), broker.WithUIApp(appPath), broker.WithLock(lock))
	return b, lock, closeAll, nil
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

// DefaultAppNames lists the file names that the daemon looks for next to
// its own binary, in this order, if neither the flag nor the configuration
// names the approval app. The paths are relative to the directory of the
// binary.
var DefaultAppNames = []string{
	"a38-approve",
	filepath.Join("a38-approve.app", "Contents", "MacOS", "a38-approve"),
}

// executablePath returns the path of the running binary with links
// resolved, or an empty string.
func executablePath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return exe
}

// resolveUIApp returns the path of the approval app: the flag, else the
// configuration, else the first of [DefaultAppNames] next to the binary
// that exists. If none exists it returns the first name, so that the
// message names a path.
func resolveUIApp(flagValue, configured string, exe func() string) string {
	switch {
	case flagValue != "":
		if abs, err := filepath.Abs(flagValue); err == nil {
			return abs // the app runs with another working directory
		}
		return flagValue
	case configured != "":
		return configured
	}
	self := exe()
	if self == "" {
		return DefaultAppNames[0] // relative: the file check refuses it
	}
	dir := filepath.Dir(self)
	for _, n := range DefaultAppNames {
		if _, err := os.Stat(filepath.Join(dir, n)); err == nil {
			return filepath.Join(dir, n)
		}
	}
	return filepath.Join(dir, DefaultAppNames[0])
}
