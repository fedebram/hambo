package rpc

import (
	"sync"

	"github.com/fedebram/hambo/muxer"
)

// Handler serves one stream opened for its method.
// If it returns an error, the error is sent to the other side.
type Handler func(s *Stream) error

// Router sends each incoming stream to the handler of its method.
type Router struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

func NewRouter() *Router {
	return &Router{handlers: make(map[string]Handler)}
}

// Handle registers h for method. It panics if method is already registered.
func (r *Router) Handle(method string, h Handler) {
	if h == nil {
		panic("rpc: nil handler for " + method)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.handlers[method]; ok {
		panic("rpc: method already registered: " + method)
	}
	r.handlers[method] = h
}

// Serve serves every stream the other side opens on sess,
// until the session ends.
func (r *Router) Serve(sess *muxer.Session) error {
	for {
		st, err := sess.Accept()
		if err != nil {
			return err
		}
		go r.serveStream(st)
	}
}

// serveStream finds the handler of the stream's method and runs it.
func (r *Router) serveStream(st *muxer.Stream) {
	s := newStream(st)
	defer s.CloseSend()

	r.mu.RLock()
	h, ok := r.handlers[s.Method()]
	r.mu.RUnlock()

	if !ok {
		s.sendError("unknown method: " + s.Method())
		return
	}
	if err := h(s); err != nil {
		s.sendError(err.Error())
	}
}
