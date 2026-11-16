package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/seletz/passierschein/internal/config"
)

const configVaultUsage = `Usage:
  a38 config vault add NAME --environment dev|test|prod --account ACCOUNT [--config FILE]
  a38 config vault remove NAME [--config FILE]
  a38 config vault list [--config FILE] [--json]

"add" and "remove" change the vault list in the configuration file. They
keep the comments, the order and the blank lines of the file. They check
the change with the rules of the daemon before they write it. They write
a temporary file next to the original and rename it, so a reader never sees
a part of the file. They refuse to write through a symbolic link, and they
fail if another program changes the file at the same time.

"add" fails if the name is in the list, the environment is not dev, test
or prod, or the account is not in backend.onepassword.accounts. Each vault
names its 1Password account; there is no default account (D70). "remove"
fails if a project still uses the vault, and it names the projects.

A running daemon reads the file only at its start. Restart it after a change.
`

const configAccountUsage = `Usage:
  a38 config account add NAME [--token-source keychain|file] [--token-file FILE] [--config FILE]
  a38 config account remove NAME [--config FILE]
  a38 config account list [--config FILE] [--json]

"add" and "remove" change the list backend.onepassword.accounts in the
configuration file, with the same rules as "a38 config vault" (D71). Each
account has its own 1Password service account and token.

NAME has 1 to 32 characters: lower-case letters, digits and hyphens. The
name "service-account" is reserved. The token source is keychain (the
default) or file; --token-file is allowed only with the source file and
has the default ~/.config/a38/onepassword-token-NAME.

"remove" fails if a vault still names the account. It does not delete the
keychain item; run "a38 config onepassword-token delete --account NAME".

"list" shows each account with its token source and its vaults. It never
reads a token.

After "add", store the token: a38 config onepassword-token set --account NAME
A running daemon reads the file only at its start. Restart it after a change.
`

const configProjectUsage = `Usage:
  a38 config project add NAME --path DIR [--path DIR ...] --vault VAULT [--vault VAULT ...] [--config FILE]
  a38 config project add-path NAME --path DIR [--path DIR ...] [--config FILE]
  a38 config project remove NAME [--config FILE]
  a38 config project list [--config FILE] [--json]

"add", "add-path" and "remove" change the project list in the configuration
file. They keep the comments, the order and the blank lines of the file. They check
the change with the rules of the daemon before they write it.

Each --path must be absolute or start with "~/"; a38 writes the text as
you give it. A relative path is refused: it would mean another directory
for each working directory. A path that does not exist is accepted with
a note. Each --vault must be in the vault list (a38 config vault list).
The path of a project must not belong to another project.

"add-path" adds paths to a project that is in the list already. Use it for
the directory of the worktrees next to the repository: a session in a
worktree matches the project only if the worktree directory is a path of
the project. It fails if the project is not in the list, if the project has
the path already, or if another project has it.

A running daemon reads the file only at its start. Restart it after a change.
`

var vaultCommands = cmdTable{name: "config vault", usage: configVaultUsage, cmds: []command{
	{"add", "Add a vault to the vault list", runVaultAdd},
	{"remove", "Remove a vault from the vault list", runVaultRemove},
	{"list", "List the vaults", runVaultList},
}}

var accountCommands = cmdTable{name: "config account", usage: configAccountUsage, cmds: []command{
	{"add", "Add a 1Password account", runAccountAdd},
	{"remove", "Remove a 1Password account", runAccountRemove},
	{"list", "List the 1Password accounts", runAccountList},
}}

var projectCommands = cmdTable{name: "config project", usage: configProjectUsage, cmds: []command{
	{"add", "Add a project", runProjectAdd},
	{"add-path", "Add a path to a project", runProjectAddPath},
	{"remove", "Remove a project", runProjectRemove},
	{"list", "List the projects", runProjectList},
}}

func runVault(args []string, stdout, stderr io.Writer) int {
	return vaultCommands.run(args, stdout, stderr)
}
func runAccount(args []string, stdout, stderr io.Writer) int {
	return accountCommands.run(args, stdout, stderr)
}
func runProject(args []string, stdout, stderr io.Writer) int {
	return projectCommands.run(args, stdout, stderr)
}

