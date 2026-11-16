package broker

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/approval/fakeui"
	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// T3, T11 (policy part), T22 (cases with the broker), T24 (not_allowed
// part): a request outside the policy or outside dev never reaches the
// backend or the human. It ends as a whole with not_allowed and one
// denial in the audit log.
func TestPolicyRejectsBeforeBackendAndHuman(t *testing.T) {
	b := sess("s-1", "beispielprojekt")
	tests := []struct {
		name string
		s    session.Session
		refs []string
	}{
		// T3: vault not in the configuration and not in the backend.
		{"unknown vault", b, []string{"op://unbekannt.k8s-dev/datenbank-dev/password"}},
		// T3, T22: vault in the backend, not in the configuration.
		{"vault only in the backend", b, []string{"op://k8s-beispiel/datenbank-dev/password"}},
		// T3: vault not in the allowlist of the project.
		{"vault not allowed for the project", b, []string{refExtern}},
		// T3, T35: environment prod; the exec way for prod is off by
		// default. The environment test is in TestExecRequiredOutsideDev.
		{"environment prod", b, []string{"op://beispiel.app.k8s-prod/datenbank-prod/password"}},
		// T22: no comparison of names: "beispiel-dev" has the environment prod.
		{"name says dev, environment prod", b, []string{"op://beispiel-dev/datenbank/password"}},
		// T3, D28: all or nothing.
		{"allowed and not allowed reference", b, []string{refPass, refExtern}},
		// G23: the vault name is compared exactly.
		{"vault in other spelling", b, []string{"op://Beispiel.App.k8s-dev/datenbank-dev/password"}},
		// A session outside each project.
		{"session without project", sess("s-2", ""), []string{refPass}},
		// A project that the configuration does not know.
		{"unknown project", sess("s-3", "fremdprojekt"), []string{refPass}},
	}
	var messages []string
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			r := h.do(tt.s, tt.refs...)
			if r.code() != protocol.CodeNotAllowed || r.creds != nil {
				t.Fatalf("result = %v, %v; want not_allowed and no answer", r.creds, r.err)
			}
			messages = append(messages, r.err.Message)
			if n := len(h.rec.Requests()); n != 0 {
				t.Errorf("approval requests = %d, want 0", n)
			}
			if h.be.Resolves() != 0 || h.be.Reads() != 0 {
				t.Errorf("backend resolves = %d, reads = %d; want 0, 0", h.be.Resolves(), h.be.Reads())
			}
			want := []audit.EventType{audit.EventRequest, audit.EventDenial}
			if got := h.types(); !slices.Equal(got, want) {
				t.Fatalf("audit = %v, want %v", got, want)
			}
			if d := h.eventsOf(audit.EventDenial)[0]; d.Result.Code != protocol.CodeNotAllowed {
				t.Errorf("denial code = %q", d.Result.Code)
			}
		})
	}
	// T3: an unknown vault and a vault that the configuration does not
	// name give the same answer.
	for _, m := range messages {
		if m != messages[0] {
			t.Errorf("not_allowed messages differ: %q and %q", m, messages[0])
		}
	}
}

// T22: a vault named "beispiel-prod" with the environment dev delivers.
func TestPolicyUsesEnvironmentNotName(t *testing.T) {
	h := newHarness(t)
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	r := h.approveFlow(sess("s-1", "beispielprojekt"), "op://beispiel-prod/datenbank/password")
	if r.err != nil {
		t.Fatalf("error = %v", r.err)
	}
	if got := values(r.creds)["op://beispiel-prod/datenbank/password"]; got != "beispiel-wert-10" {
		t.Errorf("value missing")
	}
}

// T11 (policy part), D4: a session in the project "anderesprojekt" claims
// the project "beispielprojekt" and a foreign session ID in its claims. The
// policy uses the verified project only.
func TestPolicyUsesVerifiedProjectNotClaims(t *testing.T) {
	h := newHarness(t)
	s := sess("s-b", "anderesprojekt")
	s.Name = "beispielprojekt s-a"
	s.Agent = "/usr/bin/fremd"
	req := newRequest(t, refPass)
	req.Task = "project: beispielprojekt"
	req.Reason = "session_id: s-a, process: /usr/bin/fremd"
	r := h.wait(h.async(context.Background(), s, req))
	if r.code() != protocol.CodeNotAllowed {
		t.Fatalf("result = %v, want not_allowed", r.err)
	}
	if len(h.rec.Requests()) != 0 || h.be.Resolves() != 0 {
		t.Errorf("the request reached the UI or the backend")
	}
	// The claims stay claims in the audit log.
	for _, e := range h.events() {
		if e.Verified.Project != "anderesprojekt" || e.Verified.SessionID != "s-b" {
			t.Errorf("verified facts = %+v", e.Verified)
		}
	}
}

