package container

import (
	"context"
	"errors"
	"fmt"
)

type worker struct {
	store       Store
	runtime     Runtime
	netAttacher NetworkAttacher
	queue       Queue
}

func newWorker(store Store, runtime Runtime, netAttacher NetworkAttacher, queue Queue) *worker {
	return &worker{
		store:       store,
		runtime:     runtime,
		netAttacher: netAttacher,
		queue:       queue,
	}
}

func (w *worker) handle(ctx context.Context, name string) error {
	container, err := w.store.Get(name)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	if !container.DeletionTimestamp.IsZero() {
		return w.handleDeletion(ctx, container)
	}

	switch container.State {
	case StateCreating:
		return w.handleCreation(ctx, container)
	case StateStarting:
		return w.handleStart(ctx, container)
	case StateRunning:
		return w.handleRunning(ctx, container)
	case StateStopping:
		return w.handleStop(ctx, container)
	default:
		return nil
	}
}

func (w *worker) handleCreation(ctx context.Context, container Container) error {
	if err := w.runtime.CreateContainer(ctx, container.Name, container.Image); err != nil {
		return err
	}

	return w.store.Modify(container.Name, func(current *Container) error {
		// between the store get in handle and this modify, the container state can't change thanks to the queue.
		// but what if the container service is buggy and changes the state?
		// TODO: add a guard?
		current.State = StateCreated
		return nil
	})
}

func (w *worker) handleStart(ctx context.Context, container Container) error {
	task, err := w.runtime.CreateTask(ctx, container.Name)
	if err != nil {
		return err
	}

	network, err := w.netAttacher.Attach(ctx, container.Name, task.NetNSPath)
	if err != nil {
		return errors.Join(
			err,
			w.netAttacher.Detach(ctx, container.Name, task.NetNSPath),
			w.runtime.DeleteTask(ctx, container.Name),
		)
	}

	if err := w.runtime.StartTask(ctx, container.Name); err != nil {
		return errors.Join(
			err,
			w.netAttacher.Detach(ctx, container.Name, task.NetNSPath),
			w.runtime.DeleteTask(ctx, container.Name),
		)
	}

	return w.store.Modify(container.Name, func(current *Container) error {
		current.State = StateRunning
		current.Network = network
		return nil
	})
}

func (w *worker) handleRunning(ctx context.Context, container Container) error {
	runtimeContainer, err := w.runtime.Inspect(ctx, container.Name)
	if err != nil {
		return err
	}
	if runtimeContainer.Task == nil {
		return fmt.Errorf(
			"handle running container %q: runtime task %w",
			container.Name,
			ErrNotFound,
		)
	}

	switch runtimeContainer.Task.State {
	case TaskStateRunning:
		return nil
	case TaskStateStopped:
		return w.cleanupStoppedTask(ctx, container, *runtimeContainer.Task)
	default:
		return fmt.Errorf(
			"handle running container %q with runtime task in state %q: %w",
			container.Name,
			runtimeContainer.Task.State,
			ErrOperationNotAllowed,
		)
	}
}

func (w *worker) handleStop(ctx context.Context, container Container) error {
	if err := w.runtime.StopTask(ctx, container.Name); err != nil {
		return err
	}

	runtimeContainer, err := w.runtime.Inspect(ctx, container.Name)
	if err != nil {
		return err
	}
	if runtimeContainer.Task == nil {
		return fmt.Errorf(
			"handle stopping container %q: runtime task %w",
			container.Name,
			ErrNotFound,
		)
	}
	if runtimeContainer.Task.State != TaskStateStopped {
		return fmt.Errorf(
			"handle stopping container %q with runtime task in state %q: %w",
			container.Name,
			runtimeContainer.Task.State,
			ErrOperationNotAllowed,
		)
	}

	return w.cleanupStoppedTask(ctx, container, *runtimeContainer.Task)
}

func (w *worker) cleanupStoppedTask(ctx context.Context, container Container, task RuntimeTask) error {
	if err := w.netAttacher.Detach(ctx, container.Name, ""); err != nil {
		return err
	}

	if err := w.runtime.DeleteTask(ctx, container.Name); err != nil {
		return err
	}

	return w.store.Modify(container.Name, func(current *Container) error {
		current.State = StateStopped
		current.Network = NetworkAttachment{}
		current.Exit = ContainerExit{
			Code:     task.ExitCode,
			ExitedAt: task.ExitedAt,
		}
		return nil
	})
}

func (w *worker) handleDeletion(ctx context.Context, container Container) error {
	if err := w.store.Modify(container.Name, func(current *Container) error {
		current.State = StateDeleting
		return nil
	}); err != nil {
		return err
	}

	if err := w.runtime.DeleteContainer(ctx, container.Name); err != nil {
		return err
	}

	return w.store.Delete(container.Name)
}

func (w *worker) handleNext(ctx context.Context) (shutdown bool, err error) {
	name, shutdown := w.queue.Get()
	if shutdown {
		return true, nil
	}
	defer w.queue.Done(name)

	if err = w.handle(ctx, name); err != nil {
		if recordErr := w.store.Modify(name, func(container *Container) error {
			container.Error = err.Error()
			return nil
		}); recordErr != nil {
			return false, errors.Join(
				err,
				fmt.Errorf("record worker error for container %q: %w", name, recordErr),
			)
		}
		return false, err
	}

	return false, nil
}

func (w *worker) run(ctx context.Context) error {
	for {
		if shutdown, err := w.handleNext(ctx); err != nil || shutdown {
			return err
		}
	}
}
