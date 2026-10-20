package watch

import (
	"errors"
	"sync"
)

type NodeWatcher struct {
	mu    sync.Mutex
	nodes map[string]chan struct{}
}

func NewNodeWatcher() *NodeWatcher {
	return &NodeWatcher{
		nodes: make(map[string]chan struct{}),
	}
}

func (w *NodeWatcher) Watch(nodeID string) (<-chan struct{}, error) {
	if nodeID == "" {
		return nil, errors.New("node ID must not be empty")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	ch, ok := w.nodes[nodeID]
	if !ok {
		ch = make(chan struct{})
		w.nodes[nodeID] = ch
	}

	return ch, nil
}

func (w *NodeWatcher) Notify(nodeID string) error {
	if nodeID == "" {
		return errors.New("node ID must not be empty")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	ch, ok := w.nodes[nodeID]
	if !ok {
		// no one is watching this node, nothing to do.
		return nil
	}

	// broadcasting
	close(ch)
	delete(w.nodes, nodeID)
	return nil
}
