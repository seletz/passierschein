package fault_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/approval/fakeui"
	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/audit/faultstore"
	"github.com/seletz/passierschein/internal/audit/sqlite"
	"github.com/seletz/passierschein/internal/clock"
	"github.com/seletz/passierschein/internal/fault"
	"github.com/seletz/passierschein/internal/protocol"
)

// NoLock never locks delivery.
func TestNoLock(t *testing.T) {
	var l fault.Lock = fault.NoLock{}
	l.Raise(protocol.CodeAuditFailed, "test")
	if l.Check() != nil || l.Fault() != nil || l.Inspect(context.Background()) != nil {
		t.Fatal("NoLock locked")
	}
	if ok, finding := l.Clear(context.Background()); !ok || finding != "" {
		t.Fatalf("Clear = %v, %q", ok, finding)
	}
}

// syncBuffer is a log buffer that is safe for concurrent use.
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

// fixture is one lock with a fault store, a recorder UI and a fake clock.
type fixture struct {
	t     *testing.T
	inner *sqlite.Store
	store *faultstore.Store
	rec   *fakeui.Recorder
	clk   *clock.Fake
	logs  *syncBuffer
	g     *fault.Global
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		t:    t,
		rec:  fakeui.NewRecorder(),
		clk:  clock.NewFake(time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)),
		logs: &syncBuffer{},
	}
	var err error
	f.inner, err = sqlite.Open(filepath.Join(t.TempDir(), "audit.db"), sqlite.WithClock(f.clk))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.inner.Close() })
	f.store = faultstore.New(f.inner)
	f.g = f.lock(f.store)
	return f
}

// lock returns a new lock on the store with the UI, clock and log of f.
func (f *fixture) lock(store audit.Store) *fault.Global {
	return fault.New(fault.Options{
		Store:  store,
		UI:     f.rec,
		Clock:  f.clk,
		Logger: slog.New(slog.NewTextHandler(f.logs, nil)),
	})
}

// details returns the details of the error entries in the audit log.
func (f *fixture) details() []string {
	f.t.Helper()
	es, err := f.inner.List(context.Background(), audit.Filter{Types: []audit.EventType{audit.EventError}})
	if err != nil {
		f.t.Fatal(err)
	}
	var out []string
	for _, e := range es {
		out = append(out, e.Result.Detail)
	}
	return out
}

func (f *fixture) notices() []approval.Notice {
	f.g.Wait()
	return f.rec.Notices()
}

// D48, G21, G22: a fault with a working audit log. The lock closes the
// gate, sends one notice, writes the begin at once and keeps the first
// fault.
func TestRaiseWithWorkingAuditLog(t *testing.T) {
	f := newFixture(t)
	if f.g.Check() != nil || f.g.Fault() != nil {
		t.Fatal("a new lock is closed")
	}
	f.g.Raise(protocol.CodeDaemonFaulted, "The broker found an internal fault.")
	f.g.Raise(protocol.CodeAuditFailed, "second")
	e := f.g.Check()
	if e == nil || e.Code != protocol.CodeDaemonFaulted {
		t.Fatalf("Check = %v", e)
	}
	ft := f.g.Fault()
	if ft == nil || ft.Code != protocol.CodeDaemonFaulted || ft.BeginPending || !ft.Since.Equal(f.clk.Now()) {
		t.Fatalf("Fault = %+v", ft)
	}
	if got := f.details(); len(got) != 1 || got[0] != audit.DetailFaultBegin {
		t.Fatalf("audit = %v", got)
	}
	if n := f.notices(); len(n) != 1 {
		t.Fatalf("notices = %d", len(n))
	}
}

