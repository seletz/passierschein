package daemon

import (
	"bytes"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Values of <sys/proc_info.h>. The package unix has no wrapper for
// proc_pidinfo; the daemon calls the system call proc_info directly, so that
// the build needs no cgo.
const (
	// procInfoCallPIDInfo is PROC_INFO_CALL_PIDINFO.
	procInfoCallPIDInfo = 2
	// procPIDVnodePathInfo is PROC_PIDVNODEPATHINFO: the working directory
	// and the root directory of a process.
	procPIDVnodePathInfo = 9
	// procPIDPathInfo is PROC_PIDPATHINFO: the path of the program.
	procPIDPathInfo = 11
	// maxPathLen is MAXPATHLEN.
	maxPathLen = 1024
	// vnodeInfoSize is the size of struct vnode_info. A struct
	// vnode_info_path is a vnode_info followed by the path.
	vnodeInfoSize = 152
	// vnodePathInfoSize is the size of struct proc_vnodepathinfo: two
	// vnode_info_path structs, first the working directory.
	vnodePathInfoSize = 2 * (vnodeInfoSize + maxPathLen)
)

// peerCred returns the process ID and the user ID of the peer of a socket.
// It uses LOCAL_PEERPID and LOCAL_PEERCRED.
func peerCred(fd int) (pid, uid int, err error) {
	cred, err := unix.GetsockoptXucred(fd, unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
	if err != nil {
		return 0, 0, err
	}
	pid, err = unix.GetsockoptInt(fd, unix.SOL_LOCAL, unix.LOCAL_PEERPID)
	if err != nil {
		return 0, 0, err
	}
	return pid, int(cred.Uid), nil
}

// processInfo returns the program path and the working directory of a
// process. A value that the daemon cannot read is empty.
func processInfo(pid int) (process, cwd string) {
	buf := make([]byte, 4*maxPathLen)
	// For this flavor the kernel returns 0 and not the length.
	if _, err := procInfo(pid, procPIDPathInfo, buf); err == nil {
		process = cString(buf)
	}
	buf = make([]byte, vnodePathInfoSize)
	if n, err := procInfo(pid, procPIDVnodePathInfo, buf); err == nil && n == vnodePathInfoSize {
		cwd = cString(buf[vnodeInfoSize : vnodeInfoSize+maxPathLen])
	}
	return process, cwd
}

// procInfo calls proc_info(PROC_INFO_CALL_PIDINFO, pid, flavor, 0, buf,
// len(buf)) and returns the number of bytes that the kernel wrote.
//
// The libproc wrappers proc_pidpath and proc_pidinfo need cgo. The direct
// system call goes through the syscall function of libSystem, as each
// unix.Syscall6 on darwin does. If a future macOS removes it, the call
// fails and the program path and the working directory stay unknown; the
// daemon then derives no project (fail closed).
func procInfo(pid, flavor int, buf []byte) (int, error) {
	//nolint:staticcheck // SA1019: no libSystem wrapper without cgo; see above.
	n, _, errno := unix.Syscall6(unix.SYS_PROC_INFO, procInfoCallPIDInfo, uintptr(pid), uintptr(flavor), 0,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if errno != 0 {
		return 0, errno
	}
	if int(n) < 0 || int(n) > len(buf) {
		return 0, unix.EINVAL
	}
	return int(n), nil
}

// cString returns the text up to the first NUL byte.
func cString(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}
