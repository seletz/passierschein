package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"

	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
	"github.com/seletz/passierschein/internal/version"
)

// maxInFlight is the maximum number of open requests of one connection.
// The server reads the next message of the connection only when a request
// ends.
const maxInFlight = 64

// wire reads messages from a connection and writes messages to it. Writes
// from more than one goroutine are serialized.
type wire struct {
	r   *bufio.Reader
	mu  sync.Mutex
	w   io.Writer
	log *slog.Logger
}

// send writes one message. If the message cannot be encoded, for example
// because a handler returned an invalid response, send writes the error
// daemon_faulted with the correlation ID instead.
func (w *wire) send(id string, m protocol.Message) {
	line, err := protocol.Encode(m)
	if err != nil {
		// The error text never holds a secret value: the protocol package
		// does not repeat field content.
		w.log.Error("cannot encode a response", "type", m.MessageType(), "error", err)
		line, err = protocol.Encode(faulted(id))
		if err != nil {
			line, _ = protocol.Encode(faulted(""))
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	_, _ = w.w.Write(append(line, '\n'))
}

// errorResponse returns the error message for a failed request.
func errorResponse(id string, err error) *protocol.ErrorResponse {
	var perr *protocol.Error
	if !errors.As(err, &perr) {
		perr = protocol.Errorf(protocol.CodeInvalidRequest, "the message is not valid")
	}
	return &protocol.ErrorResponse{ID: id, Error: *perr}
}

// faulted returns the error daemon_faulted for an internal error of the
// daemon.
//
// D48 says that an internal error locks delivery for all sessions. The
// global lock belongs to the broker (issue 23); the socket layer only
// answers with the code.
func faulted(id string) *protocol.ErrorResponse {
	return &protocol.ErrorResponse{ID: id, Error: *protocol.Errorf(protocol.CodeDaemonFaulted, "the daemon has an internal error")}
}

// result makes the response of a handler call. A handler returns exactly
// one of a response and an error; any other result is an internal error.
func result(id string, resp protocol.Message, ok bool, perr *protocol.Error) protocol.Message {
	switch {
	case ok && perr == nil:
		return resp
	case !ok && perr != nil && perr.Code.Valid():
		return &protocol.ErrorResponse{ID: id, Error: *perr}
	default:
		return faulted(id)
	}
}

// requestID returns the correlation ID of a request message.
func requestID(m protocol.Message) string {
	switch m := m.(type) {
	case *protocol.RequestCredentials:
		return m.ID
	case *protocol.GrantStatus:
		return m.ID
	case *protocol.GrantsList:
		return m.ID
	case *protocol.GrantRevoke:
		return m.ID
	case *protocol.Status:
		return m.ID
	case *protocol.FaultClear:
		return m.ID
	case *protocol.RequestExec:
		return m.ID
	case *protocol.ExecResult:
		return m.ID
	}
	return ""
}

// lenientID reads the correlation ID of a line that failed the strict
// decoding, so that the client can match the error. It returns the empty
// string if the line has no valid ID.
func lenientID(line []byte) string {
	// A map matches the key "id" exactly, without regard to case.
	var fields map[string]json.RawMessage
	if json.Unmarshal(line, &fields) != nil {
		return ""
	}
	var id string
	if json.Unmarshal(fields["id"], &id) != nil {
		return ""
	}
	if (&protocol.GrantStatus{ID: id}).Validate() != nil {
		return ""
	}
	return id
}

// serveRequests reads requests from the connection until it ends and runs
// each request in its own goroutine. wg counts the goroutines; the caller
// waits for them. A line that is not a valid message gets the error
// invalid_request; the connection continues. A line above the size limit
// gets the error and ends the connection, because the stream position is
// then not defined.
func (s *Server) serveRequests(ctx context.Context, w *wire, set protocol.Set, wg *sync.WaitGroup, handle func(context.Context, protocol.Message) protocol.Message) {
	sem := make(chan struct{}, maxInFlight)
	for {
		line, err := protocol.ReadLine(w.r)
		if err != nil {
			if protocol.CodeOf(err) != "" {
				w.send("", errorResponse("", err))
			}
			return
		}
		msg, err := set.Decode(line)
		if err != nil {
			id := lenientID(line)
			w.send(id, errorResponse(id, err))
			continue
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			id := requestID(msg)
			w.send(id, s.safeCall(ctx, id, msg, handle))
		}()
	}
}

// safeCall calls the handler. A panic of the handler is an internal error
// of the daemon; the other sessions continue.
func (s *Server) safeCall(ctx context.Context, id string, msg protocol.Message, handle func(context.Context, protocol.Message) protocol.Message) (resp protocol.Message) {
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("a handler failed", "type", msg.MessageType())
			resp = faulted(id)
		}
	}()
	return handle(ctx, msg)
}

// serveAgent serves one connection of the agent socket: the hello
// handshake, then the requests of the session.
func (s *Server) serveAgent(ctx context.Context, rwc io.ReadWriter, peer Peer) {
	w := &wire{r: bufio.NewReader(rwc), w: rwc, log: s.log}
	sess, ok := s.handshake(w, peer)
	if !ok {
		return
	}
	id := sess.Facts.SessionID
	s.reg.add(id, sess.Name)
	s.log.Info("session started", "session_id", id, "project", sess.Facts.Project, "peer_pid", peer.PID)

	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	defer func() {
		s.reg.remove(id)
		cancel()
		s.agent.SessionEnded(sess)
		wg.Wait()
		s.log.Info("session ended", "session_id", id)
	}()
	w.send("", &protocol.Welcome{SessionID: string(id), Project: sess.Facts.Project})
	s.serveRequests(ctx, w, protocol.AgentRequests, &wg, func(ctx context.Context, msg protocol.Message) protocol.Message {
		return s.agentRequest(ctx, sess, msg)
	})
}

