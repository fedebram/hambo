package container

import "sync"

// Inspired by the workqueue of kubernetes client-go
// https://github.com/kubernetes/client-go/blob/master/util/workqueue/queue.go
// TODO: license?

type Queue struct {
	mu           sync.Mutex
	cond         *sync.Cond
	items        []string
	shuttingDown bool
}

func NewQueue() *Queue {
	q := &Queue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *Queue) add(name string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.shuttingDown {
		return
	}

	q.items = append(q.items, name)

	q.cond.Signal()
}

func (q *Queue) get() (name string, shutdown bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.items) == 0 && !q.shuttingDown {
		q.cond.Wait()
	}

	// right now we don't drain the queue when shutting down.
	// it is fine like that, because on restart/startup we hydrate the queue.
	if q.shuttingDown {
		return "", true
	}

	name = q.items[0]
	q.items = q.items[1:]
	return name, false
}

func (q *Queue) len() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	return len(q.items)
}

func (q *Queue) shutdown() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.shuttingDown = true

	q.cond.Broadcast()
}
