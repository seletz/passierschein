package session

import (
	"encoding/json"
	"testing"
)

func TestSessionClaims(t *testing.T) {
	// T11 (part), T21 (part): claims and verified facts stay in separate
	// values.
	s := Session{
		Facts: Facts{SessionID: "s-0001", Origin: OriginLocal, Project: "beispielprojekt"},
		Name:  "beispiel-koordinator",
		Agent: "coding-agent",
	}
	got := s.Claims("Run the tests", "The tests need the login.")
	want := Claims{SessionName: "beispiel-koordinator", Agent: "coding-agent", Task: "Run the tests", Reason: "The tests need the login."}
	if got != want {
		t.Errorf("Claims() = %+v, want %+v", got, want)
	}
}

func TestJSONFieldNames(t *testing.T) {
	// The names are part of the protocol of the approval app.
	tests := []struct {
		name string
		in   any
		want string
	}{
		{"facts", Facts{SessionID: "s-0001", Origin: OriginLocal, Project: "beispielprojekt", PeerPID: 4242, PeerUID: 501,
			Process: "/usr/local/bin/agent-cli", Cwd: "/home/beispiel/develop/beispielprojekt"},
			`{"session_id":"s-0001","origin":"local","project":"beispielprojekt","peer_pid":4242,"peer_uid":501,"process":"/usr/local/bin/agent-cli","cwd":"/home/beispiel/develop/beispielprojekt"}`},
		{"claims", Claims{SessionName: "beispiel-koordinator", Agent: "coding-agent", Task: "Run the tests", Reason: "The tests need the login."},
			`{"session_name":"beispiel-koordinator","agent":"coding-agent","task":"Run the tests","reason":"The tests need the login."}`},
		{"claims without name", Claims{Task: "Run the tests"}, `{"task":"Run the tests"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("Marshal = %s, want %s", got, tt.want)
			}
		})
	}
}
