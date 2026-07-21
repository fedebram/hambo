package container

import (
	"errors"
	"sync"
)

var ErrNotFound = errors.New("container not found")
var ErrAlreadyExists = errors.New("container already exists")

type Store interface {
	Create(Container) error
	Get(name string) (Container, error)
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