// applyEdit reads the file, runs the change, and writes the result. It
// prints the notes and the restart hint.
func applyEdit(cmd string, configPath string, stdout, stderr io.Writer, change func([]byte) (config.Edit, error), done string) int {
	loc, err := config.Locate(configPath)
	if err != nil {
		return fail(stderr, cmd, err)
	}
	old, err := os.ReadFile(loc.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return fail(stderr, cmd, fmt.Errorf("there is no configuration file at %s. Create it with: %s", loc.Path, withConfig(loc, "a38 config init")))
		}
		return fail(stderr, cmd, err)
	}
	edit, err := change(old)
	if err != nil {
		return fail(stderr, cmd, err)
	}
	if err := config.Replace(loc.Path, old, edit.Data); err != nil {
		return fail(stderr, cmd, err)
	}
	_, _ = fmt.Fprintf(stdout, "%s in %s.\n", done, loc.Path)
	for _, n := range edit.Notes {
		_, _ = fmt.Fprintf(stdout, "Note: %s\n", n)
	}
	_, _ = fmt.Fprintln(stdout, restartNote)
	return exitOK
}

func runVaultAdd(args []string, stdout, stderr io.Writer) int {
	var (
		af           adminFlags
		env, account string
	)
	fs := newFlagSet("config vault add", stderr)
	af.register(fs)
	fs.StringVar(&env, "environment", "", "environment of the vault: dev, test or prod")
	fs.StringVar(&account, "account", "", "1Password account of the vault (backend.onepassword.accounts)")
	pos, code, done := parseCmd(fs, configVaultUsage, args, stdout, stderr)
	if done {
		return code
	}
	if len(pos) != 1 {
		return usageError(stderr, "vault add", configVaultUsage, "give exactly one vault name")
	}
	if env == "" {
		return usageError(stderr, "vault add", configVaultUsage, "--environment is necessary (dev, test or prod)")
	}
	if account == "" {
		return usageError(stderr, "vault add", configVaultUsage, "--account is necessary; each vault names its 1Password account (a38 config account list)")
	}
	name := pos[0]
	return applyEdit("vault add", af.configPath, stdout, stderr, func(b []byte) (config.Edit, error) {
		return config.AddVault(b, name, config.Environment(env), account)
	}, fmt.Sprintf("Added the vault %q (environment %s, account %s)", name, env, account))
}

func runVaultRemove(args []string, stdout, stderr io.Writer) int {
	var af adminFlags
	fs := newFlagSet("config vault remove", stderr)
	af.register(fs)
	pos, code, done := parseCmd(fs, configVaultUsage, args, stdout, stderr)
	if done {
		return code
	}
	if len(pos) != 1 {
		return usageError(stderr, "vault remove", configVaultUsage, "give exactly one vault name")
	}
	name := pos[0]
	return applyEdit("vault remove", af.configPath, stdout, stderr, func(b []byte) (config.Edit, error) {
		return config.RemoveVault(b, name)
	}, fmt.Sprintf("Removed the vault %q", name))
}

func runVaultList(args []string, stdout, stderr io.Writer) int {
	var af adminFlags
	fs := newFlagSet("config vault list", stderr)
	af.register(fs)
	pos, code, done := parseCmd(fs, configVaultUsage, args, stdout, stderr)
	if done {
		return code
	}
	if len(pos) != 0 {
		return usageError(stderr, "vault list", configVaultUsage, "unexpected argument")
	}
	_, _, cfg, err := loadConfigFile(af.configPath)
	if err != nil {
		return fail(stderr, "vault list", err)
	}
	vaults := []showVault{}
	for _, v := range cfg.Vaults {
		vaults = append(vaults, showVault{Name: v.Name, Environment: string(v.Environment), Account: v.Account})
	}
	if af.asJSON {
		if err := writeJSON(stdout, vaults); err != nil {
			return fail(stderr, "vault list", err)
		}
		return exitOK
	}
	if len(vaults) == 0 {
		_, _ = fmt.Fprintln(stdout, "No vaults. Add one with: a38 config vault add NAME --environment dev --account ACCOUNT")
		return exitOK
	}
	_, _ = fmt.Fprintf(stdout, "%-30s %-11s %s\n", "VAULT", "ENVIRONMENT", "ACCOUNT")
	for _, v := range vaults {
		_, _ = fmt.Fprintf(stdout, "%-30s %-11s %s\n", v.Name, v.Environment, v.Account)
	}
	return exitOK
}

