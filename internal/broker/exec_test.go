package broker

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/approval/fakeui"
	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/clock"
	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// References and values of test and prod in the fake backend.
const (
	refTest   = "op://beispiel.app.k8s-test/datenbank-test/password"
	refProd   = "op://beispiel.app.k8s-prod/datenbank-prod/password"
	valueTest = "beispiel-wert-8"
	valueProd = "beispiel-wert-9"
	// projectDir is the path of the project beispielprojekt in the tests.
	projectDir = "/tmp/a38-test/beispielprojekt"
)

// prodOn switches the exec way for prod on.
const prodOn = `
environments:
  prod:
    delivery: [exec]
`

// execResult is the answer of RequestExec.
type execResult struct {
	creds *protocol.ExecCredentials
	err   *protocol.Error
}

func (r execResult) code() protocol.Code {
	if r.err == nil {
		return ""
	}
	return r.err.Code
}

// value returns the revealed value of the variable, or "".
func (r execResult) value(name string) string {
	if r.creds == nil {
		return ""
	}
	for _, v := range r.creds.Values {
		if v.Name == name {
			return v.Value.Reveal()
		}
	}
	return ""
}

// execReq returns a request_exec for the program in the project directory.
// env alternates the name and the reference of each variable. The request
// has no ID; harness.exec gives it the next ID x1, x2 ... of the harness.
func execReq(t *testing.T, program string, args []string, env ...string) *protocol.RequestExec {
	t.Helper()
	req := &protocol.RequestExec{
		Program: program, Args: args, Cwd: projectDir,
		Task: "Check the number of orders", Reason: "Verify migration 42",
	}
	for i := 0; i+1 < len(env); i += 2 {
		req.Env = append(req.Env, protocol.ExecVar{Name: env[i], Reference: ref(t, env[i+1])})
	}
	return req
}

// exec runs one request_exec. It does not wait for a bundle window.
func (h *harness) exec(s session.Session, req *protocol.RequestExec) execResult {
	h.t.Helper()
	if req.ID == "" {
		h.execIDs++
		req.ID = "x" + strconv.Itoa(h.execIDs)
	}
	ch := make(chan execResult, 1)
	go func() {
		c, e := h.b.RequestExec(context.Background(), s, req)
		ch <- execResult{c, e}
	}()
	select {
	case r := <-ch:
		return r
	case <-time.After(10 * time.Second):
		h.t.Fatal("the request_exec did not end")
		return execResult{}
	}
}

// approvals returns the number of approval requests.
func (h *harness) approvals() int { return len(h.rec.Requests()) }

// lastApproval returns the last approval request.
func (h *harness) lastApproval() approval.Request {
	h.t.Helper()
	reqs := h.rec.Requests()
	if len(reqs) == 0 {
		h.t.Fatal("no approval request")
	}
	return reqs[len(reqs)-1]
}

// T35 steps 1 and 2: request_credentials for a vault outside dev ends with
// exec_required. The human sees no approval request, and the backend gets
// no call. A mixed request ends as a whole.
func TestExecRequiredOutsideDev(t *testing.T) {
	tests := []struct {
		name   string
		config string
		refs   []string
		want   protocol.Code
	}{
		{"test", "", []string{refTest}, protocol.CodeExecRequired},
		{"dev and test", "", []string{refPass, refTest}, protocol.CodeExecRequired},
		{"prod switched off", "", []string{refProd}, protocol.CodeNotAllowed},
		{"prod switched on", prodOn, []string{refProd}, protocol.CodeExecRequired},
		{"test switched off", "environments:\n  test:\n    delivery: []\n", []string{refTest}, protocol.CodeNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, withConfig(tt.config))
			r := h.do(sess("s-1", "beispielprojekt"), tt.refs...)
			if r.code() != tt.want || r.creds != nil {
				t.Fatalf("result = %v, %v; want %s", r.creds, r.err, tt.want)
			}
			if h.approvals() != 0 || h.be.Resolves() != 0 || h.be.Reads() != 0 {
				t.Errorf("approvals %d, resolves %d, reads %d; want 0", h.approvals(), h.be.Resolves(), h.be.Reads())
			}
			want := []audit.EventType{audit.EventRequest, audit.EventDenial}
			if got := h.types(); !slices.Equal(got, want) {
				t.Fatalf("audit = %v, want %v", got, want)
			}
			if d := h.eventsOf(audit.EventDenial)[0]; d.Result.Code != tt.want {
				t.Errorf("denial code = %q", d.Result.Code)
			}
		})
	}
}