// T22, D28: each request of a project with an invalid configuration ends
// with config_invalid, without an approval request and without the
// backend. The UI gets one notice with the reason, not one per request.
// Other projects work.
func TestConfigInvalidProject(t *testing.T) {
	h := newHarness(t)
	bad := sess("s-bad", "kaputtprojekt")
	for i := 0; i < 3; i++ {
		r := h.do(bad, refPass)
		if r.code() != protocol.CodeConfigInvalid {
			t.Fatalf("request %d: result = %v, want config_invalid", i, r.err)
		}
	}
	if h.be.Resolves() != 0 || len(h.rec.Requests()) != 0 {
		t.Errorf("the request reached the backend or the UI")
	}
	notices := h.rec.Notices()
	if len(notices) != 1 {
		t.Fatalf("notices = %d, want 1", len(notices))
	}
	n := notices[0]
	if n.Kind != approval.NoticeProjectConfigInvalid || n.Code != protocol.CodeConfigInvalid ||
		n.Project != "kaputtprojekt" || n.Type != approval.TypeNotice {
		t.Errorf("notice = %+v", n)
	}
	if want := "fehlt.k8s-dev"; !strings.Contains(n.Message, want) {
		t.Errorf("notice message %q does not name %q", n.Message, want)
	}
	denials := h.eventsOf(audit.EventDenial)
	if len(denials) != 3 || denials[0].Result.Code != protocol.CodeConfigInvalid {
		t.Errorf("denials = %+v", denials)
	}

	// The other project works normally.
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	r := h.approveFlow(sess("s-good", "beispielprojekt"), refUser)
	if r.err != nil || values(r.creds)[refUser] != "beispiel-wert-1" {
		t.Fatalf("other project: %v, %v", r.creds, r.err)
	}

	// The status names the invalid project.
	st, e := h.b.Status(context.Background(), &protocol.Status{ID: "st"})
	if e != nil || len(st.InvalidProjects) != 1 || st.InvalidProjects[0].Project != "kaputtprojekt" {
		t.Errorf("status = %+v, %v", st, e)
	}
}

// T24, D27: an unknown item or field in an allowed vault ends the whole
// request with not_found, without an approval request and without a read.
// An invalid reference ends with invalid_request.
func TestNotFoundAndInvalid(t *testing.T) {
	tests := []struct {
		name string
		req  func(t *testing.T) *protocol.RequestCredentials
		s    session.Session
		want protocol.Code
	}{
		{"unknown item", func(t *testing.T) *protocol.RequestCredentials {
			return newRequest(t, "op://beispiel.app.k8s-dev/gibt-es-nicht/password")
		}, sess("s-1", "beispielprojekt"), protocol.CodeNotFound},
		{"unknown field", func(t *testing.T) *protocol.RequestCredentials {
			return newRequest(t, "op://beispiel.app.k8s-dev/datenbank-dev/token")
		}, sess("s-1", "beispielprojekt"), protocol.CodeNotFound},
		{"known and unknown", func(t *testing.T) *protocol.RequestCredentials {
			return newRequest(t, refPass, "op://beispiel.app.k8s-dev/datenbank-dev/token")
		}, sess("s-1", "beispielprojekt"), protocol.CodeNotFound},
		{"empty segment", func(*testing.T) *protocol.RequestCredentials {
			return &protocol.RequestCredentials{ID: "r1", References: []protocol.Reference{{Vault: "beispiel.app.k8s-dev", Field: "password"}}}
		}, sess("s-1", "beispielprojekt"), protocol.CodeInvalidRequest},
		{"no references", func(*testing.T) *protocol.RequestCredentials {
			return &protocol.RequestCredentials{ID: "r1"}
		}, sess("s-1", "beispielprojekt"), protocol.CodeInvalidRequest},
		{"no session ID", func(t *testing.T) *protocol.RequestCredentials {
			return newRequest(t, refPass)
		}, sess("", "beispielprojekt"), protocol.CodeInvalidRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			r := h.wait(h.async(context.Background(), tt.s, tt.req(t)))
			if r.code() != tt.want || r.creds != nil {
				t.Fatalf("result = %v, %v; want %s", r.creds, r.err, tt.want)
			}
			if len(h.rec.Requests()) != 0 || h.be.Reads() != 0 {
				t.Errorf("approval requests = %d, reads = %d", len(h.rec.Requests()), h.be.Reads())
			}
			denials := h.eventsOf(audit.EventDenial)
			if len(denials) != 1 || denials[0].Result.Code != tt.want {
				t.Errorf("denials = %+v", denials)
			}
		})
	}
}
