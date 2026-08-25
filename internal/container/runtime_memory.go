package container

import (
	"context"
	"fmt"
	"sync"
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

func (r *MemoryRuntime) Inspect(_ context.Context, id string) (RuntimeContainer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, found := r.containers[id]
	if !found {
		return RuntimeContainer{}, fmt.Errorf(
			"inspect runtime container %q: %w",
			id,
			ErrNotFound,
		)
	}
	if c.Task != nil {
		// return a copy so callers cannot mutate the task stored by the runtime
		task := *c.Task
		c.Task = &task
	}
	return c, nil
}

func (r *MemoryRuntime) CreateContainer(_ context.Context, id, image string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, found := r.containers[id]; found {
		return fmt.Errorf(
			"create runtime container %q: %w",
			id,
			ErrAlreadyExists,
		)
	}

	r.containers[id] = RuntimeContainer{
		ID:    id,
		Image: image,
	}

	return nil
}

func (r *MemoryRuntime) DeleteContainer(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, found := r.containers[id]
	if !found {
		return nil
	}
	if c.Task != nil {
		return fmt.Errorf(
			"delete runtime container %q while its task exists: %w",
			id,
			ErrOperationNotAllowed,
		)
	}

	delete(r.containers, id)
	return nil
}

func (r *MemoryRuntime) CreateTask(_ context.Context, containerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, found := r.containers[containerID]
	if !found {
		return fmt.Errorf(
			"create task for runtime container %q: container %w",
			containerID,
			ErrNotFound,
		)
	}
	if c.Task != nil {
		return fmt.Errorf(
			"create task for runtime container %q: task %w",
			containerID,
			ErrAlreadyExists,
		)
	}

	c.Task = &RuntimeTask{State: TaskStateCreated}
	r.containers[containerID] = c
	return nil
}

func (r *MemoryRuntime) StartTask(_ context.Context, containerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, found := r.containers[containerID]
	if !found {
		return fmt.Errorf(
			"start task for runtime container %q: container %w",
			containerID,
			ErrNotFound,
		)
	}
	if c.Task == nil {
		return fmt.Errorf(
			"start task for runtime container %q: task %w",
			containerID,
			ErrNotFound,
		)
	}

	switch c.Task.State {
	case TaskStateCreated:
		c.Task.State = TaskStateRunning
		return nil
	case TaskStateRunning:
		return nil
	default:
		return fmt.Errorf(
			"start task for runtime container %q from state %q: %w",
			containerID,
			c.Task.State,
			ErrOperationNotAllowed,
		)
	}
}

func (r *MemoryRuntime) StopTask(_ context.Context, containerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, found := r.containers[containerID]
	if !found {
		return fmt.Errorf(
			"stop task for runtime container %q: container %w",
			containerID,
			ErrNotFound,
		)
	}
	if c.Task == nil {
		return fmt.Errorf(
			"stop task for runtime container %q: task %w",
			containerID,
			ErrNotFound,
		)
	}

	switch c.Task.State {
	case TaskStateRunning:
		c.Task.State = TaskStateStopped
		return nil
	case TaskStateStopped:
		return nil
	default:
		return fmt.Errorf(
			"stop task for runtime container %q from state %q: %w",
			containerID,
			c.Task.State,
			ErrOperationNotAllowed,
		)
	}
}

func (r *MemoryRuntime) DeleteTask(_ context.Context, containerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, found := r.containers[containerID]
	if !found {
		return fmt.Errorf(
			"delete task for runtime container %q: container %w",
			containerID,
			ErrNotFound,
		)
	}
	if c.Task == nil {
		return nil
	}
	if c.Task.State == TaskStateRunning {
		return fmt.Errorf(
			"delete running task for runtime container %q: %w",
			containerID,
			ErrOperationNotAllowed,
		)
	}

	c.Task = nil
	r.containers[containerID] = c
	return nil
}
