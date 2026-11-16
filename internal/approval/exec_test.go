package approval

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// execRequest returns a valid approval request of the exec way for env.
func execRequest(env config.Environment) Request {
	vault := "beispiel.app.k8s-" + string(env)
	req := Request{
		Version: Version, Type: TypeRequest, RequestID: "req-exec-1", TimeoutSeconds: 120,
		Verified: session.Facts{SessionID: "s-1", Origin: session.OriginLocal, Project: "beispielprojekt", PeerUID: 501},
		Claims:   session.Claims{Task: "Count the orders", Reason: "Check migration 42"},
		Credentials: []Credential{
			{ID: "c1", Vault: vault, Title: "datenbank-" + string(env), Field: "password", Environment: env},
		},
		Grant: GrantProposal{TTLSeconds: 1800, Scope: GrantScope{Project: "beispielprojekt", Vaults: []string{vault}}},
		Exec: &ExecCommand{
			Environment: env,
			Program:     "/opt/homebrew/bin/psql",
			Args:        []string{"-h", "datenbank.beispiel.internal", "-c", "select count(*) from auftrag"},
			Cwd:         "/Users/beispiel/develop/beispielprojekt",
			Env:         []ExecVariable{{Name: "PGPASSWORD", CredentialID: "c1"}},
			Approval: ExecApproval{
				Kind: ExecApprovalGrant, Binding: BindingProgram,
				LifetimeChoicesSeconds: []int64{1800, 900, 300},
			},
		},
	}
	switch env {
	case config.EnvDev:
		req.Grant.TTLSeconds = 14400
		req.Exec.Approval = ExecApproval{Kind: ExecApprovalGrant, Binding: BindingNone,
			LifetimeChoicesSeconds: []int64{14400, 7200, 900, 300}}
	case config.EnvProd:
		req.Grant.TTLSeconds = 0
		req.Exec.Approval = ExecApproval{Kind: ExecApprovalOnce, Binding: BindingCommand}
	}
	return req
}

