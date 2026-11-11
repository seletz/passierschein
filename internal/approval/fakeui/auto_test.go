package fakeui_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/approval/fakeui"
)

func TestAutoApprove(t *testing.T) {
	req := sampleRequest()
	d, err := fakeui.AutoApprove{}.Approve(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if d.Outcome != approval.OutcomeApproved || d.TTLSeconds != 3600 || d.RequestID != "req-1" {
		t.Errorf("decision = %+v", d)
	}
	if !reflect.DeepEqual(d.ApprovedCredentialIDs, []string{"c1", "c2", "c3"}) {
		t.Errorf("ids = %v", d.ApprovedCredentialIDs)
	}
	if d.Auth == nil || d.Auth.Policy != "skipped" {
		t.Errorf("auth = %+v", d.Auth)
	}
	if _, err := fakeui.ValidateDecision(req, d); err != nil {
		t.Errorf("own decision is invalid: %v", err)
	}
}

func TestAutoApproveEdgeCases(t *testing.T) {
	empty := sampleRequest()
	empty.Credentials = nil
	d, err := fakeui.AutoApprove{}.Approve(context.Background(), empty)
	if err != nil || d.Outcome != approval.OutcomeDenied {
		t.Errorf("no entries: %+v, %v", d, err)
	}
	noTTL := sampleRequest()
	noTTL.Grant.TTLSeconds = 0
	if _, err := (fakeui.AutoApprove{}).Approve(context.Background(), noTTL); err == nil {
		t.Error("no lifetime: want an error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (fakeui.AutoApprove{}).Approve(ctx, sampleRequest()); err == nil {
		t.Error("ended context: want an error")
	}
}

// T6 (part): auto-deny gives denied and no entries.
func TestAutoDeny(t *testing.T) {
	d, err := fakeui.AutoDeny{}.Approve(context.Background(), sampleRequest())
	if err != nil {
		t.Fatal(err)
	}
	if d.Outcome != approval.OutcomeDenied || len(d.ApprovedCredentialIDs) != 0 || d.TTLSeconds != 0 {
		t.Errorf("decision = %+v", d)
	}
	if err := (fakeui.AutoDeny{}).Notify(context.Background(), approval.Notice{}); err != nil {
		t.Error(err)
	}
}
