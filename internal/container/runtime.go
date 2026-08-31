package container

import (
	"context"
	"time"
)

type Runtime interface {
	Inspect(ctx context.Context, id string) (RuntimeContainer, error)
	CreateContainer(ctx context.Context, id, image string) error
	DeleteContainer(ctx context.Context, id string) error
	CreateTask(ctx context.Context, containerID string) (RuntimeTask, error)
	StartTask(ctx context.Context, containerID string) error
	StopTask(ctx context.Context, containerID string) error
	DeleteTask(ctx context.Context, containerID string) error
}

type RuntimeContainer struct {
	ID    string
	Image string
	Task  *RuntimeTask
}

type RuntimeTask struct {
	PID       uint32
	NetNSPath string
	State     TaskState
	ExitCode  uint32
	ExitedAt  time.Time
}

type TaskState string

const (
	TaskStateCreated TaskState = "created"
	TaskStateRunning TaskState = "running"
	TaskStateStopped TaskState = "stopped"
)