// handshake reads the hello message and makes the session. The verified
// facts come from the peer data and the configuration only; the session
// name and the agent name from hello are claims (D4, D25). On a failure
// handshake writes the error, and the caller ends the connection.
func (s *Server) handshake(w *wire, peer Peer) (session.Session, bool) {
	line, err := protocol.ReadLine(w.r)
	if err != nil {
		if protocol.CodeOf(err) != "" {
			w.send("", errorResponse("", err))
		}
		return session.Session{}, false
	}
	msg, err := protocol.AgentRequests.Decode(line)
	if err != nil {
		id := lenientID(line)
		w.send(id, errorResponse(id, err))
		return session.Session{}, false
	}
	hello, ok := msg.(*protocol.Hello)
	if !ok {
		id := requestID(msg)
		w.send(id, errorResponse(id, protocol.Errorf(protocol.CodeInvalidRequest, "the first message must be hello")))
		return session.Session{}, false
	}
	if err := s.cfg.SessionName.Check(hello.SessionName); err != nil {
		w.send("", errorResponse("", err))
		return session.Session{}, false
	}
	id, err := newSessionID()
	if err != nil {
		s.log.Error("cannot make a session ID", "error", err)
		w.send("", faulted(""))
		return session.Session{}, false
	}
	facts := session.Facts{
		SessionID: id,
		Origin:    session.OriginLocal,
		PeerPID:   peer.PID,
		PeerUID:   peer.UID,
		Process:   peer.Process,
		Cwd:       peer.Cwd,
	}
	if peer.Cwd != "" {
		if p, ok := s.cfg.ProjectFor(peer.Cwd); ok {
			facts.Project = p.Name
		}
	}
	return session.Session{Facts: facts, Name: hello.SessionName, Agent: hello.Agent}, true
}

// agentRequest handles one request of a session. While the session has a
// session name conflict, each request ends with session_name_conflict and
// the handler is not called (D25).
func (s *Server) agentRequest(ctx context.Context, sess session.Session, msg protocol.Message) protocol.Message {
	id := requestID(msg)
	if _, ok := msg.(*protocol.Hello); ok {
		return errorResponse("", protocol.Errorf(protocol.CodeInvalidRequest, "the session sent hello before; the session name is fixed for the connection"))
	}
	if s.reg.conflict(sess.Facts.SessionID) {
		return conflict(id)
	}
	var resp protocol.Message
	switch m := msg.(type) {
	case *protocol.RequestCredentials:
		r, perr := s.agent.RequestCredentials(ctx, sess, m)
		if r != nil {
			r.ID = id
		}
		resp = result(id, r, r != nil, perr)
	case *protocol.GrantStatus:
		r, perr := s.agent.GrantStatus(ctx, sess, m)
		if r != nil {
			r.ID = id
		}
		resp = result(id, r, r != nil, perr)
	case *protocol.RequestExec, *protocol.ExecResult:
		// The broker does not implement the exec way yet (issue 99). The
		// daemon delivers nothing and records nothing.
		return &protocol.ErrorResponse{ID: id, Error: *protocol.Errorf(protocol.CodeNotAllowed,
			"this version of the daemon does not implement the exec way")}
	default:
		return faulted(id)
	}
	// A conflict that started while the handler ran also stops the
	// delivery: fail closed.
	if s.reg.conflict(sess.Facts.SessionID) {
		return conflict(id)
	}
	return resp
}

// conflict returns the error session_name_conflict.
func conflict(id string) *protocol.ErrorResponse {
	return &protocol.ErrorResponse{ID: id, Error: *protocol.Errorf(protocol.CodeSessionNameConflict,
		"another connected session has the same session name; use a different name or end the other session")}
}

// serveAdmin serves one connection of the admin socket.
func (s *Server) serveAdmin(ctx context.Context, rwc io.ReadWriter) {
	w := &wire{r: bufio.NewReader(rwc), w: rwc, log: s.log}
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()
	s.serveRequests(ctx, w, protocol.AdminRequests, &wg, s.adminRequest)
}

// adminRequest handles one request of the admin socket.
func (s *Server) adminRequest(ctx context.Context, msg protocol.Message) protocol.Message {
	id := requestID(msg)
	switch m := msg.(type) {
	case *protocol.GrantsList:
		r, perr := s.admin.GrantsList(ctx, m)
		if r != nil {
			r.ID = id
		}
		return result(id, r, r != nil, perr)
	case *protocol.GrantRevoke:
		r, perr := s.admin.GrantRevoke(ctx, m)
		if r != nil {
			r.ID = id
		}
		return result(id, r, r != nil, perr)
	case *protocol.Status:
		r, perr := s.admin.Status(ctx, m)
		if r != nil {
			r.ID = id
			r.Sessions = s.reg.count()
			r.DaemonVersion = version.Version
		}
		return result(id, r, r != nil, perr)
	case *protocol.FaultClear:
		r, perr := s.admin.FaultClear(ctx, m)
		if r != nil {
			r.ID = id
		}
		return result(id, r, r != nil, perr)
	}
	return faulted(id)
}
