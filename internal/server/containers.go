package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/rpc"
	"github.com/fedebram/hambo/internal/store"
)

func (s *Server) runContainer(ctx context.Context, stream *rpc.Stream) error {
	var spec api.ContainerSpec
	if err := stream.Recv(&spec); err != nil {
		return err
	}
	if spec.Name == "" {
		return rpc.Errorf("the name is required")
	}
	if spec.Image == "" {
		return rpc.Errorf("the image is required")
	}

	c, err := s.st.AddContainer(ctx, spec)
	if errors.Is(err, store.ErrNameTaken) {
		return rpc.Errorf("container %s already exists", spec.Name)
	}
	if err != nil {
		return err
	}
	fmt.Println("container added:", c.Spec.Name, c.ID)
	s.scheduleLater(c.ID)
	return stream.Send(c)
}

func (s *Server) listContainers(ctx context.Context, stream *rpc.Stream) error {
	containers, err := s.st.ListContainers(ctx)
	if err != nil {
		return err
	}
	return stream.Send(containers)
}

func (s *Server) removeContainer(ctx context.Context, stream *rpc.Stream) error {
	var name string
	if err := stream.Recv(&name); err != nil {
		return err
	}
	found, err := s.st.RemoveContainer(ctx, name)
	if err != nil {
		return err
	}
	if !found {
		return rpc.Errorf("no container named %s", name)
	}
	fmt.Println("container removed:", name)
	return stream.Send("removed")
}
