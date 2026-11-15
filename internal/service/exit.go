package service

import (
	"fmt"
	"strconv"
)

// Exit is how the daemon ended: an exit code, or a signal.
type Exit struct {
	// Code is the exit code; it is valid if Signal is 0.
	Code int
	// Signal is the number of the signal that ended the process; 0 if the
	// process exited by itself.
	Signal int
	// Reason is the line "last exit reason" of launchctl print, for
	// example OS_REASON_CODESIGNING; empty if launchd names none. A kill
	// by the kernel can have a reason without a code and a signal.
	Reason string
}

// ReasonCodeSigning is the exit reason of launchd when the kernel killed
// the process for its code signature, for example because the hardened
// runtime forbids code in memory (issue 89).
const ReasonCodeSigning = "OS_REASON_CODESIGNING"

// CodeSigning reports whether the kernel killed the process for its code
// signature.
func (e Exit) CodeSigning() bool { return e.Reason == ReasonCodeSigning }

// Crash reports whether a signal ended the process that launchd counts as
// a crash (SIGILL, SIGTRAP, SIGABRT, SIGFPE, SIGBUS, SIGSEGV, SIGSYS).
func (e Exit) Crash() bool {
	switch e.Signal {
	case 4, 5, 6, 8, 10, 11, 12:
		return true
	}
	return false
}

// ExitMeaning is one row of the table of exit codes of "a38 serve".
type ExitMeaning struct {
	// Code is the exit code.
	Code int
	// Meaning says what the code means for the human.
	Meaning string
}

// ExitCodes is the exit code convention of "a38 serve" under launchd.
// With the plist of this package no exit code and no signal leads to a
// restart ([Restarts]); the human restarts the daemon after a fix.
var ExitCodes = []ExitMeaning{
	{0, "the daemon stopped normally (SIGTERM at logout, a38 service uninstall or a38 service restart)"},
	{1, "the daemon did not start or failed: configuration, token source, socket directory or audit log; the log names the cause"},
	{2, "wrong arguments in the plist, or a crash of the Go runtime (panic); the log names the cause; run a38 service install again"},
}

// Explain returns the meaning of an exit for the output of status and
// doctor.
func Explain(e Exit) string {
	switch {
	case e.CodeSigning():
		return "macOS killed the daemon for its code signature (" + ReasonCodeSigning + "): the signature of this a38 does not allow code that a38 runs, for example the WebAssembly core of the 1Password SDK; check it with a38 selftest sdk-core"
	case e.Reason != "" && e.Code == 0 && e.Signal == 0:
		return "launchd reports the exit reason " + e.Reason + "; the log may name the cause"
	case e.Reason != "":
		return explainStatus(e) + " (exit reason " + e.Reason + ")"
	}
	return explainStatus(e)
}

// explainStatus explains the exit code or the signal of e.
func explainStatus(e Exit) string {
	if e.Signal != 0 {
		if e.Crash() {
			return fmt.Sprintf("signal %d: the daemon crashed; the log may name the cause", e.Signal)
		}
		return fmt.Sprintf("signal %d: something stopped the daemon from outside", e.Signal)
	}
	for _, m := range ExitCodes {
		if m.Code == e.Code {
			return "exit code " + strconv.Itoa(e.Code) + ": " + m.Meaning
		}
	}
	return fmt.Sprintf("exit code %d: unknown; the log may name the cause", e.Code)
}

// Restarts reports whether launchd restarts a job with the plist p after
// the exit e, following the key KeepAlive of launchd.plist(5): true
// restarts always; a dictionary restarts if one of its conditions holds
// (SuccessfulExit, Crashed); other conditions (PathState, OtherJobEnabled)
// count as a restart, because they do not depend on the exit. A missing key
// or false restarts never.
func Restarts(p Plist, e Exit) bool {
	switch v := p.Values["KeepAlive"].(type) {
	case bool:
		return v
	case map[string]any:
		for k, c := range v {
			switch k {
			case "SuccessfulExit":
				want, _ := c.(bool)
				success := e.Signal == 0 && e.Code == 0
				if want == success {
					return true
				}
			case "Crashed":
				want, _ := c.(bool)
				if want == e.Crash() {
					return true
				}
			case "AfterInitialDemand":
			default:
				return true
			}
		}
	}
	return false
}
