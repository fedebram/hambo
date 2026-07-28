package container

import (
	"sync"
)

type Store interface {
	Create(Container) error
	Get(name string) (Container, error)
	Modify(name string, modify func(*Container)) error
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

func (s *MemoryStore) Modify(name string, modify func(*Container)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.containers[name]
	if !ok {
		return ErrNotFound
	}

	modify(&c)
	s.containers[name] = c
	return nil
}
