package protocol

import (
	"time"
)

// Version is the version of the wire protocol. A message with a different
// version is invalid.
const Version = 1

// Limits of the wire protocol.
const (
	// MaxMessageSize is the maximum size of one message in bytes. The line
	// end counts.
	MaxMessageSize = 64 * 1024
	// MaxIDLength is the maximum length of a correlation ID, a session ID
	// or a grant ID in bytes.
	MaxIDLength = 64
	// MaxReferences is the maximum number of references in one request.
	// The bundle maximum of the configuration is a different, smaller limit.
	MaxReferences = 100
	// MaxClaimLength is the maximum length of a task, a reason or an error
	// message in characters.
	MaxClaimLength = 2000
	// MaxAgentLength is the maximum length of the agent name in characters.
	MaxAgentLength = 80
	// MaxSessionNameLength is the absolute maximum length of the session
	// name in characters. The configuration sets the limit that applies; it
	// cannot be larger than this.
	MaxSessionNameLength = 256
)

// Type is the type of a message.
type Type string

// Message types of the agent socket.
const (
	// TypeHello is the first message of "a38 mcp" on a connection.
	TypeHello Type = "hello"
	// TypeWelcome is the answer of the daemon to hello.
	TypeWelcome Type = "welcome"
	// TypeRequestCredentials asks for the values of credentials.
	TypeRequestCredentials Type = "request_credentials"
	// TypeCredentials is the answer to request_credentials.
	TypeCredentials Type = "credentials"
	// TypeGrantStatus asks for the grants of the own session.
	TypeGrantStatus Type = "grant_status"
)

// Message types of the admin socket.
const (
	// TypeGrantsList asks for the grants of all sessions.
	TypeGrantsList Type = "grants_list"
	// TypeGrantRevoke ends one grant.
	TypeGrantRevoke Type = "grant_revoke"
	// TypeGrantRevoked is the answer to grant_revoke.
	TypeGrantRevoked Type = "grant_revoked"
	// TypeStatus asks for the state of the daemon.
	TypeStatus Type = "status"
	// TypeStatusResult is the answer to status.
	TypeStatusResult Type = "status_result"
	// TypeFaultClear asks the daemon to remove the global delivery lock.
	TypeFaultClear Type = "fault_clear"
	// TypeFaultClearResult is the answer to fault_clear.
	TypeFaultClearResult Type = "fault_clear_result"
)

// Message types of both sockets.
const (
	// TypeGrants is the answer to grant_status and to grants_list.
	TypeGrants Type = "grants"
	// TypeError is the answer to a request that failed as a whole.
	TypeError Type = "error"
)

// Header holds the two fields that each message has. [Encode] sets both
// fields; a caller does not set them.
type Header struct {
	// Version is the version of the protocol.
	Version int `json:"version"`
	// Type is the type of the message.
	Type Type `json:"type"`
}

// header gives Encode and Decode access to the header of each message type
// that embeds Header.
func (h *Header) header() *Header { return h }

// Message is one message of the wire protocol. Only the types of this
// package implement it.
type Message interface {
	// MessageType returns the type of the message.
	MessageType() Type
	// Validate checks the content of the message. A failure is an *Error
	// with the code invalid_request.
	Validate() error
	header() *Header
}

// Hello is the first message on a connection to the agent socket. "a38 mcp"
// sends it exactly once. A second hello is invalid.
//
// Both fields are claims from the zone of the agent. The daemon does not
// verify them and shows them only with the claims of the agent.
type Hello struct {
	Header
	// SessionName is the name of the session in the agent tool, for example
	// "beispiel-koordinator". It is a start value of "a38 mcp". Empty means
	// that no name is set. The daemon checks the name against the rules of
	// the configuration.
	SessionName string `json:"session_name,omitempty"`
	// Agent is the name of the agent tool, as the MCP client gives it.
	Agent string `json:"agent,omitempty"`
}

// Welcome is the answer to [Hello]. The daemon made each field; the agent
// cannot set them.
type Welcome struct {
	Header
	// SessionID is the identifier that the daemon gave to the session.
	SessionID string `json:"session_id"`
	// Project is the project that follows from the working directory of
	// the peer. Empty means that the directory belongs to no project.
	Project string `json:"project,omitempty"`
}

// RequestCredentials asks for the values of credentials. One connection can
// have more than one open request; the ID connects request and answer.
type RequestCredentials struct {
	Header
	// ID is the correlation ID. The sender selects it.
	ID string `json:"id"`
	// References names the credentials. The sequence is the sequence of
	// the entries in the answer.
	References []Reference `json:"references"`
	// Task is the task of the agent. It is a claim.
	Task string `json:"task,omitempty"`
	// Reason says why the agent needs the credentials. It is a claim.
	Reason string `json:"reason,omitempty"`
}

