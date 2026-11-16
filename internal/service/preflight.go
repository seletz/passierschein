package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/token"
	"github.com/seletz/passierschein/internal/tokenfile"
)

// Values of --backend and --ui of "a38 service install". The script UI is
// missing on purpose: it is a tool for tests in the foreground.
var (
	Backends = []string{"1password", "fake"}
	UIs      = []string{"app", "auto-approve", "auto-deny"}
)

// maxSocketPath is the limit of macOS for the path of a Unix socket.
const maxSocketPath = 103

// Options is the command line of "a38 service install".
type Options struct {
	// Backend is the backend of the daemon; the default is 1password.
	Backend string
	// UI is the approval UI; the default is app (D40).
	UI string
	// FakeFile is the absolute path of the file of the fake backend.
	FakeFile string
	// UIApp is the absolute path of the approval app; empty means the
	// search of a38 serve (D50).
	UIApp string
	// ConfigPath is the absolute path of the configuration file.
	ConfigPath string
	// ConfigGiven is true if the human named the file with --config or
	// A38_CONFIG. Only then the plist passes --config: launchd does not
	// pass the environment of the shell.
	ConfigGiven bool
	// Env holds the variables of [AllowedEnv] of the installing shell.
	Env map[string]string
	// Force replaces or removes a plist that a38 did not write.
	Force bool
	// DryRun checks and prints the plist, and changes nothing.
	DryRun bool
}

// Validate checks the combination of the flags (D40).
func (o Options) Validate() error {
	switch {
	case !slices.Contains(Backends, o.Backend):
		return fmt.Errorf("--backend must be one of: 1password, fake")
	case !slices.Contains(UIs, o.UI):
		return fmt.Errorf("--ui must be one of: app, auto-approve, auto-deny")
	case o.Backend != "fake" && o.UI != "app":
		return fmt.Errorf("--backend %s needs --ui app (D40)", o.Backend)
	case o.Backend == "fake" && o.FakeFile == "":
		return errors.New("--backend fake needs --fake-file")
	case o.Backend != "fake" && o.FakeFile != "":
		return errors.New("--fake-file needs --backend fake")
	case o.UIApp != "" && o.UI != "app":
		return errors.New("--ui-app needs --ui app")
	case !filepath.IsAbs(o.ConfigPath):
		return errors.New("the path of the configuration file must be absolute")
	}
	return nil
}

// ServeArgs returns the arguments of "a38 serve" for the plist.
func (o Options) ServeArgs() []string {
	args := []string{"serve", "--backend", o.Backend, "--ui", o.UI}
	if o.FakeFile != "" {
		args = append(args, "--fake-file", o.FakeFile)
	}
	if o.UIApp != "" {
		args = append(args, "--ui-app", o.UIApp)
	}
	if o.ConfigGiven {
		args = append(args, "--config", o.ConfigPath)
	}
	return args
}

// Finding is a reason not to install. It names the next step.
type Finding struct {
	// What is the problem.
	What string `json:"finding"`
	// Next is what the human does next.
	Next string `json:"next_step"`
}

// Probe reads the world for the preflight. A test passes fakes.
type Probe struct {
	// LoadConfig loads and checks the configuration file.
	LoadConfig func(path string) (*config.Config, error)
	// AppPath returns the path of the approval app as a38 serve finds it
	// with the configuration (D50).
	AppPath func(cfg *config.Config) string
	// TokenItem returns the metadata of the keychain item of the token,
	// never the token. nil means: the platform has no keychain.
	TokenItem func(account string) (token.StoreInfo, error)
	// DaemonAnswers reports whether a daemon answers on the admin socket.
	DaemonAnswers func(ctx context.Context, socket string) bool
	// UID is the user id.
	UID int
}

// Preflight is the outcome of the checks before an install.
type Preflight struct {
	// Findings stop the install.
	Findings []Finding `json:"findings"`
	// Warnings do not stop it.
	Warnings []string `json:"warnings"`
	// Config is the loaded configuration; nil if it does not load.
	Config *config.Config `json:"-"`
}

