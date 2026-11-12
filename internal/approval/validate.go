package approval

import "fmt"

// ValidateDecision checks a decision of a UI against its request (T4,
// T23). It returns the decision to use, or an error.
//
// A decision with the outcome approved is valid only if the request ID
// matches, the approved IDs are not empty, unique and part of the request,
// and the lifetime is more than zero and not more than the proposed
// lifetime. An approval with no approved ID becomes a denial. The outcome
// error is an error. A decision with another outcome carries no entries.
func ValidateDecision(req Request, d Decision) (Decision, error) {
	if d.Version != Version {
		return Decision{}, fmt.Errorf("unsupported decision version %d", d.Version)
	}
	if d.Type != TypeDecision {
		return Decision{}, fmt.Errorf("unexpected message type %q", d.Type)
	}
	if d.RequestID != req.RequestID {
		return Decision{}, fmt.Errorf("the decision answers another request")
	}
	switch d.Outcome {
	case OutcomeDenied, OutcomeTimeout:
		d.ApprovedCredentialIDs = []string{}
		d.TTLSeconds = 0
		return d, nil
	case OutcomeError:
		msg := "no details"
		if d.Error != nil {
			msg = d.Error.Code
		}
		return Decision{}, fmt.Errorf("the UI reported an error: %s", msg)
	case OutcomeApproved:
	default:
		return Decision{}, fmt.Errorf("unknown outcome %q", d.Outcome)
	}
	if len(d.ApprovedCredentialIDs) == 0 {
		d.Outcome = OutcomeDenied
		d.ApprovedCredentialIDs = []string{}
		d.TTLSeconds = 0
		return d, nil
	}
	known := make(map[string]bool, len(req.Credentials))
	for _, c := range req.Credentials {
		known[c.ID] = true
	}
	seen := make(map[string]bool, len(d.ApprovedCredentialIDs))
	for _, id := range d.ApprovedCredentialIDs {
		if !known[id] {
			return Decision{}, fmt.Errorf("the decision approves an entry that is not in the request")
		}
		if seen[id] {
			return Decision{}, fmt.Errorf("the decision approves an entry twice")
		}
		seen[id] = true
	}
	if d.TTLSeconds <= 0 || d.TTLSeconds > req.Grant.TTLSeconds {
		return Decision{}, fmt.Errorf("the lifetime %d is not valid", d.TTLSeconds)
	}
	return d, nil
}
