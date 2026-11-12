//go:build darwin

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/seletz/passierschein/internal/client"
	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/doctor"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/service"
	"github.com/seletz/passierschein/internal/token"
)

const serviceUsage = `Usage:
  a38 service install [--backend 1password|fake] [--ui app|auto-approve|auto-deny]
                      [--fake-file FILE] [--ui-app PATH] [--config FILE] [--force] [--dry-run]
  a38 service uninstall [--force]
  a38 service restart
  a38 service status [--config FILE] [--json]

The service is a LaunchAgent: launchd starts "a38 serve" with each login
of the user and never restarts it by itself. The default of install is
--backend 1password --ui app. Install checks first that the daemon can
start; it never reads the token. The plist names the real path of this
a38. The log is ~/Library/Logs/a38/serve.log.
`

// serviceProbeTimeout limits the question to the daemon on the admin
// socket.
const serviceProbeTimeout = 3 * time.Second

// serviceDeps is the world of "a38 service". A test passes fakes: a
// temporary HOME and a fake launchctl.
type serviceDeps struct {
	// manager returns the manager of the LaunchAgent.
	manager func() (service.Manager, error)
	// probe returns the probe of the preflight.
	probe func() service.Probe
	// daemon asks the daemon of the configuration file for its status.
	daemon func(ctx context.Context, configPath string) (*protocol.StatusResult, string, error)
}

// systemServiceDeps returns the real world.
func systemServiceDeps() serviceDeps {
	return serviceDeps{manager: systemServiceManager, probe: systemServiceProbe, daemon: daemonStatus}
}

// systemServiceManager returns the manager for the user with the real
// launchctl and the real path of this a38.
func systemServiceManager() (service.Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) {
		return service.Manager{}, errors.New("cannot find the home directory of the user")
	}
	lc, err := service.NewLaunchctl()
	if err != nil {
		return service.Manager{}, err
	}
	exe, err := os.Executable()
	if err != nil {
		return service.Manager{}, err
	}
	self, err := service.ResolveProgram(exe)
	if err != nil {
		return service.Manager{}, err
	}
	return service.Manager{Layout: service.Layout{Home: home, UID: os.Getuid()}, Launchctl: lc, Self: self}, nil
}

// systemServiceProbe returns the real probe. It reads only the metadata
// of the keychain item (D51).
func systemServiceProbe() service.Probe {
	p := service.Probe{
		LoadConfig: config.Load,
		AppPath: func(cfg *config.Config) string {
			return resolveUIApp("", cfg.Paths.UIApp, executablePath, homeDir)
		},
		DaemonAnswers: func(ctx context.Context, socket string) bool {
			ctx, cancel := context.WithTimeout(ctx, serviceProbeTimeout)
			defer cancel()
			c, err := client.DialAdmin(ctx, socket)
			if err != nil {
				return false
			}
			defer func() { _ = c.Close() }()
			_, err = c.Status(ctx)
			return err == nil
		},
		UID: os.Getuid(),
	}
	if store, err := newTokenStore(); err == nil {
		p.TokenItem = func() (token.StoreInfo, error) { return store.Describe() }
	}
	return p
}

// daemonStatus asks the daemon of the configuration file on the admin
// socket. It returns the socket path for the output.
func daemonStatus(ctx context.Context, configPath string) (*protocol.StatusResult, string, error) {
	if configPath == "" {
		var err error
		if configPath, err = config.DefaultPath(); err != nil {
			return nil, "", err
		}
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, "", err
	}
	ctx, cancel := context.WithTimeout(ctx, serviceProbeTimeout)
	defer cancel()
	c, err := client.DialAdmin(ctx, cfg.AdminSocket())
	if err != nil {
		return nil, cfg.AdminSocket(), err
	}
	defer func() { _ = c.Close() }()
	st, err := c.Status(ctx)
	return st, cfg.AdminSocket(), err
}

// newServiceState gives a38 doctor the state of the LaunchAgent. A test
// replaces it, so that no test asks the real launchd.
var newServiceState = serviceDoctorState

// serviceDoctorState gives a38 doctor the state of the LaunchAgent. Without
// a plist it does not call launchctl: the service is not installed.
func serviceDoctorState(ctx context.Context) *doctor.ServiceState {
	m, err := systemServiceManager()
	if err != nil {
		return &doctor.ServiceState{Error: err.Error()}
	}
	if _, err := os.Lstat(m.Layout.PlistPath()); errors.Is(err, os.ErrNotExist) {
		return &doctor.ServiceState{PlistPath: m.Layout.PlistPath(), LogPath: m.Layout.LogPath()}
	}
	return doctorState(m.Status(ctx))
}

