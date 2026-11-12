package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/client"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/token"
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

// fakeBackend is the fake backend of the tests. The values are fake.
const fakeBackend = `vaults:
  - name: beispiel.app.k8s-dev
    items:
      - name: datenbank-dev
        fields:
          - name: username
            value: beispiel-wert-user
          - name: password
            value: beispiel-wert-serve
`

// writeConfig writes a configuration file with the sockets in a short
// directory under /tmp and a fake backend file. The project
// "beispielprojekt" contains the working directory of the test. It returns
// the path of the configuration, the socket directory and the path of the
// fake backend file.
func writeConfig(t *testing.T) (path, socketDir, fakeFile string) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "a38-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if wd, err = filepath.EvalSymlinks(wd); err != nil {
		t.Fatal(err)
	}
	socketDir = filepath.Join(dir, "run")
	path = filepath.Join(dir, "config.yaml")
	data := fmt.Sprintf(`version: 1
vaults:
  - {name: beispiel.app.k8s-dev, environment: dev}
projects:
  - name: beispielprojekt
    paths: [%q]
    vaults: [beispiel.app.k8s-dev]
bundle:
  window: 50ms
paths:
  socket_dir: %s
  audit_file: %s
# A token file in the test directory: a test never reads the token file
# of the user (rule for 1Password, 2026-11-12). The source is the file,
# so that no test and no built binary touches the keychain (issue 59).
backend:
  onepassword:
    token_source: file
    token_file: %s
`, wd, socketDir, filepath.Join(dir, "audit.db"), filepath.Join(dir, "onepassword-token"))
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	fakeFile = filepath.Join(dir, "fake.yaml")
	if err := os.WriteFile(fakeFile, []byte(fakeBackend), 0o600); err != nil {
		t.Fatal(err)
	}
	return path, socketDir, fakeFile
}

