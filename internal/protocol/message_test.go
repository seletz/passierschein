package protocol

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

const testRef = "op://beispiel.app.k8s-dev/datenbank-dev/password"

func mustRef(t testing.TB, s string) Reference {
	t.Helper()
	ref, err := ParseReference(s)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func TestDecodeRejects(t *testing.T) {
	// T10 (part): unknown type, unknown field, wrong version, no JSON,
	// message above the size limit.
	// T12 (part), T28 (part): the agent socket does not know the admin messages.
	// T21 (part): request_credentials has no field for the session name.
	valid := `{"version":1,"type":"request_credentials","id":"r1","references":["` + testRef + `"],"task":"Run the tests","reason":"The tests need the login."}`
	tests := []struct {
		name string
		set  Set
		line string
	}{
		{"unknown type", AgentRequests, `{"version":1,"type":"execute","id":"r1"}`},
		{"type missing", AgentRequests, `{"version":1,"id":"r1"}`},
		{"type is a number", AgentRequests, `{"version":1,"type":7,"id":"r1"}`},
		{"unknown field", AgentRequests, strings.Replace(valid, `"task"`, `"extra":true,"task"`, 1)},
		{"field for the session name", AgentRequests, strings.Replace(valid, `"task"`, `"session_name":"beispiel-koordinator","task"`, 1)},
		{"unknown field in a nested object", AgentResponses, `{"version":1,"type":"error","error":{"code":"denied","message":"The human denied the request.","extra":1}}`},
		{"wrong version", AgentRequests, strings.Replace(valid, `"version":1`, `"version":2`, 1)},
		{"version 0", AgentRequests, strings.Replace(valid, `"version":1`, `"version":0`, 1)},
		{"version missing", AgentRequests, strings.Replace(valid, `"version":1,`, ``, 1)},
		{"version is text", AgentRequests, strings.Replace(valid, `"version":1`, `"version":"1"`, 1)},
		{"version is a fraction", AgentRequests, strings.Replace(valid, `"version":1`, `"version":1.5`, 1)},
		{"not JSON", AgentRequests, `request_credentials ` + testRef},
		{"empty line", AgentRequests, ``},
		{"only white space", AgentRequests, "  \n"},
		{"JSON array", AgentRequests, `[` + valid + `]`},
		{"JSON text", AgentRequests, `"hello"`},
		{"cut off", AgentRequests, valid[:len(valid)-10]},
		{"two objects on one line", AgentRequests, valid + valid},
		{"text after the object", AgentRequests, valid + ` x`},
		{"key in upper case", AgentRequests, strings.Replace(valid, `"task"`, `"Task"`, 1)},
		{"key used twice", AgentRequests, strings.Replace(valid, `"task"`, `"id":"r2","task"`, 1)},
		{"line break in the message", AgentRequests, strings.Replace(valid, `,"task"`, ",\n\"task\"", 1)},
		{"invalid UTF-8", AgentRequests, strings.Replace(valid, `Run`, "R\xffn", 1)},
		{"above the size limit", AgentRequests, strings.Replace(valid, `Run the tests`, strings.Repeat("a", MaxMessageSize), 1)},
		{"null", AgentRequests, `null`},
		{"id missing", AgentRequests, strings.Replace(valid, `"id":"r1",`, ``, 1)},
		{"id with a space", AgentRequests, strings.Replace(valid, `"id":"r1"`, `"id":"r 1"`, 1)},
		{"id too long", AgentRequests, strings.Replace(valid, `"id":"r1"`, `"id":"`+strings.Repeat("r", MaxIDLength+1)+`"`, 1)},
		{"no references", AgentRequests, strings.Replace(valid, `"`+testRef+`"`, ``, 1)},
		{"references missing", AgentRequests, `{"version":1,"type":"request_credentials","id":"r1","task":"Run the tests","reason":"The tests need the login."}`},
		{"reference without field", AgentRequests, strings.Replace(valid, `/password`, ``, 1)},
		{"reference is null", AgentRequests, strings.Replace(valid, `"`+testRef+`"`, `null`, 1)},
		{"too many references", AgentRequests, strings.Replace(valid, `"`+testRef+`"`, strings.TrimSuffix(strings.Repeat(`"`+testRef+`",`, MaxReferences+1), ","), 1)},
		{"control character in the task", AgentRequests, strings.Replace(valid, `Run the`, `Run\u001bthe`, 1)},
		{"direction mark in the reason", AgentRequests, strings.Replace(valid, `The tests`, `The\u202etests`, 1)},
		{"task too long", AgentRequests, strings.Replace(valid, `Run the tests`, strings.Repeat("a", MaxClaimLength+1), 1)},
		{"agent name with a control character", AgentRequests, `{"version":1,"type":"hello","agent":"coding\u0000agent"}`},
		{"agent name too long", AgentRequests, `{"version":1,"type":"hello","agent":"` + strings.Repeat("a", MaxAgentLength+1) + `"}`},
		{"admin list on the agent socket", AgentRequests, `{"version":1,"type":"grants_list","id":"a1"}`},
		{"admin revoke on the agent socket", AgentRequests, `{"version":1,"type":"grant_revoke","id":"a1","grant_id":"grant-0007"}`},
		{"admin fault clear on the agent socket", AgentRequests, `{"version":1,"type":"fault_clear","id":"a1"}`},
		{"admin status on the agent socket", AgentRequests, `{"version":1,"type":"status","id":"a1"}`},
		{"response sent as a request", AgentRequests, `{"version":1,"type":"welcome","session_id":"s-1"}`},
		{"agent request on the admin socket", AdminRequests, valid},
		{"revoke without grant", AdminRequests, `{"version":1,"type":"grant_revoke","id":"a1"}`},
		{"error with an unknown code", AgentResponses, `{"version":1,"type":"error","error":{"code":"try_again","message":"x"}}`},
		{"error without code", AgentResponses, `{"version":1,"type":"error","id":"r1"}`},
		{"entry with value and error", AgentResponses, `{"version":1,"type":"credentials","id":"r1","entries":[{"reference":"` + testRef + `","value":"x","error":{"code":"denied","message":"x"}}]}`},
		{"entry without value and error", AgentResponses, `{"version":1,"type":"credentials","id":"r1","entries":[{"reference":"` + testRef + `"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := tt.set.Decode([]byte(tt.line))
			if err == nil {
				t.Fatalf("Decode accepted the message: %T", msg)
			}
			if msg != nil {
				t.Errorf("Decode returned a message together with an error")
			}
			if got := CodeOf(err); got != CodeInvalidRequest {
				t.Errorf("code = %q, want %q (error: %v)", got, CodeInvalidRequest, err)
			}
		})
	}
}

func TestDecodeErrorHidesContent(t *testing.T) {
	// T8 (part): a decode error does not repeat the content of the message.
	const marker = "kennwert-nur-fuer-tests"
	lines := []string{
		`{"version":1,"type":"credentials","id":"r1","entries":[{"reference":"` + testRef + `","value":"` + marker + `","extra":1}]}`,
		`{"version":1,"type":"credentials","id":"r1","entries":[{"reference":"` + testRef + `","value":"` + marker + `"`,
		`{"version":1,"type":"credentials","id":"r1","` + marker + `":1}`,
		`{"version":1,"type":"` + marker + `"}`,
	}
	for _, line := range lines {
		_, err := AgentResponses.Decode([]byte(line))
		if err == nil {
			t.Fatalf("Decode accepted %q", line)
		}
		if strings.Contains(err.Error(), marker) {
			t.Errorf("the error contains content of the message: %v", err)
		}
	}
}

func roundTripCases(t testing.TB) []struct {
	name string
	set  Set
	msg  Message
} {
	ref := mustRef(t, testRef)
	ref2 := mustRef(t, "op://beispiel.app.k8s-dev/objektspeicher-dev/access-key")
	value := NewSecret("kennwert-nur-fuer-tests")
	grantedAt := time.Date(2026, 11, 11, 17, 30, 0, 0, time.UTC)
	expiresAt := grantedAt.Add(4 * time.Hour)
	// T32 (part): the messages of the exec way are part of the round trip.
	return append([]struct {
		name string
		set  Set
		msg  Message
	}{
		{"hello", AgentRequests, &Hello{SessionName: "beispiel-koordinator", Agent: "coding-agent"}},
		{"hello without name", AgentRequests, &Hello{}},
		{"welcome", AgentResponses, &Welcome{SessionID: "s-0001", Project: "beispielprojekt"}},
		{"request_credentials", AgentRequests, &RequestCredentials{ID: "r1", References: []Reference{ref, ref2},
			Task: "Run the database migration tests", Reason: "The tests need the login\nfor the dev database."}},
		{"credentials", AgentResponses, &Credentials{ID: "r1", Entries: []CredentialEntry{
			{Reference: ref, Value: &value},
			{Reference: ref2, Error: &Error{Code: CodeDenied, Message: "The human did not select this credential."}},
		}}},
		{"grant_status", AgentRequests, &GrantStatus{ID: "r2"}},
		{"grants for a session", AgentResponses, &Grants{ID: "r2", Grants: []Grant{
			{GrantID: "grant-0007", References: []Reference{ref}, GrantedAt: grantedAt, ExpiresAt: expiresAt, RemainingSeconds: 14400},
		}}},
		{"no grants", AgentResponses, &Grants{ID: "r2", Grants: []Grant{}}},
		{"error", AgentResponses, &ErrorResponse{ID: "r1", Error: Error{Code: CodeNotAllowed, Message: "The policy does not allow the request."}}},
		{"error without id", AgentResponses, &ErrorResponse{Error: Error{Code: CodeInvalidRequest, Message: "The message is not valid."}}},
		{"grants_list", AdminRequests, &GrantsList{ID: "a1"}},
		{"grants for the admin", AdminResponses, &Grants{ID: "a1", Grants: []Grant{
			{GrantID: "grant-0007", SessionID: "s-0001", SessionName: "beispiel-koordinator", Project: "beispielprojekt",
				References: []Reference{ref}, GrantedAt: grantedAt, ExpiresAt: expiresAt, RemainingSeconds: 14400},
		}}},
		{"grant_revoke", AdminRequests, &GrantRevoke{ID: "a2", GrantID: "grant-0007"}},
		{"grant_revoked", AdminResponses, &GrantRevoked{ID: "a2", GrantID: "grant-0007"}},
		{"status", AdminRequests, &Status{ID: "a3"}},
		{"status_result", AdminResponses, &StatusResult{ID: "a3", DaemonVersion: "0.1.0", Sessions: 2, Grants: 1,
			Fault:           &Fault{Code: CodeAuditFailed, Reason: "The daemon cannot write the audit log.", Since: grantedAt},
			InvalidProjects: []InvalidProject{{Project: "beispielprojekt", Reason: `The vault "beispiel.infra.k8s-dev" is not in the vault list.`}}}},
		{"status_result without findings", AdminResponses, &StatusResult{ID: "a3", DaemonVersion: "0.1.0"}},
		{"status_result with the check of the cause", AdminResponses, &StatusResult{ID: "a3", DaemonVersion: "0.1.0",
			Fault: &Fault{Code: CodeAuditFailed, Reason: "The daemon cannot write the audit log.", Since: grantedAt,
				CheckPassed: new(bool), Finding: "audit: cannot write the audit file", BeginPending: true},
			Backend: &BackendStatus{State: BackendLocked, Reason: "The backend is locked."},
			UI:      &UIStatus{Mode: "script", Usable: true}}},
		// T49: the status names the state of each 1Password account (D78).
		{"status_result with accounts", AdminResponses, &StatusResult{ID: "a3", DaemonVersion: "0.1.0",
			Backend: &BackendStatus{Kind: "1password", State: BackendOK, Accounts: []AccountStatus{
				{Name: "privat", State: BackendOK, VaultsConfigured: 2, VaultsVisible: 2},
				{Name: "firma", State: BackendLocked, Reason: "1Password rejects the token of the account.", VaultsConfigured: 1},
			}}}},
		{"fault_clear", AdminRequests, &FaultClear{ID: "a4"}},
		{"fault_clear_result", AdminResponses, &FaultClearResult{ID: "a4", Cleared: false, Finding: "The daemon cannot write the audit log."}},
		{"admin error", AdminResponses, &ErrorResponse{ID: "a2", Error: Error{Code: CodeNotFound, Message: "The grant does not exist."}}},
	}, execRoundTripCases(t)...)
}

func TestRoundTrip(t *testing.T) {
	for _, tt := range roundTripCases(t) {
		t.Run(tt.name, func(t *testing.T) {
			line, err := Encode(tt.msg)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if bytes.ContainsAny(line, "\n\r") {
				t.Errorf("the encoded message has a line break: %q", line)
			}
			if !bytes.HasPrefix(line, []byte(`{"version":1,"type":"`+string(tt.msg.MessageType())+`"`)) {
				t.Errorf("the encoded message does not start with version and type: %s", line)
			}
			got, err := tt.set.Decode(line)
			if err != nil {
				t.Fatalf("Decode(%s): %v", line, err)
			}
			if !reflect.DeepEqual(got, tt.msg) {
				t.Errorf("round trip = %#v, want %#v", got, tt.msg)
			}
		})
	}
}

func TestEncodeRejects(t *testing.T) {
	ref := mustRef(t, testRef)
	tests := []struct {
		name string
		msg  Message
	}{
		{"nil", nil},
		{"invalid message", &RequestCredentials{ID: "r1"}},
		{"zero reference", &RequestCredentials{ID: "r1", References: []Reference{{}}}},
		{"above the size limit", &RequestCredentials{ID: "r1", References: []Reference{ref}, Task: "t",
			Reason: strings.Repeat("a", MaxClaimLength+1)}},
		{"unknown code", &ErrorResponse{Error: Error{Code: "try_again", Message: "x"}}},
		{"unknown backend state", &StatusResult{ID: "a3", Backend: &BackendStatus{State: "sleepy"}}},
		{"unknown account state", &StatusResult{ID: "a3", Backend: &BackendStatus{State: BackendOK,
			Accounts: []AccountStatus{{Name: "privat", State: "sleepy"}}}}},
		{"account without name", &StatusResult{ID: "a3", Backend: &BackendStatus{State: BackendOK,
			Accounts: []AccountStatus{{State: BackendOK}}}}},
		{"control character in an account reason", &StatusResult{ID: "a3", Backend: &BackendStatus{State: BackendOK,
			Accounts: []AccountStatus{{Name: "privat", State: BackendLocked, Reason: "a\x00b"}}}}},
		{"control character in a fault finding", &StatusResult{ID: "a3",
			Fault: &Fault{Code: CodeAuditFailed, Reason: "x", Finding: "a\x00b"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if line, err := Encode(tt.msg); err == nil {
				t.Errorf("Encode accepted the message: %s", line)
			}
		})
	}
}

func TestCredentialsValueOnlyOnTheWire(t *testing.T) {
	// T8 (part): only Encode writes a secret value.
	const marker = "kennwert-nur-fuer-tests"
	value := NewSecret(marker)
	msg := &Credentials{ID: "r1", Entries: []CredentialEntry{{Reference: mustRef(t, testRef), Value: &value}}}
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
}

func TestReadLine(t *testing.T) {
	// T10 (part): size limit and a connection that ends in the middle of a message.
	long := strings.Repeat("a", MaxMessageSize-1)
	tests := []struct {
		name      string
		in        string
		want      []string
		wantErr   error
		wantCode  Code
		bufferLen int
	}{
		{"two lines", "{\"a\":1}\n{\"b\":2}\n", []string{`{"a":1}`, `{"b":2}`}, io.EOF, "", 0},
		{"CR LF", "{\"a\":1}\r\n", []string{`{"a":1}`}, io.EOF, "", 0},
		{"end in the middle of a message", "{\"a\":1}\n{\"b\"", []string{`{"a":1}`}, io.ErrUnexpectedEOF, "", 0},
		{"empty stream", "", nil, io.EOF, "", 0},
		{"exactly the limit", long + "\n", []string{long}, io.EOF, "", 0},
		{"above the limit", long + "a\n{\"a\":1}\n", nil, nil, CodeInvalidRequest, 0},
		{"above the limit without line end", long + "aaaa", nil, nil, CodeInvalidRequest, 0},
		{"small buffer", "{\"a\":1}\n" + long + "\n", []string{`{"a":1}`, long}, io.EOF, "", 16},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := tt.bufferLen
			if size == 0 {
				size = 4096
			}
			r := bufio.NewReaderSize(strings.NewReader(tt.in), size)
			var got []string
			var err error
			for {
				var line []byte
				line, err = ReadLine(r)
				if err != nil {
					break
				}
				got = append(got, string(line))
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %d lines, want %d", len(got), len(tt.want))
			}
			if tt.wantCode != "" {
				if CodeOf(err) != tt.wantCode {
					t.Errorf("error = %v, want code %q", err, tt.wantCode)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestWrite(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, &GrantStatus{ID: "r2"}); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), `{"version":1,"type":"grant_status","id":"r2"}`+"\n"; got != want {
		t.Errorf("Write = %q, want %q", got, want)
	}
	if err := Write(&buf, &GrantStatus{}); err == nil {
		t.Error("Write accepted an invalid message")
	}
}

func FuzzDecode(f *testing.F) {
	// T10 (part): the decoder does not crash on any input.
	for _, tt := range roundTripCases(f) {
		line, err := Encode(tt.msg)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(line)
	}
	f.Add([]byte(`{"version":1,"type":"hello","session_name":"\u0000"}`))
	f.Add([]byte(`{"version":1e0,"type":"hello"}`))
	f.Add([]byte(`{"version":1,"type":"request_credentials","id":"r1","references":[{}]}`))
	sets := []Set{AgentRequests, AgentResponses, AdminRequests, AdminResponses}
	f.Fuzz(func(t *testing.T, line []byte) {
		for _, set := range sets {
			msg, err := set.Decode(line)
			if err != nil {
				if CodeOf(err) != CodeInvalidRequest {
					t.Fatalf("error without the code invalid_request: %v", err)
				}
				continue
			}
			again, err := Encode(msg)
			if err != nil {
				t.Fatalf("Encode of a decoded message: %v", err)
			}
			back, err := set.Decode(again)
			if err != nil {
				t.Fatalf("Decode of an encoded message: %v", err)
			}
			third, err := Encode(back)
			if err != nil || !bytes.Equal(again, third) {
				t.Fatalf("round trip changed the message")
			}
		}
	})
}
