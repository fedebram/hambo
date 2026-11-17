package server

import (
	"sync"

	"github.com/fedebram/hambo/internal/muxer"
)

// sessions holds the session of each connected node, by public key.
type sessions struct {
	mu     sync.Mutex
	byNode map[string]*muxer.Session
}

func newSessions() *sessions {
	return &sessions{byNode: make(map[string]*muxer.Session)}
}

// add saves the node's session. If the node had an older one, it is
// closed.
func (s *sessions) add(pub []byte, sess *muxer.Session) {
	s.mu.Lock()
	old := s.byNode[string(pub)]
	s.byNode[string(pub)] = sess
	s.mu.Unlock()

	if old != nil && old != sess {
		old.Close()
	}
}

// remove takes sess out, only if it is still the node's session.
func (s *sessions) remove(pub []byte, sess *muxer.Session) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.byNode[string(pub)] == sess {
		delete(s.byNode, string(pub))
	}
}

// get returns the node's session, or nil if it isn't connected.
func (s *sessions) get(pub []byte) *muxer.Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.byNode[string(pub)]
}

func (s *sessions) isConnected(pub []byte) bool {
	return s.get(pub) != nil
}

// connected returns the keys of the connected nodes.
func (s *sessions) connected() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()

	var pubs [][]byte
	for pub := range s.byNode {
		pubs = append(pubs, []byte(pub))
	}
	return pubs
}
