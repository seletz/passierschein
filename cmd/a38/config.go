package main

import (
	"fmt"
	"io"
)

// configCommands lists the subcommands of "a38 config" in the sequence of
// the usage text.
var configCommands = []command{
	{"path", "Show which configuration file applies and why", runConfigPath},
	{"show", "Show the effective configuration with the origin of each value", runConfigShow},
	{"init", "Write a new configuration file with the default values", runConfigInit},
	{"validate", "Check the configuration file with the rules of the daemon", runConfigValidate},
	{"vault", "Add, remove and list vaults in the configuration file", runVault},
	{"project", "Add, remove and list projects in the configuration file", runProject},
	{"onepassword-token", "Store, show and delete the token of the 1Password service account (macOS keychain)", runOnePasswordToken},
}

// runConfig dispatches "a38 config <subcommand>".
func runConfig(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		configUsage(stderr)
		return exitUsage
	}
	switch args[0] {
	case "-h", "--help", "help":
		configUsage(stdout)
		return exitOK
	}
	for _, cmd := range configCommands {
		if cmd.name == args[0] {
			return cmd.run(args[1:], stdout, stderr)
		}
	}
	_, _ = fmt.Fprintf(stderr, "a38 config: unknown subcommand %q. Use \"a38 config --help\".\n", args[0])
	return exitUsage
}

// configUsage prints the usage text of "a38 config".
func configUsage(w io.Writer) {
	_, _ = fmt.Fprint(w, "Usage:\n  a38 config <subcommand> [arguments]\n\nSubcommands:\n")
	for _, cmd := range configCommands {
		_, _ = fmt.Fprintf(w, "  %-18s %s\n", cmd.name, cmd.summary)
	}
	_, _ = fmt.Fprint(w, "\nUse \"a38 config <subcommand> --help\" for the arguments of a subcommand.\n")
}
