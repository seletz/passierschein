package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/seletz/passierschein/internal/client"
	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/daemon"
	a38mcp "github.com/seletz/passierschein/internal/mcp"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

const (
	testTimeout = 10 * time.Second
	// marker is a neutral stand-in for a secret value.
	marker = "MARKER-beispiel-wert-4711"
	refOK  = "op://beispiel.app.k8s-dev/datenbank-dev/password"
)

// fakeBroker implements daemon.AgentHandler and daemon.AdminHandler.
type fakeBroker struct {
	daemon.Faulted
	mu       sync.Mutex
	sessions []session.Session
	creds    func(req *protocol.RequestCredentials) (*protocol.Credentials, *protocol.Error)
	grants   *protocol.Grants
}

func (b *fakeBroker) RequestCredentials(_ context.Context, s session.Session, req *protocol.RequestCredentials) (*protocol.Credentials, *protocol.Error) {
	b.mu.Lock()
	b.sessions = append(b.sessions, s)
	b.mu.Unlock()
	return b.creds(req)
}

func (b *fakeBroker) GrantStatus(_ context.Context, s session.Session, _ *protocol.GrantStatus) (*protocol.Grants, *protocol.Error) {
	b.mu.Lock()
	b.sessions = append(b.sessions, s)
	b.mu.Unlock()
	return b.grants, nil
}

func (b *fakeBroker) lastSession() session.Session {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.sessions[len(b.sessions)-1]
}

// valueFor answers each reference with the marker value.
func valueFor(req *protocol.RequestCredentials) (*protocol.Credentials, *protocol.Error) {
	out := &protocol.Credentials{}
	for _, ref := range req.References {
		v := protocol.NewSecret(marker)
		out.Entries = append(out.Entries, protocol.CredentialEntry{Reference: ref, Value: &v})
	}
	return out, nil
}

// shortDir returns a short directory under /tmp for sockets.
func shortDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "a38-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	dir := shortDir(t)
	yaml := fmt.Sprintf("version: 1\nbackend:\n  onepassword:\n    accounts:\n      - name: privat\nvaults:\n  - name: beispiel.app.k8s-dev\n    environment: dev\n    account: privat\npaths:\n  socket_dir: %s/run\n  audit_file: %s/audit.db\n", dir, dir)
	cfg, err := config.Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// startDaemon starts the socket layer with the broker.
func startDaemon(t *testing.T, cfg *config.Config, b *fakeBroker) *daemon.Server {
	t.Helper()
	srv, err := daemon.New(daemon.Options{Config: cfg, Agent: b, Admin: b})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Serve() error = %v", err)
		}
	})
	return srv
}

// connectClient starts the MCP server over an in-memory transport and
// returns an MCP client session. name is the client name.
func connectClient(t *testing.T, socket, sessionName, clientName string, logs *syncBuilder) *mcp.ClientSession {
	t.Helper()
	opts := a38mcp.Options{
		Dial:        func(ctx context.Context) (*client.Conn, error) { return client.DialAgent(ctx, socket) },
		SessionName: sessionName,
		Version:     "test",
	}
	if logs != nil {
		opts.Logger = newLogger(logs)
	}
	srv := a38mcp.New(opts)
	st, ct := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = srv.Run(ctx, st); close(done) }()
	c := mcp.NewClient(&mcp.Implementation{Name: clientName, Version: "1"}, nil)
	cs, err := c.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close(); cancel(); <-done })
	return cs
}

func call(t *testing.T, cs *mcp.ClientSession, tool string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s) error = %v", tool, err)
	}
	return res
}

func request(refs ...string) map[string]any {
	return map[string]any{"references": refs, "task": "Run the tests", "reason": "The tests need the login."}
}

// errCode returns the code of a tool error and checks text and structure.
func errCode(t *testing.T, res *mcp.CallToolResult) protocol.Code {
	t.Helper()
	if !res.IsError {
		t.Fatalf("result is not an error: %s", text(res))
	}
	sc, _ := json.Marshal(res.StructuredContent)
	var got struct {
		Error struct{ Code, Message string }
	}
	if err := json.Unmarshal(sc, &got); err != nil || got.Error.Code == "" || got.Error.Message == "" {
		t.Fatalf("structured content = %s, want error code and message", sc)
	}
	if want := got.Error.Code + ": " + got.Error.Message; text(res) != want {
		t.Errorf("text = %q, want %q", text(res), want)
	}
	return protocol.Code(got.Error.Code)
}