// T26, T28 (part): a fault of the audit log. The begin entry stays
// pending; the notice names the fault, the lock and the next steps; the
// log names the fault. A cause that goes away does not lift the lock.
// Clear with the cause present keeps the lock and names the finding.
// Clear after the repair writes begin and end, in this sequence.
func TestRaiseWithBrokenAuditLog(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.store.FailAppends(nil)
	f.g.Raise(protocol.CodeAuditFailed, "The daemon cannot write the audit log.")

	ft := f.g.Inspect(ctx)
	if ft == nil || !ft.BeginPending || ft.CheckPassed == nil || *ft.CheckPassed || ft.Finding == "" {
		t.Fatalf("Inspect = %+v", ft)
	}
	ns := f.notices()
	if len(ns) != 1 {
		t.Fatalf("notices = %d", len(ns))
	}
	n := ns[0]
	if n.Kind != approval.NoticeDaemonFault || n.Code != protocol.CodeAuditFailed || n.Type != approval.TypeNotice || n.NoticeID == "" {
		t.Errorf("notice = %+v", n)
	}
	for _, want := range []string{"cannot write the audit log", "locked", "a38 doctor", "a38 fault clear"} {
		if !strings.Contains(n.Message, want) {
			t.Errorf("notice message %q lacks %q", n.Message, want)
		}
	}
	logs := f.logs.String()
	for _, want := range []string{"delivery is locked", "audit_failed", "stays pending"} {
		if !strings.Contains(logs, want) {
			t.Errorf("log lacks %q:\n%s", want, logs)
		}
	}

	// Clear with the cause present.
	ok, finding := f.g.Clear(ctx)
	if ok || !strings.Contains(finding, "audit log cannot be written") || f.g.Check() == nil {
		t.Fatalf("Clear with the cause = %v, %q", ok, finding)
	}

	// The cause goes away: the lock stays.
	f.store.Heal()
	if f.g.Check() == nil {
		t.Fatal("the lock ended by itself")
	}
	ft = f.g.Inspect(ctx)
	if ft == nil || ft.CheckPassed == nil || !*ft.CheckPassed || ft.Finding != "" {
		t.Fatalf("Inspect after the repair = %+v", ft)
	}
	if len(f.details()) != 0 {
		t.Fatal("Inspect wrote to the audit log")
	}

	ok, finding = f.g.Clear(ctx)
	if !ok || finding != "" || f.g.Check() != nil || f.g.Fault() != nil {
		t.Fatalf("Clear after the repair = %v, %q", ok, finding)
	}
	if got := f.details(); len(got) != 2 || got[0] != audit.DetailFaultBegin || got[1] != audit.DetailFaultEnd {
		t.Fatalf("audit = %v", got)
	}
	if ok, _ := f.g.Clear(ctx); !ok {
		t.Fatal("Clear on an open lock")
	}
	if len(f.notices()) != 1 {
		t.Fatal("Clear sent a notice")
	}

	// A second fault is a new fault with a new notice.
	f.g.Raise(protocol.CodeAuditFailed, "again")
	if f.g.Check() == nil || len(f.notices()) != 2 {
		t.Fatal("second fault")
	}
}

// endFails is a store without a probe that fails the end entry while
// fail is set.
type endFails struct {
	audit.Store
	fail atomic.Bool
}

func (s *endFails) Append(ctx context.Context, ev audit.Event) (audit.Entry, error) {
	if s.fail.Load() && ev.Result.Detail == audit.DetailFaultEnd {
		return audit.Entry{}, faultstore.ErrInjected
	}
	return s.Store.Append(ctx, ev)
}

// D48: Clear keeps the lock if the end entry cannot be written. A store
// without a probe has no result of the check in Inspect.
func TestClearFailsAtEndEntry(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	store := &endFails{Store: f.inner}
	store.fail.Store(true)
	g := f.lock(store)
	g.Raise(protocol.CodeDaemonFaulted, "The broker found an internal fault.")
	if ft := g.Inspect(ctx); ft == nil || ft.CheckPassed != nil {
		t.Fatalf("Inspect = %+v", ft)
	}
	if ok, finding := g.Clear(ctx); ok || finding == "" || g.Check() == nil {
		t.Fatalf("Clear = %v, %q", ok, finding)
	}
	store.fail.Store(false)
	if ok, _ := g.Clear(ctx); !ok || g.Check() != nil {
		t.Fatal("Clear after the repair")
	}
	if got := strings.Join(f.details(), ","); got != "fault_begin,fault_end" {
		t.Fatalf("audit = %v", got)
	}
}

