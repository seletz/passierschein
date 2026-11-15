package audit

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestEventTypes(t *testing.T) {
	// The nine kinds of event of D36 and the kind "execution" of D65. A
	// change here is a change of the design.
	want := []EventType{"request", "bundle", "decision", "grant", "delivery", "denial", "revocation", "expiry", "error", "execution"}
	got := EventTypes()
	if len(got) != len(want) {
		t.Fatalf("EventTypes() has %d kinds, want %d", len(got), len(want))
	}
	for i, typ := range want {
		if got[i] != typ {
			t.Errorf("EventTypes()[%d] = %q, want %q", i, got[i], typ)
		}
		if !typ.Valid() {
			t.Errorf("%q.Valid() = false", typ)
		}
	}
	for _, typ := range []EventType{"", "Request", "fault"} {
		if typ.Valid() {
			t.Errorf("%q.Valid() = true", typ)
		}
	}
}

// T38, D65: the event keeps the arguments up to 4 KiB and marks the cut.
func TestTruncateArgs(t *testing.T) {
	long := strings.Repeat("a", MaxCommandArgsBytes-1) + "äöü"
	tests := []struct {
		name      string
		args      []string
		want      []string
		truncated bool
	}{
		{"none", nil, nil, false},
		{"short", []string{"-c", "select 1"}, []string{"-c", "select 1"}, false},
		{"exact", []string{strings.Repeat("b", MaxCommandArgsBytes)}, []string{strings.Repeat("b", MaxCommandArgsBytes)}, false},
		{"cut in a character", []string{long}, []string{strings.Repeat("a", MaxCommandArgsBytes-1)}, true},
		{"cut drops the rest", []string{strings.Repeat("c", MaxCommandArgsBytes), "-x"}, []string{strings.Repeat("c", MaxCommandArgsBytes)}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, cut := TruncateArgs(tt.args)
			if !slices.Equal(got, tt.want) || cut != tt.truncated {
				t.Errorf("TruncateArgs = %d args, %v; want %d args, %v", len(got), cut, len(tt.want), tt.truncated)
			}
			size := 0
			for _, a := range got {
				size += len(a)
				if !utf8.ValidString(a) {
					t.Error("the cut splits a character")
				}
			}
			if size > MaxCommandArgsBytes {
				t.Errorf("size = %d", size)
			}
		})
	}
}