func text(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// entries decodes the entries of a successful request.
func entries(t *testing.T, res *mcp.CallToolResult) []struct {
	Reference string
	Value     *string
	Error     *struct{ Code, Message string }
} {
	t.Helper()
	if res.IsError {
		t.Fatalf("unexpected error: %s", text(res))
	}
	sc, _ := json.Marshal(res.StructuredContent)
	var got struct {
		Entries []struct {
			Reference string
			Value     *string
			Error     *struct{ Code, Message string }
		}
	}
	if err := json.Unmarshal(sc, &got); err != nil {
		t.Fatal(err)
	}
	return got.Entries
}

func TestListTools(t *testing.T) {
	// T1 (part): the server offers the two tools of D41.
	cfg := testConfig(t)
	d := startDaemon(t, cfg, &fakeBroker{creds: valueFor})
	cs := connectClient(t, d.AgentSocket(), "", "coding-agent", nil)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]*mcp.Tool{}
	for _, tool := range res.Tools {
		got[tool.Name] = tool
	}
	if len(got) != 2 || got["request_credentials"] == nil || got["grant_status"] == nil {
		t.Fatalf("tools = %v, want request_credentials and grant_status", got)
	}
	schema, _ := json.Marshal(got["request_credentials"].InputSchema)
	for _, p := range []string{`"references"`, `"task"`, `"reason"`} {
		if !strings.Contains(string(schema), p) {
			t.Errorf("schema %s lacks %s", schema, p)
		}
	}
	if got["request_credentials"].Description == "" || got["grant_status"].Description == "" {
		t.Error("a tool has no description")
	}
}

func TestRequestCredentialsSuccessAndMixed(t *testing.T) {
	// T1 (part): a request returns per reference the value or the code, in
	// the order of the request.
	const ref2 = "op://beispiel.app.k8s-dev/datenbank-dev/user"
	b := &fakeBroker{creds: func(req *protocol.RequestCredentials) (*protocol.Credentials, *protocol.Error) {
		v := protocol.NewSecret(marker)
		return &protocol.Credentials{Entries: []protocol.CredentialEntry{
			{Reference: req.References[0], Value: &v},
			{Reference: req.References[1], Error: protocol.Errorf(protocol.CodeDenied, "the human denied the request")},
		}}, nil
	}}
	d := startDaemon(t, testConfig(t), b)
	cs := connectClient(t, d.AgentSocket(), "beispiel-koordinator", "coding-agent", nil)
	res := call(t, cs, "request_credentials", request(refOK, ref2))
	got := entries(t, res)
	if len(got) != 2 || got[0].Reference != refOK || got[1].Reference != ref2 {
		t.Fatalf("entries = %+v, want the order of the request", got)
	}
	if got[0].Value == nil || *got[0].Value != marker || got[0].Error != nil {
		t.Errorf("entry 0 = %+v, want the value", got[0])
	}
	if got[1].Value != nil || got[1].Error == nil || got[1].Error.Code != "denied" {
		t.Errorf("entry 1 = %+v, want the code denied", got[1])
	}
	// The daemon got the claims; the name of the client is the agent.
	s := b.lastSession()
	if s.Name != "beispiel-koordinator" || s.Agent != "coding-agent" {
		t.Errorf("session = name %q agent %q", s.Name, s.Agent)
	}
}

func TestWholeRequestErrors(t *testing.T) {
	// T1 (part): each protocol error code reaches the agent as a tool error
	// with the stable code.
	for _, code := range protocol.Codes() {
		if code == protocol.CodeDaemonUnavailable {
			continue // only a38 mcp creates it, see TestDaemonUnavailable
		}
		t.Run(string(code), func(t *testing.T) {
			b := &fakeBroker{creds: func(*protocol.RequestCredentials) (*protocol.Credentials, *protocol.Error) {
				return nil, protocol.Errorf(code, "the daemon says %s", code)
			}}
			d := startDaemon(t, testConfig(t), b)
			cs := connectClient(t, d.AgentSocket(), "", "coding-agent", nil)
			res := call(t, cs, "request_credentials", request(refOK))
			if got := errCode(t, res); got != code {
				t.Errorf("code = %q, want %q", got, code)
			}
		})
	}
}

