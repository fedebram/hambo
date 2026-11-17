package server

import (
	"context"
	"encoding/hex"
	"fmt"
)

// onNodeConnected: if its endpoint changed, every node needs new peers.
// otherwise only this one needs its config. Pending containers may fit now.
func (s *Server) onNodeConnected(pub []byte, endpointChanged bool) {
	if endpointChanged {
		s.enqueueConnectedNodes()
	} else {
		s.enqueueNode(pub)
	}
	s.enqueuePendingContainers()
}

// onNodeRemoved: the nodeSyncer disconnects it, the others drop it from their
// peers.
func (s *Server) onNodeRemoved() {
	s.enqueueConnectedNodes()
}

func (s *Server) onContainerCreated(id string) {
	s.scheduler.enqueue(id)
}

// onContainerAssigned: the node gets its new list of containers.
func (s *Server) onContainerAssigned(node []byte) {
	s.enqueueNode(node)
}

// onContainerRemoved: the node gets the list without it.
// if the node is nil we don't enqueue.
func (s *Server) onContainerRemoved(node []byte) {
	if node != nil {
		s.enqueueNode(node)
	}
}

func (s *Server) enqueueNode(pub []byte) {
	s.nodeSyncer.enqueue(hex.EncodeToString(pub))
}

func (s *Server) enqueueConnectedNodes() {
	for _, pub := range s.sessions.connected() {
		s.enqueueNode(pub)
	}
}

func (s *Server) enqueuePendingContainers() {
	ids, err := s.st.PendingContainers(context.Background())
	if err != nil {
		fmt.Println("pending containers:", err)
		return
	}
	for _, id := range ids {
		s.scheduler.enqueue(id)
	}
}