func TestServeFlags(t *testing.T) {
	cfg, _, _ := writeConfig(t)
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
		// T9: each faked approval with the real backend kind stops the
		// start before the token file is read and before a socket opens.
		{"real backend with auto-deny", []string{"--config", cfg, "--backend", "1password", "--ui", "auto-deny"}, "--backend 1password needs --ui app"},
		{"argument", []string{"--config", cfg, "--backend", "fake", "--ui", "auto-deny", "extra"}, "unexpected argument"},
		{"unknown flag", []string{"--verbose"}, "flag provided but not defined"},
		{"fake backend without file", []string{"--config", cfg, "--backend", "fake", "--ui", "auto-deny"}, "--backend fake needs --fake-file"},
		{"ui-app without app mode", []string{"--config", cfg, "--backend", "fake", "--fake-file", "x.yaml", "--ui", "auto-deny", "--ui-app", "/x"}, "--ui-app needs --ui app"},
		{"script without program", []string{"--config", cfg, "--backend", "fake", "--fake-file", "x.yaml", "--ui", "script"}, "--ui script needs --ui-script"},
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
	good, _, fakeFile := writeConfig(t)
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
			args := []string{"--backend", "fake", "--fake-file", fakeFile, "--ui", "auto-approve"}
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

// TestServeStartFailures: the daemon does not start without its parts.
func TestServeStartFailures(t *testing.T) {
	cfg, socketDir, _ := writeConfig(t)
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{"missing fake file", []string{"--backend", "fake", "--fake-file", "/tmp/a38-missing/fake.yaml", "--ui", "auto-approve"}, "fake backend"},
	}
	// D42, G3: the 1Password backend does not start without a good token
	// file; on linux it does not exist at all.
	opArgs := []string{"--backend", "1password", "--ui", "app", "--ui-app", "/tmp/a38-missing/Passierschein"}
	tokenFile := filepath.Join(filepath.Dir(cfg), "onepassword-token")
	opCases := []struct {
		name, want string
		prepare    func(t *testing.T)
	}{
		{"1password without token file", "token file does not exist", func(*testing.T) {}},
		{"1password with token file mode 0644", "mode 0600", func(t *testing.T) {
			writeMode(t, tokenFile, 0o644)
		}},
		{"1password with token file as directory", "not a regular file", func(t *testing.T) {
			if err := os.Mkdir(tokenFile, 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		{"1password with token file as link", "symbolic link", func(t *testing.T) {
			writeMode(t, tokenFile+".real", 0o600)
			if err := os.Symlink(tokenFile+".real", tokenFile); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, c := range opCases {
		want := c.want
		if runtime.GOOS != "darwin" {
			want = "not available on this platform"
		}
		tests = append(tests, struct {
			name       string
			args       []string
			wantStderr string
		}{c.name, opArgs, want})
	}
	prepare := map[string]func(t *testing.T){}
	for _, c := range opCases {
		prepare[c.name] = c.prepare
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.RemoveAll(tokenFile)
			_ = os.Remove(tokenFile + ".real")
			if p := prepare[tt.name]; p != nil {
				p(t)
			}
			var stdout, stderr bytes.Buffer
			args := append([]string{"--config", cfg}, tt.args...)
			if code := serve(context.Background(), args, &stdout, &stderr); code != exitFailure {
				t.Errorf("exit code = %d, want %d", code, exitFailure)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want %q", stderr.String(), tt.wantStderr)
			}
			if strings.Contains(stderr.String(), "kein-token") {
				t.Error("stderr contains the content of the token file")
			}
			if _, err := os.Stat(filepath.Join(socketDir, "agent.sock")); !os.IsNotExist(err) {
				t.Error("a socket exists after a failed start")
			}
		})
	}
}

func TestServeRuns(t *testing.T) {
	// T1 (part): the daemon starts with the broker, delivers a value
	// with automatic approval, shows the grant and stops cleanly.
	cfg, socketDir, fakeFile := writeConfig(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stdout, stderr syncBuffer
	done := make(chan int, 1)
	go func() {
		done <- serve(ctx, []string{"--config", cfg, "--backend", "fake", "--fake-file", fakeFile, "--ui", "auto-approve"}, &stdout, &stderr)
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
	creds, err := c.RequestCredentials(reqCtx, &protocol.RequestCredentials{References: []protocol.Reference{ref}})
	if err != nil {
		t.Fatalf("request error = %v", err)
	}
	if len(creds.Entries) != 1 || creds.Entries[0].Value == nil || creds.Entries[0].Value.Reveal() != "beispiel-wert-serve" {
		t.Fatalf("the answer has no value")
	}
	grants, err := c.GrantStatus(reqCtx)
	if err != nil || len(grants.Grants) != 1 {
		t.Errorf("grant status = %+v, %v", grants, err)
	}
	other, _ := protocol.ParseReference("op://beispiel.app.k8s-prod/datenbank/password")
	if _, err := c.RequestCredentials(reqCtx, &protocol.RequestCredentials{References: []protocol.Reference{other}}); protocol.CodeOf(err) != protocol.CodeNotAllowed {
		t.Errorf("request outside the policy: %v, want not_allowed", err)
	}
	admin, err := client.DialAdmin(reqCtx, filepath.Join(socketDir, "admin.sock"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := admin.Status(reqCtx)
	if err != nil || st.Grants != 1 || st.Fault != nil {
		t.Errorf("status = %+v, %v", st, err)
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
	for _, want := range []string{"daemon started", "daemon stopped", "session started", "session ended"} {
		if !strings.Contains(log, want) {
			t.Errorf("log does not contain %q:\n%s", want, log)
		}
	}
	if strings.Contains(log, "beispiel-wert-serve") {
		t.Error("the log contains the value")
	}
}

// writeMode writes a fake token file with the mode.
func writeMode(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte("ops_beispiel-kein-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

// TestServeKeychainSource: with the source keychain (the default) a
// missing keychain item stops the start; the message names
// "a38 config onepassword-token set" (D51, issue 59). The test uses a fake store; it never
// touches the keychain.
func TestServeKeychainSource(t *testing.T) {
	cfgFile, socketDir, _ := writeConfig(t)
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ name, from, to string }{
		{"default source", "    token_source: file\n", ""},
		{"source keychain", "    token_source: file\n", "    token_source: keychain\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := filepath.Join(filepath.Dir(cfgFile), "keychain.yaml")
			if err := os.WriteFile(cfg, []byte(strings.Replace(string(data), tt.from, tt.to, 1)), 0o600); err != nil {
				t.Fatal(err)
			}
			store := newFakeStore()
			old := newTokenStore
			newTokenStore = func() (token.Store, error) { return store, nil }
			t.Cleanup(func() { newTokenStore = old })

			var stdout, stderr bytes.Buffer
			args := []string{"--config", cfg, "--backend", "1password", "--ui", "app", "--ui-app", "/tmp/a38-missing/Passierschein"}
			if code := serve(context.Background(), args, &stdout, &stderr); code != exitFailure {
				t.Errorf("exit code = %d, want %d", code, exitFailure)
			}
			want := `store the token with "a38 config onepassword-token set"`
			if runtime.GOOS != "darwin" {
				want = "not available on this platform"
			}
			if !strings.Contains(stderr.String(), want) {
				t.Errorf("stderr = %q, want %q", stderr.String(), want)
			}
			if _, err := os.Stat(filepath.Join(socketDir, "agent.sock")); !os.IsNotExist(err) {
				t.Error("a socket exists after a failed start")
			}
		})
	}
}
