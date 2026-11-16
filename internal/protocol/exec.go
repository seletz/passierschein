package protocol

import (
	"path"
	"strings"
)

// Limits of the exec way (D64, D66). The configuration of the daemon sets
// the limits that apply; they cannot be larger than these.
const (
	// MaxExecArgs is the maximum number of arguments of a command, without
	// the program.
	MaxExecArgs = 256
	// MaxExecArgsSize is the maximum sum of the lengths of all arguments of
	// a command in bytes, without the program.
	MaxExecArgsSize = 16 * 1024
	// MaxPathLength is the maximum length of the program path and of the
	// working directory in bytes (PATH_MAX of macOS).
	MaxPathLength = 1024
	// MaxExecTimeoutSeconds is the absolute maximum of the time limit of a
	// command in seconds: the maximum of exec.max_timeout.
	MaxExecTimeoutSeconds = 3600
	// MinExecOutputLimit is the minimum of the output limit for each stream
	// in bytes: the minimum of exec.output_limit.
	MinExecOutputLimit = 1024
	// MaxExecOutputLimit is the maximum of the output limit for each stream
	// in bytes: the maximum of exec.output_limit.
	MaxExecOutputLimit = 1024 * 1024
	// maxSignalLength is the maximum length of a signal name.
	maxSignalLength = 16
)

// Message types of the exec way on the agent socket (D61).
const (
	// TypeRequestExec asks for the values of credentials for one command.
	// "a38 mcp" sends it.
	TypeRequestExec Type = "request_exec"
	// TypeExecCredentials is the answer to request_exec: the values and
	// the limits of the command.
	TypeExecCredentials Type = "exec_credentials"
	// TypeExecResult reports the result of the command to the daemon.
	// "a38 mcp" sends it after the end of the command.
	TypeExecResult Type = "exec_result"
	// TypeExecRecorded is the answer to exec_result (G39).
	TypeExecRecorded Type = "exec_recorded"
)

// forbiddenEnvNames are the environment names that a command cannot get
// from a credential (D64).
var forbiddenEnvNames = map[string]bool{
	"PATH": true, "HOME": true, "USER": true, "LOGNAME": true, "SHELL": true,
	"PWD": true, "TMPDIR": true, "IFS": true,
}

// forbiddenEnvPrefixes are the prefixes of the forbidden environment names
// (D64).
var forbiddenEnvPrefixes = []string{"DYLD_", "LD_", "A38_"}

// CheckEnvName checks the name of an environment variable for the exec way
// (D64). The name must match [A-Z_][A-Z0-9_]*. The names PATH, HOME, USER,
// LOGNAME, SHELL, PWD, TMPDIR and IFS and each name that starts with
// DYLD_, LD_ or A38_ are forbidden. A failure is an *Error with the code
// invalid_request. The error text repeats the name only if the name
// matches the pattern.
func CheckEnvName(name string) error {
	if name == "" {
		return invalid("the name of an environment variable is empty")
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c < 'A' || c > 'Z') && c != '_' && (c < '0' || c > '9' || i == 0) {
			return invalid("the name of an environment variable does not match [A-Z_][A-Z0-9_]*")
		}
	}
	if forbiddenEnvNames[name] {
		return invalid("the environment variable %s is forbidden", name)
	}
	for _, p := range forbiddenEnvPrefixes {
		if strings.HasPrefix(name, p) {
			return invalid("the environment variable %s is forbidden: the prefix %s is forbidden", name, p)
		}
	}
	return nil
}

