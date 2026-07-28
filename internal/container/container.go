package container

import "time"

type Container struct {
	Name      string    `json:"name"`
	Image     string    `json:"image"`
	State     State     `json:"state,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type State string

const (
	StateCreating State = "creating"
	StateCreated  State = "created"
	StateRunning  State = "running"
)
