package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/rpc"
	"github.com/fedebram/hambo/internal/store"
)

// runContainer saves a new pending container.
func (s *Server) runContainer(ctx context.Context, spec api.ContainerSpec) (api.Container, error) {
	if spec.Name == "" {
		return api.Container{}, rpc.Errorf("the name is required")
	}
	if spec.Image == "" {
		return api.Container{}, rpc.Errorf("the image is required")
	}

	c := store.Container{ID: newContainerID(), Spec: spec, CreatedAt: time.Now()}
	err := s.st.AddContainer(ctx, c)
	if errors.Is(err, store.ErrConflict) {
		return api.Container{}, rpc.Errorf("container %s already exists", spec.Name)
	}
	if err != nil {
		return api.Container{}, err
	}
	fmt.Println("container added:", spec.Name, c.ID)
	s.onContainerCreated(c.ID)
	return api.Container{ID: c.ID, Spec: c.Spec, CreatedAt: c.CreatedAt}, nil
}

// listContainers returns the containers, with the name of their node.
func (s *Server) listContainers(ctx context.Context) ([]api.Container, error) {
	stored, err := s.st.ListContainers(ctx)
	if err != nil {
		return nil, err
	}

	containers := []api.Container{}
	for _, c := range stored {
		containers = append(containers, api.Container{
			ID:        c.ID,
			Spec:      c.Spec,
			NodeName:  c.NodeName,
			CreatedAt: c.CreatedAt,
		})
	}
	return containers, nil
}

// removeContainer deletes the container (hard delete)
func (s *Server) removeContainer(ctx context.Context, name string) error {
	node, found, err := s.st.RemoveContainer(ctx, name)
	if err != nil {
		return err
	}
	if !found {
		return rpc.Errorf("no container named %s", name)
	}
	fmt.Println("container removed:", name)
	s.onContainerRemoved(node)
	return nil
}

func newContainerID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
