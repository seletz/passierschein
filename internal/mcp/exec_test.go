package mcp_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/seletz/passierschein/internal/client"
	a38mcp "github.com/seletz/passierschein/internal/mcp"
	"github.com/seletz/passierschein/internal/protocol"
)

// execValue is a neutral stand-in for a secret value.
const execValue = "beispiel-wert-4711"

// fakeDaemon speaks the agent protocol for the exec way. The real daemon
// answers request_exec with not_allowed until the broker supports it.
type fakeDaemon struct {
	socket string
	// answer gives the answer to request_exec. Nil answers with the
	// value execValue for each variable.
	answer func(req *protocol.RequestExec) (*protocol.ExecCredentials, *protocol.Error)
	// recordErr is the answer to exec_result, if set.
	recordErr *protocol.Error

	mu       sync.Mutex
	requests []*protocol.RequestExec
	results  []*protocol.ExecResult
	lines    []string
}

// startFakeDaemon listens on a socket in a short directory.
func startFakeDaemon(t *testing.T) *fakeDaemon {
	t.Helper()
	d := &fakeDaemon{socket: filepath.Join(shortDir(t), "agent.sock")}
	ln, err := net.Listen("unix", d.socket)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	t.Cleanup(func() { _ = ln.Close(); wg.Wait() })
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func() { defer wg.Done(); d.serve(c) }()
		}
	}()
	return d
}

// serve answers the messages of one connection.
func (d *fakeDaemon) serve(c net.Conn) {
	defer func() { _ = c.Close() }()
	r := bufio.NewReader(c)
	for {
		line, err := protocol.ReadLine(r)
		if err != nil {
			return
		}
		d.mu.Lock()
		d.lines = append(d.lines, string(line))
		d.mu.Unlock()
		msg, err := protocol.AgentRequests.Decode(line)
		if err != nil {
			return
		}
		var out protocol.Message
		switch m := msg.(type) {
		case *protocol.Hello:
			out = &protocol.Welcome{SessionID: "s-1", Project: "beispielprojekt"}
		case *protocol.RequestExec:
			d.mu.Lock()
			d.requests = append(d.requests, m)
			d.mu.Unlock()
			answer := d.answer
			if answer == nil {
				answer = deliver
			}
			creds, perr := answer(m)
			if perr != nil {
				out = &protocol.ErrorResponse{ID: m.ID, Error: *perr}
			} else {
				creds.ID = m.ID
				out = creds
			}
		case *protocol.ExecResult:
			d.mu.Lock()
			d.results = append(d.results, m)
			d.mu.Unlock()
			if d.recordErr != nil {
				out = &protocol.ErrorResponse{ID: m.ID, Error: *d.recordErr}
			} else {
				out = &protocol.ExecRecorded{ID: m.ID}
			}
		default:
			return
		}
		if err := protocol.Write(c, out); err != nil {
			return
		}
	}
}

// deliver answers with execValue for each variable, the time limit of the
// request (default 5 s) and an output limit of 64 KiB.
func deliver(req *protocol.RequestExec) (*protocol.ExecCredentials, *protocol.Error) {
	out := &protocol.ExecCredentials{TimeoutSeconds: 5, OutputLimit: 64 * 1024}
	if req.TimeoutSeconds > 0 {
		out.TimeoutSeconds = req.TimeoutSeconds
	}
	for _, v := range req.Env {
		out.Values = append(out.Values, protocol.ExecValue{Name: v.Name, Value: protocol.NewSecret(execValue)})
	}
	return out, nil
}

func (d *fakeDaemon) counts() (requests, results int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.requests), len(d.results)
}

func (d *fakeDaemon) lastRequest(t *testing.T) *protocol.RequestExec {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.requests) == 0 {
		t.Fatal("the daemon got no request_exec")
	}
	return d.requests[len(d.requests)-1]
}

func (d *fakeDaemon) lastResult(t *testing.T) *protocol.ExecResult {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.results) == 0 {
		t.Fatal("the daemon got no exec_result")
	}
	return d.results[len(d.results)-1]
}

