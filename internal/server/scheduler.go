package server

import (
	"context"
	"fmt"

	"github.com/fedebram/hambo/internal/store"
)

// scheduleLater puts the container in the scheduler's queue.
func (s *Server) scheduleLater(id string) {
	s.schedQueue.Add(id)
}

// scheduleAllPending puts every container without a node in the queue.
func (s *Server) scheduleAllPending() {
	ids, err := s.st.PendingContainers(context.Background())
	if err != nil {
		fmt.Println("pending containers:", err)
		return
	}
	for _, id := range ids {
		s.scheduleLater(id)
	}
}

// runScheduler takes containers from the queue and gives each one a node,
// retrying with backoff when it fails. It runs until the queue shuts down.
func (s *Server) runScheduler() {
	for {
		id, shutdown := s.schedQueue.Get()
		if shutdown {
			return
		}
		if err := s.scheduleContainer(id); err != nil {
			fmt.Println("schedule container:", err)
			s.schedQueue.AddAfter(id, s.schedBackoff.Next(id))
		} else {
			s.schedBackoff.Reset(id)
		}
		s.schedQueue.Done(id)
	}
}

// scheduleContainer puts the container on the connected node with the
// fewest containers, if it's still pending. An error means retry.
func (s *Server) scheduleContainer(id string) error {
	ctx := context.Background()

	// here we make choices based on an optimistic approach.
	// this way we don't rely on transactions.
	// switching the store will be "easy" (like changing to rqlite which doesn't support interactive transactions)

	pending, err := s.st.IsPending(ctx, id)
	if err != nil {
		return err
	}
	// removed, or it already has a node
	if !pending {
		return nil
	}

	nodes, err := s.st.ContainersPerNode(ctx)
	if err != nil {
		return err
	}
	node, found := pickNode(nodes, s.reg)
	// no node is connected
	if !found {
		return nil
	}

	assigned, err := s.st.AssignContainer(ctx, id, node.PublicKey)
	if err != nil {
		return err
	}
	if !assigned {
		return fmt.Errorf("container %s changed while scheduling", id)
	}
	fmt.Println("container scheduled:", id, node.Name)
	// the node gets its new list of containers
	s.updateLater(node.PublicKey)
	return nil
}

// pickNode returns the connected node with the fewest containers.
// nodes is sorted by name, so on a tie the first name wins.
func pickNode(nodes []store.NodeCount, reg *registry) (store.NodeCount, bool) {
	var best store.NodeCount
	found := false
	for _, n := range nodes {
		if !reg.has(n.PublicKey) {
			continue
		}
		if !found || n.Count < best.Count {
			best = n
			found = true
		}
	}
	return best, found
}
