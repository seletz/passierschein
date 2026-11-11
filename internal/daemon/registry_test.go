package daemon

import (
	"testing"

	"github.com/seletz/passierschein/internal/protocol"
	"github.com/seletz/passierschein/internal/session"
)

func TestRegistryConflict(t *testing.T) {
	// T21 (part): two connected sessions with the same name conflict; a
	// session without a name never conflicts; the conflict ends when one
	// session ends.
	type step struct {
		add, remove session.ID
		name        string
		want        map[session.ID]bool
		count       int
	}
	tests := []struct {
		name  string
		steps []step
	}{
		{"one named session", []step{
			{add: "a", name: "beispiel-koordinator", want: map[session.ID]bool{"a": false}, count: 1},
		}},
		{"same name conflicts, removal ends it", []step{
			{add: "a", name: "beispiel-koordinator", want: map[session.ID]bool{"a": false}, count: 1},
			{add: "b", name: "beispiel-koordinator", want: map[session.ID]bool{"a": true, "b": true}, count: 2},
			{remove: "a", want: map[session.ID]bool{"a": false, "b": false}, count: 1},
		}},
		{"three sessions with one name", []step{
			{add: "a", name: "beispiel-koordinator"},
			{add: "b", name: "beispiel-koordinator"},
			{add: "c", name: "beispiel-koordinator", want: map[session.ID]bool{"a": true, "b": true, "c": true}, count: 3},
			{remove: "b", want: map[session.ID]bool{"a": true, "c": true}, count: 2},
			{remove: "c", want: map[session.ID]bool{"a": false}, count: 1},
		}},
		{"no name never conflicts", []step{
			{add: "a", name: ""},
			{add: "b", name: "", want: map[session.ID]bool{"a": false, "b": false}, count: 2},
		}},
		{"different names", []step{
			{add: "a", name: "beispiel-koordinator"},
			{add: "b", name: "beispiel-koordinator (2)", want: map[session.ID]bool{"a": false, "b": false}, count: 2},
		}},
		{"remove unknown", []step{
			{remove: "x", want: map[session.ID]bool{"x": false}, count: 0},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRegistry()
			for i, s := range tt.steps {
				if s.add != "" {
					r.add(s.add, s.name)
				}
				if s.remove != "" {
					r.remove(s.remove)
				}
				for id, want := range s.want {
					if got := r.conflict(id); got != want {
						t.Errorf("step %d: conflict(%s) = %v, want %v", i, id, got, want)
					}
				}
				if s.want != nil && r.count() != s.count {
					t.Errorf("step %d: count() = %d, want %d", i, r.count(), s.count)
				}
			}
		})
	}
}

func TestNewSessionID(t *testing.T) {
	// T11 (part): the daemon makes the session ID; it is random and a
	// valid ID of the wire protocol.
	seen := map[session.ID]bool{}
	for range 100 {
		id, err := newSessionID()
		if err != nil {
			t.Fatal(err)
		}
		if seen[id] {
			t.Fatalf("newSessionID() returned %s twice", id)
		}
		seen[id] = true
		if err := (&protocol.Welcome{SessionID: string(id)}).Validate(); err != nil {
			t.Fatalf("session ID %s is not valid on the wire: %v", id, err)
		}
		if len(id) != 34 {
			t.Errorf("len(%s) = %d, want 34", id, len(id))
		}
	}
}
