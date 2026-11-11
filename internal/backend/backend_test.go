package backend

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/seletz/passierschein/internal/protocol"
)

func TestCode(t *testing.T) {
	// T17 (part): each backend error maps to one stable code.
	tests := []struct {
		name string
		err  error
		want protocol.Code
	}{
		{"no error", nil, ""},
		{"locked", ErrLocked, protocol.CodeBackendLocked},
		{"wrapped locked", fmt.Errorf("resolve: %w", ErrLocked), protocol.CodeBackendLocked},
		{"unavailable", ErrUnavailable, protocol.CodeBackendUnavailable},
		{"not found", fmt.Errorf("item: %w", ErrNotFound), protocol.CodeNotFound},
		{"other error", errors.New("connection reset"), protocol.CodeBackendUnavailable},
		{"context ended", context.DeadlineExceeded, protocol.CodeBackendUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Code(tt.err); got != tt.want {
				t.Errorf("Code(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

func TestKindReal(t *testing.T) {
	// T9 (part): only the fake backend permits a faked approval.
	tests := []struct {
		kind Kind
		want bool
	}{
		{KindFake, false},
		{KindOnePassword, true},
		{Kind("other"), true},
		{Kind(""), true},
	}
	for _, tt := range tests {
		if got := tt.kind.Real(); got != tt.want {
			t.Errorf("Kind(%q).Real() = %v, want %v", tt.kind, got, tt.want)
		}
	}
}
