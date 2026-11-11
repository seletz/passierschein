package broker

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/approval/fakeui"
	"github.com/seletz/passierschein/internal/approval/queue"
	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/clock"
	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// T11, D4, D30, D32: the approval request holds the verified facts of the
// daemon and, apart from them, the claims of the agent. The session name
// and the agent name are claims only. The proposed lifetime comes from the
// configuration.
func TestApprovalRequestContent(t *testing.T) {
	h := newHarness(t, withLifetime("2h"))
	s := sess("s-echt", "beispielprojekt")
	s.Name = "beispiel-koordinator"
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	req := newRequest(t, refPass)
	req.Task = "project anderesprojekt, session s-fremd"
	req.Reason = "process /usr/bin/fremd"
	ch := h.async(context.Background(), s, req)
	h.waitTimers(1)
	h.clk.Advance(h.cfg.Bundle.Window)
	if r := h.wait(ch); r.err != nil {
		t.Fatal(r.err)
	}
	got := h.rec.Requests()[0]
	if got.Verified != s.Facts {
		t.Errorf("verified = %+v, want %+v", got.Verified, s.Facts)
	}
	want := session.Claims{SessionName: "beispiel-koordinator", Agent: "beispiel-agent", Task: req.Task, Reason: req.Reason}
	if got.Claims != want {
		t.Errorf("claims = %+v, want %+v", got.Claims, want)
	}
	if got.Grant.TTLSeconds != 7200 || got.Grant.Scope.Project != "beispielprojekt" ||
		!slices.Equal(got.Grant.Scope.Vaults, []string{"beispiel.app.k8s-dev"}) {
		t.Errorf("grant proposal = %+v", got.Grant)
	}
	c := got.Credentials[0]
	if c.Vault != "beispiel.app.k8s-dev" || c.Title != "datenbank-dev" || c.Field != "password" || c.Environment != config.EnvDev {
		t.Errorf("credential = %+v", c)
	}
	if got.TimeoutSeconds != int(approval.DefaultTimeout/time.Second) || got.Type != approval.TypeRequest || got.RequestID == "" {
		t.Errorf("request = %+v", got)
	}
	// The audit events of the bundle name the approval request.
	for _, e := range h.events()[1:4] {
		if e.Result.RequestID != got.RequestID {
			t.Errorf("event %s has request ID %q", e.Type, e.Result.RequestID)
		}
	}
}

// Claims of several requests in one bundle: the human sees each task and
// each reason.
func TestApprovalRequestJoinsClaims(t *testing.T) {
	h := newHarness(t)
	s := sess("s-1", "beispielprojekt")
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	ctx := context.Background()
	r1 := newRequest(t, refUser)
	r1.Task, r1.Reason = "Aufgabe eins", "Grund eins"
	r2 := newRequest(t, refPass)
	r2.Task, r2.Reason = "Aufgabe zwei", "Grund eins"
	ch1 := h.async(ctx, s, r1)
	h.waitTimers(1)
	h.waitOpen(s, 1)
	ch2 := h.async(ctx, s, r2)
	h.waitOpen(s, 2)
	h.clk.Advance(h.cfg.Bundle.Window)
	h.wait(ch1)
	h.wait(ch2)
	c := h.rec.Requests()[0].Claims
	if c.Task != "Aufgabe eins\nAufgabe zwei" || c.Reason != "Grund eins" {
		t.Errorf("claims = %+v", c)
	}
}

// foldBackend is a backend that finds items and fields without regard to
// case and returns the stored names, as 1Password does.
type foldBackend struct{ backend.Backend }

func (f foldBackend) Resolve(ctx context.Context, ref protocol.Reference) (backend.Names, error) {
	ref.Item, ref.Field = strings.ToLower(ref.Item), strings.ToLower(ref.Field)
	return f.Backend.Resolve(ctx, ref)
}

