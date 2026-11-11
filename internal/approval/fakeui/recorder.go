package fakeui

import (
	"context"
	"sync"

	"github.com/seletz/passierschein/internal/approval"
)

// Reply is a scripted answer of a [Recorder] to one request.
type Reply func(ctx context.Context, req approval.Request) (approval.Decision, error)

// Recorder is a UI for in-process tests. It records the requests and the
// notices and answers with scripted replies. It is safe for concurrent use.
//
// The recorder returns its replies as they are. It does not validate them,
// so that a test of the broker can feed it an invalid decision.
type Recorder struct {
	mu       sync.Mutex
	requests []approval.Request
	notices  []approval.Notice
	replies  []Reply
	notifyFn func(approval.Notice) error
}

// NewRecorder returns a recorder with the replies, used in this sequence.
// After the last reply the recorder denies.
func NewRecorder(replies ...Reply) *Recorder {
	return &Recorder{replies: replies}
}

// Enqueue adds replies at the end of the sequence.
func (r *Recorder) Enqueue(replies ...Reply) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.replies = append(r.replies, replies...)
}

// FailNotify makes Notify return err after it records the notice. A nil
// err resets this.
func (r *Recorder) FailNotify(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err == nil {
		r.notifyFn = nil
		return
	}
	r.notifyFn = func(approval.Notice) error { return err }
}

// Approve records the request and runs the next reply.
func (r *Recorder) Approve(ctx context.Context, req approval.Request) (approval.Decision, error) {
	r.mu.Lock()
	r.requests = append(r.requests, req)
	reply := ReplyDeny()
	if len(r.replies) > 0 {
		reply, r.replies = r.replies[0], r.replies[1:]
	}
	r.mu.Unlock()
	return reply(ctx, req)
}

// Notify records the notice.
func (r *Recorder) Notify(_ context.Context, n approval.Notice) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notices = append(r.notices, n)
	if r.notifyFn != nil {
		return r.notifyFn(n)
	}
	return nil
}

// Requests returns a copy of the recorded requests.
func (r *Recorder) Requests() []approval.Request {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]approval.Request(nil), r.requests...)
}

// Notices returns a copy of the recorded notices.
func (r *Recorder) Notices() []approval.Notice {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]approval.Notice(nil), r.notices...)
}

// ReplyApproveAll approves all entries with the proposed lifetime.
func ReplyApproveAll() Reply {
	return func(ctx context.Context, req approval.Request) (approval.Decision, error) {
		return AutoApprove{}.Approve(ctx, req)
	}
}

// ReplyApprove approves the entries with the IDs and the lifetime. It does
// not check them against the request.
func ReplyApprove(ttlSeconds int64, ids ...string) Reply {
	return func(_ context.Context, req approval.Request) (approval.Decision, error) {
		d := decision(req, approval.OutcomeApproved)
		d.ApprovedCredentialIDs = append([]string{}, ids...)
		d.TTLSeconds = ttlSeconds
		return d, nil
	}
}

// ReplyDeny denies the request.
func ReplyDeny() Reply {
	return func(_ context.Context, req approval.Request) (approval.Decision, error) {
		return decision(req, approval.OutcomeDenied), nil
	}
}

// ReplyTimeout answers with the outcome timeout.
func ReplyTimeout() Reply {
	return func(_ context.Context, req approval.Request) (approval.Decision, error) {
		return decision(req, approval.OutcomeTimeout), nil
	}
}

// ReplyFail makes the UI fail with err.
func ReplyFail(err error) Reply {
	return func(context.Context, approval.Request) (approval.Decision, error) {
		return approval.Decision{}, err
	}
}

// ReplyBlock never answers. It returns the error of ctx when ctx ends.
func ReplyBlock() Reply {
	return func(ctx context.Context, _ approval.Request) (approval.Decision, error) {
		<-ctx.Done()
		return approval.Decision{}, ctx.Err()
	}
}

// ReplyFunc adapts a function that has the decision only.
func ReplyFunc(f func(req approval.Request) approval.Decision) Reply {
	return func(_ context.Context, req approval.Request) (approval.Decision, error) {
		return f(req), nil
	}
}