// execSetup is the environment of one test: a fake daemon, a project
// directory and an MCP client session.
type execSetup struct {
	daemon  *fakeDaemon
	project string
	cs      *mcp.ClientSession
	logs    *syncBuilder
}

// newExecSetup starts a fake daemon and an MCP server whose working
// directory is a new project directory.
func newExecSetup(t *testing.T, change func(*a38mcp.Options)) *execSetup {
	t.Helper()
	s := &execSetup{daemon: startFakeDaemon(t), logs: &syncBuilder{}}
	project, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.project = project
	opts := a38mcp.Options{
		Dial:    func(ctx context.Context) (*client.Conn, error) { return client.DialAgent(ctx, s.daemon.socket) },
		Version: "test",
		Logger:  newLogger(s.logs),
		WorkDir: project,
		ProjectFor: func(dir string) (string, bool) {
			if dir == project || strings.HasPrefix(dir, project+"/") {
				return "beispielprojekt", true
			}
			return "", false
		},
	}
	if change != nil {
		change(&opts)
	}
	srv := a38mcp.New(opts)
	st, ct := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = srv.Run(ctx, st); close(done) }()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "coding-agent", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close(); cancel(); <-done })
	s.cs = cs
	return s
}

// self returns the path of the test binary: the helper program.
func self(t *testing.T) string {
	t.Helper()
	p, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// realSelf returns the path of the helper program after the symbolic
// links are resolved: the path that a38 mcp sends and starts.
func realSelf(t *testing.T) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(self(t))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// runArgs returns the arguments of run_with_credentials for the helper.
func runArgs(t *testing.T, helperArgs ...string) map[string]any {
	return map[string]any{
		"command": self(t),
		"args":    append([]string{helperArg}, helperArgs...),
		"env":     map[string]string{"DB_PASSWORD": refOK},
		"task":    "Run the migration of beispielprojekt",
		"reason":  "The migration needs the password of datenbank-dev.",
	}
}

// runResult is the JSON result of run_with_credentials (D61).
type runResult struct {
	ExitCode        *int   `json:"exit_code"`
	Signal          string `json:"signal"`
	TimedOut        bool   `json:"timed_out"`
	DurationMS      int64  `json:"duration_ms"`
	Stdout          string `json:"stdout"`
	Stderr          string `json:"stderr"`
	StdoutTruncated bool   `json:"stdout_truncated"`
	StderrTruncated bool   `json:"stderr_truncated"`
	Masked          *int   `json:"masked"`
}

// runOK calls the tool and decodes a result without isError. It checks
// that no byte of the answer holds the value.
func runOK(t *testing.T, s *execSetup, args map[string]any) runResult {
	t.Helper()
	res := callWithin(t, s.cs, args, 30*time.Second)
	if res.IsError {
		t.Fatalf("isError: %s", text(res))
	}
	noValue(t, res)
	var got runResult
	dec := json.NewDecoder(strings.NewReader(text(res)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("result %q: %v", text(res), err)
	}
	if got.ExitCode == nil || got.Masked == nil {
		t.Fatalf("result %q lacks exit_code or masked", text(res))
	}
	sc, _ := json.Marshal(res.StructuredContent)
	var again runResult
	if err := json.Unmarshal(sc, &again); err != nil || again.Stdout != got.Stdout || *again.ExitCode != *got.ExitCode {
		t.Errorf("structured content %s differs from the text", sc)
	}
	return got
}

func callWithin(t *testing.T, cs *mcp.ClientSession, args map[string]any, d time.Duration) *mcp.CallToolResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "run_with_credentials", Arguments: args})
	if err != nil {
		t.Fatalf("CallTool error = %v", err)
	}
	return res
}

// noValue fails if the answer holds the value.
func noValue(t *testing.T, res *mcp.CallToolResult) {
	t.Helper()
	sc, _ := json.Marshal(res.StructuredContent)
	if strings.Contains(text(res), execValue) || strings.Contains(string(sc), execValue) {
		t.Error("the MCP answer holds the value")
	}
}

