package container

import "time"

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
