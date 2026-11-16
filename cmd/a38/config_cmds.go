package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/doctor"
)

// restartNote tells the human that a change needs a restart (D49: the
// daemon reads the file once at its start, there is no reload).
const restartNote = "A running daemon reads the configuration file only at its start. Restart it (a38 serve) so that it uses the change."

// stringList is a flag that can be set more than once.
type stringList []string

func (l *stringList) String() string     { return strings.Join(*l, ",") }
func (l *stringList) Set(v string) error { *l = append(*l, v); return nil }

// cmdTable is a group of subcommands with a usage text.
type cmdTable struct {
	name  string
	usage string
	cmds  []command
}

// run dispatches "a38 <group> <subcommand>".
func (g cmdTable) run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprint(stderr, g.usage)
		return exitUsage
	}
	switch args[0] {
	case "-h", "--help", "help":
		_, _ = fmt.Fprint(stdout, g.usage)
		return exitOK
	}
	for _, c := range g.cmds {
		if c.name == args[0] {
			return c.run(args[1:], stdout, stderr)
		}
	}
	_, _ = fmt.Fprintf(stderr, "a38 %s: unknown subcommand %q. Use \"a38 %s --help\".\n", g.name, args[0], g.name)
	return exitUsage
}

// parseCmd parses the flags and positional arguments of one subcommand.
// done is true if the command is finished: the code is then the exit code
// (help or a usage error).
func parseCmd(fs *flag.FlagSet, usage string, args []string, stdout, stderr io.Writer) (pos []string, code int, done bool) {
	for _, a := range args {
		if a == "--" {
			break
		}
		if a == "-h" || a == "--help" {
			_, _ = fmt.Fprint(stdout, usage)
			return nil, exitOK, true
		}
	}
	fs.Usage = func() {}
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "\n%s", usage)
		return nil, exitUsage, true
	}
	return pos, 0, false
}

// usageError prints a message and the usage text.
func usageError(stderr io.Writer, cmd, usage, msg string) int {
	_, _ = fmt.Fprintf(stderr, "a38 config %s: %s\n\n%s", cmd, msg, usage)
	return exitUsage
}

// fail prints an error of a command and returns the exit code 1.
func fail(stderr io.Writer, cmd string, err error) int {
	_, _ = fmt.Fprintf(stderr, "a38 config %s: %v\n", cmd, err)
	return exitFailure
}

// withConfig suffixes a command with --config if the path does not come
// from the standard places, so that the next step works for the same file.
func withConfig(loc config.Location, cmd string) string {
	if loc.Source == config.SourceFlag {
		return cmd + " --config " + loc.Path
	}
	return cmd
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// ---- path ----

const configPathUsage = `Usage:
  a38 config path [--config FILE] [--json]

Prints the path of the configuration file that applies and why. The
sequence is: the flag --config, the environment variable A38_CONFIG,
$XDG_CONFIG_HOME/a38/config.yaml, ~/.config/a38/config.yaml. It also says
whether the file exists. It exits with 0 even if the file is missing.
`

type pathDoc struct {
	Path   string `json:"path"`
	Source string `json:"source"`
	Why    string `json:"why"`
	Exists bool   `json:"exists"`
	Mode   string `json:"mode,omitempty"`
}

func runConfigPath(args []string, stdout, stderr io.Writer) int {
	var af adminFlags
	fs := newFlagSet("config path", stderr)
	af.register(fs)
	pos, code, done := parseCmd(fs, configPathUsage, args, stdout, stderr)
	if done {
		return code
	}
	if len(pos) != 0 {
		return usageError(stderr, "path", configPathUsage, "unexpected argument")
	}
	loc, err := config.Locate(af.configPath)
	if err != nil {
		return fail(stderr, "path", err)
	}
	doc := pathDoc{Path: loc.Path, Source: string(loc.Source), Why: loc.Why}
	if fi, err := os.Stat(loc.Path); err == nil {
		doc.Exists = true
		doc.Mode = fmt.Sprintf("%04o", fi.Mode().Perm())
	}
	if af.asJSON {
		if err := writeJSON(stdout, doc); err != nil {
			return fail(stderr, "path", err)
		}
		return exitOK
	}
	_, _ = fmt.Fprintf(stdout, "Configuration file: %s\nApplies because:   %s\n", doc.Path, doc.Why)
	if doc.Exists {
		_, _ = fmt.Fprintf(stdout, "Exists:            yes (mode %s)\n", doc.Mode)
	} else {
		_, _ = fmt.Fprintf(stdout, "Exists:            no. Create it with: %s\n", withConfig(loc, "a38 config init"))
	}
	return exitOK
}

// ---- show ----

const configShowUsage = `Usage:
  a38 config show [--config FILE] [--json]

Prints the effective configuration: each value with its origin (file or
default), the resolved paths, the projects with their paths (as written,
resolved, missing) and vaults, and the validation result. The file holds
no secret, and the command never reads the token.

A file that does not load gives exit code 1 and the error with its
position, never a partial output.
`

type showValue struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Origin string `json:"origin"`
}

