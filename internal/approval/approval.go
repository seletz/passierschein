// Package approval defines the interface between the daemon and the
// approval UI: the approval request, the decision and the notice.
//
// The daemon gives a [Request] to a [UI] and receives a [Decision]. The
// request contains names only; it never contains a secret value. The JSON
// form of Request and Decision is the protocol of the approval app: the
// daemon writes the request to the standard input of the app, and the app
// writes the decision to its standard output. The UI variants for tests
// (auto-approve, auto-deny, script) use the same types.
//
// The package holds types and the interface only. The implementations, the
// queue (one approval request at a time) and the timeout live in
// sub-packages.
package approval

import (
	"context"
	"time"

	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// Version is the version of the JSON schema of request, decision and
// notice.
const Version = 1

// DefaultTimeout is the time after which an approval request without a
// decision expires. The expiry counts as a denial with the code timeout.
const DefaultTimeout = 2 * time.Minute

// The range of Request.TimeoutSeconds that the approval app accepts. The app
// has the same limits in Swift (app/Sources/A38ApprovalCore, the check of
// timeout_seconds). A request outside the range is no approval (appui).
const (
	// MinTimeoutSeconds is the smallest valid TimeoutSeconds.
	MinTimeoutSeconds = 1
	// MaxTimeoutSeconds is the largest valid TimeoutSeconds.
	MaxTimeoutSeconds = 3600
)

// The values of the field "type".
const (
	// TypeRequest is the type of a Request.
	TypeRequest = "approval_request"
	// TypeDecision is the type of a Decision.
	TypeDecision = "approval_decision"
	// TypeNotice is the type of a Notice.
	TypeNotice = "notice"
)

// Mode names a UI implementation. It is the value of the flag "--ui" of
// "a38 serve".
type Mode string

// The UI modes.
const (
	// ModeApp starts the approval app. It is the only mode that the daemon
	// accepts together with a real backend.
	ModeApp Mode = "app"
	// ModeAutoApprove approves each request in full. Fake backend only.
	ModeAutoApprove Mode = "auto-approve"
	// ModeAutoDeny denies each request. Fake backend only.
	ModeAutoDeny Mode = "auto-deny"
	// ModeScript starts a script that gets the request on its standard
	// input and writes the decision. Fake backend only.
	ModeScript Mode = "script"
)

// Faked reports whether the mode gives a decision without the human. The
// daemon does not start with a faked mode and a real backend.
func (m Mode) Faked() bool { return m != ModeApp }

// Request is one approval request. It is made from one bundle of one
// session.
type Request struct {
	// Version is the version of the schema. It is always [Version].
	Version int `json:"version"`
	// Type is always [TypeRequest].
	Type string `json:"type"`
	// RequestID identifies the request. The decision repeats it.
	RequestID string `json:"request_id"`
	// TimeoutSeconds is the time until the request expires.
	TimeoutSeconds int `json:"timeout_seconds"`
	// Verified holds the facts that the daemon verified. A UI shows them
	// at the top.
	Verified session.Facts `json:"verified"`
	// Claims holds the unverified text of the agent, with the session
	// name. A UI shows them apart from the verified facts.
	Claims session.Claims `json:"claims"`
	// Credentials lists the entries of the bundle. For a deviation it
	// lists only the entries that no grant covers.
	Credentials []Credential `json:"credentials"`
	// Grant describes the grant that an approval creates.
	Grant GrantProposal `json:"grant"`
	// ExistingGrant is set if the session already has a grant and the
	// request goes beyond it. It marks the request as a deviation.
	ExistingGrant *ExistingGrant `json:"existing_grant,omitempty"`
}

// Credential is one entry of an approval request. Vault, Title and Field
// are the names that the backend stores, not the text of the agent.
type Credential struct {
	// ID identifies the entry in this request. The decision names the
	// approved entries by this ID.
	ID string `json:"id"`
	// Vault is the stored name of the vault.
	Vault string `json:"vault"`
	// Title is the stored title of the item.
	Title string `json:"title"`
	// Field is the stored label of the field.
	Field string `json:"field"`
	// Environment is the environment of the vault from the configuration.
	Environment config.Environment `json:"environment"`
	// Deviation says how the entry goes beyond the existing grant. It is
	// empty for a first request.
	Deviation Deviation `json:"deviation,omitempty"`
}

// Deviation says how an entry goes beyond an existing grant.
type Deviation string

// The kinds of deviation.
const (
	// DeviationNewItem: the vault is part of the existing grant, the item
	// or field is new.
	DeviationNewItem Deviation = "new_item"
	// DeviationNewVault: no entry of the existing grant is in this vault.
	DeviationNewVault Deviation = "new_vault"
)

// GrantProposal describes the grant that an approval creates.
type GrantProposal struct {
	// TTLSeconds is the lifetime from the configuration. The human can
	// select a shorter lifetime, never a longer one.
	TTLSeconds int64 `json:"ttl_seconds"`
	// Scope names the project and the vaults of the request. It is text
	// for the UI. The grant covers exactly the approved credentials, not
	// the vaults.
	Scope GrantScope `json:"scope"`
}

// GrantScope names a project and vaults for the display.
type GrantScope struct {
	// Project is the project of the session.
	Project string `json:"project"`
	// Vaults lists the stored names of the vaults.
	Vaults []string `json:"vaults"`
}

// ExistingGrant describes the newest grant that the session already has.
type ExistingGrant struct {
	// GrantID is the identifier of the grant.
	GrantID string `json:"grant_id"`
	// ExpiresAt is the end of its lifetime.
	ExpiresAt time.Time `json:"expires_at"`
	// Scope names the project and the vaults of the grant.
	Scope GrantScope `json:"scope"`
	// ItemCount is the number of credentials that the grant covers.
	ItemCount int `json:"item_count"`
}

// Outcome is the result of an approval request.
type Outcome string

// The outcomes.
const (
	// OutcomeApproved: the human approved the selected entries.
	OutcomeApproved Outcome = "approved"
	// OutcomeDenied: the human denied the request.
	OutcomeDenied Outcome = "denied"
	// OutcomeTimeout: the request expired without a decision.
	OutcomeTimeout Outcome = "timeout"
	// OutcomeError: the UI failed.
	OutcomeError Outcome = "error"
)

// Decision is the answer of a UI to a Request.
//
// The daemon does not trust a decision. It accepts an approval only if the
// outcome is approved, RequestID matches the request, the approved IDs are
// a subset of the request and are not empty, and TTLSeconds is more than
// zero and not more than the lifetime of the request. Each other decision
// with the outcome approved ends with the code ui_failed. An approval with
// no approved ID is a denial.
type Decision struct {
	// Version is the version of the schema.
	Version int `json:"version"`
	// Type is always [TypeDecision].
	Type string `json:"type"`
	// RequestID repeats the ID of the request. It is empty if the UI could
	// not read the request.
	RequestID string `json:"request_id,omitempty"`
	// Outcome is the result.
	Outcome Outcome `json:"outcome"`
	// ApprovedCredentialIDs lists the entries that the human selected. It
	// is empty for each outcome that is not approved.
	ApprovedCredentialIDs []string `json:"approved_credential_ids"`
	// TTLSeconds is the lifetime that the human selected. It is set only
	// for the outcome approved.
	TTLSeconds int64 `json:"ttl_seconds,omitempty"`
	// Auth says how the human confirmed. A faked mode reports the policy
	// "skipped".
	Auth *AuthResult `json:"auth,omitempty"`
	// Error is set for the outcome error.
	Error *DecisionError `json:"error,omitempty"`
	// ElapsedMS is the time from the start of the UI to the decision.
	ElapsedMS int64 `json:"elapsed_ms"`
}

// AuthResult says how the human confirmed an approval.
type AuthResult struct {
	// Policy is "biometrics", "biometrics_or_companion", "owner" or
	// "skipped".
	Policy string `json:"policy"`
	// OK reports whether the authentication passed.
	OK bool `json:"ok"`
}

// DecisionError is the error of a UI. The codes are codes of the UI, not
// the stable codes of the wire protocol; the agent receives ui_failed.
type DecisionError struct {
	// Code is "invalid_request", "unsupported_version", "auth_unavailable",
	// "auth_failed" or "internal".
	Code string `json:"code"`
	// Message is text for the log of the daemon.
	Message string `json:"message"`
}

// NoticeKind says what a notice is about.
type NoticeKind string

// The kinds of notice.
const (
	// NoticeDaemonFault: the daemon found a fault in itself and locked
	// delivery for all sessions. The daemon sends one notice for each
	// fault, not one for each request.
	NoticeDaemonFault NoticeKind = "daemon_fault"
	// NoticeProjectConfigInvalid: the configuration of one project is
	// invalid. The daemon rejects the requests of this project only.
	NoticeProjectConfigInvalid NoticeKind = "project_config_invalid"
)

// Notice tells the human about a fault. It informs only: it has no actions
// and no answer. It never contains a secret value.
type Notice struct {
	// Version is the version of the schema. It is always [Version].
	Version int `json:"version"`
	// Type is always [TypeNotice].
	Type string `json:"type"`
	// NoticeID identifies the notice.
	NoticeID string `json:"notice_id"`
	// Kind says what the notice is about.
	Kind NoticeKind `json:"kind"`
	// Code is the stable code that agents receive because of the fault:
	// audit_failed and then daemon_faulted for a daemon fault,
	// config_invalid for a project.
	Code protocol.Code `json:"code"`
	// Project is the project with the invalid configuration. It is empty
	// for a daemon fault.
	Project string `json:"project,omitempty"`
	// Message is the finding in one or two English sentences. It names the
	// next step, for example "Run a38 doctor."
	Message string `json:"message"`
	// OccurredAt is the time of the fault.
	OccurredAt time.Time `json:"occurred_at"`
}

// UI shows approval requests and notices to the human.
//
// The daemon calls Approve for one request at a time; the caller keeps the
// queue. An implementation is safe for a call of Notify at the same time
// as Approve.
type UI interface {
	// Approve shows the request and waits for the decision. It returns
	// when the human decided, when the timeout of the request passed
	// (outcome timeout) or when ctx ended.
	//
	// An error means that the UI itself failed: it did not start, it
	// crashed, or it wrote no valid decision. The daemon then delivers
	// nothing and answers ui_failed. An implementation never returns the
	// outcome approved together with an error.
	Approve(ctx context.Context, req Request) (Decision, error)
	// Notify shows a notice. It does not wait for the human. The lock of
	// the daemon does not depend on the result: an error goes to the log
	// of the daemon only.
	Notify(ctx context.Context, n Notice) error
}
