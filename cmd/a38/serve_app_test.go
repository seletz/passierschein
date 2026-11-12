package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/client"
	"github.com/seletz/passierschein/internal/protocol"
)

var (
	fakeAppOnce sync.Once
	fakeAppBin  string
	fakeAppErr  error
)

// buildFakeApp compiles the fake approval app of the appui tests. It never
// opens a window and never asks for Touch ID.
func buildFakeApp(t *testing.T) string {
	t.Helper()
	fakeAppOnce.Do(func() {
		dir, err := os.MkdirTemp("/tmp", "a38-fakeapp-")
		if err != nil {
			fakeAppErr = err
			return
		}
		fakeAppBin = filepath.Join(dir, "fakeapp")
		out, err := exec.Command("go", "build", "-o", fakeAppBin, "../../internal/approval/appui/testdata/fakeapp").CombinedOutput()
		if err != nil {
			fakeAppErr = &buildError{string(out)}
		}
	})
	if fakeAppErr != nil {
		t.Fatal(fakeAppErr)
	}
	return fakeAppBin
}

type buildError struct{ out string }

func (e *buildError) Error() string { return "build fake app: " + e.out }

// writeApp writes the script that starts the fake app with the flags.
func writeApp(t *testing.T, dir string, mode os.FileMode, flags string) string {
	t.Helper()
	path := filepath.Join(dir, "Passierschein")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexec "+buildFakeApp(t)+" "+flags+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

// startServe starts the daemon with --ui app and returns the socket
// directory and the log.
func startServeApp(t *testing.T, appPath string) (socketDir string, log *syncBuffer) {
	t.Helper()
	cfg, socketDir, fakeFile := writeConfig(t)
	ctx, cancel := context.WithCancel(context.Background())
	log = &syncBuffer{}
	done := make(chan int, 1)
	go func() {
		done <- serve(ctx, []string{"--config", cfg, "--backend", "fake", "--fake-file", fakeFile, "--ui", "app", "--ui-app", appPath}, log, log)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	waitDaemonStarted(t, log, nil)
	return socketDir, log
}

// T14 (part 1, Go side), T6, T9: a38 serve --ui app with the fake backend
// starts the approval app as a process for each request. Only an approval
// with the authentication of the human delivers a value; each other answer
// delivers nothing. The request that the app reads holds no value.
func TestServeUIApp(t *testing.T) {
	const value = "beispiel-wert-serve"
	tests := []struct {
		name     string
		flags    string
		mode     os.FileMode
		missing  bool
		wantCode protocol.Code // empty: delivered
	}{
		{"approved with authentication", "-tick all", 0o700, false, ""},
		{"approved without authentication", "-tick all -auth none", 0o700, false, protocol.CodeUIFailed},
		{"approved with failed authentication", "-tick all -auth failed", 0o700, false, protocol.CodeUIFailed},
		{"approved with skipped authentication", "-tick all -auth skipped", 0o700, false, protocol.CodeUIFailed},
		{"denied", "-outcome denied -exit 10", 0o700, false, protocol.CodeDenied},
		{"app crashes", "-tick all -signal", 0o700, false, protocol.CodeUIFailed},
		{"no decision", "-raw garbage", 0o700, false, protocol.CodeUIFailed},
		{"approved but exit code 1", "-tick all -exit 1", 0o700, false, protocol.CodeUIFailed},
		{"entry not in the request", "-tick id:zzz", 0o700, false, protocol.CodeUIFailed},
		{"app writable by others", "-tick all", 0o707, false, protocol.CodeUIFailed},
		{"app missing", "-tick all", 0o700, true, protocol.CodeUIFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, err := os.MkdirTemp("/tmp", "a38-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(dir) })
			rec := filepath.Join(dir, "rec")
			path := writeApp(t, dir, tt.mode, tt.flags+" -record "+rec)
			if tt.missing {
				path = filepath.Join(dir, "nope")
			}
			socketDir, log := startServeApp(t, path)

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			c, err := client.DialAgent(ctx, filepath.Join(socketDir, "agent.sock"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = c.Close() }()
			if _, err := c.Hello(ctx, "beispiel-koordinator", "coding-agent"); err != nil {
				t.Fatal(err)
			}
			ref, _ := protocol.ParseReference("op://beispiel.app.k8s-dev/datenbank-dev/password")
			res, err := c.RequestCredentials(ctx, &protocol.RequestCredentials{References: []protocol.Reference{ref}})
			if tt.wantCode == "" {
				if err != nil || len(res.Entries) != 1 || res.Entries[0].Value == nil || res.Entries[0].Value.Reveal() != value {
					t.Fatalf("no delivery: %v", err)
				}
			} else {
				if protocol.CodeOf(err) != tt.wantCode {
					t.Fatalf("code = %q (%v), want %q", protocol.CodeOf(err), err, tt.wantCode)
				}
				if res != nil && len(res.Entries) > 0 {
					t.Fatal("a value was delivered")
				}
			}
			if strings.Contains(log.String(), value) {
				t.Error("the log contains the value")
			}
			if tt.missing || tt.mode != 0o700 {
				if !strings.Contains(log.String(), "the approval app") {
					t.Errorf("no clear log line:\n%s", log.String())
				}
				return
			}
			in, err := os.ReadFile(rec)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(in), value) {
				t.Error("the request of the app contains the value")
			}
			if !strings.Contains(string(in), "datenbank-dev") {
				t.Errorf("request = %s", in)
			}
		})
	}
}

// writeExe creates an empty executable file with its directories.
func writeExe(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolveUIApp(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "a38-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	home := filepath.Join(root, "home")
	inApps := filepath.Join("/Applications", appInBundle)
	if _, err := os.Stat(inApps); err == nil {
		t.Skip("a real app is installed in /Applications")
	}
	homeApp := filepath.Join(home, "Applications", appInBundle)
	bin := filepath.Join(root, "bin")
	binExe := func() string { return filepath.Join(bin, "a38") }
	homeFn := func() string { return home }

	// Case a: a38 runs from inside a bundle; its symlink lives in another
	// directory, as a link in the PATH does. The real path decides.
	bundle := filepath.Join(root, "Beispiel.app", "Contents", "MacOS")
	writeExe(t, filepath.Join(bundle, "a38"))
	link := filepath.Join(root, "pathdir", "a38")
	if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(bundle, "a38"), link); err != nil {
		t.Fatal(err)
	}
	realOfLink := func() string {
		r, err := filepath.EvalSymlinks(link)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	if got := realOfLink(); filepath.Base(filepath.Dir(got)) != "MacOS" {
		t.Fatalf("link resolves to %q", got)
	}
	sibling := executablePathOf(writeExe(t, filepath.Join(bundle, appName)))
	// A plain file next to the link must not win over the sibling.
	writeExe(t, filepath.Join(filepath.Dir(link), appName))
	if got := resolveUIApp("", "", func() string { return executablePathOf(link) }, homeFn); got != sibling {
		t.Errorf("case a: %q, want %q", got, sibling)
	}

	tests := []struct {
		name  string
		setup func()
		want  string
	}{
		{"nothing there: first candidate", func() {}, filepath.Join(bin, appInBundle)},
		{"home applications", func() { writeExe(t, homeApp) }, homeApp},
		{"plain executable next to a38", func() { writeExe(t, filepath.Join(bin, appName)) }, filepath.Join(bin, appName)},
		{"bundle next to a38 wins over plain", func() { writeExe(t, filepath.Join(bin, appInBundle)) }, filepath.Join(bin, appInBundle)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			if got := resolveUIApp("", "", binExe, homeFn); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
	t.Run("flag and configuration come first", func(t *testing.T) {
		if got := resolveUIApp("/flag/app", "/cfg/app", binExe, homeFn); got != "/flag/app" {
			t.Errorf("flag: %q", got)
		}
		if got := resolveUIApp("", "/cfg/app", binExe, homeFn); got != "/cfg/app" {
			t.Errorf("config: %q", got)
		}
	})
	t.Run("no executable path", func(t *testing.T) {
		if got := resolveUIApp("", "", func() string { return "" }, func() string { return "" }); got != inApps {
			t.Errorf("got %q, want %q", got, inApps)
		}
	})
}

// executablePathOf resolves links as executablePath does.
func executablePathOf(p string) string {
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		return p
	}
	return r
}

// G5 (part): the layout of the release bundle. The human unpacks
// Passierschein.app into an Applications folder and links
// Contents/MacOS/a38 into a folder of the PATH. The daemon finds the app
// next to the real a38 inside the bundle (D50), before any other place.
func TestResolveUIAppReleaseBundle(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "a38-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	macos := filepath.Join(root, "Applications", appBundle, "Contents", "MacOS")
	a38 := writeExe(t, filepath.Join(macos, "a38"))
	app := executablePathOf(writeExe(t, filepath.Join(macos, appName)))
	link := filepath.Join(root, "home", ".local", "bin", "a38")
	if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(a38, link); err != nil {
		t.Fatal(err)
	}
	// Another app in the home Applications folder must not win.
	writeExe(t, filepath.Join(root, "home", "Applications", appInBundle))
	got := resolveUIApp("", "", func() string { return executablePathOf(link) }, func() string { return filepath.Join(root, "home") })
	if got != app {
		t.Errorf("got %q, want %q", got, app)
	}
}
