package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/seletz/passierschein/internal/config"
)

const configVaultUsage = `Usage:
  a38 config vault add NAME --environment dev|test|prod [--config FILE]
  a38 config vault remove NAME [--config FILE]
  a38 config vault list [--config FILE] [--json]

"add" and "remove" change the vault list in the configuration file. They
keep the comments, the order and the blank lines of the file. They check
the change with the rules of the daemon before they write it. They write
a temporary file next to the original and rename it, so a reader never sees
a part of the file. They refuse to write through a symbolic link, and they
fail if another program changes the file at the same time.

"add" fails if the name is in the list or the environment is not dev,
test or prod. "remove" fails if a project still uses the vault, and it
names the projects.

A running daemon reads the file only at its start. Restart it after a change.
`

const configProjectUsage = `Usage:
  a38 config project add NAME --path DIR [--path DIR ...] --vault VAULT [--vault VAULT ...] [--config FILE]
  a38 config project remove NAME [--config FILE]
  a38 config project list [--config FILE] [--json]

"add" and "remove" change the project list in the configuration file. They
keep the comments, the order and the blank lines of the file. They check
the change with the rules of the daemon before they write it.

Each --path must be absolute or start with "~/"; a38 writes the text as
you give it. A relative path is refused: it would mean another directory
for each working directory. A path that does not exist is accepted with
a note. Each --vault must be in the vault list (a38 config vault list).
The path of a project must not belong to another project.

A running daemon reads the file only at its start. Restart it after a change.
`

var vaultCommands = cmdTable{name: "config vault", usage: configVaultUsage, cmds: []command{
	{"add", "Add a vault to the vault list", runVaultAdd},
	{"remove", "Remove a vault from the vault list", runVaultRemove},
	{"list", "List the vaults", runVaultList},
}}

var projectCommands = cmdTable{name: "config project", usage: configProjectUsage, cmds: []command{
	{"add", "Add a project", runProjectAdd},
	{"remove", "Remove a project", runProjectRemove},
	{"list", "List the projects", runProjectList},
}}

func runVault(args []string, stdout, stderr io.Writer) int {
	return vaultCommands.run(args, stdout, stderr)
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
		af  adminFlags
		env string
	)
	fs := newFlagSet("config vault add", stderr)
	af.register(fs)
	fs.StringVar(&env, "environment", "", "environment of the vault: dev, test or prod")
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
	name := pos[0]
	return applyEdit("vault add", af.configPath, stdout, stderr, func(b []byte) (config.Edit, error) {
		return config.AddVault(b, name, config.Environment(env))
	}, fmt.Sprintf("Added the vault %q (environment %s)", name, env))
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
		vaults = append(vaults, showVault{Name: v.Name, Environment: string(v.Environment)})
	}
	if af.asJSON {
		if err := writeJSON(stdout, vaults); err != nil {
			return fail(stderr, "vault list", err)
		}
		return exitOK
	}
	if len(vaults) == 0 {
		_, _ = fmt.Fprintln(stdout, "No vaults. Add one with: a38 config vault add NAME --environment dev")
		return exitOK
	}
	for _, v := range vaults {
		_, _ = fmt.Fprintf(stdout, "%-30s %s\n", v.Name, v.Environment)
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