// T35 steps 3 and 4: the exec way for prod is off by default. Switched on,
// it shows an approval request and delivers.
func TestExecProd(t *testing.T) {
	s := sess("s-1", "beispielprojekt")
	req := func() *protocol.RequestExec {
		return execReq(t, "/usr/bin/true", nil, "DB_PASSWORD", refProd)
	}
	h := newHarness(t)
	if r := h.exec(s, req()); r.code() != protocol.CodeNotAllowed || r.creds != nil {
		t.Fatalf("prod off: %v, %v; want not_allowed", r.creds, r.err)
	}
	if h.approvals() != 0 || h.be.Resolves() != 0 {
		t.Errorf("prod off: approvals %d, resolves %d", h.approvals(), h.be.Resolves())
	}

	h = newHarness(t, withConfig(prodOn))
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	r := h.exec(s, req())
	if r.err != nil || r.value("DB_PASSWORD") != valueProd {
		t.Fatalf("prod on: %v; want the value", r.err)
	}
	a := h.lastApproval()
	if a.Exec == nil || a.Exec.Environment != config.EnvProd || a.Exec.Approval.Kind != approval.ExecApprovalOnce ||
		a.Exec.Approval.Binding != approval.BindingCommand || a.Grant.TTLSeconds != 0 {
		t.Errorf("approval request = %+v", a.Exec)
	}
	if err := a.Validate(); err != nil {
		t.Errorf("the approval request is not valid: %v", err)
	}
}

// T36: a grant in test covers only the approved program; a program
// without a statement needs an approval for each command; the grant ends
// with its lifetime; in prod each command needs an approval and leaves no
// grant; in dev a grant of one way covers the other way.
func TestExecGrantBinding(t *testing.T) {
	s := sess("s-1", "beispielprojekt")
	approveAll := func(h *harness, n int) {
		for range n {
			h.rec.Enqueue(fakeui.ReplyApproveAll())
		}
	}
	t.Run("test: same program", func(t *testing.T) {
		h := newHarness(t)
		approveAll(h, 3)
		steps := []struct {
			program string
			args    []string
			want    int
		}{
			{"/usr/bin/true", []string{"-a"}, 1},
			{"/usr/bin/true", []string{"-b", "-c"}, 1},
			{"/usr/bin/env", nil, 2},
		}
		for i, st := range steps {
			r := h.exec(s, execReq(t, st.program, st.args, "DB_PASSWORD", refTest))
			if r.err != nil || r.value("DB_PASSWORD") != valueTest {
				t.Fatalf("step %d: %v", i+1, r.err)
			}
			if h.approvals() != st.want {
				t.Errorf("step %d: approvals = %d, want %d", i+1, h.approvals(), st.want)
			}
		}
		if b := h.rec.Requests()[0].Exec.Approval.Binding; b != approval.BindingProgram {
			t.Errorf("binding = %s, want program", b)
		}
		// Other variable names need a new approval.
		approveAll(h, 1)
		if r := h.exec(s, execReq(t, "/usr/bin/true", nil, "PGPASSWORD", refTest)); r.err != nil || h.approvals() != 3 {
			t.Errorf("other names: %v, approvals = %d, want 3", r.err, h.approvals())
		}
	})
	t.Run("test: shell", func(t *testing.T) {
		h := newHarness(t)
		approveAll(h, 3)
		for i, text := range []string{"psql -c 'select 1'", "psql -c 'select 2'"} {
			if r := h.exec(s, execReq(t, "/bin/sh", []string{"-c", text}, "DB_PASSWORD", refTest)); r.err != nil {
				t.Fatalf("command %d: %v", i+1, r.err)
			}
			if h.approvals() != i+1 {
				t.Errorf("command %d: approvals = %d", i+1, h.approvals())
			}
			if b := h.lastApproval().Exec.Approval.Binding; b != approval.BindingCommand {
				t.Errorf("binding of sh = %s, want command", b)
			}
		}
		// The same text again is the same command.
		if r := h.exec(s, execReq(t, "/bin/sh", []string{"-c", "psql -c 'select 2'"}, "DB_PASSWORD", refTest)); r.err != nil || h.approvals() != 2 {
			t.Errorf("same command: %v, approvals = %d, want 2", r.err, h.approvals())
		}
	})
	t.Run("test: lifetime", func(t *testing.T) {
		h := newHarness(t)
		approveAll(h, 2)
		if r := h.exec(s, execReq(t, "/usr/bin/true", nil, "DB_PASSWORD", refTest)); r.err != nil {
			t.Fatal(r.err)
		}
		if g := h.eventsOf(audit.EventGrant); len(g) != 1 || g[0].Result.LifetimeSeconds != 1800 {
			t.Fatalf("grant events = %+v", g)
		}
		h.clk.Advance(31 * time.Minute)
		if r := h.exec(s, execReq(t, "/usr/bin/true", nil, "DB_PASSWORD", refTest)); r.err != nil || h.approvals() != 2 {
			t.Errorf("after 31 minutes: %v, approvals = %d, want 2", r.err, h.approvals())
		}
	})
	t.Run("prod: each command", func(t *testing.T) {
		h := newHarness(t, withConfig(prodOn))
		approveAll(h, 2)
		for i := range 2 {
			if r := h.exec(s, execReq(t, "/usr/bin/true", nil, "DB_PASSWORD", refProd)); r.err != nil || r.value("DB_PASSWORD") != valueProd {
				t.Fatalf("command %d: %v", i+1, r.err)
			}
		}
		if h.approvals() != 2 {
			t.Errorf("approvals = %d, want 2", h.approvals())
		}
		g, e := h.b.GrantStatus(context.Background(), s, &protocol.GrantStatus{ID: "g1"})
		if e != nil || len(g.Grants) != 0 || len(h.grants()) != 0 {
			t.Errorf("grant_status = %+v, %v; want no grant", g, e)
		}
		// Each grant of one delivery has its grant and its expiry.
		grants, exp := h.eventsOf(audit.EventGrant), h.eventsOf(audit.EventExpiry)
		if len(grants) != 2 || len(exp) != 2 {
			t.Fatalf("grant events %d, expiry events %d; want 2 and 2", len(grants), len(exp))
		}
		for i := range grants {
			if exp[i].Result.GrantID != grants[i].Result.GrantID || exp[i].Result.Detail != audit.DetailSingleDelivery || grants[i].Result.LifetimeSeconds != 0 {
				t.Errorf("grant %+v, expiry %+v", grants[i].Result, exp[i].Result)
			}
		}
		want := []audit.EventType{audit.EventRequest, audit.EventDecision, audit.EventGrant, audit.EventDelivery, audit.EventExpiry}
		if got := h.types(); !slices.Equal(got[:5], want) {
			t.Errorf("audit = %v, want %v first", got, want)
		}
	})
	t.Run("dev: other way", func(t *testing.T) {
		h := newHarness(t)
		approveAll(h, 1)
		if r := h.approveFlow(s, refPass); r.err != nil {
			t.Fatal(r.err)
		}
		r := h.exec(s, execReq(t, "/usr/bin/true", nil, "DB_PASSWORD", refPass))
		if r.err != nil || r.value("DB_PASSWORD") != marker || h.approvals() != 1 {
			t.Errorf("exec after request_credentials: %v, approvals = %d, want 1", r.err, h.approvals())
		}
		// And the other direction: a grant of the exec way covers the MCP way.
		h = newHarness(t)
		approveAll(h, 1)
		if r := h.exec(s, execReq(t, "/usr/bin/true", nil, "DB_PASSWORD", refPass)); r.err != nil {
			t.Fatal(r.err)
		}
		if b := h.lastApproval().Exec.Approval.Binding; b != approval.BindingNone {
			t.Errorf("binding in dev = %s, want none", b)
		}
		if r := h.do(s, refPass); r.err != nil || values(r.creds)[refPass] != marker || h.approvals() != 1 {
			t.Errorf("request_credentials after exec: %v, approvals = %d, want 1", r.err, h.approvals())
		}
	})
}

