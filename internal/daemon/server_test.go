package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/client"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
	"github.com/seletz/passierschein/internal/version"
)

func TestSocketModes(t *testing.T) {
	// T12 (part): the directory has 0700, each socket 0600.
	r := startServer(t, testConfig(t, nil), newFakeBroker())
	tests := []struct {
		name string
		path string
		want os.FileMode
	}{
		{"socket directory", r.cfg.Paths.SocketDir, 0o700},
		{"agent socket", r.srv.AgentSocket(), 0o600},
		{"admin socket", r.srv.AdminSocket(), 0o600},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fileMode(t, tt.path); got != tt.want {
				t.Errorf("mode of %s = %04o, want %04o", tt.path, got, tt.want)
			}
		})
	}
}

func TestListenRejectsSocketDir(t *testing.T) {
	// T12 (part): the daemon does not start if the directory has wider
	// rights or is not a directory of the user.
	tests := []struct {
		name    string
		prepare func(t *testing.T, dir string)
		wantErr string
	}{
		{"mode 0755", mkdirMode(0o755), "must be 0700"},
		{"mode 0750", mkdirMode(0o750), "must be 0700"},
		{"mode 0701", mkdirMode(0o701), "must be 0700"},
		{"mode 0777", mkdirMode(0o777), "must be 0700"},
		{"mode 0500", mkdirMode(0o500), "must be 0700"},
		{"regular file", func(t *testing.T, dir string) {
			if err := os.WriteFile(dir, nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}, "not a directory"},
		{"symbolic link to a directory", func(t *testing.T, dir string) {
			target := filepath.Join(filepath.Dir(dir), "target")
			if err := os.Mkdir(target, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, dir); err != nil {
				t.Fatal(err)
			}
		}, "not a directory"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig(t, nil)
			tt.prepare(t, cfg.Paths.SocketDir)
			srv, err := New(Options{Config: cfg, Agent: newFakeBroker(), Admin: newFakeBroker()})
			if err != nil {
				t.Fatal(err)
			}
			err = srv.Listen()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Listen() error = %v, want %q", err, tt.wantErr)
			}
			if _, err := os.Lstat(srv.AgentSocket()); err == nil {
				t.Error("Listen() created the agent socket")
			}
		})
	}
}

// mkdirMode returns a preparation that creates the directory with a mode.
func mkdirMode(mode os.FileMode) func(t *testing.T, dir string) {
	return func(t *testing.T, dir string) {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, mode); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	}
}

func TestListenCreatesSocketDir(t *testing.T) {
	// T12 (part): a missing directory is created with 0700, also with a
	// wide umask of the test process.
	cfg := testConfig(t, nil)
	cfg.Paths.SocketDir = filepath.Join(cfg.Paths.SocketDir, "a", "b")
	r := startServer(t, cfg, newFakeBroker())
	if got := fileMode(t, cfg.Paths.SocketDir); got != 0o700 {
		t.Errorf("mode = %04o, want 0700", got)
	}
	r.wantWorks(t)
}

func TestListenRejectsLongPath(t *testing.T) {
	cfg := testConfig(t, nil)
	cfg.Paths.SocketDir = filepath.Join(cfg.Paths.SocketDir, strings.Repeat("x", 100))
	srv, err := New(Options{Config: cfg, Agent: newFakeBroker(), Admin: newFakeBroker()})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Listen(); err == nil || !strings.Contains(err.Error(), "longer than 103 bytes") {
		t.Fatalf("Listen() error = %v, want a path length error", err)
	}
	if _, err := os.Lstat(cfg.Paths.SocketDir); err == nil {
		t.Error("Listen() created the directory of a path that is too long")
	}
}

func TestRemoveStale(t *testing.T) {
	// An old socket that no process serves is removed. A live socket and
	// a file that is not a socket stay.
	tests := []struct {
		name      string
		prepare   func(t *testing.T, path string)
		wantErr   string
		wantExist bool
	}{
		{"no file", func(*testing.T, string) {}, "", false},
		{"stale socket", func(t *testing.T, path string) {
			l, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
			if err != nil {
				t.Fatal(err)
			}
			l.SetUnlinkOnClose(false)
			_ = l.Close()
		}, "", false},
		{"live socket", func(t *testing.T, path string) {
			l, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = l.Close() })
			go func() {
				for {
					c, err := l.Accept()
					if err != nil {
						return
					}
					_ = c.Close()
				}
			}()
		}, "another daemon runs", true},
		{"regular file", func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("beispiel"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, "is not a socket", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(shortDir(t), "agent.sock")
			tt.prepare(t, path)
			err := removeStale(path)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("removeStale() error = %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("removeStale() error = %v, want %q", err, tt.wantErr)
			}
			_, statErr := os.Lstat(path)
			if exists := statErr == nil; exists != tt.wantExist {
				t.Errorf("file exists = %v, want %v", exists, tt.wantExist)
			}
		})
	}
}