// Run checks that the daemon can start with the options: the
// configuration loads, the approval app passes the file checks, the token
// source is ready (metadata only, the token is never read), the socket
// paths are short enough and no other daemon answers. ownJobRuns is true if
// the LaunchAgent of this package runs already; an answering daemon is then
// this job.
func (p Probe) Run(ctx context.Context, o Options, ownJobRuns bool) Preflight {
	var r Preflight
	add := func(what, next string) { r.Findings = append(r.Findings, Finding{what, next}) }

	cfg, err := p.LoadConfig(o.ConfigPath)
	if err != nil {
		// T22 (part): a configuration that does not load stops the start
		// of the daemon, so it stops the install too.
		add(fmt.Sprintf("the configuration %s does not load: %v", o.ConfigPath, err),
			"create or fix the configuration file (a38 doctor names each finding), then run a38 service install again")
		return r
	}
	r.Config = cfg
	for _, s := range []string{cfg.AgentSocket(), cfg.AdminSocket()} {
		if len(s) > maxSocketPath {
			add(fmt.Sprintf("the socket path %s is longer than %d bytes", s, maxSocketPath),
				"set paths.socket_dir to a shorter directory in the configuration file")
		}
	}

	if o.UI == "app" {
		app := o.UIApp
		if app == "" {
			app = p.AppPath(cfg)
		}
		fi, err := os.Stat(app)
		if err == nil {
			err = approval.CheckAppFile(fi, p.UID)
		}
		if err != nil {
			add(fmt.Sprintf("the approval app %s cannot be used: %v", app, err),
				"install Passierschein.app in /Applications (how-to: Daemon als Dienst einrichten), or name the app with --ui-app or paths.ui_app")
		}
	}

	switch o.Backend {
	case "1password":
		p.tokenSource(cfg, &r)
	case "fake":
		if _, err := os.Stat(o.FakeFile); err != nil {
			add(fmt.Sprintf("the file of the fake backend cannot be read: %v", err), "name an existing file with --fake-file")
		}
	}

	if !ownJobRuns && p.DaemonAnswers(ctx, cfg.AdminSocket()) {
		add("a daemon answers on "+cfg.AdminSocket()+"; it is not the LaunchAgent",
			"stop the daemon that you started by hand (Ctrl-C in its terminal), then run a38 service install again")
	}
	return r
}

// tokenSource checks the token source of each account that a vault names,
// without reading a token (D72, D73). The daemon starts if at least one
// account has its token; it locks the vaults of an account without token.
// So the preflight refuses only if no account is ready. For each other
// account that is not ready it gives a warning.
func (p Probe) tokenSource(cfg *config.Config, r *Preflight) {
	var used []config.Account
	for _, a := range cfg.Backend.OnePassword.Accounts {
		if len(cfg.VaultsOf(a.Name)) > 0 {
			used = append(used, a)
		}
	}
	if len(used) == 0 {
		r.Findings = append(r.Findings, Finding{
			"the configuration names no vault with a 1Password account, so the 1Password backend has nothing to read",
			"add an account and a vault: a38 config account add NAME, then a38 config vault add VAULT --environment dev --account NAME"})
		return
	}
	var bad []Finding
	for _, a := range used {
		if f, ready := p.accountToken(a, r); !ready {
			bad = append(bad, f)
		}
	}
	if len(bad) == len(used) {
		r.Findings = append(r.Findings, bad...)
		return
	}
	for _, f := range bad {
		r.Warnings = append(r.Warnings, f.What+"; the daemon starts and locks the vaults of this account. Next step: "+f.Next)
	}
}

// accountToken checks the token source of one account. ready is false
// with a finding if the daemon cannot read the token at its start.
func (p Probe) accountToken(a config.Account, r *Preflight) (f Finding, ready bool) {
	if a.TokenSource == config.TokenSourceFile {
		if err := tokenfile.Check(a.TokenFile); err != nil {
			return Finding{
				fmt.Sprintf("account %q: the token file %s is not ready: %v (token_source: file)", a.Name, a.TokenFile, err),
				"write the token of the service account to the file with the mode 0600, or use the keychain: a38 config onepassword-token set --account " + a.Name}, false
		}
		return Finding{}, true
	}
	if p.TokenItem == nil {
		return Finding{"this build of a38 has no keychain", "use the macOS build of a38"}, false
	}
	info, err := p.TokenItem(a.Name)
	switch {
	case err != nil:
		return Finding{
			fmt.Sprintf("account %q: the keychain item of the token cannot be read: %v", a.Name, err),
			"run a38 config onepassword-token status --account " + a.Name + "; if the login keychain is locked, unlock it"}, false
	case !info.Exists:
		return Finding{
			fmt.Sprintf("account %q: the keychain has no token of the 1Password service account (token_source: keychain)", a.Name),
			"run a38 config onepassword-token set --account " + a.Name + " in this terminal, then run a38 service install again"}, false
	case info.AccessListError == "" && !info.ReadableWithoutDialog:
		r.Warnings = append(r.Warnings, fmt.Sprintf(
			"account %q: the keychain item may not trust this a38; the daemon may show a macOS dialog after the login and waits for it for 2 minutes. Store the token again with a38 config onepassword-token set --account %s --replace instead of confirming a dialog", a.Name, a.Name))
	}
	return Finding{}, true
}
