package container

import (
	"slices"
	"sync"
)

type Store interface {
	Create(Container) error
	Get(name string) (Container, error)
	Modify(name string, modify func(*Container) error) error
	Delete(name string) error
	List() ([]Container, error)
}

type MemoryStore struct {
	containers map[string]Container
	mu         sync.Mutex
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		containers: make(map[string]Container),
	}
}

func (s *MemoryStore) Create(c Container) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.containers[c.Name]; ok {
		return ErrAlreadyExists
	}
	s.containers[c.Name] = c
	return nil
}

func (s *MemoryStore) Get(name string) (Container, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.containers[name]
	if !ok {
		return Container{}, ErrNotFound
	}
	return c, nil
}

func (s *MemoryStore) Modify(name string, modify func(*Container) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.containers[name]
	if !ok {
		return ErrNotFound
	}

	if err := modify(&c); err != nil {
		return err
	}
	s.containers[name] = c
	return nil
}

func (s *MemoryStore) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.containers, name)
	return nil
}

func (s *MemoryStore) List() ([]Container, error) {
	s.mu.Lock()

	cs := make([]Container, 0, len(s.containers))
	for _, container := range s.containers {
		cs = append(cs, container)
	}

	s.mu.Unlock()

	// basically, they will be ordered alphabetically
	slices.SortFunc(cs, func(a, b Container) int {
		switch {
		case a.Name < b.Name:
			return -1
		case a.Name > b.Name:
			return 1
		default:
			return 0
		}
	})

	return cs, nil
}
