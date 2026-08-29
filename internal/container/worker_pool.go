package container

import (
	"context"
	"sync"
	"time"
)

func RunWorkerPool(
	ctx context.Context,
	gracePeriod time.Duration,
	count int,
	store Store,
	runtime Runtime,
	netAttacher NetworkAttacher,
	queue Queue,
	reportError func(error),
) {
	runWorkerPool(ctx, gracePeriod, count, newWorker(store, runtime, netAttacher, queue), reportError)
}

func runWorkerPool(
	ctx context.Context,
	gracePeriod time.Duration,
	count int,
	worker *worker,
	reportError func(error),
) {
	operationCtx, cancelOperations := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelOperations()

	var workers sync.WaitGroup

	for range count {
		workers.Add(1)

		go func() {
			defer workers.Done()

			for {
				err := worker.run(operationCtx)
				if err == nil {
					return
				}

				// we use this reportError callback to have great flexibility in what we can do
				// with the errors returned by the workers.
				// we can also simply log the errors without the need for the worker to have a logger dependency.
				reportError(err)

				if operationCtx.Err() != nil {
					return
				}
			}
		}()
	}

	<-ctx.Done()

	worker.queue.Shutdown()

	timer := time.AfterFunc(gracePeriod, cancelOperations)
	defer timer.Stop()

	workers.Wait()
}