func TestListenReplacesStaleSockets(t *testing.T) {
	// A daemon that ended without cleanup leaves the sockets. The next
	// start removes them.
	cfg := testConfig(t, nil)
	if err := os.Mkdir(cfg.Paths.SocketDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{cfg.AgentSocket(), cfg.AdminSocket()} {
		l, err := net.ListenUnix("unix", &net.UnixAddr{Name: p, Net: "unix"})
		if err != nil {
			t.Fatal(err)
		}
		l.SetUnlinkOnClose(false)
		_ = l.Close()
	}
	r := startServer(t, cfg, newFakeBroker())
	r.wantWorks(t)
}

func TestSecondDaemonRefused(t *testing.T) {
	// A second daemon with the same socket directory does not start and
	// does not remove the sockets of the first.
	cfg := testConfig(t, nil)
	r := startServer(t, cfg, newFakeBroker())
	second, err := New(Options{Config: cfg, Agent: newFakeBroker(), Admin: newFakeBroker()})
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Listen(); err == nil || !strings.Contains(err.Error(), "another daemon") {
		t.Fatalf("Listen() of the second daemon error = %v, want another daemon", err)
	}
	r.wantWorks(t)
}

func TestShutdown(t *testing.T) {
	// The cancel of the context ends open requests, closes the
	// connections, calls SessionEnded and removes the sockets.
	broker := newFakeBroker()
	started := make(chan struct{})
	handlerEnded := make(chan error, 1)
	broker.creds = func(ctx context.Context, _ session.Session, _ *protocol.RequestCredentials) (*protocol.Credentials, *protocol.Error) {
		close(started)
		<-ctx.Done()
		handlerEnded <- ctx.Err()
		return nil, protocol.Errorf(protocol.CodeTimeout, "the request ended")
	}
	r := startServer(t, testConfig(t, nil), broker)
	c, w := r.agent(t, "beispiel-koordinator")
	admin, err := client.DialAdmin(testCtx(t), r.srv.AdminSocket())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = admin.Close() }()

	reqErr := make(chan error, 1)
	go func() {
		_, err := request(t, c, "datenbank-dev")
		reqErr <- err
	}()
	<-started
	r.stop(t)

	if err := <-handlerEnded; err == nil {
		t.Error("the context of the open request did not end")
	}
	// The request ends with an error code or with the end of the
	// connection; it never gets a value.
	if err := <-reqErr; err == nil {
		t.Error("the open request did not end with an error")
	}
	if n := broker.endedCount(session.ID(w.SessionID)); n != 1 {
		t.Errorf("SessionEnded calls = %d, want 1", n)
	}
	for _, p := range []string{r.srv.AgentSocket(), r.srv.AdminSocket()} {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Errorf("socket %s exists after the shutdown", p)
		}
	}
	if _, err := admin.Status(testCtx(t)); !isClosed(err) {
		t.Errorf("admin request after the shutdown error = %v, want the end of the connection", err)
	}
	// The directory is free again: a new daemon starts.
	startServer(t, r.cfg, newFakeBroker()).wantWorks(t)
}

