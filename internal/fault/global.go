package fault

import (
	"context"
	"crypto/rand"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/clock"
	"github.com/seletz/passierschein/internal/protocol"
)

// RetryInterval is the time between two tries of [Global.Run] to write a
// pending entry for the begin of the lock.
const RetryInterval = 30 * time.Second

// maxText is the maximum length in characters of a reason or a finding.
// It is below the limit of the wire protocol.
const maxText = 500

// Prober is an audit store that can check that it can write an entry,
// without a write of an entry. The SQLite store of the daemon is a Prober.
type Prober interface {
	// Probe returns nil if the store can write an entry now.
	Probe(ctx context.Context) error
}

// Options configures a [Global] lock.
type Options struct {
	// Store is the audit log. The lock writes the begin and the end of
	// the lock to it, and checks it before it lifts the lock.
	Store audit.Store
	// UI shows the notice of a fault to the human. Nil means no notice.
	UI approval.UI
	// Clock gives the time of a fault. Nil means the system clock.
	Clock clock.Clock
	// Logger is the log of the daemon. Nil means a logger that discards.
	Logger *slog.Logger
}

// Global is the global delivery lock of the daemon (D48). It is safe for
// concurrent use.
//
// Rules:
//   - The first [Global.Raise] sets the fault. A Raise while a fault is
//     set keeps the first fault and sends no further notice.
//   - Raise sends exactly one notice to the UI for each fault (D48, T26).
//     The notice goes out in its own goroutine: a slow UI never holds up
//     the daemon.
//   - Raise writes the begin of the lock to the audit log: an event of the
//     type error with the detail fault_begin (G22). If the audit log
//     cannot be written, the entry stays pending and the log says so.
//     [Global.Run] and [Global.Clear] write it as soon as possible.
//   - Nothing lifts the lock except [Global.Clear]. A cause that goes away
//     by itself does not lift it.
//   - Clear lifts the lock only if the check of the cause passes: the
//     store probe passes, the pending begin entry is written, and the end
//     entry (detail fault_end) is written.
type Global struct {
	store audit.Store
	ui    approval.UI
	clock clock.Clock
	log   *slog.Logger

	// wmu serialises the writes of the begin and the end entry, and
	// Clear. The lock never holds wmu while it waits for mu longer than
	// a write.
	wmu sync.Mutex

	// mu guards the fields below.
	mu           sync.Mutex
	fault        *protocol.Fault
	beginPending bool
	// raises counts each call of Raise. Clear compares it, so that a
	// fault that occurs during the check keeps the lock.
	raises uint64

	notices sync.WaitGroup
}

var _ Lock = (*Global)(nil)

// New returns an open lock.
func New(o Options) *Global {
	g := &Global{store: o.Store, ui: o.UI, clock: o.Clock, log: o.Logger}
	if g.clock == nil {
		g.clock = clock.System{}
	}
	if g.log == nil {
		g.log = slog.New(slog.DiscardHandler)
	}
	return g
}

// Check returns nil while delivery is open, else an error with the code
// daemon_faulted.
func (g *Global) Check() *protocol.Error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.fault == nil {
		return nil
	}
	return protocol.Errorf(protocol.CodeDaemonFaulted,
		"the daemon has a fault and delivers nothing; the human must run a38 doctor and a38 fault clear")
}

// Raise sets the fault, logs it, sends one notice and writes the begin
// entry to the audit log. An unknown code is stored as daemon_faulted.
func (g *Global) Raise(code protocol.Code, reason string) {
	if !code.Valid() {
		code = protocol.CodeDaemonFaulted
	}
	reason = cleanText(reason)
	if reason == "" {
		reason = "The daemon found a fault."
	}
	g.mu.Lock()
	g.raises++
	if g.fault != nil {
		g.mu.Unlock()
		return
	}
	f := protocol.Fault{Code: code, Reason: reason, Since: g.clock.Now()}
	g.fault = &f
	g.beginPending = true
	g.mu.Unlock()

	g.log.Error("delivery is locked for all sessions", "code", string(code), "reason", reason)
	g.notify(f)
	g.wmu.Lock()
	defer g.wmu.Unlock()
	_ = g.writeBeginLocked(context.Background())
}

// Fault returns a copy of the current fault, or nil.
func (g *Global) Fault() *protocol.Fault {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.fault == nil {
		return nil
	}
	f := *g.fault
	f.BeginPending = g.beginPending
	return &f
}

// Inspect returns the current fault with the result of the store probe,
// or nil. CheckPassed is nil if the store is not a [Prober].
func (g *Global) Inspect(ctx context.Context) *protocol.Fault {
	f := g.Fault()
	if f == nil {
		return nil
	}
	p, ok := g.store.(Prober)
	if !ok {
		return f
	}
	err := p.Probe(ctx)
	passed := err == nil
	f.CheckPassed = &passed
	if err != nil {
		f.Finding = checkFinding(err)
	}
	return f
}

