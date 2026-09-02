package api

import (
	"net/netip"
	"time"
)

type HealthResponse struct {
	Status string `json:"status"`
}

type CreateContainerRequest struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

type ValidationErrorResponse struct {
	Errors map[string]string `json:"errors"`
}

type Container struct {
	Name              string            `json:"name"`
	Image             string            `json:"image"`
	Network           NetworkAttachment `json:"network,omitzero"`
	Exit              ContainerExit     `json:"exit,omitzero"`
	State             State             `json:"state"`
	Error             string            `json:"error,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	DeletionTimestamp time.Time         `json:"deletion_timestamp,omitzero"`
}

type NetworkAttachment struct {
	IP netip.Addr `json:"ip"`
}

type ContainerExit struct {
	Code     uint32    `json:"code"`
	ExitedAt time.Time `json:"exited_at"`
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