// checkClean checks that the daemon never got a value or output, and that
// the log holds no value.
func (s *execSetup) checkClean(t *testing.T) {
	t.Helper()
	s.daemon.mu.Lock()
	defer s.daemon.mu.Unlock()
	for _, l := range s.daemon.lines {
		if strings.Contains(l, execValue) || strings.Contains(l, "stdout\":") || strings.Contains(l, "beispiel-ausgabe") {
			t.Errorf("a message to the daemon holds a value or output: %s", l)
		}
	}
	if strings.Contains(s.logs.String(), execValue) {
		t.Errorf("the log holds the value: %s", s.logs.String())
	}
}

func TestRunWithCredentialsDeliversTheValue(t *testing.T) {
	// T32 step 1: the command gets the value in its environment; the agent
	// gets exit code and output, never the value.
	s := newExecSetup(t, nil)
	got := runOK(t, s, runArgs(t, "envlen", "DB_PASSWORD"))
	if *got.ExitCode != 0 || got.Signal != "" || got.TimedOut {
		t.Errorf("result = %+v, want exit code 0", got)
	}
	if want := "len=" + strconv.Itoa(len(execValue)) + "\nbeispiel-ausgabe\n"; got.Stdout != want {
		t.Errorf("stdout = %q, want %q", got.Stdout, want)
	}
	req := s.daemon.lastRequest(t)
	if req.Program != realSelf(t) || req.Cwd != s.project || len(req.Args) != 3 || req.Args[0] != helperArg {
		t.Errorf("request_exec = program %q cwd %q args %q", req.Program, req.Cwd, req.Args)
	}
	if len(req.Env) != 1 || req.Env[0].Name != "DB_PASSWORD" || req.Env[0].Reference.String() != refOK {
		t.Errorf("request_exec env = %+v", req.Env)
	}
	if req.Task == "" || req.Reason == "" || req.TimeoutSeconds != 0 {
		t.Errorf("request_exec task %q reason %q timeout %d", req.Task, req.Reason, req.TimeoutSeconds)
	}
	res := s.daemon.lastResult(t)
	if res.RequestID != req.ID || res.ExitCode != 0 || res.StdoutBytes != int64(len(got.Stdout)) || res.Masked != 0 || res.Error != nil {
		t.Errorf("exec_result = %+v", res)
	}
	s.checkClean(t)
}

func TestRunWithCredentialsExitCode(t *testing.T) {
	// T32 step 2: an exit code other than 0 is a result, not an error.
	s := newExecSetup(t, nil)
	got := runOK(t, s, runArgs(t, "exit", "3"))
	if *got.ExitCode != 3 || got.Signal != "" {
		t.Errorf("result = %+v, want exit code 3", got)
	}
	if res := s.daemon.lastResult(t); res.ExitCode != 3 {
		t.Errorf("exec_result exit_code = %d, want 3", res.ExitCode)
	}
}

