package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// validRequestExec returns a valid request_exec message for the tests.
func validRequestExec(t testing.TB) *RequestExec {
	t.Helper()
	return &RequestExec{
		ID:      "c-1",
		Program: "/opt/homebrew/bin/psql",
		Args:    []string{"-h", "datenbank-dev.beispiel.internal", "-c", "select count(*)\nfrom auftrag"},
		Cwd:     "/Users/beispiel/develop/beispielprojekt",
		Env: []ExecVar{
			{Name: "PGPASSWORD", Reference: mustRef(t, testRef)},
		},
		TimeoutSeconds: 120,
		Task:           "Count the orders in dev",
		Reason:         "Verify migration 42",
	}
}

// execRoundTripCases returns the messages of the exec way for the round
// trip test and the fuzz corpus.
func execRoundTripCases(t testing.TB) []struct {
	name string
	set  Set
	msg  Message
} {
	value := NewSecret("kennwert-nur-fuer-tests")
	empty := NewSecret("")
	minimal := &RequestExec{ID: "c-2", Program: "/usr/bin/env", Cwd: "/",
		Env:  []ExecVar{{Name: "DB_PASSWORD", Reference: mustRef(t, testRef)}},
		Task: "t", Reason: "r"}
	return []struct {
		name string
		set  Set
		msg  Message
	}{
		{"request_exec", AgentRequests, validRequestExec(t)},
		{"request_exec without args and timeout", AgentRequests, minimal},
		{"exec_credentials", AgentResponses, &ExecCredentials{ID: "c-1",
			Values:         []ExecValue{{Name: "PGPASSWORD", Value: value}, {Name: "PGUSER", Value: empty}},
			TimeoutSeconds: 120, OutputLimit: 64 * 1024}},
		{"exec_result", AgentRequests, &ExecResult{ID: "c-3", RequestID: "c-1", ExitCode: 3,
			DurationMS: 412, StdoutBytes: 200 * 1024, StderrBytes: 12, StdoutTruncated: true, Masked: 2}},
		{"exec_result after a timeout", AgentRequests, &ExecResult{ID: "c-3", RequestID: "c-1", ExitCode: -1,
			Signal: "SIGKILL", TimedOut: true, DurationMS: 125000}},
		{"exec_result without start", AgentRequests, &ExecResult{ID: "c-3", RequestID: "c-1",
			Error: &Error{Code: CodeExecFailed, Message: "the program is not executable"}}},
		{"exec_recorded", AgentResponses, &ExecRecorded{ID: "c-3"}},
		{"error exec_required", AgentResponses, &ErrorResponse{ID: "c-4",
			Error: Error{Code: CodeExecRequired, Message: "the vault delivers only on the exec way"}}},
	}
}

