package fakeui

import (
	"context"
	"errors"

	"github.com/seletz/passierschein/internal/approval"
)

// AutoApprove approves every entry of a request with the proposed
// lifetime.
type AutoApprove struct{}

// Approve approves all entries. A request with no entries is a denial. A
// request with no proposed lifetime is an error, except an approval of one
// command (exec way, prod), which has the lifetime 0.
func (AutoApprove) Approve(ctx context.Context, req approval.Request) (approval.Decision, error) {
	if err := ctx.Err(); err != nil {
		return approval.Decision{}, err
	}
	if len(req.Credentials) == 0 {
		return decision(req, approval.OutcomeDenied), nil
	}
	once := req.Exec != nil && req.Exec.Approval.Kind == approval.ExecApprovalOnce
	if req.Grant.TTLSeconds <= 0 && !once {
		return approval.Decision{}, errors.New("auto-approve: the request has no lifetime")
	}
	d := decision(req, approval.OutcomeApproved)
	for _, c := range req.Credentials {
		d.ApprovedCredentialIDs = append(d.ApprovedCredentialIDs, c.ID)
	}
	d.TTLSeconds = req.Grant.TTLSeconds
	return d, nil
}

// Notify ignores the notice.
func (AutoApprove) Notify(context.Context, approval.Notice) error { return nil }

// AutoDeny denies every request.
type AutoDeny struct{}

// Approve denies the request.
func (AutoDeny) Approve(ctx context.Context, req approval.Request) (approval.Decision, error) {
	if err := ctx.Err(); err != nil {
		return approval.Decision{}, err
	}
	return decision(req, approval.OutcomeDenied), nil
}

// Notify ignores the notice.
func (AutoDeny) Notify(context.Context, approval.Notice) error { return nil }

// decision returns a decision for req with the outcome and the policy
// "skipped".
func decision(req approval.Request, o approval.Outcome) approval.Decision {
	return approval.Decision{
		Version:               approval.Version,
		Type:                  approval.TypeDecision,
		RequestID:             req.RequestID,
		Outcome:               o,
		ApprovedCredentialIDs: []string{},
		Auth:                  &approval.AuthResult{Policy: "skipped", OK: true},
	}
}
