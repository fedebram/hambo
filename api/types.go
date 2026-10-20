package api

import "time"

type HealthResponse struct {
	Status string `json:"status"`
}

type ErrorResponse struct {
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

type CreateTokenRequest struct {
	TTL string `json:"ttl,omitempty"`
}

type CreateTokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type JoinNodeRequest struct {
	Name string `json:"name"`
	CSR  []byte `json:"csr"`
}

type JoinNodeResponse struct {
	NodeID      string `json:"node_id"`
	Certificate string `json:"certificate"`
}

type CreateContainerRequest struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

type ContainerResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image"`

	NodeID string `json:"node_id,omitempty"`

	State    ContainerState `json:"state"`
	ExitCode *int           `json:"exit_code,omitempty"`

	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`

	CreatedAt           time.Time  `json:"created_at"`
	DeletionRequestedAt *time.Time `json:"deletion_requested_at,omitempty"`
}

type ContainerState string

const (
	ContainerStatePending ContainerState = "pending"
	ContainerStateRunning ContainerState = "running"
	ContainerStateExited  ContainerState = "exited"
)

type ContainerAssignment struct {
	ID                  string     `json:"id"`
	Image               string     `json:"image"`
	DeletionRequestedAt *time.Time `json:"deletion_requested_at,omitempty"`
}

type NodeWatchResponse struct {
	Containers []ContainerAssignment `json:"containers"`
}

type ReportNodeStatusRequest struct {
	Sequence     uint64                  `json:"sequence"`
	RuntimeReady bool                    `json:"runtime_ready"`
	Containers   []ContainerStatusReport `json:"containers"`
}

type ContainerStatusReport struct {
	ID       string         `json:"id"`
	State    ContainerState `json:"state"`
	ExitCode *int           `json:"exit_code,omitempty"`
	Reason   string         `json:"reason,omitempty"`
	Message  string         `json:"message,omitempty"`
}