func TestHelloSessionName(t *testing.T) {
	// T21 (part): the session name is checked against the rules of the
	// configuration. A failure ends with invalid_request when the
	// connection starts; the daemon serves other sessions.
	letters := "beispiel-koordinator_(Teil:2) "
	name80 := strings.Repeat(letters, 3)[:80]
	tests := []struct {
		name      string
		maxLength int
		extra     string
		session   string
		wantErr   bool
	}{
		{"no name", 0, "", "", false},
		{"example name", 0, "", "beispiel-koordinator-7f3a", false},
		{"all allowed characters", 0, "", "beispiel: koordinator (2)", false},
		{"80 characters", 0, "", name80, false},
		{"81 characters", 0, "", name80 + "x", true},
		{"dot", 0, "", "beispiel.koordinator", true},
		{"umlaut", 0, "", "beispiel-koordinatör", true},
		{"slash", 0, "", "beispiel/koordinator", true},
		{"line feed", 0, "", "beispiel\nkoordinator", true},
		{"escape sequence", 0, "", "beispiel\x1b[31m", true},
		{"NUL", 0, "", "beispiel\x00", true},
		{"right-to-left override", 0, "", "beispiel\u202ekoordinator", true},
		{"control character that the configuration allows", 0, "\x1b\u202e\t", "beispiel\x1b", true},
		{"direction mark that the configuration allows", 0, "\x1b\u202e\t", "beispiel\u202e", true},
		{"tab that the configuration allows", 0, "\x1b\u202e\t", "beispiel\tkoordinator", true},
		{"max 20: 20 characters", 20, ".", strings.Repeat("a", 20), false},
		{"max 20: 21 characters", 20, ".", strings.Repeat("a", 21), true},
		{"max 20: dot allowed", 20, ".", "beispiel.koordinator", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig(t, nil)
			if tt.maxLength > 0 {
				cfg.SessionName.MaxLength = tt.maxLength
			}
			cfg.SessionName.AllowedCharacters += tt.extra
			broker := newFakeBroker()
			r := startServer(t, cfg, broker)
			// The test sends the hello line without the client package: the
			// client checks a control character before it sends.
			name, err := json.Marshal(tt.session)
			if err != nil {
				t.Fatal(err)
			}
			conn := rawDial(t, r.srv.AgentSocket())
			resp := conn.roundTrip(t, `{"version":1,"type":"hello","agent":"coding-agent","session_name":`+string(name)+`}`)
			const req = `{"version":1,"type":"request_credentials","id":"r-1","references":["op://beispiel.app.k8s-dev/datenbank-dev/password"]}`
			if !tt.wantErr {
				if _, ok := resp.(*protocol.Welcome); !ok {
					t.Fatalf("response = %#v, want welcome", resp)
				}
				if _, ok := conn.roundTrip(t, req).(*protocol.Credentials); !ok {
					t.Fatal("the session does not work")
				}
				if got := broker.lastSession().Name; got != tt.session {
					t.Errorf("session name = %q, want %q", got, tt.session)
				}
				return
			}
			e := wantErrorResponse(t, resp, protocol.CodeInvalidRequest)
			if strings.ContainsFunc(e.Error.Message, protocol.IsControl) {
				t.Errorf("the error text contains a control character: %q", e.Error.Message)
			}
			if m := conn.read(t, protocol.AgentResponses); m != nil {
				t.Errorf("response after a rejected hello = %#v, want the end of the connection", m)
			}
			if n := broker.callCount(); n != 0 {
				t.Errorf("handler calls = %d, want 0", n)
			}
			r.wantWorks(t)
		})
	}
}

func TestSessionNameConflict(t *testing.T) {
	// T21 (part), D25: two connected sessions with the same name. Each
	// request of both ends with session_name_conflict and no handler is
	// called. When one ends, the other works again.
	broker := newFakeBroker()
	r := startServer(t, testConfig(t, nil), broker)
	first, firstWelcome := r.agent(t, "beispiel-koordinator")
	if _, err := request(t, first, "datenbank-dev"); err != nil {
		t.Fatalf("request before the conflict error = %v", err)
	}
	second, _ := r.agent(t, "beispiel-koordinator")
	callsBefore := broker.callCount()

	for name, c := range map[string]*client.Conn{"first": first, "second": second} {
		_, err := request(t, c, "datenbank-dev")
		wantCode(t, err, protocol.CodeSessionNameConflict)
		_, err = c.GrantStatus(testCtx(t))
		wantCode(t, err, protocol.CodeSessionNameConflict)
		t.Logf("%s session: rejected", name)
	}
	if n := broker.callCount(); n != callsBefore {
		t.Errorf("handler calls during the conflict = %d, want 0", n-callsBefore)
	}
	// A session without a name and a session with a different name are
	// not part of the conflict.
	r.wantWorks(t)
	other, _ := r.agent(t, "beispiel-koordinator (2)")
	if _, err := request(t, other, "datenbank-dev"); err != nil {
		t.Errorf("request of a different name error = %v", err)
	}

	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	broker.waitEnded(t, session.ID(firstWelcome.SessionID))
	if _, err := request(t, second, "datenbank-dev"); err != nil {
		t.Errorf("request after the end of the conflict error = %v", err)
	}
}