// T38 (part, approval request): the exec part of the request has the
// fields of D65 under fixed JSON names. Issue 99 relies on them.
func TestExecRequestWireFields(t *testing.T) {
	data, err := json.Marshal(execRequest(config.EnvTest))
	if err != nil {
		t.Fatal(err)
	}
	var tree map[string]any
	if err := json.Unmarshal(data, &tree); err != nil {
		t.Fatal(err)
	}
	ex, ok := tree["exec"].(map[string]any)
	if !ok {
		t.Fatalf("no object exec in %s", data)
	}
	keys := func(m map[string]any) []string {
		var out []string
		for k := range m {
			out = append(out, k)
		}
		return sortStrings(out)
	}
	if got, want := keys(ex), []string{"approval", "args", "cwd", "env", "environment", "program"}; !reflect.DeepEqual(got, want) {
		t.Errorf("exec keys = %v, want %v", got, want)
	}
	env := ex["env"].([]any)[0].(map[string]any)
	if got, want := keys(env), []string{"credential_id", "name"}; !reflect.DeepEqual(got, want) {
		t.Errorf("exec.env keys = %v, want %v", got, want)
	}
	ap := ex["approval"].(map[string]any)
	if got, want := keys(ap), []string{"binding", "kind", "lifetime_choices_seconds"}; !reflect.DeepEqual(got, want) {
		t.Errorf("exec.approval keys = %v, want %v", got, want)
	}

	// A plain request has no key exec: the window of a plain request and
	// its JSON stay unchanged.
	plain := execRequest(config.EnvDev)
	plain.Exec = nil
	data, err = json.Marshal(plain)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"exec"`) {
		t.Errorf("a plain request writes exec: %s", data)
	}

	// prod: "only this command" has no lifetime choices on the wire.
	data, err = json.Marshal(execRequest(config.EnvProd))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "lifetime_choices_seconds") {
		t.Errorf("a request for prod offers lifetimes: %s", data)
	}
}

func sortStrings(s []string) []string {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
	return s
}

// T38 (part, approval request): the daemon sends only a request of the
// exec way that the approval app can show in full (D65, D63, D62).
func TestRequestValidateExec(t *testing.T) {
	long := strings.Repeat("x", protocol.MaxExecArgsSize/2)
	tests := []struct {
		name    string
		env     config.Environment
		mutate  func(*Request)
		wantErr string
	}{
		{name: "dev grant", env: config.EnvDev, mutate: func(*Request) {}},
		{name: "test grant", env: config.EnvTest, mutate: func(*Request) {}},
		{name: "prod once", env: config.EnvProd, mutate: func(*Request) {}},
		{name: "plain request", env: config.EnvDev, mutate: func(r *Request) { r.Exec = nil }},
		{name: "no args", env: config.EnvTest, mutate: func(r *Request) { r.Exec.Args = nil }},
		{name: "arg with line break", env: config.EnvTest, mutate: func(r *Request) { r.Exec.Args = []string{"-c", "select 1;\nselect 2;"} }},
		{name: "two names for one credential", env: config.EnvTest, mutate: func(r *Request) {
			r.Exec.Env = append(r.Exec.Env, ExecVariable{Name: "PGPASS_COPY", CredentialID: "c1"})
		}},

		{name: "unknown environment", env: config.EnvTest, mutate: func(r *Request) { r.Exec.Environment = "stage" }, wantErr: "environment"},
		{name: "environment not the strictest", env: config.EnvTest, mutate: func(r *Request) { r.Exec.Environment = config.EnvDev }, wantErr: "strictest"},
		{name: "mixed request takes prod", env: config.EnvProd, mutate: func(r *Request) {
			r.Credentials = append(r.Credentials, Credential{ID: "c2", Vault: "beispiel.app.k8s-dev", Title: "datenbank-dev", Field: "password", Environment: config.EnvDev})
			r.Exec.Env = append(r.Exec.Env, ExecVariable{Name: "DEV_PASSWORD", CredentialID: "c2"})
		}},
		{name: "relative program", env: config.EnvTest, mutate: func(r *Request) { r.Exec.Program = "psql" }, wantErr: "program"},
		{name: "program not clean", env: config.EnvTest, mutate: func(r *Request) { r.Exec.Program = "/opt/../bin/psql" }, wantErr: "program"},
		{name: "relative cwd", env: config.EnvTest, mutate: func(r *Request) { r.Exec.Cwd = "beispielprojekt" }, wantErr: "cwd"},
		{name: "control character in arg", env: config.EnvTest, mutate: func(r *Request) { r.Exec.Args = []string{"a\x1bb"} }, wantErr: "args"},
		{name: "args too long", env: config.EnvTest, mutate: func(r *Request) { r.Exec.Args = []string{long, long, "x"} }, wantErr: "args"},
		{name: "no variable", env: config.EnvTest, mutate: func(r *Request) { r.Exec.Env = nil }, wantErr: "env"},
		{name: "forbidden name", env: config.EnvTest, mutate: func(r *Request) { r.Exec.Env[0].Name = "PATH" }, wantErr: "environment variable"},
		{name: "name twice", env: config.EnvTest, mutate: func(r *Request) {
			r.Exec.Env = append(r.Exec.Env, ExecVariable{Name: "PGPASSWORD", CredentialID: "c1"})
		}, wantErr: "twice"},
		{name: "unknown credential", env: config.EnvTest, mutate: func(r *Request) { r.Exec.Env[0].CredentialID = "c9" }, wantErr: "credential"},
		{name: "credential without variable", env: config.EnvTest, mutate: func(r *Request) {
			r.Credentials = append(r.Credentials, Credential{ID: "c2", Vault: "beispiel.app.k8s-test", Title: "cache-test", Field: "password", Environment: config.EnvTest})
		}, wantErr: "no variable"},
		{name: "unknown kind", env: config.EnvTest, mutate: func(r *Request) { r.Exec.Approval.Kind = "always" }, wantErr: "kind"},
		{name: "prod with grant", env: config.EnvProd, mutate: func(r *Request) {
			r.Grant.TTLSeconds = 300
			r.Exec.Approval = ExecApproval{Kind: ExecApprovalGrant, Binding: BindingCommand, LifetimeChoicesSeconds: []int64{300}}
		}, wantErr: "prod"},
		{name: "test with once", env: config.EnvTest, mutate: func(r *Request) {
			r.Grant.TTLSeconds = 0
			r.Exec.Approval = ExecApproval{Kind: ExecApprovalOnce, Binding: BindingCommand}
		}, wantErr: "once"},
		{name: "once with lifetime", env: config.EnvProd, mutate: func(r *Request) { r.Grant.TTLSeconds = 60 }, wantErr: "lifetime"},
		{name: "once with choices", env: config.EnvProd, mutate: func(r *Request) { r.Exec.Approval.LifetimeChoicesSeconds = []int64{60} }, wantErr: "lifetime"},
		{name: "once with binding program", env: config.EnvProd, mutate: func(r *Request) { r.Exec.Approval.Binding = BindingProgram }, wantErr: "binding"},
		{name: "unknown binding", env: config.EnvTest, mutate: func(r *Request) { r.Exec.Approval.Binding = "argument" }, wantErr: "binding"},
		{name: "test with binding none", env: config.EnvTest, mutate: func(r *Request) { r.Exec.Approval.Binding = BindingNone }, wantErr: "binding"},
		{name: "grant without lifetime", env: config.EnvTest, mutate: func(r *Request) { r.Grant.TTLSeconds = 0 }, wantErr: "lifetime"},
		{name: "grant longer than 24h", env: config.EnvDev, mutate: func(r *Request) {
			r.Grant.TTLSeconds = 25 * 3600
			r.Exec.Approval.LifetimeChoicesSeconds = []int64{25 * 3600}
		}, wantErr: "lifetime"},
		{name: "no choices", env: config.EnvTest, mutate: func(r *Request) { r.Exec.Approval.LifetimeChoicesSeconds = nil }, wantErr: "lifetime"},
		{name: "first choice is not the lifetime", env: config.EnvTest, mutate: func(r *Request) {
			r.Exec.Approval.LifetimeChoicesSeconds = []int64{900, 300}
		}, wantErr: "lifetime"},
		{name: "choice longer than the lifetime", env: config.EnvTest, mutate: func(r *Request) {
			r.Exec.Approval.LifetimeChoicesSeconds = []int64{1800, 3600}
		}, wantErr: "lifetime"},
		{name: "choice twice", env: config.EnvTest, mutate: func(r *Request) {
			r.Exec.Approval.LifetimeChoicesSeconds = []int64{1800, 900, 900}
		}, wantErr: "lifetime"},
		{name: "choice zero", env: config.EnvTest, mutate: func(r *Request) {
			r.Exec.Approval.LifetimeChoicesSeconds = []int64{1800, 0}
		}, wantErr: "lifetime"},
		{name: "plain request outside dev", env: config.EnvTest, mutate: func(r *Request) { r.Exec = nil }, wantErr: "exec"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := execRequest(tt.env)
			tt.mutate(&req)
			err := req.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() = %v, want an error with %q", err, tt.wantErr)
			}
		})
	}
}

// T38 (part, approval request): an error text never repeats an argument,
// a path or a name of the request.
func TestRequestValidateExecHidesContent(t *testing.T) {
	req := execRequest(config.EnvTest)
	req.Exec.Args = []string{"geheim-beispiel\x1b"}
	req.Exec.Program = "geheim-programm"
	err := req.Validate()
	if err == nil {
		t.Fatal("Validate() = nil")
	}
	if strings.Contains(err.Error(), "geheim") {
		t.Errorf("the error repeats content: %v", err)
	}
}

// T38 (part, approval request): the offered lifetimes are never longer
// than the configured lifetime (D63) and start with it.
func TestExecLifetimeChoices(t *testing.T) {
	tests := []struct {
		ttl  int64
		want []int64
	}{
		{1800, []int64{1800, 900, 300}},
		{1200, []int64{1200, 900, 600, 300}},
		{3600, []int64{3600, 1800, 900, 300}},
		{14400, []int64{14400, 7200, 900, 300}},
		{600, []int64{600, 300}},
		{300, []int64{300}},
		{90, []int64{90}},
		{0, nil},
	}
	for _, tt := range tests {
		got := ExecLifetimeChoices(tt.ttl)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ExecLifetimeChoices(%d) = %v, want %v", tt.ttl, got, tt.want)
		}
	}
}

// T38 (part, approval request), T4 (part): the daemon does not trust the
// decision for a command. A grant needs every variable and one of the
// offered lifetimes; "only this command" has no lifetime.
func TestValidateDecisionExec(t *testing.T) {
	approved := func(ids []string, ttl int64) Decision {
		return Decision{Version: Version, Type: TypeDecision, RequestID: "req-exec-1",
			Outcome: OutcomeApproved, ApprovedCredentialIDs: ids, TTLSeconds: ttl}
	}
	two := func(env config.Environment) Request {
		r := execRequest(env)
		r.Credentials = append(r.Credentials, Credential{ID: "c2", Vault: r.Credentials[0].Vault,
			Title: "cache", Field: "password", Environment: env})
		r.Exec.Env = append(r.Exec.Env, ExecVariable{Name: "CACHE_PASSWORD", CredentialID: "c2"})
		return r
	}
	tests := []struct {
		name    string
		req     Request
		d       Decision
		wantErr bool
		wantTTL int64
	}{
		{name: "test proposed lifetime", req: execRequest(config.EnvTest), d: approved([]string{"c1"}, 1800), wantTTL: 1800},
		{name: "test shorter choice", req: execRequest(config.EnvTest), d: approved([]string{"c1"}, 300), wantTTL: 300},
		{name: "test lifetime not offered", req: execRequest(config.EnvTest), d: approved([]string{"c1"}, 600), wantErr: true},
		{name: "test longer lifetime", req: execRequest(config.EnvTest), d: approved([]string{"c1"}, 3600), wantErr: true},
		{name: "test zero lifetime", req: execRequest(config.EnvTest), d: approved([]string{"c1"}, 0), wantErr: true},
		{name: "prod once", req: execRequest(config.EnvProd), d: approved([]string{"c1"}, 0), wantTTL: 0},
		{name: "prod with lifetime", req: execRequest(config.EnvProd), d: approved([]string{"c1"}, 60), wantErr: true},
		{name: "all variables", req: two(config.EnvTest), d: approved([]string{"c2", "c1"}, 1800), wantTTL: 1800},
		{name: "part of the variables", req: two(config.EnvTest), d: approved([]string{"c1"}, 1800), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateDecision(tt.req, tt.d)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, want error %v", err, tt.wantErr)
			}
			if err == nil && (got.Outcome != OutcomeApproved || got.TTLSeconds != tt.wantTTL) {
				t.Errorf("decision = %+v", got)
			}
		})
	}
}
