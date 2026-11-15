package approval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/session"
)

func TestModeFaked(t *testing.T) {
	// T9 (part): only the mode "app" is not a faked approval.
	tests := []struct {
		mode Mode
		want bool
	}{
		{ModeApp, false},
		{ModeAutoApprove, true},
		{ModeAutoDeny, true},
		{ModeScript, true},
		{Mode("other"), true},
		{Mode(""), true},
	}
	for _, tt := range tests {
		if got := tt.mode.Faked(); got != tt.want {
			t.Errorf("Mode(%q).Faked() = %v, want %v", tt.mode, got, tt.want)
		}
	}
}

// TestPrototypeSamples checks that the request type reads the sample
// requests of the UI prototype. The prototype has no session ID, no session
// name and no field name; these are additions.
//
// The spike stays as a record. The contract with the approval app is the
// directory app/Samples now; TestAppSamples reads it.
func TestPrototypeSamples(t *testing.T) {
	samples, err := filepath.Glob("../../spikes/ui-popup/samples/*.json")
	if err != nil || len(samples) == 0 {
		t.Fatalf("no sample requests found: %v", err)
	}
	for _, path := range samples {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var req Request
			dec := json.NewDecoder(strings.NewReader(string(data)))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&req); err != nil {
				// The facts of a VM come with stage 5; session.Facts does
				// not have them yet.
				if strings.Contains(err.Error(), "vm_name") || strings.Contains(err.Error(), "ssh_session") {
					t.Skipf("sample of stage 5 (VM facts): %v", err)
				}
				t.Fatalf("Decode: %v", err)
			}
			if req.Version != Version || req.Type != TypeRequest {
				t.Errorf("version = %d, type = %q", req.Version, req.Type)
			}
			if req.RequestID == "" || len(req.Credentials) == 0 || req.Grant.TTLSeconds <= 0 {
				t.Errorf("the request lost fields: %+v", req)
			}
			if req.Verified.Origin != session.OriginLocal || req.Verified.Project == "" {
				t.Errorf("verified = %+v", req.Verified)
			}
		})
	}
}

