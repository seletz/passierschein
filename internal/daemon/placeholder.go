package daemon

import (
	"context"

	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// Faulted is a handler for both sockets that answers each request with
// daemon_faulted. "a38 serve" uses it until the broker exists: the daemon
// then delivers nothing (fail closed).
type Faulted struct{}

// errNoBroker is the answer of [Faulted].
func errNoBroker() *protocol.Error {
	return protocol.Errorf(protocol.CodeDaemonFaulted, "the daemon has no broker; delivery is locked")
}

// RequestCredentials returns daemon_faulted.
func (Faulted) RequestCredentials(context.Context, session.Session, *protocol.RequestCredentials) (*protocol.Credentials, *protocol.Error) {
	return nil, errNoBroker()
}

// GrantStatus returns daemon_faulted.
func (Faulted) GrantStatus(context.Context, session.Session, *protocol.GrantStatus) (*protocol.Grants, *protocol.Error) {
	return nil, errNoBroker()
}

// SessionEnded does nothing.
func (Faulted) SessionEnded(session.Session) {}

// GrantsList returns daemon_faulted.
func (Faulted) GrantsList(context.Context, *protocol.GrantsList) (*protocol.Grants, *protocol.Error) {
	return nil, errNoBroker()
}

// GrantRevoke returns daemon_faulted.
func (Faulted) GrantRevoke(context.Context, *protocol.GrantRevoke) (*protocol.GrantRevoked, *protocol.Error) {
	return nil, errNoBroker()
}

// Status returns daemon_faulted.
func (Faulted) Status(context.Context, *protocol.Status) (*protocol.StatusResult, *protocol.Error) {
	return nil, errNoBroker()
}

// FaultClear returns daemon_faulted.
func (Faulted) FaultClear(context.Context, *protocol.FaultClear) (*protocol.FaultClearResult, *protocol.Error) {
	return nil, errNoBroker()
}

// Interface checks.
var (
	_ AgentHandler = Faulted{}
	_ AdminHandler = Faulted{}
)
