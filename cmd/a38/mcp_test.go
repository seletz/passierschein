package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/daemon"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

func TestMCPFlags(t *testing.T) {
	// D25, G19: the flag --session-name has priority over the variable
	// A38_SESSION_NAME; without both there is no name.
	tests := []struct {
		name string
		args []string
		env  string
		want string
	}{
		{"nothing", nil, "", ""},
		{"variable", nil, "beispiel-koordinator", "beispiel-koordinator"},
		{"flag", []string{"--session-name", "beispiel-flag"}, "", "beispiel-flag"},
		{"flag wins", []string{"--session-name", "beispiel-flag"}, "beispiel-env", "beispiel-flag"},
		{"empty flag wins", []string{"--session-name", ""}, "beispiel-env", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv := func(k string) string {
				if k == "A38_SESSION_NAME" {
					return tt.env
				}
				return ""
			}
			f, err := parseMCPFlags(tt.args, &bytes.Buffer{}, getenv)
			if err != nil {
				t.Fatal(err)
			}
			if f.sessionName != tt.want {
				t.Errorf("session name = %q, want %q", f.sessionName, tt.want)
			}
		})
	}
	if _, err := parseMCPFlags([]string{"extra"}, &bytes.Buffer{}, os.Getenv); err == nil {
		t.Error("an argument is accepted")
	}
}

// nameBroker records the claims of each session.
type nameBroker struct {
	daemon.Faulted
	mu    sync.Mutex
	names []string
}

func (b *nameBroker) GrantStatus(_ context.Context, s session.Session, _ *protocol.GrantStatus) (*protocol.Grants, *protocol.Error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.names = append(b.names, s.Name+"|"+s.Agent)
	return &protocol.Grants{Grants: []protocol.Grant{}}, nil
}

func TestMCPSessionNameReachesDaemon(t *testing.T) {
	// T21 (part): the name from the flag or from the variable reaches the
	// daemon in hello, with the name of the MCP client.
	for _, tt := range []struct {
		name string
		args []string
		env  string
		want string
	}{
		{"flag", []string{"--session-name", "beispiel-flag"}, "beispiel-env", "beispiel-flag|coding-agent"},
		{"variable", nil, "beispiel-env", "beispiel-env|coding-agent"},
		{"no name", nil, "", "|coding-agent"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfgPath, _ := writeConfig(t)
			cfg, err := config.Load(cfgPath)
			if err != nil {
				t.Fatal(err)
			}
			b := &nameBroker{}
			srv, err := daemon.New(daemon.Options{Config: cfg, Agent: b, Admin: b})
			if err != nil {
				t.Fatal(err)
			}
			if err := srv.Listen(); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			served := make(chan error, 1)
			go func() { served <- srv.Serve(ctx) }()
			defer func() {
				cancel()
				if err := <-served; err != nil {
					t.Errorf("Serve() error = %v", err)
				}
			}()

			st, ct := mcp.NewInMemoryTransports()
			var stderr syncBuffer
			done := make(chan int, 1)
			getenv := func(k string) string {
				if k == "A38_SESSION_NAME" {
					return tt.env
				}
				return ""
			}
			go func() {
				done <- runMCPWith(ctx, append([]string{"--config", cfgPath}, tt.args...), st, &stderr, getenv)
			}()
			cs, err := mcp.NewClient(&mcp.Implementation{Name: "coding-agent", Version: "1"}, nil).Connect(ctx, ct, nil)
			if err != nil {
				t.Fatal(err)
			}
			res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "grant_status"})
			if err != nil || res.IsError {
				t.Fatalf("grant_status: %v %+v; stderr:\n%s", err, res, stderr.String())
			}
			_ = cs.Close()
			if code := <-done; code != exitOK {
				t.Errorf("exit code = %d; stderr:\n%s", code, stderr.String())
			}
			b.mu.Lock()
			defer b.mu.Unlock()
			if len(b.names) != 1 || b.names[0] != tt.want {
				t.Errorf("claims = %v, want %q", b.names, tt.want)
			}
		})
	}
}

func TestMCPProcessesEndToEnd(t *testing.T) {
	// T1 (part): the real processes "a38 serve" and "a38 mcp". The daemon
	// has the placeholder handler, so the call ends with daemon_faulted.
	// When the broker is wired in, only the expectation changes.
	if testing.Short() {
		t.Skip("builds the binary")
	}
	bin := filepath.Join(t.TempDir(), "a38")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	cfgPath, socketDir := writeConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var serveErr syncBuffer
	serve := exec.CommandContext(ctx, bin, "serve", "--config", cfgPath, "--backend", "fake", "--ui", "auto-approve")
	serve.Stderr = &serveErr
	if err := serve.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); _ = serve.Wait() })
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(socketDir, "agent.sock")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no agent socket; stderr:\n%s", serveErr.String())
		}
		time.Sleep(10 * time.Millisecond)
	}

	cmd := exec.CommandContext(ctx, bin, "mcp", "--config", cfgPath, "--session-name", "beispiel-koordinator")
	var mcpErr syncBuffer
	cmd.Stderr = &mcpErr
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "coding-agent", Version: "1"}, nil).
		Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect: %v; stderr:\n%s", err, mcpErr.String())
	}
	defer func() { _ = cs.Close() }()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "request_credentials",
		Arguments: map[string]any{
			"references": []string{"op://beispiel.app.k8s-dev/datenbank-dev/password"},
			"task":       "Run the tests", "reason": "The tests need the login.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	sc, _ := json.Marshal(res.StructuredContent)
	if !res.IsError || !strings.Contains(string(sc), `"code":"daemon_faulted"`) {
		t.Errorf("result = isError %v, %s; want daemon_faulted", res.IsError, sc)
	}
}
