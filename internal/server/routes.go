package server

import (
	"context"

	"github.com/fedebram/hambo/internal/rpc"
	"github.com/fedebram/hambo/internal/store"
)

// routes returns the router with the methods clients can call.
func (s *Server) routes() *rpc.Router {
	admin := s.requireRole(store.RoleAdmin)
	anyone := s.requireRole(store.RoleAdmin, store.RoleNode)
	newcomer := s.requireRole(store.RoleUnknown, store.RoleNode)

	r := rpc.NewRouter()
	r.Handle("ping", anyone(s.handlePing))
	r.Handle("tokens.create", admin(s.handleCreateToken))
	r.Handle("join", newcomer(s.handleJoin))
	r.Handle("nodes.list", admin(s.handleListNodes))
	r.Handle("nodes.remove", admin(s.handleRemoveNode))
	r.Handle("containers.run", admin(s.handleRunContainer))
	r.Handle("containers.list", admin(s.handleListContainers))
	r.Handle("containers.remove", admin(s.handleRemoveContainer))
	return r
}

func (s *Server) handlePing(ctx context.Context, stream *rpc.Stream) error {
	return stream.Send("pong")
}