// RequestExec asks the daemon for the values of credentials for one
// command (D61). "a38 mcp" sends it after it checked the parameters of the
// tool run_with_credentials and found the program. The daemon checks the
// policy, the delivery way, the grant and the working directory, and asks
// the human if necessary. A request_exec is always a bundle of its own.
//
// The daemon never starts the command. Program, Args and Cwd are what the
// approval request and the audit log show (D65).
type RequestExec struct {
	Header
	// ID is the correlation ID. The sender selects it.
	ID string `json:"id"`
	// Program is the absolute, clean path of the program, as "a38 mcp"
	// found it, with the symbolic links resolved (G48). "a38 mcp" starts
	// exactly this path. The daemon resolves it again.
	Program string `json:"program"`
	// Args are the arguments without the program. "a38 mcp" passes them
	// unchanged. At most MaxExecArgs arguments and MaxExecArgsSize bytes
	// together. Line feed and tab are permitted; other control characters
	// are not.
	Args []string `json:"args,omitempty"`
	// Cwd is the absolute, clean path of the working directory of the
	// command. The daemon checks that it is in the project of the session.
	Cwd string `json:"cwd"`
	// Env maps the environment variables to the credentials: at least one
	// entry, at most MaxReferences entries, each name once. The
	// configuration sets the smaller limit bundle.max_entries. The wire has
	// a list, because a key of a message is always in lower case.
	Env []ExecVar `json:"env"`
	// TimeoutSeconds is the time limit that the agent asks for, in
	// seconds. 0 means the default of the configuration
	// (exec.default_timeout). The protocol permits at most
	// MaxExecTimeoutSeconds; the daemon rejects a value above
	// exec.max_timeout with invalid_request.
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
	// Task is the task of the agent. It is a claim. It is mandatory.
	Task string `json:"task"`
	// Reason says why the agent needs the credentials. It is a claim. It
	// is mandatory.
	Reason string `json:"reason"`
}

// ExecVar is one entry of [RequestExec.Env]: the name of an environment
// variable and the credential that gives its value.
type ExecVar struct {
	// Name is the name of the environment variable. See [CheckEnvName].
	Name string `json:"name"`
	// Reference names the credential.
	Reference Reference `json:"reference"`
}

// ExecCredentials is the answer to [RequestExec]. It holds the values and
// the limits of the command. The daemon sends it only if it delivers all
// values; each failure is an [ErrorResponse].
type ExecCredentials struct {
	Header
	// ID is the correlation ID of the request.
	ID string `json:"id"`
	// Values holds one value for each entry of [RequestExec.Env], in the
	// same sequence.
	Values []ExecValue `json:"values"`
	// TimeoutSeconds is the time limit of the command in seconds that
	// applies: the value of the request or the default of the
	// configuration. 1 to MaxExecTimeoutSeconds.
	TimeoutSeconds int `json:"timeout_seconds"`
	// OutputLimit is the limit of the output for each stream in bytes
	// (exec.output_limit). MinExecOutputLimit to MaxExecOutputLimit.
	OutputLimit int `json:"output_limit"`
}

// ExecValue is the value of one environment variable in
// [ExecCredentials].
type ExecValue struct {
	// Name is the name of the environment variable.
	Name string `json:"name"`
	// Value is the secret value. Only [Encode] writes it to the wire.
	Value Secret `json:"value"`
}

// ExecResult reports the result of a command to the daemon (D61, D65).
// "a38 mcp" sends it after the end of the command. It never holds the
// output or a value. Each field is a claim of "a38 mcp". The daemon
// answers with [ExecRecorded].
//
// If Error is set, "a38 mcp" did not start the command after the
// delivery; then each other result field has its zero value (G39).
type ExecResult struct {
	Header
	// ID is the correlation ID of this message. The sender selects it.
	ID string `json:"id"`
	// RequestID is the correlation ID of the [RequestExec] that gave the
	// values.
	RequestID string `json:"request_id"`
	// ExitCode is the exit code of the command, 0 to 255. It is -1 if a
	// signal ended the command; then Signal is set.
	ExitCode int `json:"exit_code"`
	// Signal is the name of the signal that ended the command, for example
	// "SIGTERM". It is empty if the command ended with an exit code.
	Signal string `json:"signal,omitempty"`
	// TimedOut reports that the command reached the time limit.
	TimedOut bool `json:"timed_out"`
	// DurationMS is the run time of the command in milliseconds.
	DurationMS int64 `json:"duration_ms"`
	// StdoutBytes is the number of bytes that the command wrote to stdout,
	// before the limit.
	StdoutBytes int64 `json:"stdout_bytes"`
	// StderrBytes is the number of bytes that the command wrote to stderr,
	// before the limit.
	StderrBytes int64 `json:"stderr_bytes"`
	// StdoutTruncated reports that "a38 mcp" cut stdout to the limit.
	StdoutTruncated bool `json:"stdout_truncated"`
	// StderrTruncated reports that "a38 mcp" cut stderr to the limit.
	StderrTruncated bool `json:"stderr_truncated"`
	// Masked is the number of masked values in stdout and stderr.
	Masked int `json:"masked"`
	// Error is set if "a38 mcp" did not start the command. Its code is
	// always exec_failed.
	Error *Error `json:"error,omitempty"`
}

