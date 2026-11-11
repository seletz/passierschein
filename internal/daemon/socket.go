package daemon

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"syscall"
	"time"
)

// Modes of the socket directory and of each socket (D43).
const (
	dirMode    fs.FileMode = 0o700
	socketMode fs.FileMode = 0o600
)

// maxSocketPath is the maximum length of a socket path in bytes. macOS
// permits 103 bytes and the end mark.
const maxSocketPath = 103

// staleDialTimeout limits the test connection to an old socket.
const staleDialTimeout = time.Second

// prepareDir creates the socket directory with the mode 0700 if it does not
// exist. It fails if the path is not a directory, if the directory belongs
// to a different user or if its mode is not exactly 0700. The daemon does
// not correct the mode: a different mode is a finding for the human (D43).
func prepareDir(dir string) error {
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("daemon: cannot create the socket directory %s: %w", dir, err)
	}
	fi, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("daemon: cannot read the socket directory %s: %w", dir, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("daemon: the socket path %s is not a directory", dir)
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != os.Getuid() {
		return fmt.Errorf("daemon: the socket directory %s does not belong to the user of the daemon", dir)
	}
	if perm := fi.Mode().Perm(); perm != dirMode || fi.Mode()&(fs.ModeSetuid|fs.ModeSetgid|fs.ModeSticky) != 0 {
		return fmt.Errorf("daemon: the socket directory %s has the mode %04o; it must be %04o (chmod 700 %s)", dir, perm, dirMode, dir)
	}
	return nil
}

// lockDir takes an exclusive lock on the socket directory. The lock makes
// sure that only one daemon uses the directory. The lock ends when the
// returned file is closed or the process ends.
func lockDir(dir string) (*os.File, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("daemon: cannot open the socket directory %s: %w", dir, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("daemon: another daemon uses the socket directory %s", dir)
		}
		return nil, fmt.Errorf("daemon: cannot lock the socket directory %s: %w", dir, err)
	}
	return f, nil
}

// removeStale removes an old socket at path. It never removes a socket that
// accepts a connection, and it never removes a file that is not a socket.
func removeStale(path string) error {
	fi, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("daemon: cannot read %s: %w", path, err)
	}
	if fi.Mode().Type() != fs.ModeSocket {
		return fmt.Errorf("daemon: %s exists and is not a socket; remove it by hand", path)
	}
	d := net.Dialer{Timeout: staleDialTimeout}
	c, err := d.Dial("unix", path)
	if err == nil {
		_ = c.Close()
		return fmt.Errorf("daemon: a process accepts connections on %s; another daemon runs", path)
	}
	if !errors.Is(err, syscall.ECONNREFUSED) {
		return fmt.Errorf("daemon: cannot check the old socket %s: %w", path, err)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("daemon: cannot remove the old socket %s: %w", path, err)
	}
	return nil
}

// listen creates the Unix socket at path with the mode 0600. An old socket
// must be removed before.
func listen(path string) (*net.UnixListener, error) {
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, fmt.Errorf("daemon: cannot create the socket %s: %w", path, err)
	}
	fail := func(err error) (*net.UnixListener, error) {
		_ = l.Close()
		return nil, err
	}
	// The directory has the mode 0700, so no other user reaches the socket
	// before the change of the mode.
	if err := os.Chmod(path, socketMode); err != nil {
		return fail(fmt.Errorf("daemon: cannot set the mode of the socket %s: %w", path, err))
	}
	fi, err := os.Lstat(path)
	if err != nil {
		return fail(fmt.Errorf("daemon: cannot read the socket %s: %w", path, err))
	}
	if fi.Mode().Perm() != socketMode {
		return fail(fmt.Errorf("daemon: the socket %s has the mode %04o; it must be %04o", path, fi.Mode().Perm(), socketMode))
	}
	return l, nil
}

// checkSocketPath fails if a socket path is too long for the platform.
func checkSocketPath(path string) error {
	if len(path) > maxSocketPath {
		return fmt.Errorf("daemon: the socket path %s is longer than %d bytes", path, maxSocketPath)
	}
	return nil
}
