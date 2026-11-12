// Command a38 is the command-line interface of Passierschein, a credential
// broker for coding agents.
//
// a38 is one binary with subcommands. Each subcommand has its own file in
// this package; the file holds one function with the signature of
// [command.run]. This file holds the list of commands only.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/seletz/passierschein/internal/version"
)

// Exit codes of a38.
const (
	// exitOK: the command completed.
	exitOK = 0
	// exitFailure: the command failed.
	exitFailure = 1
	// exitUsage: the command line is wrong.
	exitUsage = 2
)

// command is one subcommand of a38.
type command struct {
	// name is the word on the command line.
	name string
	// summary is one line for the usage text.
	summary string
	// run executes the command with the arguments after its name and
	// returns the exit code.
	run func(args []string, stdout, stderr io.Writer) int
}

// commands lists the subcommands in the sequence of the usage text.
var commands = []command{
	{"serve", "Run the daemon: agent socket, admin socket, approval and audit log", runServe},
	{"mcp", "Run the MCP server for one agent session (stdio)", runMCP},
	{"grants", "List and revoke grants", runGrants},
	{"fault", "Show and lift the global delivery lock", runFault},
	{"audit", "List the audit log, verify its hash chain and print its figures (stats)", runAudit},
	{"tui", "Show approvals and deliveries of the audit log in the terminal (read-only, macOS)", runTUI},
	{"config", "Manage the configuration: the token of the 1Password service account (onepassword-token)", runConfig},
	{"doctor", "Check the state of a38 and name the next step for each finding", runDoctor},
	{"llm", "Print the instructions for agents (--skill)", runLLM},
	{"version", "Print the version", runVersion},
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the command line and returns the exit code.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return exitUsage
	}
	switch args[0] {
	case "--version":
		return runVersion(nil, stdout, stderr)
	case "--help", "-h", "help":
		usage(stdout)
		return exitOK
	}
	for _, cmd := range commands {
		if cmd.name == args[0] {
			return cmd.run(args[1:], stdout, stderr)
		}
	}
	_, _ = fmt.Fprintf(stderr, "a38: unknown command %q. Use \"a38 --help\".\n", args[0])
	return exitUsage
}

// usage prints the usage text.
func usage(w io.Writer) {
	_, _ = fmt.Fprint(w, "a38 is a credential broker for coding agents.\n\nUsage:\n  a38 <command> [arguments]\n\nCommands:\n")
	for _, cmd := range commands {
		_, _ = fmt.Fprintf(w, "  %-8s %s\n", cmd.name, cmd.summary)
	}
	_, _ = fmt.Fprint(w, "\nFlags:\n  --help     Print this text\n  --version  Print the version\n")
}

// runVersion prints the version line.
func runVersion(_ []string, stdout, _ io.Writer) int {
	_, _ = fmt.Fprintln(stdout, version.String())
	return exitOK
}