// Clear lifts the lock if the check of the cause passes. See [Global].
func (g *Global) Clear(ctx context.Context) (bool, string) {
	g.wmu.Lock()
	defer g.wmu.Unlock()
	g.mu.Lock()
	f, raises := g.fault, g.raises
	g.mu.Unlock()
	if f == nil {
		return true, ""
	}
	if p, ok := g.store.(Prober); ok {
		if err := p.Probe(ctx); err != nil {
			g.log.Warn("the lock stays: the check of the cause failed", "code", string(f.Code), "error", err)
			return false, checkFinding(err)
		}
	}
	if err := g.writeBeginLocked(ctx); err != nil {
		return false, checkFinding(err)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.raises != raises {
		g.log.Warn("the lock stays: a new fault occurred during the check")
		return false, "A new fault occurred during the check. Run a38 doctor, then a38 fault clear again."
	}
	if _, err := g.store.Append(ctx, audit.Event{
		Type:   audit.EventError,
		Result: audit.Result{Code: f.Code, Detail: audit.DetailFaultEnd},
	}); err != nil {
		g.log.Warn("the lock stays: cannot write the end of the lock to the audit log", "error", err)
		return false, checkFinding(err)
	}
	g.fault = nil
	g.log.Info("the human lifted the lock; delivery is open", "code", string(f.Code))
	return true, ""
}

// Run writes a pending begin entry every [RetryInterval] until ctx ends.
func (g *Global) Run(ctx context.Context) {
	for {
		t := g.clock.NewTimer(RetryInterval)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C():
			g.wmu.Lock()
			_ = g.writeBeginLocked(ctx)
			g.wmu.Unlock()
		}
	}
}

// SelfCheck checks at the start of the daemon that the audit log can be
// written. If not, it raises the fault audit_failed: a restart with the
// cause still present leads into the lock again (D48, T28). It returns the
// error of the check.
func (g *Global) SelfCheck(ctx context.Context) error {
	p, ok := g.store.(Prober)
	if !ok {
		return nil
	}
	err := p.Probe(ctx)
	if err != nil {
		g.log.Error("the self-check at the start cannot write the audit log", "error", err)
		g.Raise(protocol.CodeAuditFailed, "The self-check at the start of the daemon cannot write the audit log.")
	}
	return err
}

// Wait waits until each notice went to the UI.
func (g *Global) Wait() { g.notices.Wait() }

// writeBeginLocked writes the pending begin entry. The caller holds wmu.
// A failure leaves the entry pending and goes to the log.
func (g *Global) writeBeginLocked(ctx context.Context) error {
	g.mu.Lock()
	if !g.beginPending || g.fault == nil {
		g.mu.Unlock()
		return nil
	}
	f := *g.fault
	g.mu.Unlock()
	_, err := g.store.Append(ctx, audit.Event{
		Type:   audit.EventError,
		Result: audit.Result{Code: f.Code, Detail: audit.DetailFaultBegin},
	})
	if err != nil {
		g.log.Warn("cannot write the begin of the lock to the audit log; the entry stays pending",
			"code", string(f.Code), "since", f.Since, "error", err)
		return err
	}
	g.mu.Lock()
	g.beginPending = false
	g.mu.Unlock()
	g.log.Info("the begin of the lock is in the audit log", "code", string(f.Code), "since", f.Since)
	return nil
}

// notify sends the notice of the fault in its own goroutine.
func (g *Global) notify(f protocol.Fault) {
	if g.ui == nil {
		return
	}
	n := approval.Notice{
		Version:  approval.Version,
		Type:     approval.TypeNotice,
		NoticeID: "notice-" + strings.ToLower(rand.Text()),
		Kind:     approval.NoticeDaemonFault,
		Code:     f.Code,
		Message: f.Reason + " Delivery is locked for all sessions. " +
			"Run a38 doctor and remove the cause, then lift the lock with a38 fault clear or restart a38 serve.",
		OccurredAt: f.Since,
	}
	g.notices.Add(1)
	go func() {
		defer g.notices.Done()
		if err := g.ui.Notify(context.Background(), n); err != nil {
			g.log.Warn("cannot show the notice", "kind", string(n.Kind), "error", err)
		}
	}()
}

// checkFinding returns the finding of a failed check of the cause.
func checkFinding(err error) string {
	return cleanText("The audit log cannot be written: " + err.Error())
}

// cleanText replaces each control character with a space and cuts the
// text to maxText characters, so that it is valid on the wire.
func cleanText(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		if n == maxText {
			break
		}
		if protocol.IsControl(r) {
			r = ' '
		}
		b.WriteRune(r)
		n++
	}
	return strings.TrimSpace(b.String())
}
