// Package queue makes sure that the daemon shows at most one approval
// request at a time, and that a request without a decision ends after the
// wait time with the outcome timeout (D30).
//
// A [Queue] wraps a [approval.UI]. Requests wait in the order of their
// calls. The wait time starts when the request is shown, not when it
// enters the queue.
package queue

import (
	"context"
	"time"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/clock"
)

// Queue is a [approval.UI] that serialises the requests to the UI behind
// it.
type Queue struct {
	ui      approval.UI
	clock   clock.Clock
	timeout time.Duration
	// slots is a semaphore with one place. A channel gives a fair order
	// and lets a waiting caller leave when its context ends.
	slots chan struct{}
}

// New returns a Queue. A zero timeout means [approval.DefaultTimeout]. A
// nil clock means the system clock.
func New(ui approval.UI, clk clock.Clock, timeout time.Duration) *Queue {
	if timeout <= 0 {
		timeout = approval.DefaultTimeout
	}
	// The approval app accepts 1 to 3600 seconds only. A value outside the
	// range would end each request with ui_failed, so the queue clamps it.
	timeout = min(max(timeout, approval.MinTimeoutSeconds*time.Second), approval.MaxTimeoutSeconds*time.Second)
	if clk == nil {
		clk = clock.System{}
	}
	return &Queue{ui: ui, clock: clk, timeout: timeout, slots: make(chan struct{}, 1)}
}

// Timeout returns the wait time.
func (q *Queue) Timeout() time.Duration { return q.timeout }

// Approve waits for its turn and then shows the request. It sets
// TimeoutSeconds of the request to the wait time. If the UI gives no
// decision in time, Approve cancels the context of the UI and returns the
// outcome timeout. If ctx ends first, Approve returns its error.
//
// The queue frees its place only after the UI call returned. A UI must
// therefore end its call when its context ends.
func (q *Queue) Approve(ctx context.Context, req approval.Request) (approval.Decision, error) {
	select {
	case q.slots <- struct{}{}:
	case <-ctx.Done():
		return approval.Decision{}, ctx.Err()
	}
	defer func() { <-q.slots }()

	timer := q.clock.NewTimer(q.timeout)
	defer timer.Stop()
	req.TimeoutSeconds = int(q.timeout / time.Second)

	uiCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	type result struct {
		d   approval.Decision
		err error
	}
	done := make(chan result, 1)
	go func() {
		d, err := q.ui.Approve(uiCtx, req)
		done <- result{d, err}
	}()

	select {
	case r := <-done:
		return r.d, r.err
	case <-timer.C():
		cancel()
		<-done
		return approval.Decision{
			Version:               approval.Version,
			Type:                  approval.TypeDecision,
			RequestID:             req.RequestID,
			Outcome:               approval.OutcomeTimeout,
			ApprovedCredentialIDs: []string{},
			ElapsedMS:             q.timeout.Milliseconds(),
		}, nil
	case <-ctx.Done():
		cancel()
		<-done
		return approval.Decision{}, ctx.Err()
	}
}

// Notify passes the notice to the UI at once. A notice does not wait in
// the queue: a fault must reach the human also while a request is open.
func (q *Queue) Notify(ctx context.Context, n approval.Notice) error {
	return q.ui.Notify(ctx, n)
}