// doctorState converts the status for the doctor.
func doctorState(s service.Status) *doctor.ServiceState {
	return &doctor.ServiceState{
		PlistPath: s.PlistPath, Installed: s.Installed, Foreign: s.Foreign,
		Program: s.Program, ProgramMissing: s.ProgramMissing, Current: s.Current, ProgramDiffers: s.ProgramDiffers,
		Loaded: s.Loaded, Disabled: s.Disabled, Running: s.Running, PID: s.PID,
		LastExit: s.LastExitMeaning, LogPath: s.LogPath, Error: s.LaunchctlError,
	}
}

// runService dispatches "a38 service <subcommand>".
func runService(args []string, stdout, stderr io.Writer) int {
	return runServiceWith(args, stdout, stderr, systemServiceDeps())
}

func runServiceWith(args []string, stdout, stderr io.Writer, deps serviceDeps) int {
	if len(args) == 0 {
		_, _ = fmt.Fprint(stderr, serviceUsage)
		return exitUsage
	}
	switch args[0] {
	case "install":
		return serviceInstall(args[1:], stdout, stderr, deps)
	case "uninstall":
		return serviceUninstall(args[1:], stdout, stderr, deps)
	case "restart":
		return serviceRestart(args[1:], stdout, stderr, deps)
	case "status":
		return serviceStatus(args[1:], stdout, stderr, deps)
	case "-h", "--help", "help":
		_, _ = fmt.Fprint(stdout, serviceUsage)
		return exitOK
	}
	_, _ = fmt.Fprintf(stderr, "a38 service: unknown subcommand %q\n\n%s", args[0], serviceUsage)
	return exitUsage
}

// parseServiceFlags parses the flags of a subcommand. It allows no
// positional argument.
func parseServiceFlags(fs *flag.FlagSet, args []string, stderr io.Writer) int {
	pos, err := parseInterspersed(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		return exitOK
	}
	if err != nil {
		return exitUsage
	}
	if len(pos) > 0 {
		_, _ = fmt.Fprintf(stderr, "%s: unexpected argument %q\n\n%s", fs.Name(), pos[0], serviceUsage)
		return exitUsage
	}
	return -1
}

// absOrEmpty returns the absolute form of a path, or "".
func absOrEmpty(p string) (string, error) {
	if p == "" {
		return "", nil
	}
	return filepath.Abs(p)
}

