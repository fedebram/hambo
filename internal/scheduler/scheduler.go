package scheduler

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/fedebram/hambo/internal/container"
	"github.com/fedebram/hambo/internal/node"
	"github.com/fedebram/hambo/internal/queue"
	"github.com/fedebram/hambo/internal/store"

	bolt "go.etcd.io/bbolt"
)

type Scheduler struct {
	db         *store.Store
	nodes      *store.Collection[node.Node]
	containers *store.Collection[container.Container]
	queue      queue.Queue
}

func NewScheduler(
	db *store.Store,
	workQueue queue.Queue,
	nodes *store.Collection[node.Node],
	containers *store.Collection[container.Container],
) *Scheduler {
	if db == nil {
		panic("scheduler: db cannot be nil")
	}
	if workQueue == nil {
		panic("scheduler: queue cannot be nil")
	}
	if nodes == nil {
		panic("scheduler: nodes collection cannot be nil")
	}
	if containers == nil {
		panic("scheduler: containers collection cannot be nil")
	}

	return &Scheduler{
		db:         db,
		nodes:      nodes,
		containers: containers,
		queue:      workQueue,
	}
}

var errNoReachableNodes = errors.New("no reachable nodes")

// for now we assign containers with a simple count:
// we pick a reachable node with the fewest assigned containers that haven't exited.
// containers waiting to start count too (their state is not StateExited), because they are already scheduled to a node.

func (s *Scheduler) assign(containerID string) error {
	retry := false

	err := s.db.WriteTx(func(tx *bolt.Tx) error {
		current, err := s.containers.GetTx(tx, containerID)
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}

		// the container might have changed while its id was waiting in the queue!
		if current.NodeID != "" ||
			current.DeletionRequestedAt != nil ||
			current.State != container.StatePending {
			return nil
		}

		nodes, err := s.nodes.ListTx(tx)
		if err != nil {
			return err
		}

		containers, err := s.containers.ListTx(tx)
		if err != nil {
			return err
		}

		// NodeID -> number of assigned containers that haven't exited
		counts := make(map[string]int)
		for _, entry := range containers {
			c := entry.Value
			if c.NodeID != "" && c.State != container.StateExited {
				counts[c.NodeID]++
			}
		}

		now := time.Now().UTC()
		var selectedNodeID string
		var selectedCount int

		for _, entry := range nodes {
			n := entry.Value
			if !n.IsReachable(now) {
				continue
			}

			count := counts[n.ID]
			if selectedNodeID == "" || count < selectedCount {
				selectedNodeID = n.ID
				selectedCount = count
			}
		}

		if selectedNodeID == "" {
			retry = true

			reason := "NoReachableNodes"
			message := "No reachable node is available. Scheduler will retry."
			if current.Reason == reason && current.Message == message {
				return nil
			}

			current.Reason = reason
			current.Message = message
			return s.containers.UpdateTx(tx, containerID, current)
		}

		current.NodeID = selectedNodeID
		current.Reason = ""
		current.Message = ""

		return s.containers.UpdateTx(tx, containerID, current)
	})
	if err != nil {
		return err
	}

	if retry {
		return errNoReachableNodes
	}
	return nil
}

const retryDelay = 10 * time.Second

func (s *Scheduler) Run() error {
	entries, err := s.containers.List()
	if err != nil {
		return fmt.Errorf("load pending containers: %w", err)
	}

	// Run is called on startup. we go through the stored containers
	// and queue the ones that still need an assignment.
	for _, entry := range entries {
		c := entry.Value
		if c.NodeID == "" &&
			c.State == container.StatePending &&
			c.DeletionRequestedAt == nil {
			s.queue.Add(entry.Key)
		}
	}

	for {
		containerID, shutdown := s.queue.Get()
		if shutdown {
			return nil
		}

		err := s.assign(containerID)
		if err != nil {
			if !errors.Is(err, errNoReachableNodes) {
				slog.Error(
					"assign container",
					"container_id", containerID,
					"error", err,
				)
			}

			s.queue.AddAfter(containerID, retryDelay)
		}

		s.queue.Done(containerID)
	}
}
