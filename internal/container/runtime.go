package container

import "sync"

type Runtime interface {
	Inspect(name string) (bool, error)
	Create(name string) error
	Delete(name string) error
}

type MemoryRuntime struct {
	mu         sync.Mutex
	containers map[string]struct{}
}

func NewMemoryRuntime() *MemoryRuntime {
	return &MemoryRuntime{
		containers: make(map[string]struct{}),
	}
}

func (r *MemoryRuntime) Inspect(name string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, found := r.containers[name]
	return found, nil
}

func (r *MemoryRuntime) Create(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, found := r.containers[name]; found {
		return ErrAlreadyExists
	}

	r.containers[name] = struct{}{}

	return nil
}

func (r *MemoryRuntime) Delete(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.containers, name)
	return nil
}
