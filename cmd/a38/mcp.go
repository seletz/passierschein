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
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/seletz/passierschein/internal/client"
	"github.com/seletz/passierschein/internal/config"
	a38mcp "github.com/seletz/passierschein/internal/mcp"
	"github.com/seletz/passierschein/internal/version"
)

// envSessionName is the environment variable with the start value of the
// session name (D25, G19). The flag --session-name has priority.
const envSessionName = "A38_SESSION_NAME"

// runMCP runs the MCP server for one agent session on stdin and stdout
// until the agent tool closes stdin or SIGINT or SIGTERM arrives.
func runMCP(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runMCPWith(ctx, args, &mcp.StdioTransport{}, stderr, os.Getenv)
}

// mcpFlags holds the command line of "a38 mcp".
type mcpFlags struct {
	config      string
	sessionName string
}

// parseMCPFlags reads the command line. The session name follows the
// flag, then the environment, else it is empty.
func parseMCPFlags(args []string, stderr io.Writer, getenv func(string) string) (mcpFlags, error) {
	var f mcpFlags
	fs := flag.NewFlagSet("a38 mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&f.config, "config", "", "path of the configuration file (default: $"+config.EnvConfig+", else $XDG_CONFIG_HOME/a38/config.yaml)")
	fs.StringVar(&f.sessionName, "session-name", "", "name of the session in the agent tool (default: $"+envSessionName+", else no name)")
	if err := fs.Parse(args); err != nil {
		return f, err
	}
	if fs.NArg() > 0 {
		return f, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	given := false
	fs.Visit(func(fl *flag.Flag) { given = given || fl.Name == "session-name" })
	if !given {
		f.sessionName = getenv(envSessionName)
	}
	return f, nil
}

// runMCPWith serves MCP on the transport. Stdout carries only protocol
// traffic, so the log goes to stderr.
func runMCPWith(ctx context.Context, args []string, t mcp.Transport, stderr io.Writer, getenv func(string) string) int {
	f, err := parseMCPFlags(args, stderr, getenv)
	if errors.Is(err, flag.ErrHelp) {
		return exitOK
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 mcp: %v\n", err)
		return exitUsage
	}
	// D49: the flag has priority over A38_CONFIG.
	path := f.config
	if path == "" {
		if path, err = config.DefaultPath(); err != nil {
			_, _ = fmt.Fprintf(stderr, "a38 mcp: %v\n", err)
			return exitFailure
		}
	}
	cfg, err := config.Load(path)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 mcp: %v\n", err)
		return exitFailure
	}
	socket := cfg.AgentSocket()
	srv := a38mcp.New(a38mcp.Options{
		Dial:        func(ctx context.Context) (*client.Conn, error) { return client.DialAgent(ctx, socket) },
		SessionName: f.sessionName,
		Version:     version.Version,
		Logger:      slog.New(slog.NewTextHandler(stderr, nil)),
		// D64: run_with_credentials accepts only a cwd in the project of
		// the working directory of a38 mcp. The daemon checks again.
		ProjectFor: func(dir string) (string, bool) {
			p, ok := cfg.ProjectFor(dir)
			return p.Name, ok
		},
	})
	if err := srv.Run(ctx, t); err != nil && ctx.Err() == nil {
		_, _ = fmt.Fprintf(stderr, "a38 mcp: %v\n", err)
		return exitFailure
	}
	return exitOK
}
