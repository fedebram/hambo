package container

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const memoryRuntimeTaskExitBufferSize = 10

type memoryRuntimeTaskExitSubscription struct {
	ctx        context.Context
	taskExitCh chan RuntimeTaskExit
	errCh      chan error
}

type MemoryRuntime struct {
	mu                 sync.Mutex
	containers         map[string]RuntimeContainer
	nextPID            uint32
	nextSubscriptionID uint64
	subscriptions      map[uint64]memoryRuntimeTaskExitSubscription
}

func NewMemoryRuntime() *MemoryRuntime {
	return &MemoryRuntime{
		containers:    make(map[string]RuntimeContainer),
		nextPID:       1,
		subscriptions: make(map[uint64]memoryRuntimeTaskExitSubscription),
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

func (r *MemoryRuntime) CreateTask(_ context.Context, containerID string) (RuntimeTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, found := r.containers[containerID]
	if !found {
		return RuntimeTask{}, fmt.Errorf(
			"create task for runtime container %q: container %w",
			containerID,
			ErrNotFound,
		)
	}
	if c.Task != nil {
		return RuntimeTask{}, fmt.Errorf(
			"create task for runtime container %q: task %w",
			containerID,
			ErrAlreadyExists,
		)
	}

	pid := r.nextPID
	r.nextPID++
	task := RuntimeTask{
		PID:       pid,
		NetNSPath: fmt.Sprintf("/memory-runtime/%d/ns/net", pid),
		State:     TaskStateCreated,
	}
	c.Task = &task
	r.containers[containerID] = c
	return task, nil
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
		exitedAt := time.Now().UTC()
		c.Task.PID = 0
		c.Task.NetNSPath = ""
		c.Task.State = TaskStateStopped
		c.Task.ExitCode = 0
		c.Task.ExitedAt = exitedAt
		r.publishTaskExitLocked(RuntimeTaskExit{
			ContainerID: containerID,
			ExitCode:    c.Task.ExitCode,
			ExitedAt:    exitedAt,
		})
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

func (r *MemoryRuntime) SubscribeTaskExit(ctx context.Context) (<-chan RuntimeTaskExit, <-chan error) {
	taskExitCh := make(chan RuntimeTaskExit, memoryRuntimeTaskExitBufferSize)
	errCh := make(chan error)

	r.mu.Lock()
	subscriptionID := r.nextSubscriptionID
	r.nextSubscriptionID++
	r.subscriptions[subscriptionID] = memoryRuntimeTaskExitSubscription{
		ctx:        ctx,
		taskExitCh: taskExitCh,
		errCh:      errCh,
	}
	r.mu.Unlock()

	go func() {
		<-ctx.Done()

		r.mu.Lock()
		defer r.mu.Unlock()

		subscription, found := r.subscriptions[subscriptionID]
		if !found {
			return
		}

		delete(r.subscriptions, subscriptionID)
		close(subscription.taskExitCh)
		close(subscription.errCh)
	}()

	return taskExitCh, errCh
}

func (r *MemoryRuntime) publishTaskExitLocked(taskExit RuntimeTaskExit) {
	for _, subscription := range r.subscriptions {
		select {
		case subscription.taskExitCh <- taskExit:
		case <-subscription.ctx.Done():
		}
	}
}
