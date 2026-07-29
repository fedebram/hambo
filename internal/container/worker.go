package container

import (
	"errors"
	"time"
)

const retryDelay = time.Second

type worker struct {
	store   Store
	runtime runtime
	queue   Queue
}

func newWorker(store Store, runtime runtime, queue Queue) *worker {
	return &worker{
		store:   store,
		runtime: runtime,
		queue:   queue,
	}
}

func (w *worker) handle(name string) error {
	if err := w.runtime.create(name); err != nil && !errors.Is(err, ErrAlreadyExists) {
		return err
	}

	return w.store.Modify(name, func(container *Container) {
		container.State = StateCreated
	})
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
