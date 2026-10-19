package container

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

type State string

const (
	StatePending State = "pending"
	StateRunning State = "running"
	StateExited  State = "exited"
)

type Container struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image"`

	NodeID string `json:"node_id,omitempty"`

	State    State `json:"state"`
	ExitCode *int  `json:"exit_code,omitempty"`

	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`

	CreatedAt           time.Time  `json:"created_at"`
	DeletionRequestedAt *time.Time `json:"deletion_requested_at,omitempty"`
}

func GenerateContainerID() string {
	var id [16]byte
	rand.Read(id[:])
	return hex.EncodeToString(id[:])
}
