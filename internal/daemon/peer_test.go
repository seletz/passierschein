package daemon

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

// shortDir returns a new directory under /tmp. A socket path below
// t.TempDir() is often longer than the 103 bytes that macOS permits.
func shortDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "a38-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// mustResolve returns path with symbolic links resolved.
func mustResolve(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestReadPeer(t *testing.T) {
	// T11 (part): the daemon reads the facts of the peer from the kernel,
	// not from a message.
	path := filepath.Join(shortDir(t), "peer.sock")
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	client, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	server, err := l.AcceptUnix()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	got, err := readPeer(server)
	if err != nil {
		t.Fatalf("readPeer() error = %v", err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	want := Peer{PID: os.Getpid(), UID: os.Getuid(), Process: mustResolve(t, exe), Cwd: mustResolve(t, wd)}
	if got != want {
		t.Errorf("readPeer() = %+v, want %+v", got, want)
	}
}

func TestCleanPath(t *testing.T) {
	dir := shortDir(t)
	link := filepath.Join(dir, "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty is unknown", "", ""},
		{"relative is unknown", "beispielprojekt", ""},
		{"missing is unknown", filepath.Join(dir, "missing"), ""},
		{"link is resolved", link, mustResolve(t, dir)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cleanPath(tt.in); got != tt.want {
				t.Errorf("cleanPath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
