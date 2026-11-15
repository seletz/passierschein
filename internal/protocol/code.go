package protocol

import (
	"errors"
	"fmt"
)

// Code is a stable error code. An agent decides on the code, not on the
// message text. The set of codes is fixed by the design.
type Code string

// The stable error codes.
const (
	// CodeDenied: the human denied the request or did not select the entry.
	CodeDenied Code = "denied"
	// CodeTimeout: the approval request expired without a decision.
	CodeTimeout Code = "timeout"
	// CodeNotAllowed: the policy does not allow the request. The request
	// did not reach the human.
	CodeNotAllowed Code = "not_allowed"
	// CodeBundleTooLarge: the bundle of the session is full. The entry is
	// not part of the approval request.
	CodeBundleTooLarge Code = "bundle_too_large"
	// CodeDaemonUnavailable: "a38 mcp" cannot reach the daemon. Only
	// "a38 mcp" creates this code; the daemon never sends it.
	CodeDaemonUnavailable Code = "daemon_unavailable"
	// CodeBackendLocked: the backend is locked.
	CodeBackendLocked Code = "backend_locked"
	// CodeBackendUnavailable: the daemon cannot reach the backend.
	CodeBackendUnavailable Code = "backend_unavailable"
	// CodeUIFailed: the approval UI failed or gave an invalid decision.
	CodeUIFailed Code = "ui_failed"
	// CodeAuditFailed: the daemon cannot write the audit log.
	CodeAuditFailed Code = "audit_failed"
	// CodeDaemonFaulted: the daemon found a fault in itself. Delivery is
	// locked for all sessions until the human removes the lock.
	CodeDaemonFaulted Code = "daemon_faulted"
	// CodeConfigInvalid: the configuration of the project of the session is
	// invalid. Other projects continue to work.
	CodeConfigInvalid Code = "config_invalid"
	// CodeSessionNameConflict: two connected sessions have the same session
	// name.
	CodeSessionNameConflict Code = "session_name_conflict"
	// CodeNotFound: an item or a field does not exist in an allowed vault.
	// On the admin socket: the grant does not exist.
	CodeNotFound Code = "not_found"
	// CodeInvalidRequest: the message does not obey the schema.
	CodeInvalidRequest Code = "invalid_request"
	// CodeExecRequired: the vault delivers only on the exec way.
	// request_credentials gives no value for it; the agent asks again with
	// run_with_credentials (D61, D62).
	CodeExecRequired Code = "exec_required"
	// CodeExecFailed: "a38 mcp" did not start the command after the
	// delivery, for example because the file is not executable any more.
	// Only "a38 mcp" creates this code for the agent; it reports it to the
	// daemon in exec_result (D61).
	CodeExecFailed Code = "exec_failed"
)

// codes lists all codes in the sequence of the design: D35, then D61.
var codes = [...]Code{
	CodeDenied, CodeTimeout, CodeNotAllowed, CodeBundleTooLarge,
	CodeDaemonUnavailable, CodeBackendLocked, CodeBackendUnavailable,
	CodeUIFailed, CodeAuditFailed, CodeDaemonFaulted, CodeConfigInvalid,
	CodeSessionNameConflict, CodeNotFound, CodeInvalidRequest,
	CodeExecRequired, CodeExecFailed,
}

// Codes returns all stable error codes. The caller owns the slice.
func Codes() []Code {
	out := make([]Code, len(codes))
	copy(out, codes[:])
	return out
}

// Valid reports whether c is one of the stable error codes.
func (c Code) Valid() bool {
	for _, known := range codes {
		if c == known {
			return true
		}
	}
	return false
}

// Error is an error with a stable code. It is also the error object of the
// wire protocol. The message is for a human or an agent to read. The message
// never contains a secret value.
type Error struct {
	// Code is the stable error code.
	Code Code `json:"code"`
	// Message is a short English sentence.
	Message string `json:"message"`
}

// Errorf returns an *Error with the code and a formatted message. Do not
// pass a secret value or untrusted message content as an argument.
func Errorf(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Error returns the code and the message.
func (e *Error) Error() string {
	return string(e.Code) + ": " + e.Message
}

// Is reports whether target is an *Error with the same code. This lets a
// caller write errors.Is(err, &protocol.Error{Code: protocol.CodeDenied}).
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Code == e.Code
}

// CodeOf returns the code of the first *Error in the chain of err. It
// returns the empty code if there is none.
func CodeOf(err error) Code {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// validate checks an error object that came from the wire.
func (e *Error) validate() error {
	if !e.Code.Valid() {
		return invalid("the error code is not a known code")
	}
	return checkText("error message", e.Message, MaxClaimLength, true)
}
