package server

import (
	"context"
	"slices"

	"github.com/fedebram/hambo/internal/rpc"
	"github.com/fedebram/hambo/internal/store"
)

// caller identifies who makes an rpc call: an admin, a node, or unknown.
type caller struct {
	PubKey []byte
	Role   store.Role
}

type callerKey struct{}

// withCaller returns a ctx that carries c.
func withCaller(ctx context.Context, c caller) context.Context {
	return context.WithValue(ctx, callerKey{}, c)
}

// callerFrom returns the caller in ctx, or an unknown one.
func callerFrom(ctx context.Context) caller {
	c, _ := ctx.Value(callerKey{}).(caller)
	return c
}

// requireRole lets through only callers that have one of these roles
// in the store. The others get "unknown method".
func (s *Server) requireRole(roles ...store.Role) func(rpc.Handler) rpc.Handler {
	return func(next rpc.Handler) rpc.Handler {
		return func(ctx context.Context, stream *rpc.Stream) error {
			c := callerFrom(ctx)
			role, err := s.st.Role(ctx, c.PubKey)
			if err != nil {
				return err
			}
			if !slices.Contains(roles, role) {
				return rpc.Errorf("unknown method")
			}
			c.Role = role
			return next(withCaller(ctx, c), stream)
		}
	}
}