func TestSessionNameConflictDuringRequest(t *testing.T) {
	// D25, fail closed: a conflict that starts while the handler runs also
	// stops the response of the handler.
	broker := newFakeBroker()
	started := make(chan struct{})
	release := make(chan struct{})
	broker.creds = func(_ context.Context, _ session.Session, req *protocol.RequestCredentials) (*protocol.Credentials, *protocol.Error) {
		close(started)
		<-release
		return credentialsFor(req), nil
	}
	r := startServer(t, testConfig(t, nil), broker)
	first, _ := r.agent(t, "beispiel-koordinator")
	errc := make(chan error, 1)
	go func() {
		_, err := request(t, first, "datenbank-dev")
		errc <- err
	}()
	<-started
	r.agent(t, "beispiel-koordinator")
	close(release)
	wantCode(t, <-errc, protocol.CodeSessionNameConflict)
}

func TestSessionNameFixedPerConnection(t *testing.T) {
	// T21 (part): a second hello and a field for the name in
	// request_credentials end with invalid_request.
	broker := newFakeBroker()
	r := startServer(t, testConfig(t, nil), broker)
	conn := rawAgent(t, r, "beispiel-koordinator")
	tests := []struct {
		name string
		line string
	}{
		{"second hello", `{"version":1,"type":"hello","session_name":"beispiel-koordinator-2"}`},
		{"name field in request", `{"version":1,"type":"request_credentials","id":"r-1","session_name":"beispiel-koordinator-2","references":["op://beispiel.app.k8s-dev/datenbank-dev/password"]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := conn.roundTrip(t, tt.line)
			wantErrorResponse(t, resp, protocol.CodeInvalidRequest)
		})
	}
	// The session keeps its first name.
	resp := conn.roundTrip(t, `{"version":1,"type":"request_credentials","id":"r-2","references":["op://beispiel.app.k8s-dev/datenbank-dev/password"]}`)
	if _, ok := resp.(*protocol.Credentials); !ok {
		t.Fatalf("response = %#v, want credentials", resp)
	}
	if got := broker.lastSession().Name; got != "beispiel-koordinator" {
		t.Errorf("session name = %q, want the name of the first hello", got)
	}
}

func TestSessionEndedOnce(t *testing.T) {
	// Each session gets exactly one call of SessionEnded when its
	// connection ends. A rejected hello is no session.
	broker := newFakeBroker()
	r := startServer(t, testConfig(t, nil), broker)
	var ids []session.ID
	for range 5 {
		c, w := r.agent(t, "")
		ids = append(ids, session.ID(w.SessionID))
		if _, err := request(t, c, "datenbank-dev"); err != nil {
			t.Fatal(err)
		}
		_ = c.Close()
	}
	c, err := client.DialAgent(testCtx(t), r.srv.AgentSocket())
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Hello(testCtx(t), "beispiel.koordinator", "")
	wantCode(t, err, protocol.CodeInvalidRequest)
	_ = c.Close()
	r.stop(t)
	for _, id := range ids {
		if n := broker.endedCount(id); n != 1 {
			t.Errorf("SessionEnded(%s) calls = %d, want 1", id, n)
		}
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	if len(broker.ended) != len(ids) {
		t.Errorf("SessionEnded was called for %d sessions, want %d", len(broker.ended), len(ids))
	}
}

func TestAdminSocket(t *testing.T) {
	// T12 (part), D44: the admin socket accepts the admin messages. The
	// status names the number of sessions and the version.
	broker := newFakeBroker()
	broker.revoke = func(req *protocol.GrantRevoke) (*protocol.GrantRevoked, *protocol.Error) {
		if req.GrantID != "g-1" {
			return nil, protocol.Errorf(protocol.CodeNotFound, "the grant does not exist")
		}
		return &protocol.GrantRevoked{GrantID: req.GrantID}, nil
	}
	r := startServer(t, testConfig(t, nil), broker)
	r.agent(t, "beispiel-koordinator")
	r.agent(t, "")
	admin, err := client.DialAdmin(testCtx(t), r.srv.AdminSocket())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = admin.Close() }()

	status, err := admin.Status(testCtx(t))
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Sessions != 2 || status.DaemonVersion != version.Version || status.Grants != 3 {
		t.Errorf("Status() = %+v, want 2 sessions, version %q and the grants of the broker", status, version.Version)
	}
	if _, err := admin.GrantsList(testCtx(t)); err != nil {
		t.Errorf("GrantsList() error = %v", err)
	}
	if got, err := admin.GrantRevoke(testCtx(t), "g-1"); err != nil || got.GrantID != "g-1" {
		t.Errorf("GrantRevoke(g-1) = %+v, %v", got, err)
	}
	_, err = admin.GrantRevoke(testCtx(t), "g-2")
	wantCode(t, err, protocol.CodeNotFound)
	if got, err := admin.FaultClear(testCtx(t)); err != nil || !got.Cleared {
		t.Errorf("FaultClear() = %+v, %v", got, err)
	}

	// A message of the agent socket on the admin socket is invalid.
	raw := rawDial(t, r.srv.AdminSocket())
	resp := raw.roundTripSet(t, `{"version":1,"type":"hello","session_name":"beispiel-koordinator"}`, protocol.AdminResponses)
	wantErrorResponse(t, resp, protocol.CodeInvalidRequest)
}

func TestAdminMessagesOnAgentSocket(t *testing.T) {
	// T12 (part), D44: a revoke and the list of all grants on the agent
	// socket end with invalid_request.
	r := startServer(t, testConfig(t, nil), newFakeBroker())
	conn := rawAgent(t, r, "beispiel-koordinator")
	tests := []struct {
		name string
		line string
	}{
		{"grant_revoke", `{"version":1,"type":"grant_revoke","id":"r-1","grant_id":"g-1"}`},
		{"grants_list", `{"version":1,"type":"grants_list","id":"r-2"}`},
		{"status", `{"version":1,"type":"status","id":"r-3"}`},
		{"fault_clear", `{"version":1,"type":"fault_clear","id":"r-4"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := conn.roundTrip(t, tt.line)
			e := wantErrorResponse(t, resp, protocol.CodeInvalidRequest)
			if want := lenientID([]byte(tt.line)); e.ID != want {
				t.Errorf("ID = %q, want %q", e.ID, want)
			}
		})
	}
	// Also as the first message, before hello.
	raw := rawDial(t, r.srv.AgentSocket())
	wantErrorResponse(t, raw.roundTrip(t, `{"version":1,"type":"grants_list","id":"r-5"}`), protocol.CodeInvalidRequest)
}

