// Package audit defines the entries of the audit log and the interface of
// its store.
//
// The daemon writes each event when it occurs, also each failure. It
// delivers a value only after the entry for the delivery is written. If
// Append fails, the daemon delivers nothing, answers audit_failed and locks
// delivery for all sessions.
//
// The entries form a hash chain: the hash of an entry is SHA-256 over the
// hash of the entry before it and the canonical form of the entry. An
// entry never contains a secret value; it holds the references.
//
// The package holds types and the interface only. The SQLite store with
// the hash chain lives in a sub-package.
package audit

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

// EventType is the kind of an event.
type EventType string

// The kinds of event.
const (
	// EventRequest: a request of an agent arrived.
	EventRequest EventType = "request"
	// EventBundle: the bundle window closed and the bundle is complete.
	EventBundle EventType = "bundle"
	// EventDecision: the approval UI gave a decision.
	EventDecision EventType = "decision"
	// EventGrant: the daemon created a grant.
	EventGrant EventType = "grant"
	// EventDelivery: the daemon delivers values. The daemon writes the
	// entry before it reads the values and before the answer.
	EventDelivery EventType = "delivery"
	// EventDenial: the policy or the human denied a request or an entry.
	EventDenial EventType = "denial"
	// EventRevocation: the human revoked a grant.
	EventRevocation EventType = "revocation"
	// EventExpiry: an approval request or a grant expired, or a grant
	// ended with its session.
	EventExpiry EventType = "expiry"
	// EventError: a fault occurred, for example a locked backend, a failed
	// UI, or the start and the end of the global delivery lock.
	EventError EventType = "error"
	// EventExecution: "a38 mcp" reported the result of a command of the
	// exec way (D65). The fields of the result are claims of "a38 mcp".
	EventExecution EventType = "execution"
)

// EventTypes returns all kinds of event. The caller owns the slice.
func EventTypes() []EventType {
	return []EventType{
		EventRequest, EventBundle, EventDecision, EventGrant, EventDelivery,
		EventDenial, EventRevocation, EventExpiry, EventError, EventExecution,
	}
}

// Valid reports whether t is a known kind of event.
func (t EventType) Valid() bool {
	for _, known := range EventTypes() {
		if t == known {
			return true
		}
	}
	return false
}

// The values of [Result.Detail] that code compares. Each other detail is
// free text for the human.
const (
	// DetailFaultBegin marks the error event at the start of the global
	// delivery lock.
	DetailFaultBegin = "fault_begin"
	// DetailFaultEnd marks the error event at the end of the global
	// delivery lock.
	DetailFaultEnd = "fault_end"
	// DetailSessionEnded marks the expiry of a grant because the
	// connection of its session ended.
	DetailSessionEnded = "session_ended"
	// DetailDaemonStopped marks the expiry of a grant because the daemon
	// stopped on SIGINT or SIGTERM. A grant lives only in memory. An
	// ungraceful end (SIGKILL, crash) writes nothing.
	DetailDaemonStopped = "daemon_stopped"
	// DetailLifetimeEnded marks the expiry of a grant at the end of its
	// lifetime.
	DetailLifetimeEnded = "lifetime_ended"
	// DetailSingleDelivery marks the expiry of a grant for exactly one
	// delivery (lifetime 0, D63). The grant ends with its delivery.
	DetailSingleDelivery = "single_delivery"
	// DetailValueInCommand marks the denial of a command of the exec way
	// whose arguments or working directory contain a value (D65, RI14).
	// The event never names the value.
	DetailValueInCommand = "an argument or the working directory contains a value of a credential"
	// DetailNotStarted marks the event execution of a command that "a38
	// mcp" did not start after the delivery (exec_failed, G39).
	DetailNotStarted = "not_started"
)

// Event is what the daemon gives to the store. The store adds the number,
// the time and the hashes.
type Event struct {
	// Type is the kind of the event.
	Type EventType `json:"type"`
	// Verified holds the verified facts of the session, with the session
	// ID. It is the zero value for an event without a session, for example
	// the start of the global delivery lock.
	Verified session.Facts `json:"verified"`
	// Claims holds the unverified claims of the agent, with the session
	// name. The session name is in this field only.
	Claims session.Claims `json:"claims"`
	// References lists the credentials of the event. An entry never holds
	// a value.
	References []protocol.Reference `json:"references"`
	// Result holds the result of the event.
	Result Result `json:"result"`
	// Command holds the command of the exec way (D65). It is nil for an
	// event of the MCP way.
	Command *Command `json:"command,omitempty"`
	// Execution holds the result of a command as "a38 mcp" reports it. Only
	// the event execution has it. Its fields are claims of "a38 mcp", apart
	// from the verified facts (D4, D65).
	Execution *Execution `json:"execution,omitempty"`
}

// Way is the delivery way of an event (D65).
type Way string

// The delivery ways.
const (
	// WayMCP is the answer of request_credentials.
	WayMCP Way = "mcp"
	// WayExec is the exec way: "a38 mcp" starts a command with the values
	// in its environment.
	WayExec Way = "exec"
)

// MaxCommandArgsBytes is the maximum sum of the lengths of the arguments
// that an event keeps (D65). The rest is cut, and Command.ArgsTruncated is
// set.
const MaxCommandArgsBytes = 4 * 1024

// Command is the command of the exec way in an event (D65). It never holds
// a value.
type Command struct {
	// Way is the delivery way.
	Way Way `json:"way"`
	// Program is the absolute path of the program.
	Program string `json:"program"`
	// Args are the arguments without the program, together at most
	// MaxCommandArgsBytes bytes.
	Args []string `json:"args,omitempty"`
	// ArgsTruncated is set if the event does not keep all arguments.
	ArgsTruncated bool `json:"args_truncated,omitempty"`
	// Cwd is the working directory.
	Cwd string `json:"cwd"`
	// Env lists the environment variables with their references.
	Env []EnvVar `json:"env"`
	// TimeoutSeconds is the time limit of the command. It is 0 before the
	// daemon knows it.
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
	// Binding is the binding of the grant (none, program, command). It is
	// empty before the daemon knows it.
	Binding string `json:"binding,omitempty"`
}

