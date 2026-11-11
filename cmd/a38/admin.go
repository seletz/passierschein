package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/seletz/passierschein/internal/client"
	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/protocol"
)

// adminTimeout limits one admin command.
const adminTimeout = 15 * time.Second

// adminFlags are the flags that each admin command has.
type adminFlags struct {
	configPath string
	asJSON     bool
}

func (a *adminFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&a.configPath, "config", "", "configuration file (default: the standard location)")
	fs.BoolVar(&a.asJSON, "json", false, "print JSON")
}

// adminSession is one connection to the admin socket.
type adminSession struct {
	conn *client.Conn
	ctx  context.Context
	end  func()
}

func (s *adminSession) close() {
	_ = s.conn.Close()
	s.end()
}

// dialAdmin loads the configuration and connects to the admin socket. A
// failure is a message for the human: it names what is wrong and what to
// do.
func dialAdmin(configPath string) (*adminSession, error) {
	if configPath == "" {
		var err error
		if configPath, err = config.DefaultPath(); err != nil {
			return nil, err
		}
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), adminTimeout)
	conn, err := client.DialAdmin(ctx, cfg.AdminSocket())
	if err != nil {
		cancel()
		return nil, fmt.Errorf("the daemon does not run, or it uses another configuration (%v). Start it with \"a38 serve\"", errors.Unwrap(err))
	}
	return &adminSession{conn: conn, ctx: ctx, end: cancel}, nil
}

// adminError returns the text for a failed request.
func adminError(err error) string {
	var perr *protocol.Error
	if errors.As(err, &perr) {
		return fmt.Sprintf("%s (%s)", perr.Message, perr.Code)
	}
	if errors.Is(err, client.ErrClosed) {
		return "the daemon closed the connection"
	}
	return err.Error()
}

// newFlagSet returns a flag set for the subcommand "a38 name".
func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("a38 "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}
