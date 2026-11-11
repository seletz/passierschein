package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"sync"

	"github.com/seletz/passierschein/internal/session"
)

// registry holds the connected sessions of the agent socket. It finds a
// session name conflict (D25): two connected sessions with the same name.
// A session without a name never conflicts.
type registry struct {
	mu       sync.Mutex
	sessions map[session.ID]string
	names    map[string]int
}

// newRegistry returns an empty registry.
func newRegistry() *registry {
	return &registry{sessions: map[session.ID]string{}, names: map[string]int{}}
}

// add registers a session with its name.
func (r *registry) add(id session.ID, name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[id] = name
	if name != "" {
		r.names[name]++
	}
}

// remove removes a session. It does nothing for an unknown ID.
func (r *registry) remove(id session.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name, ok := r.sessions[id]
	if !ok {
		return
	}
	delete(r.sessions, id)
	if name == "" {
		return
	}
	if r.names[name]--; r.names[name] <= 0 {
		delete(r.names, name)
	}
}

// conflict reports whether the session has a name that another connected
// session also has.
func (r *registry) conflict(id session.ID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	name, ok := r.sessions[id]
	return ok && name != "" && r.names[name] > 1
}

// count returns the number of connected sessions.
func (r *registry) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.sessions)
}

// newSessionID returns a random session ID: "s-" and 128 random bits in
// hexadecimal. The agent cannot guess or select it.
func newSessionID() (session.ID, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return session.ID("s-" + hex.EncodeToString(b[:])), nil
}
