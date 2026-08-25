package container

import "time"

type Container struct {
	Name              string    `json:"name"`
	Image             string    `json:"image"`
	State             State     `json:"state"`
	Error             string    `json:"error,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	DeletionTimestamp time.Time `json:"deletion_timestamp,omitzero"`
}

type State string

const (
	StateCreating State = "creating"
	StateCreated  State = "created"
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateStopping State = "stopping"
	StateStopped  State = "stopped"
	StateDeleting State = "deleting"
)
