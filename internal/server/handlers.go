package server

import (
	"context"

	"github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/rpc"
	"github.com/fedebram/hambo/internal/store"
)

func (s *Server) handleCreateToken(ctx context.Context, stream *rpc.Stream) error {
	token, err := s.createToken(ctx)
	if err != nil {
		return err
	}
	return stream.Send(token)
}

func (s *Server) handleJoin(ctx context.Context, stream *rpc.Stream) error {
	c := callerFrom(ctx)
	if c.Role == store.RoleNode {
		return stream.Send("already joined")
	}

	var req api.JoinRequest
	if err := stream.Recv(&req); err != nil {
		return err
	}
	if err := s.joinNode(ctx, c.PubKey, req); err != nil {
		return err
	}
	return stream.Send("joined")
}

func (s *Server) handleListNodes(ctx context.Context, stream *rpc.Stream) error {
	nodes, err := s.listNodes(ctx)
	if err != nil {
		return err
	}
	return stream.Send(nodes)
}

func (s *Server) handleRemoveNode(ctx context.Context, stream *rpc.Stream) error {
	var name string
	if err := stream.Recv(&name); err != nil {
		return err
	}
	if err := s.removeNode(ctx, name); err != nil {
		return err
	}
	return stream.Send("removed")
}

func (s *Server) handleRunContainer(ctx context.Context, stream *rpc.Stream) error {
	var spec api.ContainerSpec
	if err := stream.Recv(&spec); err != nil {
		return err
	}
	c, err := s.runContainer(ctx, spec)
	if err != nil {
		return err
	}
	return stream.Send(c)
}

func (s *Server) handleListContainers(ctx context.Context, stream *rpc.Stream) error {
	containers, err := s.listContainers(ctx)
	if err != nil {
		return err
	}
	return stream.Send(containers)
}

func (s *Server) handleRemoveContainer(ctx context.Context, stream *rpc.Stream) error {
	var name string
	if err := stream.Recv(&name); err != nil {
		return err
	}
	if err := s.removeContainer(ctx, name); err != nil {
		return err
	}
	return stream.Send("removed")
}
