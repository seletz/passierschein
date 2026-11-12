// Package appui holds the [approval.UI] that starts the approval app (F11,
// D6).
//
// The daemon starts the app as a child process for each request and for
// each notice. The contract:
//
//   - The daemon writes one JSON message to the standard input of the app
//     and closes it. The message is an [approval.Request] (type
//     "approval_request") or an [approval.Notice] (type "notice"). The app
//     reads the field "type" and acts as the type says. It gets no
//     argument.
//   - For a request the app writes one decision ([approval.Decision]) to
//     its standard output and ends with the exit code of the outcome. For
//     a notice it shows the text, writes nothing and ends with 0 after the
//     human closed it.
//   - The standard error of the app is dropped.
//
// Every irregularity is no approval (fail closed, D35): a start failure, a
// crash, a signal, an exit code that does not fit the outcome, no JSON,
// more than one JSON value, too much output, a decision for another
// request, entries that are not in the request, a lifetime out of range, a
// missing or failed authentication of the human. Approve then returns an
// error and the daemon answers ui_failed. After the wait time, Approve
// kills the app with its process group and returns the outcome timeout.
//
// The package checks the file of the app before each start with
// [approval.CheckAppFile] and with a [Verifier]. The Verifier is the
// extension point for the check of the code signature (G5). The default
// does nothing.
package appui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/clock"
)

// The exit codes of the approval app. They are the codes of the spike
// report. Code 0 needs the outcome approved, 10 denied and 11 timeout.
// Each other code, among them 12, 64, 65 and 70, is a failure.
const (
	ExitApproved = 0
	ExitDenied   = 10
	ExitTimeout  = 11
)

const (
	// maxOutput is the largest standard output of the app that we read.
	maxOutput = 1 << 20
	// DefaultGrace is the time after the wait time of the request that the
	// app has to end on its own. The app shows the remaining time and ends
	// with the outcome timeout by itself; the daemon is the second line.
	DefaultGrace = 5 * time.Second
	// DefaultNoticeLifetime is the longest time that a notice stays open.
	DefaultNoticeLifetime = 10 * time.Minute
)

// Verifier checks the approval app before each start. It is the extension
// point for the check of the code signature (G5): a later version checks
// the signature and the team ID of the app here. An error stops the start.
type Verifier interface {
	// Verify checks the file at path. It runs after the file checks.
	Verify(ctx context.Context, path string) error
}

// NoopVerifier accepts each file. It is the default until G5 is closed.
type NoopVerifier struct{}

// Verify implements [Verifier]. It does nothing.
func (NoopVerifier) Verify(context.Context, string) error { return nil }

// Options holds the settings for [New].
type Options struct {
	// Path is the path of the approval app. It is required.
	Path string
	// Clock is the time source. Nil means the system clock.
	Clock clock.Clock
	// Verifier checks the app before each start. Nil means
	// [NoopVerifier].
	Verifier Verifier
	// Logger receives one line for each refused start and for each
	// failure of a notice. Nil means no log.
	Logger *slog.Logger
	// Grace overrides [DefaultGrace] if it is more than zero.
	Grace time.Duration
	// NoticeLifetime overrides [DefaultNoticeLifetime] if it is more than
	// zero.
	NoticeLifetime time.Duration
	// UID is the user that must own the app, besides root. Zero means the
	// user of this process.
	UID int
}

// UI is the approval UI that starts the approval app.
type UI struct {
	path     string
	clock    clock.Clock
	verifier Verifier
	log      *slog.Logger
	grace    time.Duration
	noticeTT time.Duration
	uid      int

	mu      sync.Mutex
	closed  bool
	notices map[*proc]struct{}
	wg      sync.WaitGroup
}

