package container

import "time"

type Container struct {
	Name              string    `json:"name"`
	Image             string    `json:"image"`
	State             State     `json:"state"`
	CreatedAt         time.Time `json:"created_at"`
	DeletionTimestamp time.Time `json:"deletion_timestamp,omitzero"`
}

type State string

const (
	StateCreating State = "creating"
	StateCreated  State = "created"
	StateRunning  State = "running"
	StateDeleting State = "deleting"
)
