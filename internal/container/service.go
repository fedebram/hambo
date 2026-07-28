package container

import "time"

// the service needs only queue.add!! So no need for the entire behaviour of the queue.

type enqueuer interface {
	add(name string)
}

type service struct {
	store Store
	queue enqueuer
	now   func() time.Time
}

// option represents a function that modifies or extends the service.
type option func(*service)

// withClock configures the function used by the service to get the current time.
func withClock(now func() time.Time) option {
	if now == nil {
		panic("service: clock cannot be nil")
	}

	return func(s *service) {
		s.now = now
	}
}

func newService(store Store, queue enqueuer, options ...option) *service {
	s := &service{
		store: store,
		queue: queue,
		now:   time.Now,
	}

	for _, option := range options {
		option(s)
	}
	return s
}

func (s *service) create(container Container) (Container, error) {
	container.State = StateCreating
	container.CreatedAt = s.now().UTC()

	if err := s.store.Create(container); err != nil {
		return Container{}, err
	}

	s.queue.add(container.Name)

	return container, nil
}
