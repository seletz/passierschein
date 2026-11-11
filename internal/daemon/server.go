package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"github.com/seletz/passierschein/internal/clock"
	"github.com/seletz/passierschein/internal/config"
)

// acceptRetry is the pause after a failed accept, for example when the
// process has no free file descriptor.
const acceptRetry = 100 * time.Millisecond

// Limits of the drain before the server closes a connection. See
// [Server.closeConn].
const (
	drainLimit   = 1 << 20
	drainTimeout = 500 * time.Millisecond
)

// Options holds the parts of a [Server].
type Options struct {
	// Config is the loaded configuration. The server uses the socket
	// directory, the projects and the rules for the session name.
	Config *config.Config
	// Agent handles the requests of the agent socket.
	Agent AgentHandler
	// Admin handles the requests of the admin socket.
	Admin AdminHandler
	// Logger receives the log of the socket layer. Nil discards the log.
	Logger *slog.Logger
	// Clock is the clock of the server. Nil is the system clock.
	Clock clock.Clock
}

// Server is the socket layer of the daemon: the agent socket, the admin
// socket and the session registry. Call [Server.Listen] once, then
// [Server.Serve] once.
type Server struct {
	cfg   *config.Config
	agent AgentHandler
	admin AdminHandler
	log   *slog.Logger
	clock clock.Clock
	reg   *registry
	// readPeer reads the peer data of a connection. A test can replace it.
	readPeer func(*net.UnixConn) (Peer, error)

	lock   *os.File
	agentL *net.UnixListener
	adminL *net.UnixListener

	mu     sync.Mutex
	conns  map[net.Conn]struct{}
	closed bool
	wg     sync.WaitGroup
}

// New returns a server. It does not create a socket.
func New(opts Options) (*Server, error) {
	if opts.Config == nil || opts.Agent == nil || opts.Admin == nil {
		return nil, errors.New("daemon: the configuration and both handlers are necessary")
	}
	s := &Server{
		cfg:      opts.Config,
		agent:    opts.Agent,
		admin:    opts.Admin,
		log:      opts.Logger,
		clock:    opts.Clock,
		reg:      newRegistry(),
		readPeer: readPeer,
		conns:    map[net.Conn]struct{}{},
	}
	if s.log == nil {
		s.log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if s.clock == nil {
		s.clock = clock.System{}
	}
	return s, nil
}

// AgentSocket returns the path of the agent socket.
func (s *Server) AgentSocket() string { return s.cfg.AgentSocket() }

// AdminSocket returns the path of the admin socket.
func (s *Server) AdminSocket() string { return s.cfg.AdminSocket() }

// Listen creates the socket directory and both sockets (D43). It fails if
// a socket path is too long, if the directory has a mode other than 0700,
// or if another daemon uses the directory. It removes an old socket that
// no process serves.
func (s *Server) Listen() (err error) {
	if s.lock != nil {
		return errors.New("daemon: the server listens already")
	}
	dir := s.cfg.Paths.SocketDir
	paths := []string{s.AgentSocket(), s.AdminSocket()}
	for _, p := range paths {
		if err := checkSocketPath(p); err != nil {
			return err
		}
	}
	if err := prepareDir(dir); err != nil {
		return err
	}
	lock, err := lockDir(dir)
	if err != nil {
		return err
	}
	var listeners []*net.UnixListener
	defer func() {
		if err != nil {
			for _, l := range listeners {
				_ = l.Close()
			}
			_ = lock.Close()
		}
	}()
	for _, p := range paths {
		if err := removeStale(p); err != nil {
			return err
		}
		l, err := listen(p)
		if err != nil {
			return err
		}
		listeners = append(listeners, l)
	}
	s.lock, s.agentL, s.adminL = lock, listeners[0], listeners[1]
	return nil
}

// Serve accepts connections on both sockets until ctx ends. Then it closes
// both sockets and removes them, closes each connection, ends each open
// request and waits for the goroutines of the server. Call Listen before.
func (s *Server) Serve(ctx context.Context) error {
	if s.agentL == nil {
		return errors.New("daemon: call Listen before Serve")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var accepts sync.WaitGroup
	accepts.Add(2)
	go func() { defer accepts.Done(); s.acceptLoop(ctx, s.agentL, false) }()
	go func() { defer accepts.Done(); s.acceptLoop(ctx, s.adminL, true) }()

	<-ctx.Done()
	s.mu.Lock()
	s.closed = true
	// Closing a listener removes its socket file.
	_ = s.agentL.Close()
	_ = s.adminL.Close()
	for c := range s.conns {
		_ = c.Close()
	}
	s.mu.Unlock()
	accepts.Wait()
	s.wg.Wait()
	if err := s.lock.Close(); err != nil {
		return fmt.Errorf("daemon: cannot unlock the socket directory: %w", err)
	}
	return nil
}

// acceptLoop accepts connections until the listener closes.
func (s *Server) acceptLoop(ctx context.Context, l *net.UnixListener, admin bool) {
	for {
		c, err := l.AcceptUnix()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				return
			}
			s.log.Error("cannot accept a connection", "error", err)
			t := s.clock.NewTimer(acceptRetry)
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C():
			}
			continue
		}
		if !s.track(c) {
			_ = c.Close()
			return
		}
		go func() {
			defer s.wg.Done()
			defer s.untrack(c)
			s.handleConn(ctx, c, admin)
		}()
	}
}

// track registers an open connection. It returns false after the start of
// the shutdown.
func (s *Server) track(c net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	s.conns[c] = struct{}{}
	s.wg.Add(1)
	return true
}

// closeConn ends a connection so that the peer receives each response.
//
// If the server closes a socket while unread input is in its receive
// buffer, Linux sends a reset, and the peer can lose the last response, for
// example the error for an oversized message. closeConn therefore closes
// the write side first, then reads and discards the rest of the input up
// to drainLimit bytes or drainTimeout, and closes after that. The limits
// make sure that a peer cannot keep the connection open. At the shutdown of
// the server, closeConn closes at once.
func (s *Server) closeConn(ctx context.Context, c *net.UnixConn) {
	defer func() { _ = c.Close() }()
	if ctx.Err() != nil {
		return
	}
	if err := c.CloseWrite(); err != nil {
		return
	}
	if err := c.SetReadDeadline(s.clock.Now().Add(drainTimeout)); err != nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(c, drainLimit))
}

// untrack closes and removes a connection.
func (s *Server) untrack(c net.Conn) {
	_ = c.Close()
	s.mu.Lock()
	delete(s.conns, c)
	s.mu.Unlock()
}

// handleConn reads the peer data and serves one connection. It serves only
// a peer of the same user as the daemon.
func (s *Server) handleConn(ctx context.Context, c *net.UnixConn, admin bool) {
	defer s.closeConn(ctx, c)
	peer, err := s.readPeer(c)
	if err != nil {
		s.log.Warn("connection closed: cannot read the peer data", "error", err)
		return
	}
	if peer.UID != os.Getuid() {
		s.log.Warn("connection closed: the peer is a different user", "peer_uid", peer.UID, "peer_pid", peer.PID)
		return
	}
	if admin {
		s.serveAdmin(ctx, c)
		return
	}
	s.serveAgent(ctx, c, peer)
}
