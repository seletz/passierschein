package fakeui_test

import (
	"context"
	"errors"
	"testing"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/approval/fakeui"
)

func TestRecorder(t *testing.T) {
	boom := errors.New("boom")
	r := fakeui.NewRecorder(fakeui.ReplyApproveAll(), fakeui.ReplyApprove(60, "c2"), fakeui.ReplyFail(boom))
	r.Enqueue(fakeui.ReplyTimeout())
	ctx := context.Background()

	d, err := r.Approve(ctx, sampleRequest())
	if err != nil || d.Outcome != approval.OutcomeApproved || len(d.ApprovedCredentialIDs) != 3 {
		t.Errorf("reply 1 = %+v, %v", d, err)
	}
	d, err = r.Approve(ctx, sampleRequest())
	if err != nil || d.TTLSeconds != 60 || len(d.ApprovedCredentialIDs) != 1 {
		t.Errorf("reply 2 = %+v, %v", d, err)
	}
	if _, err = r.Approve(ctx, sampleRequest()); !errors.Is(err, boom) {
		t.Errorf("reply 3 err = %v", err)
	}
	d, _ = r.Approve(ctx, sampleRequest())
	if d.Outcome != approval.OutcomeTimeout {
		t.Errorf("reply 4 = %+v", d)
	}
	d, _ = r.Approve(ctx, sampleRequest())
	if d.Outcome != approval.OutcomeDenied {
		t.Errorf("default reply = %+v", d)
	}
	if got := len(r.Requests()); got != 5 {
		t.Errorf("recorded %d requests, want 5", got)
	}

	r.FailNotify(boom)
	if err := r.Notify(ctx, approval.Notice{NoticeID: "n1"}); !errors.Is(err, boom) {
		t.Errorf("Notify err = %v", err)
	}
	r.FailNotify(nil)
	if err := r.Notify(ctx, approval.Notice{NoticeID: "n2"}); err != nil {
		t.Error(err)
	}
	if n := r.Notices(); len(n) != 2 || n[1].NoticeID != "n2" {
		t.Errorf("notices = %+v", n)
	}
}

func TestRecorderBlock(t *testing.T) {
	r := fakeui.NewRecorder(fakeui.ReplyBlock())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Approve(ctx, sampleRequest()); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v", err)
	}
}