func TestOtherUserRejected(t *testing.T) {
	// Only peers of the same user are served.
	broker := newFakeBroker()
	r := startServer(t, testConfig(t, nil), broker, func(s *Server) {
		s.readPeer = func(c *net.UnixConn) (Peer, error) {
			p, err := readPeer(c)
			p.UID++
			return p, err
		}
	})
	for _, path := range []string{r.srv.AgentSocket(), r.srv.AdminSocket()} {
		c, err := client.DialAgent(testCtx(t), path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.Hello(testCtx(t), "beispiel-koordinator", ""); !isClosed(err) {
			t.Errorf("%s: Hello() error = %v, want the end of the connection", path, err)
		}
		_ = c.Close()
	}
	if broker.callCount() != 0 {
		t.Error("the broker received a call")
	}
}

func TestHandlerContract(t *testing.T) {
	// D35, D48: a handler that breaks the contract is an internal error.
	// The response is daemon_faulted, and the daemon serves other
	// sessions.
	tests := []struct {
		name  string
		creds func(context.Context, session.Session, *protocol.RequestCredentials) (*protocol.Credentials, *protocol.Error)
		want  protocol.Code
	}{
		{"neither response nor error", func(context.Context, session.Session, *protocol.RequestCredentials) (*protocol.Credentials, *protocol.Error) {
			return nil, nil
		}, protocol.CodeDaemonFaulted},
		{"response and error", func(_ context.Context, _ session.Session, req *protocol.RequestCredentials) (*protocol.Credentials, *protocol.Error) {
			return credentialsFor(req), protocol.Errorf(protocol.CodeDenied, "denied")
		}, protocol.CodeDaemonFaulted},
		{"unknown code", func(context.Context, session.Session, *protocol.RequestCredentials) (*protocol.Credentials, *protocol.Error) {
			return nil, &protocol.Error{Code: "beispiel", Message: "unknown"}
		}, protocol.CodeDaemonFaulted},
		{"invalid response", func(context.Context, session.Session, *protocol.RequestCredentials) (*protocol.Credentials, *protocol.Error) {
			return &protocol.Credentials{}, nil
		}, protocol.CodeDaemonFaulted},
		{"panic", func(context.Context, session.Session, *protocol.RequestCredentials) (*protocol.Credentials, *protocol.Error) {
			panic("beispiel")
		}, protocol.CodeDaemonFaulted},
		{"stable code passes", func(context.Context, session.Session, *protocol.RequestCredentials) (*protocol.Credentials, *protocol.Error) {
			return nil, protocol.Errorf(protocol.CodeNotAllowed, "the policy does not allow the request")
		}, protocol.CodeNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			broker := newFakeBroker()
			broker.creds = tt.creds
			r := startServer(t, testConfig(t, nil), broker)
			c, _ := r.agent(t, "beispiel-koordinator")
			_, err := request(t, c, "datenbank-dev")
			wantCode(t, err, tt.want)
			broker.creds = nil
			if _, err := request(t, c, "datenbank-dev"); err != nil {
				t.Errorf("next request of the session error = %v", err)
			}
		})
	}
}