func TestInvalidReferences(t *testing.T) {
	// T24 (part): an invalid reference ends with invalid_request. a38 mcp
	// rejects it before it sends it; the daemon checks again.
	b := &fakeBroker{creds: valueFor}
	d := startDaemon(t, testConfig(t), b)
	cs := connectClient(t, d.AgentSocket(), "", "coding-agent", nil)
	tests := []struct{ name, ref string }{
		{"two segments", "op://vault/item"},
		{"four segments", "op://vault/item/field/extra"},
		{"other scheme", "https://vault/item/field"},
		{"empty segment", "op://vault//field"},
		{"empty field", "op://vault/item/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := call(t, cs, "request_credentials", request(tt.ref))
			if got := errCode(t, res); got != protocol.CodeInvalidRequest {
				t.Errorf("code = %q, want invalid_request", got)
			}
		})
	}
	for _, args := range []map[string]any{
		{"references": []string{}},
		{"task": "no references"},
	} {
		// The schema of the SDK or the tool rejects these.
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "request_credentials", Arguments: args})
		if err == nil && !res.IsError {
			t.Errorf("arguments %v: no error", args)
		}
	}
	b.mu.Lock()
	n := len(b.sessions)
	b.mu.Unlock()
	if n != 0 {
		t.Errorf("the daemon got %d requests, want 0: a38 mcp rejects early", n)
	}
}

func TestDaemonRejectsReference(t *testing.T) {
	// T24 (part): the daemon stays the authority. Its invalid_request
	// arrives unchanged.
	b := &fakeBroker{creds: func(*protocol.RequestCredentials) (*protocol.Credentials, *protocol.Error) {
		return nil, protocol.Errorf(protocol.CodeInvalidRequest, "the daemon does not accept the request")
	}}
	d := startDaemon(t, testConfig(t), b)
	cs := connectClient(t, d.AgentSocket(), "", "coding-agent", nil)
	res := call(t, cs, "request_credentials", request(refOK))
	if got := errCode(t, res); got != protocol.CodeInvalidRequest {
		t.Errorf("code = %q", got)
	}
	if !strings.Contains(text(res), "the daemon does not accept") {
		t.Errorf("text = %q, want the message of the daemon", text(res))
	}
}

func TestGrantStatusHasNoValue(t *testing.T) {
	// T25 (part): grant_status returns grants and no value.
	now := time.Now().UTC().Truncate(time.Second)
	ref, _ := protocol.ParseReference(refOK)
	b := &fakeBroker{creds: valueFor, grants: &protocol.Grants{Grants: []protocol.Grant{{
		GrantID: "g-1", References: []protocol.Reference{ref}, GrantedAt: now, ExpiresAt: now.Add(time.Minute), RemainingSeconds: 60,
	}}}}
	d := startDaemon(t, testConfig(t), b)
	cs := connectClient(t, d.AgentSocket(), "", "coding-agent", nil)
	res := call(t, cs, "grant_status", nil)
	if res.IsError {
		t.Fatalf("error: %s", text(res))
	}
	sc, _ := json.Marshal(res.StructuredContent)
	var got struct {
		Grants []struct {
			GrantID          string   `json:"grant_id"`
			References       []string `json:"references"`
			RemainingSeconds int64    `json:"remaining_seconds"`
		}
	}
	if err := json.Unmarshal(sc, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Grants) != 1 || got.Grants[0].GrantID != "g-1" || got.Grants[0].References[0] != refOK || got.Grants[0].RemainingSeconds != 60 {
		t.Errorf("grants = %s", sc)
	}
	if strings.Contains(string(sc), marker) || strings.Contains(text(res), marker) || strings.Contains(string(sc), `"value"`) {
		t.Error("grant_status result holds a value")
	}
	if res := call(t, cs, "grant_status", map[string]any{"x": 1}); res != nil && !res.IsError {
		t.Error("grant_status accepts an argument")
	}
}