// Credentials is the answer to [RequestCredentials]. It has one entry for
// each reference of the request, in the same sequence.
type Credentials struct {
	Header
	// ID is the correlation ID of the request.
	ID string `json:"id"`
	// Entries holds a value or an error for each reference.
	Entries []CredentialEntry `json:"entries"`
}

// CredentialEntry is the result for one reference. Exactly one of Value and
// Error is set.
type CredentialEntry struct {
	// Reference is the reference of the request.
	Reference Reference `json:"reference"`
	// Value is the secret value.
	Value *Secret `json:"value,omitempty"`
	// Error says why there is no value, for example denied or
	// bundle_too_large.
	Error *Error `json:"error,omitempty"`
}

// GrantStatus asks for the grants of the own session.
type GrantStatus struct {
	Header
	// ID is the correlation ID.
	ID string `json:"id"`
}

// Grants is the answer to [GrantStatus] and to [GrantsList]. It never holds
// a secret value.
type Grants struct {
	Header
	// ID is the correlation ID of the request.
	ID string `json:"id"`
	// Grants lists the grants.
	Grants []Grant `json:"grants"`
}

// Grant describes one grant. On the agent socket the session fields are
// empty: a session sees only its own grants.
type Grant struct {
	// GrantID is the identifier of the grant.
	GrantID string `json:"grant_id"`
	// SessionID is the session that holds the grant. Admin socket only.
	SessionID string `json:"session_id,omitempty"`
	// SessionName is the name that the session claims. Admin socket only.
	SessionName string `json:"session_name,omitempty"`
	// Project is the project of the session. Admin socket only.
	Project string `json:"project,omitempty"`
	// References lists the credentials that the grant covers.
	References []Reference `json:"references"`
	// GrantedAt is the time of the approval.
	GrantedAt time.Time `json:"granted_at"`
	// ExpiresAt is the end of the lifetime.
	ExpiresAt time.Time `json:"expires_at"`
	// RemainingSeconds is the remaining lifetime when the daemon answered.
	RemainingSeconds int64 `json:"remaining_seconds"`
}

// ErrorResponse is the answer to a request that failed as a whole.
type ErrorResponse struct {
	Header
	// ID is the correlation ID of the request. It is empty if the daemon
	// could not read the ID.
	ID string `json:"id,omitempty"`
	// Error holds the stable code and the message.
	Error Error `json:"error"`
}

// GrantsList asks for the grants of all sessions. Admin socket only.
type GrantsList struct {
	Header
	// ID is the correlation ID.
	ID string `json:"id"`
}

// GrantRevoke ends one grant. Admin socket only.
type GrantRevoke struct {
	Header
	// ID is the correlation ID.
	ID string `json:"id"`
	// GrantID is the grant to end.
	GrantID string `json:"grant_id"`
}

// GrantRevoked is the answer to [GrantRevoke].
type GrantRevoked struct {
	Header
	// ID is the correlation ID of the request.
	ID string `json:"id"`
	// GrantID is the grant that ended.
	GrantID string `json:"grant_id"`
}

// Status asks for the state of the daemon. "a38 doctor" uses it. Admin
// socket only.
type Status struct {
	Header
	// ID is the correlation ID.
	ID string `json:"id"`
}

// StatusResult is the answer to [Status].
type StatusResult struct {
	Header
	// ID is the correlation ID of the request.
	ID string `json:"id"`
	// DaemonVersion is the version of the daemon binary.
	DaemonVersion string `json:"daemon_version"`
	// Sessions is the number of connected sessions.
	Sessions int `json:"sessions"`
	// Grants is the number of active grants.
	Grants int `json:"grants"`
	// Fault is set while delivery is locked for all sessions.
	Fault *Fault `json:"fault,omitempty"`
	// InvalidProjects lists the projects with an invalid configuration.
	InvalidProjects []InvalidProject `json:"invalid_projects,omitempty"`
	// Backend is the state of the backend. It is nil if the daemon does
	// not know it.
	Backend *BackendStatus `json:"backend,omitempty"`
	// UI is the state of the approval UI. It is nil if the daemon does
	// not know it.
	UI *UIStatus `json:"ui,omitempty"`
}

// The states of the backend in [BackendStatus].
const (
	// BackendOK: the backend answers.
	BackendOK = "ok"
	// BackendLocked: the backend is locked (backend_locked).
	BackendLocked = "locked"
	// BackendUnreachable: the backend does not answer
	// (backend_unavailable).
	BackendUnreachable = "unreachable"
)

// BackendStatus is the state of the backend for a38 doctor. It never
// holds a value.
type BackendStatus struct {
	// Kind is the backend kind, the value of the flag --backend ("fake"
	// or "1password"). It is empty for an old daemon.
	Kind string `json:"kind,omitempty"`
	// State is [BackendOK], [BackendLocked] or [BackendUnreachable].
	State string `json:"state"`
	// Reason is the finding in one sentence. It is empty if State is ok.
	Reason string `json:"reason,omitempty"`
	// Accounts lists the state of each account of a backend with more
	// than one account (1Password, D78), in the order of the
	// configuration. State is ok while one account answers. It is empty
	// for each other backend and for an old daemon. Admin socket only.
	Accounts []AccountStatus `json:"accounts,omitempty"`
}