// New returns a UI for the app at path.
func New(opts Options) (*UI, error) {
	if opts.Path == "" {
		return nil, errors.New("the approval app has no path")
	}
	u := &UI{
		path: opts.Path, clock: opts.Clock, verifier: opts.Verifier, log: opts.Logger,
		grace: opts.Grace, noticeTT: opts.NoticeLifetime, uid: opts.UID,
		notices: map[*proc]struct{}{},
	}
	if u.clock == nil {
		u.clock = clock.System{}
	}
	if u.verifier == nil {
		u.verifier = NoopVerifier{}
	}
	if u.log == nil {
		u.log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if u.grace <= 0 {
		u.grace = DefaultGrace
	}
	if u.noticeTT <= 0 {
		u.noticeTT = DefaultNoticeLifetime
	}
	if u.uid == 0 {
		u.uid = os.Getuid()
	}
	return u, nil
}

// Check checks the file of the app: it is a regular file, owned by the
// user or root, not writable by group or others. The result does not
// include the [Verifier].
func (u *UI) Check() error {
	fi, err := os.Stat(u.path) // follows a link; the target is what runs
	if err != nil {
		return fmt.Errorf("cannot use the approval app: %w", err)
	}
	if err := approval.CheckAppFile(fi, u.uid); err != nil {
		return err
	}
	return nil
}

// prepare runs the checks before a start. A failure is logged and
// returned.
func (u *UI) prepare(ctx context.Context) error {
	err := u.Check()
	if err == nil {
		err = u.verifier.Verify(ctx, u.path)
	}
	if err != nil {
		u.log.Error("the approval app is refused", "path", u.path, "error", err)
		return fmt.Errorf("appui: %w", err)
	}
	return nil
}

// Approve starts the app with the request and waits for its decision. It
// returns the outcome timeout after the wait time of the request plus the
// grace time, and ctx.Err() if ctx ends first. The app is killed in both
// cases, with its process group, and Approve returns after it was reaped.
func (u *UI) Approve(ctx context.Context, req approval.Request) (approval.Decision, error) {
	if err := u.prepare(ctx); err != nil {
		return approval.Decision{}, err
	}
	in, err := json.Marshal(req)
	if err != nil {
		return approval.Decision{}, fmt.Errorf("appui: encode request: %w", err)
	}
	wait := time.Duration(req.TimeoutSeconds) * time.Second
	if wait <= 0 {
		wait = approval.DefaultTimeout
	}
	timer := u.clock.NewTimer(wait + u.grace)
	defer timer.Stop()

	p, err := start(u.path, in)
	if err != nil {
		return approval.Decision{}, err
	}
	select {
	case err := <-p.done:
		return u.finish(req, p, err)
	case <-timer.C():
		p.killAndReap()
		return approval.Decision{
			Version: approval.Version, Type: approval.TypeDecision, RequestID: req.RequestID,
			Outcome: approval.OutcomeTimeout, ApprovedCredentialIDs: []string{}, ElapsedMS: wait.Milliseconds(),
		}, nil
	case <-ctx.Done():
		p.killAndReap()
		return approval.Decision{}, ctx.Err()
	}
}

// finish turns the end of the app into a decision.
func (u *UI) finish(req approval.Request, p *proc, waitErr error) (approval.Decision, error) {
	if p.out.over.Load() {
		return approval.Decision{}, errors.New("appui: the output of the app is too large")
	}
	code, err := exitCode(waitErr)
	if err != nil {
		return approval.Decision{}, err
	}
	var want approval.Outcome
	switch code {
	case ExitApproved:
		want = approval.OutcomeApproved
	case ExitDenied:
		want = approval.OutcomeDenied
	case ExitTimeout:
		want = approval.OutcomeTimeout
	default:
		// 12, 64, 65, 70 and each other code. The text names the code
		// only, never the output of the app.
		return approval.Decision{}, fmt.Errorf("appui: the app failed with exit code %d", code)
	}
	var d approval.Decision
	dec := json.NewDecoder(bytes.NewReader(p.out.buf.Bytes()))
	if err := dec.Decode(&d); err != nil {
		return approval.Decision{}, errors.New("appui: the output of the app is not a decision")
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return approval.Decision{}, errors.New("appui: the output of the app has more than one value")
	}
	if d.Outcome != want {
		return approval.Decision{}, fmt.Errorf("appui: exit code %d does not fit the outcome %q", code, d.Outcome)
	}
	d, err = approval.ValidateDecision(req, d)
	if err != nil {
		return approval.Decision{}, fmt.Errorf("appui: %w", err)
	}
	if d.Outcome == approval.OutcomeApproved {
		if err := checkAuth(d.Auth); err != nil {
			return approval.Decision{}, err
		}
	}
	return d, nil
}

// checkAuth requires the authentication of the human for an approval (D7).
// The policy "skipped" is for faked modes and never counts.
func checkAuth(a *approval.AuthResult) error {
	switch {
	case a == nil:
		return errors.New("appui: the approval has no authentication of the human")
	case !a.OK:
		return errors.New("appui: the authentication of the human failed")
	}
	switch a.Policy {
	case "biometrics", "biometrics_or_companion", "owner":
		return nil
	}
	return errors.New("appui: the authentication policy is not valid for an approval")
}

// exitCode returns the exit code of a finished process. A signal, a wait
// failure and a pipe that stays open are errors.
func exitCode(err error) (int, error) {
	if err == nil {
		return 0, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if ee.ExitCode() < 0 {
			return 0, errors.New("appui: the app ended by a signal")
		}
		return ee.ExitCode(), nil
	}
	return 0, errors.New("appui: the app did not end cleanly")
}

// Notify starts the app with the notice and returns. The app shows the
// notice until the human closes it, at most for the notice lifetime. The
// result of the app does not matter for the daemon; a failure goes to the
// log. [UI.Close] ends the apps that still run.
func (u *UI) Notify(ctx context.Context, n approval.Notice) error {
	if err := u.prepare(ctx); err != nil {
		return err
	}
	in, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("appui: encode notice: %w", err)
	}
	u.mu.Lock()
	if u.closed {
		u.mu.Unlock()
		return errors.New("appui: closed")
	}
	p, err := start(u.path, in)
	if err != nil {
		u.mu.Unlock()
		return err
	}
	u.notices[p] = struct{}{}
	u.wg.Add(1)
	u.mu.Unlock()
	timer := u.clock.NewTimer(u.noticeTT)
	go func() {
		defer u.wg.Done()
		defer timer.Stop()
		select {
		case err := <-p.done:
			if code, cerr := exitCode(err); cerr != nil || code != 0 {
				u.log.Warn("the approval app failed to show a notice", "kind", n.Kind)
			}
		case <-timer.C():
			p.killAndReap()
		case <-p.stop:
			p.killAndReap()
		}
		u.mu.Lock()
		delete(u.notices, p)
		u.mu.Unlock()
	}()
	return nil
}