type showPath struct {
	Written  string `json:"written"`
	Resolved string `json:"resolved"`
	Missing  bool   `json:"missing"`
}

type showProject struct {
	Name    string     `json:"name"`
	Valid   bool       `json:"valid"`
	Invalid string     `json:"invalid,omitempty"`
	Paths   []showPath `json:"paths"`
	Vaults  []string   `json:"vaults"`
}

type showVault struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
	Account     string `json:"account"`
}

// showAccount is one 1Password account (D71). It shows the token source,
// never a token.
type showAccount struct {
	Name        string   `json:"name"`
	TokenSource string   `json:"token_source"`
	TokenFile   string   `json:"token_file"`
	Vaults      []string `json:"vaults"`
}

type showDoc struct {
	Path       string            `json:"path"`
	Source     string            `json:"source"`
	Why        string            `json:"why"`
	Values     []showValue       `json:"values"`
	Resolved   map[string]string `json:"resolved"`
	Vaults     []showVault       `json:"vaults"`
	Projects   []showProject     `json:"projects"`
	Valid      bool              `json:"valid"`
	Problems   []string          `json:"problems"`
	Deprecated []string          `json:"deprecated"`
	Accounts   []showAccount     `json:"accounts"`
}

func runConfigShow(args []string, stdout, stderr io.Writer) int {
	var af adminFlags
	fs := newFlagSet("config show", stderr)
	af.register(fs)
	pos, code, done := parseCmd(fs, configShowUsage, args, stdout, stderr)
	if done {
		return code
	}
	if len(pos) != 0 {
		return usageError(stderr, "show", configShowUsage, "unexpected argument")
	}
	loc, data, cfg, err := loadConfigFile(af.configPath)
	if err != nil {
		return fail(stderr, "show", err)
	}
	in, err := config.Inspect(data)
	if err != nil {
		return fail(stderr, "show", err)
	}
	doc := buildShow(loc, cfg, in)
	if af.asJSON {
		if err := writeJSON(stdout, doc); err != nil {
			return fail(stderr, "show", err)
		}
		return exitOK
	}
	renderShow(stdout, doc, loc)
	return exitOK
}

// loadConfigFile reads and checks the file that applies. An error names
// the file; nothing is returned in part.
func loadConfigFile(flagPath string) (config.Location, []byte, *config.Config, error) {
	loc, err := config.Locate(flagPath)
	if err != nil {
		return loc, nil, nil, err
	}
	data, err := os.ReadFile(loc.Path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return loc, nil, nil, fmt.Errorf("there is no configuration file at %s (%s). Create it with: %s",
				loc.Path, loc.Why, withConfig(loc, "a38 config init"))
		}
		return loc, nil, nil, fmt.Errorf("cannot read %s: %w", loc.Path, err)
	}
	cfg, err := config.Parse(data)
	if err != nil {
		return loc, nil, nil, fmt.Errorf("the configuration file %s is not valid:\n%w", loc.Path, err)
	}
	return loc, data, cfg, nil
}

