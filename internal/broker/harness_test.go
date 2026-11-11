package broker

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/approval/fakeui"
	"github.com/seletz/passierschein/internal/approval/queue"
	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/audit/faultstore"
	"github.com/seletz/passierschein/internal/audit/sqlite"
	"github.com/seletz/passierschein/internal/backend/fake"
	"github.com/seletz/passierschein/internal/clock"
	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/fault"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// marker is the value of the field that T8 follows through each path.
const marker = "kennwert-7f3a9c41"

// backendYAML is the fake backend of the tests. All names and values are
// fake.
const backendYAML = `
vaults:
  - name: beispiel.app.k8s-dev
    items:
      - name: datenbank-dev
        fields:
          - name: username
            value: beispiel-wert-1
          - name: password
            value: ` + marker + `
      - name: objektspeicher-dev
        fields:
          - name: username
            value: beispiel-wert-3
          - name: password
            value: beispiel-wert-4
      - name: dienst-dev
        fields:
          - {name: f01, value: beispiel-wert-f01}
          - {name: f02, value: beispiel-wert-f02}
          - {name: f03, value: beispiel-wert-f03}
          - {name: f04, value: beispiel-wert-f04}
          - {name: f05, value: beispiel-wert-f05}
          - {name: f06, value: beispiel-wert-f06}
          - {name: f07, value: beispiel-wert-f07}
          - {name: f08, value: beispiel-wert-f08}
          - {name: f09, value: beispiel-wert-f09}
          - {name: f10, value: beispiel-wert-f10}
          - {name: f11, value: beispiel-wert-f11}
          - {name: f12, value: beispiel-wert-f12}
  - name: beispiel.infra.k8s-dev
    items:
      - name: datenbank-dev
        fields:
          - name: password
            value: beispiel-wert-6
  - name: beispiel.extern.k8s-dev
    items:
      - name: datenbank-dev
        fields:
          - name: password
            value: beispiel-wert-7
  - name: beispiel.app.k8s-test
    items:
      - name: datenbank-test
        fields:
          - name: password
            value: beispiel-wert-8
  - name: beispiel.app.k8s-prod
    items:
      - name: datenbank-prod
        fields:
          - name: password
            value: beispiel-wert-9
  - name: beispiel-prod
    items:
      - name: datenbank
        fields:
          - name: password
            value: beispiel-wert-10
  - name: beispiel-dev
    items:
      - name: datenbank
        fields:
          - name: password
            value: beispiel-wert-11
  - name: k8s-beispiel
    items:
      - name: datenbank-dev
        fields:
          - name: password
            value: beispiel-wert-12
`

// configYAML returns the configuration of the tests with the grant
// lifetime and the bundle maximum.
func configYAML(lifetime string, maxEntries string) string {
	return `
version: 1
vaults:
  - {name: beispiel.app.k8s-dev, environment: dev}
  - {name: beispiel.infra.k8s-dev, environment: dev}
  - {name: beispiel.extern.k8s-dev, environment: dev}
  - {name: beispiel.app.k8s-test, environment: test}
  - {name: beispiel.app.k8s-prod, environment: prod}
  - {name: beispiel-prod, environment: dev}
  - {name: beispiel-dev, environment: prod}
projects:
  - name: beispielprojekt
    paths: [/tmp/a38-test/beispielprojekt]
    vaults: [beispiel.app.k8s-dev, beispiel.infra.k8s-dev, beispiel.app.k8s-test, beispiel.app.k8s-prod, beispiel-prod, beispiel-dev]
  - name: anderesprojekt
    paths: [/tmp/a38-test/anderesprojekt]
    vaults: [beispiel.extern.k8s-dev]
  - name: kaputtprojekt
    paths: [/tmp/a38-test/kaputtprojekt]
    vaults: [beispiel.app.k8s-dev, fehlt.k8s-dev]
grant:
  lifetime: ` + lifetime + `
bundle:
  window: 3s
  max_entries: ` + maxEntries + `
paths:
  socket_dir: /tmp/a38-test/run
  audit_file: /tmp/a38-test/audit.db
`
}

// Neutral references of the tests.
const (
	refUser    = "op://beispiel.app.k8s-dev/datenbank-dev/username"
	refPass    = "op://beispiel.app.k8s-dev/datenbank-dev/password"
	refObj     = "op://beispiel.app.k8s-dev/objektspeicher-dev/username"
	refObjPass = "op://beispiel.app.k8s-dev/objektspeicher-dev/password"
	refInfra   = "op://beispiel.infra.k8s-dev/datenbank-dev/password"
	refExtern  = "op://beispiel.extern.k8s-dev/datenbank-dev/password"
)