func TestRunWithCredentialsUnknownProgram(t *testing.T) {
	// T32 step 3: a program that does not exist ends with invalid_request;
	// the daemon gets no request.
	s := newExecSetup(t, func(o *a38mcp.Options) {
		o.Environ = func() []string { return []string{"PATH=/usr/bin:/bin"} }
	})
	notExec := filepath.Join(s.project, "daten.txt")
	if err := os.WriteFile(notExec, []byte("beispiel"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"a38-gibt-es-nicht", "./gibt-es-nicht", "/gibt/es/nicht", "daten.txt", "./daten.txt", s.project, ""} {
		t.Run(command, func(t *testing.T) {
			args := runArgs(t)
			args["command"] = command
			if got := errCode(t, callWithin(t, s.cs, args, testTimeout)); got != protocol.CodeInvalidRequest {
				t.Errorf("code = %q, want invalid_request", got)
			}
		})
	}
	if n, _ := s.daemon.counts(); n != 0 {
		t.Errorf("the daemon got %d requests, want 0", n)
	}
}

func TestRunWithCredentialsFindsTheProgram(t *testing.T) {
	// T32 (part): a name without "/" follows the PATH of a38 mcp; a path
	// with "/" is relative to cwd. a38 mcp resolves the symbolic links and
	// sends and starts the resolved path (G48).
	s := newExecSetup(t, func(o *a38mcp.Options) {
		o.Environ = func() []string { return []string{"PATH=relativ:/usr/bin:/bin"} }
	})
	args := runArgs(t)
	args["command"] = "sh"
	args["args"] = []string{"-c", "printf beispiel-ausgabe"}
	got := runOK(t, s, args)
	if got.Stdout != "beispiel-ausgabe" {
		t.Errorf("stdout = %q", got.Stdout)
	}
	sh := "/usr/bin/sh"
	if !fileExists(sh) {
		sh = "/bin/sh"
	}
	wantSh, err := filepath.EvalSymlinks(sh)
	if err != nil {
		t.Fatal(err)
	}
	if p := s.daemon.lastRequest(t).Program; p != wantSh {
		t.Errorf("program = %q, want the resolved path of sh %q", p, wantSh)
	}

	sub := filepath.Join(s.project, "werkzeug")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(sub, "hilfe")
	if err := os.Symlink(self(t), link); err != nil {
		t.Fatal(err)
	}
	args = runArgs(t, "pwd")
	args["command"] = "./hilfe"
	args["cwd"] = sub
	got = runOK(t, s, args)
	if got.Stdout != sub {
		t.Errorf("working directory = %q, want %q", got.Stdout, sub)
	}
	if p := s.daemon.lastRequest(t).Program; p != realSelf(t) {
		t.Errorf("program = %q, want the resolved path %q, not the link", p, realSelf(t))
	}
}

func TestRunWithCredentialsStartsTheResolvedProgram(t *testing.T) {
	// T32 (part, G48): a38 mcp resolves the link before request_exec. The
	// request carries the resolved path, and a38 mcp starts exactly that
	// path: a change of the link after the check has no effect.
	s := newExecSetup(t, nil)
	link := filepath.Join(s.project, "hilfe")
	if err := os.Symlink(self(t), link); err != nil {
		t.Fatal(err)
	}
	args := runArgs(t, "exe")
	args["command"] = link
	got := runOK(t, s, args)
	if p := s.daemon.lastRequest(t).Program; p != realSelf(t) {
		t.Errorf("request_exec program = %q, want %q", p, realSelf(t))
	}
	// argv[0] stays the command of the agent: a program can depend on
	// the name that started it.
	if want := realSelf(t) + "\n" + link; got.Stdout != want {
		t.Errorf("started path and argv[0] = %q, want %q", got.Stdout, want)
	}

	// A link that does not resolve is a program that a38 mcp cannot find.
	dangling := filepath.Join(s.project, "kaputt")
	if err := os.Symlink(filepath.Join(s.project, "gibt-es-nicht"), dangling); err != nil {
		t.Fatal(err)
	}
	args["command"] = dangling
	if got := errCode(t, callWithin(t, s.cs, args, testTimeout)); got != protocol.CodeInvalidRequest {
		t.Errorf("dangling link: code = %q, want invalid_request", got)
	}
}

// fileExists reports whether the path exists.
func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func TestRunWithCredentialsMasksTheValue(t *testing.T) {
	// T33 (with the tool): each exact value in stdout and stderr is masked.
	s := newExecSetup(t, nil)
	got := runOK(t, s, runArgs(t, "echo", "DB_PASSWORD"))
	if got.Stdout != "eins [masked:DB_PASSWORD] zwei [masked:DB_PASSWORD]\n" || got.Stderr != "fehler: [masked:DB_PASSWORD]\n" {
		t.Errorf("stdout %q stderr %q", got.Stdout, got.Stderr)
	}
	if *got.Masked != 3 || s.daemon.lastResult(t).Masked != 3 {
		t.Errorf("masked = %d, want 3", *got.Masked)
	}
	s.checkClean(t)
}

func TestRunWithCredentialsEmptyStdin(t *testing.T) {
	// D61: stdin is empty.
	s := newExecSetup(t, nil)
	if got := runOK(t, s, runArgs(t, "stdin")); got.Stdout != "stdin=0\n" {
		t.Errorf("stdout = %q, want stdin=0", got.Stdout)
	}
}

func TestRunWithCredentialsDaemonError(t *testing.T) {
	// D61: an error of the daemon arrives with its code; nothing starts.
	s := newExecSetup(t, nil)
	s.daemon.answer = func(*protocol.RequestExec) (*protocol.ExecCredentials, *protocol.Error) {
		return nil, protocol.Errorf(protocol.CodeNotAllowed, "the working directory is not in the project")
	}
	marker := filepath.Join(s.project, "gestartet")
	args := runArgs(t)
	args["command"] = "/bin/sh"
	args["args"] = []string{"-c", "echo x > " + marker}
	if got := errCode(t, callWithin(t, s.cs, args, testTimeout)); got != protocol.CodeNotAllowed {
		t.Errorf("code = %q, want not_allowed", got)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Error("the command started")
	}
	if _, n := s.daemon.counts(); n != 0 {
		t.Errorf("the daemon got %d exec_result, want 0", n)
	}
}

func TestRunWithCredentialsExecFailed(t *testing.T) {
	// D61, G39: the start fails after the delivery: exec_failed for the
	// agent and in exec_result.
	s := newExecSetup(t, nil)
	script := filepath.Join(s.project, "skript")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho beispiel\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	s.daemon.answer = func(req *protocol.RequestExec) (*protocol.ExecCredentials, *protocol.Error) {
		// The file is no longer executable after the check of a38 mcp.
		if err := os.Chmod(script, 0o644); err != nil {
			return nil, protocol.Errorf(protocol.CodeDaemonFaulted, "chmod failed")
		}
		return deliver(req)
	}
	args := runArgs(t)
	args["command"] = script
	args["args"] = nil
	res := callWithin(t, s.cs, args, testTimeout)
	if got := errCode(t, res); got != protocol.CodeExecFailed {
		t.Errorf("code = %q, want exec_failed", got)
	}
	noValue(t, res)
	if r := s.daemon.lastResult(t); r.Error == nil || r.Error.Code != protocol.CodeExecFailed {
		t.Errorf("exec_result = %+v, want the error exec_failed", r)
	}
	s.checkClean(t)
}

func TestRunWithCredentialsRecordError(t *testing.T) {
	// G39: an error of the daemon for exec_result does not hide the result
	// of the command that ran.
	s := newExecSetup(t, nil)
	s.daemon.recordErr = protocol.Errorf(protocol.CodeDaemonFaulted, "the audit log failed")
	if got := runOK(t, s, runArgs(t, "exit", "0")); *got.ExitCode != 0 {
		t.Errorf("exit code = %d", *got.ExitCode)
	}
	if !strings.Contains(s.logs.String(), "daemon_faulted") {
		t.Errorf("log = %q, want the code of the daemon", s.logs.String())
	}
}

// pids reads "pid=N child=M" from the output.
func pids(t *testing.T, out string) []int {
	t.Helper()
	m := regexp.MustCompile(`pid=(\d+) child=(\d+)`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("output %q has no process IDs", out)
	}
	a, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[2])
	return []int{a, b}
}