func buildShow(loc config.Location, cfg *config.Config, in config.Inspection) showDoc {
	origin := func(key string) string {
		if in.Present[key] {
			return "file"
		}
		return "default"
	}
	val := func(key, v string) showValue { return showValue{Key: key, Value: v, Origin: origin(key)} }
	uiApp := cfg.Paths.UIApp
	if uiApp == "" {
		uiApp = "(not set: a38 serve searches next to its own binary)"
	}
	doc := showDoc{
		Path: loc.Path, Source: string(loc.Source), Why: loc.Why,
		Values: []showValue{
			val("version", fmt.Sprint(config.FileVersion)),
			val("bundle.window", cfg.Bundle.Window.String()),
			val("bundle.max_entries", fmt.Sprint(cfg.Bundle.MaxEntries)),
			val("session_name.max_length", fmt.Sprint(cfg.SessionName.MaxLength)),
			val("session_name.allowed_characters", cfg.SessionName.AllowedCharacters),
			val("paths.socket_dir", cfg.Paths.SocketDir),
			val("paths.audit_file", cfg.Paths.AuditFile),
			val("paths.ui_app", uiApp),
		},
		Resolved: map[string]string{
			"agent_socket": cfg.AgentSocket(),
			"admin_socket": cfg.AdminSocket(),
			"audit_file":   cfg.Paths.AuditFile,
			"ui_app":       cfg.Paths.UIApp,
		},
		Vaults:     []showVault{},
		Projects:   []showProject{},
		Problems:   []string{},
		Deprecated: []string{},
		Accounts:   []showAccount{},
	}
	for _, a := range cfg.Backend.OnePassword.Accounts {
		vaults := cfg.VaultsOf(a.Name)
		if vaults == nil {
			vaults = []string{}
		}
		doc.Accounts = append(doc.Accounts, showAccount{Name: a.Name, TokenSource: string(a.TokenSource), TokenFile: a.TokenFile, Vaults: vaults})
	}
	// The environments and the limits of the exec way (D66). The deprecated
	// key grant.lifetime sets the lifetime of dev; its origin is the file.
	var env []showValue
	for _, e := range []config.Environment{config.EnvDev, config.EnvTest, config.EnvProd} {
		p := cfg.Policy(e)
		key := func(k string) string { return "environments." + string(e) + "." + k }
		lifetime := val(key("grant_lifetime"), p.GrantLifetime.String())
		if e == config.EnvDev && in.Present["grant.lifetime"] {
			lifetime.Origin = "file"
		}
		env = append(env,
			val(key("delivery"), config.FormatDelivery(p.Delivery)),
			lifetime,
			val(key("binding"), string(p.Binding)))
	}
	env = append(env,
		val("exec.default_timeout", cfg.Exec.DefaultTimeout.String()),
		val("exec.max_timeout", cfg.Exec.MaxTimeout.String()),
		val("exec.output_limit", config.FormatSize(cfg.Exec.OutputLimit)))
	doc.Values = slices.Insert(doc.Values, 1, env...)
	for _, d := range cfg.Deprecations {
		doc.Deprecated = append(doc.Deprecated, d.String())
	}
	for _, v := range cfg.Vaults {
		doc.Vaults = append(doc.Vaults, showVault{Name: v.Name, Environment: string(v.Environment), Account: v.Account})
	}
	for _, p := range cfg.Projects {
		sp := showProject{Name: p.Name, Valid: p.Invalid == "", Invalid: p.Invalid, Vaults: p.Vaults, Paths: []showPath{}}
		if sp.Vaults == nil {
			sp.Vaults = []string{}
		}
		written := in.Written[p.Name]
		for i, path := range p.Paths {
			w := path
			if i < len(written) {
				w = written[i]
			}
			missing := false
			for _, m := range p.MissingPaths {
				missing = missing || m == path
			}
			sp.Paths = append(sp.Paths, showPath{Written: w, Resolved: p.RealPaths[i], Missing: missing})
		}
		doc.Projects = append(doc.Projects, sp)
		if p.Invalid != "" {
			doc.Problems = append(doc.Problems, p.Invalid)
		}
	}
	doc.Valid = len(doc.Problems) == 0
	return doc
}

func renderShow(w io.Writer, d showDoc, loc config.Location) {
	p := func(format string, a ...any) { _, _ = fmt.Fprintf(w, format, a...) }
	p("Configuration file: %s\n  (%s)\n\nValues\n", d.Path, d.Why)
	for _, v := range d.Values {
		p("  %-34s %-9s %s\n", v.Key, "["+v.Origin+"]", v.Value)
	}
	p("\nResolved\n  agent socket   %s\n  admin socket   %s\n  audit file     %s\n",
		d.Resolved["agent_socket"], d.Resolved["admin_socket"], d.Resolved["audit_file"])
	p("\nAccounts (%d) (the tokens are never read here)\n", len(d.Accounts))
	for _, a := range d.Accounts {
		src := a.TokenSource
		if a.TokenSource == string(config.TokenSourceFile) {
			src += " " + a.TokenFile
		}
		p("  %-20s token_source %s\n", a.Name, src)
	}
	p("\nVaults (%d)\n", len(d.Vaults))
	for _, v := range d.Vaults {
		p("  %-30s %-5s account %s\n", v.Name, v.Environment, v.Account)
	}
	p("\nProjects (%d)\n", len(d.Projects))
	for _, pr := range d.Projects {
		state := "valid"
		if !pr.Valid {
			state = "INVALID"
		}
		p("  %s [%s]\n", pr.Name, state)
		for _, path := range pr.Paths {
			note := ""
			if path.Missing {
				note = "  (missing)"
			}
			if path.Written != path.Resolved {
				p("    path     %s -> %s%s\n", path.Written, path.Resolved, note)
			} else {
				p("    path     %s%s\n", path.Written, note)
			}
		}
		p("    vaults   %s\n", strings.Join(pr.Vaults, ", "))
		if pr.Invalid != "" {
			p("    problem  %s\n", pr.Invalid)
		}
	}
	for _, text := range d.Deprecated {
		p("\nDeprecated\n  %s\n", text)
	}
	p("\nValidation\n")
	if d.Valid {
		p("  The file loads. No invalid project.\n")
	} else {
		p("  The file loads, but %d project(s) are invalid. The daemon rejects their requests.\n", len(d.Problems))
		p("  Next step: %s, or fix the project in the file.\n", withConfig(loc, "a38 config vault add NAME --environment dev"))
	}
}

