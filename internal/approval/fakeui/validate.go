package fakeui

import "github.com/seletz/passierschein/internal/approval"

// ValidateDecision checks a decision of a UI against its request (T4,
// T23). It calls [approval.ValidateDecision], which holds the rules and is
// shared with the approval app UI.
func ValidateDecision(req approval.Request, d approval.Decision) (approval.Decision, error) {
	return approval.ValidateDecision(req, d)
}