// ExecRecorded is the answer to [ExecResult]: the daemon received the
// result (G39). An error is an [ErrorResponse].
type ExecRecorded struct {
	Header
	// ID is the correlation ID of the exec_result message.
	ID string `json:"id"`
}

// MessageType returns [TypeRequestExec].
func (*RequestExec) MessageType() Type { return TypeRequestExec }

// MessageType returns [TypeExecCredentials].
func (*ExecCredentials) MessageType() Type { return TypeExecCredentials }

// MessageType returns [TypeExecResult].
func (*ExecResult) MessageType() Type { return TypeExecResult }

// MessageType returns [TypeExecRecorded].
func (*ExecRecorded) MessageType() Type { return TypeExecRecorded }

// checkPath checks an absolute, clean path without control characters.
func checkPath(name, value string) error {
	if value == "" {
		return invalid("the field %s is empty", name)
	}
	if len(value) > MaxPathLength {
		return invalid("the field %s is longer than %d bytes", name, MaxPathLength)
	}
	if err := checkText(name, value, MaxPathLength, false); err != nil {
		return err
	}
	if !path.IsAbs(value) {
		return invalid("the field %s is not an absolute path", name)
	}
	if path.Clean(value) != value {
		return invalid("the field %s is not a clean path", name)
	}
	return nil
}

// checkArgs checks the arguments of a command.
func checkArgs(args []string) error {
	if len(args) > MaxExecArgs {
		return invalid("the field args has more than %d entries", MaxExecArgs)
	}
	size := 0
	for _, a := range args {
		size += len(a)
		if size > MaxExecArgsSize {
			return invalid("the arguments are longer than %d bytes together", MaxExecArgsSize)
		}
		if err := checkText("args", a, MaxExecArgsSize, true); err != nil {
			return err
		}
	}
	return nil
}

// checkTimeout checks a time limit in seconds. minimum is 0 if the field
// is optional.
func checkTimeout(seconds, minimum int) error {
	if seconds < minimum || seconds > MaxExecTimeoutSeconds {
		return invalid("the field timeout_seconds must be %d to %d", minimum, MaxExecTimeoutSeconds)
	}
	return nil
}

// checkClaim checks a mandatory claim of the agent.
func checkClaim(name, value string) error {
	if value == "" {
		return invalid("the field %s is empty", name)
	}
	return checkText(name, value, MaxClaimLength, true)
}

// checkEnvNames checks the names of the environment variables: each name
// valid and once.
func checkEnvNames(field string, n int, name func(int) string) error {
	if n == 0 {
		return invalid("the field %s is empty", field)
	}
	if n > MaxReferences {
		return invalid("the field %s has more than %d entries", field, MaxReferences)
	}
	seen := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		v := name(i)
		if err := CheckEnvName(v); err != nil {
			return err
		}
		if seen[v] {
			return invalid("the environment variable %s appears twice", v)
		}
		seen[v] = true
	}
	return nil
}

