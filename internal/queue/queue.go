package queue

import (
	"sync"
	"time"
)

// Inspired by the workqueue of kubernetes client-go
// https://github.com/kubernetes/client-go/blob/master/util/workqueue/queue.go

type MemoryQueue struct {
	mu           sync.Mutex
	cond         *sync.Cond
	items        []string
	dirty        map[string]struct{}
	processing   map[string]struct{}
	shuttingDown bool
	shutdownCh   chan struct{}
}

func NewMemoryQueue() *MemoryQueue {
	q := &MemoryQueue{
		shutdownCh: make(chan struct{}),
		dirty:      make(map[string]struct{}),
		processing: make(map[string]struct{}),
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *MemoryQueue) Add(name string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.shuttingDown {
		return
	}
	if _, dirty := q.dirty[name]; dirty {
		return
	}

	q.dirty[name] = struct{}{}

	if _, processing := q.processing[name]; processing {
		return
	}

	q.items = append(q.items, name)

	q.cond.Signal()
}

func (q *MemoryQueue) AddAfter(name string, delay time.Duration) {
	q.mu.Lock()
	if q.shuttingDown {
		q.mu.Unlock()
		return
	}
	q.mu.Unlock()

	// client-go uses a waiting loop for this:
	// https://github.com/kubernetes/client-go/blob/master/util/workqueue/delaying_queue.go
	// A timer goroutine per delayed item is simpler and lightweight enough here.
	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()

		select {
		case <-timer.C:
			q.Add(name)
		case <-q.shutdownCh:
		}
	}()
}

func (q *MemoryQueue) Get() (name string, shutdown bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.items) == 0 && !q.shuttingDown {
		q.cond.Wait()
	}

	// the queue is not drained on shutdown: on startup every name is enqueued again.
	if q.shuttingDown {
		return "", true
	}

	name = q.items[0]
	// clear the entry so the backing array doesn't keep the old string alive
	q.items[0] = ""
	q.items = q.items[1:]
	delete(q.dirty, name)
	q.processing[name] = struct{}{}
	return name, false
}

func (q *MemoryQueue) Done(name string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	delete(q.processing, name)

	if _, dirty := q.dirty[name]; !dirty {
		return
	}

	q.items = append(q.items, name)
	q.cond.Signal()
}

func (q *MemoryQueue) Shutdown() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.shuttingDown {
		return
	}

	q.shuttingDown = true
	close(q.shutdownCh)

	q.cond.Broadcast()
}

func (q *MemoryQueue) len() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	return len(q.items)
}