// AccountStatus is the state of one account of the backend for a38
// doctor (D78). It never holds a value or a token.
type AccountStatus struct {
	// Name is the name of the account in the configuration.
	Name string `json:"name"`
	// State is [BackendOK], [BackendLocked] or [BackendUnreachable].
	State string `json:"state"`
	// Reason is the finding in one sentence. It is empty if State is ok.
	Reason string `json:"reason,omitempty"`
	// VaultsConfigured is the number of vaults of the account in the
	// configuration.
	VaultsConfigured int `json:"vaults_configured"`
	// VaultsVisible is the number of these vaults that the service
	// account can see; zero if State is not ok.
	VaultsVisible int `json:"vaults_visible"`
}

// UIStatus is the state of the approval UI for a38 doctor.
type UIStatus struct {
	// Mode is the mode of the UI, the value of the flag --ui.
	Mode string `json:"mode"`
	// Usable reports whether the daemon can show an approval request.
	Usable bool `json:"usable"`
	// Reason is the finding in one sentence if Usable is false.
	Reason string `json:"reason,omitempty"`
	// AppPath is the path of the approval app for the mode app. The
	// doctor checks the file. It is empty for each other mode.
	AppPath string `json:"app_path,omitempty"`
}

// Fault describes the fault that locks delivery for all sessions.
type Fault struct {
	// Code is the code of the cause, for example audit_failed.
	Code Code `json:"code"`
	// Reason is the finding in one sentence.
	Reason string `json:"reason"`
	// Since is the start of the lock.
	Since time.Time `json:"since"`
	// CheckPassed reports whether the check of the cause passes now. The
	// daemon checks without a write to the audit log. It is nil if the
	// daemon cannot check the cause. A passed check does not lift the
	// lock: only the command fault_clear or a restart lifts it (D48).
	CheckPassed *bool `json:"check_passed,omitempty"`
	// Finding is the result of a failed check of the cause. It is empty
	// if the check passed.
	Finding string `json:"finding,omitempty"`
	// BeginPending reports that the entry for the begin of the lock is
	// not in the audit log yet. The daemon writes it as soon as the audit
	// log can be written, at the latest before the entry for the end.
	BeginPending bool `json:"begin_pending,omitempty"`
}

// InvalidProject names a project that the daemon rejects with
// config_invalid.
type InvalidProject struct {
	// Project is the name of the project.
	Project string `json:"project"`
	// Reason is the finding in one sentence.
	Reason string `json:"reason"`
}

// FaultClear asks the daemon to remove the global delivery lock. The daemon
// removes the lock only if the check of the cause passes. Admin socket only.
type FaultClear struct {
	Header
	// ID is the correlation ID.
	ID string `json:"id"`
}

// FaultClearResult is the answer to [FaultClear].
type FaultClearResult struct {
	Header
	// ID is the correlation ID of the request.
	ID string `json:"id"`
	// Cleared reports whether delivery is open after the command.
	Cleared bool `json:"cleared"`
	// Finding names the cause that keeps the lock. It is empty if Cleared
	// is true.
	Finding string `json:"finding,omitempty"`
}

// MessageType returns [TypeHello].
func (*Hello) MessageType() Type { return TypeHello }

// MessageType returns [TypeWelcome].
func (*Welcome) MessageType() Type { return TypeWelcome }

// MessageType returns [TypeRequestCredentials].
func (*RequestCredentials) MessageType() Type { return TypeRequestCredentials }

// MessageType returns [TypeCredentials].
func (*Credentials) MessageType() Type { return TypeCredentials }

// MessageType returns [TypeGrantStatus].
func (*GrantStatus) MessageType() Type { return TypeGrantStatus }

// MessageType returns [TypeGrants].
func (*Grants) MessageType() Type { return TypeGrants }

// MessageType returns [TypeError].
func (*ErrorResponse) MessageType() Type { return TypeError }

// MessageType returns [TypeGrantsList].
func (*GrantsList) MessageType() Type { return TypeGrantsList }

// MessageType returns [TypeGrantRevoke].
func (*GrantRevoke) MessageType() Type { return TypeGrantRevoke }

// MessageType returns [TypeGrantRevoked].
func (*GrantRevoked) MessageType() Type { return TypeGrantRevoked }

// MessageType returns [TypeStatus].
func (*Status) MessageType() Type { return TypeStatus }

// MessageType returns [TypeStatusResult].
func (*StatusResult) MessageType() Type { return TypeStatusResult }

// MessageType returns [TypeFaultClear].
func (*FaultClear) MessageType() Type { return TypeFaultClear }

// MessageType returns [TypeFaultClearResult].
func (*FaultClearResult) MessageType() Type { return TypeFaultClearResult }