func TestConcurrentSessions(t *testing.T) {
	// The requests of one session run at the same time; the correlation
	// ID matches each response. More sessions run at the same time. Run
	// with the race detector.
	const sessions, perSession = 4, 16
	broker := newFakeBroker()
	var arrived sync.WaitGroup
	arrived.Add(sessions * perSession)
	all := make(chan struct{})
	go func() { arrived.Wait(); close(all) }()
	broker.creds = func(ctx context.Context, _ session.Session, req *protocol.RequestCredentials) (*protocol.Credentials, *protocol.Error) {
		// Each handler waits until all requests are open: this works only
		// if the requests run at the same time.
		arrived.Done()
		select {
		case <-all:
		case <-ctx.Done():
			return nil, protocol.Errorf(protocol.CodeTimeout, "the request ended")
		}
		return credentialsFor(req), nil
	}
	r := startServer(t, testConfig(t, nil), broker)
	var wg sync.WaitGroup
	for s := range sessions {
		c, _ := r.agent(t, "beispiel-koordinator-"+string(rune('a'+s)))
		for i := range perSession {
			wg.Add(1)
			go func() {
				defer wg.Done()
				item := "datenbank-dev-" + string(rune('a'+s)) + "-" + string(rune('a'+i))
				creds, err := request(t, c, item)
				if err != nil {
					t.Errorf("request %s error = %v", item, err)
					return
				}
				if got := creds.Entries[0].Value.Reveal(); got != "beispiel-wert-"+item {
					t.Errorf("request %s got the response of another request", item)
				}
			}()
		}
	}
	wg.Wait()
}

func TestFactsComeFromTheDaemon(t *testing.T) {
	// T11 (part), D4, D25: the session ID, the process, the working
	// directory and the project come from the daemon. The claims of the
	// agent do not change them.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	wd = mustResolve(t, wd)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	broker := newFakeBroker()
	r := startServer(t, testConfig(t, map[string]string{"beispielprojekt": wd, "anderes-projekt": "/beispiel/anderes-projekt"}), broker)
	claim := "anderes-projekt s-00000000000000000000000000000000 /usr/bin/beispiel"
	c, w := r.agent(t, "beispiel-koordinator")
	if w.Project != "beispielprojekt" {
		t.Errorf("welcome project = %q, want beispielprojekt", w.Project)
	}
	ref, _ := protocol.ParseReference("op://beispiel.app.k8s-dev/datenbank-dev/password")
	if _, err := c.RequestCredentials(testCtx(t), &protocol.RequestCredentials{References: []protocol.Reference{ref}, Task: claim, Reason: claim}); err != nil {
		t.Fatal(err)
	}
	got := broker.lastSession()
	want := session.Facts{
		SessionID: session.ID(w.SessionID),
		Origin:    session.OriginLocal,
		Project:   "beispielprojekt",
		PeerPID:   os.Getpid(),
		PeerUID:   os.Getuid(),
		Process:   mustResolve(t, exe),
		Cwd:       wd,
	}
	if got.Facts != want {
		t.Errorf("facts = %+v, want %+v", got.Facts, want)
	}
	if got.Name != "beispiel-koordinator" || got.Agent != "coding-agent" {
		t.Errorf("claims = %q, %q", got.Name, got.Agent)
	}
}

