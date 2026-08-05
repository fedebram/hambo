package container

import (
	"fmt"
	"time"
)

// the service needs only queue.add!! So no need for the entire behaviour of the queue.

type enqueuer interface {
	Add(name string)
}

type Service struct {
	store Store
	queue enqueuer
	now   func() time.Time
}

func NewService(store Store, queue enqueuer, options ...ServiceOption) *Service {
	s := &Service{
		store: store,
		queue: queue,
		now:   time.Now,
	}

	for _, option := range options {
		option(s)
	}
	return s
}

func (s *Service) Create(container Container) (Container, error) {
	container.State = StateCreating
	container.CreatedAt = s.now().UTC()

	if err := s.store.Create(container); err != nil {
		return Container{}, err
	}

	s.queue.Add(container.Name)

	return container, nil
}

func (s *Service) Get(name string) (Container, error) {
	return s.store.Get(name)
}

func (s *Service) Delete(name string) (Container, error) {
	var container Container
	err := s.store.Modify(name, func(c *Container) error {
		if c.State == StateRunning {
			return fmt.Errorf(
				"%w: stop container %q before requesting deletion",
				ErrOperationNotAllowed,
				c.Name,
			)
		}
		if c.DeletionTimestamp.IsZero() {
			c.DeletionTimestamp = s.now().UTC()
		}
		container = *c
		return nil
	})

	if err != nil {
		return Container{}, err
	}

	s.queue.Add(container.Name)
	return container, nil
}
