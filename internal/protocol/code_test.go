package protocol

import (
	"errors"
	"fmt"
	"testing"
)

func TestCodes(t *testing.T) {
	// The list of D35. A change here is a change of the design.
	want := []Code{
		"denied", "timeout", "not_allowed", "bundle_too_large",
		"daemon_unavailable", "backend_locked", "backend_unavailable",
		"ui_failed", "audit_failed", "daemon_faulted", "config_invalid",
		"session_name_conflict", "not_found", "invalid_request",
	}
	got := Codes()
	if len(got) != len(want) {
		t.Fatalf("Codes() has %d codes, want %d", len(got), len(want))
	}
	for i, c := range want {
		if got[i] != c {
			t.Errorf("Codes()[%d] = %q, want %q", i, got[i], c)
		}
		if !c.Valid() {
			t.Errorf("%q.Valid() = false", c)
		}
	}
	got[0] = "changed"
	if Codes()[0] != CodeDenied {
		t.Error("Codes() returns shared memory")
	}
	for _, c := range []Code{"", "ok", "Denied", "unknown"} {
		if c.Valid() {
			t.Errorf("%q.Valid() = true", c)
		}
	}
}

func TestError(t *testing.T) {
	err := Errorf(CodeNotAllowed, "the vault %q is not allowed", "beispiel.app.k8s-prod")
	if got, want := err.Error(), `not_allowed: the vault "beispiel.app.k8s-prod" is not allowed`; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	wrapped := fmt.Errorf("request: %w", err)
	if got := CodeOf(wrapped); got != CodeNotAllowed {
		t.Errorf("CodeOf(wrapped) = %q, want %q", got, CodeNotAllowed)
	}
	if got := CodeOf(errors.New("other")); got != "" {
		t.Errorf("CodeOf(other) = %q, want an empty code", got)
	}
	if got := CodeOf(nil); got != "" {
		t.Errorf("CodeOf(nil) = %q, want an empty code", got)
	}
	if !errors.Is(wrapped, &Error{Code: CodeNotAllowed}) {
		t.Error("errors.Is does not match an error with the same code")
	}
	if errors.Is(wrapped, &Error{Code: CodeDenied}) {
		t.Error("errors.Is matches an error with a different code")
	}
}
