package rpc

import (
	"sync"
	"time"

	"github.com/fedebram/hambo/internal/muxer"
)

// methodTimeout is how long we wait for the method name after a stream opens.
const methodTimeout = 10 * time.Second

// Handler serves one stream that the other side opened for its method.
// If it returns an error, the other side's next Recv returns that error.
type Handler func(s *Stream) error

// Router runs the right handler for each stream the other side opens,
// picked by the method name the stream starts with.
type Router struct {
	mu       sync.Mutex
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
		raw, err := sess.Accept()
		if err != nil {
			return err
		}
		go r.serveStream(raw)
	}
}

// serveStream finds the handler of the stream's method and runs it.
func (r *Router) serveStream(raw *muxer.Stream) {
	s := newStream(raw)
	defer s.Close() // the call is over

	// the first message is the method name; give up if it doesn't come
	timer := time.AfterFunc(methodTimeout, func() { raw.Close() })
	var method string
	err := s.dec.Decode(&method)
	timer.Stop()
	if err != nil {
		return
	}

	r.mu.Lock()
	h, ok := r.handlers[method]
	r.mu.Unlock()

	if !ok {
		s.sendError("unknown method: " + method)
		return
	}
	if err := h(s); err != nil {
		s.sendError(err.Error())
	}
}
