// Package daemon is the process "a38 serve": the two sockets, the session
// registry and the wiring of all parts.
//
// This file defines the seam between the socket layer and the broker. The
// socket layer accepts connections, reads the peer data, checks each
// message against the schema and owns the session registry. The broker
// decides: policy, bundling, approval, grants, audit log and delivery. The
// two sides know each other only through [AgentHandler] and
// [AdminHandler], so that each side has its own tests.
package daemon

import (
	"context"

	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// AgentHandler handles the requests of the agent socket. The broker
// implements it; the socket layer calls it.
//
// The socket layer calls a method only for a valid message of a session
// that sent hello and that has no session name conflict. It calls the
// methods from more than one goroutine: one session can have more than one
// open request.
//
// A method returns a response or an error, never both and never neither.
// The error type is *protocol.Error, not error, so that each failure has a
// stable code. The socket layer sets the correlation ID of the response.
// ctx ends when the connection of the session ends.
type AgentHandler interface {
	// RequestCredentials runs one request through the policy, the bundle
	// window, the approval and the delivery. It returns when each
	// reference has a value or an error.
	RequestCredentials(ctx context.Context, s session.Session, req *protocol.RequestCredentials) (*protocol.Credentials, *protocol.Error)
	// RequestExec runs one request of the exec way through the policy, the
	// check of the command, the approval and the delivery (D61 to D65).
	// It returns the values of all variables or an error, never a part.
	RequestExec(ctx context.Context, s session.Session, req *protocol.RequestExec) (*protocol.ExecCredentials, *protocol.Error)
	// ExecResult records the result of a command that "a38 mcp" reports
	// after a delivery of RequestExec (D65, G39).
	ExecResult(ctx context.Context, s session.Session, req *protocol.ExecResult) (*protocol.ExecRecorded, *protocol.Error)
	// GrantStatus returns the grants of the session, without values.
	GrantStatus(ctx context.Context, s session.Session, req *protocol.GrantStatus) (*protocol.Grants, *protocol.Error)
	// SessionEnded tells the broker that the connection of the session
	// ended. The broker ends the grants and the open bundle of the
	// session.
	SessionEnded(s session.Session)
}

// AdminHandler handles the requests of the admin socket. The broker
// implements it; the socket layer calls it. The rules for the result are
// the rules of [AgentHandler]. The admin commands work also while delivery
// is locked.
type AdminHandler interface {
	// GrantsList returns the grants of all sessions.
	GrantsList(ctx context.Context, req *protocol.GrantsList) (*protocol.Grants, *protocol.Error)
	// GrantRevoke ends one grant. An unknown grant is the code not_found.
	GrantRevoke(ctx context.Context, req *protocol.GrantRevoke) (*protocol.GrantRevoked, *protocol.Error)
	// Status returns the state of the broker. The socket layer sets the
	// number of sessions and the version of the daemon.
	Status(ctx context.Context, req *protocol.Status) (*protocol.StatusResult, *protocol.Error)
	// FaultClear removes the global delivery lock if the check of the
	// cause passes.
	FaultClear(ctx context.Context, req *protocol.FaultClear) (*protocol.FaultClearResult, *protocol.Error)
}
