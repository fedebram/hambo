package container

import "errors"

type worker struct {
	store   Store
	runtime runtime
	queue   *queue
}

func newWorker(store Store, runtime runtime, queue *queue) *worker {
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
	name, shutdown := w.queue.get()
	if shutdown {
		return true, nil
	}

	if err = w.handle(name); err != nil {
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
