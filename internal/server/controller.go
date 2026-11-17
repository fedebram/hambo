package server

import (
	"fmt"
	"time"

	"github.com/fedebram/hambo/internal/backoff"
	"github.com/fedebram/hambo/internal/queue"
)

type controller struct {
	name    string
	syncers int
	sync    func(key string) error
	queue   *queue.MemoryQueue
	backoff *backoff.Backoff
}

func newController(name string, syncers int, sync func(key string) error) *controller {
	return &controller{
		name:    name,
		syncers: syncers,
		sync:    sync,
		queue:   queue.NewMemoryQueue(),
		backoff: backoff.NewBackoff(time.Second, time.Minute),
	}
}

// enqueue asks for key to be synced.
func (c *controller) enqueue(key string) {
	c.queue.Add(key)
}

// run starts the syncers. They stop when the controller shuts down.
func (c *controller) run() {
	for range c.syncers {
		go c.loop()
	}
}

func (c *controller) shutdown() {
	c.queue.Shutdown()
}

func (c *controller) loop() {
	for {
		key, shutdown := c.queue.Get()
		if shutdown {
			return
		}
		if err := c.sync(key); err != nil {
			fmt.Println(c.name+":", err)
			c.queue.AddAfter(key, c.backoff.Next(key))
		} else {
			c.backoff.Reset(key)
		}
		c.queue.Done(key)
	}
}