// D62, D63: a mixed request takes the strictest environment, the shortest
// lifetime and the strictest binding.
func TestExecMixedRequest(t *testing.T) {
	h := newHarness(t, withLifetime(""), withConfig("environments:\n  dev:\n    grant_lifetime: 10m\n    binding: command\n"))
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	s := sess("s-1", "beispielprojekt")
	r := h.exec(s, execReq(t, "/usr/bin/true", nil, "DB_PASSWORD", refPass, "TEST_PASSWORD", refTest))
	if r.err != nil || r.value("DB_PASSWORD") != marker || r.value("TEST_PASSWORD") != valueTest {
		t.Fatalf("result: %v", r.err)
	}
	a := h.lastApproval()
	if a.Exec.Environment != config.EnvTest || a.Grant.TTLSeconds != 600 || a.Exec.Approval.Binding != approval.BindingCommand {
		t.Errorf("approval: environment %s, lifetime %d, binding %s; want test, 600, command",
			a.Exec.Environment, a.Grant.TTLSeconds, a.Exec.Approval.Binding)
	}
	if !slices.Equal(a.Exec.Approval.LifetimeChoicesSeconds, approval.ExecLifetimeChoices(600)) {
		t.Errorf("lifetime choices = %v", a.Exec.Approval.LifetimeChoicesSeconds)
	}
	// The order of the values follows the variables of the request.
	if names := []string{r.creds.Values[0].Name, r.creds.Values[1].Name}; !slices.Equal(names, []string{"DB_PASSWORD", "TEST_PASSWORD"}) {
		t.Errorf("names = %v", names)
	}
}