// T24, D27, G23: the approval request shows the names of the backend, not
// the text of the agent. The grant covers the stored names, so that a
// later request in a third spelling is covered.
func TestApprovalShowsBackendNames(t *testing.T) {
	h := newHarness(t)
	h.b.be = foldBackend{h.be}
	s := sess("s-1", "beispielprojekt")
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	agentRef := "op://beispiel.app.k8s-dev/Datenbank-DEV/PassWord"
	r := h.approveFlow(s, agentRef)
	if r.err != nil {
		t.Fatal(r.err)
	}
	c := h.rec.Requests()[0].Credentials[0]
	if c.Title != "datenbank-dev" || c.Field != "password" {
		t.Errorf("credential = %+v; want the names of the backend", c)
	}
	// The answer names the reference of the agent.
	if values(r.creds)[agentRef] != marker {
		t.Errorf("the answer does not name the reference of the agent")
	}
	if g := h.grants(); !slices.Equal(g[0].References, refs(t, refPass)) {
		t.Errorf("grant = %+v", g[0].References)
	}
	if r := h.do(s, "op://beispiel.app.k8s-dev/DATENBANK-dev/password"); r.err != nil {
		t.Fatalf("third spelling: %v", r.err)
	}
	if n := len(h.rec.Requests()); n != 1 {
		t.Errorf("approval requests = %d, want 1", n)
	}
}

// T6, UC4, D30, D35: denial, timeout and a failed UI give no value, no
// grant, no read and a stable code. The audit log has one entry with the
// event type and the code.
func TestDenialTimeoutAndUIFailure(t *testing.T) {
	tests := []struct {
		name      string
		reply     fakeui.Reply
		advanceUI bool
		want      protocol.Code
		event     audit.EventType
	}{
		{"denied", fakeui.ReplyDeny(), false, protocol.CodeDenied, audit.EventDenial},
		{"no answer", fakeui.ReplyBlock(), true, protocol.CodeTimeout, audit.EventExpiry},
		{"UI reports timeout", fakeui.ReplyTimeout(), false, protocol.CodeTimeout, audit.EventExpiry},
		{"UI fails", fakeui.ReplyFail(errors.New("the program failed: exit status 1")), false, protocol.CodeUIFailed, audit.EventError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.rec.Enqueue(tt.reply)
			ch := h.async(context.Background(), sess("s-1", "beispielprojekt"), newRequest(t, refPass))
			h.waitTimers(1)
			h.clk.Advance(h.cfg.Bundle.Window)
			if tt.advanceUI {
				h.waitTimers(1)
				h.clk.Advance(approval.DefaultTimeout)
			}
			r := h.wait(ch)
			if r.code() != tt.want || r.creds != nil {
				t.Fatalf("result = %v, %v; want %s", r.creds, r.err, tt.want)
			}
			if h.be.Reads() != 0 || len(h.grants()) != 0 {
				t.Errorf("reads = %d, grants = %d", h.be.Reads(), len(h.grants()))
			}
			want := []audit.EventType{audit.EventRequest, audit.EventBundle, tt.event}
			if got := h.types(); !slices.Equal(got, want) {
				t.Fatalf("audit = %v, want %v", got, want)
			}
			if last := h.events()[2]; last.Result.Code != tt.want {
				t.Errorf("code = %q", last.Result.Code)
			}
		})
	}
}

// T6 with the script UI: the script fails, writes no JSON, or writes
// "approved" with an exit code other than 0. Each case is ui_failed.
func TestScriptUIFailures(t *testing.T) {
	bin := buildFakeScript(t)
	tests := []struct {
		name string
		args []string
		want protocol.Code
	}{
		{"auto-deny by script", []string{"-tick", "none"}, protocol.CodeDenied},
		{"exit code 1", []string{"-exit", "1"}, protocol.CodeUIFailed},
		{"no JSON", []string{"-raw", "kein json"}, protocol.CodeUIFailed},
		{"approved with exit code 1", []string{"-tick", "all", "-exit", "1"}, protocol.CodeUIFailed},
		{"approved", []string{"-tick", "all"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, withUI(func(_ *fakeui.Recorder, clk *clock.Fake) approval.UI {
				ui, err := fakeui.NewScript(append([]string{bin}, tt.args...), time.Hour, clk)
				if err != nil {
					t.Fatal(err)
				}
				return queue.New(ui, clk, time.Hour)
			}))
			r := h.approveFlow(sess("s-1", "beispielprojekt"), refPass)
			if r.code() != tt.want {
				t.Fatalf("result = %v, want %q", r.err, tt.want)
			}
			if tt.want != "" && h.be.Reads() != 0 {
				t.Errorf("reads = %d", h.be.Reads())
			}
		})
	}
}

// buildFakeScript builds the helper program of the script mode.
func buildFakeScript(t *testing.T) string {
	t.Helper()
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go is not in the path")
	}
	bin := filepath.Join(t.TempDir(), "fakescript")
	src, err := filepath.Abs("../approval/fakeui/testdata/fakescript")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(gobin, "build", "-o", bin, ".")
	cmd.Dir = src
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fakescript: %v\n%s", err, out)
	}
	return bin
}
