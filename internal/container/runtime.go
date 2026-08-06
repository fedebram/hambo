package container

import (
	"fmt"
	"sync"
)

type Runtime interface {
	Inspect(id string) (RuntimeContainer, error)
	CreateContainer(id, image string) error
	DeleteContainer(id string) error
	CreateTask(containerID string) error
	StartTask(containerID string) error
	StopTask(containerID string) error
	DeleteTask(containerID string) error
}

type RuntimeContainer struct {
	ID    string
	Image string
	Task  *RuntimeTask
}

type RuntimeTask struct {
	State TaskState
}

type TaskState string

const (
	TaskStateCreated TaskState = "created"
	TaskStateRunning TaskState = "running"
	TaskStateStopped TaskState = "stopped"
)

type MemoryRuntime struct {
	mu         sync.Mutex
	containers map[string]RuntimeContainer
}

func NewMemoryRuntime() *MemoryRuntime {
	return &MemoryRuntime{
		containers: make(map[string]RuntimeContainer),
	}
}

func (r *MemoryRuntime) Inspect(id string) (RuntimeContainer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, found := r.containers[id]
	if !found {
		return RuntimeContainer{}, ErrNotFound
	}
	if c.Task != nil {
		// return a copy so callers cannot mutate the task stored by the runtime
		task := *c.Task
		c.Task = &task
	}
	return c, nil
}

func (r *MemoryRuntime) CreateContainer(id, image string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, found := r.containers[id]; found {
		return ErrAlreadyExists
	}

	r.containers[id] = RuntimeContainer{
		ID:    id,
		Image: image,
	}

	return nil
}

func (r *MemoryRuntime) DeleteContainer(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, found := r.containers[id]
	if !found {
		return nil
	}
	if c.Task != nil {
		return fmt.Errorf(
			"%w: delete task before deleting runtime container %q",
			ErrOperationNotAllowed,
			id,
		)
	}

	delete(r.containers, id)
	return nil
}

func (r *MemoryRuntime) CreateTask(containerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, found := r.containers[containerID]
	if !found {
		return fmt.Errorf("%w: id %q", ErrNotFound, containerID)
	}
	if c.Task != nil {
		return ErrAlreadyExists
	}

	c.Task = &RuntimeTask{State: TaskStateCreated}
	r.containers[containerID] = c
	return nil
}

func (r *MemoryRuntime) StartTask(containerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, found := r.containers[containerID]
	if !found || c.Task == nil {
		return ErrNotFound
	}

	switch c.Task.State {
	case TaskStateCreated:
		// the runtime store is guarded by a mutex. So it is safe to mutate in place the content of the pointer.
		c.Task.State = TaskStateRunning
		return nil
	case TaskStateRunning:
		return nil
	default:
		return fmt.Errorf(
			"%w: task for runtime container %q cannot be started from state %q",
			ErrOperationNotAllowed,
			containerID,
			c.Task.State,
		)
	}
}

func (r *MemoryRuntime) StopTask(containerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, found := r.containers[containerID]
	if !found || c.Task == nil {
		return ErrNotFound
	}

	switch c.Task.State {
	case TaskStateRunning:
		c.Task.State = TaskStateStopped
		return nil
	case TaskStateStopped:
		return nil
	default:
		return fmt.Errorf(
			"%w: task for runtime container %q cannot be stopped from state %q",
			ErrOperationNotAllowed,
			containerID,
			c.Task.State,
		)
	}
}

func (r *MemoryRuntime) DeleteTask(containerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, found := r.containers[containerID]
	if !found || c.Task == nil {
		return nil
	}
	if c.Task.State == TaskStateRunning {
		return fmt.Errorf(
			"%w: stop task for runtime container %q before deleting it",
			ErrOperationNotAllowed,
			containerID,
		)
	}

	c.Task = nil
	r.containers[containerID] = c
	return nil
}