// D63, G44: the broker checks the decision of a command itself. A longer
// lifetime than the configuration and a part of the variables end with
// ui_failed; a shorter offered lifetime creates a grant with it.
func TestExecDecisions(t *testing.T) {
	s := sess("s-1", "beispielprojekt")
	tests := []struct {
		name     string
		config   string
		refs     []string
		reply    fakeui.Reply
		want     protocol.Code
		lifetime int64
	}{
		{"all, configured lifetime", "", []string{refTest}, fakeui.ReplyApproveAll(), "", 1800},
		{"shorter offered lifetime", "", []string{refTest}, fakeui.ReplyApprove(900, "c1"), "", 900},
		{"longer lifetime", "", []string{refTest}, fakeui.ReplyApprove(3600, "c1"), protocol.CodeUIFailed, 0},
		{"lifetime not offered", "", []string{refTest}, fakeui.ReplyApprove(1000, "c1"), protocol.CodeUIFailed, 0},
		{"part of the variables", "", []string{refPass, refTest}, fakeui.ReplyApprove(1800, "c1"), protocol.CodeUIFailed, 0},
		{"no variable", "", []string{refTest}, fakeui.ReplyApprove(1800), protocol.CodeDenied, 0},
		{"denied", "", []string{refTest}, fakeui.ReplyDeny(), protocol.CodeDenied, 0},
		{"timeout", "", []string{refTest}, fakeui.ReplyTimeout(), protocol.CodeTimeout, 0},
		{"UI fails", "", []string{refTest}, fakeui.ReplyFail(errors.New("app failed")), protocol.CodeUIFailed, 0},
		{"prod with a lifetime", prodOn, []string{refProd}, fakeui.ReplyApprove(300, "c1"), protocol.CodeUIFailed, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, withConfig(tt.config))
			h.rec.Enqueue(tt.reply)
			var env []string
			for i, r := range tt.refs {
				env = append(env, []string{"A_PASSWORD", "B_PASSWORD"}[i], r)
			}
			r := h.exec(s, execReq(t, "/usr/bin/true", nil, env...))
			if r.code() != tt.want {
				t.Fatalf("code = %q, want %q", r.code(), tt.want)
			}
			if tt.want != "" {
				if r.creds != nil || h.be.Reads() != 0 || len(h.eventsOf(audit.EventGrant)) != 0 {
					t.Errorf("a failed decision delivered: %v, reads %d", r.creds, h.be.Reads())
				}
				return
			}
			if g := h.eventsOf(audit.EventGrant); len(g) != 1 || g[0].Result.LifetimeSeconds != tt.lifetime {
				t.Errorf("grant events = %+v", g)
			}
		})
	}
}

// T38: the approval request and the audit log name the command and never
// hold a value or the output of the command. The chain stays valid.
func TestExecApprovalRequestAndAudit(t *testing.T) {
	const outputText = "beispiel-ausgabe-4711"
	h := newHarness(t)
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	s := sess("s-1", "beispielprojekt")
	req := execReq(t, "/opt/homebrew/bin/psql", []string{"-h", "datenbank-test.beispiel.internal", "-c", "select count(*) from auftrag"},
		"PGPASSWORD", refTest)
	req.TimeoutSeconds = 30
	r := h.exec(s, req)
	if r.err != nil || r.value("PGPASSWORD") != valueTest {
		t.Fatalf("result: %v", r.err)
	}
	if r.creds.TimeoutSeconds != 30 || r.creds.OutputLimit != config.DefaultExecOutputLimit {
		t.Errorf("limits = %d s, %d bytes", r.creds.TimeoutSeconds, r.creds.OutputLimit)
	}
	rec, e := h.b.ExecResult(context.Background(), s, &protocol.ExecResult{
		ID: "x2", RequestID: "x1", ExitCode: 3, DurationMS: 412, StdoutBytes: int64(len(outputText)), Masked: 1,
	})
	if e != nil || rec == nil {
		t.Fatalf("exec_result = %v, %v", rec, e)
	}

	// The approval request.
	a := h.lastApproval()
	if a.Exec == nil {
		t.Fatal("the approval request has no command")
	}
	if a.Exec.Environment != config.EnvTest || a.Exec.Program != req.Program || !slices.Equal(a.Exec.Args, req.Args) ||
		a.Exec.Cwd != projectDir || len(a.Exec.Env) != 1 || a.Exec.Env[0].Name != "PGPASSWORD" {
		t.Errorf("command of the approval request = %+v", a.Exec)
	}
	c := a.Credentials[0]
	if c.ID != a.Exec.Env[0].CredentialID || c.Vault != "beispiel.app.k8s-test" || c.Title != "datenbank-test" || c.Field != "password" || c.Environment != config.EnvTest {
		t.Errorf("credential = %+v", c)
	}
	if a.Exec.Approval.Kind != approval.ExecApprovalGrant || a.Grant.TTLSeconds != 1800 {
		t.Errorf("approval = %+v, lifetime %d", a.Exec.Approval, a.Grant.TTLSeconds)
	}

	// The audit log.
	want := []audit.EventType{audit.EventRequest, audit.EventDecision, audit.EventGrant, audit.EventDelivery, audit.EventExecution}
	if got := h.types(); !slices.Equal(got, want) {
		t.Fatalf("audit = %v, want %v", got, want)
	}
	for _, ev := range h.events() {
		if ev.Command == nil || ev.Command.Way != audit.WayExec || ev.Command.Program != req.Program ||
			!slices.Equal(ev.Command.Args, req.Args) || ev.Command.Cwd != projectDir ||
			len(ev.Command.Env) != 1 || ev.Command.Env[0].Name != "PGPASSWORD" || ev.Command.Env[0].Reference.String() != refTest {
			t.Errorf("%s: command = %+v", ev.Type, ev.Command)
		}
	}
	for _, typ := range []audit.EventType{audit.EventGrant, audit.EventDelivery} {
		ev := h.eventsOf(typ)[0]
		if ev.Command.Binding != string(config.BindingProgram) || ev.Command.TimeoutSeconds != 30 {
			t.Errorf("%s: binding %q, timeout %d", typ, ev.Command.Binding, ev.Command.TimeoutSeconds)
		}
	}
	x := h.eventsOf(audit.EventExecution)[0]
	if x.Execution == nil || x.Execution.ExitCode != 3 || x.Execution.ExecRequestID != "x1" || x.Execution.Masked != 1 ||
		x.Execution.DurationMS != 412 || x.Result.GrantID != h.eventsOf(audit.EventGrant)[0].Result.GrantID {
		t.Errorf("execution = %+v, %+v", x.Execution, x.Result)
	}
	// The verified facts stay apart from the claims of a38 mcp.
	if x.Verified != s.Facts {
		t.Errorf("verified facts = %+v", x.Verified)
	}

	// No value and no output anywhere.
	raw, err := os.ReadFile(h.auditPath)
	if err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{
		"approval": mustJSON(t, h.rec.Requests()), "audit": mustJSON(t, h.events()),
		"audit file": string(raw), "log": h.logs.String(),
	} {
		if strings.Contains(text, valueTest) || strings.Contains(text, outputText) {
			t.Errorf("%s contains a value or the output", name)
		}
	}
	if v, err := h.inner.Verify(context.Background()); err != nil || !v.OK {
		t.Errorf("Verify = %+v, %v", v, err)
	}
}

