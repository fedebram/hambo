package main

import (
	"sync"

	"github.com/fedebram/hambo/internal/muxer"
)

// registry keeps the session of each connected worker, by public key.
type registry struct {
	mu       sync.Mutex
	sessions map[string]*muxer.Session
}

func newRegistry() *registry {
	return &registry{sessions: make(map[string]*muxer.Session)}
}

// add stores the worker's session, and closes the old one if there was one.
func (r *registry) add(pub []byte, sess *muxer.Session) {
	r.mu.Lock()
	old := r.sessions[string(pub)]
	r.sessions[string(pub)] = sess
	r.mu.Unlock()

	// if the worker reconnects with a new session,
	// there are cases where the old session remains alive.
	// this way gets closed without waiting for keepalive timeout.
	if old != nil {
		old.Close()
	}
}

// remove deletes sess from the registry. If the worker already has
// a newer session it does nothing.
func (r *registry) remove(pub []byte, sess *muxer.Session) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sessions[string(pub)] == sess {
		delete(r.sessions, string(pub))
	}
}

// get returns the worker's session, or nil.
func (r *registry) get(pub []byte) *muxer.Session {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.sessions[string(pub)]
}