// ---- validate ----

const configValidateUsage = `Usage:
  a38 config validate [--config FILE] [--json]

Loads the configuration file and checks it with the rules of the daemon,
the same as the configuration check of "a38 doctor". It names each finding
with the next step. It exits with 0 if it finds nothing and with 1
otherwise.
`

func runConfigValidate(args []string, stdout, stderr io.Writer) int {
	var af adminFlags
	fs := newFlagSet("config validate", stderr)
	af.register(fs)
	pos, code, done := parseCmd(fs, configValidateUsage, args, stdout, stderr)
	if done {
		return code
	}
	if len(pos) != 0 {
		return usageError(stderr, "validate", configValidateUsage, "unexpected argument")
	}
	loc, err := config.Locate(af.configPath)
	if err != nil {
		return fail(stderr, "validate", err)
	}
	results := doctor.ConfigResults(context.Background(), doctor.SystemEnv{ConfigPath: loc.Path})
	rep := doctor.Report{OK: true, Results: results}
	for _, r := range results {
		if !r.OK {
			rep.OK = false
		}
	}
	if af.asJSON {
		if err := writeJSON(stdout, rep); err != nil {
			return fail(stderr, "validate", err)
		}
	} else {
		_, _ = fmt.Fprintf(stdout, "Configuration file: %s\n\n", loc.Path)
		doctor.Render(stdout, rep)
		if rep.OK {
			_, _ = fmt.Fprintln(stdout, "Next step: a38 doctor checks the daemon, the sockets and the token.")
		}
	}
	if !rep.OK {
		return exitFailure
	}
	return exitOK
}

// ---- init ----

const configInitUsage = `Usage:
  a38 config init [--force] [--config FILE]

Writes a new configuration file with the default values and short
comments at the path that "a38 config path" shows. The file names the
token source (keychain). The command creates the directory with the mode
0700 and the file with the mode 0600.

The command never overwrites a file: if the file exists, it fails and
names the path. With --force it first moves the old file to
config.yaml.bak-<timestamp>. It refuses to write through a symbolic link.
`

func runConfigInit(args []string, stdout, stderr io.Writer) int {
	var (
		af    adminFlags
		force bool
	)
	fs := newFlagSet("config init", stderr)
	af.register(fs)
	fs.BoolVar(&force, "force", false, "move an existing file to a backup and write a new one")
	pos, code, done := parseCmd(fs, configInitUsage, args, stdout, stderr)
	if done {
		return code
	}
	if len(pos) != 0 {
		return usageError(stderr, "init", configInitUsage, "unexpected argument")
	}
	loc, err := config.Locate(af.configPath)
	if err != nil {
		return fail(stderr, "init", err)
	}
	data, err := config.Template()
	if err != nil {
		return fail(stderr, "init", err)
	}
	backup, err := config.WriteNew(loc.Path, data, force, time.Now())
	if err != nil {
		if errors.Is(err, config.ErrExists) {
			_, _ = fmt.Fprintf(stderr, "a38 config init: the file %s exists. a38 does not overwrite it.\n"+
				"Use --force to move it to %s.bak-<timestamp> and write a new one, or change it with a38 config vault add and a38 config project add.\n",
				loc.Path, loc.Path)
			return exitFailure
		}
		return fail(stderr, "init", err)
	}
	if backup != "" {
		_, _ = fmt.Fprintf(stdout, "Moved the old file to %s\n", backup)
	}
	_, _ = fmt.Fprintf(stdout, "Created %s (mode 0600), because of %s.\n\nNext steps:\n", loc.Path, loc.Why)
	_, _ = fmt.Fprintf(stdout, "  1. %s\n", withConfig(loc, "a38 config account add ACCOUNT"))
	_, _ = fmt.Fprintf(stdout, "  2. %s\n", withConfig(loc, "a38 config vault add NAME --environment dev --account ACCOUNT"))
	_, _ = fmt.Fprintf(stdout, "  3. %s\n", withConfig(loc, "a38 config project add NAME --path DIR --vault NAME"))
	_, _ = fmt.Fprintf(stdout, "  4. %s   (token source keychain; for a file token: a38 config account add ACCOUNT --token-source file)\n", withConfig(loc, "a38 config onepassword-token set --account ACCOUNT"))
	_, _ = fmt.Fprintf(stdout, "  5. %s\n", withConfig(loc, "a38 config validate"))
	_, _ = fmt.Fprintf(stdout, "  6. %s\n", withConfig(loc, "a38 doctor"))
	return exitOK
}