func serviceInstall(args []string, stdout, stderr io.Writer, deps serviceDeps) int {
	o := service.Options{Env: map[string]string{}}
	var cfgFlag string
	fs := newFlagSet("service install", stderr)
	fs.StringVar(&o.Backend, "backend", "1password", "backend of the daemon: 1password or fake")
	fs.StringVar(&o.UI, "ui", "app", "approval UI: app, or with --backend fake also auto-approve or auto-deny")
	fs.StringVar(&o.FakeFile, "fake-file", "", "YAML file of the fake backend (with --backend fake)")
	fs.StringVar(&o.UIApp, "ui-app", "", "path of the approval app (default: the search of a38 serve)")
	fs.StringVar(&cfgFlag, "config", "", "configuration file (default: $A38_CONFIG, else the standard location)")
	fs.BoolVar(&o.Force, "force", false, "replace a plist that a38 did not write")
	fs.BoolVar(&o.DryRun, "dry-run", false, "check and print the plist; change nothing")
	if code := parseServiceFlags(fs, args, stderr); code >= 0 {
		return code
	}
	var err error
	if o.FakeFile, err = absOrEmpty(o.FakeFile); err == nil {
		o.UIApp, err = absOrEmpty(o.UIApp)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 service install: %v\n", err)
		return exitUsage
	}
	// launchd does not pass the environment of this shell. A file that the
	// human named with --config or A38_CONFIG goes into the plist; the XDG
	// variables go into EnvironmentVariables (no secret, D49).
	switch {
	case cfgFlag != "":
		o.ConfigPath, err = filepath.Abs(cfgFlag)
		o.ConfigGiven = true
	case os.Getenv(config.EnvConfig) != "":
		o.ConfigPath, err = config.DefaultPath()
		o.ConfigGiven = true
	default:
		o.ConfigPath, err = config.DefaultPath()
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 service install: %v\n", err)
		return exitFailure
	}
	for _, k := range service.AllowedEnv {
		if v := os.Getenv(k); filepath.IsAbs(v) {
			o.Env[k] = v
		}
	}
	if err := o.Validate(); err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 service install: %v\n", err)
		return exitUsage
	}
	m, err := deps.manager()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 service install: %v\n", err)
		return exitFailure
	}
	r, err := m.Install(context.Background(), o, deps.probe())
	for _, w := range r.Warnings {
		_, _ = fmt.Fprintf(stderr, "WARNING  %s\n", w)
	}
	for _, f := range r.Preflight.Findings {
		_, _ = fmt.Fprintf(stderr, "FINDING  %s\n         Next step: %s\n", clean(f.What), clean(f.Next))
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 service install: %v\n", err)
		return exitFailure
	}
	if o.DryRun {
		_, _ = stdout.Write(r.Plist)
		_, _ = fmt.Fprintf(stdout, "\nDry run: the checks passed. a38 service install without --dry-run writes %s and loads it.\n", r.PlistPath)
		return exitOK
	}
	if r.Unchanged {
		_, _ = fmt.Fprintf(stdout, "The LaunchAgent %s is installed and loaded with this plist already. Nothing changed.\n", service.Label)
		return exitOK
	}
	_, _ = fmt.Fprintf(stdout, "Installed the LaunchAgent %s.\n", service.Label)
	_, _ = fmt.Fprintf(stdout, "  Plist:   %s\n  Program: %s %s\n  Log:     %s\n\n", r.PlistPath, m.Self, strings.Join(o.ServeArgs(), " "), m.Layout.LogPath())
	_, _ = fmt.Fprint(stdout, "macOS may show the notification \"Background Items Added\". System Settings > General >\n"+
		"Login Items lists the item under \"Allow in the Background\"; keep it switched on.\n"+
		"launchd starts the daemon now and with each login. Check it: a38 service status, then a38 doctor.\n")
	return exitOK
}

func serviceUninstall(args []string, stdout, stderr io.Writer, deps serviceDeps) int {
	var force bool
	fs := newFlagSet("service uninstall", stderr)
	fs.BoolVar(&force, "force", false, "remove a plist that a38 did not write")
	if code := parseServiceFlags(fs, args, stderr); code >= 0 {
		return code
	}
	m, err := deps.manager()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 service uninstall: %v\n", err)
		return exitFailure
	}
	r, err := m.Uninstall(context.Background(), force)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 service uninstall: %v\n", err)
		return exitFailure
	}
	if !r.WasInstalled {
		_, _ = fmt.Fprintln(stdout, "The LaunchAgent is not installed. Nothing changed.")
		return exitOK
	}
	if r.Unloaded {
		_, _ = fmt.Fprintln(stdout, "Stopped the daemon (SIGTERM; the daemon ends its grants) and unloaded the LaunchAgent.")
	}
	if r.Removed {
		_, _ = fmt.Fprintf(stdout, "Removed %s.\n", m.Layout.PlistPath())
	}
	_, _ = fmt.Fprintf(stdout, "The log %s stays.\n", m.Layout.LogPath())
	return exitOK
}

func serviceRestart(args []string, stdout, stderr io.Writer, deps serviceDeps) int {
	fs := newFlagSet("service restart", stderr)
	if code := parseServiceFlags(fs, args, stderr); code >= 0 {
		return code
	}
	m, err := deps.manager()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 service restart: %v\n", err)
		return exitFailure
	}
	if err := m.Restart(context.Background()); err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 service restart: %v\n", err)
		return exitFailure
	}
	_, _ = fmt.Fprintln(stdout, "Restarted the daemon. It read the configuration and the token again; all grants ended. Check it: a38 service status.")
	return exitOK
}

// serviceStatusJSON is the output of "a38 service status --json".
type serviceStatusJSON struct {
	Service service.Status  `json:"service"`
	Daemon  serviceDaemonJS `json:"daemon"`
	OK      bool            `json:"ok"`
}

type serviceDaemonJS struct {
	Socket  string                 `json:"admin_socket,omitempty"`
	Answers bool                   `json:"answers"`
	Error   string                 `json:"error,omitempty"`
	Status  *protocol.StatusResult `json:"status,omitempty"`
}

