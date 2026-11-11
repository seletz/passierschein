package queue_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/approval/queue"
	"github.com/seletz/passierschein/internal/clock"
)

// gateUI blocks in Approve until the test releases it. It counts the calls
// that run at the same time.
type gateUI struct {
	mu      sync.Mutex
	running int
	maxRun  int
	started chan string
	release chan struct{}
	seenTTL atomic.Int64
	notices atomic.Int64
}

func newGate() *gateUI {
	return &gateUI{started: make(chan string, 10), release: make(chan struct{})}
}

func (g *gateUI) Approve(ctx context.Context, req approval.Request) (approval.Decision, error) {
	g.mu.Lock()
	g.running++
	g.maxRun = max(g.maxRun, g.running)
	g.mu.Unlock()
	defer func() { g.mu.Lock(); g.running--; g.mu.Unlock() }()
	g.seenTTL.Store(int64(req.TimeoutSeconds))
	g.started <- req.RequestID
	select {
	case <-g.release:
		return approval.Decision{Version: 1, Type: approval.TypeDecision, RequestID: req.RequestID, Outcome: approval.OutcomeDenied}, nil
	case <-ctx.Done():
		return approval.Decision{}, ctx.Err()
	}
}

func (g *gateUI) Notify(context.Context, approval.Notice) error { g.notices.Add(1); return nil }

func waitStarted(t *testing.T, g *gateUI) string {
	t.Helper()
	select {
	case id := <-g.started:
		return id
	case <-time.After(10 * time.Second):
		t.Fatal("no request started")
		return ""
	}
}

// D30: at most one request at a time, in the order of the calls.
func TestQueueOneAtATime(t *testing.T) {
	g := newGate()
	clk := clock.NewFake(time.Unix(0, 0))
	q := queue.New(g, clk, 0)
	var wg sync.WaitGroup
	results := make(chan string, 3)
	call := func(id string) {
		defer wg.Done()
		d, err := q.Approve(context.Background(), approval.Request{RequestID: id})
		if err != nil {
			t.Error(err)
			return
		}
		results <- d.RequestID
	}
	wg.Add(1)
	go call("r1")
	if id := waitStarted(t, g); id != "r1" {
		t.Fatalf("first = %s", id)
	}
	wg.Add(1)
	go call("r2")
	// r2 must wait while r1 is open.
	select {
	case id := <-g.started:
		t.Fatalf("%s started while r1 was open", id)
	case <-time.After(100 * time.Millisecond):
	}
	g.release <- struct{}{}
	if id := waitStarted(t, g); id != "r2" {
		t.Fatalf("second = %s", id)
	}
	g.release <- struct{}{}
	wg.Wait()
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.maxRun != 1 {
		t.Errorf("max concurrent = %d, want 1", g.maxRun)
	}
	if got := g.seenTTL.Load(); got != 120 {
		t.Errorf("TimeoutSeconds seen by the UI = %d, want 120", got)
	}
}

// T6 (part), D30: a request without a decision ends with timeout after two
// minutes. The wait in the queue does not count.
func TestQueueTimeout(t *testing.T) {
	g := newGate()
	clk := clock.NewFake(time.Unix(0, 0))
	q := queue.New(g, clk, 0)
	type res struct {
		d   approval.Decision
		err error
	}
	ch := make(chan res, 2)
	for _, id := range []string{"r1", "r2"} {
		go func() {
			d, err := q.Approve(context.Background(), approval.Request{RequestID: id})
			ch <- res{d, err}
		}()
		if id == "r1" {
			waitStarted(t, g)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := clk.BlockUntil(ctx, 1); err != nil {
		t.Fatal(err)
	}
	clk.Advance(approval.DefaultTimeout - time.Second)
	select {
	case <-ch:
		t.Fatal("ended too early")
	case <-time.After(50 * time.Millisecond):
	}
	clk.Advance(time.Second)
	r := <-ch
	if r.err != nil || r.d.Outcome != approval.OutcomeTimeout || len(r.d.ApprovedCredentialIDs) != 0 || r.d.RequestID == "" {
		t.Fatalf("result = %+v, %v", r.d, r.err)
	}
	// r2 now runs and has its own full wait time.
	if id := waitStarted(t, g); id != "r2" && id != "r1" {
		t.Fatalf("unexpected %s", id)
	}
	if err := clk.BlockUntil(ctx, 1); err != nil {
		t.Fatal(err)
	}
	clk.Advance(approval.DefaultTimeout)
	if r := <-ch; r.d.Outcome != approval.OutcomeTimeout {
		t.Errorf("second result = %+v", r.d)
	}
}

func TestQueueContextEndsWhileWaiting(t *testing.T) {
	g := newGate()
	q := queue.New(g, clock.NewFake(time.Unix(0, 0)), 0)
	go func() { _, _ = q.Approve(context.Background(), approval.Request{RequestID: "r1"}) }()
	waitStarted(t, g)
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { _, err := q.Approve(ctx, approval.Request{RequestID: "r2"}); errc <- err }()
	cancel()
	if err := <-errc; err == nil {
		t.Error("want an error")
	}
	g.release <- struct{}{}
}

// A notice does not wait for an open request (D48).
func TestQueueNotifyDoesNotWait(t *testing.T) {
	g := newGate()
	q := queue.New(g, clock.NewFake(time.Unix(0, 0)), time.Minute)
	if q.Timeout() != time.Minute {
		t.Errorf("Timeout = %v", q.Timeout())
	}
	go func() { _, _ = q.Approve(context.Background(), approval.Request{RequestID: "r1"}) }()
	waitStarted(t, g)
	if err := q.Notify(context.Background(), approval.Notice{}); err != nil {
		t.Fatal(err)
	}
	if g.notices.Load() != 1 {
		t.Error("the UI got no notice")
	}
	g.release <- struct{}{}
}
