// Package session defines the data that describes a session: the facts
// that the daemon verified and the claims of the agent.
//
// A session is one connection of a process "a38 mcp" to the agent socket.
// The daemon gives the session its ID and reads the origin from the peer
// data of the socket. The approval request and the audit log keep the
// verified facts and the claims in separate fields. Code must never copy a
// claim into a verified field.
//
// The package holds types only. The daemon owns the session registry.
package session

// ID is the identifier of a session. Only the daemon creates an ID. An ID
// has 1 to 64 characters from letters, digits, ".", "_", ":" and "-", so
// that it is a valid ID of the wire protocol.
type ID string

// Origin says how the peer reached the daemon.
type Origin string

// The origins of a session.
const (
	// OriginLocal is a process on the Mac of the daemon.
	OriginLocal Origin = "local"
	// OriginVM is a process in a VM. No stage before stage 5 uses it.
	OriginVM Origin = "vm"
)

// Facts holds what the daemon verified about a session. The agent cannot
// set a field of Facts. Locally the peer data is a hint and not a proof.
//
// The JSON form is the object "verified" of the approval request.
type Facts struct {
	// SessionID is the identifier that the daemon gave to the session. A
	// grant belongs to this ID, never to the session name.
	SessionID ID `json:"session_id"`
	// Origin says how the peer reached the daemon.
	Origin Origin `json:"origin"`
	// Project is the project that follows from the working directory. It
	// is empty if the directory belongs to no project.
	Project string `json:"project"`
	// PeerPID is the process ID of the peer.
	PeerPID int `json:"peer_pid,omitempty"`
	// PeerUID is the user ID of the peer.
	PeerUID int `json:"peer_uid"`
	// Process is the path of the program of the peer.
	Process string `json:"process,omitempty"`
	// Cwd is the working directory of the peer, absolute and with symbolic
	// links resolved.
	Cwd string `json:"cwd,omitempty"`
}

// Claims holds what the agent said about itself. Each field is unverified
// text from the zone of the agent. The approval request shows the claims
// apart from the verified facts.
//
// The JSON form is the object "claims" of the approval request.
type Claims struct {
	// SessionName is the name of the session in the agent tool. It comes
	// from the hello message. Empty means that no name is set; a UI then
	// shows an explicit mark and no invented value.
	SessionName string `json:"session_name,omitempty"`
	// Agent is the name of the agent tool. It comes from the hello message.
	Agent string `json:"agent,omitempty"`
	// Task is the task of the agent. It comes from the request.
	Task string `json:"task,omitempty"`
	// Reason says why the agent needs the credentials. It comes from the
	// request.
	Reason string `json:"reason,omitempty"`
}

// Session is the view of one connected session that the socket layer gives
// to the code that handles a request.
type Session struct {
	// Facts holds the verified facts.
	Facts Facts
	// Name is the session name from the hello message. It is a claim.
	Name string
	// Agent is the agent name from the hello message. It is a claim.
	Agent string
}

// Claims returns the claims of the session together with the task and the
// reason of one request.
func (s Session) Claims(task, reason string) Claims {
	return Claims{SessionName: s.Name, Agent: s.Agent, Task: task, Reason: reason}
}
