package daemon

import (
	"fmt"
	"net"
	"path/filepath"
)

// Peer holds the data that the daemon reads about the process at the other
// end of a socket connection. The kernel gives the user ID and the process
// ID. The daemon reads the program path and the working directory of the
// process with that ID. An empty Process or Cwd means "unknown": the daemon
// could not read the value. The daemon never takes a value of Peer from the
// agent.
//
// Locally the peer data is a hint and not a proof (RI5): a process can exit
// and its process ID can go to a new process.
type Peer struct {
	// PID is the process ID of the peer. 0 means unknown.
	PID int
	// UID is the user ID of the peer.
	UID int
	// Process is the absolute path of the program of the peer. Empty means
	// unknown.
	Process string
	// Cwd is the working directory of the peer, absolute and with symbolic
	// links resolved. Empty means unknown.
	Cwd string
}

// readPeer reads the peer data of a Unix socket connection. An error means
// that the kernel did not give the user ID; the caller must close the
// connection. A program path or a working directory that the daemon cannot
// read stays empty.
func readPeer(c *net.UnixConn) (Peer, error) {
	raw, err := c.SyscallConn()
	if err != nil {
		return Peer{}, fmt.Errorf("daemon: cannot read the peer data: %w", err)
	}
	var p Peer
	var credErr error
	if err := raw.Control(func(fd uintptr) {
		p.PID, p.UID, credErr = peerCred(int(fd))
	}); err != nil {
		return Peer{}, fmt.Errorf("daemon: cannot read the peer data: %w", err)
	}
	if credErr != nil {
		return Peer{}, fmt.Errorf("daemon: cannot read the peer data: %w", credErr)
	}
	if p.PID > 0 {
		p.Process, p.Cwd = processInfo(p.PID)
		p.Process = cleanPath(p.Process)
		p.Cwd = cleanPath(p.Cwd)
	}
	return p, nil
}

// cleanPath returns the absolute path with symbolic links resolved. It
// returns the empty string (unknown) if path is not absolute or if a link
// cannot be resolved: the daemon does not derive a project from a path it
// cannot check.
func cleanPath(path string) string {
	if !filepath.IsAbs(path) {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return ""
	}
	return resolved
}