func TestDaemonUnavailableThenReachable(t *testing.T) {
	// D35: no daemon: daemon_unavailable on each call; a38 mcp does not
	// exit and connects again on the next call.
	cfg := testConfig(t)
	b := &fakeBroker{creds: valueFor, grants: &protocol.Grants{Grants: []protocol.Grant{}}}
	socket := cfg.AgentSocket()
	cs := connectClient(t, socket, "", "coding-agent", nil)
	for _, tool := range []string{"request_credentials", "grant_status"} {
		args := request(refOK)
		if tool == "grant_status" {
			args = nil
		}
		res := call(t, cs, tool, args)
		if got := errCode(t, res); got != protocol.CodeDaemonUnavailable {
			t.Errorf("%s: code = %q, want daemon_unavailable", tool, got)
		}
		if !strings.Contains(text(res), "new session") || !strings.Contains(text(res), "a38 serve") {
			t.Errorf("%s: text = %q, want the hint about the new session and a38 serve", tool, text(res))
		}
	}
	// The daemon starts: the next call works.
	srv, err := daemon.New(daemon.Options{Config: cfg, Agent: b, Admin: b})
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()
	res := call(t, cs, "request_credentials", request(refOK))
	if e := entries(t, res); len(e) != 1 || e[0].Value == nil {
		t.Fatalf("after start: %s", text(res))
	}
	// The daemon stops: unavailable again, and available after a restart.
	cancel()
	if err := <-done; err != nil {
		t.Errorf("Serve() error = %v", err)
	}
	deadline := time.Now().Add(testTimeout)
	for {
		res = call(t, cs, "grant_status", nil)
		if res.IsError {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the daemon stopped, but grant_status still works")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := errCode(t, res); got != protocol.CodeDaemonUnavailable {
		t.Errorf("after stop: code = %q", got)
	}
	startDaemon(t, cfg, b)
	if res = call(t, cs, "grant_status", nil); res.IsError {
		t.Errorf("after restart: %s", text(res))
	}
}

func TestSessionName(t *testing.T) {
	// T21 (part): the name reaches the daemon; the daemon rejects a bad
	// name with invalid_request and a38 mcp reports it.
	tests := []struct {
		name     string
		session  string
		wantCode protocol.Code
	}{
		{"no name", "", ""},
		{"valid name", "beispiel-koordinator", ""},
		{"character not allowed", "beispiel/koordinator", protocol.CodeInvalidRequest},
		{"control character", "beispiel\x07koordinator", protocol.CodeInvalidRequest},
		{"too long", strings.Repeat("a", 81), protocol.CodeInvalidRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &fakeBroker{creds: valueFor}
			d := startDaemon(t, testConfig(t), b)
			cs := connectClient(t, d.AgentSocket(), tt.session, "coding-agent", nil)
			res := call(t, cs, "request_credentials", request(refOK))
			if tt.wantCode == "" {
				if res.IsError {
					t.Fatalf("error: %s", text(res))
				}
				if got := b.lastSession().Name; got != tt.session {
					t.Errorf("session name = %q, want %q", got, tt.session)
				}
				return
			}
			if got := errCode(t, res); got != tt.wantCode {
				t.Errorf("code = %q, want %q", got, tt.wantCode)
			}
		})
	}
}

func TestSecretOnlyInResult(t *testing.T) {
	// T1 (part): the value is in the result of the approved entry and
	// nowhere else: not in the log, not in an error.
	var logs syncBuilder
	b := &fakeBroker{creds: valueFor}
	d := startDaemon(t, testConfig(t), b)
	cs := connectClient(t, d.AgentSocket(), "", "coding-agent", &logs)
	res := call(t, cs, "request_credentials", request(refOK))
	if !strings.Contains(text(res), marker) {
		t.Fatal("the result has no value")
	}
	// A transport failure and a rejected request leave no value in a log.
	call(t, cs, "request_credentials", request("op://bad"))
	call(t, cs, "grant_status", map[string]any{"x": 1})
	if strings.Contains(logs.String(), marker) {
		t.Errorf("the log holds the value: %s", logs.String())
	}
}

// syncBuilder is a log buffer that more than one goroutine can write.
type syncBuilder struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuilder) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuilder) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func newLogger(w io.Writer) *slog.Logger { return slog.New(slog.NewTextHandler(w, nil)) }