// Close kills the apps of the notices that still run and waits until they
// are gone. A later Notify fails.
func (u *UI) Close() {
	u.mu.Lock()
	u.closed = true
	for p := range u.notices {
		p.stopOnce.Do(func() { close(p.stop) })
	}
	u.mu.Unlock()
	u.wg.Wait()
}

// proc is a running app.
type proc struct {
	cmd  *exec.Cmd
	out  *capped
	done chan error // gets the result of Wait once

	stop     chan struct{}
	stopOnce sync.Once
}

// childEnv is the environment of the app. It holds a short list of
// variables and no DYLD_ or LD_ variable, so that the environment of the
// daemon cannot inject code into the app.
func childEnv() []string {
	var env []string
	for _, k := range []string{"HOME", "USER", "LOGNAME", "TMPDIR", "LANG", "LC_ALL", "PATH", "__CF_USER_TEXT_ENCODING"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// start starts the app in its own process group and writes the message to
// its standard input.
func start(path string, in []byte) (*proc, error) {
	p := &proc{out: &capped{}, done: make(chan error, 1), stop: make(chan struct{})}
	cmd := exec.Command(path) //nolint:gosec // the path passed the file checks
	cmd.Dir = "/"
	cmd.Env = childEnv()
	cmd.Stdin = bytes.NewReader(in)
	cmd.Stdout = p.out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 2 * time.Second
	p.cmd = cmd
	p.out.kill = p.kill
	if err := cmd.Start(); err != nil {
		// The text holds no output of the app.
		return nil, errors.New("appui: cannot start the approval app")
	}
	go func() { p.done <- cmd.Wait() }()
	return p, nil
}

// kill sends SIGKILL to the process group of the app.
func (p *proc) kill() {
	if p.cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
	_ = p.cmd.Process.Kill()
}

// killAndReap kills the app and waits until the system reaped it.
func (p *proc) killAndReap() {
	p.kill()
	for range p.done { // the result is the kill that we caused
		return
	}
}

// capped collects output up to maxOutput bytes. At the first byte over the
// limit it sets over and kills the app.
type capped struct {
	buf  bytes.Buffer
	over atomic.Bool
	kill func()
}

func (c *capped) Write(b []byte) (int, error) {
	if c.over.Load() || c.buf.Len()+len(b) > maxOutput {
		if !c.over.Swap(true) && c.kill != nil {
			go c.kill()
		}
		return 0, errors.New("output too large")
	}
	return c.buf.Write(b)
}
