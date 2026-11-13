package rpc

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/fedebram/hambo/internal/muxer"
)

const (
	// methodTimeout is how long we wait for the method name after a stream opens.
	methodTimeout = 10 * time.Second
	// lingerTimeout is how long a served stream may stay after our answer,
	// waiting for the caller to close it, before we reset it.
	lingerTimeout = time.Minute
)

// Handler serves one stream that the other side opened for its method.
// If it returns an error, the other side's next Recv returns that error.
// ctx is cancelled when the session ends.
type Handler func(ctx context.Context, s *Stream) error

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

// Serve serves every stream the other side opens on sess, until the
// session ends. Handlers get a ctx made from ctx, cancelled when the
// session ends.
func (r *Router) Serve(ctx context.Context, sess *muxer.Session) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for {
		raw, err := sess.Accept()
		if err != nil {
			return err
		}
		go r.serveStream(ctx, raw)
	}
}

// serveStream finds the handler of the stream's method and runs it.
func (r *Router) serveStream(ctx context.Context, raw *muxer.Stream) {
	s := newStream(raw)
	defer s.finish()

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
	if err := h(ctx, s); err != nil {
		var rpcErr *Error
		if errors.As(err, &rpcErr) {
			s.sendError(rpcErr.Msg)
		} else {
			log.Printf("rpc %s: %v", method, err)
			s.sendError("internal error")
		}
	}
}