func TestDecisionJSON(t *testing.T) {
	// The example decisions of the UI spike.
	tests := []struct {
		name string
		line string
		want Decision
	}{
		{"approved",
			`{"approved_credential_ids":["c1"],"auth":{"ok":true,"policy":"biometrics"},"elapsed_ms":8421,"outcome":"approved","request_id":"req-a-0001","ttl_seconds":14400,"type":"approval_decision","version":1}`,
			Decision{Version: 1, Type: TypeDecision, RequestID: "req-a-0001", Outcome: OutcomeApproved,
				ApprovedCredentialIDs: []string{"c1"}, TTLSeconds: 14400,
				Auth: &AuthResult{Policy: "biometrics", OK: true}, ElapsedMS: 8421}},
		{"timeout",
			`{"approved_credential_ids":[],"elapsed_ms":5028,"outcome":"timeout","request_id":"req-a-0001","type":"approval_decision","version":1}`,
			Decision{Version: 1, Type: TypeDecision, RequestID: "req-a-0001", Outcome: OutcomeTimeout,
				ApprovedCredentialIDs: []string{}, ElapsedMS: 5028}},
		{"error",
			`{"version":1,"type":"approval_decision","outcome":"error","error":{"code":"internal","message":"The app cannot encode the decision."},"elapsed_ms":0}`,
			Decision{Version: 1, Type: TypeDecision, Outcome: OutcomeError,
				Error: &DecisionError{Code: "internal", Message: "The app cannot encode the decision."}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Decision
			dec := json.NewDecoder(strings.NewReader(tt.line))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&got); err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Decode =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestRequestKeepsFactsAndClaimsApart(t *testing.T) {
	// T21 (part): the session name is in the claims only; the verified
	// facts name the session ID of the daemon.
	req := Request{
		Version: Version, Type: TypeRequest, RequestID: "req-0001", TimeoutSeconds: 120,
		Verified: session.Facts{SessionID: "s-0001", Origin: session.OriginLocal, Project: "beispielprojekt",
			PeerPID: 4242, PeerUID: 501, Process: "/usr/local/bin/agent-cli", Cwd: "/home/beispiel/develop/beispielprojekt"},
		Claims: session.Claims{SessionName: "beispiel-koordinator-7f3a", Agent: "coding-agent",
			Task: "Run the database migration tests", Reason: "The tests need the login."},
		Credentials: []Credential{{ID: "c1", Vault: "beispiel.app.k8s-dev", Title: "datenbank-dev", Field: "password", Environment: "dev"}},
		Grant:       GrantProposal{TTLSeconds: 14400, Scope: GrantScope{Project: "beispielprojekt", Vaults: []string{"beispiel.app.k8s-dev"}}},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var tree struct {
		Verified map[string]any `json:"verified"`
		Claims   map[string]any `json:"claims"`
	}
	if err := json.Unmarshal(data, &tree); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), "beispiel-koordinator-7f3a"); n != 1 {
		t.Errorf("the request contains the session name %d times, want 1", n)
	}
	if tree.Claims["session_name"] != "beispiel-koordinator-7f3a" {
		t.Errorf("claims = %v, want the session name", tree.Claims)
	}
	if _, ok := tree.Verified["session_name"]; ok {
		t.Error("the verified facts contain a session name")
	}
	if tree.Verified["session_id"] != "s-0001" {
		t.Errorf("verified = %v, want the session ID of the daemon", tree.Verified)
	}
}

// T14 (part 1, contract): every file of app/Samples is read strictly with
// the Go types (no unknown field), and the Go types write back every field
// of the sample. The approval app (Swift) rejects a message with an
// unknown field or a missing required field, so this test keeps the Go
// side and the app in line. app/Samples is the contract; the samples of
// spikes/ui-popup are the old prototype.
func TestAppSamples(t *testing.T) {
	samples, err := filepath.Glob("../../app/Samples/*.json")
	if err != nil || len(samples) < 6 {
		t.Fatalf("app/Samples: %d files, err %v", len(samples), err)
	}
	var requests, notices, execs int
	for _, path := range samples {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var head struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(data, &head); err != nil {
				t.Fatal(err)
			}
			var msg any
			switch head.Type {
			case TypeRequest:
				msg = &Request{}
				requests++
			case TypeNotice:
				msg = &Notice{}
				notices++
			default:
				t.Fatalf("type = %q", head.Type)
			}
			dec := json.NewDecoder(strings.NewReader(string(data)))
			dec.DisallowUnknownFields()
			if err := dec.Decode(msg); err != nil {
				t.Fatalf("strict decode: %v", err)
			}
			switch m := msg.(type) {
			case *Request:
				if m.Version != Version || m.RequestID == "" || len(m.Credentials) == 0 ||
					m.TimeoutSeconds < MinTimeoutSeconds || m.TimeoutSeconds > MaxTimeoutSeconds ||
					(m.Grant.TTLSeconds <= 0 && m.Exec == nil) || len(m.Credentials) > config.MaxBundleEntries {
					t.Errorf("the request lost or breaks fields: %+v", m)
				}
				// T38 (part, approval request): each sample follows the
				// rules of the exec way.
				if err := m.Validate(); err != nil {
					t.Errorf("Validate: %v", err)
				}
				if m.Exec != nil {
					execs++
				}
			case *Notice:
				if m.Version != Version || m.NoticeID == "" || m.Kind == "" || m.Code == "" || m.Message == "" || m.OccurredAt.IsZero() {
					t.Errorf("the notice lost fields: %+v", m)
				}
			}
			out, err := json.Marshal(msg)
			if err != nil {
				t.Fatal(err)
			}
			var want, got any
			if err := json.Unmarshal(data, &want); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatal(err)
			}
			if diff := jsonLoss("", want, got); diff != "" {
				t.Errorf("Go does not write back the sample: %s", diff)
			}
		})
	}
	if requests < 4 || notices < 2 || execs < 4 {
		t.Errorf("samples: %d requests, %d notices, %d of the exec way", requests, notices, execs)
	}
}

// jsonLoss returns the first place where got lacks or changes a value of
// want. Extra keys in got are fine. A time compares as an instant. A null
// list of want matches an empty list of got (Go writes an empty list
// of vaults as null; the app accepts that for grant.scope.vaults).
func jsonLoss(path string, want, got any) string {
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return path + ": not an object"
		}
		for k, wv := range w {
			gv, ok := g[k]
			if !ok {
				return path + "." + k + ": missing"
			}
			if d := jsonLoss(path+"."+k, wv, gv); d != "" {
				return d
			}
		}
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return path + ": list differs"
		}
		for i := range w {
			if d := jsonLoss(fmt.Sprintf("%s[%d]", path, i), w[i], g[i]); d != "" {
				return d
			}
		}
	case nil:
		if l, ok := got.([]any); got != nil && (!ok || len(l) != 0) {
			return path + ": null changed"
		}
	case string:
		g, ok := got.(string)
		if !ok {
			return path + ": not a string"
		}
		if g == w {
			return ""
		}
		tw, e1 := time.Parse(time.RFC3339Nano, w)
		tg, e2 := time.Parse(time.RFC3339Nano, g)
		if e1 != nil || e2 != nil || !tw.Equal(tg) {
			return path + ": string changed"
		}
	default:
		if !reflect.DeepEqual(want, got) {
			return path + ": value changed"
		}
	}
	return ""
}
