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

func (s *Service) Start(name string) (Container, error) {
	var container Container
	err := s.store.Modify(name, func(c *Container) error {
		if !c.DeletionTimestamp.IsZero() {
			return fmt.Errorf(
				"%w: container %q cannot be started because deletion has been requested",
				ErrOperationNotAllowed,
				c.Name,
			)
		}
		if c.State != StateCreated {
			return fmt.Errorf(
				"%w: container %q cannot be started from state %q",
				ErrOperationNotAllowed,
				c.Name,
				c.State,
			)
		}

		c.State = StateStarting
		container = *c
		return nil
	})
	if err != nil {
		return Container{}, err
	}

	s.queue.Add(container.Name)
	return container, nil
}

func (s *Service) Stop(name string) (Container, error) {
	var container Container
	err := s.store.Modify(name, func(c *Container) error {
		if !c.DeletionTimestamp.IsZero() {
			return fmt.Errorf(
				"%w: container %q cannot be stopped because deletion has been requested",
				ErrOperationNotAllowed,
				c.Name,
			)
		}
		if c.State != StateRunning {
			return fmt.Errorf(
				"%w: container %q cannot be stopped from state %q",
				ErrOperationNotAllowed,
				c.Name,
				c.State,
			)
		}

		c.State = StateStopping
		container = *c
		return nil
	})
	if err != nil {
		return Container{}, err
	}

	s.queue.Add(container.Name)
	return container, nil
}

func (s *Service) Delete(name string) (Container, error) {
	var container Container
	err := s.store.Modify(name, func(c *Container) error {
		if !c.DeletionTimestamp.IsZero() {
			return fmt.Errorf(
				"%w: deletion for container %q already requested",
				ErrOperationNotAllowed,
				c.Name,
			)
		}
		switch c.State {
		case StateStarting, StateRunning, StateStopping:
			return fmt.Errorf(
				"%w: container %q must be stopped before requesting deletion",
				ErrOperationNotAllowed,
				c.Name,
			)
		}

		c.DeletionTimestamp = s.now().UTC()
		container = *c
		return nil
	})

	if err != nil {
		return Container{}, err
	}

	s.queue.Add(container.Name)
	return container, nil
}
