package mcp

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// maxHeld is the maximum number of bytes of each stream that a38 mcp holds
// in memory (D64). It counts the rest.
const maxHeld = 4 * 1024 * 1024

// killGrace is the time from SIGTERM to SIGKILL (D64). A test changes it.
var killGrace = 5 * time.Second

// readerGrace is the time that a38 mcp waits for the end of the output
// after the end of the process group. A process that left the group can
// hold the pipe open.
const readerGrace = time.Second

// pollInterval is the interval of the check for the end of the process
// group.
const pollInterval = 20 * time.Millisecond

// command is one command to start.
type command struct {
	// program is the absolute path. a38 mcp starts exactly this path.
	program string
	// argv0 is the first entry of the argument list of the process.
	argv0   string
	args    []string
	dir     string
	env     []string
	timeout time.Duration
}

// outcome is the result of a command that started.
type outcome struct {
	exitCode int
	signal   string
	timedOut bool
	duration time.Duration
	stdout   *tailBuffer
	stderr   *tailBuffer
}

// run starts the command in its own process group with an empty stdin and
// collects stdout and stderr. At the time limit, or when ctx ends, it
// sends SIGTERM to the process group and SIGKILL after killGrace (D64).
// When the command ends, it ends the rest of the process group the same
// way (G40). An error means that the command did not start.
func run(ctx context.Context, c command) (*outcome, error) {
	outR, outW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		closeAll(outR, outW)
		return nil, err
	}
	cmd := &exec.Cmd{
		Path:        c.program,
		Args:        append([]string{c.argv0}, c.args...),
		Dir:         c.dir,
		Env:         c.env,
		Stdout:      outW,
		Stderr:      errW,
		SysProcAttr: &syscall.SysProcAttr{Setpgid: true},
	}
	start := time.Now()
	if err := cmd.Start(); err != nil {
		closeAll(outR, outW, errR, errW)
		return nil, err
	}
	closeAll(outW, errW)
	pgid := cmd.Process.Pid
	o := &outcome{stdout: newTailBuffer(maxHeld), stderr: newTailBuffer(maxHeld)}

	var readers sync.WaitGroup
	readers.Add(2)
	go func() { defer readers.Done(); collect(outR, o.stdout) }()
	go func() { defer readers.Done(); collect(errR, o.stderr) }()

	exited := make(chan struct{})
	var end time.Time
	go func() {
		_ = cmd.Wait()
		end = time.Now()
		close(exited)
	}()

	timer := time.NewTimer(c.timeout)
	select {
	case <-exited:
		timer.Stop()
		if groupAlive(pgid) {
			stopGroup(pgid, exited)
		}
	case <-timer.C:
		o.timedOut = true
		stopGroup(pgid, exited)
	case <-ctx.Done():
		timer.Stop()
		stopGroup(pgid, exited)
	}
	<-exited
	o.duration = end.Sub(start)

	done := make(chan struct{})
	go func() { readers.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(readerGrace):
	}
	closeAll(outR, errR)
	<-done

	ws, _ := cmd.ProcessState.Sys().(syscall.WaitStatus)
	if ws.Signaled() {
		o.exitCode, o.signal = -1, signalName(ws.Signal())
	} else {
		o.exitCode = ws.ExitStatus()
	}
	return o, nil
}

// collect reads r into b until the end of r. It overwrites its read
// buffer at the end.
func collect(r *os.File, b *tailBuffer) {
	buf := make([]byte, 32*1024)
	defer wipe(buf)
	for {
		n, err := r.Read(buf)
		_, _ = b.Write(buf[:n])
		if err != nil {
			return
		}
	}
}

// stopGroup sends SIGTERM to the process group. It sends SIGKILL if the
// command or a process of the group still lives after killGrace. It
// returns when the command ended.
func stopGroup(pgid int, exited <-chan struct{}) {
	_ = syscall.Kill(-pgid, syscall.SIGTERM)
	grace := time.NewTimer(killGrace)
	defer grace.Stop()
	tick := time.NewTicker(pollInterval)
	defer tick.Stop()
	for {
		select {
		case <-grace.C:
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
			<-exited
			return
		case <-tick.C:
		}
		select {
		case <-exited:
			if !groupAlive(pgid) {
				return
			}
		default:
		}
	}
}

// groupAlive reports whether a process of the group still lives.
func groupAlive(pgid int) bool {
	err := syscall.Kill(-pgid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// signalName returns the name of a signal, for example "SIGTERM".
func signalName(s syscall.Signal) string {
	if name := unix.SignalName(s); name != "" {
		return name
	}
	return "SIG" + strconv.Itoa(int(s))
}

// startError returns the reason why a start failed, without a path or a
// value.
func startError(err error) string {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno.Error()
	}
	return "unknown error"
}

func closeAll(files ...*os.File) {
	for _, f := range files {
		_ = f.Close()
	}
}
