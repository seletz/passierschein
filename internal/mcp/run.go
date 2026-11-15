package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unsafe"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/seletz/passierschein/internal/client"
	"github.com/seletz/passierschein/internal/protocol"
)

// ToolRunWithCredentials is the tool that starts a command with
// credentials in its environment (D61).
const ToolRunWithCredentials = "run_with_credentials"

// runSchema is the input schema of run_with_credentials (D61). The handler
// checks each rule again.
var runSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "command": {"type": "string", "description": "Name or path of the program, for example psql. Not text for a shell. A name without / follows the PATH; a path with / is relative to cwd."},
    "args": {"type": "array", "items": {"type": "string"}, "description": "Arguments of the program. a38 passes them unchanged and never puts a value into them."},
    "env": {
      "type": "object",
      "additionalProperties": {"type": "string"},
      "minProperties": 1,
      "description": "Map from the name of an environment variable to a reference op://vault/item/field, for example {\"PGPASSWORD\": \"op://vault/item/password\"}."
    },
    "cwd": {"type": "string", "description": "Absolute path of the working directory, in the project. Default: the working directory of a38 mcp."},
    "timeout_seconds": {"type": "integer", "minimum": 0, "description": "Time limit of the command in seconds. 0 or no value: the default of the configuration."},
    "task": {"type": "string", "description": "What you are doing. One short sentence."},
    "reason": {"type": "string", "description": "Why the command needs these credentials. One short sentence."}
  },
  "required": ["command", "env", "task", "reason"],
  "additionalProperties": false
}`)

// runArgs are the arguments of run_with_credentials.
type runArgs struct {
	Command        string            `json:"command"`
	Args           []string          `json:"args"`
	Env            map[string]string `json:"env"`
	Cwd            string            `json:"cwd"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	Task           string            `json:"task"`
	Reason         string            `json:"reason"`
}

// runWithCredentials handles the tool run_with_credentials (D61, D64).
func (s *Server) runWithCredentials(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args runArgs
	if err := decodeArgs(req.Params.Arguments, &args); err != nil {
		return toolError(protocol.Errorf(protocol.CodeInvalidRequest, "%v", err)), nil
	}
	wire, perr := s.prepareExec(&args)
	if perr != nil {
		return toolError(perr), nil
	}
	conn, perr := s.connect(ctx, req.Session)
	if perr != nil {
		return toolError(perr), nil
	}
	creds, err := conn.RequestExec(ctx, wire)
	if err != nil {
		return s.failed(conn, err), nil
	}
	defer func() { creds.Values = nil }()
	if err := matchValues(wire, creds); err != nil {
		s.log.Warn("the daemon sent values that do not match the request")
		return toolError(err), nil
	}

	env, values, arena := s.commandEnv(creds)
	defer wipe(arena)
	o, startErr := run(ctx, command{
		program: wire.Program,
		argv0:   args.Command,
		args:    wire.Args,
		dir:     wire.Cwd,
		env:     env,
		timeout: time.Duration(creds.TimeoutSeconds) * time.Second,
	})
	if startErr != nil {
		wipe(arena)
		reason := startError(startErr)
		s.log.Warn("the command did not start", "reason", reason)
		perr := protocol.Errorf(protocol.CodeExecFailed, "a38 mcp could not start the program: %s", reason)
		s.report(ctx, conn, &protocol.ExecResult{RequestID: wire.ID, Error: perr})
		return toolError(perr), nil
	}
	stdout := mask(o.stdout.Bytes(), o.stdout.Dropped(), values, creds.OutputLimit)
	stderr := mask(o.stderr.Bytes(), o.stderr.Dropped(), values, creds.OutputLimit)
	o.stdout.Wipe()
	o.stderr.Wipe()
	wipe(arena)

	s.report(ctx, conn, &protocol.ExecResult{
		RequestID:       wire.ID,
		ExitCode:        o.exitCode,
		Signal:          o.signal,
		TimedOut:        o.timedOut,
		DurationMS:      o.duration.Milliseconds(),
		StdoutBytes:     o.stdout.Total(),
		StderrBytes:     o.stderr.Total(),
		StdoutTruncated: stdout.truncated,
		StderrTruncated: stderr.truncated,
		Masked:          stdout.masked + stderr.masked,
	})
	return structured(map[string]any{
		"exit_code":        o.exitCode,
		"signal":           o.signal,
		"timed_out":        o.timedOut,
		"duration_ms":      o.duration.Milliseconds(),
		"stdout":           stdout.text,
		"stderr":           stderr.text,
		"stdout_truncated": stdout.truncated,
		"stderr_truncated": stderr.truncated,
		"masked":           stdout.masked + stderr.masked,
	}), nil
}

