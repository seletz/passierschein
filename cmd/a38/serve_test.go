package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/client"
	"github.com/seletz/passierschein/internal/protocol"
)

// syncBuffer is a buffer that more than one goroutine can write.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// writeConfig writes a configuration file with the sockets in a short
// directory under /tmp and returns its path and the socket directory.
func writeConfig(t *testing.T) (path, socketDir string) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "a38-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socketDir = filepath.Join(dir, "run")
	path = filepath.Join(dir, "config.yaml")
	data := fmt.Sprintf("version: 1\npaths:\n  socket_dir: %s\n  audit_file: %s\n", socketDir, filepath.Join(dir, "audit.db"))
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path, socketDir
}

func TestServeFlags(t *testing.T) {
	cfg, _ := writeConfig(t)
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{"no backend", []string{"--config", cfg, "--ui", "auto-approve"}, "--backend must be one of"},
		{"unknown backend", []string{"--config", cfg, "--backend", "vault", "--ui", "auto-approve"}, "--backend must be one of"},
		{"no ui", []string{"--config", cfg, "--backend", "fake"}, "--ui must be one of"},
		{"unknown ui", []string{"--config", cfg, "--backend", "fake", "--ui", "popup"}, "--ui must be one of"},
		{"real backend with fake approval", []string{"--config", cfg, "--backend", "1password", "--ui", "auto-approve"}, "--backend 1password needs --ui app"},
		{"real backend with script", []string{"--config", cfg, "--backend", "1password", "--ui", "script"}, "--backend 1password needs --ui app"},
		{"argument", []string{"--config", cfg, "--backend", "fake", "--ui", "auto-deny", "extra"}, "unexpected argument"},
		{"unknown flag", []string{"--verbose"}, "flag provided but not defined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := serve(context.Background(), tt.args, &stdout, &stderr); code != exitUsage {
				t.Errorf("exit code = %d, want %d", code, exitUsage)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestServeConfigPriority(t *testing.T) {
	// D49: the flag --config has priority over A38_CONFIG.
	good, _ := writeConfig(t)
	missing := "/tmp/a38-missing/config.yaml"
	tests := []struct {
		name     string
		env      string
		flag     string
		wantCode int
	}{
		{"flag wins over the variable", missing, good, exitOK},
		{"variable without flag", good, "", exitOK},
		{"flag with a missing file", good, missing, exitFailure},
		{"variable with a missing file", missing, "", exitFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("A38_CONFIG", tt.env)
			args := []string{"--backend", "fake", "--ui", "auto-approve"}
			if tt.flag != "" {
				args = append(args, "--config", tt.flag)
			}
			// A cancelled context stops the daemon after the start.
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			var stdout, stderr bytes.Buffer
			if code := serve(ctx, args, &stdout, &stderr); code != tt.wantCode {
				t.Errorf("exit code = %d, want %d; stderr:\n%s", code, tt.wantCode, stderr.String())
			}
		})
	}
}

func TestServeRuns(t *testing.T) {
	// The daemon starts, answers each request with daemon_faulted until
	// the broker exists, and stops cleanly.
	cfg, socketDir := writeConfig(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stdout, stderr syncBuffer
	done := make(chan int, 1)
	go func() {
		done <- serve(ctx, []string{"--config", cfg, "--backend", "fake", "--ui", "auto-approve"}, &stdout, &stderr)
	}()
	agentSocket := filepath.Join(socketDir, "agent.sock")
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(agentSocket); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the agent socket does not exist; stderr:\n%s", stderr.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	reqCtx, reqCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer reqCancel()

	c, err := client.DialAgent(reqCtx, agentSocket)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Hello(reqCtx, "beispiel-koordinator", "coding-agent"); err != nil {
		t.Fatalf("Hello() error = %v", err)
	}
	ref, _ := protocol.ParseReference("op://beispiel.app.k8s-dev/datenbank-dev/password")
	_, err = c.RequestCredentials(reqCtx, &protocol.RequestCredentials{References: []protocol.Reference{ref}})
	if protocol.CodeOf(err) != protocol.CodeDaemonFaulted {
		t.Errorf("request error = %v, want daemon_faulted", err)
	}
	admin, err := client.DialAdmin(reqCtx, filepath.Join(socketDir, "admin.sock"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Status(reqCtx); protocol.CodeOf(err) != protocol.CodeDaemonFaulted {
		t.Errorf("status error = %v, want daemon_faulted", err)
	}

	cancel()
	if code := <-done; code != exitOK {
		t.Errorf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
	}
	_ = c.Close()
	_ = admin.Close()
	if _, err := os.Stat(agentSocket); !os.IsNotExist(err) {
		t.Error("the agent socket exists after the stop")
	}
	log := stderr.String()
	for _, want := range []string{"each request ends with daemon_faulted", "daemon stopped", "session started", "session ended"} {
		if !strings.Contains(log, want) {
			t.Errorf("log does not contain %q:\n%s", want, log)
		}
	}
}