// D48: Run writes a pending begin entry as soon as the audit log works.
// The lock stays.
func TestRunWritesPendingBegin(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.store.FailAppends(nil)
	f.g.Raise(protocol.CodeAuditFailed, "The daemon cannot write the audit log.")
	done := make(chan struct{})
	go func() { f.g.Run(ctx); close(done) }()
	waitTimer := func() {
		t.Helper()
		bctx, bcancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer bcancel()
		if err := f.clk.BlockUntil(bctx, 1); err != nil {
			t.Fatal(err)
		}
	}
	waitTimer()
	f.clk.Advance(fault.RetryInterval)
	waitTimer()
	if len(f.details()) != 0 || !f.g.Fault().BeginPending {
		t.Fatal("begin written while the audit log fails")
	}
	f.store.Heal()
	f.clk.Advance(fault.RetryInterval)
	waitTimer()
	if got := f.details(); len(got) != 1 || got[0] != audit.DetailFaultBegin {
		t.Fatalf("audit = %v", got)
	}
	if ft := f.g.Fault(); ft == nil || ft.BeginPending {
		t.Fatalf("fault = %+v", ft)
	}
	cancel()
	<-done
}

// T28 (part): the self-check at the start raises the fault if the audit
// log cannot be written, and passes else.
func TestSelfCheck(t *testing.T) {
	tests := []struct {
		name   string
		broken bool
	}{
		{"audit log works", false},
		{"audit log fails", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			if tt.broken {
				f.store.FailAppends(nil)
			}
			err := f.g.SelfCheck(context.Background())
			if (err != nil) != tt.broken || (f.g.Check() != nil) != tt.broken {
				t.Fatalf("SelfCheck = %v, locked = %v", err, f.g.Check() != nil)
			}
			if tt.broken {
				if ft := f.g.Fault(); ft.Code != protocol.CodeAuditFailed {
					t.Errorf("fault = %+v", ft)
				}
				if len(f.notices()) != 1 {
					t.Error("no notice")
				}
			}
		})
	}
}

// T26, T28 (part): many faults at the same time give one fault and one
// notice. A failing notice changes nothing but the log.
func TestConcurrentRaise(t *testing.T) {
	f := newFixture(t)
	f.store.FailAppends(nil)
	f.rec.FailNotify(errors.New("the UI is gone"))
	var wg sync.WaitGroup
	for range 64 {
		wg.Go(func() {
			f.g.Raise(protocol.CodeAuditFailed, "The daemon cannot write the audit log.")
			if f.g.Check() == nil {
				t.Error("open after Raise")
			}
		})
	}
	wg.Wait()
	if n := len(f.notices()); n != 1 {
		t.Fatalf("notices = %d", n)
	}
	if !strings.Contains(f.logs.String(), "cannot show the notice") {
		t.Error("the failed notice is not in the log")
	}
}

// A reason with control characters or an unknown code is cleaned, so that
// the status message stays valid.
func TestRaiseCleansInput(t *testing.T) {
	f := newFixture(t)
	f.g.Raise("no_such_code", "line one\nline two\x00")
	ft := f.g.Fault()
	if ft.Code != protocol.CodeDaemonFaulted || ft.Reason != "line one line two" {
		t.Fatalf("fault = %+v", ft)
	}
	res := &protocol.StatusResult{ID: "s1", Fault: ft}
	if err := res.Validate(); err != nil {
		t.Fatalf("status with the fault is not valid: %v", err)
	}
}
