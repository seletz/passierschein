// Package client talks to the sockets of the daemon "a38 serve". "a38 mcp"
// uses it on the agent socket; the admin commands use it on the admin
// socket.
//
// A [Conn] is one connection. It can have more than one open request; the
// correlation ID connects each response to its request. The client sets the
// ID of each request.
package client

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"

	"github.com/seletz/passierschein/internal/protocol"
)

// ErrClosed is the error of a request when the connection ended before the
// response arrived.
var ErrClosed = errors.New("client: the connection to the daemon ended")

// Conn is one connection to a socket of the daemon. It is safe for use by
// more than one goroutine.
type Conn struct {
	conn net.Conn
	set  protocol.Set

	wmu sync.Mutex

	mu      sync.Mutex
	next    uint64
	pending map[string]chan protocol.Message
	// untagged receives a response without correlation ID: welcome, or an
	// error for hello or for a line that the daemon could not read.
	untagged chan protocol.Message
	err      error
	done     chan struct{}
}

// DialAgent connects to the agent socket at path. Send [Conn.Hello] first.
func DialAgent(ctx context.Context, path string) (*Conn, error) {
	return dial(ctx, path, protocol.AgentResponses)
}

// DialAdmin connects to the admin socket at path.
func DialAdmin(ctx context.Context, path string) (*Conn, error) {
	return dial(ctx, path, protocol.AdminResponses)
}

// dial connects to a socket and starts the reader of the responses.
func dial(ctx context.Context, path string, set protocol.Set) (*Conn, error) {
	var d net.Dialer
	nc, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		return nil, fmt.Errorf("client: cannot connect to %s: %w", path, err)
	}
	c := &Conn{
		conn:     nc,
		set:      set,
		pending:  map[string]chan protocol.Message{},
		untagged: make(chan protocol.Message, 1),
		done:     make(chan struct{}),
	}
	go c.readLoop()
	return c, nil
}

// Close ends the connection. Each open request ends with [ErrClosed].
func (c *Conn) Close() error {
	err := c.conn.Close()
	<-c.done
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

// Hello sends the hello message and returns the welcome of the daemon. A
// rejection is a *protocol.Error, for example invalid_request for a session
// name that the configuration does not allow. Call Hello once, before each
// other request of the agent socket.
func (c *Conn) Hello(ctx context.Context, sessionName, agent string) (*protocol.Welcome, error) {
	if err := c.write(&protocol.Hello{SessionName: sessionName, Agent: agent}); err != nil {
		return nil, err
	}
	resp, err := c.wait(ctx, c.untagged)
	if err != nil {
		return nil, err
	}
	switch m := resp.(type) {
	case *protocol.Welcome:
		return m, nil
	case *protocol.ErrorResponse:
		e := m.Error
		return nil, &e
	}
	return nil, fmt.Errorf("client: unexpected response %s to hello", resp.MessageType())
}

// RequestCredentials sends a request for credentials. The client sets the
// correlation ID.
func (c *Conn) RequestCredentials(ctx context.Context, req *protocol.RequestCredentials) (*protocol.Credentials, error) {
	return roundTrip[*protocol.Credentials](ctx, c, req, &req.ID)
}

// GrantStatus asks for the grants of the own session.
func (c *Conn) GrantStatus(ctx context.Context) (*protocol.Grants, error) {
	req := &protocol.GrantStatus{}
	return roundTrip[*protocol.Grants](ctx, c, req, &req.ID)
}

// GrantsList asks for the grants of all sessions (admin socket).
func (c *Conn) GrantsList(ctx context.Context) (*protocol.Grants, error) {
	req := &protocol.GrantsList{}
	return roundTrip[*protocol.Grants](ctx, c, req, &req.ID)
}

// GrantRevoke ends one grant (admin socket).
func (c *Conn) GrantRevoke(ctx context.Context, grantID string) (*protocol.GrantRevoked, error) {
	req := &protocol.GrantRevoke{GrantID: grantID}
	return roundTrip[*protocol.GrantRevoked](ctx, c, req, &req.ID)
}

// Status asks for the state of the daemon (admin socket).
func (c *Conn) Status(ctx context.Context) (*protocol.StatusResult, error) {
	req := &protocol.Status{}
	return roundTrip[*protocol.StatusResult](ctx, c, req, &req.ID)
}

// FaultClear asks the daemon to remove the global delivery lock (admin
// socket).
func (c *Conn) FaultClear(ctx context.Context) (*protocol.FaultClearResult, error) {
	req := &protocol.FaultClear{}
	return roundTrip[*protocol.FaultClearResult](ctx, c, req, &req.ID)
}

// roundTrip sets the correlation ID of req, sends it and waits for the
// response with the same ID. An error response is a *protocol.Error.
func roundTrip[R protocol.Message](ctx context.Context, c *Conn, req protocol.Message, id *string) (R, error) {
	var zero R
	ch := make(chan protocol.Message, 1)
	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return zero, c.err
	}
	c.next++
	*id = "c-" + strconv.FormatUint(c.next, 10)
	c.pending[*id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, *id)
		c.mu.Unlock()
	}()
	if err := c.write(req); err != nil {
		return zero, err
	}
	resp, err := c.wait(ctx, ch)
	if err != nil {
		return zero, err
	}
	if e, ok := resp.(*protocol.ErrorResponse); ok {
		perr := e.Error
		return zero, &perr
	}
	r, ok := resp.(R)
	if !ok {
		return zero, fmt.Errorf("client: unexpected response %s", resp.MessageType())
	}
	return r, nil
}