// EnvVar is one environment variable of a command: its name and the
// reference of its credential.
type EnvVar struct {
	// Name is the name of the variable.
	Name string `json:"name"`
	// Reference names the credential.
	Reference protocol.Reference `json:"reference"`
}

// TruncateArgs returns the arguments that fit into MaxCommandArgsBytes and
// reports whether it cut. It cuts an argument only at the boundary of a
// UTF-8 character. The caller owns the result.
func TruncateArgs(args []string) ([]string, bool) {
	var out []string
	left := MaxCommandArgsBytes
	for _, a := range args {
		if len(a) <= left {
			out = append(out, a)
			left -= len(a)
			continue
		}
		cut := left
		for cut > 0 && cut < len(a) && !utf8.RuneStart(a[cut]) {
			cut--
		}
		if cut > 0 {
			out = append(out, a[:cut])
		}
		return out, true
	}
	return out, false
}

// Execution is the result of a command as "a38 mcp" reports it in
// exec_result (D65). It never holds the output or a value.
type Execution struct {
	// ExecRequestID is the ID of the request_exec of the command.
	ExecRequestID string `json:"exec_request_id"`
	// NotStarted is set if "a38 mcp" did not start the command (G39).
	// Each other field is then zero.
	NotStarted bool `json:"not_started,omitempty"`
	// ExitCode is the exit code, or -1 if a signal ended the command.
	ExitCode int `json:"exit_code"`
	// Signal is the name of the signal that ended the command, or empty.
	Signal string `json:"signal,omitempty"`
	// TimedOut is set if the time limit ended the command.
	TimedOut bool `json:"timed_out"`
	// DurationMS is the run time in milliseconds.
	DurationMS int64 `json:"duration_ms"`
	// StdoutBytes and StderrBytes are the sizes of the outputs before the
	// cut.
	StdoutBytes int64 `json:"stdout_bytes"`
	StderrBytes int64 `json:"stderr_bytes"`
	// StdoutTruncated and StderrTruncated are set if "a38 mcp" cut the
	// output.
	StdoutTruncated bool `json:"stdout_truncated"`
	StderrTruncated bool `json:"stderr_truncated"`
	// Masked is the number of masked values in the output.
	Masked int `json:"masked"`
}

// Result is the result of an event.
type Result struct {
	// Code is the stable error code of a failure. It is empty for a
	// success.
	Code protocol.Code `json:"code,omitempty"`
	// RequestID is the ID of the approval request, for the events bundle,
	// decision, denial and expiry of an approval request.
	RequestID string `json:"request_id,omitempty"`
	// GrantID is the grant of the event: the new grant, the grant that
	// covers a delivery, or the grant that ended.
	GrantID string `json:"grant_id,omitempty"`
	// LifetimeSeconds is the lifetime of a new grant.
	LifetimeSeconds int64 `json:"lifetime_seconds,omitempty"`
	// Detail is one of the Detail constants or a short English sentence.
	// It never contains a secret value.
	Detail string `json:"detail,omitempty"`
}

// Entry is one stored entry of the audit log.
type Entry struct {
	// Seq is the running number. The first entry has the number 1.
	Seq uint64 `json:"seq"`
	// Time is the time at which the store wrote the entry, in UTC.
	Time time.Time `json:"time"`
	// Event is the content.
	Event
	// PrevHash is the hash of the entry before, as lower-case hex. It is
	// 64 zeros for the first entry.
	PrevHash string `json:"prev_hash"`
	// Hash is SHA-256 over PrevHash and the canonical form of the entry,
	// as lower-case hex.
	Hash string `json:"hash"`
}

// Filter selects entries for List. A zero field selects all.
type Filter struct {
	// Project selects entries of one project (verified fact).
	Project string
	// SessionName selects entries of sessions that claim the name.
	SessionName string
	// Reference selects entries that name the credential.
	Reference *protocol.Reference
	// Types selects entries of the listed kinds.
	Types []EventType
	// Code selects entries with the error code.
	Code protocol.Code
	// Since selects entries at or after the time.
	Since time.Time
	// Until selects entries before the time.
	Until time.Time
	// Limit is the maximum number of entries. Zero means no limit. With a
	// limit, List returns the newest entries.
	Limit int
}

// Verification is the result of a check of the hash chain.
type Verification struct {
	// OK reports whether the chain is intact.
	OK bool
	// Entries is the number of entries that the check read.
	Entries uint64
	// FirstBadSeq is the number of the first entry whose hash or number
	// does not fit. It is zero if OK is true.
	FirstBadSeq uint64
	// Finding says what is wrong with the entry FirstBadSeq.
	Finding string
}

// Store is the audit log. All methods are safe for concurrent use.
type Store interface {
	// Append writes one event as the next entry of the chain and returns
	// the entry. The entry is durable when Append returns. An error means
	// that the entry is not written: the caller then delivers nothing,
	// answers audit_failed and locks delivery.
	Append(ctx context.Context, ev Event) (Entry, error)
	// List returns the entries that the filter selects, in the sequence of
	// their numbers.
	List(ctx context.Context, f Filter) ([]Entry, error)
	// Verify calculates the hash chain again over all entries. A broken
	// chain is a result with OK false, not an error. An error means that
	// the store could not read the entries.
	Verify(ctx context.Context) (Verification, error)
	// Close releases the store.
	Close() error
}
