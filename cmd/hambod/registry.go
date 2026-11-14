package main

import (
	"context"
	"sync"

	"github.com/fedebram/hambo/internal/muxer"
	"github.com/fedebram/hambo/internal/store"
)

// registry keeps the session of each connected node, by public key.
type registry struct {
	st *store.Store

	mu       sync.Mutex
	sessions map[string]*muxer.Session
}

func newRegistry(st *store.Store) *registry {
	return &registry{
		st:       st,
		sessions: make(map[string]*muxer.Session),
	}
}

// addSession saves the node's session, and closes its old session if
// there is one. It returns false if pub is not a worker in the store.
func (r *registry) addSession(ctx context.Context, pub []byte, sess *muxer.Session) bool {
	r.mu.Lock()
	old := r.sessions[string(pub)]
	r.sessions[string(pub)] = sess
	r.mu.Unlock()

	// we need to be sure that the old one is closed because the server might
	// not know that the old session ended.
	if old != nil {
		old.Close()
	}

	// adding to the registry and removing from the store are not atomic.
	// removeNode can delete the node from the store and find no session,
	// just before we add this one -> nobody closes it.
	// so we check the store again after adding the session.
	role, err := r.st.Role(ctx, pub)
	if err != nil || role != store.RoleWorker {
		r.removeSession(pub, sess)
		return false
	}
	return true
}

// removeSession takes sess out of the registry. If the node already has
// a newer session, it does nothing.
func (r *registry) removeSession(pub []byte, sess *muxer.Session) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// we need to be sure to delete THIS session. without the check:
	// the node reconnects -> the new session is added -> the old session
	// is closed -> removeSession is called -> it deletes the NEW session!
	if r.sessions[string(pub)] == sess {
		delete(r.sessions, string(pub))
	}
}

// removeNode deletes the node with this name from the store, takes its
// session out of the registry and closes it. found is false if there is
// no such node.
func (r *registry) removeNode(ctx context.Context, name string) (found bool, err error) {
	pub, found, err := r.st.RemoveNode(ctx, name)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	r.mu.Lock()
	sess := r.sessions[string(pub)]
	delete(r.sessions, string(pub))
	r.mu.Unlock()

	if sess != nil {
		sess.Close()
	}
	return true, nil
}

// hasSession reports whether the node has a session.
func (r *registry) hasSession(pub []byte) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.sessions[string(pub)] != nil
}