func serviceStatus(args []string, stdout, stderr io.Writer, deps serviceDeps) int {
	var af adminFlags
	fs := newFlagSet("service status", stderr)
	af.register(fs)
	if code := parseServiceFlags(fs, args, stderr); code >= 0 {
		return code
	}
	m, err := deps.manager()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 service status: %v\n", err)
		return exitFailure
	}
	ctx := context.Background()
	s := m.Status(ctx)
	cfgPath := af.configPath
	if cfgPath == "" {
		cfgPath = s.ConfigPath // the file of the plist, if it names one
	}
	var d serviceDaemonJS
	st, sock, err := deps.daemon(ctx, cfgPath)
	d.Socket = sock
	if err != nil {
		d.Error = err.Error()
	} else {
		d.Answers, d.Status = true, st
	}
	code := exitOK
	if !s.OK() {
		code = exitFailure
	}
	if af.asJSON {
		if err := json.NewEncoder(stdout).Encode(serviceStatusJSON{Service: s, Daemon: d, OK: s.OK()}); err != nil {
			_, _ = fmt.Fprintf(stderr, "a38 service status: %v\n", err)
			return exitFailure
		}
		return code
	}
	renderServiceStatus(stdout, s, d)
	return code
}

// renderServiceStatus writes the status as text.
func renderServiceStatus(w io.Writer, s service.Status, d serviceDaemonJS) {
	line := func(k, format string, a ...any) {
		if k != "" {
			k += ":"
		}
		_, _ = fmt.Fprintf(w, "%-10s %s\n", k, clean(fmt.Sprintf(format, a...)))
	}
	switch {
	case !s.Installed:
		line("Service", "not installed (%s missing); install it with a38 service install", s.PlistPath)
	case s.Foreign:
		line("Service", "a plist with the label exists, but a38 service install did not write it: %s", s.PlistPath)
	default:
		line("Service", "installed: %s", s.PlistPath)
	}
	if s.PlistError != "" {
		line("Plist", "cannot be read: %s", s.PlistError)
	}
	if s.Program != "" {
		line("Program", "%s %s", s.Program, strings.Join(s.Arguments, " "))
		switch {
		case s.ProgramMissing:
			line("", "WARNING: the program does not exist; run a38 service install with the a38 to use")
		case s.ProgramDiffers:
			line("", "WARNING: this a38 is %s; run a38 service install if this one is right", s.Current)
		default:
			line("", "the same a38 as this command")
		}
	}
	switch {
	case s.LaunchctlError != "":
		line("launchd", "no answer: %s", s.LaunchctlError)
	case s.Running:
		line("launchd", "loaded, running, pid %d", s.PID)
	case s.Loaded:
		last := "no exit known"
		if s.LastExitMeaning != "" {
			last = "last exit: " + s.LastExitMeaning
		}
		line("launchd", "loaded, not running (%s); launchd does not restart it; fix the cause, then a38 service restart", last)
	default:
		line("launchd", "not loaded")
	}
	if s.Disabled {
		line("", "WARNING: disabled; allow the item in System Settings > General > Login Items")
	}
	if s.LogSize >= 0 {
		line("Log", "%s (%d bytes)", s.LogPath, s.LogSize)
	} else {
		line("Log", "%s (missing)", s.LogPath)
	}
	switch {
	case d.Answers && d.Status != nil:
		lock := "delivery open"
		if f := d.Status.Fault; f != nil {
			lock = "delivery LOCKED (" + string(f.Code) + "); run a38 doctor, then a38 fault clear"
		}
		line("Daemon", "answers on %s: version %s, %d sessions, %d grants, %s", d.Socket, d.Status.DaemonVersion, d.Status.Sessions, d.Status.Grants, lock)
	case d.Socket != "":
		line("Daemon", "does not answer on %s", d.Socket)
	default:
		line("Daemon", "unknown: %s", d.Error)
	}
	if s.OK() {
		_, _ = fmt.Fprintln(w, "\nThe service runs.")
	} else {
		_, _ = fmt.Fprintln(w, "\nThe service does not run as installed. a38 doctor names the next step.")
	}
}

// clean replaces control characters in a text from a file or from launchd.
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if protocol.IsControl(r) {
			return '?'
		}
		return r
	}, s)
}
