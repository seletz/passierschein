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
			cfgPath, _, _ := writeConfig(t)
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
	// T1: the real processes "a38 serve --backend fake --ui auto-approve"
	// and "a38 mcp". An MCP client asks for two references of a dev
	// vault. The answer has both values, and the audit log has request,
	// bundle, decision, grant and delivery in this sequence. A second case
	// with auto-deny delivers no value and writes the denial.
	if testing.Short() {
		t.Skip("builds the binary")
	}
	bin := filepath.Join(t.TempDir(), "a38")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	refs := []string{
		"op://beispiel.app.k8s-dev/datenbank-dev/username",
		"op://beispiel.app.k8s-dev/datenbank-dev/password",
	}
	tests := []struct {
		ui         string
		wantValues map[string]string
		wantCode   protocol.Code
		wantAudit  []string
	}{
		{
			ui:         "auto-approve",
			wantValues: map[string]string{refs[0]: "beispiel-wert-user", refs[1]: "beispiel-wert-serve"},
			wantAudit:  []string{"request", "bundle", "decision", "grant", "delivery"},
		},
		{
			ui:        "auto-deny",
			wantCode:  protocol.CodeDenied,
			wantAudit: []string{"request", "bundle", "denial"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.ui, func(t *testing.T) {
			cfgPath, socketDir, fakeFile := writeConfig(t)
			stop := startServe(t, bin, cfgPath, socketDir, "--backend", "fake", "--fake-file", fakeFile, "--ui", tt.ui)
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			cmd := a38Cmd(ctx, t, bin, "mcp", "--config", cfgPath, "--session-name", "beispiel-koordinator")
			var mcpErr syncBuffer
			cmd.Stderr = &mcpErr
			cs, err := mcp.NewClient(&mcp.Implementation{Name: "coding-agent", Version: "1"}, nil).
				Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
			if err != nil {
				t.Fatalf("connect: %v; stderr:\n%s", err, mcpErr.String())
			}
			res, err := cs.CallTool(ctx, &mcp.CallToolParams{
				Name: "request_credentials",
				Arguments: map[string]any{
					"references": refs,
					"task":       "Run the tests", "reason": "The tests need the login.",
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			_ = cs.Close()
			sc, _ := json.Marshal(res.StructuredContent)
			var got struct {
				Entries []struct {
					Reference string  `json:"reference"`
					Value     *string `json:"value"`
				} `json:"entries"`
				Error *struct {
					Code protocol.Code `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(sc, &got); err != nil {
				t.Fatal(err)
			}
			if tt.wantCode != "" {
				if !res.IsError || got.Error == nil || got.Error.Code != tt.wantCode || strings.Contains(string(sc), "beispiel-wert") {
					t.Errorf("result: isError %v, error %+v; want %s and no value", res.IsError, got.Error, tt.wantCode)
				}
			} else {
				values := map[string]string{}
				for _, e := range got.Entries {
					if e.Value != nil {
						values[e.Reference] = *e.Value
					}
				}
				if res.IsError || len(values) != 2 || values[refs[0]] != tt.wantValues[refs[0]] || values[refs[1]] != tt.wantValues[refs[1]] {
					t.Errorf("result: isError %v, %d values; want both values", res.IsError, len(values))
				}
			}
			stop()
			// The end of the session can add the expiry of the grant.
			types := auditTypes(t, bin, cfgPath)
			if len(types) < len(tt.wantAudit) || !slicesEqual(types[:len(tt.wantAudit)], tt.wantAudit) {
				t.Errorf("audit = %v, want %v first", types, tt.wantAudit)
			}
			for _, rest := range types[min(len(types), len(tt.wantAudit)):] {
				if rest != "expiry" {
					t.Errorf("audit = %v: unexpected event %q", types, rest)
				}
			}
		})
	}
}

// startServe starts "a38 serve" as a process and waits for the line
// "daemon started" (waitDaemonStarted). The returned function stops the daemon and waits for its end.
func startServe(t *testing.T, bin, cfgPath, socketDir string, args ...string) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	var serveErr syncBuffer
	serve := a38Cmd(ctx, t, bin, append([]string{"serve", "--config", cfgPath}, args...)...)
	serve.Stderr = &serveErr
	serve.Cancel = func() error { return serve.Process.Signal(os.Interrupt) }
	if err := serve.Start(); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			_ = serve.Wait()
			if strings.Contains(serveErr.String(), "beispiel-wert") {
				t.Error("the log of the daemon contains a value")
			}
		})
	}
	t.Cleanup(stop)
	waitDaemonStarted(t, &serveErr, nil)
	return stop
}

// auditTypes reads the event types with "a38 audit list --json".
func auditTypes(t *testing.T, bin, cfgPath string) []string {
	t.Helper()
	out, err := a38Cmd(context.Background(), t, bin, "audit", "list", "--config", cfgPath, "--json").Output()
	if err != nil {
		t.Fatalf("a38 audit list: %v", err)
	}
	if strings.Contains(string(out), "beispiel-wert") {
		t.Error("the audit log contains a value")
	}
	var types []string
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var e struct {
			Type string `json:"type"`
		}
		if err := dec.Decode(&e); err != nil {
			t.Fatal(err)
		}
		types = append(types, e.Type)
	}
	return types
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMCPRunWithCredentialsWiring(t *testing.T) {
	// T34 step 5 (part): "a38 mcp" checks cwd against the projects of the
	// configuration. A cwd in the project reaches the daemon, which
	// answers not_allowed until the broker supports the exec way.
	cfgPath, _, _ := writeConfig(t)
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
	go func() {
		done <- runMCPWith(ctx, []string{"--config", cfgPath}, st, &stderr, func(string) string { return "" })
	}()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "coding-agent", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		cwd  string
		want protocol.Code
	}{
		{"", protocol.CodeNotAllowed},
		{outside, protocol.CodeInvalidRequest},
	} {
		args := map[string]any{
			"command": "/bin/sh",
			"args":    []string{"-c", "exit 0"},
			"env":     map[string]string{"DB_PASSWORD": "op://beispiel.app.k8s-dev/datenbank-dev/password"},
			"task":    "Run the migration of beispielprojekt",
			"reason":  "The migration needs the password of datenbank-dev.",
		}
		if tt.cwd != "" {
			args["cwd"] = tt.cwd
		}
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "run_with_credentials", Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		sc, _ := json.Marshal(res.StructuredContent)
		var got struct {
			Error struct {
				Code protocol.Code `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(sc, &got); err != nil || !res.IsError || got.Error.Code != tt.want {
			t.Errorf("cwd %q: result %s, want %s", tt.cwd, sc, tt.want)
		}
	}
	_ = cs.Close()
	if code := <-done; code != exitOK {
		t.Errorf("exit code = %d; stderr:\n%s", code, stderr.String())
	}
}
