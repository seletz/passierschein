package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/client"
	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// testTimeout limits each wait of a test.
const testTimeout = 10 * time.Second

// fakeBroker implements AgentHandler and AdminHandler for the tests of the
// socket layer. It records each call.
type fakeBroker struct {
	mu       sync.Mutex
	sessions []session.Session
	ended    map[session.ID]int
	calls    int

	// creds replaces the default answer of RequestCredentials.
	creds func(ctx context.Context, s session.Session, req *protocol.RequestCredentials) (*protocol.Credentials, *protocol.Error)
	// revoke replaces the default answer of GrantRevoke.
	revoke func(req *protocol.GrantRevoke) (*protocol.GrantRevoked, *protocol.Error)
}

func newFakeBroker() *fakeBroker {
	return &fakeBroker{ended: map[session.ID]int{}}
}

func (b *fakeBroker) record(s session.Session) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls++
	b.sessions = append(b.sessions, s)
}

// callCount returns the number of handler calls of the agent socket.
func (b *fakeBroker) callCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls
}

// lastSession returns the session of the last call.
func (b *fakeBroker) lastSession() session.Session {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.sessions) == 0 {
		return session.Session{}
	}
	return b.sessions[len(b.sessions)-1]
}

// endedCount returns how often SessionEnded was called for the session.
func (b *fakeBroker) endedCount(id session.ID) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ended[id]
}

// waitEnded waits until SessionEnded was called for the session.
func (b *fakeBroker) waitEnded(t *testing.T, id session.ID) {
	t.Helper()
	deadline := time.Now().Add(testTimeout)
	for b.endedCount(id) == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("SessionEnded was not called for %s", id)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (b *fakeBroker) RequestCredentials(ctx context.Context, s session.Session, req *protocol.RequestCredentials) (*protocol.Credentials, *protocol.Error) {
	b.record(s)
	if b.creds != nil {
		return b.creds(ctx, s, req)
	}
	return credentialsFor(req), nil
}

// credentialsFor returns a value for each reference of the request. The
// value names the reference, so that a test can match it.
func credentialsFor(req *protocol.RequestCredentials) *protocol.Credentials {
	out := &protocol.Credentials{}
	for _, ref := range req.References {
		v := protocol.NewSecret("beispiel-wert-" + ref.Item)
		out.Entries = append(out.Entries, protocol.CredentialEntry{Reference: ref, Value: &v})
	}
	return out
}

func (b *fakeBroker) RequestExec(_ context.Context, s session.Session, req *protocol.RequestExec) (*protocol.ExecCredentials, *protocol.Error) {
	b.record(s)
	out := &protocol.ExecCredentials{ID: "wrong", TimeoutSeconds: 120, OutputLimit: 2048}
	for _, v := range req.Env {
		out.Values = append(out.Values, protocol.ExecValue{Name: v.Name, Value: protocol.NewSecret("beispiel-wert")})
	}
	return out, nil
}

func (b *fakeBroker) ExecResult(_ context.Context, s session.Session, _ *protocol.ExecResult) (*protocol.ExecRecorded, *protocol.Error) {
	b.record(s)
	return &protocol.ExecRecorded{ID: "wrong"}, nil
}

func (b *fakeBroker) GrantStatus(_ context.Context, s session.Session, _ *protocol.GrantStatus) (*protocol.Grants, *protocol.Error) {
	b.record(s)
	return &protocol.Grants{Grants: []protocol.Grant{}}, nil
}

func (b *fakeBroker) SessionEnded(s session.Session) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ended[s.Facts.SessionID]++
}

func (b *fakeBroker) GrantsList(context.Context, *protocol.GrantsList) (*protocol.Grants, *protocol.Error) {
	return &protocol.Grants{Grants: []protocol.Grant{}}, nil
}

func (b *fakeBroker) GrantRevoke(_ context.Context, req *protocol.GrantRevoke) (*protocol.GrantRevoked, *protocol.Error) {
	if b.revoke != nil {
		return b.revoke(req)
	}
	return &protocol.GrantRevoked{GrantID: req.GrantID}, nil
}

func (b *fakeBroker) Status(context.Context, *protocol.Status) (*protocol.StatusResult, *protocol.Error) {
	return &protocol.StatusResult{Grants: 3}, nil
}