// checkGone waits until no process of the list lives.
func checkGone(t *testing.T, list []int) {
	t.Helper()
	deadline := time.Now().Add(testTimeout)
	for _, pid := range list {
		for syscall.Kill(pid, 0) == nil {
			if time.Now().After(deadline) {
				t.Errorf("process %d still lives", pid)
				_ = syscall.Kill(pid, syscall.SIGKILL)
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func TestRunWithCredentialsTimeLimit(t *testing.T) {
	// T34 step 1: the command runs longer than the time limit and starts a
	// child. SIGTERM goes to the process group; no process lives after.
	s := newExecSetup(t, nil)
	args := runArgs(t, "child")
	args["timeout_seconds"] = 1
	got := runOK(t, s, args)
	if !got.TimedOut || got.Signal != "SIGTERM" || *got.ExitCode != -1 {
		t.Errorf("result = %+v, want timed_out with SIGTERM", got)
	}
	checkGone(t, pids(t, got.Stdout))
	if r := s.daemon.lastResult(t); !r.TimedOut || r.Signal != "SIGTERM" || r.ExitCode != -1 {
		t.Errorf("exec_result = %+v", r)
	}
	if r := s.daemon.lastRequest(t); r.TimeoutSeconds != 1 {
		t.Errorf("request_exec timeout_seconds = %d, want 1", r.TimeoutSeconds)
	}
}

func TestRunWithCredentialsKillsAfterGrace(t *testing.T) {
	// T34 step 1: a command that ignores SIGTERM gets SIGKILL after the
	// grace period (5 s in D64, shorter in the test).
	defer a38mcp.SetKillGrace(200 * time.Millisecond)()
	s := newExecSetup(t, nil)
	args := runArgs(t, "ignore-term")
	args["timeout_seconds"] = 1
	got := runOK(t, s, args)
	if !got.TimedOut || got.Signal != "SIGKILL" || *got.ExitCode != -1 {
		t.Errorf("result = %+v, want timed_out with SIGKILL", got)
	}
	checkGone(t, pids(t, got.Stdout))
}

func TestRunWithCredentialsEndsTheGroup(t *testing.T) {
	// G40: a child that the command leaves behind does not keep the value
	// alive; the tool returns.
	s := newExecSetup(t, nil)
	start := time.Now()
	got := runOK(t, s, runArgs(t, "background"))
	if *got.ExitCode != 0 || got.TimedOut {
		t.Errorf("result = %+v, want exit code 0", got)
	}
	if d := time.Since(start); d > 15*time.Second {
		t.Errorf("the tool took %v", d)
	}
	checkGone(t, pids(t, got.Stdout))
}

func TestRunWithCredentialsOutputLimit(t *testing.T) {
	// T34 step 2: 200 KiB output with the limit 64 KiB: the end with 64
	// KiB, stdout_truncated.
	s := newExecSetup(t, nil)
	got := runOK(t, s, runArgs(t, "big", strconv.Itoa(200*1024)))
	if !got.StdoutTruncated || got.StderrTruncated || len(got.Stdout) != 64*1024 {
		t.Fatalf("truncated %v, length %d", got.StdoutTruncated, len(got.Stdout))
	}
	// The helper writes 'a' + i%26; the end starts at byte 200 KiB - 64 KiB.
	if first := got.Stdout[0]; first != byte('a'+(200*1024-64*1024)%26) {
		t.Errorf("first byte %q, want the end of the output", first)
	}
	r := s.daemon.lastResult(t)
	if r.StdoutBytes != 200*1024 || !r.StdoutTruncated {
		t.Errorf("exec_result stdout_bytes %d truncated %v", r.StdoutBytes, r.StdoutTruncated)
	}
}

func TestRunWithCredentialsTimeoutAboveMaximum(t *testing.T) {
	// T34 step 3: a time limit above the maximum ends with invalid_request
	// without a start. a38 mcp rejects values above the protocol limit;
	// the daemon rejects values above exec.max_timeout.
	s := newExecSetup(t, nil)
	s.daemon.answer = func(req *protocol.RequestExec) (*protocol.ExecCredentials, *protocol.Error) {
		if req.TimeoutSeconds > 600 {
			return nil, protocol.Errorf(protocol.CodeInvalidRequest, "timeout_seconds is above exec.max_timeout")
		}
		return deliver(req)
	}
	for _, tt := range []struct {
		value    any
		requests int
	}{{3601, 0}, {-1, 0}, {1.5, 0}, {"60", 0}, {601, 1}} {
		args := runArgs(t, "exit", "0")
		args["timeout_seconds"] = tt.value
		before, _ := s.daemon.counts()
		if got := errCode(t, callWithin(t, s.cs, args, testTimeout)); got != protocol.CodeInvalidRequest {
			t.Errorf("timeout_seconds %v: code = %q, want invalid_request", tt.value, got)
		}
		after, results := s.daemon.counts()
		if after-before != tt.requests || results != 0 {
			t.Errorf("timeout_seconds %v: %d requests %d results", tt.value, after-before, results)
		}
	}
}

func TestRunWithCredentialsForbiddenNames(t *testing.T) {
	// T34 step 4: forbidden or invalid names in env end with
	// invalid_request; the daemon gets no request.
	s := newExecSetup(t, nil)
	for _, env := range []map[string]any{
		{"PATH": refOK},
		{"DYLD_INSERT_LIBRARIES": refOK},
		{"A38_CONFIG": refOK},
		{"kleinbuchstaben": refOK},
		{"LD_PRELOAD": refOK},
		{},
		{"DB_PASSWORD": "op://beispiel.app.k8s-dev/datenbank-dev"},
		{"DB_PASSWORD": 42},
	} {
		args := runArgs(t, "exit", "0")
		args["env"] = env
		res := callWithin(t, s.cs, args, testTimeout)
		if got := errCode(t, res); got != protocol.CodeInvalidRequest {
			t.Errorf("env %v: code = %q, want invalid_request", env, got)
		}
		if strings.Contains(text(res), "kleinbuchstaben") {
			t.Errorf("env %v: the error repeats the invalid name", env)
		}
	}
	if n, _ := s.daemon.counts(); n != 0 {
		t.Errorf("the daemon got %d requests, want 0", n)
	}
}

func TestRunWithCredentialsWorkingDirectory(t *testing.T) {
	// T34 step 5: cwd outside the project ends with invalid_request without
	// a start. A relative or missing directory too. A symbolic link into
	// the project is resolved before the check.
	s := newExecSetup(t, nil)
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	toOutside := filepath.Join(s.project, "nach-aussen")
	if err := os.Symlink(outside, toOutside); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(s.project, "datei")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, cwd := range []any{outside, toOutside, "relativ", filepath.Join(s.project, "fehlt"), file, 7} {
		args := runArgs(t, "exit", "0")
		args["cwd"] = cwd
		if got := errCode(t, callWithin(t, s.cs, args, testTimeout)); got != protocol.CodeInvalidRequest {
			t.Errorf("cwd %v: code = %q, want invalid_request", cwd, got)
		}
	}
	if n, _ := s.daemon.counts(); n != 0 {
		t.Errorf("the daemon got %d requests, want 0", n)
	}
	inside := filepath.Join(outside, "nach-innen")
	if err := os.Symlink(s.project, inside); err != nil {
		t.Fatal(err)
	}
	args := runArgs(t, "pwd")
	args["cwd"] = inside
	if got := runOK(t, s, args); *got.ExitCode != 0 {
		t.Errorf("cwd through a link into the project: %+v", got)
	}
}

func TestRunWithCredentialsNoProject(t *testing.T) {
	// D64: if the working directory of a38 mcp is in no project, each cwd
	// ends with invalid_request.
	s := newExecSetup(t, func(o *a38mcp.Options) {
		o.ProjectFor = func(string) (string, bool) { return "", false }
	})
	if got := errCode(t, callWithin(t, s.cs, runArgs(t, "exit", "0"), testTimeout)); got != protocol.CodeInvalidRequest {
		t.Errorf("code = %q, want invalid_request", got)
	}
}

func TestRunWithCredentialsLimitsOfTheCommand(t *testing.T) {
	// D64: more than 256 arguments, 16 KiB of arguments, a missing task or
	// reason end with invalid_request.
	s := newExecSetup(t, nil)
	tooMany := make([]string, protocol.MaxExecArgs+1)
	for i := range tooMany {
		tooMany[i] = "a"
	}
	for name, change := range map[string]func(map[string]any){
		"too many arguments": func(a map[string]any) { a["args"] = tooMany },
		"too long":           func(a map[string]any) { a["args"] = []string{strings.Repeat("a", protocol.MaxExecArgsSize+1)} },
		"control character":  func(a map[string]any) { a["args"] = []string{"a\x07"} },
		"no task":            func(a map[string]any) { delete(a, "task") },
		"no reason":          func(a map[string]any) { a["reason"] = "" },
		"unknown field":      func(a map[string]any) { a["shell"] = true },
		"no env":             func(a map[string]any) { delete(a, "env") },
	} {
		args := runArgs(t, "exit", "0")
		change(args)
		if got := errCode(t, callWithin(t, s.cs, args, testTimeout)); got != protocol.CodeInvalidRequest {
			t.Errorf("%s: code = %q, want invalid_request", name, got)
		}
	}
	if n, _ := s.daemon.counts(); n != 0 {
		t.Errorf("the daemon got %d requests, want 0", n)
	}
}
