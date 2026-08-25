package container

import (
	"context"
	"errors"
	"time"
)

const retryDelay = time.Second

type worker struct {
	store   Store
	runtime Runtime
	queue   Queue
}

func newWorker(store Store, runtime Runtime, queue Queue) *worker {
	return &worker{
		store:   store,
		runtime: runtime,
		queue:   queue,
	}
}

func (w *worker) handle(name string) error {
	container, err := w.store.Get(name)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	if !container.DeletionTimestamp.IsZero() {
		return w.handleDeletion(container)
	}

	switch container.State {
	case StateCreating:
		return w.handleCreation(container)
	case StateStarting:
		return w.handleStart(container)
	case StateStopping:
		return w.handleStop(container)
	default:
		return nil
	}
}

func (w *worker) handleCreation(container Container) error {
	if err := w.runtime.CreateContainer(context.TODO(), container.Name, container.Image); err != nil {
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

func (w *worker) handleStart(container Container) error {
	if err := w.runtime.CreateTask(context.TODO(), container.Name); err != nil {
		return err
	}

	if err := w.runtime.StartTask(context.TODO(), container.Name); err != nil {
		return err
	}

	return w.store.Modify(container.Name, func(current *Container) error {
		current.State = StateRunning
		return nil
	})
}

func (w *worker) handleStop(container Container) error {
	if err := w.runtime.StopTask(context.TODO(), container.Name); err != nil {
		return err
	}

	if err := w.runtime.DeleteTask(context.TODO(), container.Name); err != nil {
		return err
	}

	return w.store.Modify(container.Name, func(current *Container) error {
		current.State = StateStopped
		return nil
	})
}

func (w *worker) handleDeletion(container Container) error {
	if err := w.store.Modify(container.Name, func(current *Container) error {
		current.State = StateDeleting
		return nil
	}); err != nil {
		return err
	}

	if err := w.runtime.DeleteContainer(context.TODO(), container.Name); err != nil {
		return err
	}

	return w.store.Delete(container.Name)
}

func (w *worker) handleNext() (shutdown bool, err error) {
	name, shutdown := w.queue.Get()
	if shutdown {
		return true, nil
	}
	defer w.queue.Done(name)

	if err = w.handle(name); err != nil {
		// fixed delay for now.
		w.queue.AddAfter(name, retryDelay)
		return false, err
	}

	return false, nil
}

func (w *worker) run() error {
	for {
		if shutdown, err := w.handleNext(); err != nil || shutdown {
			return err
		}
	}
}
