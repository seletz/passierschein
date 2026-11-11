package broker

import (
	"github.com/seletz/passierschein/internal/approval"
)

// authSkipped is the policy that a faked UI reports.
const authSkipped = "skipped"

// verdict is the result of the check of a decision.
type verdict struct {
	// outcome is approved, denied or timeout. It is empty for an invalid
	// decision; finding then says why.
	outcome    approval.Outcome
	ids        []string
	ttlSeconds int64
	finding    string
}

// checkDecision checks a decision of the UI against its request. The
// broker trusts no UI (D32, T23): it checks each decision itself, also if
// the UI checked it before.
//
// An approval is valid only if:
//   - the version, the type and the request ID fit the request,
//   - the authentication did not fail; if authRequired is set, the
//     authentication is present, passed and not "skipped",
//   - each approved ID is an entry of the request, and no ID repeats,
//   - the lifetime is more than zero and not more than the proposed
//     lifetime of the configuration.
//
// An approval with no approved ID is a denial. The outcome error and each
// unknown outcome are invalid. An invalid decision creates no grant.
func checkDecision(req approval.Request, d approval.Decision, authRequired bool) verdict {
	switch {
	case d.Version != approval.Version:
		return invalidDecision("the decision has an unsupported version")
	case d.Type != approval.TypeDecision:
		return invalidDecision("the decision has an unexpected type")
	case d.RequestID != req.RequestID:
		return invalidDecision("the decision answers a different request")
	}
	switch d.Outcome {
	case approval.OutcomeDenied, approval.OutcomeTimeout:
		return verdict{outcome: d.Outcome}
	case approval.OutcomeApproved:
		return checkApproval(req, d, authRequired)
	case approval.OutcomeError:
		return invalidDecision("the UI reported an error")
	default:
		return invalidDecision("the decision has an unknown outcome")
	}
}

// checkApproval checks a decision with the outcome approved.
func checkApproval(req approval.Request, d approval.Decision, authRequired bool) verdict {
	if d.Auth != nil && !d.Auth.OK {
		return invalidDecision("the authentication of the human failed")
	}
	if authRequired && (d.Auth == nil || d.Auth.Policy == authSkipped) {
		return invalidDecision("the decision has no authentication of the human")
	}
	if len(d.ApprovedCredentialIDs) == 0 {
		return verdict{outcome: approval.OutcomeDenied}
	}
	known := make(map[string]bool, len(req.Credentials))
	for _, c := range req.Credentials {
		known[c.ID] = true
	}
	seen := make(map[string]bool, len(d.ApprovedCredentialIDs))
	for _, id := range d.ApprovedCredentialIDs {
		if !known[id] {
			return invalidDecision("the decision approves an entry that is not in the request")
		}
		if seen[id] {
			return invalidDecision("the decision approves an entry twice")
		}
		seen[id] = true
	}
	if d.TTLSeconds <= 0 || d.TTLSeconds > req.Grant.TTLSeconds {
		return invalidDecision("the lifetime of the decision is not valid")
	}
	return verdict{
		outcome:    approval.OutcomeApproved,
		ids:        append([]string(nil), d.ApprovedCredentialIDs...),
		ttlSeconds: d.TTLSeconds,
	}
}

// invalidDecision returns the verdict for an invalid decision.
func invalidDecision(finding string) verdict { return verdict{finding: finding} }