// Validate checks the message. The limits of the configuration
// (bundle.max_entries, exec.max_timeout) and the project of the working
// directory are not part of this check; the daemon checks them.
func (m *RequestExec) Validate() error {
	if err := firstError(
		checkID("id", m.ID),
		checkPath("program", m.Program),
		checkArgs(m.Args),
		checkPath("cwd", m.Cwd),
		checkEnvNames("env", len(m.Env), func(i int) string { return m.Env[i].Name }),
		checkTimeout(m.TimeoutSeconds, 0),
		checkClaim("task", m.Task),
		checkClaim("reason", m.Reason),
	); err != nil {
		return err
	}
	for _, v := range m.Env {
		if err := v.Reference.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks the message. The error text never repeats a value.
func (m *ExecCredentials) Validate() error {
	if err := firstError(
		checkID("id", m.ID),
		checkEnvNames("values", len(m.Values), func(i int) string { return m.Values[i].Name }),
		checkTimeout(m.TimeoutSeconds, 1),
	); err != nil {
		return err
	}
	for _, v := range m.Values {
		if v.Value.v == nil {
			return invalid("an entry of the field values has no value")
		}
	}
	if m.OutputLimit < MinExecOutputLimit || m.OutputLimit > MaxExecOutputLimit {
		return invalid("the field output_limit must be %d to %d", MinExecOutputLimit, MaxExecOutputLimit)
	}
	return nil
}

// Validate checks the message.
func (m *ExecResult) Validate() error {
	if err := firstError(checkID("id", m.ID), checkID("request_id", m.RequestID)); err != nil {
		return err
	}
	if m.Error != nil {
		if m.Error.Code != CodeExecFailed {
			return invalid("the error of exec_result must have the code %s", CodeExecFailed)
		}
		if err := m.Error.validate(); err != nil {
			return err
		}
		zero := ExecResult{Header: m.Header, ID: m.ID, RequestID: m.RequestID, Error: m.Error}
		if *m != zero {
			return invalid("an exec_result with an error has no result fields")
		}
		return nil
	}
	if m.ExitCode < -1 || m.ExitCode > 255 {
		return invalid("the field exit_code must be -1 to 255")
	}
	if err := checkSignal(m.Signal); err != nil {
		return err
	}
	if (m.ExitCode == -1) != (m.Signal != "") {
		return invalid("the field exit_code is -1 exactly when the field signal is set")
	}
	if m.DurationMS < 0 || m.StdoutBytes < 0 || m.StderrBytes < 0 || m.Masked < 0 {
		return invalid("a count of exec_result is negative")
	}
	return nil
}

// checkSignal checks a signal name: empty, or "SIG" and upper-case letters
// and digits.
func checkSignal(s string) error {
	if s == "" {
		return nil
	}
	if len(s) > maxSignalLength || len(s) < 4 || !strings.HasPrefix(s, "SIG") {
		return invalid("the field signal is not a signal name")
	}
	for i := 3; i < len(s); i++ {
		if c := s[i]; (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return invalid("the field signal is not a signal name")
		}
	}
	return nil
}

// Validate checks the message.
func (m *ExecRecorded) Validate() error { return checkID("id", m.ID) }

// wireExecCredentials is the wire form of ExecCredentials. Next to
// wireCredentials, it is the only place that writes a secret value.
type wireExecCredentials struct {
	Header
	ID             string          `json:"id"`
	Values         []wireExecValue `json:"values"`
	TimeoutSeconds int             `json:"timeout_seconds"`
	OutputLimit    int             `json:"output_limit"`
}

type wireExecValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// wire returns the wire form of the message with the values.
func (m *ExecCredentials) wire() wireExecCredentials {
	w := wireExecCredentials{Header: m.Header, ID: m.ID, Values: make([]wireExecValue, len(m.Values)),
		TimeoutSeconds: m.TimeoutSeconds, OutputLimit: m.OutputLimit}
	for i, v := range m.Values {
		w.Values[i] = wireExecValue{Name: v.Name, Value: v.Value.Reveal()}
	}
	return w
}
