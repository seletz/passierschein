package appui_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/approval/appui"
	"github.com/seletz/passierschein/internal/clock"
	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// helperPath is the compiled fake approval app.
var helperPath string

func TestMain(m *testing.M) { os.Exit(run(m)) }

func run(m *testing.M) int {
	dir, err := os.MkdirTemp("/tmp", "a38-fakeapp-")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	helperPath = filepath.Join(dir, "fakeapp")
	if out, err := exec.Command("go", "build", "-o", helperPath, "./testdata/fakeapp").CombinedOutput(); err != nil {
		panic("build helper: " + string(out))
	}
	return m.Run()
}

// shortDir returns a short directory that is removed after the test.
func shortDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "a38-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// app writes a script that starts the fake app with the flags. The
// daemon passes no argument, so the flags live in the script.
func app(t *testing.T, dir string, mode os.FileMode, flags ...string) string {
	t.Helper()
	path := filepath.Join(dir, "app-"+strconv.Itoa(len(flags))+"-"+strings.ReplaceAll(t.Name(), "/", "_"))
	script := "#!/bin/sh\nexec " + helperPath + " " + strings.Join(flags, " ") + "\n"
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func sampleRequest() approval.Request {
	return approval.Request{
		Version: approval.Version, Type: approval.TypeRequest, RequestID: "req-1", TimeoutSeconds: 120,
		Verified: session.Facts{Project: "beispielprojekt"},
		Credentials: []approval.Credential{
			{ID: "c1", Vault: "beispiel.app.k8s-dev", Title: "datenbank-dev", Field: "password"},
			{ID: "c2", Vault: "beispiel.app.k8s-dev", Title: "datenbank-dev", Field: "username"},
		},
		Grant: approval.GrantProposal{TTLSeconds: 3600},
	}
}

func newUI(t *testing.T, path string, clk clock.Clock) *appui.UI {
	t.Helper()
	u, err := appui.New(appui.Options{Path: path, Clock: clk, Grace: time.Second, NoticeLifetime: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(u.Close)
	return u
}

// T14 (part 1, Go side), T6 (part), T23 (part): what the app can answer,
// and that each irregularity is an error and never an approval.
func TestApprove(t *testing.T) {
	tests := []struct {
		name        string
		flags       []string
		wantErr     string // empty: no error
		wantOutcome approval.Outcome
		wantIDs     []string
	}{
		{name: "approve with authentication", flags: []string{"-tick", "0"}, wantOutcome: approval.OutcomeApproved, wantIDs: []string{"c1"}},
		{name: "approve all", flags: []string{"-tick", "all"}, wantOutcome: approval.OutcomeApproved, wantIDs: []string{"c1", "c2"}},
		{name: "deny", flags: []string{"-outcome", "denied", "-exit", "10"}, wantOutcome: approval.OutcomeDenied, wantIDs: []string{}},
		{name: "timeout by the app", flags: []string{"-outcome", "timeout", "-exit", "11"}, wantOutcome: approval.OutcomeTimeout, wantIDs: []string{}},
		{name: "approval of nothing is a denial", flags: []string{"-tick", "none", "-outcome", "approved"}, wantOutcome: approval.OutcomeDenied, wantIDs: []string{}},

		{name: "no authentication", flags: []string{"-tick", "0", "-auth", "none"}, wantErr: "no authentication"},
		{name: "failed authentication", flags: []string{"-tick", "0", "-auth", "failed"}, wantErr: "authentication of the human failed"},
		{name: "skipped authentication", flags: []string{"-tick", "0", "-auth", "skipped"}, wantErr: "policy"},
		{name: "unknown policy", flags: []string{"-tick", "0", "-auth", "unknown"}, wantErr: "policy"},
		{name: "approved with exit 10", flags: []string{"-tick", "0", "-exit", "10"}, wantErr: "does not fit"},
		{name: "approved with exit 1", flags: []string{"-tick", "0", "-exit", "1"}, wantErr: "exit code 1"},
		{name: "denied with exit 0", flags: []string{"-outcome", "denied"}, wantErr: "does not fit"},
		{name: "exit 12 auth failure", flags: []string{"-tick", "0", "-exit", "12"}, wantErr: "exit code 12"},
		{name: "exit 64", flags: []string{"-exit", "64", "-raw", "x"}, wantErr: "exit code 64"},
		{name: "exit 65", flags: []string{"-exit", "65", "-outcome", "error"}, wantErr: "exit code 65"},
		{name: "exit 70", flags: []string{"-exit", "70", "-raw", "x"}, wantErr: "exit code 70"},
		{name: "outcome error with exit 0", flags: []string{"-outcome", "error"}, wantErr: "does not fit"},
		{name: "no json", flags: []string{"-raw", "hello"}, wantErr: "not a decision"},
		{name: "empty output", flags: []string{"-raw", "' '"}, wantErr: "not a decision"},
		{name: "two values", flags: []string{"-tick", "0", "-twice"}, wantErr: "more than one value"},
		{name: "oversized output", flags: []string{"-big", "3000000"}, wantErr: "too large"},
		{name: "killed by signal", flags: []string{"-signal"}, wantErr: "signal"},
		{name: "other request ID", flags: []string{"-tick", "0", "-rid", "req-other"}, wantErr: "another request"},
		{name: "entry not in request", flags: []string{"-tick", "id:c1,zzz"}, wantErr: "not in the request"},
		{name: "longer lifetime", flags: []string{"-tick", "0", "-ttl", "plus1"}, wantErr: "lifetime"},
		{name: "zero lifetime", flags: []string{"-tick", "0", "-ttl", "0"}, wantErr: "lifetime"},
		{name: "negative lifetime", flags: []string{"-tick", "0", "-ttl", "-5"}, wantErr: "lifetime"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := app(t, shortDir(t), 0o700, tt.flags...)
			d, err := newUI(t, path, clock.NewFake(time.Unix(0, 0))).Approve(context.Background(), sampleRequest())
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				if d.Outcome == approval.OutcomeApproved {
					t.Fatal("an error came with an approval")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if d.Outcome != tt.wantOutcome || len(d.ApprovedCredentialIDs) != len(tt.wantIDs) {
				t.Fatalf("decision = %+v", d)
			}
			for i, id := range tt.wantIDs {
				if d.ApprovedCredentialIDs[i] != id {
					t.Errorf("IDs = %v, want %v", d.ApprovedCredentialIDs, tt.wantIDs)
				}
			}
			if d.Outcome == approval.OutcomeApproved && (d.Auth == nil || !d.Auth.OK || d.TTLSeconds != 3600) {
				t.Errorf("approval = %+v", d)
			}
		})
	}
}

// waitGone waits until no process with the PID exists. A zombie still
// exists for kill(pid, 0), so this also proves that the process was
// reaped.
func waitGone(t *testing.T, pidfile string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var pid int
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(pidfile); err == nil {
			if p, err := strconv.Atoi(string(b)); err == nil {
				pid = p
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatalf("no PID in %s", pidfile)
	}
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("process %d is still there", pid)
}

// T14 (part 1), T6 (part): an app that never answers ends with the outcome
// timeout after the wait time. The app and its group are killed and
// reaped.
func TestApproveTimeoutKillsGroup(t *testing.T) {
	dir := shortDir(t)
	pid, child := filepath.Join(dir, "pid"), filepath.Join(dir, "child")
	path := app(t, dir, 0o700, "-hang", "-pidfile", pid, "-spawn", child)
	clk := clock.NewFake(time.Unix(0, 0))
	u := newUI(t, path, clk)
	type res struct {
		d   approval.Decision
		err error
	}
	got := make(chan res, 1)
	go func() {
		d, err := u.Approve(context.Background(), sampleRequest())
		got <- res{d, err}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := clk.BlockUntil(ctx, 1); err != nil {
		t.Fatal(err)
	}
	// Wait until the app runs, so that the kill has a target.
	waitFile(t, child)
	clk.Advance(120 * time.Second) // the wait time alone is not enough
	select {
	case <-got:
		t.Fatal("Approve returned before the grace time")
	case <-time.After(50 * time.Millisecond):
	}
	clk.Advance(time.Second)
	r := <-got
	if r.err != nil || r.d.Outcome != approval.OutcomeTimeout || r.d.RequestID != "req-1" {
		t.Fatalf("result = %+v, %v", r.d, r.err)
	}
	waitGone(t, pid)
	waitGone(t, child)
}

func waitFile(t *testing.T, path string) {
	t.Helper()
	for i := 0; i < 500; i++ {
		if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("file %s did not appear", path)
}

// The end of the context kills the app and returns the error of the
// context.
func TestApproveContextEnd(t *testing.T) {
	dir := shortDir(t)
	pid := filepath.Join(dir, "pid")
	path := app(t, dir, 0o700, "-hang", "-pidfile", pid)
	u := newUI(t, path, clock.NewFake(time.Unix(0, 0)))
	ctx, cancel := context.WithCancel(context.Background())
	got := make(chan error, 1)
	go func() {
		_, err := u.Approve(ctx, sampleRequest())
		got <- err
	}()
	waitFile(t, pid)
	cancel()
	if err := <-got; !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	waitGone(t, pid)
}

// T14 (part 1): the request holds names only. The marker stands for a
// secret value that the request must never carry.
func TestRequestHoldsNoSecret(t *testing.T) {
	const marker = "beispiel-wert-geheim"
	dir := shortDir(t)
	rec := filepath.Join(dir, "rec")
	path := app(t, dir, 0o700, "-tick", "0", "-record", rec)
	req := sampleRequest()
	if _, err := newUI(t, path, nil).Approve(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	in, err := os.ReadFile(rec)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(in), marker) {
		t.Fatal("the marker is in the request")
	}
	if !strings.Contains(string(in), `"type":"approval_request"`) || !strings.Contains(string(in), "datenbank-dev") {
		t.Errorf("request = %s", in)
	}
}

// T14 (part 1): the app rejects a request without timeout_seconds from 1
// to 3600, so appui does not start it and gives an error (fail closed).
func TestApproveRejectsTimeoutOutOfRange(t *testing.T) {
	for _, secs := range []int{-1, 0, 3601, 100000} {
		t.Run(strconv.Itoa(secs), func(t *testing.T) {
			dir := shortDir(t)
			rec := filepath.Join(dir, "rec")
			path := app(t, dir, 0o700, "-tick", "all", "-record", rec)
			req := sampleRequest()
			req.TimeoutSeconds = secs
			d, err := newUI(t, path, nil).Approve(context.Background(), req)
			if err == nil || d.Outcome == approval.OutcomeApproved {
				t.Fatalf("decision %+v, error %v; want an error", d, err)
			}
			if _, statErr := os.Stat(rec); statErr == nil {
				t.Error("the app was started")
			}
		})
	}
	for _, secs := range []int{1, 3600} {
		req := sampleRequest()
		req.TimeoutSeconds = secs
		path := app(t, shortDir(t), 0o700, "-tick", "all")
		if _, err := newUI(t, path, nil).Approve(context.Background(), req); err != nil {
			t.Errorf("timeout_seconds %d: %v", secs, err)
		}
	}
}

// T6 (part): the checks of the file before the start.
func TestFileChecks(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root passes the owner check")
	}
	dir := shortDir(t)
	tests := []struct {
		name string
		path func(t *testing.T) string
		want string
	}{
		{"missing", func(*testing.T) string { return filepath.Join(dir, "nope") }, "cannot use"},
		{"directory", func(*testing.T) string { return dir }, "not a regular file"},
		{"writable by group", func(t *testing.T) string { return app(t, dir, 0o770) }, "writable by group or others"},
		{"writable by others", func(t *testing.T) string { return app(t, dir, 0o707) }, "writable by group or others"},
		{"not executable", func(t *testing.T) string { return app(t, dir, 0o600) }, "cannot execute"},
		{"setuid", func(t *testing.T) string { return app(t, dir, 0o700|os.ModeSetuid) }, "setuid"},
		{"readable by others is fine", func(t *testing.T) string { return app(t, dir, 0o755, "-tick", "0") }, ""},
		{"link to a bad file", func(t *testing.T) string {
			bad := app(t, dir, 0o777, "-flag")
			link := filepath.Join(dir, "link")
			_ = os.Symlink(bad, link)
			return link
		}, "writable by group or others"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newUI(t, tt.path(t), nil)
			d, err := u.Approve(context.Background(), sampleRequest())
			if tt.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
			if d.Outcome == approval.OutcomeApproved {
				t.Fatal("approval from a refused app")
			}
			if err := u.Notify(context.Background(), sampleNotice()); err == nil {
				t.Error("Notify started a refused app")
			}
		})
	}
}

// The owner check also refuses a file of another user that is not root.
func TestOwnerCheck(t *testing.T) {
	path := app(t, shortDir(t), 0o700, "-tick", "0")
	u, err := appui.New(appui.Options{Path: path, UID: os.Getuid() + 12345})
	if err != nil {
		t.Fatal(err)
	}
	if os.Getuid() == 0 {
		t.Skip("root owns the file")
	}
	if _, err := u.Approve(context.Background(), sampleRequest()); err == nil || !strings.Contains(err.Error(), "belongs to user") {
		t.Fatalf("err = %v", err)
	}
}

type refuse struct{}

func (refuse) Verify(context.Context, string) error { return errors.New("bad signature") }

// G5 extension point: a Verifier can stop the start.
func TestVerifierStopsStart(t *testing.T) {
	path := app(t, shortDir(t), 0o700, "-tick", "0")
	u, err := appui.New(appui.Options{Path: path, Verifier: refuse{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := u.Approve(context.Background(), sampleRequest()); err == nil || !strings.Contains(err.Error(), "bad signature") {
		t.Fatalf("err = %v", err)
	}
}

func TestNewNeedsPath(t *testing.T) {
	if _, err := appui.New(appui.Options{}); err == nil {
		t.Fatal("no error without a path")
	}
}

func sampleNotice() approval.Notice {
	return approval.Notice{
		Version: approval.Version, Type: approval.TypeNotice, NoticeID: "n-1",
		Kind: approval.NoticeDaemonFault, Code: protocol.CodeDaemonFaulted,
		Message: "Run a38 doctor.", OccurredAt: time.Unix(0, 0).UTC(),
	}
}

// D36, D48 (part): a notice starts the app with a message of the type
// notice. Notify does not wait for the app.
func TestNotify(t *testing.T) {
	dir := shortDir(t)
	rec := filepath.Join(dir, "rec")
	path := app(t, dir, 0o700, "-record", rec)
	u := newUI(t, path, nil)
	if err := u.Notify(context.Background(), sampleNotice()); err != nil {
		t.Fatal(err)
	}
	waitFile(t, rec)
	b, _ := os.ReadFile(rec)
	if !strings.Contains(string(b), `"type":"notice"`) || !strings.Contains(string(b), `"kind":"daemon_fault"`) {
		t.Errorf("notice = %s", b)
	}
}

// A notice that stays open ends after its lifetime, and Close ends it too.
func TestNoticeLifetimeAndClose(t *testing.T) {
	dir := shortDir(t)
	clk := clock.NewFake(time.Unix(0, 0))
	pid1 := filepath.Join(dir, "pid1")
	u1 := newUI(t, app(t, dir, 0o700, "-hang", "-pidfile", pid1), clk)
	if err := u1.Notify(context.Background(), sampleNotice()); err != nil {
		t.Fatal(err)
	}
	waitFile(t, pid1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := clk.BlockUntil(ctx, 1); err != nil {
		t.Fatal(err)
	}
	clk.Advance(time.Minute)
	waitGone(t, pid1)

	pid2 := filepath.Join(dir, "pid2")
	u2 := newUI(t, app(t, dir, 0o700, "-hang", "-pidfile", pid2, "-tick", "0"), nil)
	if err := u2.Notify(context.Background(), sampleNotice()); err != nil {
		t.Fatal(err)
	}
	waitFile(t, pid2)
	u2.Close()
	waitGone(t, pid2)
	if err := u2.Notify(context.Background(), sampleNotice()); err == nil {
		t.Error("Notify after Close")
	}
}

// execRequest is a valid approval request of the exec way for test.
func execRequest() approval.Request {
	req := sampleRequest()
	req.Credentials = []approval.Credential{
		{ID: "c1", Vault: "beispiel.app.k8s-test", Title: "datenbank-test", Field: "password", Environment: config.EnvTest},
	}
	req.Grant.TTLSeconds = 1800
	req.Exec = &approval.ExecCommand{
		Environment: config.EnvTest, Program: "/opt/homebrew/bin/psql", Args: []string{"-c", "select 1"},
		Cwd: "/Users/beispiel/develop/beispielprojekt",
		Env: []approval.ExecVariable{{Name: "PGPASSWORD", CredentialID: "c1"}},
		Approval: approval.ExecApproval{Kind: approval.ExecApprovalGrant, Binding: approval.BindingProgram,
			LifetimeChoicesSeconds: approval.ExecLifetimeChoices(1800)},
	}
	return req
}

// T38 (part, approval request): appui gives the app only a valid request
// of the exec way, and the decision for a command must fit its kind of
// approval (fail closed).
func TestApproveExec(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*approval.Request)
		flags   []string
		started bool
		wantErr string
		wantTTL int64
	}{
		{name: "grant with the proposed lifetime", mutate: func(*approval.Request) {}, flags: []string{"-tick", "all"}, started: true, wantTTL: 1800},
		{name: "grant with a shorter choice", mutate: func(*approval.Request) {}, flags: []string{"-tick", "all", "-ttl", "300"}, started: true, wantTTL: 300},
		{name: "lifetime not offered", mutate: func(*approval.Request) {}, flags: []string{"-tick", "all", "-ttl", "600"}, started: true, wantErr: "offered"},
		{name: "prod once", mutate: func(r *approval.Request) {
			r.Credentials[0].Environment = config.EnvProd
			r.Exec.Environment = config.EnvProd
			r.Grant.TTLSeconds = 0
			r.Exec.Approval = approval.ExecApproval{Kind: approval.ExecApprovalOnce, Binding: approval.BindingCommand}
		}, flags: []string{"-tick", "all", "-ttl", "0"}, started: true, wantTTL: 0},
		{name: "invalid exec is not started", mutate: func(r *approval.Request) { r.Exec.Program = "psql" }, flags: []string{"-tick", "all"}, wantErr: "exec.program"},
		{name: "prod with a grant is not started", mutate: func(r *approval.Request) {
			r.Credentials[0].Environment = config.EnvProd
			r.Exec.Environment = config.EnvProd
		}, flags: []string{"-tick", "all"}, wantErr: "prod"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := shortDir(t)
			rec := filepath.Join(dir, "rec")
			path := app(t, dir, 0o700, append(tt.flags, "-record", rec)...)
			req := execRequest()
			tt.mutate(&req)
			d, err := newUI(t, path, nil).Approve(context.Background(), req)
			_, statErr := os.Stat(rec)
			if started := statErr == nil; started != tt.started {
				t.Errorf("app started = %v, want %v", started, tt.started)
			}
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) || d.Outcome == approval.OutcomeApproved {
					t.Fatalf("decision %+v, err = %v, want %q", d, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if d.Outcome != approval.OutcomeApproved || d.TTLSeconds != tt.wantTTL {
				t.Errorf("decision = %+v", d)
			}
		})
	}
}