// prepareExec checks the arguments, finds the program and returns the
// request_exec. Each failure is invalid_request before any request to the
// daemon (D61).
func (s *Server) prepareExec(args *runArgs) (*protocol.RequestExec, *protocol.Error) {
	if len(args.Env) == 0 {
		return nil, protocol.Errorf(protocol.CodeInvalidRequest, "env must have at least one entry")
	}
	if len(args.Env) > protocol.MaxReferences {
		return nil, protocol.Errorf(protocol.CodeInvalidRequest, "env has more than %d entries", protocol.MaxReferences)
	}
	if args.TimeoutSeconds < 0 || args.TimeoutSeconds > protocol.MaxExecTimeoutSeconds {
		return nil, protocol.Errorf(protocol.CodeInvalidRequest, "timeout_seconds must be 0 to %d", protocol.MaxExecTimeoutSeconds)
	}
	names := make([]string, 0, len(args.Env))
	for name := range args.Env {
		names = append(names, name)
	}
	slices.Sort(names)
	wire := &protocol.RequestExec{
		// The client sets the ID; this one only passes the check.
		ID:             "c-0",
		Args:           args.Args,
		TimeoutSeconds: args.TimeoutSeconds,
		Task:           args.Task,
		Reason:         args.Reason,
	}
	for _, name := range names {
		if err := protocol.CheckEnvName(name); err != nil {
			return nil, asProtocolError(err)
		}
		ref, err := protocol.ParseReference(args.Env[name])
		if err != nil {
			return nil, protocol.Errorf(protocol.CodeInvalidRequest, "env %s: %s", name, asProtocolError(err).Message)
		}
		wire.Env = append(wire.Env, protocol.ExecVar{Name: name, Reference: ref})
	}
	cwd, perr := s.checkCwd(args.Cwd)
	if perr != nil {
		return nil, perr
	}
	wire.Cwd = cwd
	program, perr := findProgram(args.Command, cwd, s.getenv("PATH"))
	if perr != nil {
		return nil, perr
	}
	wire.Program = program
	if err := wire.Validate(); err != nil {
		return nil, asProtocolError(err)
	}
	return wire, nil
}

// checkCwd checks the working directory and returns its clean path. The
// directory must exist and be in the project of the session (D64). The
// check resolves symbolic links; the request keeps the clean path. The
// daemon checks the directory again.
func (s *Server) checkCwd(cwd string) (string, *protocol.Error) {
	if cwd == "" {
		var err error
		if cwd, err = s.workDir(); err != nil {
			return "", protocol.Errorf(protocol.CodeInvalidRequest, "a38 mcp cannot find its working directory")
		}
	}
	if !filepath.IsAbs(cwd) {
		return "", protocol.Errorf(protocol.CodeInvalidRequest, "cwd must be an absolute path")
	}
	cwd = filepath.Clean(cwd)
	if info, err := os.Stat(cwd); err != nil || !info.IsDir() {
		return "", protocol.Errorf(protocol.CodeInvalidRequest, "cwd is not a directory")
	}
	if s.opts.ProjectFor == nil {
		return cwd, nil
	}
	own, err := s.workDir()
	if err == nil {
		own, err = filepath.EvalSymlinks(own)
	}
	if err != nil {
		return "", protocol.Errorf(protocol.CodeInvalidRequest, "a38 mcp cannot find its working directory")
	}
	session, ok := s.opts.ProjectFor(own)
	if !ok {
		return "", protocol.Errorf(protocol.CodeInvalidRequest, "the working directory of a38 mcp is in no project")
	}
	resolved, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return "", protocol.Errorf(protocol.CodeInvalidRequest, "cwd is not a directory")
	}
	if p, ok := s.opts.ProjectFor(resolved); !ok || p != session {
		return "", protocol.Errorf(protocol.CodeInvalidRequest, "cwd is not in the project of the session")
	}
	return cwd, nil
}

