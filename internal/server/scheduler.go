package server

import (
	"context"
	"fmt"

	"github.com/fedebram/hambo/internal/store"
)

// scheduleContainer puts the pending container on the connected node with the
// fewest containers.
func (s *Server) scheduleContainer(id string) error {
	ctx := context.Background()

	counts, err := s.st.ContainersPerNode(ctx)
	if err != nil {
		return err
	}
	node, found := pickNode(counts, s.sessions)
	// no node is connected
	if !found {
		return nil
	}

	assigned, err := s.st.AssignContainer(ctx, id, node.PublicKey)
	if err != nil {
		return err
	}
	// removed, or it already has a node: nothing to do
	if !assigned {
		return nil
	}
	fmt.Println("container scheduled:", id, node.Name)
	s.onContainerAssigned(node.PublicKey)
	return nil
}

// pickNode returns the connected node with the fewest containers.
// counts is sorted by name, so on a tie the first name wins.
func pickNode(counts []store.NodeCount, sessions *sessions) (store.NodeCount, bool) {
	var best store.NodeCount
	found := false
	for _, n := range counts {
		if !sessions.isConnected(n.PublicKey) {
			continue
		}
		if !found || n.Count < best.Count {
			best = n
			found = true
		}
	}
	return best, found
}