func TestProjectFromWorkingDirectory(t *testing.T) {
	// T11 (part): a process in the directory of project B is in project
	// B. The project of the welcome follows from the working directory of
	// the process, not from a claim.
	projA := mustResolve(t, shortDir(t))
	projB := mustResolve(t, shortDir(t))
	sub := filepath.Join(projB, "unterordner")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	nowhere := mustResolve(t, shortDir(t))
	broker := newFakeBroker()
	r := startServer(t, testConfig(t, map[string]string{"projekt-a": projA, "projekt-b": projB}), broker)
	tests := []struct {
		name string
		dir  string
		want string
	}{
		{"project A", projA, "projekt-a"},
		{"project B", projB, "projekt-b"},
		{"below project B", sub, "projekt-b"},
		{"no project", nowhere, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The helper process claims project A in its session name.
			out := runHelper(t, tt.dir, r.srv.AgentSocket(), "projekt-a")
			if out.project != tt.want {
				t.Errorf("project = %q, want %q", out.project, tt.want)
			}
			got := broker.lastSession().Facts
			if got.Cwd != tt.dir || got.PeerPID != out.pid || got.Project != tt.want {
				t.Errorf("facts = %+v, want the directory %s, the PID %d and the project %q", got, tt.dir, out.pid, tt.want)
			}
		})
	}
}

func TestUnknownCwdHasNoProject(t *testing.T) {
	// T11 (part): if the daemon cannot read the working directory, the
	// session has no project. The daemon never takes it from the agent.
	broker := newFakeBroker()
	wd, _ := os.Getwd()
	r := startServer(t, testConfig(t, map[string]string{"beispielprojekt": mustResolve(t, wd)}), broker, func(s *Server) {
		s.readPeer = func(c *net.UnixConn) (Peer, error) {
			p, err := readPeer(c)
			p.Cwd, p.Process = "", ""
			return p, err
		}
	})
	c, w := r.agent(t, "beispielprojekt")
	if w.Project != "" {
		t.Errorf("welcome project = %q, want none", w.Project)
	}
	if _, err := request(t, c, "datenbank-dev"); err != nil {
		t.Fatal(err)
	}
	if f := broker.lastSession().Facts; f.Project != "" || f.Cwd != "" || f.Process != "" {
		t.Errorf("facts = %+v, want no project, no directory and no process", f)
	}
}

func TestNewRejectsMissingParts(t *testing.T) {
	cfg := testConfig(t, nil)
	b := newFakeBroker()
	tests := []struct {
		name string
		opts Options
	}{
		{"no configuration", Options{Agent: b, Admin: b}},
		{"no agent handler", Options{Config: cfg, Admin: b}},
		{"no admin handler", Options{Config: cfg, Agent: b}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.opts); err == nil {
				t.Error("New() error = nil")
			}
		})
	}
	srv, err := New(Options{Config: cfg, Agent: b, Admin: b})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Serve(context.Background()); err == nil {
		t.Error("Serve() without Listen() error = nil")
	}
}

func TestSocketFileMeansListening(t *testing.T) {
	// Readiness: when the socket file exists, a connection is accepted. A
	// socket that is bound under its final name is refused between bind(2)
	// and listen(2); the daemon publishes the file only when it listens
	// (flake of TestServeRuns, 2026-11-12). The test polls for the file
	// without a pause and connects at once.
	for round := 0; round < 50; round++ {
		cfg := testConfig(t, nil)
		srv, err := New(Options{Config: cfg, Agent: &fakeBroker{}, Admin: &fakeBroker{}})
		if err != nil {
			t.Fatal(err)
		}
		dialErr := make(chan error, 1)
		go func() {
			deadline := time.Now().Add(testTimeout)
			for time.Now().Before(deadline) {
				if _, err := os.Lstat(srv.AgentSocket()); err == nil {
					c, err := net.Dial("unix", srv.AgentSocket())
					if err == nil {
						_ = c.Close()
					}
					dialErr <- err
					return
				}
			}
			dialErr <- errors.New("the socket file did not appear")
		}()
		if err := srv.Listen(); err != nil {
			t.Fatal(err)
		}
		if err := <-dialErr; err != nil {
			t.Fatalf("round %d: connect after the file appeared: %v", round, err)
		}
		// Close the sockets; Serve with a done context removes the files.
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := srv.Serve(ctx); err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(cfg.Paths.SocketDir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			t.Errorf("file %q is left in the socket directory after the stop", e.Name())
		}
	}
}
