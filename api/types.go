package api

import (
	"net/netip"
	"time"
)

type HealthResponse struct {
	Status string `json:"status"`
}

type Image struct {
	Reference string `json:"reference"`
	Digest    string `json:"digest"`
	SizeBytes int64  `json:"size_bytes"`
}

type PullImageRequest struct {
	Reference string `json:"reference"`
}

type ImagePullEvent struct {
	Type         string        `json:"type"`
	Status       string        `json:"status,omitempty"`
	Name         string        `json:"name,omitempty"`
	Digest       string        `json:"digest,omitempty"`
	CurrentBytes int64         `json:"current_bytes,omitempty"`
	TotalBytes   int64         `json:"total_bytes,omitempty"`
	Image        Image         `json:"image,omitzero"`
	Error        ErrorResponse `json:"error,omitzero"`
}

type CreateContainerRequest struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

type ErrorResponse struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

const (
	ErrorCodeNotFound            = "not_found"
	ErrorCodeAlreadyExists       = "already_exists"
	ErrorCodeOperationNotAllowed = "operation_not_allowed"
	ErrorCodeInternal            = "internal_error"
	ErrorCodeValidationFailed    = "validation_failed"
	ErrorCodeInvalidJSON         = "invalid_json"
	ErrorCodeUnsupportedMediaType = "unsupported_media_type"
)

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