func runAccountAdd(args []string, stdout, stderr io.Writer) int {
	var (
		af                adminFlags
		source, tokenFile string
	)
	fs := newFlagSet("config account add", stderr)
	af.register(fs)
	fs.StringVar(&source, "token-source", string(config.DefaultTokenSource), "source of the token: keychain or file")
	fs.StringVar(&tokenFile, "token-file", "", "token file of the account (only with --token-source file)")
	pos, code, done := parseCmd(fs, configAccountUsage, args, stdout, stderr)
	if done {
		return code
	}
	if len(pos) != 1 {
		return usageError(stderr, "account add", configAccountUsage, "give exactly one account name")
	}
	name := pos[0]
	rc := applyEdit("account add", af.configPath, stdout, stderr, func(b []byte) (config.Edit, error) {
		return config.AddAccount(b, name, config.TokenSource(source), tokenFile)
	}, fmt.Sprintf("Added the account %q (token source %s)", name, source))
	if rc == exitOK && config.TokenSource(source) == config.TokenSourceKeychain {
		_, _ = fmt.Fprintf(stdout, "Next step: store the token of the account in a terminal: a38 config onepassword-token set --account %s\n", name)
	}
	return rc
}

func runAccountRemove(args []string, stdout, stderr io.Writer) int {
	var af adminFlags
	fs := newFlagSet("config account remove", stderr)
	af.register(fs)
	pos, code, done := parseCmd(fs, configAccountUsage, args, stdout, stderr)
	if done {
		return code
	}
	if len(pos) != 1 {
		return usageError(stderr, "account remove", configAccountUsage, "give exactly one account name")
	}
	name := pos[0]
	rc := applyEdit("account remove", af.configPath, stdout, stderr, func(b []byte) (config.Edit, error) {
		return config.RemoveAccount(b, name)
	}, fmt.Sprintf("Removed the account %q", name))
	if rc == exitOK {
		_, _ = fmt.Fprintf(stdout, "The keychain item of the account stays. Delete it with: a38 config onepassword-token delete --account %s\n", name)
	}
	return rc
}

func runAccountList(args []string, stdout, stderr io.Writer) int {
	var af adminFlags
	fs := newFlagSet("config account list", stderr)
	af.register(fs)
	pos, code, done := parseCmd(fs, configAccountUsage, args, stdout, stderr)
	if done {
		return code
	}
	if len(pos) != 0 {
		return usageError(stderr, "account list", configAccountUsage, "unexpected argument")
	}
	_, _, cfg, err := loadConfigFile(af.configPath)
	if err != nil {
		return fail(stderr, "account list", err)
	}
	accounts := []showAccount{}
	for _, a := range cfg.Backend.OnePassword.Accounts {
		vaults := cfg.VaultsOf(a.Name)
		if vaults == nil {
			vaults = []string{}
		}
		accounts = append(accounts, showAccount{Name: a.Name, TokenSource: string(a.TokenSource), TokenFile: a.TokenFile, Vaults: vaults})
	}
	if af.asJSON {
		if err := writeJSON(stdout, accounts); err != nil {
			return fail(stderr, "account list", err)
		}
		return exitOK
	}
	if len(accounts) == 0 {
		_, _ = fmt.Fprintln(stdout, "No accounts. Add one with: a38 config account add NAME")
		return exitOK
	}
	_, _ = fmt.Fprintf(stdout, "%-20s %-9s %s\n", "ACCOUNT", "SOURCE", "VAULTS")
	for _, a := range accounts {
		vaults := strings.Join(a.Vaults, ", ")
		if vaults == "" {
			vaults = "(none: the daemon does not read its token)"
		}
		_, _ = fmt.Fprintf(stdout, "%-20s %-9s %s\n", a.Name, a.TokenSource, vaults)
	}
	return exitOK
}