// syncBuffer is a buffer for the log that is safe for concurrent use.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// harness is one broker with all fakes.
type harness struct {
	t         *testing.T
	cfg       *config.Config
	clk       *clock.Fake
	be        *fake.Backend
	rec       *fakeui.Recorder
	inner     *sqlite.Store
	store     *faultstore.Store
	auditPath string
	logs      *syncBuffer
	b         *Broker
}

// options of a harness.
type hopt struct {
	lifetime   string
	maxEntries string
	ui         func(*fakeui.Recorder, *clock.Fake) approval.UI
	lock       fault.Lock
}

type harnessOption func(*hopt)

func withLifetime(l string) harnessOption   { return func(o *hopt) { o.lifetime = l } }
func withMaxEntries(n string) harnessOption { return func(o *hopt) { o.maxEntries = n } }
func withLockOpt(l fault.Lock) harnessOption {
	return func(o *hopt) { o.lock = l }
}
func withUI(f func(*fakeui.Recorder, *clock.Fake) approval.UI) harnessOption {
	return func(o *hopt) { o.ui = f }
}

func newHarness(t *testing.T, opts ...harnessOption) *harness {
	t.Helper()
	o := hopt{lifetime: "4h", maxEntries: "10"}
	for _, f := range opts {
		f(&o)
	}
	cfg, err := config.Parse([]byte(configYAML(o.lifetime, o.maxEntries)))
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	be, err := fake.Parse([]byte(backendYAML))
	if err != nil {
		t.Fatalf("backend: %v", err)
	}
	h := &harness{
		t:         t,
		cfg:       cfg,
		clk:       clock.NewFake(time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)),
		be:        be,
		rec:       fakeui.NewRecorder(),
		auditPath: filepath.Join(t.TempDir(), "audit.db"),
		logs:      &syncBuffer{},
	}
	h.inner, err = sqlite.Open(h.auditPath, sqlite.WithClock(h.clk))
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	t.Cleanup(func() { _ = h.inner.Close() })
	h.store = faultstore.New(h.inner)
	var ui approval.UI = queue.New(h.rec, h.clk, 0)
	if o.ui != nil {
		ui = o.ui(h.rec, h.clk)
	}
	var bopts []Option
	if o.lock != nil {
		bopts = append(bopts, WithLock(o.lock))
	}
	logger := slog.New(slog.NewTextHandler(h.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	h.b = New(cfg, be, ui, h.store, h.clk, logger, bopts...)
	return h
}

// restart returns a new broker on the same backend, UI and audit log: a
// restart of the daemon.
func (h *harness) restart() {
	logger := slog.New(slog.NewTextHandler(h.logs, nil))
	h.b = New(h.cfg, h.be, queue.New(h.rec, h.clk, 0), h.store, h.clk, logger)
}

// sess returns a session of the project with verified facts.
func sess(id, project string) session.Session {
	return session.Session{
		Facts: session.Facts{
			SessionID: session.ID(id),
			Origin:    session.OriginLocal,
			Project:   project,
			PeerPID:   4242,
			PeerUID:   501,
			Process:   "/usr/local/bin/a38",
			Cwd:       "/tmp/a38-test/" + project,
		},
		Name:  "beispiel-koordinator",
		Agent: "beispiel-agent",
	}
}

// refs parses references.
func refs(t *testing.T, texts ...string) []protocol.Reference {
	t.Helper()
	out := make([]protocol.Reference, 0, len(texts))
	for _, s := range texts {
		r, err := protocol.ParseReference(s)
		if err != nil {
			t.Fatalf("reference %q: %v", s, err)
		}
		out = append(out, r)
	}
	return out
}

// ref parses one reference.
func ref(t *testing.T, text string) protocol.Reference { return refs(t, text)[0] }

// result is the answer of RequestCredentials.
type result struct {
	creds *protocol.Credentials
	err   *protocol.Error
}

// newRequest returns a request for the references.
func newRequest(t *testing.T, texts ...string) *protocol.RequestCredentials {
	return &protocol.RequestCredentials{
		ID:         "r1",
		References: refs(t, texts...),
		Task:       "Run the migration test",
		Reason:     "The test needs the database of the dev environment",
	}
}

// async runs a request in its own goroutine.
func (h *harness) async(ctx context.Context, s session.Session, req *protocol.RequestCredentials) <-chan result {
	ch := make(chan result, 1)
	go func() {
		c, e := h.b.RequestCredentials(ctx, s, req)
		ch <- result{c, e}
	}()
	return ch
}

// do runs a request that does not wait for a bundle window.
func (h *harness) do(s session.Session, texts ...string) result {
	h.t.Helper()
	return h.wait(h.async(context.Background(), s, newRequest(h.t, texts...)))
}

// approveFlow runs a request through one bundle window. The test enqueues
// the reply of the UI before.
func (h *harness) approveFlow(s session.Session, texts ...string) result {
	h.t.Helper()
	ch := h.async(context.Background(), s, newRequest(h.t, texts...))
	h.waitTimers(1)
	h.clk.Advance(h.cfg.Bundle.Window)
	return h.wait(ch)
}

// wait waits for a result.
func (h *harness) wait(ch <-chan result) result {
	h.t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(10 * time.Second):
		h.t.Fatal("the request did not end")
		return result{}
	}
}