// T39: the broker does not deliver if an argument or the working directory
// contains one of the values. The denial names the reason, not the value.
func TestExecValueInCommand(t *testing.T) {
	s := sess("s-1", "beispielprojekt")
	tests := []struct {
		name string
		mut  func(*protocol.RequestExec)
	}{
		{"argument", func(r *protocol.RequestExec) { r.Args = []string{"-c", "login " + valueTest + " now"} }},
		{"working directory", func(r *protocol.RequestExec) { r.Cwd = projectDir + "/" + valueTest }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, withMode(approval.ModeAutoApprove), withUI(func(*fakeui.Recorder, *clock.Fake) approval.UI {
				return fakeui.AutoApprove{}
			}))
			req := execReq(t, "/usr/bin/true", nil, "DB_PASSWORD", refTest)
			tt.mut(req)
			r := h.exec(s, req)
			if r.code() != protocol.CodeInvalidRequest || r.creds != nil {
				t.Fatalf("result = %v, %v; want invalid_request", r.creds, r.err)
			}
			if strings.Contains(r.err.Message, valueTest) {
				t.Error("the error names the value")
			}
			d := h.eventsOf(audit.EventDenial)
			if len(d) != 1 || d[0].Result.Code != protocol.CodeInvalidRequest || d[0].Result.Detail != audit.DetailValueInCommand {
				t.Fatalf("denial events = %+v", d)
			}
			if len(h.eventsOf(audit.EventDelivery)) != 0 {
				t.Error("the audit log has a delivery")
			}
			// The audit log keeps the command as the agent sent it; the
			// argument is a claim of the agent. The detail and the log of
			// the daemon never repeat a value.
			if strings.Contains(h.logs.String(), valueTest) || strings.Contains(mustJSON(t, d[0]), valueTest) {
				t.Error("the log or the denial contains the value")
			}
			// RI14: only the entries before the read of the values can
			// hold the command of the agent with the value.
			for _, e := range h.events() {
				if strings.Contains(mustJSON(t, e), valueTest) && !slices.Contains(
					[]audit.EventType{audit.EventRequest, audit.EventDecision, audit.EventGrant}, e.Type) {
					t.Errorf("the event %s contains the value", e.Type)
				}
			}
		})
	}
}

// T34 step 5 (daemon part): the broker checks the working directory itself
// and answers not_allowed if it is not in the project of the session.
func TestExecWorkingDirectory(t *testing.T) {
	s := sess("s-1", "beispielprojekt")
	tests := []struct {
		name     string
		cwd      string
		realPath func(string) (string, error)
		want     protocol.Code
	}{
		{"in the project", projectDir + "/sub", nil, ""},
		{"outside each project", "/tmp/a38-test/fremd", nil, protocol.CodeNotAllowed},
		{"other project", "/tmp/a38-test/anderesprojekt", nil, protocol.CodeNotAllowed},
		{"link to outside", projectDir + "/link", func(string) (string, error) { return "/private/etc", nil }, protocol.CodeNotAllowed},
		{"path does not exist", projectDir + "/fehlt", func(string) (string, error) { return "", os.ErrNotExist }, protocol.CodeNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			if tt.realPath != nil {
				h.b.realPath = tt.realPath
			}
			h.rec.Enqueue(fakeui.ReplyApproveAll())
			req := execReq(t, "/usr/bin/true", nil, "DB_PASSWORD", refTest)
			req.Cwd = tt.cwd
			r := h.exec(s, req)
			if r.code() != tt.want {
				t.Fatalf("code = %q, want %q", r.code(), tt.want)
			}
			if tt.want != "" && (h.approvals() != 0 || h.be.Resolves() != 0) {
				t.Errorf("approvals %d, resolves %d; want 0", h.approvals(), h.be.Resolves())
			}
		})
	}
}

