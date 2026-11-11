//go:build !darwin && !linux

package daemon

import "errors"

// errPeerUnsupported is the error of a platform that cannot read the peer
// data. The daemon then serves no connection.
var errPeerUnsupported = errors.New("daemon: this platform cannot read the peer data of a socket")

// peerCred fails: the daemon has no code for the peer data of this
// platform, and it serves no connection without the user ID of the peer.
func peerCred(int) (pid, uid int, err error) {
	return 0, 0, errPeerUnsupported
}

// processInfo returns no data on this platform.
func processInfo(int) (process, cwd string) {
	return "", ""
}
