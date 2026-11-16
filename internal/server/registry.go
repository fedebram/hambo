package server

import (
	"sync"

	"github.com/fedebram/hambo/internal/muxer"
)

// registry keeps the session of each connected node, by public key.
type registry struct {
	mu       sync.Mutex
	sessions map[string]*muxer.Session
}

func newRegistry() *registry {
	return &registry{sessions: make(map[string]*muxer.Session)}
}

// add saves the node's session. A newer session replaces the old one.
func (r *registry) add(pub []byte, sess *muxer.Session) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sessions[string(pub)] = sess
}

// remove takes sess out of the registry. If the node already has
// a newer session, it does nothing.
func (r *registry) remove(pub []byte, sess *muxer.Session) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sessions[string(pub)] == sess {
		delete(r.sessions, string(pub))
	}
}

// get returns the node's session, or nil.
func (r *registry) get(pub []byte) *muxer.Session {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.sessions[string(pub)]
}

// has reports whether the node has a session.
func (r *registry) has(pub []byte) bool {
	return r.get(pub) != nil
}