// D64, D66: the limits of the exec way come from the configuration.
func TestExecLimits(t *testing.T) {
	s := sess("s-1", "beispielprojekt")
	h := newHarness(t, withMaxEntries("2"), withConfig("exec:\n  default_timeout: 1m\n  max_timeout: 5m\n  output_limit: 2KiB\n"))
	tests := []struct {
		name        string
		timeout     int
		env         []string
		want        protocol.Code
		wantTimeout int
	}{
		{"default time limit", 0, []string{"A_PASSWORD", refTest}, "", 60},
		{"own time limit", 300, []string{"A_PASSWORD", refTest}, "", 300},
		{"time limit above the maximum", 301, []string{"A_PASSWORD", refTest}, protocol.CodeInvalidRequest, 0},
		{"more variables than bundle.max_entries", 0, []string{"A_PASSWORD", refTest, "B_PASSWORD", refTest, "C_PASSWORD", refTest}, protocol.CodeBundleTooLarge, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h.rec.Enqueue(fakeui.ReplyApproveAll())
			before := h.approvals()
			req := execReq(t, "/usr/bin/true", nil, tt.env...)
			req.TimeoutSeconds = tt.timeout
			r := h.exec(s, req)
			if r.code() != tt.want {
				t.Fatalf("code = %q, want %q", r.code(), tt.want)
			}
			if tt.want != "" {
				if h.approvals() != before {
					t.Error("a request over a limit reached the human")
				}
				return
			}
			if r.creds.TimeoutSeconds != tt.wantTimeout || r.creds.OutputLimit != 2048 {
				t.Errorf("limits = %d s, %d bytes", r.creds.TimeoutSeconds, r.creds.OutputLimit)
			}
		})
	}
}

// G39, D65: exec_result writes the event execution once for each delivery.
// A result without a delivery and a second result are invalid_request.
func TestExecResultRecords(t *testing.T) {
	s := sess("s-1", "beispielprojekt")
	h := newHarness(t)
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	if r := h.exec(s, execReq(t, "/usr/bin/true", nil, "DB_PASSWORD", refTest)); r.err != nil {
		t.Fatal(r.err)
	}
	ctx := context.Background()
	if _, e := h.b.ExecResult(ctx, sess("s-2", "beispielprojekt"), &protocol.ExecResult{ID: "x2", RequestID: "x1"}); e == nil || e.Code != protocol.CodeInvalidRequest {
		t.Errorf("result of another session = %v; want invalid_request", e)
	}
	failed := &protocol.ExecResult{ID: "x2", RequestID: "x1", Error: protocol.Errorf(protocol.CodeExecFailed, "the program is not executable")}
	if _, e := h.b.ExecResult(ctx, s, failed); e != nil {
		t.Fatalf("exec_result with exec_failed = %v", e)
	}
	x := h.eventsOf(audit.EventExecution)
	if len(x) != 1 || !x[0].Execution.NotStarted || x[0].Result.Code != protocol.CodeExecFailed || x[0].Result.Detail != audit.DetailNotStarted {
		t.Fatalf("execution events = %+v", x)
	}
	if _, e := h.b.ExecResult(ctx, s, &protocol.ExecResult{ID: "x3", RequestID: "x1"}); e == nil || e.Code != protocol.CodeInvalidRequest {
		t.Errorf("second result = %v; want invalid_request", e)
	}
	if n := len(h.eventsOf(audit.EventExecution)); n != 1 {
		t.Errorf("execution events = %d, want 1", n)
	}
}

// mustJSONExec is a guard that the JSON of an answer of the exec way holds
// the value only through protocol.Encode.
func TestExecAnswerHidesValueInJSON(t *testing.T) {
	h := newHarness(t)
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	r := h.exec(sess("s-1", "beispielprojekt"), execReq(t, "/usr/bin/true", nil, "DB_PASSWORD", refTest))
	if r.err != nil {
		t.Fatal(r.err)
	}
	out, _ := json.Marshal(r.creds)
	if strings.Contains(string(out), valueTest) {
		t.Error("json.Marshal of the answer shows the value")
	}
}

