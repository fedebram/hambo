package backoff

import (
	"sync"
	"time"
)

type Backoff struct {
	mu       sync.Mutex
	base     time.Duration
	max      time.Duration
	failures map[string]int
}

func NewBackoff(base, max time.Duration) *Backoff {
	if base > max {
		panic("backoff: base must not be greater than max")
	}
	return &Backoff{base: base, max: max, failures: make(map[string]int)}
}

func (b *Backoff) Next(id string) time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()

	delay := b.base
	for range b.failures[id] {
		delay = delay * 2
		if delay >= b.max {
			return b.max
		}
	}
	b.failures[id]++
	return delay
}

func (b *Backoff) Reset(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.failures, id)
}