// write sends one message. A failed write ends the connection: the
// daemon closed it (broken pipe), or the reader closed it after the end of
// the input (use of closed network connection). Both races end with the
// same error as a request that was open at the end: [ErrClosed].
func (c *Conn) write(m protocol.Message) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	line, err := protocol.Encode(m)
	if err != nil {
		// An invalid message is an error of the caller; the connection
		// stays.
		return err
	}
	if _, err := c.conn.Write(append(line, '\n')); err != nil {
		_ = c.conn.Close()
		<-c.done
		return fmt.Errorf("client: cannot send the request: %w", c.closedErr())
	}
	return nil
}

// wait waits for a response on ch.
func (c *Conn) wait(ctx context.Context, ch <-chan protocol.Message) (protocol.Message, error) {
	select {
	case m := <-ch:
		return m, nil
	case <-c.done:
		// A response can arrive just before the end of the connection.
		select {
		case m := <-ch:
			return m, nil
		default:
		}
		return nil, c.closedErr()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// closedErr returns the reason for the end of the connection.
func (c *Conn) closedErr() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// readLoop reads the responses and gives each to its request.
func (c *Conn) readLoop() {
	r := bufio.NewReader(c.conn)
	var err error
	// A response that the client cannot decode ends the connection: the
	// client cannot tell which request it answers.
	defer func() {
		c.mu.Lock()
		if err == nil || errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
			c.err = ErrClosed
		} else {
			c.err = fmt.Errorf("%w: %w", ErrClosed, err)
		}
		c.mu.Unlock()
		_ = c.conn.Close()
		close(c.done)
	}()
	for {
		var line []byte
		line, err = protocol.ReadLine(r)
		if err != nil {
			return
		}
		var msg protocol.Message
		msg, err = c.set.Decode(line)
		if err != nil {
			return
		}
		id := responseID(msg)
		c.mu.Lock()
		ch, ok := c.pending[id]
		c.mu.Unlock()
		if id == "" {
			ch, ok = c.untagged, true
		}
		if !ok {
			continue
		}
		select {
		case ch <- msg:
		default:
		}
	}
}

// responseID returns the correlation ID of a response message.
func responseID(m protocol.Message) string {
	switch m := m.(type) {
	case *protocol.Credentials:
		return m.ID
	case *protocol.Grants:
		return m.ID
	case *protocol.ErrorResponse:
		return m.ID
	case *protocol.GrantRevoked:
		return m.ID
	case *protocol.StatusResult:
		return m.ID
	case *protocol.FaultClearResult:
		return m.ID
	}
	return ""
}