// T36 (D76): with grant_lifetime 0s, dev and test ask the human for each
// command. The approval request has the kind once; the grant covers one
// delivery and grant_status never shows it. request_credentials in dev
// with the lifetime 0 ends with exec_required.
func TestExecOnceInDevAndTest(t *testing.T) {
	s := sess("s-1", "beispielprojekt")
	tests := []struct {
		name   string
		config string
		ref    string
		value  string
	}{
		{"dev", "environments:\n  dev:\n    grant_lifetime: 0s\n", refPass, marker},
		{"test", "environments:\n  test:\n    grant_lifetime: 0s\n", refTest, valueTest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, withLifetime(""), withConfig(tt.config))
			for i := range 2 {
				h.rec.Enqueue(fakeui.ReplyApproveAll())
				r := h.exec(s, execReq(t, "/usr/bin/true", nil, "DB_PASSWORD", tt.ref))
				if r.err != nil || r.value("DB_PASSWORD") != tt.value {
					t.Fatalf("command %d: %v; want the value", i+1, r.err)
				}
				if h.approvals() != i+1 {
					t.Errorf("command %d: approvals = %d, want %d", i+1, h.approvals(), i+1)
				}
				a := h.lastApproval()
				if err := a.Validate(); err != nil {
					t.Errorf("the approval request is not valid: %v", err)
				}
				if a.Exec.Approval.Kind != approval.ExecApprovalOnce || a.Exec.Approval.Binding != approval.BindingCommand ||
					a.Grant.TTLSeconds != 0 || len(a.Exec.Approval.LifetimeChoicesSeconds) != 0 {
					t.Errorf("approval = %+v, ttl %d; want once", a.Exec.Approval, a.Grant.TTLSeconds)
				}
				if g := h.grants(); len(g) != 0 {
					t.Errorf("grant_status = %v, want no grant", g)
				}
			}
			if n := len(h.eventsOf(audit.EventExpiry)); n != 2 {
				t.Errorf("expiry events = %d, want 2", n)
			}
		})
	}
	t.Run("dev request_credentials", func(t *testing.T) {
		h := newHarness(t, withLifetime(""), withConfig(tests[0].config))
		if r := h.do(s, refPass); r.code() != protocol.CodeExecRequired || r.creds != nil {
			t.Fatalf("result = %v, %v; want exec_required", r.creds, r.err)
		}
		if h.approvals() != 0 || h.be.Resolves() != 0 {
			t.Errorf("approvals %d, resolves %d; want 0", h.approvals(), h.be.Resolves())
		}
	})
}

// T36 (D63, review of pull request 115): a grant with the binding program
// covers only the approved mapping of names to references. The agent
// cannot move the password to another variable name without a new
// approval.
func TestExecGrantMapping(t *testing.T) {
	s := sess("s-1", "beispielprojekt")
	h := newHarness(t, withLifetime(""), withConfig("environments:\n  dev:\n    binding: program\n"))
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	if r := h.exec(s, execReq(t, "/usr/bin/true", nil, "PGUSER", refUser, "PGPASSWORD", refPass)); r.err != nil {
		t.Fatal(r.err)
	}
	if r := h.exec(s, execReq(t, "/usr/bin/true", []string{"-x"}, "PGPASSWORD", refPass, "PGUSER", refUser)); r.err != nil || h.approvals() != 1 {
		t.Fatalf("same mapping: %v, approvals %d; want 1", r.err, h.approvals())
	}
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	r := h.exec(s, execReq(t, "/usr/bin/true", nil, "PGUSER", refPass))
	if r.err != nil || h.approvals() != 2 {
		t.Errorf("password under PGUSER: %v, approvals %d; want a new approval", r.err, h.approvals())
	}
}

