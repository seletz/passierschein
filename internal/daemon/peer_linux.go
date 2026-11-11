package daemon

import (
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

// peerCred returns the process ID and the user ID of the peer of a socket.
// It uses SO_PEERCRED.
func peerCred(fd int) (pid, uid int, err error) {
	cred, err := unix.GetsockoptUcred(fd, unix.SOL_SOCKET, unix.SO_PEERCRED)
	if err != nil {
		return 0, 0, err
	}
	return int(cred.Pid), int(cred.Uid), nil
}

// processInfo returns the program path and the working directory of a
// process from /proc. A value that the daemon cannot read is empty.
func processInfo(pid int) (process, cwd string) {
	dir := "/proc/" + strconv.Itoa(pid)
	process, _ = os.Readlink(dir + "/exe")
	cwd, _ = os.Readlink(dir + "/cwd")
	return process, cwd
}
