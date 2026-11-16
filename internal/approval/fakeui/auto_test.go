package fakeui_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/approval/fakeui"
	"github.com/seletz/passierschein/internal/config"
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

// T38 (part, approval request): auto-approve gives a valid decision for a
// command: a grant with the configured lifetime, or "only this command"
// with lifetime 0 in prod.
func TestAutoApproveExec(t *testing.T) {
	for _, env := range []config.Environment{config.EnvDev, config.EnvTest, config.EnvProd} {
		t.Run(string(env), func(t *testing.T) {
			req := sampleRequest()
			for i := range req.Credentials {
				req.Credentials[i].Environment = env
			}
			req.Exec = &approval.ExecCommand{
				Environment: env, Program: "/usr/bin/true", Cwd: "/tmp",
				Env: []approval.ExecVariable{
					{Name: "A", CredentialID: "c1"}, {Name: "B", CredentialID: "c2"}, {Name: "C", CredentialID: "c3"},
				},
				Approval: approval.ExecApproval{Kind: approval.ExecApprovalGrant, Binding: approval.BindingProgram,
					LifetimeChoicesSeconds: approval.ExecLifetimeChoices(3600)},
			}
			wantTTL := int64(3600)
			if env == config.EnvProd {
				req.Grant.TTLSeconds = 0
				req.Exec.Approval = approval.ExecApproval{Kind: approval.ExecApprovalOnce, Binding: approval.BindingCommand}
				wantTTL = 0
			}
			if err := req.Validate(); err != nil {
				t.Fatalf("test request: %v", err)
			}
			d, err := fakeui.AutoApprove{}.Approve(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			if d.Outcome != approval.OutcomeApproved || d.TTLSeconds != wantTTL || len(d.ApprovedCredentialIDs) != 3 {
				t.Errorf("decision = %+v", d)
			}
			if _, err := fakeui.ValidateDecision(req, d); err != nil {
				t.Errorf("own decision is invalid: %v", err)
			}
		})
	}
}
