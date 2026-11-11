package audit

import "testing"

func TestEventTypes(t *testing.T) {
	// The nine kinds of event of D36. A change here is a change of the
	// design.
	want := []EventType{"request", "bundle", "decision", "grant", "delivery", "denial", "revocation", "expiry", "error"}
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
