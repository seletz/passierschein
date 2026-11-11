package fakeui_test

import (
	"testing"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/approval/fakeui"
)

// T4 (part), T23 (part): the daemon does not trust a decision.
func TestValidateDecision(t *testing.T) {
	ok := func() approval.Decision {
		return approval.Decision{
			Version: 1, Type: approval.TypeDecision, RequestID: "req-1",
			Outcome: approval.OutcomeApproved, ApprovedCredentialIDs: []string{"c1", "c2"}, TTLSeconds: 600,
		}
	}
	tests := []struct {
		name        string
		mutate      func(*approval.Decision)
		wantErr     bool
		wantOutcome approval.Outcome
	}{
		{name: "valid subset", mutate: func(*approval.Decision) {}, wantOutcome: approval.OutcomeApproved},
		{name: "full lifetime", mutate: func(d *approval.Decision) { d.TTLSeconds = 3600 }, wantOutcome: approval.OutcomeApproved},
		{name: "entry not in request", mutate: func(d *approval.Decision) { d.ApprovedCredentialIDs = []string{"c1", "x9"} }, wantErr: true},
		{name: "entry twice", mutate: func(d *approval.Decision) { d.ApprovedCredentialIDs = []string{"c1", "c1"} }, wantErr: true},
		{name: "longer than proposed", mutate: func(d *approval.Decision) { d.TTLSeconds = 3601 }, wantErr: true},
		{name: "zero lifetime", mutate: func(d *approval.Decision) { d.TTLSeconds = 0 }, wantErr: true},
		{name: "negative lifetime", mutate: func(d *approval.Decision) { d.TTLSeconds = -5 }, wantErr: true},
		{name: "no entry is a denial", mutate: func(d *approval.Decision) { d.ApprovedCredentialIDs = nil }, wantOutcome: approval.OutcomeDenied},
		{name: "other request", mutate: func(d *approval.Decision) { d.RequestID = "req-2" }, wantErr: true},
		{name: "no request id", mutate: func(d *approval.Decision) { d.RequestID = "" }, wantErr: true},
		{name: "wrong version", mutate: func(d *approval.Decision) { d.Version = 2 }, wantErr: true},
		{name: "wrong type", mutate: func(d *approval.Decision) { d.Type = "notice" }, wantErr: true},
		{name: "unknown outcome", mutate: func(d *approval.Decision) { d.Outcome = "maybe" }, wantErr: true},
		{name: "outcome error", mutate: func(d *approval.Decision) { d.Outcome = approval.OutcomeError }, wantErr: true},
		{name: "denied drops entries", mutate: func(d *approval.Decision) { d.Outcome = approval.OutcomeDenied }, wantOutcome: approval.OutcomeDenied},
		{name: "timeout", mutate: func(d *approval.Decision) { d.Outcome = approval.OutcomeTimeout }, wantOutcome: approval.OutcomeTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := ok()
			tt.mutate(&d)
			got, err := fakeui.ValidateDecision(sampleRequest(), d)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, want error %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.Outcome != tt.wantOutcome {
				t.Errorf("outcome = %s, want %s", got.Outcome, tt.wantOutcome)
			}
			if got.Outcome != approval.OutcomeApproved && (len(got.ApprovedCredentialIDs) != 0 || got.TTLSeconds != 0) {
				t.Errorf("non-approval keeps entries: %+v", got)
			}
		})
	}
}