func runProjectAdd(args []string, stdout, stderr io.Writer) int {
	var (
		af            adminFlags
		paths, vaults stringList
	)
	fs := newFlagSet("config project add", stderr)
	af.register(fs)
	fs.Var(&paths, "path", "directory of the project (repeat for more); absolute or starting with ~/")
	fs.Var(&vaults, "vault", "vault that the project can use (repeat for more)")
	pos, code, done := parseCmd(fs, configProjectUsage, args, stdout, stderr)
	if done {
		return code
	}
	if len(pos) != 1 {
		return usageError(stderr, "project add", configProjectUsage, "give exactly one project name")
	}
	name := pos[0]
	return applyEdit("project add", af.configPath, stdout, stderr, func(b []byte) (config.Edit, error) {
		return config.AddProject(b, name, paths, vaults)
	}, fmt.Sprintf("Added the project %q (paths: %s; vaults: %s)", name, strings.Join(paths, ", "), strings.Join(vaults, ", ")))
}

func runProjectAddPath(args []string, stdout, stderr io.Writer) int {
	var (
		af    adminFlags
		paths stringList
	)
	fs := newFlagSet("config project add-path", stderr)
	af.register(fs)
	fs.Var(&paths, "path", "directory to add (repeat for more); absolute or starting with ~/")
	pos, code, done := parseCmd(fs, configProjectUsage, args, stdout, stderr)
	if done {
		return code
	}
	if len(pos) != 1 {
		return usageError(stderr, "project add-path", configProjectUsage, "give exactly one project name")
	}
	if len(paths) == 0 {
		return usageError(stderr, "project add-path", configProjectUsage, "--path is necessary")
	}
	name := pos[0]
	return applyEdit("project add-path", af.configPath, stdout, stderr, func(b []byte) (config.Edit, error) {
		return config.AddProjectPath(b, name, paths)
	}, fmt.Sprintf("Added the paths %s to the project %q", strings.Join(paths, ", "), name))
}

func runProjectRemove(args []string, stdout, stderr io.Writer) int {
	var af adminFlags
	fs := newFlagSet("config project remove", stderr)
	af.register(fs)
	pos, code, done := parseCmd(fs, configProjectUsage, args, stdout, stderr)
	if done {
		return code
	}
	if len(pos) != 1 {
		return usageError(stderr, "project remove", configProjectUsage, "give exactly one project name")
	}
	name := pos[0]
	return applyEdit("project remove", af.configPath, stdout, stderr, func(b []byte) (config.Edit, error) {
		return config.RemoveProject(b, name)
	}, fmt.Sprintf("Removed the project %q", name))
}

func runProjectList(args []string, stdout, stderr io.Writer) int {
	var af adminFlags
	fs := newFlagSet("config project list", stderr)
	af.register(fs)
	pos, code, done := parseCmd(fs, configProjectUsage, args, stdout, stderr)
	if done {
		return code
	}
	if len(pos) != 0 {
		return usageError(stderr, "project list", configProjectUsage, "unexpected argument")
	}
	loc, data, cfg, err := loadConfigFile(af.configPath)
	if err != nil {
		return fail(stderr, "project list", err)
	}
	in, err := config.Inspect(data)
	if err != nil {
		return fail(stderr, "project list", err)
	}
	projects := buildShow(loc, cfg, in).Projects
	if af.asJSON {
		if err := writeJSON(stdout, projects); err != nil {
			return fail(stderr, "project list", err)
		}
		return exitOK
	}
	if len(projects) == 0 {
		_, _ = fmt.Fprintln(stdout, "No projects. Add one with: a38 config project add NAME --path DIR --vault VAULT")
		return exitOK
	}
	for _, p := range projects {
		state := "valid"
		if !p.Valid {
			state = "INVALID: " + p.Invalid
		}
		var paths []string
		for _, pp := range p.Paths {
			paths = append(paths, pp.Written)
		}
		_, _ = fmt.Fprintf(stdout, "%s [%s]\n  paths:  %s\n  vaults: %s\n", p.Name, state, strings.Join(paths, ", "), strings.Join(p.Vaults, ", "))
	}
	return exitOK
}