// waitTimers waits until n timers of the fake clock are pending.
func (h *harness) waitTimers(n int) {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := h.clk.BlockUntil(ctx, n); err != nil {
		h.t.Fatalf("waiting for %d timers: %v", n, err)
	}
}

// waitFor polls cond until it is true.
func (h *harness) waitFor(what string, cond func() bool) {
	h.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			h.t.Fatalf("waiting for %s: time is up", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// waitOpen waits until the open bundle of the session has n entries.
func (h *harness) waitOpen(s session.Session, n int) {
	h.t.Helper()
	h.waitFor("the bundle entries", func() bool { return h.b.openLen(s.Facts.SessionID) == n })
}

// openLen returns the number of entries of the open bundle of the session.
func (b *Broker) openLen(sid session.ID) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	st, ok := b.sessions[sid]
	if !ok || st.open == nil {
		return 0
	}
	return st.open.b.Len()
}

// openRequests returns the number of requests that wait for the open
// bundle of the session.
func (b *Broker) openRequests(sid session.ID) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	st, ok := b.sessions[sid]
	if !ok || st.open == nil {
		return 0
	}
	return len(st.open.requests)
}

// events returns the audit entries.
func (h *harness) events() []audit.Entry {
	h.t.Helper()
	es, err := h.inner.List(context.Background(), audit.Filter{})
	if err != nil {
		h.t.Fatalf("audit list: %v", err)
	}
	return es
}

// types returns the event types of the audit log in their sequence.
func (h *harness) types() []audit.EventType {
	var out []audit.EventType
	for _, e := range h.events() {
		out = append(out, e.Type)
	}
	return out
}

// eventsOf returns the audit entries of the type.
func (h *harness) eventsOf(t audit.EventType) []audit.Entry {
	var out []audit.Entry
	for _, e := range h.events() {
		if e.Type == t {
			out = append(out, e)
		}
	}
	return out
}

// grants returns the live grants of all sessions.
func (h *harness) grants() []protocol.Grant {
	h.t.Helper()
	g, e := h.b.GrantsList(context.Background(), &protocol.GrantsList{ID: "l1"})
	if e != nil {
		h.t.Fatalf("grants list: %v", e)
	}
	return g.Grants
}

// values returns the revealed values of an answer, by reference text.
func values(c *protocol.Credentials) map[string]string {
	out := map[string]string{}
	if c == nil {
		return out
	}
	for _, e := range c.Entries {
		if e.Value != nil {
			out[e.Reference.String()] = e.Value.Reveal()
		}
	}
	return out
}

// entryCodes returns the codes of the entries of an answer, by reference
// text.
func entryCodes(c *protocol.Credentials) map[string]protocol.Code {
	out := map[string]protocol.Code{}
	if c == nil {
		return out
	}
	for _, e := range c.Entries {
		if e.Error != nil {
			out[e.Reference.String()] = e.Error.Code
		}
	}
	return out
}

// code returns the code of a whole error, or "".
func (r result) code() protocol.Code {
	if r.err == nil {
		return ""
	}
	return r.err.Code
}

// spyLock is a fault.Lock for tests: the test sets the fault, and the lock
// records each Raise.
type spyLock struct {
	mu      sync.Mutex
	faulted bool
	raised  []protocol.Code
}

func (l *spyLock) Check() *protocol.Error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.faulted {
		return protocol.Errorf(protocol.CodeDaemonFaulted, "delivery is locked")
	}
	return nil
}

func (l *spyLock) Raise(code protocol.Code, _ string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.raised = append(l.raised, code)
}

func (l *spyLock) set(on bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.faulted = on
}

func (l *spyLock) raisedCodes() []protocol.Code {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]protocol.Code(nil), l.raised...)
}

func (l *spyLock) Fault() *protocol.Fault {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.faulted {
		return nil
	}
	return &protocol.Fault{Code: protocol.CodeAuditFailed, Reason: "test"}
}

func (l *spyLock) Clear(context.Context) (bool, string) {
	l.set(false)
	return true, ""
}