func (b *fakeBroker) FaultClear(context.Context, *protocol.FaultClear) (*protocol.FaultClearResult, *protocol.Error) {
	return &protocol.FaultClearResult{Cleared: true}, nil
}

// testConfig returns a configuration with the socket directory in a short
// directory under /tmp. projects maps a project name to its directory.
func testConfig(t *testing.T, projects map[string]string) *config.Config {
	t.Helper()
	yaml := fmt.Sprintf("version: 1\nbackend:\n  onepassword:\n    accounts:\n      - name: privat\nvaults:\n  - name: beispiel.app.k8s-dev\n    environment: dev\n    account: privat\npaths:\n  socket_dir: %s\n  audit_file: %s\n",
		filepath.Join(shortDir(t), "run"), filepath.Join(shortDir(t), "audit.db"))
	if len(projects) > 0 {
		yaml += "projects:\n"
		for name, dir := range projects {
			yaml += fmt.Sprintf("  - name: %s\n    paths: [%q]\n    vaults: [beispiel.app.k8s-dev]\n", name, dir)
		}
	}
	cfg, err := config.Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// running is a server that runs in the background of a test.
type running struct {
	srv    *Server
	cfg    *config.Config
	broker *fakeBroker
	cancel context.CancelFunc
	done   chan error
}

// startServer starts a server with the configuration and the fake broker.
// Each change runs before the start. The test cleanup stops the server.
func startServer(t *testing.T, cfg *config.Config, broker *fakeBroker, changes ...func(*Server)) *running {
	t.Helper()
	srv, err := New(Options{Config: cfg, Agent: broker, Admin: broker})
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range changes {
		change(srv)
	}
	if err := srv.Listen(); err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &running{srv: srv, cfg: cfg, broker: broker, cancel: cancel, done: make(chan error, 1)}
	go func() { r.done <- srv.Serve(ctx) }()
	t.Cleanup(func() { r.stop(t) })
	return r
}

// stop stops the server and waits for the end of Serve.
func (r *running) stop(t *testing.T) {
	t.Helper()
	r.cancel()
	select {
	case err, ok := <-r.done:
		if ok && err != nil {
			t.Errorf("Serve() error = %v", err)
		}
		if ok {
			close(r.done)
		}
	case <-time.After(testTimeout):
		t.Fatal("Serve() did not return after the cancel")
	}
}

// ctx returns a context with the test timeout.
func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	t.Cleanup(cancel)
	return ctx
}

// agent connects a session to the agent socket and sends hello.
func (r *running) agent(t *testing.T, name string) (*client.Conn, *protocol.Welcome) {
	t.Helper()
	c, err := client.DialAgent(testCtx(t), r.srv.AgentSocket())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	w, err := c.Hello(testCtx(t), name, "coding-agent")
	if err != nil {
		t.Fatalf("Hello(%q) error = %v", name, err)
	}
	return c, w
}

// request sends a request for one credential.
func request(t *testing.T, c *client.Conn, item string) (*protocol.Credentials, error) {
	t.Helper()
	ref, err := protocol.ParseReference("op://beispiel.app.k8s-dev/" + item + "/password")
	if err != nil {
		t.Fatal(err)
	}
	return c.RequestCredentials(testCtx(t), &protocol.RequestCredentials{References: []protocol.Reference{ref}, Task: "Run the tests"})
}

// wantCode checks that err has the code.
func wantCode(t *testing.T, err error, want protocol.Code) {
	t.Helper()
	if got := protocol.CodeOf(err); got != want {
		t.Fatalf("error = %v, want the code %s", err, want)
	}
}

// wantWorks checks that a new session gets a value.
func (r *running) wantWorks(t *testing.T) {
	t.Helper()
	c, _ := r.agent(t, "")
	creds, err := request(t, c, "datenbank-dev")
	if err != nil {
		t.Fatalf("the daemon does not serve a new session: %v", err)
	}
	if len(creds.Entries) != 1 || creds.Entries[0].Value == nil {
		t.Fatalf("the daemon does not serve a new session: %+v", creds)
	}
}

// isClosed reports whether err says that the connection ended.
func isClosed(err error) bool {
	return errors.Is(err, client.ErrClosed)
}

// fileMode returns the permission bits of a file.
func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fi.Mode().Perm()
}