// findProgram returns the absolute, clean path of the program (D61). A
// command with "/" is relative to cwd. A name without "/" follows the
// PATH; an empty or relative entry of the PATH does not count. Symbolic
// links stay.
func findProgram(name, cwd, path string) (string, *protocol.Error) {
	if name == "" {
		return "", protocol.Errorf(protocol.CodeInvalidRequest, "command is empty")
	}
	if strings.ContainsRune(name, '/') {
		p := name
		if !filepath.IsAbs(p) {
			p = filepath.Join(cwd, p)
		}
		p = filepath.Clean(p)
		if !executable(p) {
			return "", protocol.Errorf(protocol.CodeInvalidRequest, "command is not an executable file")
		}
		return p, nil
	}
	for _, dir := range filepath.SplitList(path) {
		if dir == "" || !filepath.IsAbs(dir) {
			continue
		}
		if p := filepath.Join(dir, name); executable(p) {
			return filepath.Clean(p), nil
		}
	}
	return "", protocol.Errorf(protocol.CodeInvalidRequest, "command is not in the PATH of a38 mcp")
}

// executable reports whether path is a regular file with an execute bit.
func executable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

// matchValues checks that the answer has one value for each variable of
// the request, in the same sequence.
func matchValues(req *protocol.RequestExec, creds *protocol.ExecCredentials) *protocol.Error {
	if len(creds.Values) != len(req.Env) {
		return protocol.Errorf(protocol.CodeDaemonFaulted, "the daemon sent values that do not match the request")
	}
	for i, v := range creds.Values {
		if v.Name != req.Env[i].Name {
			return protocol.Errorf(protocol.CodeDaemonFaulted, "the daemon sent values that do not match the request")
		}
	}
	return nil
}

// commandEnv returns the environment of the command: the environment of
// a38 mcp plus the variables (D61). A variable replaces a variable of the
// same name (D64). The entries "NAME=value" and the values for the mask
// share one byte arena. The caller overwrites the arena after the end of
// the command; Go does not permit to overwrite a string.
func (s *Server) commandEnv(creds *protocol.ExecCredentials) (env []string, values []maskValue, arena []byte) {
	size := 0
	for _, v := range creds.Values {
		size += len(v.Name) + 1 + len(v.Value.Reveal())
	}
	arena = make([]byte, 0, size)
	set := make(map[string]bool, len(creds.Values))
	entries := make([]string, 0, len(creds.Values))
	for _, v := range creds.Values {
		set[v.Name] = true
		start := len(arena)
		arena = append(arena, v.Name...)
		arena = append(arena, '=')
		valueStart := len(arena)
		arena = append(arena, v.Value.Reveal()...)
		// The string uses the bytes of the arena, so that the overwrite
		// of the arena also removes the value from the environment.
		entries = append(entries, unsafe.String(&arena[start], len(arena)-start))
		values = append(values, maskValue{name: v.Name, value: arena[valueStart:len(arena):len(arena)]})
	}
	for _, kv := range s.environ() {
		name, _, _ := strings.Cut(kv, "=")
		if !set[name] {
			env = append(env, kv)
		}
	}
	return append(env, entries...), values, arena
}

// report sends exec_result to the daemon. A failure goes only to the log:
// the command ran, and the agent gets its result (G39).
func (s *Server) report(ctx context.Context, conn *client.Conn, res *protocol.ExecResult) {
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), connectTimeout)
	defer cancel()
	if _, err := conn.ExecResult(rctx, res); err != nil {
		var perr *protocol.Error
		if errors.As(err, &perr) {
			s.log.Warn("the daemon did not record the result of the command", "code", perr.Code)
			return
		}
		s.log.Warn("cannot send the result of the command to the daemon", "error", err)
	}
}

// asProtocolError returns err as *protocol.Error with the code
// invalid_request if it has no code.
func asProtocolError(err error) *protocol.Error {
	var perr *protocol.Error
	if errors.As(err, &perr) {
		return perr
	}
	return protocol.Errorf(protocol.CodeInvalidRequest, "the request is not valid")
}

// workDir returns the working directory of a38 mcp.
func (s *Server) workDir() (string, error) {
	if s.opts.WorkDir != "" {
		return s.opts.WorkDir, nil
	}
	return os.Getwd()
}

// environ returns the environment of a38 mcp.
func (s *Server) environ() []string {
	if s.opts.Environ != nil {
		return s.opts.Environ()
	}
	return os.Environ()
}

// getenv returns one variable of the environment of a38 mcp.
func (s *Server) getenv(name string) string {
	for _, kv := range s.environ() {
		if k, v, ok := strings.Cut(kv, "="); ok && k == name {
			return v
		}
	}
	return ""
}