func TestExecWireFields(t *testing.T) {
	// T32 (part): the field names on the wire are the contract for the
	// issues 96 to 99. A change here is a change of the protocol.
	value := NewSecret("kennwert-nur-fuer-tests")
	tests := []struct {
		name string
		msg  Message
		want string
	}{
		{"request_exec", validRequestExec(t),
			`{"version":1,"type":"request_exec","id":"c-1","program":"/opt/homebrew/bin/psql",` +
				`"args":["-h","datenbank-dev.beispiel.internal","-c","select count(*)\nfrom auftrag"],` +
				`"cwd":"/Users/beispiel/develop/beispielprojekt",` +
				`"env":[{"name":"PGPASSWORD","reference":"` + testRef + `"}],` +
				`"timeout_seconds":120,"task":"Count the orders in dev","reason":"Verify migration 42"}`},
		{"exec_credentials", &ExecCredentials{ID: "c-1", Values: []ExecValue{{Name: "PGPASSWORD", Value: value}},
			TimeoutSeconds: 120, OutputLimit: 65536},
			`{"version":1,"type":"exec_credentials","id":"c-1",` +
				`"values":[{"name":"PGPASSWORD","value":"kennwert-nur-fuer-tests"}],` +
				`"timeout_seconds":120,"output_limit":65536}`},
		{"exec_result", &ExecResult{ID: "c-3", RequestID: "c-1", ExitCode: 0, DurationMS: 412, StdoutBytes: 10},
			`{"version":1,"type":"exec_result","id":"c-3","request_id":"c-1","exit_code":0,"timed_out":false,` +
				`"duration_ms":412,"stdout_bytes":10,"stderr_bytes":0,"stdout_truncated":false,` +
				`"stderr_truncated":false,"masked":0}`},
		{"exec_result without start", &ExecResult{ID: "c-3", RequestID: "c-1",
			Error: &Error{Code: CodeExecFailed, Message: "x"}},
			`{"version":1,"type":"exec_result","id":"c-3","request_id":"c-1","exit_code":0,"timed_out":false,` +
				`"duration_ms":0,"stdout_bytes":0,"stderr_bytes":0,"stdout_truncated":false,` +
				`"stderr_truncated":false,"masked":0,"error":{"code":"exec_failed","message":"x"}}`},
		{"exec_recorded", &ExecRecorded{ID: "c-3"}, `{"version":1,"type":"exec_recorded","id":"c-3"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line, err := Encode(tt.msg)
			if err != nil {
				t.Fatal(err)
			}
			if string(line) != tt.want {
				t.Errorf("Encode =\n%s\nwant\n%s", line, tt.want)
			}
		})
	}
}

func TestExecValueOnlyOnTheWire(t *testing.T) {
	// T8 (part), T32 (part): only Encode writes the value of exec_credentials.
	const marker = "kennwert-nur-fuer-tests"
	msg := &ExecCredentials{ID: "c-1", Values: []ExecValue{{Name: "PGPASSWORD", Value: NewSecret(marker)}},
		TimeoutSeconds: 120, OutputLimit: 65536}
	line, err := Encode(msg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(line, []byte(marker)) {
		t.Error("Encode does not write the value")
	}
	plain, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(plain, []byte(marker)) {
		t.Error("json.Marshal of the message writes the value")
	}
	for _, out := range []string{fmt.Sprintf("%v", msg), fmt.Sprintf("%+v", msg), fmt.Sprintf("%#v", msg)} {
		if strings.Contains(out, marker) {
			t.Error("package fmt writes the value")
		}
	}
}

func TestCheckEnvName(t *testing.T) {
	// T34 (part): the pattern [A-Z_][A-Z0-9_]* and the forbidden names of D64.
	tests := []struct {
		name string
		ok   bool
	}{
		{"PGPASSWORD", true},
		{"DB_PASSWORD", true},
		{"_X", true},
		{"AWS_SECRET_ACCESS_KEY2", true},
		{"PATHX", true},
		{"MY_PATH", true},
		{"LD", true},
		{"A38", true},
		{"", false},
		{"kleinbuchstaben", false},
		{"Pgpassword", false},
		{"1PASSWORD", false},
		{"DB-PASSWORD", false},
		{"DB PASSWORD", false},
		{"DB=PASSWORD", false},
		{"PÄSSWORD", false},
		{"PATH", false},
		{"HOME", false},
		{"USER", false},
		{"LOGNAME", false},
		{"SHELL", false},
		{"PWD", false},
		{"TMPDIR", false},
		{"IFS", false},
		{"DYLD_INSERT_LIBRARIES", false},
		{"DYLD_", false},
		{"LD_PRELOAD", false},
		{"A38_CONFIG", false},
		{"A38_", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckEnvName(tt.name)
			if tt.ok {
				if err != nil {
					t.Errorf("CheckEnvName(%q) = %v, want nil", tt.name, err)
				}
				return
			}
			if CodeOf(err) != CodeInvalidRequest {
				t.Errorf("CheckEnvName(%q) = %v, want invalid_request", tt.name, err)
			}
		})
	}
}

func TestRequestExecValidate(t *testing.T) {
	// T34 (part): request_exec carries no forbidden name, no relative path,
	// no command above 256 arguments or 16 KiB and no timeout above 1 hour.
	ref := mustRef(t, testRef)
	manyVars := make([]ExecVar, MaxReferences+1)
	for i := range manyVars {
		manyVars[i] = ExecVar{Name: fmt.Sprintf("VAR_%d", i), Reference: ref}
	}
	tests := []struct {
		name   string
		change func(m *RequestExec)
		ok     bool
	}{
		{"valid", func(*RequestExec) {}, true},
		{"256 arguments", func(m *RequestExec) { m.Args = make([]string, MaxExecArgs) }, true},
		{"arguments of exactly 16 KiB", func(m *RequestExec) {
			m.Args = []string{strings.Repeat("a", MaxExecArgsSize/2), strings.Repeat("b", MaxExecArgsSize/2)}
		}, true},
		{"timeout 0 takes the default", func(m *RequestExec) { m.TimeoutSeconds = 0 }, true},
		{"timeout of 1 hour", func(m *RequestExec) { m.TimeoutSeconds = MaxExecTimeoutSeconds }, true},
		{"empty argument", func(m *RequestExec) { m.Args = []string{""} }, true},
		{"same reference for two names", func(m *RequestExec) {
			m.Env = []ExecVar{{Name: "A", Reference: ref}, {Name: "B", Reference: ref}}
		}, true},
		{"id missing", func(m *RequestExec) { m.ID = "" }, false},
		{"program missing", func(m *RequestExec) { m.Program = "" }, false},
		{"program not absolute", func(m *RequestExec) { m.Program = "psql" }, false},
		{"program relative with a slash", func(m *RequestExec) { m.Program = "./bin/psql" }, false},
		{"program not clean", func(m *RequestExec) { m.Program = "/opt/homebrew/../bin/psql" }, false},
		{"program with a control character", func(m *RequestExec) { m.Program = "/bin/p\x1bsql" }, false},
		{"program with a line feed", func(m *RequestExec) { m.Program = "/bin/p\nsql" }, false},
		{"program too long", func(m *RequestExec) { m.Program = "/" + strings.Repeat("a", MaxPathLength) }, false},
		{"cwd missing", func(m *RequestExec) { m.Cwd = "" }, false},
		{"cwd not absolute", func(m *RequestExec) { m.Cwd = "develop/beispielprojekt" }, false},
		{"cwd not clean", func(m *RequestExec) { m.Cwd = "/Users/beispiel/develop/beispielprojekt/" }, false},
		{"257 arguments", func(m *RequestExec) { m.Args = make([]string, MaxExecArgs+1) }, false},
		{"arguments above 16 KiB", func(m *RequestExec) {
			m.Args = []string{strings.Repeat("a", MaxExecArgsSize/2), strings.Repeat("b", MaxExecArgsSize/2+1)}
		}, false},
		{"argument with NUL", func(m *RequestExec) { m.Args = []string{"a\x00b"} }, false},
		{"argument with a direction mark", func(m *RequestExec) { m.Args = []string{"a\u202eb"} }, false},
		{"env missing", func(m *RequestExec) { m.Env = nil }, false},
		{"env with PATH", func(m *RequestExec) { m.Env[0].Name = "PATH" }, false},
		{"env with DYLD_INSERT_LIBRARIES", func(m *RequestExec) { m.Env[0].Name = "DYLD_INSERT_LIBRARIES" }, false},
		{"env with A38_CONFIG", func(m *RequestExec) { m.Env[0].Name = "A38_CONFIG" }, false},
		{"env with kleinbuchstaben", func(m *RequestExec) { m.Env[0].Name = "kleinbuchstaben" }, false},
		{"env with a name twice", func(m *RequestExec) {
			m.Env = []ExecVar{{Name: "A", Reference: ref}, {Name: "A", Reference: ref}}
		}, false},
		{"env with a zero reference", func(m *RequestExec) { m.Env[0].Reference = Reference{} }, false},
		{"env with too many entries", func(m *RequestExec) { m.Env = manyVars }, false},
		{"negative timeout", func(m *RequestExec) { m.TimeoutSeconds = -1 }, false},
		{"timeout above 1 hour", func(m *RequestExec) { m.TimeoutSeconds = MaxExecTimeoutSeconds + 1 }, false},
		{"task missing", func(m *RequestExec) { m.Task = "" }, false},
		{"reason missing", func(m *RequestExec) { m.Reason = "" }, false},
		{"task too long", func(m *RequestExec) { m.Task = strings.Repeat("a", MaxClaimLength+1) }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validRequestExec(t)
			tt.change(m)
			err := m.Validate()
			if tt.ok {
				if err != nil {
					t.Errorf("Validate = %v, want nil", err)
				}
				return
			}
			if CodeOf(err) != CodeInvalidRequest {
				t.Errorf("Validate = %v, want invalid_request", err)
			}
		})
	}
}

func TestExecCredentialsValidate(t *testing.T) {
	// T34 (part): exec_credentials carries valid limits and one value for
	// each name.
	value := NewSecret("kennwert-nur-fuer-tests")
	valid := func() *ExecCredentials {
		return &ExecCredentials{ID: "c-1", Values: []ExecValue{{Name: "PGPASSWORD", Value: value}},
			TimeoutSeconds: 120, OutputLimit: 64 * 1024}
	}
	tests := []struct {
		name   string
		change func(m *ExecCredentials)
		ok     bool
	}{
		{"valid", func(*ExecCredentials) {}, true},
		{"smallest limits", func(m *ExecCredentials) { m.TimeoutSeconds, m.OutputLimit = 1, MinExecOutputLimit }, true},
		{"largest limits", func(m *ExecCredentials) {
			m.TimeoutSeconds, m.OutputLimit = MaxExecTimeoutSeconds, MaxExecOutputLimit
		}, true},
		{"id missing", func(m *ExecCredentials) { m.ID = "" }, false},
		{"no values", func(m *ExecCredentials) { m.Values = nil }, false},
		{"value not set", func(m *ExecCredentials) { m.Values[0].Value = Secret{} }, false},
		{"forbidden name", func(m *ExecCredentials) { m.Values[0].Name = "LD_PRELOAD" }, false},
		{"name twice", func(m *ExecCredentials) { m.Values = append(m.Values, m.Values[0]) }, false},
		{"timeout 0", func(m *ExecCredentials) { m.TimeoutSeconds = 0 }, false},
		{"timeout above 1 hour", func(m *ExecCredentials) { m.TimeoutSeconds = MaxExecTimeoutSeconds + 1 }, false},
		{"output limit below 1 KiB", func(m *ExecCredentials) { m.OutputLimit = MinExecOutputLimit - 1 }, false},
		{"output limit above 1 MiB", func(m *ExecCredentials) { m.OutputLimit = MaxExecOutputLimit + 1 }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := valid()
			tt.change(m)
			err := m.Validate()
			if tt.ok {
				if err != nil {
					t.Errorf("Validate = %v, want nil", err)
				}
				return
			}
			if CodeOf(err) != CodeInvalidRequest {
				t.Errorf("Validate = %v, want invalid_request", err)
			}
		})
	}
}

func TestExecResultValidate(t *testing.T) {
	// T32 (part), T34 (part): exec_result carries counts and flags only;
	// a failed start carries only the code exec_failed.
	valid := func() *ExecResult {
		return &ExecResult{ID: "c-3", RequestID: "c-1", ExitCode: 0, DurationMS: 412, StdoutBytes: 10}
	}
	tests := []struct {
		name   string
		change func(m *ExecResult)
		ok     bool
	}{
		{"valid", func(*ExecResult) {}, true},
		{"exit code 255", func(m *ExecResult) { m.ExitCode = 255 }, true},
		{"signal", func(m *ExecResult) { m.ExitCode, m.Signal, m.TimedOut = -1, "SIGTERM", true }, true},
		{"timeout with an exit code", func(m *ExecResult) { m.ExitCode, m.TimedOut = 143, true }, true},
		{"failed start", func(m *ExecResult) {
			*m = ExecResult{ID: "c-3", RequestID: "c-1", Error: &Error{Code: CodeExecFailed, Message: "x"}}
		}, true},
		{"id missing", func(m *ExecResult) { m.ID = "" }, false},
		{"request_id missing", func(m *ExecResult) { m.RequestID = "" }, false},
		{"exit code 256", func(m *ExecResult) { m.ExitCode = 256 }, false},
		{"exit code -2", func(m *ExecResult) { m.ExitCode = -2 }, false},
		{"exit code -1 without signal", func(m *ExecResult) { m.ExitCode = -1 }, false},
		{"signal with exit code 0", func(m *ExecResult) { m.Signal = "SIGTERM" }, false},
		{"signal in lower case", func(m *ExecResult) { m.ExitCode, m.Signal = -1, "sigterm" }, false},
		{"signal without prefix", func(m *ExecResult) { m.ExitCode, m.Signal = -1, "TERM" }, false},
		{"signal too long", func(m *ExecResult) { m.ExitCode, m.Signal = -1, "SIG"+strings.Repeat("A", 14) }, false},
		{"negative duration", func(m *ExecResult) { m.DurationMS = -1 }, false},
		{"negative stdout bytes", func(m *ExecResult) { m.StdoutBytes = -1 }, false},
		{"negative stderr bytes", func(m *ExecResult) { m.StderrBytes = -1 }, false},
		{"negative masked", func(m *ExecResult) { m.Masked = -1 }, false},
		{"error with another code", func(m *ExecResult) {
			*m = ExecResult{ID: "c-3", RequestID: "c-1", Error: &Error{Code: CodeDenied, Message: "x"}}
		}, false},
		{"error together with a result", func(m *ExecResult) {
			m.Error = &Error{Code: CodeExecFailed, Message: "x"}
		}, false},
		{"error together with an exit code", func(m *ExecResult) {
			*m = ExecResult{ID: "c-3", RequestID: "c-1", ExitCode: 1, Error: &Error{Code: CodeExecFailed, Message: "x"}}
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := valid()
			tt.change(m)
			err := m.Validate()
			if tt.ok {
				if err != nil {
					t.Errorf("Validate = %v, want nil", err)
				}
				return
			}
			if CodeOf(err) != CodeInvalidRequest {
				t.Errorf("Validate = %v, want invalid_request", err)
			}
		})
	}
}

func TestExecDecodeRejects(t *testing.T) {
	// T34 (part): the decoder rejects a forbidden name and the messages of
	// the exec way on the wrong socket or in the wrong direction.
	valid, err := Encode(validRequestExec(t))
	if err != nil {
		t.Fatal(err)
	}
	line := string(valid)
	tests := []struct {
		name string
		set  Set
		line string
	}{
		{"forbidden name", AgentRequests, strings.Replace(line, `"PGPASSWORD"`, `"PATH"`, 1)},
		{"env as an object", AgentRequests, strings.Replace(line,
			`[{"name":"PGPASSWORD","reference":"`+testRef+`"}]`, `{"PGPASSWORD":"`+testRef+`"}`, 1)},
		{"timeout as text", AgentRequests, strings.Replace(line, `"timeout_seconds":120`, `"timeout_seconds":"120"`, 1)},
		{"timeout as a fraction", AgentRequests, strings.Replace(line, `"timeout_seconds":120`, `"timeout_seconds":1.5`, 1)},
		{"output in exec_result", AgentRequests,
			`{"version":1,"type":"exec_result","id":"c-3","request_id":"c-1","exit_code":0,"timed_out":false,` +
				`"duration_ms":1,"stdout_bytes":0,"stderr_bytes":0,"stdout_truncated":false,"stderr_truncated":false,` +
				`"masked":0,"stdout":"x"}`},
		{"request_exec on the admin socket", AdminRequests, line},
		{"request_exec as a response", AgentResponses, line},
		{"exec_credentials as a request", AgentRequests,
			`{"version":1,"type":"exec_credentials","id":"c-1","values":[{"name":"A","value":"x"}],"timeout_seconds":1,"output_limit":1024}`},
		{"exec_credentials without value", AgentResponses,
			`{"version":1,"type":"exec_credentials","id":"c-1","values":[{"name":"A"}],"timeout_seconds":1,"output_limit":1024}`},
		{"exec_credentials with a null value", AgentResponses,
			`{"version":1,"type":"exec_credentials","id":"c-1","values":[{"name":"A","value":null}],"timeout_seconds":1,"output_limit":1024}`},
		{"exec_recorded as a request", AgentRequests, `{"version":1,"type":"exec_recorded","id":"c-3"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := tt.set.Decode([]byte(tt.line))
			if err == nil {
				t.Fatalf("Decode accepted the message: %T", msg)
			}
			if got := CodeOf(err); got != CodeInvalidRequest {
				t.Errorf("code = %q, want %q (error: %v)", got, CodeInvalidRequest, err)
			}
		})
	}
}

func TestExecErrorHidesContent(t *testing.T) {
	// T8 (part): a rejected env name or a rejected exec_credentials does not
	// repeat content of the message.
	const marker = "kennwert-nur-fuer-tests"
	if err := CheckEnvName(marker); err == nil || strings.Contains(err.Error(), marker) {
		t.Errorf("CheckEnvName repeats the name or accepts it: %v", err)
	}
	m := &ExecCredentials{ID: "c-1", Values: []ExecValue{{Name: "PATH", Value: NewSecret(marker)}},
		TimeoutSeconds: 1, OutputLimit: MinExecOutputLimit}
	if err := m.Validate(); err == nil || strings.Contains(err.Error(), marker) {
		t.Errorf("Validate repeats the value or accepts it: %v", err)
	}
}
