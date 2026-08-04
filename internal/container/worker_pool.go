package container

import (
	"context"
	"sync"
)

func RunWorkerPool(
	ctx context.Context,
	count int,
	store Store,
	runtime Runtime,
	queue Queue,
	reportError func(error),
) {
	runWorkerPool(ctx, count, newWorker(store, runtime, queue), reportError)
}

func runWorkerPool(ctx context.Context, count int, worker *worker, reportError func(error)) {
	var workers sync.WaitGroup

	for range count {
		workers.Add(1)

		go func() {
			defer workers.Done()

			for {
				err := worker.run()
				if err == nil {
					return
				}

				// we use this reportError callback to have great flexibility in what we can do
				// with the errors returned by the workers.
				// we can also simply log the errors without the need for the worker to have a logger dependency.
				reportError(err)
			}
		}()
	}

	<-ctx.Done()

	worker.queue.Shutdown()
	workers.Wait()
}
