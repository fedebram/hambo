package container

import "sync"

type runtime interface {
	inspect(name string) (bool, error)
	create(name string) error
}

type memoryRuntime struct {
	mu         sync.Mutex
	containers map[string]struct{}
}

func newMemoryRuntime() *memoryRuntime {
	return &memoryRuntime{
		containers: make(map[string]struct{}),
	}
}

func (r *memoryRuntime) inspect(name string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, found := r.containers[name]
	return found, nil
}

func (r *memoryRuntime) create(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, found := r.containers[name]; found {
		return ErrAlreadyExists
	}

	r.containers[name] = struct{}{}

	return nil
}