// T36 (D63, review of pull request 115): the broker resolves the symbolic
// links of the program. The list of programs without a statement uses the
// resolved base name, case-folded and without a version suffix. The
// approval request, the audit log and the grant use the resolved path. A
// program that does not resolve ends with not_allowed.
func TestExecProgramResolved(t *testing.T) {
	s := sess("s-1", "beispielprojekt")
	links := map[string]string{
		projectDir + "/psql":   "/bin/sh",
		"/usr/bin/python3.12":  "/usr/bin/python3.12",
		"/usr/bin/perl5.34":    "/usr/bin/perl5.34",
		"/bin/SH":              "/bin/SH",
		"/opt/homebrew/bin/pg": "/opt/homebrew/Cellar/libpq/17/bin/psql",
	}
	resolve := func(p string) (string, error) {
		if r, ok := links[p]; ok {
			return r, nil
		}
		if strings.HasPrefix(p, projectDir) {
			return p, nil
		}
		return "", os.ErrNotExist
	}
	tests := []struct {
		program  string
		resolved string
		binding  approval.Binding
	}{
		{projectDir + "/psql", "/bin/sh", approval.BindingCommand},
		{"/bin/SH", "/bin/SH", approval.BindingCommand},
		{"/usr/bin/python3.12", "/usr/bin/python3.12", approval.BindingCommand},
		{"/usr/bin/perl5.34", "/usr/bin/perl5.34", approval.BindingCommand},
		{"/opt/homebrew/bin/pg", "/opt/homebrew/Cellar/libpq/17/bin/psql", approval.BindingProgram},
	}
	for _, tt := range tests {
		t.Run(tt.program, func(t *testing.T) {
			h := newHarness(t)
			h.b.realPath = resolve
			h.rec.Enqueue(fakeui.ReplyApproveAll())
			if r := h.exec(s, execReq(t, tt.program, []string{"-c", "select 1"}, "DB_PASSWORD", refTest)); r.err != nil {
				t.Fatal(r.err)
			}
			a := h.lastApproval().Exec
			if a.Program != tt.resolved || a.Approval.Binding != tt.binding {
				t.Errorf("approval: program %q, binding %s; want %q, %s", a.Program, a.Approval.Binding, tt.resolved, tt.binding)
			}
			for _, typ := range []audit.EventType{audit.EventDecision, audit.EventGrant, audit.EventDelivery} {
				for _, e := range h.eventsOf(typ) {
					if e.Command == nil || e.Command.Program != tt.resolved {
						t.Errorf("%s: command = %+v; want the resolved program", typ, e.Command)
					}
				}
			}
			// A grant binds the resolved path: another link to the same
			// program is the same program.
			if tt.binding == approval.BindingProgram {
				links[projectDir+"/pg2"] = tt.resolved
				if r := h.exec(s, execReq(t, projectDir+"/pg2", nil, "DB_PASSWORD", refTest)); r.err != nil || h.approvals() != 1 {
					t.Errorf("other link: %v, approvals %d; want 1", r.err, h.approvals())
				}
			}
		})
	}
	t.Run("not resolvable", func(t *testing.T) {
		h := newHarness(t)
		h.b.realPath = resolve
		r := h.exec(s, execReq(t, "/usr/bin/fehlt", nil, "DB_PASSWORD", refTest))
		if r.code() != protocol.CodeNotAllowed || h.approvals() != 0 || h.be.Resolves() != 0 {
			t.Errorf("result %v, approvals %d, resolves %d; want not_allowed", r.err, h.approvals(), h.be.Resolves())
		}
	})
}

// T39 (review of pull request 115): the check for a value also covers the
// program, the task and the reason. The answer and the denial name the
// reason, never the value.
func TestExecValueInProgramOrClaims(t *testing.T) {
	s := sess("s-1", "beispielprojekt")
	tests := []struct {
		name string
		mut  func(*protocol.RequestExec)
	}{
		{"program", func(r *protocol.RequestExec) { r.Program = projectDir + "/" + valueTest }},
		{"task", func(r *protocol.RequestExec) { r.Task = "Log in with " + valueTest }},
		{"reason", func(r *protocol.RequestExec) { r.Reason = "The value is " + valueTest }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, withMode(approval.ModeAutoApprove), withUI(func(*fakeui.Recorder, *clock.Fake) approval.UI {
				return fakeui.AutoApprove{}
			}))
			req := execReq(t, "/usr/bin/true", nil, "DB_PASSWORD", refTest)
			tt.mut(req)
			r := h.exec(s, req)
			if r.code() != protocol.CodeInvalidRequest || r.creds != nil {
				t.Fatalf("result = %v, %v; want invalid_request", r.creds, r.err)
			}
			if strings.Contains(r.err.Message, valueTest) {
				t.Error("the error names the value")
			}
			d := h.eventsOf(audit.EventDenial)
			if len(d) != 1 || d[0].Result.Detail != audit.DetailValueInCommand || strings.Contains(mustJSON(t, d[0]), valueTest) {
				t.Fatalf("denial events = %+v", d)
			}
			if len(h.eventsOf(audit.EventDelivery)) != 0 {
				t.Error("the audit log has a delivery")
			}
		})
	}
}

// T38 (review of pull request 115): exec_result finds its delivery by the
// ID that the client selects. A request_exec with the ID of a delivery
// that waits for its exec_result ends with invalid_request.
func TestExecDuplicateID(t *testing.T) {
	s := sess("s-1", "beispielprojekt")
	dup := func() *protocol.RequestExec {
		r := execReq(t, "/usr/bin/true", nil, "DB_PASSWORD", refTest)
		r.ID = "dup"
		return r
	}
	h := newHarness(t)
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	if r := h.exec(s, dup()); r.err != nil {
		t.Fatal(r.err)
	}
	r := h.exec(s, dup())
	if r.code() != protocol.CodeInvalidRequest || r.creds != nil {
		t.Fatalf("same ID: %v, %v; want invalid_request", r.creds, r.err)
	}
	if n := len(h.eventsOf(audit.EventDelivery)); n != 1 {
		t.Errorf("deliveries = %d, want 1", n)
	}
	// Another session can use the same ID.
	h.rec.Enqueue(fakeui.ReplyApproveAll())
	if r := h.exec(sess("s-2", "beispielprojekt"), dup()); r.err != nil {
		t.Errorf("other session: %v", r.err)
	}
	// After the exec_result the ID is free again.
	if _, e := h.b.ExecResult(context.Background(), s, &protocol.ExecResult{ID: "r1", RequestID: "dup"}); e != nil {
		t.Fatal(e)
	}
	if r := h.exec(s, dup()); r.err != nil {
		t.Errorf("after the result: %v", r.err)
	}
}
