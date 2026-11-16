package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/seletz/passierschein/internal/opguard"
)

// Exit codes of "a38 hook op-guard". They follow the hook contract of the
// agent tool, not the usage convention of a38: exit 2 blocks the command
// in the agent tool. A usage error therefore ends with exit 1, so that a
// wrong entry in the settings never blocks every command (fail-open, D15).
const (
	// hookAllow: the command may run; no output.
	hookAllow = 0
	// hookError: a38 could not do its work; the agent tool lets the
	// command run.
	hookError = 1
	// hookBlock: the guard blocks the command; the message is on stderr.
	hookBlock = 2
)

// hookStdin is the input of "a38 hook op-guard". Tests replace it.
var hookStdin io.Reader = os.Stdin

// hookExecutable returns the path of the running a38. Tests replace it.
var hookExecutable = func() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

const hookUsage = `Usage:
  a38 hook op-guard [--strict]      Read a hook input (JSON) on stdin and decide
  a38 hook op-guard --print-config [--scope project|user] [--strict]
                                    Print the hooks block for the settings.json of Claude Code

op-guard blocks a Bash command of the agent that runs the 1Password CLI "op"
or reads the 1Password token of a38. It is a hint, not a protection.
If the guard cannot inspect a command, it lets the command run. With
--strict it blocks such a command if the command has the word "op".
For all projects, use --print-config --scope user.
Exit codes: 0 allow (no output), 2 block (message for the agent on stderr),
1 error (the agent tool lets the command run).
`

// runHook runs a hook for an agent tool. "op-guard" is the only hook.
func runHook(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprint(stderr, hookUsage)
		return hookError
	}
	switch args[0] {
	case "op-guard":
		return runOpGuard(args[1:], stdout, stderr)
	case "--help", "-h", "help":
		_, _ = fmt.Fprint(stdout, hookUsage)
		return exitOK
	}
	_, _ = fmt.Fprintf(stderr, "a38 hook: unknown hook %q\n%s", args[0], hookUsage)
	return hookError
}

// runOpGuard decides on one hook input, or prints the configuration.
func runOpGuard(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("a38 hook op-guard", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	printConfig := fs.Bool("print-config", false, "print the hooks block for Claude Code")
	strict := fs.Bool("strict", false, "block a case of doubt if the command has the word op")
	scope := fs.String("scope", "project", "project or user")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			_, _ = fmt.Fprint(stdout, hookUsage)
			return exitOK
		}
		_, _ = fmt.Fprintf(stderr, "a38 hook op-guard: %v\n%s", err, hookUsage)
		return hookError
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintf(stderr, "a38 hook op-guard: unexpected argument %q\n%s", fs.Arg(0), hookUsage)
		return hookError
	}
	if *printConfig {
		return printOpGuardConfig(*scope, *strict, stdout, stderr)
	}
	if *scope != "project" {
		_, _ = fmt.Fprintf(stderr, "a38 hook op-guard: --scope needs --print-config\n%s", hookUsage)
		return hookError
	}

	d := opguard.Decide(hookStdin, opguard.Options{Strict: *strict})
	if d.Blocked() {
		_, _ = fmt.Fprint(stderr, opguard.Message(d.Rule))
		return hookBlock
	}
	if d.Note != "" {
		// The note never holds a part of the input.
		_, _ = fmt.Fprintf(stderr, "a38 hook op-guard: allowed without inspection: %s\n", d.Note)
	}
	return hookAllow
}

// printOpGuardConfig prints the hooks block. Project scope uses the bare
// name "a38" (the file can be shared); user scope uses the absolute path of
// the running a38 (independent of the PATH of the agent tool). With strict,
// the hook in the block has the switch --strict.
func printOpGuardConfig(scope string, strict bool, stdout, stderr io.Writer) int {
	command := "a38"
	switch scope {
	case "project":
	case "user":
		exe, err := hookExecutable()
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "a38 hook op-guard: cannot find the path of a38: %v\n", err)
			return exitFailure
		}
		command = exe
	default:
		_, _ = fmt.Fprintf(stderr, "a38 hook op-guard: --scope must be project or user, not %q\n", scope)
		return hookError
	}
	out, err := opguard.ClaudeCodeSettings(command, strict)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 hook op-guard: %v\n", err)
		return exitFailure
	}
	if _, err := stdout.Write(out); err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 hook op-guard: %v\n", err)
		return exitFailure
	}
	return exitOK
}
